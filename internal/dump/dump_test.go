package dump

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/config"
	"mango3s/internal/mongox"
)

func TestGlobAndSelection(t *testing.T) {
	cases := []struct {
		pat, s string
		want   bool
	}{
		{"sales.*", "sales.orders", true},
		{"sales.*", "salesx.orders", false},
		{"*.tmp_*", "a.tmp_1", true},
		{"*.tmp_*", "a.tmp", false},
		{"a.b", "a.b", true},
		{"a.b", "a.bc", false},
		{"*", "x.y", true},
	}
	for _, c := range cases {
		if got := glob(c.pat, c.s); got != c.want {
			t.Errorf("glob(%q,%q)=%v", c.pat, c.s, got)
		}
	}
	s := Selection{Include: []string{"shop.*"}, Exclude: []string{"*.log*"}}
	if !s.Match("shop", "orders") || s.Match("shop", "logs") || s.Match("other", "orders") {
		t.Fatal("selection")
	}
}

func TestRename(t *testing.T) {
	o := RestoreOptions{Renames: []Rename{{From: "shop.*", To: "shop_copy.*"}, {From: "a.x", To: "b.y"}}}
	if db, c := o.Target("shop", "orders.2024"); db != "shop_copy" || c != "orders.2024" {
		t.Fatal(db, c)
	}
	if db, c := o.Target("a", "x"); db != "b" || c != "y" {
		t.Fatal(db, c)
	}
	if db, c := o.Target("z", "q"); db != "z" || c != "q" {
		t.Fatal(db, c)
	}
}

func TestEscapeNames(t *testing.T) {
	for _, n := range []string{"plain", "a/b", "100%", "x:y*z?"} {
		e := escapeName(n)
		if bytes.ContainsAny([]byte(e), "/\\:*?") || unescapeName(e) != n {
			t.Errorf("%q → %q → %q", n, e, unescapeName(e))
		}
	}
}

func TestDocReader(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 3; i++ {
		b, _ := bson.Marshal(bson.D{{Key: "i", Value: int32(i)}})
		buf.Write(b)
	}
	full := buf.Bytes()
	r := newDocReader(bytes.NewReader(full))
	n := 0
	for {
		_, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 3 {
		t.Fatal(n)
	}
	// Truncated and corrupt streams are errors, never a silent short read.
	if _, err := drain(full[:len(full)-3]); err == nil {
		t.Fatal("truncated stream accepted")
	}
	bad := append([]byte{}, full...)
	bad[0] = 2
	if _, err := drain(bad); err == nil {
		t.Fatal("bad length accepted")
	}
}

func drain(b []byte) (int, error) {
	r := newDocReader(bytes.NewReader(b))
	n := 0
	for {
		_, err := r.Next()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		n++
	}
}

func TestInspectMissingMetadataWarns(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "db"), 0o700)
	b, _ := bson.Marshal(bson.D{{Key: "_id", Value: 1}})
	os.WriteFile(filepath.Join(dir, "db", "c.bson"), b, 0o600)
	in, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Entries) != 1 || len(in.Warnings) != 1 || in.Entries[0].Expected != -1 {
		t.Fatalf("%+v", in)
	}
	if _, err := Inspect(t.TempDir()); err == nil {
		t.Fatal("empty folder accepted")
	}
}

func TestPrepareDirRefusesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x"), nil, 0o600)
	if err := prepareDir(dir); err == nil {
		t.Fatal("non-empty folder accepted")
	}
	if err := prepareDir(filepath.Join(dir, "new")); err != nil {
		t.Fatal(err)
	}
}

func TestToolArgsKeepSecretsOffTheCommandLine(t *testing.T) {
	p := config.Profile{Name: "x", URI: "mongodb://olduser:uripass@db.example:27017/?authSource=x", Username: "ann", AuthSource: "admin",
		TLS: config.TLSConfig{Enabled: true, CAFile: "/ca.pem", CertKeyFile: "/k.pem"}}
	ta, err := DumpArgs(p, mongox.Credentials{Password: "s3cr\"et", TLSKeyPassword: "keypw"}, ToolDump{Archive: filepath.Join(t.TempDir(), "a.archive"), Gzip: true, DB: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(ta.Args, " ")
	for _, sec := range []string{"s3cr", "keypw", "uripass", "olduser"} {
		if strings.Contains(line, sec) {
			t.Fatalf("secret %q on the command line: %s", sec, line)
		}
	}
	if !strings.Contains(line, "--username=ann") || !strings.Contains(line, "--authenticationDatabase=admin") || !strings.Contains(line, "--gzip") || !strings.Contains(line, "--db=shop") {
		t.Fatal(line)
	}
	st, err := os.Stat(ta.config)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("config file: %v %v", st, err)
	}
	b, _ := os.ReadFile(ta.config)
	if !strings.Contains(string(b), `password: "s3cr\"et"`) || !strings.Contains(string(b), `sslPEMKeyPassword: "keypw"`) || strings.Contains(string(b), "uripass") {
		t.Fatalf("config:\n%s", b)
	}
	if got := ta.Scrub("auth failed for s3cr\"et"); strings.Contains(got, "s3cr") {
		t.Fatal(got)
	}
	if d := toolDisplay("/bin/mongodump", ta.Args); strings.Contains(d, ta.config) {
		t.Fatal(d)
	}
	cfg := ta.config
	ta.Cleanup()
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Fatal("config file not removed")
	}
	// SSH and proxy connections can't be used by external tools.
	p.SSH.Enabled = true
	if _, err := DumpArgs(p, mongox.Credentials{}, ToolDump{Archive: "x"}); err == nil {
		t.Fatal("ssh accepted")
	}
}

// Replica set connection strings list several hosts, which net/url rejects.
func TestToolArgsAcceptSeveralHosts(t *testing.T) {
	p := config.Profile{Name: "rs", URI: "mongodb://a:27017,b:27017,c:27017/?replicaSet=rs0", Username: "ada"}
	ta, err := DumpArgs(p, mongox.Credentials{Password: "pw"}, ToolDump{Archive: filepath.Join(t.TempDir(), "a.archive"), DB: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	defer ta.Cleanup()
	b, _ := os.ReadFile(ta.config)
	if !strings.Contains(string(b), `mongodb://a:27017,b:27017,c:27017/`) || !strings.Contains(string(b), "replicaSet=rs0") {
		t.Fatalf("config file:\n%s", b)
	}
}
