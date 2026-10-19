package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/dump"
	"mango3s/internal/jobs"
	"mango3s/internal/scheduler"
)

// chainTask runs other task types one after another. The steps' parameters
// are part of the chain's own parameters, so approving the chain approves
// exactly these steps, and editing any of them asks for approval again.
type chainTask struct {
	Steps           []chainStep `json:"steps"`
	ContinueOnError bool        `json:"continueOnError"`
}

type chainStep struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

const maxChainSteps = 20

// backupTask is a scheduled built-in backup. {date} and {time} in the folder
// are replaced at run time, so each run writes a new folder.
type backupTask struct {
	ConnID string       `json:"connId"`
	Backup dump.Options `json:"backup"`
}

func (a *App) registerChainTask() {
	s := a.p2.scheduler
	s.Register(scheduler.TaskType{
		ID: "backup", Label: "Backup", Description: "Back up databases into a new mongodump-compatible folder. Put {date} and {time} in the folder name so each run gets its own folder.",
		Validate: func(raw json.RawMessage) error {
			t, err := decode[backupTask](raw)
			if err != nil {
				return err
			}
			if t.ConnID == "" || strings.TrimSpace(t.Backup.Dir) == "" {
				return apperr.Invalid("Backup tasks need a connection and a folder.")
			}
			if !strings.Contains(t.Backup.Dir, "{date}") && !strings.Contains(t.Backup.Dir, "{time}") {
				return apperr.Invalid("Put {date} or {time} in the backup folder so each run writes a new folder (backups never overwrite each other).")
			}
			if info, err := os.Stat(filepath.Dir(t.Backup.Dir)); err != nil || !info.IsDir() {
				return apperr.Invalid("The folder %q does not exist.", filepath.Dir(t.Backup.Dir))
			}
			return nil
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			t, err := decode[backupTask](raw)
			if err != nil {
				return "", err
			}
			sess, err := a.ensureConnected(ctx, t.ConnID)
			if err != nil {
				return "", err
			}
			o := t.Backup
			o.Dir = expandPath(o.Dir, time.Now())
			o.Source = sess.Name
			m, err := dump.Run(ctx, sess.Client, o, p)
			if err != nil {
				return "", err
			}
			var docs int64
			for _, ns := range m.Namespaces {
				docs += ns.Count
			}
			return fmt.Sprintf("Backed up %d namespaces, %d documents to %s", len(m.Namespaces), docs, o.Dir), nil
		},
	})
	check := func(raw json.RawMessage) (chainTask, []scheduler.TaskType, error) {
		c, err := decode[chainTask](raw)
		if err != nil {
			return c, nil, err
		}
		if len(c.Steps) == 0 {
			return c, nil, apperr.Invalid("Add at least one step.")
		}
		if len(c.Steps) > maxChainSteps {
			return c, nil, apperr.Invalid("A multi-step task can have at most %d steps.", maxChainSteps)
		}
		types := make([]scheduler.TaskType, len(c.Steps))
		for i, st := range c.Steps {
			if st.Type == "chain" {
				return c, nil, apperr.Invalid("Step %d: multi-step tasks can't contain other multi-step tasks.", i+1)
			}
			tt, ok := s.Type(st.Type)
			if !ok {
				return c, nil, apperr.Invalid("Step %d: task type %q is not allowed.", i+1, st.Type)
			}
			params := st.Params
			if len(params) == 0 {
				params = json.RawMessage("{}")
			}
			if tt.Validate != nil {
				if err := tt.Validate(params); err != nil {
					return c, nil, apperr.Invalid("Step %d (%s): %s", i+1, stepName(st, i), apperr.From(err).Message)
				}
			}
			c.Steps[i].Params = params
			types[i] = tt
		}
		return c, types, nil
	}
	s.Register(scheduler.TaskType{
		ID: "chain", Label: "Multi-step", Description: "Run several steps (export, compare, sync, masked copy, backup…) in order. Stops at the first failed step unless told to continue.",
		Validate: func(raw json.RawMessage) error {
			_, _, err := check(raw)
			return err
		},
		Run: func(ctx context.Context, raw json.RawMessage, p *jobs.Progress) (string, error) {
			c, types, err := check(raw)
			if err != nil {
				return "", err
			}
			var results, failures []string
			for i, st := range c.Steps {
				if err := ctx.Err(); err != nil {
					return "", err
				}
				name := stepName(st, i)
				p.Log("Step %d/%d: %s", i+1, len(c.Steps), name)
				res, err := types[i].Run(ctx, st.Params, p)
				if err != nil {
					p.Log("Step %d failed: %s", i+1, apperr.From(err).Message)
					failures = append(failures, fmt.Sprintf("step %d (%s): %s", i+1, name, apperr.From(err).Message))
					if !c.ContinueOnError {
						return "", apperr.New(apperr.From(err).Code, "Step %d of %d (%s) failed: %s", i+1, len(c.Steps), name, apperr.From(err).Message)
					}
					continue
				}
				p.Log("Step %d done: %s", i+1, res)
				results = append(results, fmt.Sprintf("%d. %s", i+1, res))
			}
			if len(failures) > 0 {
				return "", fmt.Errorf("%d of %d steps failed: %s", len(failures), len(c.Steps), strings.Join(failures, "; "))
			}
			return strings.Join(results, " · "), nil
		},
	})
}

func stepName(st chainStep, i int) string {
	if strings.TrimSpace(st.Name) != "" {
		return st.Name
	}
	if st.Type != "" {
		return st.Type
	}
	return fmt.Sprintf("step %d", i+1)
}

// ExportTasks returns task definitions as JSON for saving (all when ids is
// empty). Approvals are never exported.
func (a *App) ExportTasks(ids []string) (string, error) {
	b, _, err := a.p2.scheduler.ExportDefinitions(ids)
	return string(b), a.encode("ExportTasks", err)
}

// ImportTasks adds task definitions from a file's content. Every imported
// task starts unapproved, so nothing runs until it is reviewed here.
func (a *App) ImportTasks(content string) (scheduler.ImportResult, error) {
	r, err := a.p2.scheduler.ImportDefinitions([]byte(content))
	if err == nil {
		a.log.Info("tasks imported", "count", len(r.Imported), "errors", len(r.Errors))
	}
	return r, a.encode("ImportTasks", err)
}
