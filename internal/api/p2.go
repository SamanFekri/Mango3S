package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/ai"
	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/compare"
	"mango3s/internal/config"
	"mango3s/internal/jobs"
	"mango3s/internal/masking"
	"mango3s/internal/mcpserver"
	"mango3s/internal/migrate"
	"mango3s/internal/mongox"
	"mango3s/internal/plans"
	"mango3s/internal/reschema"
	"mango3s/internal/scheduler"
	"mango3s/internal/secrets"
	"mango3s/internal/syncer"
	"mango3s/internal/transfer"
	"mango3s/internal/transform"
)

// p2 holds the P2 services; it is embedded in App by pointer.
type p2 struct {
	compare   *compare.Service
	sync      *syncer.Service
	migrate   *migrate.Service
	scheduler *scheduler.Scheduler
	ai        *ai.Service
	masks     *masking.Store
	mcp       *mcpserver.Store
	xforms    *plans.Store[xformPlan]
}

// Secret-store keys for P2 credentials.
const (
	secretAIKey      = "mango3s:ai:anthropic"
	secretMaskingKey = "mango3s:masking-key"
)

func (a *App) backupDir() string { return backup.Dir(a.store.Dir()) }

func newP2(a *App) *p2 {
	p := &p2{
		compare: compare.New(a.conns, a.jobs),
		migrate: migrate.NewService(a.conns, a.jobs, a.store.Dir()),
		ai:      ai.NewService(a.conns, a.schema, backup.Dir(a.store.Dir())),
		masks:   masking.NewStore(a.store.Dir()),
		mcp:     mcpserver.NewStore(a.store.Dir()),
		xforms:  plans.New[xformPlan](plans.DefaultTTL),
	}
	p.sync = syncer.New(a.conns, a.jobs, p.compare, backup.Dir(a.store.Dir()))
	p.scheduler = scheduler.New(a.log, a.jobs, a.store.Dir())
	p.scheduler.SetNotifier(a.notifyTaskRun)
	return p
}

// ---- compare & sync ------------------------------------------------------------

// StartCompare queues a collection comparison (possibly across connections).
func (a *App) StartCompare(o compare.Options) (jobs.Job, error) {
	j, err := a.p2.compare.StartCollections(o)
	return j, a.encode("StartCompare", err)
}

// StartCompareDatabases queues a database comparison.
func (a *App) StartCompareDatabases(o compare.DBOptions) (jobs.Job, error) {
	j, err := a.p2.compare.StartDatabases(o)
	return j, a.encode("StartCompareDatabases", err)
}

// GetCompareResult returns a finished collection comparison.
func (a *App) GetCompareResult(jobID string) (*compare.Result, error) {
	r, err := a.p2.compare.CollectionResult(jobID)
	return r, a.encode("GetCompareResult", err)
}

// GetDatabaseCompareResult returns a finished database comparison.
func (a *App) GetDatabaseCompareResult(jobID string) (*compare.DBResult, error) {
	r, err := a.p2.compare.DatabaseResult(jobID)
	return r, a.encode("GetDatabaseCompareResult", err)
}

// PreviewSync builds a sync plan (dry run) from a comparison.
func (a *App) PreviewSync(compareJobID string, o syncer.Options) (syncer.Preview, error) {
	p, err := a.p2.sync.Preview(compareJobID, o)
	return p, a.encode("PreviewSync", err)
}

// StartSync applies a previewed plan. confirm must name the target when the plan deletes.
func (a *App) StartSync(planID, confirm string) (jobs.Job, error) {
	j, err := a.p2.sync.StartApply(planID, confirm)
	if err == nil {
		a.log.Info("sync started", "job", j.ID)
	}
	return j, a.encode("StartSync", err)
}

// CompareSchema lists the index and validation differences between the
// source and target collections of o.
func (a *App) CompareSchema(o compare.Options) (syncer.SchemaDiff, error) {
	return call(a, "CompareSchema", "", func(ctx context.Context) (syncer.SchemaDiff, error) {
		return syncer.DiffSchema(ctx, a.conns, o)
	})
}

// SyncSchema copies the chosen index and validation changes to the target.
// fingerprint comes from CompareSchema; confirm must repeat the target
// namespace when indexes are dropped or rebuilt.
func (a *App) SyncSchema(o compare.Options, ids []string, fingerprint, confirm string) (syncer.SchemaResult, error) {
	r, err := call(a, "SyncSchema", "", func(ctx context.Context) (syncer.SchemaResult, error) {
		return syncer.ApplySchema(ctx, a.conns, o, ids, fingerprint, confirm)
	})
	if err == nil {
		a.log.Info("schema synced", "target", o.Target.String(), "applied", len(r.Applied), "failed", len(r.Failed))
	}
	return r, err
}

// GetSyncOutcome returns the result of a sync job.
func (a *App) GetSyncOutcome(jobID string) (*syncer.Outcome, error) {
	o, err := a.p2.sync.Outcome(jobID)
	return o, a.encode("GetSyncOutcome", err)
}

// ---- backups ---------------------------------------------------------------------

