package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The connection probe exercises the calls the app makes to the platform (see
// web/docs/API_CONTRACT.md) and reports each result, so a developer can see
// where a connection to a local or deployed backend breaks. Each call is a
// named check that can run on its own (RunCheck) or all in order (Probe). It
// never retries, and only "register" and "heartbeat" write anything.

// ProbeCheck describes one call the probe can make.
type ProbeCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	NeedsToken  bool   `json:"needsToken"`
	Writes      bool   `json:"writes"` // changes something on the server
	Description string `json:"description"`
}

// Check IDs, in the order Probe runs them.
const (
	CheckHealth        = "health"
	CheckAppInfo       = "app-info"
	CheckRegister      = "register"
	CheckHeartbeat     = "heartbeat"
	CheckUpdate        = "update-check"
	CheckNotifications = "notifications"
	CheckPrivacy       = "privacy"
)

// ProbeChecks lists every check.
var ProbeChecks = []ProbeCheck{
	{CheckHealth, "Server reachable", http.MethodGet, "/health", false, false, "The web API's health check: is anything answering at this URL?"},
	{CheckAppInfo, "App info", http.MethodGet, "/v1/app-info", false, false, "Public About/Support links; no token needed."},
	{CheckRegister, "Register installation", http.MethodPost, "/v1/installations/register", false, true, "Gets an installation token, as the app's first check-in does."},
	{CheckHeartbeat, "Heartbeat", http.MethodPost, "/v1/installations/{id}/heartbeat", true, true, "Reports activity; the app sends one every 10 minutes."},
	{CheckUpdate, "Update check", http.MethodGet, "/v1/updates/check", true, false, "Asks whether a newer release exists for this OS and architecture."},
	{CheckNotifications, "Notifications", http.MethodGet, "/v1/notifications", true, false, "Lists pending announcements (read only: nothing is acknowledged)."},
	{CheckPrivacy, "Privacy policy", http.MethodGet, "/v1/policies/privacy", true, false, "The current policy and whether this installation accepted it."},
}

// FindCheck returns the check with id.
func FindCheck(id string) (ProbeCheck, bool) {
	for _, c := range ProbeChecks {
		if c.ID == id {
			return c, true
		}
	}
	return ProbeCheck{}, false
}

// ProbeOptions says what the probe may do.
type ProbeOptions struct {
	// Token and InstallationID are an existing registration (the app's own);
	// without them the authenticated checks are skipped.
	Token          string
	InstallationID string
	// Register performs the "register" check (POST /v1/installations/register)
	// and returns the new registration. Probe runs it only when set; RunCheck
	// fills Token and InstallationID from it.
	Register func(ctx context.Context, c *Client) (token, installationID string, err error)
	// Heartbeat makes Probe include the heartbeat check.
	Heartbeat bool
	// For the update check.
	OS, Arch, Version string
	// StepTimeout bounds each check (default 10s).
	StepTimeout time.Duration
}

// ProbeStep is the outcome of one check.
type ProbeStep struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	OK         bool   `json:"ok"`
	Skipped    bool   `json:"skipped"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"durationMs"`
	Detail     string `json:"detail"`
	Error      string `json:"error,omitempty"`
}

// ProbeReport is a run of one or more checks.
type ProbeReport struct {
	BaseURL string      `json:"baseUrl"`
	OK      bool        `json:"ok"` // every check that ran succeeded
	Steps   []ProbeStep `json:"steps"`
}

// Probe runs every check in order: the public ones, "register" when
// o.Register is set, "heartbeat" when o.Heartbeat is set, then the
// authenticated ones. A failed health check skips the rest.
func (c *Client) Probe(ctx context.Context, o ProbeOptions) ProbeReport {
	rep := ProbeReport{BaseURL: c.BaseURL, OK: true}
	reachable := true
	for _, chk := range ProbeChecks {
		if chk.ID == CheckRegister && o.Register == nil || chk.ID == CheckHeartbeat && !o.Heartbeat {
			continue
		}
		var s ProbeStep
		if reachable {
			s = c.RunCheck(ctx, chk.ID, &o)
		} else {
			s = skipped(chk, "server not reachable")
		}
		if chk.ID == CheckHealth && !s.OK {
			reachable = false
		}
		if !s.OK && !s.Skipped {
			rep.OK = false
		}
		rep.Steps = append(rep.Steps, s)
	}
	return rep
}

