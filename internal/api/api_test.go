package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/query"
	"mango3s/internal/secrets"
)

type fixture struct {
	app     *App
	dir     string
	secrets *secrets.MemoryStore
	logs    *bytes.Buffer
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sec := secrets.NewMemoryStore()
	app := New(Deps{Log: log, Store: config.NewStore(dir), Secrets: sec, Conns: mongox.NewManager(log)})
	return fixture{app: app, dir: dir, secrets: sec, logs: &buf}
}

func decodeErr(t *testing.T, err error) apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var e apperr.Error
	if jerr := json.Unmarshal([]byte(err.Error()), &e); jerr != nil {
		t.Fatalf("error is not JSON: %q", err.Error())
	}
	return e
}

func TestSaveProfileMovesPasswordToSecretStore(t *testing.T) {
	f := newFixture(t)
	p, err := f.app.SaveProfile(ProfileInput{Name: "local", URI: "mongodb://bob:p%40ss@localhost:27017/?authSource=admin"})
	if err != nil {
		t.Fatal(err)
	}
	if p.URI != "mongodb://localhost:27017/?authSource=admin" || p.Username != "bob" || !p.HasPassword {
		t.Fatalf("profile: %+v", p)
	}
	if pw, _ := f.secrets.Get(p.ID); pw != "p@ss" {
		t.Fatalf("secret = %q", pw)
	}
	raw, _ := os.ReadFile(filepath.Join(f.dir, "connections.json"))
	if strings.Contains(string(raw), "p@ss") || strings.Contains(string(raw), "p%40ss") {
		t.Fatal("password persisted in connections.json")
	}
	if strings.Contains(f.logs.String(), "p@ss") || strings.Contains(f.logs.String(), "p%40ss") {
		t.Fatal("password logged")
	}

	// Editing without a password keeps the stored one.
	p2, err := f.app.SaveProfile(ProfileInput{ID: p.ID, Name: "renamed", URI: p.URI, Username: "bob"})
	if err != nil || !p2.HasPassword || p2.Name != "renamed" {
		t.Fatalf("keep password: %+v %v", p2, err)
	}
	if pw, _ := f.secrets.Get(p.ID); pw != "p@ss" {
		t.Fatal("stored password lost on edit")
	}

	// Clearing removes it.
	p3, err := f.app.SaveProfile(ProfileInput{ID: p.ID, Name: "renamed", URI: p.URI, Username: "bob", ClearPassword: true})
	if err != nil || p3.HasPassword {
		t.Fatalf("clear password: %+v %v", p3, err)
	}
	if _, err := f.secrets.Get(p.ID); err == nil {
		t.Fatal("password not deleted")
	}

	// Delete removes profile and secret.
	_ = f.secrets.Set(p.ID, "x")
	if err := f.app.DeleteProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := f.app.ListProfiles()
	if len(list) != 0 {
		t.Fatal("profile not deleted")
	}
	if _, err := f.secrets.Get(p.ID); err == nil {
		t.Fatal("secret not deleted with profile")
	}
}

func TestSaveProfileValidation(t *testing.T) {
	f := newFixture(t)
	e := decodeErr(t, func() error { _, err := f.app.SaveProfile(ProfileInput{Name: "x", URI: "localhost"}); return err }())
	if e.Code != apperr.CodeInvalidInput {
		t.Fatalf("code %s", e.Code)
	}
	e = decodeErr(t, func() error {
		_, err := f.app.SaveProfile(ProfileInput{Name: "x", URI: "mongodb://alice@h", Username: "bob"})
		return err
	}())
	if !strings.Contains(e.Message, "differs") {
		t.Fatalf("conflicting usernames: %+v", e)
	}
}

func TestOperationsRequireConnection(t *testing.T) {
	f := newFixture(t)
	_, err := f.app.RunQuery("op1", query.Request{ConnID: "nope", DB: "d", Coll: "c"})
	if e := decodeErr(t, err); e.Code != apperr.CodeNotConnected {
		t.Fatalf("code %s", e.Code)
	}
	_, err = f.app.ListDatabases("nope")
	if e := decodeErr(t, err); e.Code != apperr.CodeNotConnected {
		t.Fatalf("code %s", e.Code)
	}
}

func TestConnectFailureIsReadable(t *testing.T) {
	f := newFixture(t)
	p, err := f.app.SaveProfile(ProfileInput{Name: "dead", URI: "mongodb://127.0.0.1:1/?directConnection=true", ConnectTimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.Connect(p.ID)
	e := decodeErr(t, err)
	if e.Code != apperr.CodeUnreachable && e.Code != apperr.CodeTimeout {
		t.Fatalf("code %s: %s", e.Code, e.Message)
	}
	if len(f.app.ListConnections()) != 0 {
		t.Fatal("failed connection listed")
	}
}

func TestMissingSecretIsReported(t *testing.T) {
	f := newFixture(t)
	p, _ := f.app.SaveProfile(ProfileInput{Name: "x", URI: "mongodb://u:pw@127.0.0.1:1"})
	_ = f.secrets.Delete(p.ID)
	_, err := f.app.Connect(p.ID)
	if e := decodeErr(t, err); e.Code != apperr.CodeSecretBackend {
		t.Fatalf("code %s", e.Code)
	}
}

func TestCancelOperation(t *testing.T) {
	f := newFixture(t)
	started := make(chan struct{})
	done := make(chan error)
	go func() {
		_, err := call(f.app, "test", "op-cancel", func(ctx context.Context) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		})
		done <- err
	}()
	<-started
	if !f.app.CancelOperation("op-cancel") {
		t.Fatal("operation not found")
	}
	select {
	case err := <-done:
		if e := decodeErr(t, err); e.Code != apperr.CodeCanceled {
			t.Fatalf("code %s", e.Code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("operation not canceled")
	}
	if f.app.CancelOperation("op-cancel") {
		t.Fatal("finished operation still registered")
	}
}

func TestRerunSupersedesPrevious(t *testing.T) {
	r := newOpRegistry()
	ctx1, done1 := r.start(context.Background(), "same", time.Minute)
	ctx2, done2 := r.start(context.Background(), "same", time.Minute)
	if ctx1.Err() == nil {
		t.Fatal("first run should be canceled by the second")
	}
	done1() // must not unregister the second run
	if !r.cancel("same") || ctx2.Err() == nil {
		t.Fatal("second run should still be cancelable")
	}
	done2()
}

func TestSettingsRoundTrip(t *testing.T) {
	f := newFixture(t)
	s := f.app.GetSettings()
	s.PageSize = 7
	s.QueryTimeoutSeconds = 5
	saved, err := f.app.SaveSettings(s)
	if err != nil || saved.PageSize != 7 {
		t.Fatalf("%+v %v", saved, err)
	}
	if f.app.timeout() != 5*time.Second {
		t.Fatal("timeout not applied")
	}
}

func TestFormatDocument(t *testing.T) {
	f := newFixture(t)
	out, err := f.app.FormatDocument(`{a:1,b:'x'}`)
	if err != nil || out != "{\n  \"a\": 1,\n  \"b\": \"x\"\n}" {
		t.Fatalf("%q %v", out, err)
	}
	if e := decodeErr(t, func() error { _, err := f.app.FormatDocument(`{a:`); return err }()); e.Code != apperr.CodeInvalidInput {
		t.Fatalf("code %s", e.Code)
	}
}
