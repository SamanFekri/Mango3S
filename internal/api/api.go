// Package api is the thin facade bound to the Wails frontend. Every exported
// method of *App becomes a TypeScript function. It wires services together,
// applies timeouts and cancellation, and encodes errors as JSON so the UI can
// read the error code.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/aggregate"
	"mango3s/internal/ai"
	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/documents"
	"mango3s/internal/explain"
	"mango3s/internal/explorer"
	"mango3s/internal/jobs"
	"mango3s/internal/library"
	"mango3s/internal/logging"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/recent"
	"mango3s/internal/schema"
	"mango3s/internal/secrets"
	"mango3s/internal/shell"
	"mango3s/internal/transfer"
	"mango3s/internal/transform"
)

// Version is set at build time with -ldflags "-X mango3s/internal/api.Version=...".
var Version = "0.1.0-dev"

// Deps are the collaborators of App.
type Deps struct {
	Log      *slog.Logger
	LogLevel *slog.LevelVar
	LogPath  string
	Store    *config.Store
	Secrets  secrets.Store
	Conns    *mongox.Manager
}

// App is bound to the frontend.
type App struct {
	ctx      context.Context
	log      *slog.Logger
	logLevel *slog.LevelVar
	logPath  string
	store    *config.Store
	secrets  secrets.Store
	conns    *mongox.Manager

	explorer explorer.Service
	docs     documents.Service
	query    query.Service
	agg      aggregate.Service
	shell    shell.Service

	// P1
	jobs     *jobs.Manager
	transfer *transfer.Service
	schema   *schema.Service
	explain  *explain.Service
	library  *library.Store
	history  *library.History

	recent      *recent.Store
	unsavedWork atomic.Bool          // set by the UI; the window asks before quitting
	emitStep    func(ConnectionStep) // connection log events (nil until the UI starts)

	update *updateState // platform check-in: auto-update + notifications (see update.go)

	settingsMu    sync.RWMutex
	settings      config.Settings
	settingsWrite sync.Mutex // serializes read-modify-write saves (see mutateSettings)

	// P2
	p2         *p2
	p2Mu       sync.Mutex
	reports    map[string]*transform.Report
	aiOverride ai.Provider

	ops *opRegistry
}

// New constructs the facade and its services.
func New(d Deps) *App {
	a := &App{
		ctx:      context.Background(),
		log:      d.Log,
		logLevel: d.LogLevel,
		logPath:  d.LogPath,
		store:    d.Store,
		secrets:  d.Secrets,
		conns:    d.Conns,
		explorer: explorer.New(d.Conns),
		docs:     documents.New(d.Conns),
		query:    query.New(d.Conns),
		agg:      aggregate.New(d.Conns),
		shell:    shell.New(d.Conns),
		ops:      newOpRegistry(),
		jobs:     jobs.NewManager(d.Log, 2),
		schema:   schema.New(d.Conns),
		explain:  explain.New(d.Conns),
		library:  library.NewStore(d.Store.Dir()),
		history:  library.NewHistory(d.Store.Dir(), 500),
		recent:   recent.New(d.Store.Dir()),
		update:   newUpdateState(),
	}
	a.transfer = transfer.New(d.Conns, a.jobs)
	a.p2 = newP2(a)
	a.registerTaskTypes()
	a.registerChainTask()
	st, err := d.Store.LoadSettings()
	if err != nil {
		a.log.Warn("settings could not be loaded; using defaults", "err", err)
	}
	a.settings = st
	if a.logLevel != nil {
		a.logLevel.Set(logging.ParseLevel(st.LogLevel))
	}
	return a
}

// Lifecycle exposes startup/shutdown hooks without binding them to the UI.
type Lifecycle struct{ app *App }

// NewLifecycle returns the hooks for the desktop runtime. It is a function
// rather than a method so that Wails does not expose it to the frontend.
func NewLifecycle(a *App) Lifecycle { return Lifecycle{app: a} }

