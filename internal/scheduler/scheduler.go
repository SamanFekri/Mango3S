// Package scheduler runs approved background tasks (exports, comparisons,
// syncs, masked copies) on interval or cron schedules while the app is open.
//
// Policies:
//   - Approval: a task runs only after the user approves it; approval covers
//     the task's type and parameters, so editing them revokes it.
//   - Overlap: a run is skipped (and recorded) while the previous run of the
//     same task is still going.
//   - Failure: a failed attempt is retried up to MaxAttempts with exponential
//     backoff; every attempt's error is recorded.
//   - Restart: runs that were in progress when the app stopped are recorded
//     as interrupted. Runs missed while the app was closed are either skipped
//     (default; one "missed" record) or run once at startup (misfire runOnce).
package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/jobs"
)

// Schedule kinds.
const (
	KindInterval = "interval"
	KindCron     = "cron"
)

// Schedule says when a task runs.
type Schedule struct {
	Kind         string `json:"kind"`
	EveryMinutes int    `json:"everyMinutes"`
	Cron         string `json:"cron"`
}

// Next returns the next run strictly after t.
func (s Schedule) Next(t time.Time) (time.Time, error) {
	switch s.Kind {
	case KindInterval:
		if s.EveryMinutes < 1 || s.EveryMinutes > 60*24*31 {
			return time.Time{}, apperr.Invalid("Interval must be between 1 minute and 31 days.")
		}
		return t.Add(time.Duration(s.EveryMinutes) * time.Minute).Truncate(time.Second), nil
	case KindCron:
		c, err := ParseCron(s.Cron)
		if err != nil {
			return time.Time{}, apperr.Invalid("Cron: %v", err)
		}
		n := c.Next(t)
		if n.IsZero() {
			return n, apperr.Invalid("The cron expression never matches.")
		}
		return n, nil
	}
	return time.Time{}, apperr.Invalid("Schedule kind must be interval or cron.")
}

// Retry configures failure handling.
type Retry struct {
	MaxAttempts    int `json:"maxAttempts"`    // 1-10
	BackoffSeconds int `json:"backoffSeconds"` // first delay; doubles per attempt
}

// Misfire policies.
const (
	MisfireSkip    = "skip"
	MisfireRunOnce = "runOnce"
)

