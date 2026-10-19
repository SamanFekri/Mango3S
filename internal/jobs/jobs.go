// Package jobs runs long operations (imports, exports) in the background.
// Each job gets a cancellable context and a Progress handle; state changes are
// pushed to an emitter (the UI) with progress updates throttled so a fast job
// cannot flood the frontend.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// State of a job.
type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

// Finished reports whether the state is terminal.
func (s State) Finished() bool { return s == Succeeded || s == Failed || s == Canceled }

// Job is a snapshot of a background job.
type Job struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	State   State  `json:"state"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"` // 0 when unknown
	Unit    string `json:"unit"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Result  string `json:"result,omitempty"`
	// Log is the job's step-by-step log (the last MaxLogLines lines).
	Log        []string  `json:"log,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

// clone copies a job so a snapshot never shares the log with the running job.
func (j Job) clone() Job {
	j.Log = append([]string(nil), j.Log...)
	return j
}

// Func is the work of a job. It returns a one-line result summary.
type Func func(ctx context.Context, p *Progress) (string, error)

// Progress lets a job report how far it has come. All methods are safe to
// call from the job goroutine.
type Progress struct {
	m  *Manager
	id string
}

// ID returns the job's ID, e.g. to store a structured result under it.
func (p *Progress) ID() string { return p.id }

// SetTotal sets the expected amount of work (0 = unknown) and its unit.
func (p *Progress) SetTotal(total int64, unit string) {
	p.m.update(p.id, false, func(j *Job) { j.Total, j.Unit = total, unit })
}

// Add advances progress by n.
func (p *Progress) Add(n int64) { p.m.update(p.id, false, func(j *Job) { j.Done += n }) }

// Set sets absolute progress.
func (p *Progress) Set(n int64) { p.m.update(p.id, false, func(j *Job) { j.Done = n }) }

// MaxLogLines caps a job's log.
const MaxLogLines = 400

// Log appends a timestamped line to the job's log (and shows it as the status).
func (p *Progress) Log(format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	p.m.update(p.id, true, func(j *Job) {
		j.Log = append(j.Log, line)
		if len(j.Log) > MaxLogLines {
			j.Log = append([]string{"…"}, j.Log[len(j.Log)-MaxLogLines+1:]...)
		}
		j.Message = fmt.Sprintf(format, args...)
	})
}

// Message sets a short status line; it is emitted immediately.
func (p *Progress) Message(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.m.update(p.id, true, func(j *Job) { j.Message = msg })
}

type entry struct {
	job      Job
	cancel   context.CancelFunc
	lastEmit time.Time
}

// Manager owns all jobs.
type Manager struct {
	log      *slog.Logger
	mu       sync.Mutex
	jobs     map[string]*entry
	emit     func(Job)
	sem      chan struct{}
	wg       sync.WaitGroup
	keep     int
	throttle time.Duration
	parent   context.Context
	stop     context.CancelFunc
}

// NewManager runs at most maxConcurrent jobs at once; extra jobs wait queued.
func NewManager(log *slog.Logger, maxConcurrent int) *Manager {
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	ctx, stop := context.WithCancel(context.Background())
	return &Manager{
		log:      log,
		jobs:     map[string]*entry{},
		emit:     func(Job) {},
		sem:      make(chan struct{}, maxConcurrent),
		keep:     50,
		throttle: 150 * time.Millisecond,
		parent:   ctx,
		stop:     stop,
	}
}

