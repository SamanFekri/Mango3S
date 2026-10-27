package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/notify"
	"mango3s/internal/scheduler"
)

// Notifications for scheduled tasks (Tasks › a task › Notifications).

// The webhook URL of a task lives in the credential store: it often carries a
// token.
func taskWebhookSecret(taskID string) string { return "task-webhook:" + taskID }

// TaskRunPayload is what a task's webhook receives.
type TaskRunPayload struct {
	Event      string    `json:"event"` // task.finished | task.test
	App        string    `json:"app"`
	Version    string    `json:"version"`
	TaskID     string    `json:"taskId"`
	TaskName   string    `json:"taskName"`
	TaskType   string    `json:"taskType"`
	RunID      string    `json:"runId,omitempty"`
	State      string    `json:"state"`
	Trigger    string    `json:"trigger,omitempty"`
	Attempts   int       `json:"attempts"`
	Result     string    `json:"result,omitempty"`
	Errors     []string  `json:"errors"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

func runPayload(event string, t scheduler.Task, r scheduler.Run) TaskRunPayload {
	errs := r.Errors
	if errs == nil {
		errs = []string{}
	}
	return TaskRunPayload{Event: event, App: "Mango3S", Version: Version, TaskID: t.ID, TaskName: t.Name, TaskType: t.Type, RunID: r.ID,
		State: r.State, Trigger: r.Trigger, Attempts: r.Attempts, Result: r.Result, Errors: errs, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt}
}

// runSummary is the desktop notification's title and text.
func runSummary(t scheduler.Task, r scheduler.Run) (string, string) {
	switch r.State {
	case scheduler.RunSucceeded:
		return fmt.Sprintf("✓ %s finished", t.Name), firstNonEmpty(r.Result, "The task ran successfully.")
	case scheduler.RunSkipped:
		return fmt.Sprintf("%s was skipped", t.Name), firstNonEmpty(lastOf(r.Errors), "The previous run was still in progress.")
	}
	return fmt.Sprintf("✗ %s %s", t.Name, r.State), firstNonEmpty(lastOf(r.Errors), "See Tasks in Mango3S for details.")
}

func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// notifyTaskRun is the scheduler's notifier: it sends the notifications the
// task asks for. Failures are logged; they never affect the run.
func (a *App) notifyTaskRun(t scheduler.Task, r scheduler.Run) {
	if !t.Notify.Wants(r.State) {
		return
	}
	if err := a.sendTaskNotification("task.finished", t, r); err != nil {
		a.log.Warn("task notification failed", "task", t.Name, "err", err)
	}
}

func (a *App) sendTaskNotification(event string, t scheduler.Task, r scheduler.Run) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var errs []string
	if t.Notify.Desktop {
		title, msg := runSummary(t, r)
		if err := notify.Desktop(ctx, title, msg); err != nil {
			errs = append(errs, "Desktop notification: "+err.Error())
		}
	}
	if t.Notify.Webhook {
		url, err := a.secrets.Get(taskWebhookSecret(t.ID))
		switch {
		case err != nil || url == "":
			errs = append(errs, "Webhook: no URL is stored for this task.")
		default:
			if err := notify.Post(ctx, url, runPayload(event, t, r)); err != nil {
				errs = append(errs, "Webhook: "+apperr.From(err).Message)
			}
		}
	}
	if len(errs) > 0 {
		return apperr.New(apperr.CodeUnreachable, "%s", strings.Join(errs, " "))
	}
	return nil
}

// SetTaskWebhook stores a task's webhook URL in the credential store; an
// empty URL removes it.
func (a *App) SetTaskWebhook(taskID, url string) error {
	if _, err := a.p2.scheduler.Get(taskID); err != nil {
		return a.encode("SetTaskWebhook", err)
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return a.encode("SetTaskWebhook", a.secrets.Delete(taskWebhookSecret(taskID)))
	}
	if err := notify.CheckWebhookURL(url); err != nil {
		return a.encode("SetTaskWebhook", err)
	}
	return a.encode("SetTaskWebhook", a.secrets.Set(taskWebhookSecret(taskID), url))
}

// TaskWebhookHost tells the UI whether a task has a webhook and where it
// points (the host only; the full URL may contain a token).
func (a *App) TaskWebhookHost(taskID string) (string, error) {
	url, err := a.secrets.Get(taskWebhookSecret(taskID))
	if err != nil || url == "" {
		return "", nil
	}
	if i := strings.Index(url, "://"); i >= 0 {
		host := url[i+3:]
		if j := strings.IndexAny(host, "/?#"); j >= 0 {
			host = host[:j]
		}
		if k := strings.LastIndex(host, "@"); k >= 0 {
			host = host[k+1:]
		}
		return host, nil
	}
	return "", nil
}

// TestTaskNotification sends a test notification with the task's current
// settings, whatever "when" says.
func (a *App) TestTaskNotification(taskID string) error {
	t, err := a.p2.scheduler.Get(taskID)
	if err != nil {
		return a.encode("TestTaskNotification", err)
	}
	if !t.Notify.Desktop && !t.Notify.Webhook {
		return a.encode("TestTaskNotification", apperr.Invalid("Turn on a desktop notification or a webhook first."))
	}
	now := time.Now().UTC()
	r := scheduler.Run{ID: "test", TaskID: t.ID, TaskName: t.Name, Trigger: "manual", State: scheduler.RunSucceeded, Attempts: 1,
		Result: "This is a test notification from Mango3S.", Errors: []string{}, StartedAt: now, FinishedAt: now}
	return a.encode("TestTaskNotification", a.sendTaskNotification("task.test", t, r))
}
