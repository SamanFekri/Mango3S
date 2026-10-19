package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/dump"
	"mango3s/internal/jobs"
	"mango3s/internal/manage"
	"mango3s/internal/mongox"
	"mango3s/internal/sqlexport"
)

// ---- database & collection management ---------------------------------------

func (a *App) writableSession(connID string) (*mongox.Session, error) {
	s, err := a.conns.Session(connID)
	if err != nil {
		return nil, err
	}
	return s, s.RequireWritable()
}

// CreateCollectionWithOptions creates a collection with options (capped,
// collation, validator, time series, clustered, TTL).
func (a *App) CreateCollectionWithOptions(connID, db string, o manage.CreateOptions) error {
	_, err := call(a, "CreateCollectionWithOptions", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.CreateCollection(ctx, s.Client, db, o)
	})
	if err == nil {
		a.log.Info("collection created", "db", db, "coll", o.Name)
	}
	return err
}

// CreateView creates a view from an aggregation pipeline.
func (a *App) CreateView(connID, db, name, viewOn, pipeline, collation string) error {
	_, err := call(a, "CreateView", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.CreateView(ctx, s.Client, db, name, viewOn, pipeline, collation)
	})
	if err == nil {
		a.log.Info("view created", "db", db, "view", name, "on", viewOn)
	}
	return err
}

// RenameCollection renames or moves a collection within a server. Replacing
// an existing target (dropTarget) needs the target typed as confirmation.
func (a *App) RenameCollection(connID, db, from, toDB, to string, dropTarget bool, confirm string) error {
	if toDB == "" {
		toDB = db
	}
	if dropTarget && strings.TrimSpace(confirm) != toDB+"."+to {
		return a.encode("RenameCollection", apperr.Invalid("Replacing %s.%s deletes it. Type %s.%s to confirm.", toDB, to, toDB, to))
	}
	_, err := call(a, "RenameCollection", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.RenameCollection(ctx, s.Client, db, from, toDB, to, dropTarget)
	})
	if err == nil {
		a.log.Info("collection renamed", "from", db+"."+from, "to", toDB+"."+to, "dropTarget", dropTarget)
	}
	return err
}

// StartRenameDatabase moves every collection and view of a database to a new
// name, as a background job. The database name must be typed to confirm.
func (a *App) StartRenameDatabase(connID, db, to, confirm string) (jobs.Job, error) {
	if strings.TrimSpace(confirm) != db {
		return jobs.Job{}, a.encode("StartRenameDatabase", apperr.Invalid("Type the database name %q to confirm.", db))
	}
	s, err := a.writableSession(connID)
	if err != nil {
		return jobs.Job{}, a.encode("StartRenameDatabase", err)
	}
	j := a.jobs.Start("rename", "Rename database "+db+" → "+to, func(ctx context.Context, p *jobs.Progress) (string, error) {
		if err := manage.RenameDatabase(ctx, s.Client, db, to, p); err != nil {
			return "", err
		}
		return fmt.Sprintf("Database %s is now %s", db, to), nil
	})
	a.log.Info("database rename started", "db", db, "to", to, "job", j.ID)
	return j, nil
}

// ClearCollection deletes every document but keeps the collection, its
// indexes and options. The namespace must be typed to confirm.
func (a *App) ClearCollection(connID, db, coll, confirm string) (int64, error) {
	if strings.TrimSpace(confirm) != db+"."+coll {
		return 0, a.encode("ClearCollection", apperr.Invalid("Type %s.%s to confirm.", db, coll))
	}
	n, err := call(a, "ClearCollection", "", func(ctx context.Context) (int64, error) {
		s, c, err := mongox.Collection(a.conns, connID, db, coll)
		if err != nil {
			return 0, err
		}
		if err := s.RequireWrite(ctx, db, coll, mongox.ActionRemove); err != nil {
			return 0, err
		}
		return manage.ClearCollection(ctx, c)
	})
	if err == nil {
		a.log.Info("collection cleared", "db", db, "coll", coll, "deleted", n)
	}
	return n, err
}

// CopyRequest copies collections or a whole database, possibly to another
// open connection.
type CopyRequest struct {
	SrcConn string           `json:"srcConn"`
	DstConn string           `json:"dstConn"`
	Options dump.CopyOptions `json:"options"`
}