// Task is a scheduled job definition.
type Task struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Params       json.RawMessage `json:"params"`
	Schedule     Schedule        `json:"schedule"`
	Enabled      bool            `json:"enabled"`
	Retry        Retry           `json:"retry"`
	Misfire      string          `json:"misfire"`
	ApprovedHash string          `json:"approvedHash"`
	ApprovedAt   *time.Time      `json:"approvedAt,omitempty"`
	NextRun      time.Time       `json:"nextRun"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// ParamsHash identifies what was approved.
func ParamsHash(t Task) string {
	var buf strings.Builder
	buf.WriteString(t.Type)
	buf.WriteByte(0)
	var v any
	if err := json.Unmarshal(t.Params, &v); err == nil {
		b, _ := json.Marshal(v) // canonical: maps marshal with sorted keys
		buf.Write(b)
	} else {
		buf.Write(t.Params)
	}
	sum := sha256.Sum256([]byte(buf.String()))
	return hex.EncodeToString(sum[:12])
}

// Approved reports whether the current type and params were approved.
func (t Task) Approved() bool { return t.ApprovedHash != "" && t.ApprovedHash == ParamsHash(t) }

// Run states.
const (
	RunRunning     = "running"
	RunSucceeded   = "succeeded"
	RunFailed      = "failed"
	RunSkipped     = "skipped"
	RunMissed      = "missed"
	RunInterrupted = "interrupted"
	RunCanceled    = "canceled"
)

// Run is one execution (with all its attempts).
type Run struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"taskId"`
	TaskName   string    `json:"taskName"`
	Trigger    string    `json:"trigger"` // schedule | manual | misfire
	State      string    `json:"state"`
	Attempts   int       `json:"attempts"`
	Errors     []string  `json:"errors"`
	Result     string    `json:"result,omitempty"`
	JobID      string    `json:"jobId,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

// TaskType is a kind of task the scheduler may run. Only registered types
// can be scheduled: that registry is the allowlist of approved job kinds.
type TaskType struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// Validate checks params when the task is saved.
	Validate func(params json.RawMessage) error `json:"-"`
	// Run executes one attempt.
	Run func(ctx context.Context, params json.RawMessage, p *jobs.Progress) (string, error) `json:"-"`
}

// Scheduler owns tasks and their runs.
type Scheduler struct {
	log   *slog.Logger
	jobs  *jobs.Manager
	dir   string
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) error

	mu      sync.Mutex
	types   map[string]TaskType
	running map[string]bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// New returns a scheduler persisting to dir/tasks.json and dir/task_runs.json.
func New(log *slog.Logger, jm *jobs.Manager, dir string) *Scheduler {
	return &Scheduler{
		log: log, jobs: jm, dir: dir, now: time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		types:   map[string]TaskType{},
		running: map[string]bool{},
	}
}

// SetClock replaces time.Now and the backoff sleep (tests).
func (s *Scheduler) SetClock(now func() time.Time, sleep func(context.Context, time.Duration) error) {
	s.now, s.sleep = now, sleep
}

// Register adds an allowed task type.
func (s *Scheduler) Register(t TaskType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.types[t.ID] = t
}

// Types lists the allowed task types.
func (s *Scheduler) Types() []TaskType {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TaskType, 0, len(s.types))
	for _, t := range s.types {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- persistence ----

type tasksFile struct {
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

type runsFile struct {
	Version int   `json:"version"`
	Runs    []Run `json:"runs"`
}

// MaxRuns bounds the persisted run history.
const MaxRuns = 500

func (s *Scheduler) loadTasks() ([]Task, error) {
	var f tasksFile
	err := config.ReadJSON(filepath.Join(s.dir, "tasks.json"), &f)
	return f.Tasks, err
}

func (s *Scheduler) saveTasks(ts []Task) error {
	return config.WriteJSON(filepath.Join(s.dir, "tasks.json"), tasksFile{Version: 1, Tasks: ts})
}

func (s *Scheduler) loadRuns() ([]Run, error) {
	var f runsFile
	err := config.ReadJSON(filepath.Join(s.dir, "task_runs.json"), &f)
	return f.Runs, err
}

func (s *Scheduler) putRun(r Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	runs, err := s.loadRuns()
	if err != nil {
		return err
	}
	found := false
	for i := range runs {
		if runs[i].ID == r.ID {
			runs[i] = r
			found = true
		}
	}
	if !found {
		runs = append(runs, r)
	}
	if len(runs) > MaxRuns {
		runs = runs[len(runs)-MaxRuns:]
	}
	return config.WriteJSON(filepath.Join(s.dir, "task_runs.json"), runsFile{Version: 1, Runs: runs})
}

func (s *Scheduler) updateTask(id string, f func(*Task) error) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.loadTasks()
	if err != nil {
		return Task{}, err
	}
	for i := range ts {
		if ts[i].ID == id {
			if err := f(&ts[i]); err != nil {
				return Task{}, err
			}
			ts[i].UpdatedAt = s.now().UTC()
			return ts[i], s.saveTasks(ts)
		}
	}
	return Task{}, apperr.New(apperr.CodeNotFound, "Task not found.")
}

// ---- CRUD ----

// List returns tasks by name.
func (s *Scheduler) List() ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.loadTasks()
	if err != nil {
		return nil, err
	}
	out := append([]Task{}, ts...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

// Get returns one task.
func (s *Scheduler) Get(id string) (Task, error) {
	ts, err := s.List()
	if err != nil {
		return Task{}, err
	}
	for _, t := range ts {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, apperr.New(apperr.CodeNotFound, "Task not found.")
}

// Save creates or updates a task. Approval is kept only if type and params
// are unchanged (the hash still matches).
func (s *Scheduler) Save(t Task) (Task, error) {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return Task{}, apperr.Invalid("A task name is required.")
	}
	s.mu.Lock()
	tt, ok := s.types[t.Type]
	s.mu.Unlock()
	if !ok {
		return Task{}, apperr.Invalid("Task type %q is not allowed.", t.Type)
	}
	if len(t.Params) == 0 {
		t.Params = json.RawMessage("{}")
	}
	if tt.Validate != nil {
		if err := tt.Validate(t.Params); err != nil {
			return Task{}, err
		}
	}
	next, err := t.Schedule.Next(s.now())
	if err != nil {
		return Task{}, err
	}
	t.NextRun = next
	if t.Retry.MaxAttempts < 1 {
		t.Retry.MaxAttempts = 1
	}
	if t.Retry.MaxAttempts > 10 {
		t.Retry.MaxAttempts = 10
	}
	if t.Retry.BackoffSeconds < 0 || t.Retry.BackoffSeconds > 3600 {
		t.Retry.BackoffSeconds = 60
	}
	if t.Misfire != MisfireRunOnce {
		t.Misfire = MisfireSkip
	}
	now := s.now().UTC()
	t.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.loadTasks()
	if err != nil {
		return Task{}, err
	}
	if t.ID == "" {
		t.ID = config.NewID()
		t.CreatedAt = now
		t.ApprovedHash, t.ApprovedAt = "", nil // new tasks always start unapproved
		ts = append(ts, t)
	} else {
		found := false
		for i := range ts {
			if ts[i].ID == t.ID {
				t.CreatedAt = ts[i].CreatedAt
				t.ApprovedHash, t.ApprovedAt = ts[i].ApprovedHash, ts[i].ApprovedAt // the UI cannot approve via Save
				ts[i] = t
				found = true
			}
		}
		if !found {
			return Task{}, apperr.New(apperr.CodeNotFound, "Task not found.")
		}
	}
	return t, s.saveTasks(ts)
}

// Delete removes a task (its run history is kept).
func (s *Scheduler) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.loadTasks()
	if err != nil {
		return err
	}
	out := ts[:0]
	for _, t := range ts {
		if t.ID != id {
			out = append(out, t)
		}
	}
	return s.saveTasks(out)
}

// Approve records the user's approval of the task's current type and params.
func (s *Scheduler) Approve(id string) (Task, error) {
	return s.updateTask(id, func(t *Task) error {
		now := s.now().UTC()
		t.ApprovedHash, t.ApprovedAt = ParamsHash(*t), &now
		return nil
	})
}

// Revoke withdraws approval.
func (s *Scheduler) Revoke(id string) (Task, error) {
	return s.updateTask(id, func(t *Task) error {
		t.ApprovedHash, t.ApprovedAt = "", nil
		return nil
	})
}

// SetEnabled turns a task on or off.
func (s *Scheduler) SetEnabled(id string, enabled bool) (Task, error) {
	return s.updateTask(id, func(t *Task) error {
		t.Enabled = enabled
		if enabled {
			next, err := t.Schedule.Next(s.now())
			if err != nil {
				return err
			}
			t.NextRun = next
		}
		return nil
	})
}

// Runs returns run history, newest first, optionally for one task.
func (s *Scheduler) Runs(taskID string, limit int) ([]Run, error) {
	s.mu.Lock()
	runs, err := s.loadRuns()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := []Run{}
	for i := len(runs) - 1; i >= 0; i-- {
		if taskID == "" || runs[i].TaskID == taskID {
			out = append(out, runs[i])
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ---- execution ----

// Start recovers state after a restart and begins ticking.
func (s *Scheduler) Start(ctx context.Context, tick time.Duration) error {
	if err := s.Recover(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.Tick()
			}
		}
	}()
	return nil
}

// Stop stops ticking (running jobs are stopped by the jobs manager).
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// Recover applies the restart policies: in-progress runs become
// interrupted, and missed runs are skipped or run once.
func (s *Scheduler) Recover() error {
	runs, err := s.loadRuns()
	if err != nil {
		return err
	}
	for _, r := range runs {
		if r.State == RunRunning {
			r.State = RunInterrupted
			r.FinishedAt = s.now().UTC()
			r.Errors = append(r.Errors, "The app stopped while this run was in progress.")
			if err := s.putRun(r); err != nil {
				return err
			}
		}
	}
	tasks, err := s.List()
	if err != nil {
		return err
	}
	now := s.now()
	for _, t := range tasks {
		if !t.Enabled || !t.Approved() || t.NextRun.IsZero() || t.NextRun.After(now) {
			continue
		}
		if t.Misfire == MisfireRunOnce {
			s.trigger(t, "misfire")
			continue
		}
		_ = s.putRun(Run{ID: config.NewID(), TaskID: t.ID, TaskName: t.Name, Trigger: "schedule", State: RunMissed,
			StartedAt: t.NextRun, FinishedAt: now.UTC(), Errors: []string{"Missed while the app was closed (misfire policy: skip)."}})
		if _, err := s.reschedule(t.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) reschedule(id string) (Task, error) {
	return s.updateTask(id, func(t *Task) error {
		next, err := t.Schedule.Next(s.now())
		if err != nil {
			return err
		}
		t.NextRun = next
		return nil
	})
}

// Tick triggers every enabled, approved task that is due.
func (s *Scheduler) Tick() {
	tasks, err := s.List()
	if err != nil {
		s.log.Warn("scheduler: cannot load tasks", "err", err)
		return
	}
	now := s.now()
	for _, t := range tasks {
		if t.Enabled && t.Approved() && !t.NextRun.IsZero() && !t.NextRun.After(now) {
			s.trigger(t, "schedule")
		}
	}
}

// RunNow triggers an approved task immediately.
func (s *Scheduler) RunNow(id string) (Run, error) {
	t, err := s.Get(id)
	if err != nil {
		return Run{}, err
	}
	if !t.Approved() {
		return Run{}, apperr.New(apperr.CodeReadOnly, "Approve the task before running it.")
	}
	r, ok := s.trigger(t, "manual")
	if !ok {
		return r, apperr.New(apperr.CodeConflict, "The task is already running.")
	}
	return r, nil
}

// trigger starts a run unless one is in progress; it reschedules the task.
func (s *Scheduler) trigger(t Task, trigger string) (Run, bool) {
	if trigger != "manual" {
		if _, err := s.reschedule(t.ID); err != nil {
			s.log.Warn("scheduler: reschedule failed", "task", t.Name, "err", err)
		}
	}
	run := Run{ID: config.NewID(), TaskID: t.ID, TaskName: t.Name, Trigger: trigger, StartedAt: s.now().UTC(), Errors: []string{}}
	s.mu.Lock()
	tt, typeOK := s.types[t.Type]
	busy := s.running[t.ID]
	if !busy && typeOK {
		s.running[t.ID] = true
	}
	s.mu.Unlock()
	switch {
	case !typeOK:
		run.State, run.FinishedAt = RunFailed, run.StartedAt
		run.Errors = append(run.Errors, fmt.Sprintf("Task type %q is no longer allowed.", t.Type))
		_ = s.putRun(run)
		return run, false
	case busy:
		run.State, run.FinishedAt = RunSkipped, run.StartedAt
		run.Errors = append(run.Errors, "Skipped: the previous run is still in progress.")
		_ = s.putRun(run)
		return run, false
	}
	run.State = RunRunning
	_ = s.putRun(run)
	// The job waits until its ID is recorded, so the run record is never
	// written by two goroutines at once.
	ready := make(chan struct{})
	job := s.jobs.Start("task", "Task: "+t.Name, func(ctx context.Context, p *jobs.Progress) (string, error) {
		<-ready
		defer func() {
			s.mu.Lock()
			delete(s.running, t.ID)
			s.mu.Unlock()
		}()
		result, err := s.attempts(ctx, t, tt, &run, p)
		run.FinishedAt = s.now().UTC()
		switch {
		case err == nil:
			run.State, run.Result = RunSucceeded, result
		case ctx.Err() != nil:
			run.State = RunCanceled
		default:
			run.State = RunFailed
		}
		if perr := s.putRun(run); perr != nil {
			s.log.Warn("scheduler: cannot record run", "err", perr)
		}
		s.log.Info("task finished", "task", t.Name, "state", run.State, "attempts", run.Attempts)
		return result, err
	})
	run.JobID = job.ID
	_ = s.putRun(run)
	started := run
	close(ready)
	return started, true
}

func (s *Scheduler) attempts(ctx context.Context, t Task, tt TaskType, run *Run, p *jobs.Progress) (string, error) {
	backoff := time.Duration(t.Retry.BackoffSeconds) * time.Second
	var lastErr error
	for attempt := 1; attempt <= t.Retry.MaxAttempts; attempt++ {
		run.Attempts = attempt
		result, err := tt.Run(ctx, t.Params, p)
		if err == nil {
			return result, nil
		}
		lastErr = err
		msg := apperr.From(err).Message
		run.Errors = append(run.Errors, fmt.Sprintf("attempt %d: %s", attempt, msg))
		_ = s.putRun(*run)
		if ctx.Err() != nil || attempt == t.Retry.MaxAttempts || apperr.Is(err, apperr.CodeInvalidInput) || apperr.Is(err, apperr.CodeReadOnly) {
			break // retrying cannot fix invalid input or a permission problem
		}
		if p != nil {
			p.Message("Attempt %d failed; retrying in %s", attempt, backoff)
		}
		if err := s.sleep(ctx, backoff); err != nil {
			return "", err
		}
		backoff *= 2
	}
	return "", lastErr
}