// ListBackups returns recorded before-images, newest first.
func (a *App) ListBackups() ([]backup.Info, error) {
	l, err := backup.List(a.backupDir())
	return l, a.encode("ListBackups", err)
}

// StartRestore undoes a backed-up operation on a connection. confirm must be
// the backup's db.coll. Only files inside the backup directory are accepted.
func (a *App) StartRestore(path, connID, confirm string) (jobs.Job, error) {
	dir, _ := filepath.Abs(a.backupDir())
	abs, _ := filepath.Abs(path)
	if rel, err := filepath.Rel(dir, abs); err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return jobs.Job{}, a.encode("StartRestore", apperr.Invalid("Only backups created by Mango3S can be restored."))
	}
	h, err := backup.Read(abs, func(string, bson.D, any) error { return nil })
	if err != nil {
		return jobs.Job{}, a.encode("StartRestore", apperr.Wrap(apperr.CodeInvalidInput, err, "The backup cannot be read."))
	}
	if strings.TrimSpace(confirm) != h.DB+"."+h.Coll {
		return jobs.Job{}, a.encode("StartRestore", apperr.Invalid("Type %s.%s to confirm the restore.", h.DB, h.Coll))
	}
	sess, c, err := mongox.Collection(a.conns, connID, h.DB, h.Coll)
	if err != nil {
		return jobs.Job{}, a.encode("StartRestore", err)
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, a.encode("StartRestore", err)
	}
	j := a.jobs.Start("restore", "Restore "+h.DB+"."+h.Coll+" ("+h.Operation+")", func(ctx context.Context, p *jobs.Progress) (string, error) {
		if err := sess.RequireWrite(ctx, h.DB, h.Coll, mongox.ActionInsert, mongox.ActionUpdate, mongox.ActionRemove); err != nil {
			return "", err
		}
		st, err := backup.Restore(ctx, c, abs)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Restored %d document(s), removed %d inserted document(s)", st.Restored, st.Removed), nil
	})
	return j, nil
}

// ---- reschema & masking ------------------------------------------------------------

type xformPlan struct {
	Kind      string // reschema | mask
	Spec      transform.Spec
	Ops       []reschema.Op
	Rules     []masking.Rule
	Namespace string
}

// TransformRequest previews a reschema (Ops) or masking (Rules / RuleSetID) run.
type TransformRequest struct {
	Spec      transform.Spec `json:"spec"`
	Ops       []reschema.Op  `json:"ops"`
	Rules     []masking.Rule `json:"rules"`
	RuleSetID string         `json:"ruleSetId"`
}

// TransformPreview is a dry run plus the plan to apply.
type TransformPreview struct {
	PlanID      string            `json:"planId"`
	Preview     transform.Preview `json:"preview"`
	Target      string            `json:"target"`
	ConfirmText string            `json:"confirmText"` // required for in-place runs
}

func (a *App) maskingKey() ([]byte, error) {
	v, err := a.secrets.Get(secretMaskingKey)
	if errors.Is(err, secrets.ErrNotFound) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		v = hex.EncodeToString(b)
		if err := a.secrets.Set(secretMaskingKey, v); err != nil {
			return nil, apperr.Wrap(apperr.CodeSecretBackend, err, "Could not store the masking key.")
		}
	} else if err != nil {
		return nil, apperr.Wrap(apperr.CodeSecretBackend, err, "Could not read the masking key.")
	}
	return hex.DecodeString(v)
}

func (a *App) transformFunc(kind string, ops []reschema.Op, rules []masking.Rule) (transform.Func, error) {
	if kind == "reschema" {
		c, err := reschema.Compile(ops)
		if err != nil {
			return nil, err
		}
		return c.Apply, nil
	}
	key, err := a.maskingKey()
	if err != nil {
		return nil, err
	}
	m, err := masking.Compile(rules, key)
	if err != nil {
		return nil, err
	}
	return m.Apply, nil
}

func (a *App) previewTransform(kind string, req TransformRequest) (TransformPreview, error) {
	if kind == "mask" && req.RuleSetID != "" {
		rs, err := a.p2.masks.Get(req.RuleSetID)
		if err != nil {
			return TransformPreview{}, err
		}
		req.Rules = rs.Rules
	}
	fn, err := a.transformFunc(kind, req.Ops, req.Rules)
	if err != nil {
		return TransformPreview{}, err
	}
	return call(a, "Preview"+kind, "xform-preview", func(ctx context.Context) (TransformPreview, error) {
		pv, err := transform.RunPreview(ctx, a.conns, req.Spec, fn, 10)
		if err != nil {
			return TransformPreview{}, err
		}
		plan := xformPlan{Kind: kind, Spec: req.Spec, Ops: req.Ops, Rules: req.Rules, Namespace: req.Spec.Namespace()}
		out := TransformPreview{Preview: pv, Target: plan.Namespace, PlanID: a.p2.xforms.Put(plan)}
		if req.Spec.Mode == transform.ModeInPlace {
			out.ConfirmText = plan.Namespace
		}
		return out, nil
	})
}

// PreviewReschema dry-runs reschema operations.
func (a *App) PreviewReschema(req TransformRequest) (TransformPreview, error) {
	p, err := a.previewTransform("reschema", req)
	return p, a.encode("PreviewReschema", err)
}