// Startup stores the runtime context.
func (l Lifecycle) Startup(ctx context.Context) {
	l.app.ctx = ctx
	l.app.emitStep = func(st ConnectionStep) { runtime.EventsEmit(ctx, "conn:step", st) }
	// Push job progress to the UI; the frontend listens for "jobs:update".
	l.app.jobs.SetEmitter(func(j jobs.Job) { runtime.EventsEmit(ctx, "jobs:update", j) })
	if l.app.currentSettings().SchedulerEnabled {
		if err := l.app.p2.scheduler.Start(ctx, 30*time.Second); err != nil {
			l.app.log.Warn("scheduler did not start", "err", err)
		}
	}
	l.app.startUpdateTicker(ctx)
	l.app.log.Info("Mango3S started", "version", Version)
}

// Shutdown cancels running operations and closes all connections.
func (l Lifecycle) Shutdown(ctx context.Context) {
	l.app.ops.cancelAll()
	l.app.p2.scheduler.Stop()
	l.app.jobs.Shutdown(5 * time.Second)
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l.app.conns.CloseAll(c)
	l.app.log.Info("Mango3S stopped")
}

// ---- errors, timeouts, cancellation -------------------------------------

// encode converts any error into a JSON-encoded apperr.Error string. Wails
// rejects the JS promise with this string; the frontend parses it.
func (a *App) encode(op string, err error) error {
	if err == nil {
		return nil
	}
	ae := apperr.From(err)
	ae.Message = logging.Redact(ae.Message)
	ae.Detail = logging.Redact(ae.Detail)
	switch ae.Code {
	case apperr.CodeInvalidInput, apperr.CodeCanceled, apperr.CodeNotFound, apperr.CodeReadOnly:
		a.log.Debug("operation rejected", "op", op, "code", ae.Code, "err", ae.Message)
	default:
		a.log.Warn("operation failed", "op", op, "code", ae.Code, "err", ae.Message, "detail", ae.Detail)
	}
	b, _ := json.Marshal(ae)
	return errors.New(string(b))
}

func (a *App) currentSettings() config.Settings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings
}

func (a *App) timeout() time.Duration {
	return time.Duration(a.currentSettings().QueryTimeoutSeconds) * time.Second
}

// call runs fn with the configured timeout, registered under opID (if any) so
// CancelOperation can abort it, and logs its duration.
func call[T any](a *App, op, opID string, fn func(ctx context.Context) (T, error)) (T, error) {
	return callWithTimeout(a, op, opID, a.timeout(), fn)
}

// callWithTimeout is call with an explicit timeout (AI requests can be slow).
func callWithTimeout[T any](a *App, op, opID string, timeout time.Duration, fn func(ctx context.Context) (T, error)) (T, error) {
	ctx, done := a.ops.start(a.ctx, opID, timeout)
	defer done()
	start := time.Now()
	v, err := fn(ctx)
	a.log.Debug("operation", "op", op, "ms", time.Since(start).Milliseconds(), "ok", err == nil)
	if err != nil {
		var zero T
		return zero, a.encode(op, err)
	}
	return v, nil
}

// opRegistry tracks cancelable operations by a frontend-chosen ID.
type opRegistry struct {
	mu  sync.Mutex
	ops map[string]*runningOp
}

type runningOp struct{ cancel context.CancelFunc }

func newOpRegistry() *opRegistry { return &opRegistry{ops: map[string]*runningOp{}} }

func (r *opRegistry) start(parent context.Context, opID string, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	if opID == "" {
		return ctx, cancel
	}
	op := &runningOp{cancel: cancel}
	r.mu.Lock()
	if prev, ok := r.ops[opID]; ok {
		prev.cancel() // a re-run with the same ID supersedes the previous run
	}
	r.ops[opID] = op
	r.mu.Unlock()
	return ctx, func() {
		cancel()
		r.mu.Lock()
		// Only remove our own registration; a newer run may have replaced it.
		if r.ops[opID] == op {
			delete(r.ops, opID)
		}
		r.mu.Unlock()
	}
}

func (r *opRegistry) cancel(opID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	op, ok := r.ops[opID]
	if ok {
		op.cancel()
		delete(r.ops, opID)
	}
	return ok
}

func (r *opRegistry) cancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, op := range r.ops {
		op.cancel()
		delete(r.ops, id)
	}
}

