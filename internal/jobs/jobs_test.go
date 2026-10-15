package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"mango3s/internal/apperr"
)

func newManager(n int) *Manager {
	m := NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), n)
	m.SetThrottle(0)
	return m
}

func wait(t *testing.T, m *Manager, id string, want State) Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if j, _ := m.Get(id); j.State == want {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	j, _ := m.Get(id)
	t.Fatalf("job %s: state %s, want %s (%s)", id, j.State, want, j.Error)
	return j
}

func TestJobSucceedsWithProgress(t *testing.T) {
	m := newManager(2)
	var mu sync.Mutex
	var events []Job
	m.SetEmitter(func(j Job) {
		mu.Lock()
		events = append(events, j)
		mu.Unlock()
	})
	j := m.Start("export", "Export x", func(ctx context.Context, p *Progress) (string, error) {
		p.SetTotal(3, "documents")
		for i := 0; i < 3; i++ {
			p.Add(1)
		}
		p.Message("almost done")
		return "3 documents", nil
	})
	done := wait(t, m, j.ID, Succeeded)
	if done.Done != 3 || done.Total != 3 || done.Result != "3 documents" || done.Message != "almost done" {
		t.Fatalf("final: %+v", done)
	}
	if done.StartedAt.IsZero() || done.FinishedAt.IsZero() {
		t.Fatal("timestamps missing")
	}
	mu.Lock()
	defer mu.Unlock()
	if events[0].State != Queued || events[len(events)-1].State != Succeeded {
		t.Fatalf("events: first %s last %s", events[0].State, events[len(events)-1].State)
	}
}

func TestJobFailureAndPanic(t *testing.T) {
	m := newManager(2)
	j := m.Start("import", "bad", func(ctx context.Context, p *Progress) (string, error) {
		return "", apperr.Invalid("line 3: not JSON")
	})
	if got := wait(t, m, j.ID, Failed); got.Error != "line 3: not JSON" {
		t.Fatalf("error: %q", got.Error)
	}
	j = m.Start("import", "panic", func(ctx context.Context, p *Progress) (string, error) {
		var mp map[string]int
		mp["x"] = 1
		return "", nil
	})
	if got := wait(t, m, j.ID, Failed); got.Error == "" {
		t.Fatal("panic not reported")
	}
}

func TestCancelRunningAndQueued(t *testing.T) {
	m := newManager(1)
	started := make(chan struct{})
	block := func(ctx context.Context, p *Progress) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}
	running := m.Start("export", "long", block)
	<-started
	queued := m.Start("export", "waiting", func(ctx context.Context, p *Progress) (string, error) {
		t.Error("queued job must not run after cancel")
		return "", nil
	})
	if j, _ := m.Get(queued.ID); j.State != Queued {
		t.Fatalf("second job should be queued, got %s", j.State)
	}
	if !m.Cancel(queued.ID) || !m.Cancel(running.ID) {
		t.Fatal("cancel returned false")
	}
	wait(t, m, running.ID, Canceled)
	wait(t, m, queued.ID, Canceled)
	if m.Cancel(running.ID) {
		t.Fatal("cancel of finished job returned true")
	}
}

func TestIgnoredCancellationStillCancels(t *testing.T) {
	m := newManager(1)
	started := make(chan struct{})
	j := m.Start("x", "x", func(ctx context.Context, p *Progress) (string, error) {
		close(started)
		<-ctx.Done()
		return "partial", nil // job returns nil despite cancel
	})
	<-started
	m.Cancel(j.ID)
	wait(t, m, j.ID, Canceled)
}

func TestListPruneShutdown(t *testing.T) {
	m := newManager(4)
	m.keep = 3
	for i := 0; i < 6; i++ {
		j := m.Start("k", "t", func(ctx context.Context, p *Progress) (string, error) { return "", nil })
		wait(t, m, j.ID, Succeeded)
		time.Sleep(time.Millisecond)
	}
	m.Start("k", "t", func(ctx context.Context, p *Progress) (string, error) { return "", nil })
	if n := len(m.List()); n > 4 {
		t.Fatalf("pruning kept %d jobs", n)
	}
	list := m.List()
	for i := 1; i < len(list); i++ {
		if list[i].CreatedAt.After(list[i-1].CreatedAt) {
			t.Fatal("list not newest-first")
		}
	}
	blocked := m.Start("k", "block", func(ctx context.Context, p *Progress) (string, error) {
		<-ctx.Done()
		return "", errors.New("stopped")
	})
	m.Shutdown(2 * time.Second)
	if j, _ := m.Get(blocked.ID); !j.State.Finished() {
		t.Fatalf("shutdown left job %s", j.State)
	}
	m.ClearFinished()
	if len(m.List()) != 0 {
		t.Fatal("ClearFinished kept jobs")
	}
}

func TestThrottle(t *testing.T) {
	m := newManager(1)
	m.SetThrottle(time.Hour)
	var mu sync.Mutex
	n := 0
	m.SetEmitter(func(Job) { mu.Lock(); n++; mu.Unlock() })
	j := m.Start("k", "t", func(ctx context.Context, p *Progress) (string, error) {
		for i := 0; i < 1000; i++ {
			p.Add(1)
		}
		return "", nil
	})
	got := wait(t, m, j.ID, Succeeded)
	if got.Done != 1000 {
		t.Fatalf("progress lost: %d", got.Done)
	}
	mu.Lock()
	defer mu.Unlock()
	if n > 5 {
		t.Fatalf("throttle let %d events through", n)
	}
}