// PreviewMask dry-runs masking rules.
func (a *App) PreviewMask(req TransformRequest) (TransformPreview, error) {
	p, err := a.previewTransform("mask", req)
	return p, a.encode("PreviewMask", err)
}

// StartTransform applies a previewed reschema or masking plan.
func (a *App) StartTransform(planID, confirm string) (jobs.Job, error) {
	plan, err := a.p2.xforms.Get(planID)
	if err != nil {
		return jobs.Job{}, a.encode("StartTransform", err)
	}
	if plan.Spec.Mode == transform.ModeInPlace && strings.TrimSpace(confirm) != plan.Namespace {
		return jobs.Job{}, a.encode("StartTransform", apperr.Invalid("This modifies %s in place. Type %s to confirm.", plan.Namespace, plan.Namespace))
	}
	fn, err := a.transformFunc(plan.Kind, plan.Ops, plan.Rules)
	if err != nil {
		return jobs.Job{}, a.encode("StartTransform", err)
	}
	if _, err := a.p2.xforms.Take(planID); err != nil {
		return jobs.Job{}, a.encode("StartTransform", err)
	}
	title := map[string]string{"reschema": "Reschema ", "mask": "Mask "}[plan.Kind] + plan.Spec.DB + "." + plan.Spec.Coll + " → " + plan.Namespace
	return a.jobs.Start(plan.Kind, title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		rep, err := transform.Apply(ctx, a.conns, plan.Spec, fn, plan.Kind, a.backupDir(), p)
		if rep != nil {
			a.storeReport(p.ID(), rep)
		}
		if err != nil {
			return "", err
		}
		summary := fmt.Sprintf("%d scanned, %d changed, %d written", rep.Scanned, rep.Changed, rep.Written)
		if rep.Skipped > 0 {
			summary += fmt.Sprintf(", %d skipped", rep.Skipped)
		}
		if !rep.Valid {
			return "", apperr.New(apperr.CodeConflict, "%s; validation failed (see the report)", summary)
		}
		return summary + "; validated", nil
	}), nil
}

func (a *App) storeReport(jobID string, rep *transform.Report) {
	a.p2Mu.Lock()
	defer a.p2Mu.Unlock()
	if a.reports == nil {
		a.reports = map[string]*transform.Report{}
	}
	a.reports[jobID] = rep
}

// GetTransformReport returns the report of a reschema or masking job.
func (a *App) GetTransformReport(jobID string) (*transform.Report, error) {
	a.p2Mu.Lock()
	defer a.p2Mu.Unlock()
	if r, ok := a.reports[jobID]; ok {
		return r, nil
	}
	return nil, a.encode("GetTransformReport", apperr.New(apperr.CodeNotFound, "Report not found."))
}

// ReschemaTypes lists the convertible target types.
func (a *App) ReschemaTypes() []string { return reschema.TargetTypes }

// MaskMethods lists masking methods and the BSON types they support.
func (a *App) MaskMethods() []masking.MethodInfo { return masking.Methods }

// ListMaskRuleSets returns saved masking rule sets.
func (a *App) ListMaskRuleSets() ([]masking.RuleSet, error) {
	l, err := a.p2.masks.List()
	return l, a.encode("ListMaskRuleSets", err)
}

// SaveMaskRuleSet stores a masking rule set.
func (a *App) SaveMaskRuleSet(rs masking.RuleSet) (masking.RuleSet, error) {
	r, err := a.p2.masks.Save(rs)
	return r, a.encode("SaveMaskRuleSet", err)
}

// DeleteMaskRuleSet removes a rule set.
func (a *App) DeleteMaskRuleSet(id string) error {
	return a.encode("DeleteMaskRuleSet", a.p2.masks.Delete(id))
}

// ---- migration --------------------------------------------------------------------

// IntrospectSQL lists tables of a relational source (read-only).
func (a *App) IntrospectSQL(src migrate.Source) ([]migrate.Table, error) {
	return call(a, "IntrospectSQL", "introspect", func(ctx context.Context) ([]migrate.Table, error) {
		return migrate.Introspect(ctx, src)
	})
}

// MigrationTypes lists target field types.
func (a *App) MigrationTypes() []string { return migrate.TargetTypes }

// StartMigration queues a relational → MongoDB migration.
func (a *App) StartMigration(req migrate.Request) (jobs.Job, error) {
	j, err := a.p2.migrate.Start(req)
	if err == nil {
		a.log.Info("migration started", "job", j.ID, "source", migrate.SafeDSNHint(req.Source), "db", req.DB)
	}
	return j, a.encode("StartMigration", err)
}

// ListMigrations returns recorded migration runs.
func (a *App) ListMigrations() ([]migrate.Run, error) {
	r, err := a.p2.migrate.Runs()
	return r, a.encode("ListMigrations", err)
}