// RunCheck runs the single check id. A successful "register" stores the new
// registration in o, so later checks with the same o use it.
func (c *Client) RunCheck(ctx context.Context, id string, o *ProbeOptions) ProbeStep {
	chk, ok := FindCheck(id)
	if !ok {
		return ProbeStep{ID: id, Name: id, Error: "unknown check " + fmt.Sprintf("%q", id)}
	}
	if chk.NeedsToken && o.Token == "" {
		return skipped(chk, "no installation token: run the register check first (the app also registers on its first check-in)")
	}
	if chk.ID == CheckHeartbeat && o.InstallationID == "" {
		return skipped(chk, "no installation ID: run the register check first")
	}
	if chk.ID == CheckRegister && o.Register == nil {
		return skipped(chk, "registration isn't available here")
	}
	fast := &Client{BaseURL: c.BaseURL, NoRetry: true}
	timeout := o.StepTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	sctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	path := chk.Path
	if chk.ID == CheckHeartbeat {
		path = "/v1/installations/" + o.InstallationID + "/heartbeat"
	}
	start := time.Now()
	detail, err := fast.runCheck(sctx, chk.ID, o)
	s := ProbeStep{ID: chk.ID, Name: chk.Name, Method: chk.Method, Path: path, DurationMs: time.Since(start).Milliseconds(), Detail: detail, OK: err == nil, Status: http.StatusOK}
	if err != nil {
		s.Status, s.Error = 0, explainProbeError(err)
		if se, ok := err.(*StatusError); ok {
			s.Status = se.Status
		}
	}
	return s
}

func (c *Client) runCheck(ctx context.Context, id string, o *ProbeOptions) (string, error) {
	switch id {
	case CheckHealth:
		return c.health(ctx)
	case CheckAppInfo:
		info, err := c.GetAppInfo(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("name %q, website %s", info.Name, orNone(info.WebsiteURL)), nil
	case CheckRegister:
		token, installationID, err := o.Register(ctx, c)
		if err != nil {
			return "", err
		}
		o.Token, o.InstallationID = token, installationID
		return "installation " + installationID, nil
	case CheckHeartbeat:
		err := c.Heartbeat(ctx, o.InstallationID, o.Token, HeartbeatRequest{AppVersion: o.Version, OS: o.OS, Arch: o.Arch})
		return "recorded", err
	case CheckUpdate:
		r, err := c.CheckForUpdate(ctx, o.Token, o.OS, o.Arch, "stable", o.Version)
		if err != nil {
			return "", err
		}
		if r.UpdateAvailable && r.Latest != nil {
			return fmt.Sprintf("update available: %s (%s)", r.Latest.Version, orNone(r.Latest.Artifact.FileName)), nil
		}
		return "up to date (" + o.Version + ")", nil
	case CheckNotifications:
		ns, err := c.ListNotifications(ctx, o.Token)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d pending (read only, nothing acknowledged)", len(ns)), nil
	case CheckPrivacy:
		p, err := c.GetPrivacyPolicy(ctx, o.Token)
		if err != nil {
			return "", err
		}
		state := "accepted"
		if p.AcceptanceRequired {
			state = "acceptance required"
		} else if p.AcceptedAt == nil {
			state = "not accepted"
		}
		return fmt.Sprintf("version %d, %s", p.Version, state), nil
	}
	return "", fmt.Errorf("unknown check %q", id)
}

func skipped(chk ProbeCheck, why string) ProbeStep {
	return ProbeStep{ID: chk.ID, Name: chk.Name, Method: chk.Method, Path: chk.Path, Skipped: true, Detail: why}
}

func (c *Client) health(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return "", err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode >= 300 {
		return "", newStatusError(resp.StatusCode, body)
	}
	return "HTTP " + http.StatusText(resp.StatusCode) + oneLine(body), nil
}

// explainProbeError turns common failures into a hint a developer can act on.
func explainProbeError(err error) string {
	msg := err.Error()
	switch se, ok := err.(*StatusError); {
	case ok && se.Status == http.StatusUnauthorized:
		return msg + " — the installation token was rejected (the app re-registers on its next check-in)"
	case ok && se.Status == http.StatusForbidden:
		return msg + " — an admin blocked this installation"
	case ok && se.Status == http.StatusNotFound:
		return msg + " — endpoint missing: is the URL the API (port 4330 locally) rather than the dashboard (5330)?"
	case ok && se.Status == http.StatusTooManyRequests:
		return msg + " — rate limited"
	case strings.Contains(msg, "connection refused"):
		return msg + " — nothing is listening there: is the web API running (`pnpm run dev:api` in web/)?"
	case strings.Contains(msg, "no such host"):
		return msg + " — the host name doesn't resolve"
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "Timeout"):
		return msg + " — no answer in time"
	case strings.Contains(msg, "certificate"):
		return msg + " — TLS certificate problem"
	}
	return msg
}

func oneLine(body []byte) string {
	s := strings.Join(strings.Fields(string(body)), " ")
	if s == "" {
		return ""
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return ": " + s
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