// StartCopy queues a copy job. Drop mode replaces target collections and
// needs the target database name typed as confirmation.
func (a *App) StartCopy(r CopyRequest, confirm string) (jobs.Job, error) {
	src, err := a.conns.Session(r.SrcConn)
	if err != nil {
		return jobs.Job{}, a.encode("StartCopy", err)
	}
	dst, err := a.writableSession(r.DstConn)
	if err != nil {
		return jobs.Job{}, a.encode("StartCopy", err)
	}
	o := r.Options
	if o.DstDB == "" {
		o.DstDB = o.SrcDB
	}
	if o.Mode == dump.ModeDrop && strings.TrimSpace(confirm) != o.DstDB {
		return jobs.Job{}, a.encode("StartCopy", apperr.Invalid("Dropping target collections deletes their data. Type the target database name %q to confirm.", o.DstDB))
	}
	what := o.SrcDB
	if len(o.Collections) == 1 {
		what += "." + o.Collections[0]
	} else if len(o.Collections) > 1 {
		what += fmt.Sprintf(" (%d collections)", len(o.Collections))
	}
	target := o.DstDB
	if o.DstColl != "" {
		target += "." + o.DstColl
	}
	if r.DstConn != r.SrcConn {
		target = dst.Name + " / " + target
	}
	j := a.jobs.Start("copy", "Copy "+what+" → "+target, func(ctx context.Context, p *jobs.Progress) (string, error) {
		rep, err := dump.Copy(ctx, src.Client, dst.Client, r.SrcConn == r.DstConn, o, p)
		if err != nil {
			return "", err
		}
		var n int64
		for _, res := range rep.Results {
			n += res.Inserted + res.Replaced
		}
		return fmt.Sprintf("Copied %d namespaces, %d documents", len(rep.Results), n), nil
	})
	a.log.Info("copy started", "job", j.ID, "src", src.Name, "dst", dst.Name, "srcDb", o.SrcDB, "dstDb", o.DstDB, "mode", o.Mode)
	return j, nil
}

// GetValidation reads a collection's validation rules.
func (a *App) GetValidation(connID, db, coll string) (manage.Validation, error) {
	return call(a, "GetValidation", "", func(ctx context.Context) (manage.Validation, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return manage.Validation{}, err
		}
		return manage.GetValidation(ctx, s.Client.Database(db), coll)
	})
}

// CheckValidator counts existing documents a validator would reject.
func (a *App) CheckValidator(connID, db, coll, validator string) (manage.ValidationReport, error) {
	return call(a, "CheckValidator", "", func(ctx context.Context) (manage.ValidationReport, error) {
		_, c, err := mongox.Collection(a.conns, connID, db, coll)
		if err != nil {
			return manage.ValidationReport{}, err
		}
		return manage.CheckValidator(ctx, c, validator)
	})
}

// SetValidation replaces a collection's validation rules.
func (a *App) SetValidation(connID, db, coll string, v manage.Validation) error {
	_, err := call(a, "SetValidation", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.SetValidation(ctx, s.Client.Database(db), coll, v)
	})
	if err == nil {
		a.log.Info("validation changed", "db", db, "coll", coll, "level", v.Level, "action", v.Action)
	}
	return err
}

// ListIndexDetails returns indexes with options, sizes and usage.
func (a *App) ListIndexDetails(connID, db, coll string) ([]manage.Index, error) {
	return call(a, "ListIndexDetails", "", func(ctx context.Context) ([]manage.Index, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return nil, err
		}
		return manage.ListIndexes(ctx, s.Client.Database(db), coll)
	})
}

// CreateIndex builds an index. Large collections can take a while; the
// operation can be canceled with its opID.
func (a *App) CreateIndex(opID, connID, db, coll string, o manage.IndexOptions) (string, error) {
	name, err := callWithTimeout(a, "CreateIndex", opID, 24*time.Hour, func(ctx context.Context) (string, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return "", err
		}
		return manage.CreateIndex(ctx, s.Client.Database(db), coll, o)
	})
	if err == nil {
		a.log.Info("index created", "db", db, "coll", coll, "index", name)
	}
	return name, err
}

// DropIndex drops an index.
func (a *App) DropIndex(connID, db, coll, name string) error {
	_, err := call(a, "DropIndex", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.DropIndex(ctx, s.Client.Database(db), coll, name)
	})
	if err == nil {
		a.log.Info("index dropped", "db", db, "coll", coll, "index", name)
	}
	return err
}

// IndexChange hides/unhides an index or changes its TTL.
type IndexChange struct {
	Hidden *bool  `json:"hidden"`
	TTL    *int64 `json:"ttl"`
}

// ModifyIndex applies an IndexChange.
func (a *App) ModifyIndex(connID, db, coll, name string, ch IndexChange) error {
	_, err := call(a, "ModifyIndex", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.ModifyIndex(ctx, s.Client.Database(db), coll, name, ch.Hidden, ch.TTL)
	})
	if err == nil {
		a.log.Info("index modified", "db", db, "coll", coll, "index", name)
	}
	return err
}

