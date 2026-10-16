package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mango3s/internal/config"
)

func fullInput() ProfileInput {
	return ProfileInput{
		Name: "prod", Folder: "Clients / ACME",
		Server:   config.ServerConfig{Mode: config.ServerModeManual, Type: config.TypeReplicaSet, ReplicaSet: "rs0", Hosts: []config.HostPort{{Host: "db1", Port: 27017}, {Host: "db2"}}},
		Username: "app", Password: "db-pass", AuthSource: "admin", AuthMechanism: config.MechSCRAM256,
		TLS:      config.TLSConfig{Enabled: true},
		SSH:      config.SSHConfig{Enabled: true, Host: "bastion", Username: "ops", AuthMethod: config.SSHAuthPassword},
		Proxy:    config.ProxyConfig{Type: config.ProxySOCKS5, Host: "proxy", Username: "pu"},
		Shell:    config.ShellConfig{DefaultDB: "shop", InitScript: "show collections", RunInitScript: true},
		Tools:    config.ToolsConfig{Dir: "/opt/mongo/bin"},
		Advanced: config.AdvancedConfig{ReadPreference: "secondaryPreferred", WriteConcernW: "majority", MaxPoolSize: 50},
		Secrets: map[string]SecretInput{
			config.SecretSSHPassword:   {Value: "ssh-pass"},
			config.SecretProxyPassword: {Value: "proxy-pass"},
		},
	}
}

func TestSaveProfileAllTabsAndSecrets(t *testing.T) {
	f := newFixture(t)
	p, err := f.app.SaveProfile(fullInput())
	if err != nil {
		t.Fatal(err)
	}
	if p.URI != "mongodb://db1:27017,db2:27017/?replicaSet=rs0" || p.Folder != "Clients/ACME" || p.SSH.Port != 22 || p.Proxy.Port != 1080 {
		t.Fatalf("generated profile: %+v", p)
	}
	if !p.HasPassword || !p.HasSecret(config.SecretSSHPassword) || !p.HasSecret(config.SecretProxyPassword) || p.HasSecret(config.SecretTLSKeyPassword) {
		t.Fatalf("secret flags: %v %v", p.HasPassword, p.StoredSecrets)
	}
	for key, want := range map[string]string{p.ID: "db-pass", p.ID + ":sshPassword": "ssh-pass", p.ID + ":proxyPassword": "proxy-pass"} {
		if v, _ := f.secrets.Get(key); v != want {
			t.Fatalf("secret %s = %q", key, v)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(f.dir, "connections.json"))
	for _, s := range []string{"db-pass", "ssh-pass", "proxy-pass"} {
		if strings.Contains(string(raw), s) || strings.Contains(f.logs.String(), s) {
			t.Fatalf("%s leaked into connections.json or logs", s)
		}
	}
	// Reload: everything persisted.
	got, _ := f.app.store.GetProfile(p.ID)
	if got.Shell.InitScript != "show collections" || got.Advanced.MaxPoolSize != 50 || got.Tools.Dir != "/opt/mongo/bin" || got.Server.Hosts[1].Port != 27017 {
		t.Fatalf("persisted: %+v", got)
	}

	// Editing without secret values keeps them; clearing one removes only it.
	in := fullInput()
	in.ID, in.Password = p.ID, ""
	in.Secrets = map[string]SecretInput{config.SecretProxyPassword: {Clear: true}}
	p2, err := f.app.SaveProfile(in)
	if err != nil {
		t.Fatal(err)
	}
	if !p2.HasPassword || !p2.HasSecret(config.SecretSSHPassword) || p2.HasSecret(config.SecretProxyPassword) {
		t.Fatalf("after edit: %v %v", p2.HasPassword, p2.StoredSecrets)
	}
	if _, err := f.secrets.Get(p.ID + ":proxyPassword"); err == nil {
		t.Fatal("cleared secret still stored")
	}

	// Credentials: stored values, with form values taking precedence.
	c, err := f.app.credentials(p2, map[string]SecretInput{config.SecretSSHPassword: {Value: "typed"}})
	if err != nil || c.Password != "db-pass" || c.SSHPassword != "typed" || c.ProxyPassword != "" {
		t.Fatalf("credentials: %+v %v", c, err)
	}
}

func TestDuplicateAndDeleteProfile(t *testing.T) {
	f := newFixture(t)
	p, err := f.app.SaveProfile(fullInput())
	if err != nil {
		t.Fatal(err)
	}
	_ = f.app.RecordCollectionOpened(p.ID, "shop", "orders")
	cp, err := f.app.DuplicateProfile(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	cp2, _ := f.app.DuplicateProfile(p.ID)
	if cp.ID == p.ID || cp.Name != "prod (copy)" || cp2.Name != "prod (copy 2)" || cp.SSH.Host != "bastion" || cp.LastConnectedAt != nil {
		t.Fatalf("duplicate: %+v / %s", cp, cp2.Name)
	}
	if v, _ := f.secrets.Get(cp.ID + ":sshPassword"); v != "ssh-pass" {
		t.Fatal("duplicate must copy secrets")
	}
	if h, _ := f.app.ListCollectionHistory(cp.ID); len(h) != 1 {
		t.Fatal("duplicate must copy collection history")
	}
	if err := f.app.DeleteProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{p.ID, p.ID + ":sshPassword", p.ID + ":proxyPassword"} {
		if _, err := f.secrets.Get(k); err == nil {
			t.Fatalf("secret %s survived deleting the profile", k)
		}
	}
	if h, _ := f.app.ListCollectionHistory(p.ID); len(h) != 0 {
		t.Fatal("history survived deleting the profile")
	}
	if v, _ := f.secrets.Get(cp.ID); v != "db-pass" {
		t.Fatal("deleting the original must not touch the copy")
	}
}

func TestProfileValidationErrorsReachTheUI(t *testing.T) {
	f := newFixture(t)
	in := fullInput()
	in.SSH.Host = ""
	if e := decodeErr(t, func() error { _, err := f.app.SaveProfile(in); return err }()); !strings.HasPrefix(e.Message, "SSH:") {
		t.Fatalf("message should name the tab: %+v", e)
	}
	in = fullInput()
	in.AuthMechanism = config.MechGSSAPI
	if e := decodeErr(t, func() error { _, err := f.app.SaveProfile(in); return err }()); e.Code != "UNSUPPORTED" {
		t.Fatalf("Kerberos: %+v", e)
	}
	caps := f.app.ConnectionCapabilities()
	found := false
	for _, c := range caps["authMechanisms"] {
		if c.ID == config.MechGSSAPI && !c.Supported && c.Note != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("unsupported options must be listed as such")
	}
}

func TestParseConnectionStringNeverReturnsThePassword(t *testing.T) {
	f := newFixture(t)
	p, err := f.app.ParseConnectionString("mongodb://u:topsecret@h1:1,h2:2/?replicaSet=r")
	if err != nil || !p.HasPassword || p.Username != "u" || p.Server.Type != config.TypeReplicaSet {
		t.Fatalf("%+v %v", p, err)
	}
	if strings.Contains(strings.Join(p.Unmapped, ","), "topsecret") || strings.Contains(f.logs.String(), "topsecret") {
		t.Fatal("password leaked")
	}
}
