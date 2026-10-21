package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/jobs"
	"mango3s/internal/update"
)

// PlatformBaseURL is the Mango3S web platform this build checks in with for
// updates and announcements (see web/docs/API_CONTRACT.md in the companion
// repo). Change it here, at build time with
// `make build PLATFORM_BASE_URL=https://...`, or at runtime with the
// MANGO3S_PLATFORM_URL environment variable (handy for pointing a dev build at
// a local API, e.g. MANGO3S_PLATFORM_URL=http://localhost:4330).
var PlatformBaseURL = "https://mango3s.com"

// EnvPlatformURL overrides PlatformBaseURL at runtime.
const EnvPlatformURL = "MANGO3S_PLATFORM_URL"

// Secret-store ID of this installation's token (see package secrets — same
// mechanism as connection passwords).
const secretInstallToken = "update:installationToken"

// The platform is optional: Mango3S works fully without it, so nothing here may
// block, slow down or crash the app. Every call runs in the background, failures
// are retried (per request by package update, then by rescheduling the whole
// check sooner), and the user only ever sees something when there is an update
// or an announcement for them.
const (
	firstCheckDelay      = 5 * time.Second
	updateTickInterval   = 10 * time.Minute // also the heartbeat cadence the admin's "online" view expects
	revokedCheckInterval = time.Hour        // blocked installs check rarely, in case an admin unblocks them
	requestTimeout       = 90 * time.Second // covers a request plus its retries
	userRequestTimeout   = 20 * time.Second // the user is waiting on "Check now"
)

// failureBackoff is how soon to try again after a check that couldn't reach
// the platform at all; after the last step it settles at updateTickInterval.
var failureBackoff = []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute}

type tickResult int

const (
	tickOK tickResult = iota
	tickFailed
	tickBlocked
)

// updateState holds everything the platform check-in needs beyond what lives
// on App. emit is a no-op until Lifecycle.Startup installs the Wails emitter
// (same pattern as App.emitStep), so none of this touches the Wails runtime
// outside a running desktop app — e.g. in tests.
type updateState struct {
	tracker *update.Tracker
	emit    func(event string, data any)

	regMu sync.Mutex // one registration at a time (background check vs. "Check now")

	mu          sync.Mutex
	downloading bool
	notifiedIDs map[string]bool // shown this session, so a later check doesn't show them again
	// pendingResponses are notifications the user answered (button ID, "close"
	// for the X, or "" for a plain ack) that the platform hasn't confirmed
	// yet; retried on each check.
	pendingResponses map[string]string

	// Privacy policy and About/Support links (see platform.go).
	privacy         *update.PrivacyPolicy
	privacyPrompted bool // asked once per launch
	appInfo         *update.AppInfo
	appInfoAt       time.Time

	acct accountState // optional Mango3S account (account.go)

	console *update.Console // API console variables (test mode; platformdebug.go)
}

func newUpdateState() *updateState {
	return &updateState{
		tracker:          update.NewTracker(Version),
		emit:             func(string, any) {},
		notifiedIDs:      map[string]bool{},
		pendingResponses: map[string]string{},
		acct:             accountState{disabled: map[string]bool{}},
	}
}

func platformURL() string {
	if v := strings.TrimSpace(os.Getenv(EnvPlatformURL)); v != "" {
		return v
	}
	return PlatformBaseURL
}

func (a *App) platformClient() *update.Client { return update.New(platformURL()) }

func (a *App) emitUpdateStatus() { a.update.emit("update:status", a.update.tracker.Get()) }

// guard keeps a panic in platform code from taking down the IDE: background
// goroutines have no caller to recover for them, and an unrecovered panic in
// any goroutine exits the whole process.
func (a *App) guard(where string) {
	if r := recover(); r != nil {
		a.log.Error("platform check-in panicked; continuing without it", "where", where, "panic", r, "stack", string(debug.Stack()))
	}
}

// startUpdateTicker checks in shortly after startup, then every 10 minutes,
// sooner after a failure. It never blocks startup.
func (a *App) startUpdateTicker(ctx context.Context) {
	a.update.emit = func(event string, data any) { wruntime.EventsEmit(ctx, event, data) }
	go func() {
		defer a.guard("ticker")
		delay, failures := firstCheckDelay, 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			switch a.safeTick(ctx) {
			case tickOK:
				delay, failures = updateTickInterval, 0
			case tickBlocked:
				delay, failures = revokedCheckInterval, 0
			default:
				delay = updateTickInterval
				if failures < len(failureBackoff) {
					delay = failureBackoff[failures]
				}
				failures++
			}
		}
	}()
}

func (a *App) safeTick(ctx context.Context) (result tickResult) {
	result = tickFailed
	defer a.guard("check-in")
	return a.updateTick(ctx)
}

