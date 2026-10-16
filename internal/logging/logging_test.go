package logging

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"mongodb://bob:hunter2@h:27017/db":           "mongodb://bob:****@h:27017/db",
		"mongodb+srv://a:b%40c@cluster.x/?w=1":       "mongodb+srv://a:****@cluster.x/?w=1",
		"mongodb://onlyuser@h":                       "mongodb://onlyuser@h",
		"auth password=hunter2&x=1":                  "auth password=****&x=1",
		`{"password": "hunter2", "user": "bob"}`:     `{"password": "****", "user": "bob"}`,
		"error dialing mongodb://u:p@10.0.0.1:27017": "error dialing mongodb://u:****@10.0.0.1:27017",
		"postgres://app:s3cret@db:5432/shop":         "postgres://app:****@db:5432/shop",
		"dsn app:s3cret@tcp(db:3306)/shop":           "dsn app:****@tcp(db:3306)/shop",
		"host=db user=app password=s3cret dbname=x":  "host=db user=app password=**** dbname=x",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactingHandler(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	log.With("uri", "mongodb://u:topsecret@h").Info("connect mongodb://u:topsecret@h",
		"password", "topsecret",
		"err", errors.New("auth failed for mongodb://u:topsecret@h"),
		slog.Group("g", "inner", "mongodb://u:topsecret@h"),
	)
	out := buf.String()
	if strings.Contains(out, "topsecret") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "u:****@h") {
		t.Fatalf("expected redacted uri in %s", out)
	}
}
