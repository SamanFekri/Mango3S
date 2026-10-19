package connio

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"mango3s/internal/config"
)

func profiles() []config.Profile {
	now := time.Now()
	return []config.Profile{
		{ID: "p1", Name: "Prod", Folder: "Clients/Acme", URI: "mongodb://db1:27017,db2:27017/?replicaSet=rs0", Username: "ann", HasPassword: true,
			StoredSecrets: []string{config.SecretSSHPassword}, LastConnectedAt: &now, Position: 3, ReadOnly: true,
			SSH: config.SSHConfig{Enabled: true, Host: "bastion", Port: 22, Username: "ops", AuthMethod: config.SSHAuthPassword}},
		{ID: "p2", Name: "Local", URI: "mongodb://localhost:27017"},
	}
}

func TestExportWithoutSecretsHasNoSecrets(t *testing.T) {
	b, err := Export(profiles(), nil, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"secrets", "hasPassword\": true", "lastConnectedAt", "storedSecrets"} {
		if strings.Contains(s, bad) {
			t.Fatalf("export contains %q:\n%s", bad, s)
		}
	}
	p, err := Parse(b, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != FormatMango3S || len(p.Candidates) != 2 || p.HasSecrets {
		t.Fatalf("%+v", p)
	}
	c := p.Candidates[0]
	if c.Profile.Folder != "Clients/Acme" || !c.Profile.ReadOnly || !c.Profile.SSH.Enabled || c.Profile.Username != "ann" || c.Error != "" || len(c.Secrets) != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestExportedSecretsNeedThePassphrase(t *testing.T) {
	sec := Secrets{"p1": {config.SecretPassword: "hunter2-pw", config.SecretSSHPassword: "ssh-pw"}}
	if _, err := Export(profiles(), sec, "short", time.Now()); err == nil {
		t.Fatal("short passphrase accepted")
	}
	b, err := Export(profiles(), sec, "correct horse battery", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("hunter2")) || bytes.Contains(b, []byte("ssh-pw")) {
		t.Fatal("secret in clear text")
	}
	// Without a passphrase: connections import, secrets don't.
	p, err := Parse(b, "")
	if err != nil || !p.HasSecrets || p.SecretsUnlocked || len(p.Warnings) == 0 || len(p.Candidates[0].Secrets) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := Parse(b, "wrong passphrase!"); !errors.Is(err, ErrPassphrase) {
		t.Fatalf("wrong passphrase: %v", err)
	}
	p, err = Parse(b, "correct horse battery")
	if err != nil || !p.SecretsUnlocked {
		t.Fatal(err)
	}
	if p.Candidates[0].Secrets[config.SecretPassword] != "hunter2-pw" || p.Candidates[0].Secrets[config.SecretSSHPassword] != "ssh-pw" {
		t.Fatalf("%+v", p.Candidates[0].Secrets)
	}
	// Tampering is detected.
	var f File
	json.Unmarshal(b, &f)
	f.Secrets.Data[0] ^= 1
	b2, _ := json.Marshal(f)
	if _, err := Parse(b2, "correct horse battery"); !errors.Is(err, ErrPassphrase) {
		t.Fatalf("tampered: %v", err)
	}
}

func TestParseURIList(t *testing.T) {
	text := "# my servers\n\nmongodb://localhost:27017\nStaging = mongodb://bob:p%40ss@stage.example:27018/?authSource=admin\nAtlas\tmongodb+srv://cluster0.example.net/\nnot a uri\n"
	p, err := Parse([]byte(text), "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != FormatURIList || len(p.Candidates) != 4 {
		t.Fatalf("%+v", p)
	}
	if p.Candidates[0].Profile.Name != "localhost" {
		t.Fatalf("name %q", p.Candidates[0].Profile.Name)
	}
	st := p.Candidates[1]
	if st.Profile.Name != "Staging" || st.Profile.Username != "bob" || st.Secrets[config.SecretPassword] != "p@ss" || strings.Contains(st.Profile.URI, "p%40ss") || strings.Contains(st.Profile.URI, "bob") {
		t.Fatalf("%+v", st)
	}
	if p.Candidates[2].Profile.Name != "Atlas" || p.Candidates[2].Error != "" {
		t.Fatalf("%+v", p.Candidates[2])
	}
	if p.Candidates[3].Error == "" {
		t.Fatal("bad line accepted")
	}
}

func TestParseCompassExport(t *testing.T) {
	text := `{"type":"Compass Connections","version":{"$numberInt":"1"},"connections":[
	 {"id":"a","favorite":{"name":"Analytics"},"connectionOptions":{"connectionString":"mongodb://u:pw@an.example:27017/?tls=true",
	   "sshTunnel":{"host":"jump","port":{"$numberInt":"2222"},"username":"me","identityKeyFile":"/k"}},"connectionSecrets":"ZW5j"},
	 {"id":"b","connectionOptions":{"connectionString":"mongodb+srv://c.example.net"}}]}`
	p, err := Parse([]byte(text), "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != FormatCompass || len(p.Candidates) != 2 || len(p.Warnings) != 1 {
		t.Fatalf("%+v", p)
	}
	a := p.Candidates[0]
	if a.Profile.Name != "Analytics" || a.Secrets[config.SecretPassword] != "pw" || !a.Profile.SSH.Enabled || a.Profile.SSH.Port != 2222 || a.Profile.SSH.AuthMethod != config.SSHAuthKey {
		t.Fatalf("%+v", a)
	}
	if strings.Contains(a.Profile.URI, "pw") {
		t.Fatal(a.Profile.URI)
	}
	if p.Candidates[1].Profile.Name != "c.example.net" {
		t.Fatalf("%q", p.Candidates[1].Profile.Name)
	}
}

func TestParseRejectsUnknownFiles(t *testing.T) {
	for _, s := range []string{`{"hello":1}`, ``, "# nothing\n"} {
		if _, err := Parse([]byte(s), ""); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
	if _, err := Parse([]byte(`{"format":"mango3s-connections","version":9,"connections":[]}`), ""); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatal(err)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("mongodb://u:secret@h:1/x"); strings.Contains(got, "secret") {
		t.Fatal(got)
	}
}