// RollbackMigration drops the collections a run created. confirm must be the run's database name.
func (a *App) RollbackMigration(runID, confirm string) (migrate.Run, error) {
	run, err := a.p2.migrate.Get(runID)
	if err != nil {
		return migrate.Run{}, a.encode("RollbackMigration", err)
	}
	if strings.TrimSpace(confirm) != run.DB {
		return migrate.Run{}, a.encode("RollbackMigration", apperr.Invalid("Type the database name %q to confirm.", run.DB))
	}
	return call(a, "RollbackMigration", "", func(ctx context.Context) (migrate.Run, error) {
		if _, err := a.ensureConnected(ctx, run.ConnID); err != nil {
			return migrate.Run{}, err
		}
		return a.p2.migrate.Rollback(ctx, runID)
	})
}

// ---- scheduler -----------------------------------------------------------------------

// ListTaskTypes returns the task types that may be scheduled.
func (a *App) ListTaskTypes() []scheduler.TaskType { return a.p2.scheduler.Types() }

// ListTasks returns scheduled tasks.
func (a *App) ListTasks() ([]scheduler.Task, error) {
	t, err := a.p2.scheduler.List()
	return t, a.encode("ListTasks", err)
}

// SaveTask creates or updates a task (approval is never set here).
func (a *App) SaveTask(t scheduler.Task) (scheduler.Task, error) {
	r, err := a.p2.scheduler.Save(t)
	return r, a.encode("SaveTask", err)
}

// DeleteTask removes a task.
func (a *App) DeleteTask(id string) error {
	if err := a.p2.scheduler.Delete(id); err != nil {
		return a.encode("DeleteTask", err)
	}
	if err := a.secrets.Delete(taskWebhookSecret(id)); err != nil {
		a.log.Warn("could not delete the task's webhook URL", "err", err)
	}
	return nil
}

// ApproveTask records the user's approval of the task's current parameters.
func (a *App) ApproveTask(id string) (scheduler.Task, error) {
	t, err := a.p2.scheduler.Approve(id)
	if err == nil {
		a.log.Info("task approved", "task", t.Name, "type", t.Type)
	}
	return t, a.encode("ApproveTask", err)
}

// RevokeTask withdraws approval.
func (a *App) RevokeTask(id string) (scheduler.Task, error) {
	t, err := a.p2.scheduler.Revoke(id)
	return t, a.encode("RevokeTask", err)
}

// SetTaskEnabled turns a task on or off.
func (a *App) SetTaskEnabled(id string, enabled bool) (scheduler.Task, error) {
	t, err := a.p2.scheduler.SetEnabled(id, enabled)
	return t, a.encode("SetTaskEnabled", err)
}

// RunTaskNow runs an approved task immediately.
func (a *App) RunTaskNow(id string) (scheduler.Run, error) {
	r, err := a.p2.scheduler.RunNow(id)
	return r, a.encode("RunTaskNow", err)
}

// ListTaskRuns returns run history (all tasks when taskID is empty).
func (a *App) ListTaskRuns(taskID string, limit int) ([]scheduler.Run, error) {
	r, err := a.p2.scheduler.Runs(taskID, limit)
	return r, a.encode("ListTaskRuns", err)
}

// Task parameter shapes.
type exportTask struct {
	transfer.ExportOptions
}

type syncTask struct {
	Compare compare.Options `json:"compare"`
	Sync    syncer.Options  `json:"sync"`
}

type maskCopyTask struct {
	Spec          transform.Spec `json:"spec"`
	RuleSetID     string         `json:"ruleSetId"`
	ReplaceTarget bool           `json:"replaceTarget"` // drop the target copy before each run
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, apperr.Invalid("Invalid task parameters: %v", err)
	}
	return v, nil
}

// expandPath fills {date} and {time} placeholders in export paths.
func expandPath(p string, now time.Time) string {
	return strings.NewReplacer("{date}", now.Format("2006-01-02"), "{time}", now.Format("150405")).Replace(p)
}

