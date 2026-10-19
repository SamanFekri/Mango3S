package api

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/dump"
	"mango3s/internal/jobs"
	"mango3s/internal/mongox"
	"mango3s/internal/tools"
)

// Backup engines.
const (
	EngineBuiltin = "builtin" // Mango3S: a mongodump-compatible folder, works over SSH and proxies
	EngineTools   = "tools"   // mongodump / mongorestore (archives, users and roles, oplog)
)

// BackupRequest starts a full logical backup.
type BackupRequest struct {
	ConnID  string        `json:"connId"`
	Engine  string        `json:"engine"`
	Builtin dump.Options  `json:"builtin"`
	Tool    dump.ToolDump `json:"tool"`
}

// RestoreRequest restores a backup into a connection.
type RestoreRequest struct {
	ConnID  string              `json:"connId"`
	Engine  string              `json:"engine"`
	Builtin dump.RestoreOptions `json:"builtin"`
	Tool    dump.ToolRestore    `json:"tool"`
}

// BackupTools says whether mongodump/mongorestore can be used for a connection.
type BackupTools struct {
	Dump    tools.Info `json:"dump"`
	Restore tools.Info `json:"restore"`
	Blocked string     `json:"blocked"` // why this connection can't use them (SSH, proxy…)
}

func (a *App) profileAndCreds(connID string) (config.Profile, mongox.Credentials, error) {
	p, err := a.store.GetProfile(connID)
	if err != nil {
		return p, mongox.Credentials{}, err
	}
	c, err := a.credentials(p, nil)
	return p, c, err
}

// BackupToolsStatus reports the MongoDB Database Tools for a connection.
func (a *App) BackupToolsStatus(connID string) BackupTools {
	var out BackupTools
	p, err := a.store.GetProfile(connID)
	if err != nil {
		out.Blocked = "The connection was not found."
		return out
	}
	for _, info := range a.DetectTools(p.Tools, connID) {
		switch info.Name {
		case "mongodump":
			out.Dump = info
		case "mongorestore":
			out.Restore = info
		}
	}
	if err := dump.CanUseTools(p); err != nil {
		out.Blocked = upperFirst(err.Error()) + "."
	}
	return out
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// StartBackup queues a backup job. Backups only read, so they also run on
// read-only connections.
func (a *App) StartBackup(r BackupRequest) (jobs.Job, error) {
	sess, err := a.conns.Session(r.ConnID)
	if err != nil {
		return jobs.Job{}, a.encode("StartBackup", err)
	}
	switch r.Engine {
	case EngineBuiltin:
		o := r.Builtin
		o.Source = sess.Name
		if strings.TrimSpace(o.Dir) == "" {
			return jobs.Job{}, a.encode("StartBackup", apperr.Invalid("Choose a folder for the backup."))
		}
		title := "Back up " + scopeTitle(sess.Name, o.Selection.Databases)
		j := a.jobs.Start("backup", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
			m, err := dump.Run(ctx, sess.Client, o, p)
			docs, n := int64(0), 0
			for _, ns := range m.Namespaces {
				if ns.Error == "" {
					docs += ns.Count
					n++
				}
			}
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Backed up %d namespaces, %d documents to %s", n, docs, o.Dir), nil
		})
		a.log.Info("backup started", "job", j.ID, "engine", r.Engine, "conn", sess.Name)
		return j, nil
	case EngineTools:
		path, prof, ta, err := a.toolInvocation(r.ConnID, "mongodump", func(p config.Profile, c mongox.Credentials) (*dump.ToolArgs, error) {
			return dump.DumpArgs(p, c, r.Tool)
		})
		if err != nil {
			return jobs.Job{}, a.encode("StartBackup", err)
		}
		target := r.Tool.Archive
		if target == "" {
			target = r.Tool.Out
		}
		dbs := []string{}
		if r.Tool.DB != "" {
			dbs = append(dbs, r.Tool.DB)
		}
		j := a.jobs.Start("backup", "mongodump "+scopeTitle(prof.Name, dbs), func(ctx context.Context, p *jobs.Progress) (string, error) {
			sum, err := dump.RunTool(ctx, path, ta, p)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("mongodump saved %d documents to %s", sum.Documents, target), nil
		})
		a.log.Info("backup started", "job", j.ID, "engine", r.Engine, "conn", prof.Name)
		return j, nil
	}
	return jobs.Job{}, a.encode("StartBackup", apperr.Invalid("Unknown backup engine %q.", r.Engine))
}

func scopeTitle(conn string, dbs []string) string {
	switch len(dbs) {
	case 0:
		return conn + " (all databases)"
	case 1:
		return conn + " / " + dbs[0]
	}
	return fmt.Sprintf("%s (%d databases)", conn, len(dbs))
}

