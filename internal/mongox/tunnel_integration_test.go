//go:build integration

package mongox

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"mango3s/internal/config"
)

// TestMongoThroughSSHTunnel connects to the real test server through an
// in-process SSH server, the way a bastion host would be used.
func TestMongoThroughSSHTunnel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	// The driver must dial the MongoDB address via the tunnel only.
	target := strings.TrimSuffix(strings.TrimPrefix(uri, "mongodb://"), "/")
	if i := strings.IndexAny(target, "/?"); i >= 0 {
		target = target[:i]
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		t.Skip("MONGO_URI must be a single host:port for this test")
	}
	s := newSSHServer(t)
	p := sshProfile(t, s)
	p.URI = "mongodb://" + target + "/?directConnection=true"
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := m.Test(ctx, p, Credentials{SSHPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ServerVersion == "" || !strings.Contains(res.Via, "SSH") || s.forwards.Load() == 0 {
		t.Fatalf("expected a tunneled connection: %+v forwards=%d", res, s.forwards.Load())
	}
	p.ID, p.Name = "tun", "tunnel"
	sess, err := m.Connect(ctx, p, Credentials{SSHPassword: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Client.ListDatabaseNames(ctx, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	_ = m.Disconnect(ctx, "tun")
	var _ = config.SSHAuthPassword
}