// updateTick registers if needed, sends the heartbeat, delivers pending acks
// and new notifications, and (if enabled) checks for an update.
func (a *App) updateTick(ctx context.Context) tickResult {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	client := a.platformClient()

	token, installationID, err := a.ensureRegistered(ctx, client)
	if err != nil {
		return a.noteFailure("register", err)
	}
	hb := update.HeartbeatRequest{AppVersion: Version, OS: update.HostOS(), Arch: update.HostArch()}
	err = client.Heartbeat(ctx, installationID, token, hb)
	if update.IsUnauthorized(err) {
		// The token was invalidated server-side: register again (same
		// fingerprint, so it's the same installation) and retry once.
		a.forgetRegistration()
		if token, installationID, err = a.ensureRegistered(ctx, client); err == nil {
			err = client.Heartbeat(ctx, installationID, token, hb)
		}
	}
	if err != nil {
		return a.noteFailure("heartbeat", err)
	}
	a.markReachable()

	a.flushPendingResponses(ctx, client, token)
	if r := a.pollNotifications(ctx, client, token); r == tickBlocked {
		return r
	}
	a.refreshPrivacyPolicy(ctx, client, token)
	a.refreshAppInfo(ctx, client, false)
	a.backgroundSync(ctx, client)
	if a.currentSettings().AutoUpdateEnabled {
		if r := a.checkAndMaybeDownload(ctx, client, token); r == tickBlocked {
			return r
		}
	}
	return tickOK
}

// ensureRegistered returns this installation's credentials, registering it
// anonymously first if needed (Mango3S is free — there is no key).
func (a *App) ensureRegistered(ctx context.Context, client *update.Client) (token, installationID string, err error) {
	if token, id, ok := a.updateCredentials(); ok {
		return token, id, nil
	}
	a.update.regMu.Lock()
	defer a.update.regMu.Unlock()
	if token, id, ok := a.updateCredentials(); ok { // registered while we waited
		return token, id, nil
	}
	return a.register(ctx, client)
}

// register registers this installation (re-registering returns the same
// installation with a fresh token) and stores the credentials. Callers hold
// regMu.
func (a *App) register(ctx context.Context, client *update.Client) (token, installationID string, err error) {
	fingerprint := a.currentSettings().DeviceFingerprint
	if fingerprint == "" {
		// Persist the identity before using it, so a crash mid-registration
		// can't make the next launch look like a new installation.
		fingerprint = update.NewDeviceFingerprint()
		if err := a.mutateSettings(func(s *config.Settings) { s.DeviceFingerprint = fingerprint }); err != nil {
			return "", "", err
		}
	}

	resp, err := client.Register(ctx, update.RegisterRequest{
		DeviceFingerprint: fingerprint, AppVersion: Version, OS: update.HostOS(), Arch: update.HostArch(),
	})
	if err != nil {
		return "", "", err
	}
	if err := a.secrets.Set(secretInstallToken, resp.InstallationToken); err != nil {
		return "", "", err
	}
	if err := a.mutateSettings(func(s *config.Settings) { s.UpdateInstallationID = resp.InstallationID }); err != nil {
		return "", "", err
	}
	a.log.Info("registered with the Mango3S platform", "installation", resp.InstallationID)
	return resp.InstallationToken, resp.InstallationID, nil
}

// updateCredentials returns the stored installation token and ID, if registered.
func (a *App) updateCredentials() (token, installationID string, ok bool) {
	installationID = a.currentSettings().UpdateInstallationID
	if installationID == "" {
		return "", "", false
	}
	token, err := a.secrets.Get(secretInstallToken)
	if err != nil || token == "" {
		return "", "", false
	}
	return token, installationID, true
}

func (a *App) forgetRegistration() {
	_ = a.secrets.Delete(secretInstallToken)
	_ = a.mutateSettings(func(s *config.Settings) { s.UpdateInstallationID = "" })
}

// noteFailure records why a check didn't complete. A 403 means an admin
// blocked this installation: updates and announcements pause (the app itself
// is unaffected) until a later check finds it unblocked.
func (a *App) noteFailure(op string, err error) tickResult {
	if update.IsForbidden(err) {
		if !a.update.tracker.Get().Revoked {
			a.log.Warn("this installation was blocked by the platform admin; updates and announcements are paused", "op", op)
		}
		a.update.tracker.Update(func(s *update.Status) {
			s.Revoked = true
			s.Error = ""
		})
		a.emitUpdateStatus()
		return tickBlocked
	}
	a.log.Debug("platform check-in failed; will retry", "op", op, "err", err)
	a.update.tracker.Update(func(s *update.Status) { s.Error = err.Error() })
	a.emitUpdateStatus()
	return tickFailed
}