func (a *App) registerTaskTypes() {
	s := a.p2.scheduler
	s.Register(scheduler.TaskType{
		ID: "export", Label: "Export", Description: "Export a query to a JSON or CSV file. {date} and {time} in the path are replaced at run time.",
		Validate: func(raw json.RawMessage) error {
			t, err := decode[exportTask](raw)
			if err != nil {
				return err
			}
			if t.ConnID == "" || t.DB == "" || t.Coll == "" || t.Path == "" {
				return apperr.Invalid("Export tasks need a connection, database, collection and path.")
			}
			if info, err := os.Stat(filepath.Dir(t.Path)); err != nil || !info.IsDir() {
				return apperr.Invalid("The folder %q does not exist.", filepath.Dir(t.Path))
			}
			return nil
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			t, err := decode[exportTask](raw)
			if err != nil {
				return "", err
			}
			if _, err := a.ensureConnected(ctx, t.ConnID); err != nil {
				return "", err
			}
			_, c, err := mongox.Collection(a.conns, t.ConnID, t.DB, t.Coll)
			if err != nil {
				return "", err
			}
			o := t.ExportOptions
			o.Path = expandPath(o.Path, time.Now())
			return transfer.RunExport(ctx, c, o, p)
		},
	})
	s.Register(scheduler.TaskType{
		ID: "compare", Label: "Compare", Description: "Compare two collections and record the differences; fails when they differ.",
		Validate: func(raw json.RawMessage) error {
			o, err := decode[compare.Options](raw)
			if err != nil {
				return err
			}
			if o.Source.ConnID == "" || o.Target.ConnID == "" || o.Source.Coll == "" || o.Target.Coll == "" {
				return apperr.Invalid("Compare tasks need a source and a target collection.")
			}
			return nil
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			o, err := decode[compare.Options](raw)
			if err != nil {
				return "", err
			}
			for _, id := range []string{o.Source.ConnID, o.Target.ConnID} {
				if _, err := a.ensureConnected(ctx, id); err != nil {
					return "", err
				}
			}
			r, err := compare.Run(ctx, a.conns, o, p)
			if err != nil {
				return "", err
			}
			if !r.Equal() {
				return "", apperr.New(apperr.CodeConflict, "%d changed, %d only in source, %d only in target (fingerprint %s)", r.Changed, r.OnlySource, r.OnlyTarget, r.Fingerprint)
			}
			return fmt.Sprintf("Identical (%d documents, fingerprint %s)", r.Identical, r.Fingerprint), nil
		},
	})
	s.Register(scheduler.TaskType{
		ID: "sync", Label: "Sync", Description: "Compare and synchronize source → target. Approval covers the selected kinds (including deletes).",
		Validate: func(raw json.RawMessage) error {
			t, err := decode[syncTask](raw)
			if err != nil {
				return err
			}
			if t.Compare.Source.ConnID == "" || t.Compare.Target.ConnID == "" {
				return apperr.Invalid("Sync tasks need a source and a target.")
			}
			if !t.Sync.Insert && !t.Sync.Replace && !t.Sync.Delete {
				return apperr.Invalid("Choose at least one of insert, replace or delete.")
			}
			return nil
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			t, err := decode[syncTask](raw)
			if err != nil {
				return "", err
			}
			for _, id := range []string{t.Compare.Source.ConnID, t.Compare.Target.ConnID} {
				if _, err := a.ensureConnected(ctx, id); err != nil {
					return "", err
				}
			}
			r, err := compare.Run(ctx, a.conns, t.Compare, p)
			if err != nil {
				return "", err
			}
			plan, err := syncer.BuildPlan(r, t.Sync)
			if err != nil {
				return "", err
			}
			if len(plan.Inserts)+len(plan.Replaces)+len(plan.Deletes) == 0 {
				return "Already in sync", nil
			}
			out, err := syncer.Apply(ctx, a.conns, plan, a.backupDir(), p)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Inserted %d, replaced %d, deleted %d; skipped %d changed since comparison; backup %s",
				out.Applied.Inserts, out.Applied.Replaces, out.Applied.Deletes,
				out.Skipped.Inserts+out.Skipped.Replaces+out.Skipped.Deletes, filepath.Base(out.BackupPath)), nil
		},
	})
	s.Register(scheduler.TaskType{
		ID: "maskCopy", Label: "Masked copy", Description: "Refresh a masked copy of a collection (copy mode only; the source is never modified).",
		Validate: func(raw json.RawMessage) error {
			t, err := decode[maskCopyTask](raw)
			if err != nil {
				return err
			}
			if t.Spec.Mode != transform.ModeCopy {
				return apperr.Invalid("Scheduled masking only writes copies; in-place masking must be run by hand.")
			}
			if _, err := a.p2.masks.Get(t.RuleSetID); err != nil {
				return err
			}
			return t.Spec.Validate()
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			t, err := decode[maskCopyTask](raw)
			if err != nil {
				return "", err
			}
			rs, err := a.p2.masks.Get(t.RuleSetID)
			if err != nil {
				return "", err
			}
			fn, err := a.transformFunc("mask", nil, rs.Rules)
			if err != nil {
				return "", err
			}
			sess, err := a.ensureConnected(ctx, t.Spec.ConnID)
			if err != nil {
				return "", err
			}
			if t.ReplaceTarget {
				if err := sess.RequireWrite(ctx, t.Spec.DB, t.Spec.TargetColl, mongox.ActionDropCollection); err != nil {
					return "", err
				}
				if err := sess.Client.Database(t.Spec.DB).Collection(t.Spec.TargetColl).Drop(ctx); err != nil {
					return "", apperr.From(err)
				}
			}
			rep, err := transform.Apply(ctx, a.conns, t.Spec, fn, "mask", a.backupDir(), p)
			if err != nil {
				return "", err
			}
			if !rep.Valid {
				return "", apperr.New(apperr.CodeConflict, "validation failed: %d written of %d scanned", rep.Written, rep.Scanned)
			}
			return fmt.Sprintf("Masked copy %s refreshed: %d documents", rep.Target, rep.Written), nil
		},
	})
}

// ---- AI ------------------------------------------------------------------------------

