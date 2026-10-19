package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mango3s/internal/config"
)

func TestConnectionExportImportRoundTrip(t *testing.T) {
	f := newFixture(t)
	src, err := f.app.SaveProfile(fullInput())
	if err != nil {
		t.Fatal(err)
	}
	// Without secrets: nothing secret in the file.
	plain, _, err := f.app.connectionsFile(nil, false, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"db-pass", "ssh-pass", "proxy-pass"} {
		if strings.Contains(string(plain), s) {
			t.Fatalf("%s in export", s)
		}
	}
	if _, _, err := f.app.connectionsFile(nil, true, "short"); err == nil {
		t.Fatal("weak passphrase accepted")
	}
	sealed, _, err := f.app.connectionsFile([]string{src.ID}, true, "a long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), "db-pass") {
		t.Fatal("password in clear text")
	}
	path := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(path, sealed, 0o600)

	// Same ID exists: the preview flags it; "add" makes a copy with the secrets.
	pv, err := f.app.PreviewConnectionImport(path, "a long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if len(pv.Items) != 1 || pv.Items[0].Conflict != "id" || !pv.SecretsUnlocked || len(pv.Items[0].SecretKinds) != 3 {
		t.Fatalf("%+v", pv)
	}
	if _, err := f.app.PreviewConnectionImport(path, "wrong passphrase"); err == nil {
		t.Fatal("wrong passphrase accepted")
	}
	out, err := f.app.ApplyConnectionImport(path, "a long passphrase", []ImportChoice{{Index: 0, Action: "add", Name: "prod copy", Folder: "Imported"}})
	if err != nil || out.Added != 1 || len(out.Errors) != 0 {
		t.Fatalf("%+v %v", out, err)
	}
	list, _ := f.app.store.ListProfiles()
	var cp config.Profile
	for _, p := range list {
		if p.Name == "prod copy" {
			cp = p
		}
	}
	if cp.ID == "" || cp.ID == src.ID || cp.Folder != "Imported" || !cp.HasPassword || !cp.HasSecret(config.SecretSSHPassword) {
		t.Fatalf("copy: %+v", cp)
	}
	if v, _ := f.secrets.Get(cp.ID); v != "db-pass" {
		t.Fatalf("password %q", v)
	}
	raw, _ := os.ReadFile(filepath.Join(f.dir, "connections.json"))
	if strings.Contains(string(raw), "db-pass") || strings.Contains(f.logs.String(), "db-pass") {
		t.Fatal("password leaked into settings or logs")
	}

	// A URI list with an embedded password: it goes to the credential store.
	os.WriteFile(path, []byte("Stage = mongodb://bob:s3cret@stage:27018/?authSource=admin\n"), 0o600)
	out, err = f.app.ApplyConnectionImport(path, "", []ImportChoice{{Index: 0, Action: "add"}})
	if err != nil || out.Added != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	list, _ = f.app.store.ListProfiles()
	for _, p := range list {
		if p.Name == "Stage" {
			if strings.Contains(p.URI, "s3cret") || p.Username != "bob" || !p.HasPassword {
				t.Fatalf("%+v", p)
			}
			if u, _ := f.app.ConnectionURI(p.ID, false); strings.Contains(u, "s3cret") || !strings.Contains(u, "bob@") {
				t.Fatalf("copied URI %q", u)
			}
			if u, _ := f.app.ConnectionURI(p.ID, true); !strings.Contains(u, "bob:s3cret@") {
				t.Fatalf("copied URI with password %q", u)
			}
		}
	}
}
