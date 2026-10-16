package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/jobs"
)

func TestCronParseAndNext(t *testing.T) {
	loc := time.UTC
	at := func(s string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	cases := []struct {
		expr, from, want string
	}{
		{"*/15 * * * *", "2026-01-01 10:07", "2026-01-01 10:15"},
		{"0 2 * * *", "2026-01-01 10:07", "2026-01-02 02:00"},
		{"30 9 * * 1-5", "2026-01-02 10:00", "2026-01-05 09:30"}, // Fri -> Mon
		{"0 0 1 * *", "2026-01-15 00:00", "2026-02-01 00:00"},
		{"0 12 29 2 *", "2026-03-01 00:00", "2028-02-29 12:00"}, // next leap day
		{"5,35 */6 * * *", "2026-01-01 06:06", "2026-01-01 06:35"},
		{"0 0 13 * 5", "2026-01-01 00:00", "2026-01-02 00:00"}, // dom OR dow: Friday Jan 2
		{"0 0 * * 7", "2026-01-01 00:00", "2026-01-04 00:00"},  // 7 = Sunday
		{"10/20 * * * *", "2026-01-01 00:00", "2026-01-01 00:10"},
	}
	for _, c := range cases {
		cr, err := ParseCron(c.expr)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		if got := cr.Next(at(c.from)); !got.Equal(at(c.want)) {
			t.Errorf("%s from %s: got %s want %s", c.expr, c.from, got.Format("2006-01-02 15:04"), c.want)
		}
	}
	for _, bad := range []string{"* * * *", "60 * * * *", "* 24 * * *", "* * 0 * *", "*/0 * * * *", "a * * * *", "5-1 * * * *"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	never, _ := ParseCron("0 0 30 2 *")
	if !never.Next(at("2026-01-01 00:00")).IsZero() {
		t.Error("Feb 30 must never match")
	}
}

type fixture struct {
	s     *Scheduler
	jm    *jobs.Manager
	now   time.Time
	mu    sync.Mutex
	calls atomic.Int32
	fail  atomic.Int32 // number of upcoming attempts that fail
	slept []time.Duration
}

func newFixture(t *testing.T, dir string) *fixture {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f := &fixture{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}
	f.jm = jobs.NewManager(log, 4)
	f.s = New(log, f.jm, dir)
	f.s.SetClock(func() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }, func(ctx context.Context, d time.Duration) error {
		f.mu.Lock()
		f.slept = append(f.slept, d)
		f.mu.Unlock()
		return ctx.Err()
	})
	f.s.Register(TaskType{
		ID: "noop",
		Validate: func(p json.RawMessage) error {
			var v struct{ Bad bool }
			_ = json.Unmarshal(p, &v)
			if v.Bad {
				return apperr.Invalid("bad params")
			}
			return nil
		},
		Run: func(ctx context.Context, p json.RawMessage, pr *jobs.Progress) (string, error) {
			f.calls.Add(1)
			if f.fail.Load() > 0 {
				f.fail.Add(-1)
				return "", errors.New("transient failure")
			}
			return "ok", nil
		},
	})
	return f
}

func (f *fixture) advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

func waitRuns(t *testing.T, s *Scheduler, taskID string, n int) []Run {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := s.Runs(taskID, 0)
		done := 0
		for _, r := range runs {
			if r.State != RunRunning {
				done++
			}
		}
		if done >= n {
			return runs
		}
		time.Sleep(5 * time.Millisecond)
	}
	runs, _ := s.Runs(taskID, 0)
	t.Fatalf("expected %d finished runs, have %+v", n, runs)
	return nil
}

func TestApprovalGatesExecution(t *testing.T) {
	f := newFixture(t, t.TempDir())
	task, err := f.s.Save(Task{Name: "export nightly", Type: "noop", Params: json.RawMessage(`{"a":1}`), Enabled: true,
		Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}, ApprovedHash: "forged"})
	if err != nil {
		t.Fatal(err)
	}
	if task.Approved() {
		t.Fatal("Save must not accept an approval from the caller")
	}
	f.advance(6 * time.Minute)
	f.s.Tick()
	if _, err := f.s.RunNow(task.ID); err == nil {
		t.Fatal("unapproved task ran")
	}
	if f.calls.Load() != 0 {
		t.Fatal("unapproved task executed")
	}

	task, _ = f.s.Approve(task.ID)
	f.s.Tick()
	runs := waitRuns(t, f.s, task.ID, 1)
	if runs[0].State != RunSucceeded || runs[0].Result != "ok" || runs[0].Trigger != "schedule" || runs[0].JobID == "" {
		t.Fatalf("run: %+v", runs[0])
	}
	got, _ := f.s.Get(task.ID)
	if !got.NextRun.After(f.now) {
		t.Fatal("task was not rescheduled")
	}

	// Editing the params revokes approval; editing only the name keeps it.
	got.Name = "renamed"
	got, _ = f.s.Save(got)
	if !got.Approved() {
		t.Fatal("renaming revoked approval")
	}
	got.Params = json.RawMessage(`{"a":2}`)
	got, _ = f.s.Save(got)
	if got.Approved() {
		t.Fatal("changed params kept approval")
	}
	// Key order does not matter for the approval hash.
	a := Task{Type: "noop", Params: json.RawMessage(`{"x":1,"y":2}`)}
	b := Task{Type: "noop", Params: json.RawMessage(`{"y":2,"x":1}`)}
	if ParamsHash(a) != ParamsHash(b) {
		t.Fatal("approval hash depends on key order")
	}
}