// CancelOperation aborts a running query, aggregation or shell script.
func (a *App) CancelOperation(opID string) bool {
	ok := a.ops.cancel(opID)
	if ok {
		a.log.Info("operation canceled by user", "opId", opID)
	}
	return ok
}

// ---- app & settings -------------------------------------------------------

// AppInfo describes the installation.
type AppInfo struct {
	Version       string `json:"version"`
	ConfigDir     string `json:"configDir"`
	LogFile       string `json:"logFile"`
	SecretBackend string `json:"secretBackend"`

	// Shown in About Mango3S.
	Commit     string            `json:"commit"`    // VCS revision the binary was built from
	BuildTime  string            `json:"buildTime"` // VCS commit time
	Modified   bool              `json:"modified"`  // built with uncommitted changes
	GoVersion  string            `json:"goVersion"`
	Platform   string            `json:"platform"`   // e.g. darwin/arm64
	Executable string            `json:"executable"` // path of the running binary (used by `mango3s mcp`)
	BackupDir  string            `json:"backupDir"`
	Components map[string]string `json:"components"` // key libraries and their versions
}

// keyModules are the libraries listed in About Mango3S.
var keyModules = map[string]string{
	"github.com/wailsapp/wails/v2":           "Wails",
	"go.mongodb.org/mongo-driver/v2":         "MongoDB Go driver",
	"github.com/anthropics/anthropic-sdk-go": "Anthropic Go SDK",
	"github.com/modelcontextprotocol/go-sdk": "MCP Go SDK",
	"github.com/jackc/pgx/v5":                "pgx (PostgreSQL)",
	"github.com/go-sql-driver/mysql":         "MySQL driver",
	"modernc.org/sqlite":                     "SQLite (modernc)",
	"golang.org/x/crypto":                    "x/crypto (SSH)",
	"github.com/zalando/go-keyring":          "go-keyring",
}

// GetAppInfo returns version, build details and paths.
func (a *App) GetAppInfo() AppInfo {
	info := AppInfo{Version: Version, ConfigDir: a.store.Dir(), LogFile: a.logPath, SecretBackend: a.secrets.Backend(),
		GoVersion: goruntime.Version(), Platform: goruntime.GOOS + "/" + goruntime.GOARCH,
		BackupDir: filepath.Join(a.store.Dir(), "backups"), Components: map[string]string{}}
	if exe, err := os.Executable(); err == nil {
		info.Executable = exe
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.BuildTime = s.Value
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
		for _, d := range bi.Deps {
			if name, ok := keyModules[d.Path]; ok {
				v := d.Version
				if d.Replace != nil {
					v = d.Replace.Version
				}
				info.Components[name] = v
			}
		}
	}
	return info
}

// GetSettings returns the current settings.
func (a *App) GetSettings() config.Settings { return a.currentSettings() }

// SaveSettings validates, persists and applies settings.
func (a *App) SaveSettings(s config.Settings) (config.Settings, error) {
	a.settingsWrite.Lock()
	defer a.settingsWrite.Unlock()
	// The platform identity is owned by the backend: a Settings form loaded
	// before the app registered would otherwise send stale (empty) values and
	// make the next registration look like a brand-new installation.
	cur := a.currentSettings()
	s.DeviceFingerprint, s.UpdateInstallationID = cur.DeviceFingerprint, cur.UpdateInstallationID
	return a.applySettings(s)
}

// mutateSettings is the backend's own read-modify-write of settings,
// serialized with SaveSettings so neither overwrites the other's change.
func (a *App) mutateSettings(fn func(*config.Settings)) error {
	a.settingsWrite.Lock()
	defer a.settingsWrite.Unlock()
	s := a.currentSettings()
	fn(&s)
	_, err := a.applySettings(s)
	return err
}

