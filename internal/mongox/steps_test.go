package mongox

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mango3s/internal/config"
)

// record collects the final state of each step in order of first appearance.
func record() (Reporter, func() []Step) {
	order := []string{}
	last := map[string]Step{}
	return func(s Step) {
			if _, ok := last[s.ID]; !ok {
				order = append(order, s.ID)
			}
			last[s.ID] = s
		}, func() []Step {
			out := make([]Step, len(order))
			for i, id := range order {
				out[i] = last[id]
			}
			return out
		}
}

func states(steps []Step) string {
	var b []string
	for _, s := range steps {
		b = append(b, s.ID+"="+s.State)
	}
	return strings.Join(b, " ")
}

func TestStepsShowWhereItFails(t *testing.T) {
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	port := freePort(t)
	rep, got := record()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := m.TestWithLog(ctx, config.Profile{Name: "x", URI: fmt.Sprintf("mongodb://127.0.0.1:%d", port), Username: "app", ConnectTimeoutSeconds: 2}, Credentials{Password: "s3cret"}, rep)
	if err == nil {
		t.Fatal("expected a failure")
	}
	steps := got()
	if states(steps) != "settings=ok reach=failed auth=skipped info=skipped" {
		t.Fatalf("steps: %s", states(steps))
	}
	reach := steps[1]
	if !strings.Contains(reach.Error, "No server could be reached") || !strings.Contains(reach.Detail, "connection refused") {
		t.Fatalf("reach step should say which host failed and why: %+v", reach)
	}
	if !strings.Contains(steps[2].Label, "sign in as app (SCRAM)") {
		t.Fatalf("auth label: %q", steps[2].Label)
	}
	for _, s := range steps {
		if strings.Contains(s.Detail+s.Error+s.Label, "s3cret") {
			t.Fatal("a secret leaked into the connection log")
		}
	}
}

func TestStepsThroughSSHTunnel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := newSSHServer(t)
	p := sshProfile(t, s)
	p.URI = fmt.Sprintf("mongodb://127.0.0.1:%d/?directConnection=true", freePort(t))
	p.ConnectTimeoutSeconds = 2
	rep, got := record()
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, _ = m.TestWithLog(context.Background(), p, Credentials{SSHPassword: "pw"}, rep)
	steps := got()
	if states(steps) != "settings=ok ssh=ok reach=failed auth=skipped info=skipped" {
		t.Fatalf("steps: %s", states(steps))
	}
	if !strings.Contains(steps[1].Label, "Open SSH tunnel to me@127.0.0.1") || !strings.Contains(steps[1].Detail, "pinned") {
		t.Fatalf("ssh step: %+v", steps[1])
	}
	if !strings.Contains(steps[2].Error, "as seen from the SSH server") {
		t.Fatalf("reach through the tunnel: %+v", steps[2])
	}
	// Wrong SSH password: the SSH step fails and nothing after it runs.
	rep, got = record()
	_, _ = m.TestWithLog(context.Background(), p, Credentials{SSHPassword: "nope"}, rep)
	if states(got()) != "settings=ok ssh=failed reach=skipped auth=skipped info=skipped" {
		t.Fatalf("bad ssh password: %s", states(got()))
	}
}

func TestStepsReportBadSRVName(t *testing.T) {
	rep, got := record()
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := m.TestWithLog(ctx, config.Profile{Name: "x", URI: "mongodb+srv://no-such-cluster.invalid/", ConnectTimeoutSeconds: 2}, Credentials{}, rep)
	if err == nil {
		t.Fatal("expected a failure")
	}
	steps := got()
	if states(steps) != "settings=ok dns=failed reach=skipped tls=skipped auth=skipped info=skipped" {
		t.Fatalf("steps: %s", states(steps))
	}
	if !strings.Contains(steps[1].Error, "_mongodb._tcp.no-such-cluster.invalid") {
		t.Fatalf("dns step: %+v", steps[1])
	}
}
