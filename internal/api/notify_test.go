package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mango3s/internal/jobs"
	"mango3s/internal/scheduler"
)

// Roadmap 1.4: a task's webhook URL lives in the credential store, the UI
// only sees its host, and finished runs are posted to it.
func TestTaskWebhook(t *testing.T) {
	f := newFixture(t)
	got := make(chan TaskRunPayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p TaskRunPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		got <- p
	}))
	defer srv.Close()

	f.app.p2.scheduler.Register(scheduler.TaskType{ID: "fail", Run: func(context.Context, json.RawMessage, *jobs.Progress) (string, error) {
		return "", context.DeadlineExceeded
	}})
	task, err := f.app.SaveTask(scheduler.Task{Name: "nightly", Type: "fail", Enabled: true, Schedule: scheduler.Schedule{Kind: scheduler.KindInterval, EveryMinutes: 60},
		Notify: scheduler.Notify{On: scheduler.NotifyFailure, Webhook: true}})
	if err != nil {
		t.Fatal(err)
	}

	if err := f.app.SetTaskWebhook(task.ID, "ftp://example.com"); err == nil {
		t.Fatal("non-http webhook accepted")
	}
	url := strings.Replace(srv.URL, "http://", "http://user:pw@", 1) + "/hooks/abc?token=s3cret"
	if err := f.app.SetTaskWebhook(task.ID, url); err != nil {
		t.Fatal(err)
	}
	host, _ := f.app.TaskWebhookHost(task.ID)
	if host != strings.TrimPrefix(srv.URL, "http://") {
		t.Fatalf("host %q", host)
	}
	if b, _ := json.Marshal(task); strings.Contains(string(b), "s3cret") {
		t.Fatal("the webhook URL is in the task")
	}

	if err := f.app.TestTaskNotification(task.ID); err != nil {
		t.Fatal(err)
	}
	if p := <-got; p.Event != "task.test" || p.TaskName != "nightly" {
		t.Fatalf("test payload: %+v", p)
	}

	if _, err := f.app.ApproveTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RunTaskNow(task.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-got:
		if p.Event != "task.finished" || p.State != scheduler.RunFailed || len(p.Errors) == 0 || p.TaskType != "fail" {
			t.Fatalf("run payload: %+v", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the failed run was not posted")
	}

	if err := f.app.DeleteTask(task.ID); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.secrets.Get(taskWebhookSecret(task.ID)); v != "" {
		t.Fatal("the webhook URL outlived its task")
	}
}