func (a *App) applySettings(s config.Settings) (config.Settings, error) {
	saved, err := a.store.SaveSettings(s)
	if err != nil {
		return config.Settings{}, a.encode("SaveSettings", err)
	}
	a.settingsMu.Lock()
	prev := a.settings
	a.settings = saved
	a.settingsMu.Unlock()
	switch {
	case saved.SchedulerEnabled && !prev.SchedulerEnabled:
		if err := a.p2.scheduler.Start(a.ctx, 30*time.Second); err != nil {
			a.log.Warn("scheduler did not start", "err", err)
		}
	case !saved.SchedulerEnabled && prev.SchedulerEnabled:
		a.p2.scheduler.Stop()
	}
	if a.logLevel != nil {
		a.logLevel.Set(logging.ParseLevel(saved.LogLevel))
	}
	return saved, nil
}

// ensureConnected returns a session for a profile ID, connecting if needed.
func (a *App) ensureConnected(ctx context.Context, profileID string) (*mongox.Session, error) {
	if s, err := a.conns.Session(profileID); err == nil {
		return s, nil
	}
	p, err := a.store.GetProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.connectProfile(ctx, p)
}

// Disconnect closes a connection.
func (a *App) Disconnect(connID string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	return a.encode("Disconnect", a.conns.Disconnect(ctx, connID))
}

// ListConnections returns open connections.
func (a *App) ListConnections() []mongox.SessionInfo { return a.conns.List() }

// ---- explorer --------------------------------------------------------------

// ListDatabases lists databases visible to the user.
func (a *App) ListDatabases(connID string) ([]explorer.DatabaseInfo, error) {
	return call(a, "ListDatabases", "", func(ctx context.Context) ([]explorer.DatabaseInfo, error) {
		return a.explorer.ListDatabases(ctx, connID)
	})
}

// ListCollections lists collections and views of a database.
func (a *App) ListCollections(connID, db string) ([]explorer.CollectionInfo, error) {
	return call(a, "ListCollections", "", func(ctx context.Context) ([]explorer.CollectionInfo, error) {
		return a.explorer.ListCollections(ctx, connID, db)
	})
}

// CollectionStats returns size, count and index information.
func (a *App) CollectionStats(connID, db, coll string) (explorer.CollectionStats, error) {
	return call(a, "CollectionStats", "", func(ctx context.Context) (explorer.CollectionStats, error) {
		return a.explorer.CollectionStats(ctx, connID, db, coll)
	})
}

// SampleFields returns field paths from a random sample (for autocomplete and the query builder).
func (a *App) SampleFields(connID, db, coll string) ([]string, error) {
	return call(a, "SampleFields", "", func(ctx context.Context) ([]string, error) {
		return a.explorer.SampleFields(ctx, connID, db, coll, 100)
	})
}

// CreateCollection creates a collection (and implicitly its database).
func (a *App) CreateCollection(connID, db, coll string) error {
	_, err := call(a, "CreateCollection", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.explorer.CreateCollection(ctx, connID, db, coll)
	})
	return err
}

// DropCollection drops a collection.
func (a *App) DropCollection(connID, db, coll string) error {
	_, err := call(a, "DropCollection", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.explorer.DropCollection(ctx, connID, db, coll)
	})
	if err == nil {
		a.log.Info("collection dropped", "db", db, "coll", coll)
	}
	return err
}

// DropDatabase drops a database.
func (a *App) DropDatabase(connID, db string) error {
	_, err := call(a, "DropDatabase", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.explorer.DropDatabase(ctx, connID, db)
	})
	if err == nil {
		a.log.Info("database dropped", "db", db)
	}
	return err
}

// ---- documents & queries ----------------------------------------------------

// RunQuery executes a find query and returns one page.
func (a *App) RunQuery(opID string, req query.Request) (model.Page, error) {
	start := time.Now()
	page, err := call(a, "RunQuery", opID, func(ctx context.Context) (model.Page, error) {
		return a.query.Find(ctx, req)
	})
	if req.Page == 0 {
		a.recordQuery(req, time.Since(start), err)
	}
	return page, err
}

// CountQuery counts documents matching the query (honoring skip/limit).
func (a *App) CountQuery(opID string, req query.Request) (int64, error) {
	return call(a, "CountQuery", opID, func(ctx context.Context) (int64, error) {
		return a.query.Count(ctx, req)
	})
}

