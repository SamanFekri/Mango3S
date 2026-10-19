package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mango3s/internal/update"
)

func init() {
	update.RetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
}

// fakePlatform is a minimal stand-in for the web platform's /v1 API.
type fakePlatform struct {
	mu            sync.Mutex
	registrations []map[string]any
	heartbeats    int
	acks          []string
	ackAttempts   int
	notifications []update.Notification

	tokenSeq    int
	validTokens map[string]bool
	blocked     bool // every call answers 403
	down        bool // every call answers 503
	failAcks    bool
}

func (p *fakePlatform) handler(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.down {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if p.blocked {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.URL.Path == "/v1/installations/register" {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		p.registrations = append(p.registrations, body)
		p.tokenSeq++
		token := "it_" + strings.Repeat("x", p.tokenSeq)
		p.validTokens[token] = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(update.RegisterResponse{InstallationID: "inst-1", InstallationToken: token})
		return
	}
	if !p.validTokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/heartbeat"):
		p.heartbeats++
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/v1/notifications":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p.notifications)
	case strings.HasSuffix(r.URL.Path, "/ack"):
		p.ackAttempts++
		if p.failAcks {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.acks = append(p.acks, strings.Split(r.URL.Path, "/")[3])
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/v1/updates/check":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(update.UpdateCheckResult{UpdateAvailable: false})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (p *fakePlatform) set(fn func(*fakePlatform)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fn(p)
}

func withFakePlatform(t *testing.T) *fakePlatform {
	t.Helper()
	p := &fakePlatform{validTokens: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(p.handler))
	t.Cleanup(srv.Close)
	prev := PlatformBaseURL
	PlatformBaseURL = srv.URL
	t.Cleanup(func() { PlatformBaseURL = prev })
	return p
}

func TestFirstCheckRegistersAnonymouslyWithoutAKey(t *testing.T) {
	p := withFakePlatform(t)
	f := newFixture(t)

	if got := f.app.updateTick(f.app.ctx); got != tickOK {
		t.Fatalf("tick result = %v, want ok", got)
	}
	if len(p.registrations) != 1 {
		t.Fatalf("registrations = %d, want 1", len(p.registrations))
	}
	if _, sent := p.registrations[0]["clientKey"]; sent {
		t.Fatalf("the free app must not send a client key: %v", p.registrations[0])
	}
	s := f.app.currentSettings()
	if s.DeviceFingerprint == "" || s.UpdateInstallationID != "inst-1" {
		t.Fatalf("identity not saved: %+v", s)
	}
	if tok, _ := f.secrets.Get(secretInstallToken); tok == "" {
		t.Fatalf("installation token not stored")
	}
	if p.heartbeats != 1 || !f.app.GetUpdateStatus().Connected {
		t.Fatalf("heartbeats = %d, status = %+v", p.heartbeats, f.app.GetUpdateStatus())
	}

	// Later checks reuse the registration.
	f.app.updateTick(f.app.ctx)
	if len(p.registrations) != 1 || p.heartbeats != 2 {
		t.Fatalf("registrations = %d, heartbeats = %d", len(p.registrations), p.heartbeats)
	}
}

func TestPlatformDownNeverBreaksTheApp(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) { p.down = true })
	f := newFixture(t)

	if got := f.app.safeTick(f.app.ctx); got != tickFailed {
		t.Fatalf("tick result = %v, want failed (so the ticker backs off and retries)", got)
	}
	st := f.app.GetUpdateStatus()
	if st.Revoked || st.Error == "" {
		t.Fatalf("an outage must be recorded as a retryable error, not a block: %+v", st)
	}
	// "Check now" while down reports a friendly error but doesn't panic or wedge state.
	if _, err := f.app.CheckForUpdatesNow(); err == nil {
		t.Fatalf("expected an error from Check now while the platform is down")
	}

	p.set(func(p *fakePlatform) { p.down = false })
	if got := f.app.safeTick(f.app.ctx); got != tickOK {
		t.Fatalf("tick result after recovery = %v, want ok", got)
	}
	if st := f.app.GetUpdateStatus(); st.Error != "" || !st.Connected {
		t.Fatalf("status not cleared after recovery: %+v", st)
	}
}

func TestBlockedInstallationPausesAndRecovers(t *testing.T) {
	p := withFakePlatform(t)
	f := newFixture(t)
	f.app.updateTick(f.app.ctx)

	p.set(func(p *fakePlatform) { p.blocked = true })
	if got := f.app.updateTick(f.app.ctx); got != tickBlocked {
		t.Fatalf("tick result = %v, want blocked", got)
	}
	if !f.app.GetUpdateStatus().Revoked {
		t.Fatalf("expected Revoked after a 403")
	}

	p.set(func(p *fakePlatform) { p.blocked = false })
	if got := f.app.updateTick(f.app.ctx); got != tickOK || f.app.GetUpdateStatus().Revoked {
		t.Fatalf("an unblocked installation should resume: result=%v status=%+v", got, f.app.GetUpdateStatus())
	}
}

func TestInvalidatedTokenReRegistersAsTheSameDevice(t *testing.T) {
	p := withFakePlatform(t)
	f := newFixture(t)
	f.app.updateTick(f.app.ctx)
	fingerprint := f.app.currentSettings().DeviceFingerprint

	p.set(func(p *fakePlatform) { p.validTokens = map[string]bool{} }) // server forgot every token
	if got := f.app.updateTick(f.app.ctx); got != tickOK {
		t.Fatalf("tick result = %v, want ok after re-registering", got)
	}
	if len(p.registrations) != 2 || p.registrations[1]["deviceFingerprint"] != fingerprint {
		t.Fatalf("expected a second registration with the same fingerprint: %v", p.registrations)
	}
}

func TestDismissedNotificationAckIsRetriedUntilDelivered(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) {
		p.notifications = []update.Notification{{ID: "n1", Title: "Hi", Body: "There"}}
		p.failAcks = true
	})
	f := newFixture(t)
	f.app.updateTick(f.app.ctx)

	f.app.AckNotification("n1") // fails in the background; must not surface an error
	// Wait until the background attempt has exhausted its retries and put the ack back.
	waitFor(t, func() bool {
		p.mu.Lock()
		attempts := p.ackAttempts
		p.mu.Unlock()
		f.app.update.mu.Lock()
		defer f.app.update.mu.Unlock()
		return attempts >= len(update.RetryDelays)+1 && f.app.update.pendingAcks["n1"]
	})

	p.set(func(p *fakePlatform) { p.failAcks = false })
	f.app.updateTick(f.app.ctx)
	if len(p.acks) != 1 || p.acks[0] != "n1" {
		t.Fatalf("ack not retried on the next check: %v", p.acks)
	}
	f.app.update.mu.Lock()
	defer f.app.update.mu.Unlock()
	if f.app.update.pendingAcks["n1"] {
		t.Fatalf("delivered ack still pending")
	}
}

func TestSavingSettingsFromAStaleFormKeepsThePlatformIdentity(t *testing.T) {
	withFakePlatform(t)
	f := newFixture(t)
	stale := f.app.currentSettings() // loaded before registration: no identity yet
	f.app.updateTick(f.app.ctx)
	registered := f.app.currentSettings()

	stale.AutoUpdateEnabled = false
	saved, err := f.app.SaveSettings(stale)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DeviceFingerprint != registered.DeviceFingerprint || saved.UpdateInstallationID != registered.UpdateInstallationID {
		t.Fatalf("stale form wiped the identity: saved=%+v registered=%+v", saved, registered)
	}
	if saved.AutoUpdateEnabled {
		t.Fatalf("the user's own change was lost")
	}
}

func TestPlatformURLCanBeOverriddenAtRuntime(t *testing.T) {
	t.Setenv(EnvPlatformURL, "http://localhost:4000")
	if got := platformURL(); got != "http://localhost:4000" {
		t.Fatalf("platformURL() = %q", got)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met in time")
}