// SetEmitter installs the function that receives job updates.
func (m *Manager) SetEmitter(f func(Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f == nil {
		f = func(Job) {}
	}
	m.emit = f
}

// SetThrottle changes the minimum interval between progress emissions (tests).
func (m *Manager) SetThrottle(d time.Duration) { m.throttle = d }

// Start queues fn and returns its initial snapshot.
func (m *Manager) Start(kind, title string, fn Func) Job {
	ctx, cancel := context.WithCancel(m.parent)
	e := &entry{
		job:    Job{ID: config.NewID(), Kind: kind, Title: title, State: Queued, CreatedAt: time.Now()},
		cancel: cancel,
	}
	m.mu.Lock()
	m.jobs[e.job.ID] = e
	m.pruneLocked()
	snap := e.job.clone()
	emit := m.emit
	m.mu.Unlock()
	emit(snap)

	m.wg.Add(1)
	go m.run(ctx, e.job.ID, fn)
	return snap
}

func (m *Manager) run(ctx context.Context, id string, fn Func) {
	defer m.wg.Done()
	select {
	case m.sem <- struct{}{}:
		defer func() { <-m.sem }()
	case <-ctx.Done():
		m.finish(id, "", ctx.Err())
		return
	}
	// select picks randomly when a slot frees up as the job is canceled; never start a canceled job.
	if ctx.Err() != nil {
		m.finish(id, "", ctx.Err())
		return
	}
	m.update(id, true, func(j *Job) { j.State, j.StartedAt = Running, time.Now() })
	m.log.Info("job started", "id", id)

	var (
		result string
		err    error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				m.log.Error("job panicked", "id", id, "panic", r, "stack", string(debug.Stack()))
				err = fmt.Errorf("internal error: %v", r)
			}
		}()
		result, err = fn(ctx, &Progress{m: m, id: id})
	}()
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	m.finish(id, result, err)
}

func (m *Manager) finish(id, result string, err error) {
	m.update(id, true, func(j *Job) {
		j.FinishedAt = time.Now()
		j.Result = result
		switch {
		case err == nil:
			j.State = Succeeded
			if j.Total > 0 && j.Done < j.Total && j.Unit == "bytes" {
				j.Done = j.Total
			}
		case errors.Is(err, context.Canceled):
			j.State = Canceled
			j.Error = "Canceled by user."
		default:
			j.State = Failed
			ae := apperr.From(err)
			j.Error = ae.Message
			if ae.Detail != "" && ae.Detail != ae.Message {
				j.Error += " (" + ae.Detail + ")"
			}
		}
	})
	m.mu.Lock()
	if e, ok := m.jobs[id]; ok {
		e.cancel()
		m.log.Info("job finished", "id", id, "kind", e.job.Kind, "state", e.job.State, "done", e.job.Done, "err", e.job.Error)
	}
	m.mu.Unlock()
}

// update mutates a job and emits it if forced or the throttle interval passed.
func (m *Manager) update(id string, force bool, f func(*Job)) {
	m.mu.Lock()
	e, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	f(&e.job)
	now := time.Now()
	if !force && now.Sub(e.lastEmit) < m.throttle {
		m.mu.Unlock()
		return
	}
	e.lastEmit = now
	snap := e.job.clone()
	emit := m.emit
	m.mu.Unlock()
	emit(snap)
}

// Cancel requests cancellation; it reports whether the job was still active.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	e, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok || e.job.State.Finished() {
		return false
	}
	e.cancel()
	return true
}

// Get returns one job.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return e.job.clone(), true
}

// List returns all known jobs, newest first.
func (m *Manager) List() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Job, 0, len(m.jobs))
	for _, e := range m.jobs {
		out = append(out, e.job.clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// ClearFinished forgets finished jobs.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.jobs {
		if e.job.State.Finished() {
			delete(m.jobs, id)
		}
	}
}

// pruneLocked keeps at most m.keep finished jobs.
func (m *Manager) pruneLocked() {
	var finished []*entry
	for _, e := range m.jobs {
		if e.job.State.Finished() {
			finished = append(finished, e)
		}
	}
	if len(finished) <= m.keep {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].job.CreatedAt.Before(finished[j].job.CreatedAt) })
	for _, e := range finished[:len(finished)-m.keep] {
		delete(m.jobs, e.job.ID)
	}
}

// Shutdown cancels all jobs and waits up to timeout for them to stop.
func (m *Manager) Shutdown(timeout time.Duration) {
	m.stop()
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		m.log.Warn("jobs still running at shutdown")
	}
}