// AIStatus describes the AI configuration (never a key or token).
type AIStatus struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"`     // anthropic | claudeCode | codex | local
	ProviderName string `json:"providerName"` // e.g. "Claude Code"
	Ready        bool   `json:"ready"`        // configured well enough to try
	Detail       string `json:"detail"`       // what is (not) set up
	Model        string `json:"model"`        // model in use, as far as known
	LeavesDevice bool   `json:"leavesDevice"` // the context is sent to a remote service
	HasKey       bool   `json:"hasKey"`       // Anthropic API key available
	KeySource    string `json:"keySource"`    // stored | environment | none
	AllowSamples bool   `json:"allowSamples"`
	LocalHasKey  bool   `json:"localHasKey"` // an API key is stored for the local server
}

const secretLocalAIKey = "mango3s:ai:local"

// GetAIStatus reports whether AI assistance is usable with the chosen provider.
// For the CLIs it checks installation and sign-in without sending a request.
func (a *App) GetAIStatus() AIStatus {
	st := a.currentSettings()
	s := AIStatus{Enabled: st.AIEnabled, Provider: st.AIProvider, AllowSamples: st.AIAllowSamples, KeySource: "none"}
	if _, err := a.secrets.Get(secretAIKey); err == nil {
		s.HasKey, s.KeySource = true, "stored"
	} else if os.Getenv("ANTHROPIC_API_KEY") != "" {
		s.HasKey, s.KeySource = true, "environment"
	}
	if _, err := a.secrets.Get(secretLocalAIKey); err == nil {
		s.LocalHasKey = true
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	switch st.AIProvider {
	case config.AIProviderClaudeCode:
		info := detectCLI(ctx, config.AIProviderClaudeCode, st.ClaudeCodePath, false)
		s.ProviderName, s.Model, s.LeavesDevice = "Claude Code", orDefault(st.ClaudeCodeModel, "Claude Code's default model"), true
		s.Ready = info.Found && (info.SignedIn == nil || *info.SignedIn)
		s.Detail = cliDetail(info)
	case config.AIProviderCodex:
		info := detectCLI(ctx, config.AIProviderCodex, st.CodexPath, false)
		s.ProviderName, s.Model, s.LeavesDevice = "Codex", orDefault(st.CodexModel, "Codex's default model"), true
		s.Ready = info.Found && (info.SignedIn == nil || *info.SignedIn)
		s.Detail = cliDetail(info)
	case config.AIProviderLocal:
		s.ProviderName, s.Model, s.LeavesDevice = "Local model", st.LocalModel, !ai.IsLoopback(st.LocalURL)
		s.Ready = st.LocalModel != ""
		s.Detail = st.LocalURL + " (" + st.LocalAPI + " API)"
		if !s.Ready {
			s.Detail += " — choose a model"
		}
	default:
		s.ProviderName, s.Model, s.LeavesDevice = "Anthropic API", st.AIModel, true
		s.Ready = s.HasKey
		if !s.HasKey {
			s.Detail = "No API key. Add one below or set ANTHROPIC_API_KEY."
		}
	}
	return s
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func cliDetail(i ai.CLIInfo) string {
	if !i.Found {
		return i.Message
	}
	d := i.Name + " " + i.Version + " at " + i.Path
	switch {
	case i.SignedIn != nil && *i.SignedIn:
		d += " · signed in"
		if i.Account != "" {
			d += " with " + i.Account
		}
	case i.Message != "":
		d += " · " + i.Message
	}
	return d
}

// CLI detection runs `--version` and a sign-in check; cache it briefly so
// opening the AI dialog stays fast. Detect in Settings refreshes it.
var cliCache = struct {
	sync.Mutex
	at   map[string]time.Time
	info map[string]ai.CLIInfo
}{at: map[string]time.Time{}, info: map[string]ai.CLIInfo{}}

func detectCLI(ctx context.Context, which, override string, force bool) ai.CLIInfo {
	key := which + "|" + override
	cliCache.Lock()
	if t, ok := cliCache.at[key]; ok && !force && time.Since(t) < 30*time.Second {
		info := cliCache.info[key]
		cliCache.Unlock()
		return info
	}
	cliCache.Unlock()
	var info ai.CLIInfo
	if which == config.AIProviderClaudeCode {
		info = ai.DetectClaudeCode(ctx, override)
	} else {
		info = ai.DetectCodex(ctx, override)
	}
	cliCache.Lock()
	cliCache.at[key], cliCache.info[key] = time.Now(), info
	cliCache.Unlock()
	return info
}

// AIProviders is what Settings → AI can offer on this computer.
type AIProviders struct {
	ClaudeCode   ai.CLIInfo       `json:"claudeCode"`
	Codex        ai.CLIInfo       `json:"codex"`
	LocalServers []ai.LocalServer `json:"localServers"`
}

// DetectAIProviders looks for Claude Code, Codex and running local model
// servers (Ollama, LM Studio, llama.cpp, vLLM, Jan). Nothing is sent to a model.
func (a *App) DetectAIProviders() AIProviders {
	st := a.currentSettings()
	ctx, cancel := context.WithTimeout(a.ctx, 25*time.Second)
	defer cancel()
	var out AIProviders
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		out.ClaudeCode = detectCLI(ctx, config.AIProviderClaudeCode, st.ClaudeCodePath, true)
	}()
	go func() { defer wg.Done(); out.Codex = detectCLI(ctx, config.AIProviderCodex, st.CodexPath, true) }()
	go func() { defer wg.Done(); out.LocalServers = ai.DetectLocalServers(ctx) }()
	wg.Wait()
	return out
}

// ListLocalModels lists the models of a local model server.
func (a *App) ListLocalModels(url, api string) ([]string, error) {
	key, _ := a.secrets.Get(secretLocalAIKey)
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	m, err := ai.ListLocalModels(ctx, url, api, key)
	return m, a.encode("ListLocalModels", err)
}

// SetLocalAIKey stores an API key for an OpenAI-compatible server that needs one.
func (a *App) SetLocalAIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return a.encode("SetLocalAIKey", apperr.Invalid("Enter the server's API key."))
	}
	return a.encode("SetLocalAIKey", a.secrets.Set(secretLocalAIKey, key))
}

