package api

import (
	"context"
	"os"
	"strings"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/update"
)

// Developer tools for the platform connection (Settings › Diagnostics ›
// Platform connection, and `mango3s platform-check`): which server this build
// talks to, a step-by-step connection test, and — when MANGO3S_PLATFORM_TRACE
// is set, as `make dev` does — a log of every request the app sends to it.

// EnvTestMode turns on test mode ("1", "true", "yes" or "on"), which `make
// dev` sets: platform requests are logged, and the API console can send any
// request to the web API. EnvPlatformTrace turns on only the logging.
const (
	EnvTestMode      = "MANGO3S_TEST_MODE"
	EnvPlatformTrace = "MANGO3S_PLATFORM_TRACE"
)

func envOn(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// TestMode reports whether the app runs in test mode.
func TestMode() bool { return envOn(EnvTestMode) }

const platformTraceSize = 300

// PlatformTraceRequested reports whether platform requests should be logged.
func PlatformTraceRequested() bool { return TestMode() || envOn(EnvPlatformTrace) }

// PlatformURL is the platform base URL in effect (MANGO3S_PLATFORM_URL wins
// over the built-in PlatformBaseURL).
func PlatformURL() string { return platformURL() }

// InstallationCredentials returns a's stored platform registration (empty
// when not registered), for `mango3s platform-check`.
func InstallationCredentials(a *App) (token, installationID string) {
	token, installationID, _ = a.updateCredentials()
	return token, installationID
}

// startPlatformTrace records platform requests when the environment asks for
// it, logging each one and pushing it to the Diagnostics page ("platform:trace").
func (a *App) startPlatformTrace() {
	if !PlatformTraceRequested() {
		return
	}
	update.EnableTrace(platformTraceSize, func(e update.TraceEntry) {
		args := []any{"method", e.Method, "url", e.URL, "status", e.Status, "ms", e.DurationMs}
		if e.Error != "" {
			args = append(args, "err", e.Error)
		}
		a.log.Info("platform request", args...)
		a.update.emit("platform:trace", e)
	})
	a.log.Info("platform request tracing on", "url", platformURL())
}

// PlatformDebug describes the platform connection for the Diagnostics page.
type PlatformDebug struct {
	BaseURL        string              `json:"baseUrl"`
	BaseURLSource  string              `json:"baseUrlSource"` // "built-in" or "MANGO3S_PLATFORM_URL"
	TraceEnabled   bool                `json:"traceEnabled"`
	Registered     bool                `json:"registered"`
	InstallationID string              `json:"installationId"`
	Status         update.Status       `json:"status"`
	Requests       []update.TraceEntry `json:"requests"`
}

// GetPlatformDebug returns the platform target, registration state and (when
// tracing is on) the recorded requests.
func (a *App) GetPlatformDebug() PlatformDebug {
	source := "built-in"
	if strings.TrimSpace(os.Getenv(EnvPlatformURL)) != "" {
		source = EnvPlatformURL
	}
	_, id, ok := a.updateCredentials()
	return PlatformDebug{
		BaseURL:        platformURL(),
		BaseURLSource:  source,
		TraceEnabled:   update.TraceEnabled(),
		Registered:     ok,
		InstallationID: id,
		Status:         a.update.tracker.Get(),
		Requests:       update.TraceEntries(),
	}
}

// ClearPlatformTrace forgets the recorded requests.
func (a *App) ClearPlatformTrace() { update.ClearTrace() }

// GetPlatformChecks lists the platform API checks that can be run one by one.
func (a *App) GetPlatformChecks() []update.ProbeCheck { return update.ProbeChecks }

// RunPlatformCheck runs the single check id with this installation's
// registration. "register" always sends the request — re-registering returns
// the same installation with a fresh token, which is kept.
func (a *App) RunPlatformCheck(id string) (update.ProbeStep, error) {
	defer a.guard("RunPlatformCheck")
	if _, ok := update.FindCheck(id); !ok {
		return update.ProbeStep{}, a.encode("RunPlatformCheck", apperr.Invalid("Unknown check: %s", id))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opts := a.probeOptions()
	opts.Register = func(ctx context.Context, c *update.Client) (string, string, error) {
		a.update.regMu.Lock()
		defer a.update.regMu.Unlock()
		return a.register(ctx, c)
	}
	return a.platformClient().RunCheck(ctx, id, &opts), nil
}

func (a *App) probeOptions() update.ProbeOptions {
	opts := update.ProbeOptions{OS: update.HostOS(), Arch: update.HostArch(), Version: Version, Heartbeat: true}
	if token, id, ok := a.updateCredentials(); ok {
		opts.Token, opts.InstallationID = token, id
	}
	return opts
}

// TestPlatformConnection runs the connection probe with this installation's
// registration. With register set and no registration yet, it first registers
// exactly as the app's own check-in would (the token is kept).
func (a *App) TestPlatformConnection(register bool) (update.ProbeReport, error) {
	defer a.guard("TestPlatformConnection")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	opts := a.probeOptions()
	if opts.Token == "" && register {
		opts.Register = a.ensureRegistered
	}
	return a.platformClient().Probe(ctx, opts), nil
}

// ---- API console (test mode only) ----------------------------------------

// APIConsole is the console's state for the Diagnostics page.
type APIConsole struct {
	Enabled   bool              `json:"enabled"`
	BaseURL   string            `json:"baseUrl"`
	Endpoints []update.Endpoint `json:"endpoints"`
	Vars      map[string]string `json:"vars"`
	Secret    []string          `json:"secret"` // variables to show masked
}

func (a *App) apiConsole() *update.Console {
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	if a.update.console == nil {
		a.update.console = update.NewConsole(a.consoleDefaults())
	}
	return a.update.console
}

// consoleDefaults fills the console from this installation: OS names as the
// platform spells them, the app version and its registration.
func (a *App) consoleDefaults() map[string]string {
	vars := map[string]string{
		"os": update.HostOS(), "arch": update.HostArch(), "appVersion": Version,
		"deviceFingerprint": a.currentSettings().DeviceFingerprint,
	}
	if vars["deviceFingerprint"] == "" {
		vars["deviceFingerprint"] = "api-console-" + update.NewDeviceFingerprint()
	}
	if token, id, ok := a.updateCredentials(); ok {
		vars["installationToken"], vars["installationId"] = token, id
	}
	return vars
}

// ConsoleDefaults returns a's starting console variables, for
// `mango3s platform-api`.
func ConsoleDefaults(a *App) map[string]string { return a.consoleDefaults() }

// GetAPIConsole returns every web API endpoint and the console's variables.
// Outside test mode it reports Enabled: false and nothing else.
func (a *App) GetAPIConsole() APIConsole {
	if !TestMode() {
		return APIConsole{Endpoints: []update.Endpoint{}, Vars: map[string]string{}, Secret: []string{}}
	}
	secret := make([]string, 0, len(update.SecretVars))
	for k := range update.SecretVars {
		secret = append(secret, k)
	}
	return APIConsole{Enabled: true, BaseURL: platformURL(), Endpoints: update.APICatalog, Vars: a.apiConsole().Vars(), Secret: secret}
}

// SetAPIConsoleVar sets one console variable and returns them all.
func (a *App) SetAPIConsoleVar(name, value string) (map[string]string, error) {
	if !TestMode() {
		return nil, a.encode("SetAPIConsoleVar", apperr.Invalid("The API console is only available in test mode (make dev)."))
	}
	c := a.apiConsole()
	c.SetVar(name, value)
	return c.Vars(), nil
}

// ResetAPIConsole starts the variables over from this installation.
func (a *App) ResetAPIConsole() (map[string]string, error) {
	if !TestMode() {
		return nil, a.encode("ResetAPIConsole", apperr.Invalid("The API console is only available in test mode (make dev)."))
	}
	a.update.mu.Lock()
	a.update.console = nil
	a.update.mu.Unlock()
	return a.apiConsole().Vars(), nil
}

// SendAPIRequest sends one request to the web API (test mode only).
func (a *App) SendAPIRequest(req update.ConsoleRequest) (update.ConsoleResponse, error) {
	if !TestMode() {
		return update.ConsoleResponse{}, a.encode("SendAPIRequest", apperr.Invalid("The API console is only available in test mode (make dev)."))
	}
	defer a.guard("SendAPIRequest")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return a.apiConsole().Send(ctx, platformURL(), req), nil
}
