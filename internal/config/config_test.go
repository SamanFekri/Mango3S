package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCredentials(t *testing.T) {
	cases := []struct {
		in, uri, user, pass string
	}{
		{"mongodb://localhost:27017", "mongodb://localhost:27017", "", ""},
		{"mongodb://bob:s3cr%40t@h1:1,h2:2/db?authSource=admin", "mongodb://h1:1,h2:2/db?authSource=admin", "bob", "s3cr@t"},
		{"mongodb+srv://alice@cluster.example.net/?retryWrites=true", "mongodb+srv://cluster.example.net/?retryWrites=true", "alice", ""},
		{"mongodb://u:p@x/?appName=a@b", "mongodb://x/?appName=a@b", "u", "p"},
		{"  mongodb://u:p:q@host  ", "mongodb://host", "u", "p:q"},
	}
	for _, c := range cases {
		uri, user, pass, err := SplitCredentials(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if uri != c.uri || user != c.user || pass != c.pass {
			t.Errorf("%s: got (%q,%q,%q) want (%q,%q,%q)", c.in, uri, user, pass, c.uri, c.user, c.pass)
		}
	}
	if _, _, _, err := SplitCredentials("localhost:27017"); err == nil {
		t.Error("missing scheme accepted")
	}
}

func TestDefaultViewIsTreeUntilChosen(t *testing.T) {
	// Files written before Tree existed saved "json" without the user choosing it.
	if v := (Settings{DefaultView: "json"}).Normalize().DefaultView; v != "tree" {
		t.Fatalf("unchosen json default should become tree, got %s", v)
	}
	if v := (Settings{DefaultView: "table", DefaultViewChosen: true}).Normalize().DefaultView; v != "table" {
		t.Fatalf("an explicit choice must be kept, got %s", v)
	}
	if v := DefaultSettings().DefaultView; v != "tree" {
		t.Fatalf("factory default should be tree, got %s", v)
	}
}

func TestSettingsNormalize(t *testing.T) {
	s := Settings{Theme: "neon", PageSize: 5000, QueryTimeoutSeconds: -1, DefaultView: "x", EditorFontSize: 2, LogLevel: "trace"}.Normalize()
	d := DefaultSettings()
	if s.Theme != d.Theme || s.PageSize != 1000 || s.QueryTimeoutSeconds != d.QueryTimeoutSeconds ||
		s.DefaultView != "tree" || s.EditorFontSize != d.EditorFontSize || s.LogLevel != "info" {
		t.Fatalf("unexpected normalization: %+v", s)
	}
}

func TestStoreProfilesCRUD(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)

	list, err := st.ListProfiles()
	if err != nil || len(list) != 0 {
		t.Fatalf("empty store: %v %v", list, err)
	}
	if _, err := st.SaveProfile(Profile{Name: "", URI: "mongodb://x"}); err == nil {
		t.Fatal("missing name accepted")
	}
	if _, err := st.SaveProfile(Profile{Name: "bad", URI: "http://x"}); err == nil {
		t.Fatal("bad scheme accepted")
	}

	b, err := st.SaveProfile(Profile{Name: "beta", URI: "mongodb://b"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.SaveProfile(Profile{Name: "Alpha", URI: "mongodb://a", ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.ConnectTimeoutSeconds != 10 || a.CreatedAt.IsZero() {
		t.Fatalf("defaults not applied: %+v", a)
	}
	list, _ = st.ListProfiles()
	if len(list) != 2 || list[0].Name != "Alpha" || list[1].Name != "beta" {
		t.Fatalf("sorted list: %+v", list)
	}

	b.Name = "Beta renamed"
	if _, err := st.SaveProfile(b); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetProfile(b.ID)
	if err != nil || got.Name != "Beta renamed" {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := st.SaveProfile(Profile{ID: "missing", Name: "x", URI: "mongodb://x"}); err == nil {
		t.Fatal("update of missing profile accepted")
	}

	if err := st.DeleteProfile(a.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = st.ListProfiles()
	if len(list) != 1 {
		t.Fatalf("after delete: %+v", list)
	}

	info, err := os.Stat(filepath.Join(dir, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("connections.json perms = %v", info.Mode().Perm())
	}
}

func TestStoreSettingsAndCorruption(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	s, err := st.LoadSettings()
	if err != nil || s != DefaultSettings() {
		t.Fatalf("defaults: %+v %v", s, err)
	}
	s.PageSize = 100
	s.Theme = "dark"
	if _, err := st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got, _ := st.LoadSettings()
	if got.PageSize != 100 || got.Theme != "dark" {
		t.Fatalf("reload: %+v", got)
	}

	_ = os.WriteFile(filepath.Join(dir, "connections.json"), []byte("{not json"), 0o600)
	if _, err := st.ListProfiles(); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("expected corruption error, got %v", err)
	}
}
