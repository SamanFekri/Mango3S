//go:build integration

package mongox

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"mango3s/internal/config"
)

func TestStepsAgainstRealServer(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	rep, got := record()
	res, err := m.TestWithLog(context.Background(), config.Profile{Name: "it", URI: uri + "/?directConnection=true", ConnectTimeoutSeconds: 5}, Credentials{}, rep)
	if err != nil {
		t.Fatal(err)
	}
	steps := got()
	if states(steps) != "settings=ok reach=ok auth=ok info=ok" || res.ServerVersion == "" {
		t.Fatalf("steps: %s", states(steps))
	}
	if !strings.Contains(steps[3].Detail, "MongoDB "+res.ServerVersion) || !strings.Contains(steps[1].Detail, "✓") {
		t.Fatalf("details: %+v", steps)
	}
	// TLS on against a server without TLS: the TLS step explains it.
	rep, got = record()
	_, err = m.TestWithLog(context.Background(), config.Profile{Name: "it", URI: uri, ConnectTimeoutSeconds: 5, TLS: config.TLSConfig{Enabled: true}}, Credentials{}, rep)
	if err == nil {
		t.Fatal("expected a TLS failure")
	}
	steps = got()
	if states(steps) != "settings=ok reach=ok tls=failed auth=skipped info=skipped" {
		t.Fatalf("tls steps: %s", states(steps))
	}
	if steps[2].Code != "TLS_ERROR" {
		t.Fatalf("tls step: %+v", steps[2])
	}
}
