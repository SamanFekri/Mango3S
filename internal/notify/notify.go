// Package notify tells the user how a scheduled task went: a desktop
// notification through the operating system, and a JSON POST to a webhook.
// Wails v2 has no notification API, so desktop notifications run osascript
// (macOS), notify-send (Linux) or PowerShell (Windows). The text is always
// passed as data (arguments or environment), never spliced into a script.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"mango3s/internal/apperr"
)

// Command runs a program; replaced in tests.
var Command = func(ctx context.Context, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(out))
	}
	return nil
}

// windowsToast shows a toast through the Windows Runtime API. Title and
// message come from environment variables, so no quoting is involved.
const windowsToast = `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$t = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$x = $t.GetElementsByTagName('text')
$x.Item(0).AppendChild($t.CreateTextNode($env:M3S_TITLE)) > $null
$x.Item(1).AppendChild($t.CreateTextNode($env:M3S_MESSAGE)) > $null
$app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show([Windows.UI.Notifications.ToastNotification]::new($t))`

// Desktop shows a notification on this computer.
func Desktop(ctx context.Context, title, message string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	switch runtime.GOOS {
	case "darwin":
		return Command(ctx, nil, "osascript",
			"-e", "on run argv", "-e", "display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run", title, message)
	case "windows":
		return Command(ctx, []string{"M3S_TITLE=" + title, "M3S_MESSAGE=" + message},
			"powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", windowsToast)
	default:
		return Command(ctx, nil, "notify-send", "--app-name=Mango3S", "--", title, message)
	}
}

// CheckWebhookURL accepts only absolute http(s) URLs.
func CheckWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return apperr.Invalid("The webhook must be an http:// or https:// URL.")
	}
	return nil
}

// Post sends payload as JSON to a webhook. Any 2xx answer counts as
// delivered. Errors never contain the URL (it may hold a token), only its
// host.
func Post(ctx context.Context, rawURL string, payload any) error {
	if err := CheckWebhookURL(rawURL); err != nil {
		return err
	}
	u, _ := url.Parse(rawURL)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return apperr.Invalid("The webhook URL is not valid.")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mango3S")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return apperr.New(apperr.CodeUnreachable, "The webhook at %s could not be reached.", u.Host)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return apperr.New(apperr.CodeUnreachable, "The webhook at %s answered %s.", u.Host, res.Status)
	}
	return nil
}
