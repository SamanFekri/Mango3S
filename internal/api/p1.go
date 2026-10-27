package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/charts"
	"mango3s/internal/codegen"
	"mango3s/internal/explain"
	"mango3s/internal/jobs"
	"mango3s/internal/library"
	"mango3s/internal/logging"
	"mango3s/internal/model"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/schema"
	"mango3s/internal/shell"
	"mango3s/internal/sqlq"
	"mango3s/internal/transfer"
)

// ---- background jobs -------------------------------------------------------

// ListJobs returns known jobs, newest first.
func (a *App) ListJobs() []jobs.Job { return a.jobs.List() }

// CancelJob requests cancellation of a running or queued job.
func (a *App) CancelJob(id string) bool { return a.jobs.Cancel(id) }

// ClearFinishedJobs forgets completed, failed and canceled jobs.
func (a *App) ClearFinishedJobs() { a.jobs.ClearFinished() }

// ---- import / export -------------------------------------------------------

// ChooseImportFile opens a native file picker. An empty result means canceled.
func (a *App) ChooseImportFile() (string, error) {
	// No extension filter: exports often have other or no extensions. Unknown
	// extensions default to JSON and the wizard lets the user pick CSV/TSV.
	path, err := a.openFileDialog("file", runtime.OpenDialogOptions{Title: "Import documents"})
	return path, a.encode("ChooseImportFile", err)
}

// ChooseExportFile opens a native save dialog. An empty result means canceled.
func (a *App) ChooseExportFile(defaultName string) (string, error) {
	path, err := a.saveFileDialog(runtime.SaveDialogOptions{
		Title:           "Export documents",
		DefaultFilename: filepath.Base(defaultName),
	})
	return path, a.encode("ChooseExportFile", err)
}

// PreviewImport parses the first documents of a file with the given options.
func (a *App) PreviewImport(o transfer.ImportOptions) (transfer.Preview, error) {
	p, err := transfer.PreviewFile(o, 20)
	return p, a.encode("PreviewImport", err)
}

// StartImport queues an import job.
func (a *App) StartImport(o transfer.ImportOptions) (jobs.Job, error) {
	j, err := a.transfer.StartImport(o)
	if err == nil {
		a.log.Info("import started", "job", j.ID, "db", o.DB, "coll", o.Coll, "format", o.Format, "mode", o.Mode)
	}
	return j, a.encode("StartImport", err)
}

// StartExport queues an export job.
func (a *App) StartExport(o transfer.ExportOptions) (jobs.Job, error) {
	j, err := a.transfer.StartExport(o)
	if err == nil {
		a.log.Info("export started", "job", j.ID, "db", o.DB, "coll", o.Coll, "format", o.Format)
	}
	return j, a.encode("StartExport", err)
}

// ---- schema ----------------------------------------------------------------

// AnalyzeSchema samples a collection and reports field and type statistics.
func (a *App) AnalyzeSchema(opID string, req schema.Request) (schema.Report, error) {
	return call(a, "AnalyzeSchema", opID, func(ctx context.Context) (schema.Report, error) {
		return a.schema.Analyze(ctx, req)
	})
}

// SuggestValidator turns a schema report into a { $jsonSchema: … } validator
// in mongosh syntax, for the validation editor (Settings › General ›
// "Suggest validation rules from the schema" shows the button; off by default).
func (a *App) SuggestValidator(r schema.Report, o schema.JSONSchemaOptions) string {
	return mql.Pretty(schema.ToJSONSchema(r, o))
}

// ---- SQL -------------------------------------------------------------------

// SQLTranslation is a SQL statement translated to MongoDB.
type SQLTranslation struct {
	Kind       string `json:"kind"`
	Collection string `json:"collection"`
	Shell      string `json:"shell"`
}

