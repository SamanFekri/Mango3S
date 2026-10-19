//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mango3s/internal/api"
	"mango3s/internal/dump"
	"mango3s/internal/jobs"
	"mango3s/internal/manage"
	"mango3s/internal/scheduler"
)

// A multi-step task (export, then backup) runs through the API, and its
// definition survives export/import only as an unapproved task.
func TestMultiStepTaskAndTaskImport(t *testing.T) {
	e := setup(t)
	seed(t, e, "steps", 25)
	app, _ := newApp(t)
	p, _ := app.SaveProfile(api.ProfileInput{Name: "it", URI: mongoURI(), ConnectTimeoutSeconds: 5})
	out := t.TempDir()
	export, _ := json.Marshal(map[string]any{"connId": p.ID, "db": e.db, "coll": "steps", "path": filepath.Join(out, "steps-{date}.json"), "format": "json", "canonical": true})
	backup, _ := json.Marshal(map[string]any{"connId": p.ID, "backup": map[string]any{"dir": filepath.Join(out, "bk-{date}-{time}"), "gzip": true, "selection": map[string]any{"databases": []string{e.db}}}})
	chain, _ := json.Marshal(map[string]any{"steps": []map[string]any{
		{"name": "export", "type": "export", "params": json.RawMessage(export)},
		{"name": "backup", "type": "backup", "params": json.RawMessage(backup)},
	}})
	// Invalid steps are rejected at save time.
	bad, _ := json.Marshal(map[string]any{"steps": []map[string]any{{"type": "chain", "params": map[string]any{}}}})
	if _, err := app.SaveTask(scheduler.Task{Name: "bad", Type: "chain", Params: bad, Schedule: scheduler.Schedule{Kind: scheduler.KindCron, Cron: "0 2 * * *"}}); err == nil {
		t.Fatal("nested chain accepted")
	}
	task, err := app.SaveTask(scheduler.Task{Name: "nightly", Type: "chain", Params: chain, Schedule: scheduler.Schedule{Kind: scheduler.KindCron, Cron: "0 2 * * *"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ApproveTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RunTaskNow(task.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		runs, _ := app.ListTaskRuns(task.ID, 1)
		if len(runs) == 1 && runs[0].State != scheduler.RunRunning {
			if runs[0].State != scheduler.RunSucceeded {
				t.Fatalf("run: %+v", runs[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if files, _ := filepath.Glob(filepath.Join(out, "steps-*.json")); len(files) != 1 {
		t.Fatalf("export: %v", files)
	}
	dirs, _ := filepath.Glob(filepath.Join(out, "bk-*"))
	if len(dirs) != 1 {
		t.Fatalf("backup: %v", dirs)
	}
	if _, err := os.Stat(filepath.Join(dirs[0], e.db, "steps.bson.gz")); err != nil {
		t.Fatal(err)
	}

	// Export → import: the copy is never approved.
	js, err := app.ExportTasks([]string{task.ID})
	if err != nil || strings.Contains(js, "approved") {
		t.Fatalf("%v\n%s", err, js)
	}
	res, err := app.ImportTasks(js)
	if err != nil || len(res.Imported) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	tasks, _ := app.ListTasks()
	for _, x := range tasks {
		if x.ID != task.ID && x.ApprovedAt != nil {
			t.Fatal("imported task approved")
		}
	}
}

// Backups with the MongoDB Database Tools against a server with
// authentication: the password reaches mongodump/mongorestore only through
// the private config file and never appears in the job log.
func TestToolBackupWithAuthentication(t *testing.T) {
	uri := os.Getenv("MONGO_AUTH_URI")
	if uri == "" {
		t.Skip("MONGO_AUTH_URI not set")
	}
	if _, err := exec.LookPath("mongodump"); err != nil {
		t.Skip("mongodump not installed")
	}
	app, _ := newApp(t)
	user, pass := "admin", "change-me"
	host := uri[strings.Index(uri, "@")+1:]
	host = host[:strings.IndexAny(host, "/?")]
	p, err := app.SaveProfile(api.ProfileInput{Name: "auth", URI: "mongodb://" + host + "/?directConnection=true", Username: user, Password: pass, AuthSource: "admin", ConnectTimeoutSeconds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ConnectWithLog("op1", p.ID); err != nil {
		t.Fatal(err)
	}
	db := "mango3s_it_toolauth"
	if err := app.CreateCollectionWithOptions(p.ID, db, manage.CreateOptions{Name: "c", Validator: `{ $jsonSchema: { required: ["x"] } }`}); err != nil {
		t.Fatal(err)
	}
	defer app.DropDatabase(p.ID, db)
	defer app.DropDatabase(p.ID, db+"_restored")
	if _, err := app.InsertDocuments(p.ID, db, "c", `[{ x: 1 }, { x: 2 }, { x: 3 }]`); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "a.archive")
	j, err := app.StartBackup(api.BackupRequest{ConnID: p.ID, Engine: api.EngineTools, Tool: dump.ToolDump{Archive: archive, DB: db, UsersAndRoles: true}})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, app, j.ID)
	if done.State != jobs.Succeeded {
		t.Fatalf("backup: %+v", done)
	}
	j, err = app.StartRestoreBackup(api.RestoreRequest{ConnID: p.ID, Engine: api.EngineTools, Tool: dump.ToolRestore{Archive: archive, NsFrom: []string{db + ".*"}, NsTo: []string{db + "_restored.*"}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	restored := waitJob(t, app, j.ID)
	if restored.State != jobs.Succeeded {
		t.Fatalf("restore: %+v", restored)
	}
	for _, line := range append(done.Log, restored.Log...) {
		if strings.Contains(line, pass) {
			t.Fatalf("password in job log: %s", line)
		}
	}
	st, err := app.CollectionStats(p.ID, db+"_restored", "c")
	if err != nil || st.Count != 3 {
		t.Fatalf("%+v %v", st, err)
	}
}

func waitJob(t *testing.T, app *api.App, id string) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range app.ListJobs() {
			if j.ID == id && j.State.Finished() {
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return jobs.Job{}
}