func (a *App) toolInvocation(connID, tool string, build func(config.Profile, mongox.Credentials) (*dump.ToolArgs, error)) (string, config.Profile, *dump.ToolArgs, error) {
	p, c, err := a.profileAndCreds(connID)
	if err != nil {
		return "", p, nil, err
	}
	path, err := tools.Locate(p.Tools, tool)
	if err != nil {
		return "", p, nil, apperr.New(apperr.CodeUnsupported, "%s", err.Error())
	}
	ta, err := build(p, c)
	if err != nil {
		return "", p, nil, apperr.Invalid("%s", upperFirst(err.Error())+".")
	}
	return path, p, ta, nil
}

// InspectBackup lists what a backup folder contains, without connecting.
func (a *App) InspectBackup(dir string) (dump.Inspection, error) {
	in, err := dump.Inspect(dir)
	if err != nil {
		return in, a.encode("InspectBackup", apperr.Wrap(apperr.CodeInvalidInput, err, upperFirst(err.Error())+"."))
	}
	return in, nil
}

// StartRestoreBackup queues a restore. Drop mode replaces collections, so it
// needs the connection's name typed as confirmation.
func (a *App) StartRestoreBackup(r RestoreRequest, confirm string) (jobs.Job, error) {
	sess, err := a.conns.Session(r.ConnID)
	if err != nil {
		return jobs.Job{}, a.encode("StartRestoreBackup", err)
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, a.encode("StartRestoreBackup", err)
	}
	destructive := (r.Engine == EngineBuiltin && r.Builtin.Mode == dump.ModeDrop) || (r.Engine == EngineTools && r.Tool.Drop)
	if destructive && strings.TrimSpace(confirm) != sess.Name {
		return jobs.Job{}, a.encode("StartRestoreBackup", apperr.Invalid("Dropping collections before restoring deletes their current data. Type the connection name %q to confirm.", sess.Name))
	}
	switch r.Engine {
	case EngineBuiltin:
		o := r.Builtin
		if _, err := dump.Inspect(o.Dir); err != nil {
			return jobs.Job{}, a.encode("StartRestoreBackup", apperr.Wrap(apperr.CodeInvalidInput, err, upperFirst(err.Error())+"."))
		}
		j := a.jobs.Start("restore", "Restore into "+sess.Name, func(ctx context.Context, p *jobs.Progress) (string, error) {
			rep, err := dump.Restore(ctx, sess.Client, o, p)
			var ins, rep2, dup int64
			for _, res := range rep.Results {
				ins += res.Inserted
				rep2 += res.Replaced
				dup += res.Duplicates
			}
			if err != nil {
				return "", err
			}
			msg := fmt.Sprintf("Restored %d namespaces: %d inserted", len(rep.Results), ins)
			if rep2 > 0 {
				msg += fmt.Sprintf(", %d replaced", rep2)
			}
			if dup > 0 {
				msg += fmt.Sprintf(", %d skipped (already present)", dup)
			}
			return msg, nil
		})
		a.log.Info("restore started", "job", j.ID, "engine", r.Engine, "mode", o.Mode, "conn", sess.Name)
		return j, nil
	case EngineTools:
		path, prof, ta, err := a.toolInvocation(r.ConnID, "mongorestore", func(p config.Profile, c mongox.Credentials) (*dump.ToolArgs, error) {
			return dump.RestoreArgs(p, c, r.Tool)
		})
		if err != nil {
			return jobs.Job{}, a.encode("StartRestoreBackup", err)
		}
		j := a.jobs.Start("restore", "mongorestore into "+prof.Name, func(ctx context.Context, p *jobs.Progress) (string, error) {
			sum, err := dump.RunTool(ctx, path, ta, p)
			if err != nil {
				return "", err
			}
			if r.Tool.DryRun {
				return "Dry run finished; nothing was written", nil
			}
			return fmt.Sprintf("mongorestore restored %d documents", sum.Documents), nil
		})
		a.log.Info("restore started", "job", j.ID, "engine", r.Engine, "drop", r.Tool.Drop, "conn", prof.Name)
		return j, nil
	}
	return jobs.Job{}, a.encode("StartRestoreBackup", apperr.Invalid("Unknown restore engine %q.", r.Engine))
}

// ChooseSaveFile opens a native save dialog. An empty result means canceled.
func (a *App) ChooseSaveFile(title, defaultName string) (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: title, DefaultFilename: filepath.Base(defaultName)})
	return path, a.encode("ChooseSaveFile", err)
}