func (a *App) markReachable() {
	prev := a.update.tracker.Get()
	next := a.update.tracker.Update(func(s *update.Status) {
		s.Connected, s.Revoked, s.Error = true, false, ""
	})
	if prev.Connected != next.Connected || prev.Revoked != next.Revoked || prev.Error != next.Error {
		a.emitUpdateStatus()
	}
}

func (a *App) pollNotifications(ctx context.Context, client *update.Client, token string) tickResult {
	notifications, err := client.ListNotifications(ctx, token)
	if err != nil {
		return a.noteFailure("notifications", err)
	}
	a.update.mu.Lock()
	fresh := make([]update.Notification, 0, len(notifications))
	for _, n := range notifications {
		if !a.update.notifiedIDs[n.ID] {
			a.update.notifiedIDs[n.ID] = true
			fresh = append(fresh, n)
		}
	}
	a.update.mu.Unlock()
	for _, n := range fresh {
		a.update.emit("notifications:new", n)
	}
	return tickOK
}

// flushPendingResponses claims every pending response before sending it (so
// two concurrent flushes never send the same one) and puts back the ones that
// failed transiently, for the next check. A 4xx (e.g. the notification was
// deleted, or the button no longer exists) is final and dropped.
func (a *App) flushPendingResponses(ctx context.Context, client *update.Client, token string) {
	a.update.mu.Lock()
	pending := make(map[string]string, len(a.update.pendingResponses))
	for id, button := range a.update.pendingResponses {
		pending[id] = button
		delete(a.update.pendingResponses, id)
	}
	a.update.mu.Unlock()
	for id, button := range pending {
		var err error
		if button == "" {
			err = client.Ack(ctx, token, id)
		} else {
			err = client.Respond(ctx, token, id, button)
		}
		if update.IsTransient(err) || update.IsUnauthorized(err) {
			a.update.mu.Lock()
			if _, newer := a.update.pendingResponses[id]; !newer {
				a.update.pendingResponses[id] = button
			}
			a.update.mu.Unlock()
		}
	}
}