func TestValidationAndAllowlist(t *testing.T) {
	f := newFixture(t, t.TempDir())
	bad := []Task{
		{Name: "", Type: "noop", Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}},
		{Name: "x", Type: "dropDatabase", Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}},
		{Name: "x", Type: "noop", Params: json.RawMessage(`{"Bad":true}`), Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}},
		{Name: "x", Type: "noop", Schedule: Schedule{Kind: KindCron, Cron: "61 * * * *"}},
		{Name: "x", Type: "noop", Schedule: Schedule{Kind: KindInterval}},
	}
	for i, tk := range bad {
		if _, err := f.s.Save(tk); err == nil {
			t.Errorf("bad task %d accepted", i)
		}
	}
}

func TestRetryWithBackoff(t *testing.T) {
	f := newFixture(t, t.TempDir())
	task, _ := f.s.Save(Task{Name: "flaky", Type: "noop", Enabled: true, Schedule: Schedule{Kind: KindInterval, EveryMinutes: 60},
		Retry: Retry{MaxAttempts: 3, BackoffSeconds: 10}})
	f.s.Approve(task.ID)
	f.fail.Store(2)
	if _, err := f.s.RunNow(task.ID); err != nil {
		t.Fatal(err)
	}
	r := waitRuns(t, f.s, task.ID, 1)[0]
	if r.State != RunSucceeded || r.Attempts != 3 || len(r.Errors) != 2 {
		t.Fatalf("retry run: %+v", r)
	}
	f.mu.Lock()
	slept := append([]time.Duration{}, f.slept...)
	f.mu.Unlock()
	if len(slept) != 2 || slept[0] != 10*time.Second || slept[1] != 20*time.Second {
		t.Fatalf("backoff: %v", slept)
	}

	f.fail.Store(10)
	f.s.RunNow(task.ID)
	runs := waitRuns(t, f.s, task.ID, 2)
	if runs[0].State != RunFailed || runs[0].Attempts != 3 || len(runs[0].Errors) != 3 {
		t.Fatalf("exhausted retries: %+v", runs[0])
	}
}

func TestRestartPolicies(t *testing.T) {
	dir := t.TempDir()
	f := newFixture(t, dir)
	skip, _ := f.s.Save(Task{Name: "skip", Type: "noop", Enabled: true, Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}})
	once, _ := f.s.Save(Task{Name: "once", Type: "noop", Enabled: true, Schedule: Schedule{Kind: KindInterval, EveryMinutes: 5}, Misfire: MisfireRunOnce})
	f.s.Approve(skip.ID)
	f.s.Approve(once.ID)
	// A run that was in progress when the app "crashed".
	_ = f.s.putRun(Run{ID: "stale", TaskID: skip.ID, State: RunRunning, StartedAt: f.now})

	// Restart two hours later with a new scheduler over the same directory.
	g := newFixture(t, dir)
	g.now = f.now.Add(2 * time.Hour)
	if err := g.s.Recover(); err != nil {
		t.Fatal(err)
	}
	skipRuns := waitRuns(t, g.s, skip.ID, 2)
	states := map[string]bool{}
	for _, r := range skipRuns {
		states[r.State] = true
	}
	if !states[RunInterrupted] || !states[RunMissed] || g.calls.Load() != 1 {
		t.Fatalf("skip policy: %+v calls=%d", skipRuns, g.calls.Load())
	}
	onceRuns := waitRuns(t, g.s, once.ID, 1)
	if onceRuns[0].Trigger != "misfire" || onceRuns[0].State != RunSucceeded {
		t.Fatalf("runOnce policy: %+v", onceRuns)
	}
	for _, id := range []string{skip.ID, once.ID} {
		tk, _ := g.s.Get(id)
		if !tk.NextRun.After(g.now) {
			t.Fatalf("%s not rescheduled into the future", tk.Name)
		}
	}
}

func TestNoOverlap(t *testing.T) {
	f := newFixture(t, t.TempDir())
	release := make(chan struct{})
	f.s.Register(TaskType{ID: "slow", Run: func(ctx context.Context, _ json.RawMessage, _ *jobs.Progress) (string, error) {
		<-release
		return "done", nil
	}})
	task, _ := f.s.Save(Task{Name: "slow", Type: "slow", Enabled: true, Schedule: Schedule{Kind: KindInterval, EveryMinutes: 1}})
	f.s.Approve(task.ID)
	if _, err := f.s.RunNow(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.RunNow(task.ID); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("overlapping run allowed: %v", err)
	}
	close(release)
	runs := waitRuns(t, f.s, task.ID, 2)
	states := map[string]int{}
	for _, r := range runs {
		states[r.State]++
	}
	if states[RunSkipped] != 1 || states[RunSucceeded] != 1 {
		t.Fatalf("states: %+v", states)
	}
}