// CollectionDetails returns options, metadata and statistics.
func (a *App) CollectionDetails(connID, db, coll string) (manage.CollectionDetails, error) {
	return call(a, "CollectionDetails", "", func(ctx context.Context) (manage.CollectionDetails, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return manage.CollectionDetails{}, err
		}
		return manage.Details(ctx, s.Client.Database(db), coll)
	})
}

// DatabaseDetails returns dbStats.
func (a *App) DatabaseDetails(connID, db string) (manage.DatabaseDetails, error) {
	return call(a, "DatabaseDetails", "", func(ctx context.Context) (manage.DatabaseDetails, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return manage.DatabaseDetails{}, err
		}
		return manage.DBDetails(ctx, s.Client.Database(db))
	})
}

// ---- query profiler ------------------------------------------------------------

// GetProfiling reads a database's profiler level.
func (a *App) GetProfiling(connID, db string) (manage.Profiling, error) {
	return call(a, "GetProfiling", "", func(ctx context.Context) (manage.Profiling, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return manage.Profiling{}, err
		}
		return manage.GetProfiling(ctx, s.Client.Database(db))
	})
}

// SetProfiling changes a database's profiler level. It changes server state,
// so read-only connections can't use it.
func (a *App) SetProfiling(connID, db string, p manage.Profiling) error {
	_, err := call(a, "SetProfiling", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.SetProfiling(ctx, s.Client.Database(db), p)
	})
	if err == nil {
		a.log.Info("profiling changed", "db", db, "level", p.Level, "slowms", p.SlowMS)
	}
	return err
}

// ListProfile reads profiled operations, newest first.
func (a *App) ListProfile(connID, db string, q manage.ProfileQuery) ([]manage.ProfileEntry, error) {
	return call(a, "ListProfile", "", func(ctx context.Context) ([]manage.ProfileEntry, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return nil, err
		}
		return manage.ListProfile(ctx, s.Client.Database(db), q)
	})
}

// ClearProfile empties a database's profiler data.
func (a *App) ClearProfile(connID, db string) error {
	_, err := call(a, "ClearProfile", "", func(ctx context.Context) (struct{}, error) {
		s, err := a.writableSession(connID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, manage.ClearProfile(ctx, s.Client.Database(db))
	})
	return err
}

// ---- MongoDB → SQL ----------------------------------------------------------------

// SQLExportRequest writes a collection as an SQL script.
type SQLExportRequest struct {
	ConnID  string            `json:"connId"`
	DB      string            `json:"db"`
	Coll    string            `json:"coll"`
	Path    string            `json:"path"`
	Options sqlexport.Options `json:"options"`
}

// StartSQLExport queues an SQL export job. The file must not exist yet.
func (a *App) StartSQLExport(r SQLExportRequest) (jobs.Job, error) {
	_, c, err := mongox.Collection(a.conns, r.ConnID, r.DB, r.Coll)
	if err != nil {
		return jobs.Job{}, a.encode("StartSQLExport", err)
	}
	if strings.TrimSpace(r.Path) == "" {
		return jobs.Job{}, a.encode("StartSQLExport", apperr.Invalid("Choose where to save the SQL file."))
	}
	f, err := os.OpenFile(r.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return jobs.Job{}, a.encode("StartSQLExport", apperr.Invalid("%s already exists; choose a new file name.", filepath.Base(r.Path)))
	}
	if err != nil {
		return jobs.Job{}, a.encode("StartSQLExport", apperr.Wrap(apperr.CodeInvalidInput, err, "The file can't be created."))
	}
	j := a.jobs.Start("export", "SQL export "+r.DB+"."+r.Coll, func(ctx context.Context, p *jobs.Progress) (string, error) {
		rep, err := sqlexport.Export(ctx, c, f, r.Options, p)
		cerr := f.Close()
		if err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(r.Path) // never leave a half-written script that looks complete
			return "", err
		}
		for _, n := range rep.Notes {
			p.Log("%s", n)
		}
		return fmt.Sprintf("Wrote %d rows, %d columns to %s", rep.Rows, len(rep.Columns)+1, r.Path), nil
	})
	a.log.Info("sql export started", "job", j.ID, "db", r.DB, "coll", r.Coll, "dialect", r.Options.Dialect)
	return j, nil
}

// AnalyzeIndexes runs the index advisor on a database (or one collection):
// query shapes from the profiler with suggested indexes, and indexes that
// are unused or redundant. It only reads.
func (a *App) AnalyzeIndexes(opID, connID, db string, o manage.AdviceOptions) (manage.Advice, error) {
	return call(a, "AnalyzeIndexes", opID, func(ctx context.Context) (manage.Advice, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return manage.Advice{}, err
		}
		return manage.Analyze(ctx, s.Client.Database(db), o)
	})
}