func (a *App) checkAndMaybeDownload(ctx context.Context, client *update.Client, token string) tickResult {
	a.update.mu.Lock()
	busy := a.update.downloading
	a.update.mu.Unlock()
	if busy {
		return tickOK
	}
	if path, _ := a.update.tracker.PendingArtifact(); path != "" {
		return tickOK // already downloaded and waiting for the user to install it
	}

	result, err := client.CheckForUpdate(ctx, token, update.HostOS(), update.HostArch(), "stable", Version)
	if err != nil {
		return a.noteFailure("update check", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if !result.UpdateAvailable || result.Latest == nil {
		a.update.tracker.Update(func(s *update.Status) {
			s.State, s.LastCheckedAt, s.LatestVersion, s.ReleaseNotes = update.StateUpToDate, now, "", ""
		})
		a.emitUpdateStatus()
		return tickOK
	}

	latest := *result.Latest
	a.update.mu.Lock()
	a.update.downloading = true
	a.update.mu.Unlock()
	a.update.tracker.Update(func(s *update.Status) {
		s.State, s.LastCheckedAt = update.StateDownloading, now
		s.LatestVersion, s.ReleaseNotes = latest.Version, latest.Notes
	})
	a.emitUpdateStatus()

	format := artifactFormat(latest.Artifact.FileName)
	dest := filepath.Join(os.TempDir(), "mango3s-update-"+latest.Version+"-"+filepath.Base(latest.Artifact.FileName))
	j := a.jobs.Start("update", fmt.Sprintf("Downloading Mango3S %s", latest.Version), func(jctx context.Context, p *jobs.Progress) (string, error) {
		p.SetTotal(latest.Artifact.SizeBytes, "bytes")
		err := client.DownloadArtifact(jctx, token, latest.Artifact.DownloadURL, dest, latest.Artifact.SHA256, func(done, _ int64) {
			p.Set(done)
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Mango3S %s is ready to install", latest.Version), nil
	})
	a.log.Info("update download started", "job", j.ID, "version", latest.Version)
	go a.awaitDownload(j.ID, dest, format)
	return tickOK
}

// awaitDownload waits for the download job (jobs.Manager has no wait API),
// then moves the status to ready-to-install, or back to "update available" on
// failure so the next check simply tries again.
func (a *App) awaitDownload(jobID, dest, format string) {
	defer a.guard("download")
	defer func() {
		a.update.mu.Lock()
		a.update.downloading = false
		a.update.mu.Unlock()
	}()
	for {
		time.Sleep(500 * time.Millisecond)
		snap, ok := a.jobs.Get(jobID)
		if !ok {
			return // pruned from the job list; the next check starts over
		}
		if !snap.State.Finished() {
			continue
		}
		if snap.State != jobs.Succeeded {
			a.log.Info("update download did not finish; will retry on the next check", "state", snap.State, "err", snap.Error)
			a.update.tracker.Update(func(s *update.Status) { s.State, s.Error = update.StateUpdateAvailable, snap.Error })
			a.emitUpdateStatus()
			return
		}
		a.update.tracker.SetPendingArtifact(dest, format)
		a.update.tracker.Update(func(s *update.Status) { s.State, s.Error = update.StateReadyToInstall, "" })
		a.emitUpdateStatus()
		return
	}
}

func artifactFormat(fileName string) string {
	lower := strings.ToLower(fileName)
	if strings.HasSuffix(lower, ".tar.gz") {
		return "tar.gz"
	}
	return strings.TrimPrefix(filepath.Ext(lower), ".")
}

// ---- bound methods ---------------------------------------------------------

// GetUpdateStatus returns the current platform check-in / update state.
func (a *App) GetUpdateStatus() update.Status { return a.update.tracker.Get() }

// CheckForUpdatesNow checks immediately (Settings → "Check for updates now").
// It works whether or not automatic updates are on, and registers first if
// this installation hasn't been able to yet.
func (a *App) CheckForUpdatesNow() (update.Status, error) {
	ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
	defer cancel()
	client := a.platformClient()
	token, _, err := a.ensureRegistered(ctx, client)
	if err == nil {
		a.markReachable()
		if a.checkAndMaybeDownload(ctx, client, token) == tickFailed {
			err = fmt.Errorf("%s", a.update.tracker.Get().Error)
		}
	} else {
		a.noteFailure("check now", err)
	}
	if err != nil && !update.IsForbidden(err) {
		return a.update.tracker.Get(), a.encode("CheckForUpdatesNow", apperr.Wrap(apperr.CodeUnreachable, err,
			"Couldn't reach the Mango3S update server. Mango3S keeps working normally and will try again automatically."))
	}
	return a.update.tracker.Get(), nil
}

// ApplyPendingUpdate installs a downloaded, verified update. For a
// single-executable artifact this replaces the running binary and returns
// true: the frontend then quits so the new version runs on next launch. For an
// installer/bundle artifact it opens the platform's own installer and returns
// false, leaving Mango3S running.
func (a *App) ApplyPendingUpdate() (bool, error) {
	path, format := a.update.tracker.PendingArtifact()
	if path == "" {
		return false, a.encode("ApplyPendingUpdate", apperr.Invalid("No update is ready to install."))
	}
	a.update.tracker.Update(func(s *update.Status) { s.State = update.StateInstalling })
	a.emitUpdateStatus()

	restartRequired, err := update.Apply(path, format)
	a.update.tracker.ClearPendingArtifact()
	if err != nil {
		a.update.tracker.Update(func(s *update.Status) { s.State, s.Error = update.StateError, err.Error() })
		a.emitUpdateStatus()
		return false, a.encode("ApplyPendingUpdate", apperr.Wrap(apperr.CodeInternal, err, "The update could not be installed."))
	}
	a.log.Info("update applied", "format", format, "restartRequired", restartRequired)
	return restartRequired, nil
}

// FetchNotifications returns active, unacknowledged notifications. They also
// arrive by event after each background check; failures just return nothing.
func (a *App) FetchNotifications() []update.Notification {
	token, _, ok := a.updateCredentials()
	if !ok {
		return []update.Notification{}
	}
	ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
	defer cancel()
	list, err := a.platformClient().ListNotifications(ctx, token)
	if err != nil {
		return []update.Notification{}
	}
	a.update.mu.Lock()
	for _, n := range list {
		a.update.notifiedIDs[n.ID] = true
	}
	a.update.mu.Unlock()
	return list
}

// AckNotification marks a notification read without a response. Prefer
// RespondNotification, which also records what the user chose.
func (a *App) AckNotification(id string) { a.queueResponse(id, "") }

// RespondNotification reports the button the user pressed on a notification
// ("close" for its X), which also marks it read. If the platform can't be
// reached the response is kept and retried on later checks — the user already
// moved on, so this never reports an error.
func (a *App) RespondNotification(id, buttonID string) {
	if strings.TrimSpace(buttonID) == "" {
		buttonID = update.CloseButtonID
	}
	a.queueResponse(id, buttonID)
}

func (a *App) queueResponse(id, buttonID string) {
	a.update.mu.Lock()
	a.update.pendingResponses[id] = buttonID
	a.update.mu.Unlock()
	token, _, ok := a.updateCredentials()
	if !ok {
		return
	}
	client := a.platformClient()
	go func() {
		defer a.guard("respond")
		ctx, cancel := context.WithTimeout(a.ctx, requestTimeout)
		defer cancel()
		a.flushPendingResponses(ctx, client, token)
	}()
}