// ClearLocalAIKey removes the local server's API key.
func (a *App) ClearLocalAIKey() error {
	err := a.secrets.Delete(secretLocalAIKey)
	if errors.Is(err, secrets.ErrNotFound) {
		err = nil
	}
	return a.encode("ClearLocalAIKey", err)
}

// TestAI sends a tiny request to the chosen provider and checks that the
// answer has the expected shape. It uses a little of the provider's quota.
func (a *App) TestAI(opID string) (string, error) {
	provider, err := a.aiProvider()
	if err != nil {
		return "", a.encode("TestAI", err)
	}
	return callWithTimeout(a, "TestAI", opID, a.aiTimeout(), func(ctx context.Context) (string, error) {
		req := ai.Request{DB: "test", Coll: "orders", Prompt: "all orders with status \"A\"", Mode: ai.KindFind}
		c := ai.Context{Collection: "test.orders", Fields: []ai.FieldSummary{{Path: "status", Types: []string{"string"}, Presence: 100}}}
		start := time.Now()
		raw, usage, err := provider.Generate(ctx, ai.SystemPrompt, ai.UserMessage(req, c), ai.AnswerSchema())
		if err != nil {
			return "", err
		}
		p, err := ai.Validate(ai.ExtractJSON(raw), req)
		if err != nil {
			return "", err
		}
		model := usage.Model
		if model == "" {
			model = "the model"
		}
		return fmt.Sprintf("%s answered in %.1fs with a %s query: %s", model, time.Since(start).Seconds(), p.Kind, p.Filter), nil
	})
}

func (a *App) aiTimeout() time.Duration {
	return time.Duration(a.currentSettings().AITimeoutSeconds) * time.Second
}

// SetAIKey stores the Anthropic API key in the secret store.
func (a *App) SetAIKey(key string) error {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, "sk-ant-") || len(key) < 20 {
		return a.encode("SetAIKey", apperr.Invalid("That does not look like an Anthropic API key (sk-ant-…)."))
	}
	if err := a.secrets.Set(secretAIKey, key); err != nil {
		return a.encode("SetAIKey", apperr.Wrap(apperr.CodeSecretBackend, err, "Could not store the API key."))
	}
	a.log.Info("AI API key stored")
	return nil
}

// ClearAIKey removes the stored key.
func (a *App) ClearAIKey() error { return a.encode("ClearAIKey", a.secrets.Delete(secretAIKey)) }

func (a *App) aiProvider() (ai.Provider, error) {
	st := a.currentSettings()
	if !st.AIEnabled {
		return nil, apperr.New(apperr.CodeUnsupported, "AI assistance is turned off. Enable it in Settings → AI.")
	}
	if a.aiOverride != nil {
		return a.aiOverride, nil
	}
	switch st.AIProvider {
	case config.AIProviderClaudeCode:
		info := detectCLI(a.ctx, config.AIProviderClaudeCode, st.ClaudeCodePath, false)
		if !info.Found {
			return nil, apperr.New(apperr.CodeUnsupported, "Claude Code: %s", info.Message)
		}
		if info.SignedIn != nil && !*info.SignedIn {
			return nil, apperr.New(apperr.CodeAuthFailed, "Claude Code is not signed in. Run `claude` in a terminal and sign in.")
		}
		return &ai.ClaudeCode{Path: info.Path, Model: st.ClaudeCodeModel}, nil
	case config.AIProviderCodex:
		info := detectCLI(a.ctx, config.AIProviderCodex, st.CodexPath, false)
		if !info.Found {
			return nil, apperr.New(apperr.CodeUnsupported, "Codex: %s", info.Message)
		}
		if info.SignedIn != nil && !*info.SignedIn {
			return nil, apperr.New(apperr.CodeAuthFailed, "Codex is not signed in. Run `codex login` in a terminal.")
		}
		return &ai.Codex{Path: info.Path, Model: st.CodexModel}, nil
	case config.AIProviderLocal:
		key, _ := a.secrets.Get(secretLocalAIKey)
		return &ai.Local{URL: st.LocalURL, API: st.LocalAPI, Model: st.LocalModel, APIKey: key}, nil
	}
	key, err := a.secrets.Get(secretAIKey)
	if errors.Is(err, secrets.ErrNotFound) {
		key = "" // fall back to the SDK credential chain (ANTHROPIC_API_KEY, ant auth profiles)
	} else if err != nil {
		return nil, apperr.Wrap(apperr.CodeSecretBackend, err, "Could not read the API key.")
	}
	return ai.NewClaude(key, st.AIModel), nil
}

