package mongox

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

func TestBuildClientOptions(t *testing.T) {
	p := config.Profile{
		URI:                   "mongodb://h1:27017/?authSource=fromuri&appName=custom",
		Username:              "bob",
		ConnectTimeoutSeconds: 3,
	}
	o, err := BuildClientOptions(p, "pw")
	if err != nil {
		t.Fatal(err)
	}
	if o.Auth == nil || o.Auth.Username != "bob" || o.Auth.Password != "pw" || !o.Auth.PasswordSet {
		t.Fatalf("credential not applied: %+v", o.Auth)
	}
	if o.Auth.AuthSource != "fromuri" {
		t.Fatalf("auth source from URI lost: %q", o.Auth.AuthSource)
	}
	if *o.AppName != "custom" {
		t.Fatalf("app name from URI overridden: %q", *o.AppName)
	}
	if *o.ConnectTimeout != 3*time.Second || *o.ServerSelectionTimeout != 3*time.Second {
		t.Fatal("timeouts not applied")
	}

	p.AuthSource, p.AuthMechanism = "admin", "SCRAM-SHA-256"
	o, _ = BuildClientOptions(p, "")
	if o.Auth.AuthSource != "admin" || o.Auth.AuthMechanism != "SCRAM-SHA-256" || o.Auth.PasswordSet {
		t.Fatalf("explicit auth fields: %+v", o.Auth)
	}

	o, _ = BuildClientOptions(config.Profile{URI: "mongodb://h"}, "")
	if o.Auth != nil || *o.AppName != "Mango3S" {
		t.Fatal("anonymous profile should have no credential and default app name")
	}

	if _, err := BuildClientOptions(config.Profile{URI: "mongodb://h/?connectTimeoutMS=abc"}, ""); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("invalid URI: %v", err)
	}
}

func TestUnreachableServerIsClassified(t *testing.T) {
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := config.Profile{ID: "x", Name: "dead", URI: "mongodb://127.0.0.1:1/?directConnection=true", ConnectTimeoutSeconds: 1}
	_, err := m.Test(context.Background(), p, "")
	if !apperr.Is(err, apperr.CodeUnreachable) && !apperr.Is(err, apperr.CodeTimeout) {
		t.Fatalf("want UNREACHABLE or TIMEOUT, got %v", err)
	}
	if _, err := m.Connect(context.Background(), p, ""); err == nil {
		t.Fatal("connect to dead server succeeded")
	}
	if _, err := m.Session("x"); !apperr.Is(err, apperr.CodeNotConnected) {
		t.Fatalf("want NOT_CONNECTED, got %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatal("failed connect left a session")
	}
	if err := m.Disconnect(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
}

func TestRequireWritable(t *testing.T) {
	if err := (&Session{Name: "ro", ReadOnly: true}).RequireWritable(); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("got %v", err)
	}
	if err := (&Session{}).RequireWritable(); err != nil {
		t.Fatal(err)
	}
}