// QueryToShell renders the query as mongosh code.
func (a *App) QueryToShell(req query.Request) (string, error) {
	s, err := query.ToShell(req)
	return s, a.encode("QueryToShell", err)
}

// GetDocument fetches one document by _id.
func (a *App) GetDocument(connID, db, coll, id string) (model.Document, error) {
	return call(a, "GetDocument", "", func(ctx context.Context) (model.Document, error) {
		return a.docs.Get(ctx, connID, db, coll, id)
	})
}

// InsertDocuments inserts one document or an array of documents.
func (a *App) InsertDocuments(connID, db, coll, text string) ([]string, error) {
	return call(a, "InsertDocuments", "", func(ctx context.Context) ([]string, error) {
		return a.docs.Insert(ctx, connID, db, coll, text)
	})
}

// ReplaceDocument replaces the document with the given _id.
func (a *App) ReplaceDocument(connID, db, coll, id, text string) (model.Document, error) {
	return call(a, "ReplaceDocument", "", func(ctx context.Context) (model.Document, error) {
		return a.docs.Replace(ctx, connID, db, coll, id, text)
	})
}

// DeleteDocuments deletes documents by _id.
func (a *App) DeleteDocuments(connID, db, coll string, ids []string) (int64, error) {
	n, err := call(a, "DeleteDocuments", "", func(ctx context.Context) (int64, error) {
		return a.docs.Delete(ctx, connID, db, coll, ids)
	})
	if err == nil {
		a.log.Info("documents deleted", "db", db, "coll", coll, "count", n)
	}
	return n, err
}

// FormatDocument validates shell-syntax text and returns it pretty-printed.
func (a *App) FormatDocument(text string) (string, error) {
	v, err := mql.ParseValue(text)
	if err != nil {
		return "", a.encode("FormatDocument", apperr.Wrap(apperr.CodeInvalidInput, err, err.Error()))
	}
	return mql.Pretty(v), nil
}

// ---- aggregation & shell ---------------------------------------------------

// RunAggregation runs a pipeline (or a prefix of it) and returns one page.
func (a *App) RunAggregation(opID string, req aggregate.Request) (model.Page, error) {
	start := time.Now()
	page, err := call(a, "RunAggregation", opID, func(ctx context.Context) (model.Page, error) {
		return a.agg.Run(ctx, req)
	})
	if req.Page == 0 && req.UpTo < 0 {
		a.recordAggregation(req, time.Since(start), err)
	}
	return page, err
}

// PipelineToText renders stages as a mongosh pipeline array.
func (a *App) PipelineToText(stages []aggregate.Stage) (string, error) {
	s, err := aggregate.ToText(stages)
	return s, a.encode("PipelineToText", err)
}

// PipelineFromText parses a pipeline array into editable stages.
func (a *App) PipelineFromText(text string) ([]aggregate.Stage, error) {
	s, err := aggregate.FromText(text)
	return s, a.encode("PipelineFromText", err)
}

// RunShellPage re-runs one find/aggregate statement from the shell and
// returns the requested page (0-based) of its results.
func (a *App) RunShellPage(opID, connID, db, statement string, page int) (shell.Output, error) {
	return call(a, "RunShellPage", opID, func(ctx context.Context) (shell.Output, error) {
		return a.shell.Page(ctx, connID, db, statement, page, a.currentSettings().PageSize)
	})
}

// ParseCollectionQuery reads the collection tab's query editor text, e.g.
// db.getCollection("orders").find({}).sort({ _id: -1 }), into a query that
// runs with paging.
func (a *App) ParseCollectionQuery(text, db string) (shell.QuerySpec, error) {
	q, err := shell.ParseQuery(text, db)
	return q, a.encode("ParseCollectionQuery", err)
}

// RunShell executes shell commands against the given database.
func (a *App) RunShell(opID, connID, db, script string) (shell.Result, error) {
	start := time.Now()
	res, err := call(a, "RunShell", opID, func(ctx context.Context) (shell.Result, error) {
		return a.shell.Execute(ctx, connID, db, script, a.currentSettings().PageSize)
	})
	a.recordShell(connID, db, script, res, time.Since(start), err)
	return res, err
}