func (a *App) aiRequest(req ai.Request) ai.Request {
	if !a.currentSettings().AIAllowSamples {
		req.IncludeSamples = false
	}
	return req
}

// PreviewAIContext shows exactly what would be sent to the AI provider.
func (a *App) PreviewAIContext(req ai.Request) (ai.Context, error) {
	return call(a, "PreviewAIContext", "", func(ctx context.Context) (ai.Context, error) {
		return a.p2.ai.PreviewContext(ctx, a.aiRequest(req))
	})
}

// GenerateAI asks the provider for a validated proposal. Nothing is executed.
func (a *App) GenerateAI(opID string, req ai.Request) (*ai.Proposal, error) {
	provider, err := a.aiProvider()
	if err != nil {
		return nil, a.encode("GenerateAI", err)
	}
	return callWithTimeout(a, "GenerateAI", opID, a.aiTimeout(), func(ctx context.Context) (*ai.Proposal, error) {
		p, err := a.p2.ai.Generate(ctx, provider, a.aiRequest(req))
		if err == nil {
			a.log.Info("AI proposal", "kind", p.Kind, "valid", p.Valid, "write", p.IsWrite, "inputTokens", p.InputTokens, "outputTokens", p.OutputTokens)
		}
		return p, err
	})
}

// PreviewAIWrite counts the documents a write proposal would touch.
func (a *App) PreviewAIWrite(id string) (ai.WritePreview, error) {
	return call(a, "PreviewAIWrite", "", func(ctx context.Context) (ai.WritePreview, error) {
		return a.p2.ai.PreviewWrite(ctx, id)
	})
}

// ExecuteAIWrite runs a confirmed AI write proposal (backed up, single use).
func (a *App) ExecuteAIWrite(id, confirm string) (ai.WriteResult, error) {
	r, err := call(a, "ExecuteAIWrite", "", func(ctx context.Context) (ai.WriteResult, error) {
		return a.p2.ai.ExecuteWrite(ctx, id, confirm)
	})
	if err == nil {
		a.log.Info("AI write executed", "modified", r.Modified, "deleted", r.Deleted, "backup", r.BackupPath)
	}
	return r, err
}

// ---- MCP ------------------------------------------------------------------------------

// MCPInfo is what the settings page needs to configure clients.
type MCPInfo struct {
	Policy  mcpserver.Policy     `json:"policy"`
	Catalog []mcpserver.ToolInfo `json:"catalog"`
	Command string               `json:"command"`
	Args    []string             `json:"args"`
	Snippet string               `json:"snippet"`
}

// GetMCPInfo returns the MCP policy and client configuration.
func (a *App) GetMCPInfo() (MCPInfo, error) {
	p, err := a.p2.mcp.Load()
	if err != nil {
		return MCPInfo{}, a.encode("GetMCPInfo", err)
	}
	exe, _ := os.Executable()
	info := MCPInfo{Policy: p, Catalog: mcpserver.Catalog, Command: exe, Args: []string{"mcp"}}
	cfg := map[string]any{"mcpServers": map[string]any{"mango3s": map[string]any{"command": exe, "args": info.Args}}}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	info.Snippet = string(b)
	return info, nil
}

// SaveMCPPolicy stores the MCP policy.
func (a *App) SaveMCPPolicy(p mcpserver.Policy) (mcpserver.Policy, error) {
	saved, err := a.p2.mcp.Save(p)
	if err == nil {
		a.log.Info("MCP policy saved", "enabled", saved.Enabled, "tools", len(saved.Tools), "writes", saved.AllowWrites)
	}
	return saved, a.encode("SaveMCPPolicy", err)
}

// NewMCPServer builds the stdio MCP server for `mango3s mcp`. It is a
// function, not a method, so Wails does not bind it.
func NewMCPServer(a *App) (*mcpserver.Server, error) {
	policy, err := a.p2.mcp.Load()
	if err != nil {
		return nil, err
	}
	var masker *masking.Masker
	if policy.MaskRuleSetID != "" {
		rs, err := a.p2.masks.Get(policy.MaskRuleSetID)
		if err != nil {
			return nil, fmt.Errorf("MCP masking rule set: %w", err)
		}
		key, err := a.maskingKey()
		if err != nil {
			return nil, err
		}
		if masker, err = masking.Compile(rs.Rules, key); err != nil {
			return nil, err
		}
	}
	return mcpserver.New(mcpserver.Deps{
		Policy:   policy,
		Profiles: a.store.ListProfiles,
		Connect: func(ctx context.Context, id string) (*mongox.Session, error) {
			return a.ensureConnected(ctx, id)
		},
		Masker:    masker,
		BackupDir: a.backupDir(),
		Log:       a.log,
		Version:   Version,
	}), nil
}

// SetAIProvider replaces the AI provider (tests use a fake). It is a
// function, not a method, so Wails does not bind it.
func SetAIProvider(a *App, p ai.Provider) { a.aiOverride = p }
