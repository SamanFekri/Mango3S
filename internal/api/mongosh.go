package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/library"
	"mango3s/internal/scriptrun"
	"mango3s/internal/shell"
	"mango3s/internal/tools"
)

// Shell tab › mongosh mode: full JavaScript through the installed mongosh.

// MongoshLine is pushed as "mongosh:line" while a script runs.
type MongoshLine struct {
	OpID   string `json:"opId"`
	Stream string `json:"stream"` // stdout | stderr
	Text   string `json:"text"`
}

// MongoshResult is how a run ended.
type MongoshResult struct {
	ExitCode   int   `json:"exitCode"`
	DurationMs int64 `json:"durationMs"`
}

// MongoshAvailability says whether mongosh mode can be used on a connection.
type MongoshAvailability struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Problem   string `json:"problem,omitempty"` // why not, and how to fix it
}

// mongoshRunTimeout bounds a script; Cancel stops it sooner.
const mongoshRunTimeout = 6 * time.Hour

// GetMongoshAvailability reports whether connID can run scripts with mongosh.
func (a *App) GetMongoshAvailability(connID string) MongoshAvailability {
	sess, err := a.conns.Session(connID)
	if err != nil {
		return MongoshAvailability{Problem: apperr.From(err).Message}
	}
	p, err := a.store.GetProfile(sess.ProfileID)
	if err != nil {
		return MongoshAvailability{Problem: "This connection isn't a saved connection, so mongosh can't use its settings."}
	}
	if err := scriptrun.CanRun(p); err != nil {
		return MongoshAvailability{Problem: apperr.From(err).Message}
	}
	path, err := tools.Locate(p.Tools, "mongosh")
	if err != nil {
		return MongoshAvailability{Problem: err.Error()}
	}
	return MongoshAvailability{Available: true, Path: path}
}

// RunMongosh runs script with mongosh against connID's database db. Output
// arrives as "mongosh:line" events tagged with opID; CancelOperation(opID)
// stops it. A non-zero exit code is a failed run, not an error.
func (a *App) RunMongosh(opID, connID, db, script string) (MongoshResult, error) {
	start := time.Now()
	res, err := callWithTimeout(a, "RunMongosh", opID, mongoshRunTimeout, func(ctx context.Context) (MongoshResult, error) {
		sess, err := a.conns.Session(connID)
		if err != nil {
			return MongoshResult{}, err
		}
		p, err := a.store.GetProfile(sess.ProfileID)
		if err != nil {
			return MongoshResult{}, apperr.New(apperr.CodeUnsupported, "This connection isn't a saved connection, so mongosh can't use its settings.")
		}
		if err := scriptrun.CanRun(p); err != nil {
			return MongoshResult{}, err
		}
		path, err := tools.Locate(p.Tools, "mongosh")
		if err != nil {
			return MongoshResult{}, apperr.New(apperr.CodeUnsupported, "%s", err.Error())
		}
		creds, err := a.credentials(p, nil)
		if err != nil {
			return MongoshResult{}, err
		}
		uri, secrets, err := scriptrun.ConnectionURI(p, creds, db)
		if err != nil {
			return MongoshResult{}, err
		}
		code, err := scriptrun.Run(ctx, path, uri, script, secrets, func(l scriptrun.Line) {
			if a.emitMongosh != nil {
				a.emitMongosh(MongoshLine{OpID: opID, Stream: l.Stream, Text: l.Text})
			}
		})
		if err != nil {
			return MongoshResult{}, err
		}
		return MongoshResult{ExitCode: code, DurationMs: time.Since(start).Milliseconds()}, nil
	})
	var recErr error = err
	if err == nil && res.ExitCode != 0 {
		recErr = fmt.Errorf("mongosh exited with code %d", res.ExitCode)
	}
	if strings.TrimSpace(script) != "" {
		a.record(library.Entry{Kind: library.KindScript, ConnName: a.connName(connID), DB: db, Summary: oneLine(script), Text: script}, time.Since(start), recErr)
	}
	return res, err
}

// RunJavaScript runs script as JavaScript in the built-in engine (variables,
// loops, functions; no mongosh needed). CancelOperation(opID) stops it.
func (a *App) RunJavaScript(opID, connID, db, script string) (shell.Result, error) {
	start := time.Now()
	res, err := callWithTimeout(a, "RunJavaScript", opID, mongoshRunTimeout, func(ctx context.Context) (shell.Result, error) {
		return a.js.Run(ctx, connID, db, script)
	})
	a.recordShell(connID, db, script, res, time.Since(start), err)
	return res, err
}