// SQLRequest runs SQL against a database.
type SQLRequest struct {
	ConnID   string `json:"connId"`
	DB       string `json:"db"`
	SQL      string `json:"sql"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// SQLResult is one page of SQL results plus the translation that produced it.
type SQLResult struct {
	Translation SQLTranslation `json:"translation"`
	Page        model.Page     `json:"page"`
}

// TranslateSQL converts SQL without running it.
func (a *App) TranslateSQL(sql string) (SQLTranslation, error) {
	t, err := sqlq.Translate(sql)
	if err != nil {
		return SQLTranslation{}, a.encode("TranslateSQL", err)
	}
	return SQLTranslation{Kind: t.Kind, Collection: t.Collection, Shell: t.Shell()}, nil
}

// RunSQL translates and executes SQL, returning one page.
func (a *App) RunSQL(opID string, req SQLRequest) (SQLResult, error) {
	start := time.Now()
	res, err := call(a, "RunSQL", opID, func(ctx context.Context) (SQLResult, error) {
		t, err := sqlq.Translate(req.SQL)
		if err != nil {
			return SQLResult{}, err
		}
		out := SQLResult{Translation: SQLTranslation{Kind: t.Kind, Collection: t.Collection, Shell: t.Shell()}}
		if t.Kind == "find" {
			out.Page, err = a.query.Find(ctx, findRequest(req.ConnID, req.DB, t, req.Page, req.PageSize))
		} else {
			out.Page, err = a.agg.Run(ctx, aggregate.Request{ConnID: req.ConnID, DB: req.DB, Coll: t.Collection,
				Stages: toStages(t.Pipeline), UpTo: -1, Page: req.Page, PageSize: req.PageSize})
		}
		return out, err
	})
	if req.Page == 0 {
		a.record(library.Entry{Kind: library.KindSQL, ConnName: a.connName(req.ConnID), DB: req.DB,
			Summary: oneLine(req.SQL), Text: req.SQL}, time.Since(start), err)
	}
	return res, err
}

func findRequest(connID, db string, t sqlq.Translation, page, pageSize int) query.Request {
	r := query.Request{ConnID: connID, DB: db, Coll: t.Collection, Filter: mql.Compact(t.Filter),
		Skip: t.Skip, Limit: t.Limit, Page: page, PageSize: pageSize}
	if len(t.Projection) > 0 {
		r.Projection = mql.Compact(t.Projection)
	}
	if len(t.Sort) > 0 {
		r.Sort = mql.Compact(t.Sort)
	}
	return r
}

func toStages(p []bson.D) []aggregate.Stage {
	out := make([]aggregate.Stage, len(p))
	for i, st := range p {
		out[i] = aggregate.Stage{Operator: st[0].Key, Body: mql.Compact(st[0].Value), Enabled: true}
	}
	return out
}

// ---- code generation -------------------------------------------------------

// CodeRequest selects the source of generated code. Source is "query",
// "aggregate" or "sql".
type CodeRequest struct {
	Language string            `json:"language"`
	Source   string            `json:"source"`
	DB       string            `json:"db"`
	Coll     string            `json:"coll"`
	Query    query.Request     `json:"query"`
	Stages   []aggregate.Stage `json:"stages"`
	SQL      string            `json:"sql"`
}

// CodeLanguages lists the code generation targets.
func (a *App) CodeLanguages() []codegen.Language { return codegen.Languages }

// GenerateCode renders the query, pipeline or SQL in the requested language.
func (a *App) GenerateCode(req CodeRequest) (string, error) {
	spec, err := codeSpec(req)
	if err != nil {
		return "", a.encode("GenerateCode", err)
	}
	code, err := codegen.Generate(req.Language, spec)
	return code, a.encode("GenerateCode", err)
}

func codeSpec(req CodeRequest) (codegen.Spec, error) {
	switch req.Source {
	case "query":
		p, err := query.Parse(req.Query)
		if err != nil {
			return codegen.Spec{}, err
		}
		return codegen.Spec{Kind: "find", DB: req.DB, Coll: req.Coll, Filter: p.Filter, Projection: p.Projection,
			Sort: p.Sort, Skip: req.Query.Skip, Limit: req.Query.Limit}, nil
	case "aggregate":
		pipe, err := aggregate.Build(req.Stages, -1)
		if err != nil {
			return codegen.Spec{}, err
		}
		stages := make([]bson.D, len(pipe))
		copy(stages, pipe)
		return codegen.Spec{Kind: "aggregate", DB: req.DB, Coll: req.Coll, Pipeline: stages}, nil
	case "sql":
		t, err := sqlq.Translate(req.SQL)
		if err != nil {
			return codegen.Spec{}, err
		}
		return codegen.Spec{Kind: t.Kind, DB: req.DB, Coll: t.Collection, Filter: t.Filter, Projection: t.Projection,
			Sort: t.Sort, Skip: t.Skip, Limit: t.Limit, Pipeline: t.Pipeline}, nil
	}
	return codegen.Spec{}, apperr.Invalid("Unknown code source %q.", req.Source)
}

// ---- explain ---------------------------------------------------------------

// Explain runs explain with execution statistics and returns the analyzed plan.
func (a *App) Explain(opID string, req explain.Request) (explain.Plan, error) {
	return call(a, "Explain", opID, func(ctx context.Context) (explain.Plan, error) {
		return a.explain.Explain(ctx, req)
	})
}

// ---- saved items & history -------------------------------------------------

// ListSaved returns saved items of a kind ("" = all).
func (a *App) ListSaved(kind string) ([]library.Item, error) {
	items, err := a.library.List(kind)
	return items, a.encode("ListSaved", err)
}

// SaveItem creates or updates a saved query, pipeline, script or SQL statement.
func (a *App) SaveItem(it library.Item) (library.Item, error) {
	saved, err := a.library.Save(it)
	return saved, a.encode("SaveItem", err)
}

// DeleteItem removes a saved item.
func (a *App) DeleteItem(id string) error { return a.encode("DeleteItem", a.library.Delete(id)) }

// ListHistory returns recent executions, newest first.
func (a *App) ListHistory(kind, search string, limit int) ([]library.Entry, error) {
	list, err := a.history.List(kind, search, limit)
	return list, a.encode("ListHistory", err)
}

// ClearHistory deletes the execution history.
func (a *App) ClearHistory() error { return a.encode("ClearHistory", a.history.Clear()) }

func (a *App) connName(connID string) string {
	if s, err := a.conns.Session(connID); err == nil {
		return s.Name
	}
	return ""
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:299] + "…"
	}
	return s
}

// record stores a history entry; failures to persist are logged, not surfaced.
func (a *App) record(e library.Entry, d time.Duration, err error) {
	// Shell code may hold passwords (createUser, changeUserPassword).
	e.Summary, e.Text = logging.RedactScript(e.Summary), logging.RedactScript(e.Text)
	e.DurationMs = d.Milliseconds()
	e.OK = err == nil
	if err != nil {
		if apperr.Is(err, apperr.CodeCanceled) || strings.Contains(err.Error(), `"code":"CANCELED"`) {
			return // canceled runs are not worth remembering
		}
		e.Error = errorMessage(err)
	}
	if herr := a.history.Add(e); herr != nil {
		a.log.Warn("could not record history", "err", herr)
	}
}

// errorMessage extracts the message from an encoded API error.
func errorMessage(err error) string {
	var ae apperr.Error
	if json.Unmarshal([]byte(err.Error()), &ae) == nil && ae.Message != "" {
		return ae.Message
	}
	return err.Error()
}

func (a *App) recordQuery(req query.Request, d time.Duration, err error) {
	summary, serr := query.ToShell(req)
	if serr != nil {
		summary = req.Filter
	}
	a.record(library.Entry{
		Kind: library.KindQuery, ConnName: a.connName(req.ConnID), DB: req.DB, Coll: req.Coll,
		Summary: oneLine(summary),
		Query:   &library.QueryPayload{Filter: req.Filter, Projection: req.Projection, Sort: req.Sort, Skip: req.Skip, Limit: req.Limit},
	}, d, err)
}

func (a *App) recordAggregation(req aggregate.Request, d time.Duration, err error) {
	ops := make([]string, 0, len(req.Stages))
	for _, s := range req.Stages {
		if s.Enabled {
			ops = append(ops, s.Operator)
		}
	}
	a.record(library.Entry{
		Kind: library.KindAggregation, ConnName: a.connName(req.ConnID), DB: req.DB, Coll: req.Coll,
		Summary: "aggregate: " + strings.Join(ops, " → "), Stages: req.Stages,
	}, d, err)
}

func (a *App) recordShell(connID, db, script string, res shell.Result, d time.Duration, err error) {
	if strings.TrimSpace(script) == "" {
		return
	}
	if err == nil {
		// The script ran, but a statement may have failed.
		for _, o := range res.Outputs {
			if o.Kind == "error" {
				err = apperr.New(apperr.CodeMongo, "%s", o.Text)
				break
			}
		}
	}
	a.record(library.Entry{Kind: library.KindScript, ConnName: a.connName(connID), DB: db,
		Summary: oneLine(script), Text: script}, d, err)
}

// RunChart groups a collection's matching documents for a chart, on the
// server (Collection › Chart view).
func (a *App) RunChart(opID string, r charts.Request) (charts.Data, error) {
	return call(a, "RunChart", opID, func(ctx context.Context) (charts.Data, error) {
		return a.charts.Run(ctx, r)
	})
}
