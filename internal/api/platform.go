package api

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/update"
)

// Privacy policy (web/docs/API_CONTRACT.md §8) and About / Support links
// (§9). Like the rest of the platform check-in, both are best-effort: when the
// server can't be reached the app shows nothing new and keeps working.

// appInfoTTL is how long the About / Support links are cached (the contract
// asks for a day).
const appInfoTTL = 24 * time.Hour

// appInfoFile keeps the last links on disk, so About shows them offline too.
const appInfoFile = "platform-info.json"

type cachedAppInfo struct {
	Info      update.AppInfo `json:"info"`
	FetchedAt time.Time      `json:"fetchedAt"`
}

// refreshPrivacyPolicy fetches the policy and, once per launch, asks the user
// to accept a new version when the admin requires it.
func (a *App) refreshPrivacyPolicy(ctx context.Context, client *update.Client, token string) {
	policy, err := client.GetPrivacyPolicy(ctx, token)
	if err != nil {
		a.log.Debug("privacy policy check failed; will retry", "err", err)
		return
	}
	a.update.mu.Lock()
	a.update.privacy = &policy
	prompt := policy.AcceptanceRequired && !a.update.privacyPrompted
	if prompt {
		a.update.privacyPrompted = true
	}
	a.update.mu.Unlock()
	if prompt {
		a.update.emit("privacy:required", policy)
	}
}

// privacyPending reports whether the user still has to accept a new policy.
// Online account features wait for that (the contract allows it); local work never does.
func (a *App) privacyPending() bool {
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	return a.update.privacy != nil && a.update.privacy.AcceptanceRequired
}

// refreshAppInfo fetches the About / Support links when the cached copy is
// older than a day (or always, with force).
func (a *App) refreshAppInfo(ctx context.Context, client *update.Client, force bool) *update.AppInfo {
	a.update.mu.Lock()
	if a.update.appInfo == nil {
		var cached cachedAppInfo
		if err := config.ReadJSON(filepath.Join(a.store.Dir(), appInfoFile), &cached); err == nil && cached.Info.WebsiteURL != "" {
			a.update.appInfo, a.update.appInfoAt = &cached.Info, cached.FetchedAt
		}
	}
	fresh := a.update.appInfo != nil && time.Since(a.update.appInfoAt) < appInfoTTL
	current := a.update.appInfo
	a.update.mu.Unlock()
	if fresh && !force {
		return current
	}

	info, err := client.GetAppInfo(ctx)
	if err != nil {
		a.log.Debug("app info fetch failed; keeping the cached links", "err", err)
		return current
	}
	now := time.Now().UTC()
	a.update.mu.Lock()
	a.update.appInfo, a.update.appInfoAt = &info, now
	a.update.mu.Unlock()
	if err := config.WriteJSON(filepath.Join(a.store.Dir(), appInfoFile), cachedAppInfo{Info: info, FetchedAt: now}); err != nil {
		a.log.Debug("could not cache app info", "err", err)
	}
	return &info
}

// ---- bound methods ---------------------------------------------------------

// GetPlatformInfo returns the About / Support links (website, privacy policy,
// contact and, when the admin turned it on, a donation link). It answers from
// the cache when it can; nil means they have never been fetched.
func (a *App) GetPlatformInfo() *update.AppInfo {
	ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
	defer cancel()
	defer a.guard("app info")
	return a.refreshAppInfo(ctx, a.platformClient(), false)
}

// GetPrivacyPolicy returns the current privacy policy for this installation
// (fetched fresh when possible). nil when it can't be fetched yet.
func (a *App) GetPrivacyPolicy() *update.PrivacyPolicy {
	if token, _, ok := a.updateCredentials(); ok {
		ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
		defer cancel()
		a.refreshPrivacyPolicy(ctx, a.platformClient(), token)
	}
	a.update.mu.Lock()
	defer a.update.mu.Unlock()
	if a.update.privacy == nil {
		return nil
	}
	p := *a.update.privacy
	return &p
}

// AcceptPrivacyPolicy accepts version and returns the policy as it now
// stands. If a newer version was published in the meantime, that newer
// version comes back (still needing acceptance) instead of an error, so the
// app can show it.
func (a *App) AcceptPrivacyPolicy(version int) (*update.PrivacyPolicy, error) {
	token, _, ok := a.updateCredentials()
	if !ok {
		return nil, a.encode("AcceptPrivacyPolicy", apperr.New(apperr.CodeUnreachable, "Mango3S hasn't reached its server yet. Try again in a moment."))
	}
	ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
	defer cancel()
	client := a.platformClient()
	err := client.AcceptPrivacyPolicy(ctx, token, version)
	if err != nil && !update.IsConflict(err) {
		return nil, a.encode("AcceptPrivacyPolicy", apperr.Wrap(apperr.CodeUnreachable, err, "Couldn't save your answer. Check your connection and try again."))
	}
	policy, ferr := client.GetPrivacyPolicy(ctx, token)
	if ferr != nil {
		if err == nil {
			// Accepted; the follow-up read just failed. Record it locally.
			a.update.mu.Lock()
			if a.update.privacy != nil && a.update.privacy.Version == version {
				a.update.privacy.AcceptanceRequired = false
			}
			p := a.update.privacy
			a.update.mu.Unlock()
			return p, nil
		}
		return nil, a.encode("AcceptPrivacyPolicy", apperr.Wrap(apperr.CodeUnreachable, ferr, "A newer privacy policy was published, but it couldn't be loaded. Try again."))
	}
	a.update.mu.Lock()
	a.update.privacy = &policy
	a.update.mu.Unlock()
	if err == nil {
		a.log.Info("privacy policy accepted", "version", version)
	}
	return &policy, nil
}

// OpenPlatformLink opens a link from the platform (a notification button,
// the privacy policy, the donation page) in the default browser. Only http(s)
// links are opened.
func (a *App) OpenPlatformLink(link string) error {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return a.encode("OpenPlatformLink", apperr.Invalid("This link can't be opened."))
	}
	wruntime.BrowserOpenURL(a.ctx, u.String())
	return nil
}
