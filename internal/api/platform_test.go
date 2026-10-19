package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"mango3s/internal/config"
	"mango3s/internal/update"
)

// fakeExtras holds the fake platform's privacy policy, app info and account
// server state. Guarded by fakePlatform.mu.
type fakeExtras struct {
	responses       []string // "<notification>:<button>"
	policy          update.PrivacyPolicy
	policyAccepts   []int
	appInfo         *update.AppInfo
	appInfoCalls    int
	registrationOff bool

	accounts      map[string]*fakeAccount // by email
	accountTokens map[string]string       // ca_ token -> email
	codes         map[string]string       // email -> code
	installHeader []string                // X-Installation-Token values seen
}

type fakeAccount struct {
	password string
	items    map[string]update.SyncItem
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": msg}})
}

// extraNoInstallToken serves app info and the account API (which use no or
// an account token). Called with p.mu held.
func (p *fakePlatform) extraNoInstallToken(w http.ResponseWriter, r *http.Request) bool {
	e := &p.ext
	if r.URL.Path == "/v1/app-info" {
		e.appInfoCalls++
		if e.appInfo == nil {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		writeJSON(w, 200, e.appInfo)
		return true
	}
	if !strings.HasPrefix(r.URL.Path, "/v1/account") {
		return false
	}
	if e.accounts == nil {
		e.accounts, e.accountTokens, e.codes = map[string]*fakeAccount{}, map[string]string{}, map[string]string{}
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	str := func(k string) string { v, _ := body[k].(string); return v }
	session := func(email string) {
		p.tokenSeq++
		token := "ca_" + strings.Repeat("y", p.tokenSeq)
		e.accountTokens[token] = email
		writeJSON(w, 200, update.AccountSession{AccountToken: token, Account: update.Account{Email: email, Sync: update.AccountSync{Enabled: true}}, RecoveryKey: "RK"})
	}
	if h := r.Header.Get("X-Installation-Token"); h != "" {
		e.installHeader = append(e.installHeader, h)
	}
	switch strings.TrimPrefix(r.URL.Path, "/v1/account") {
	case "/register":
		if e.registrationOff {
			apiError(w, 403, "registration_disabled", "New sign-ups are turned off right now.")
			return true
		}
		e.codes[str("email")] = "123456"
		writeJSON(w, 202, update.CodeSent{CodeSentTo: str("email"), ExpiresInMinutes: 15})
		return true
	case "/register/verify":
		if e.codes[str("email")] != str("code") {
			apiError(w, 400, "invalid_code", "That code is wrong or has expired.")
			return true
		}
		e.accounts[str("email")] = &fakeAccount{password: str("password"), items: map[string]update.SyncItem{}}
		session(str("email"))
		return true
	case "/login":
		a := e.accounts[str("email")]
		if a == nil || a.password != str("password") {
			apiError(w, 400, "invalid_credentials", "Email or password is not correct.")
			return true
		}
		session(str("email"))
		return true
	}
	email, ok := e.accountTokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	if !ok {
		apiError(w, 401, "unauthorized", "Sign in again")
		return true
	}
	acct := e.accounts[email]
	switch strings.TrimPrefix(r.URL.Path, "/v1/account") {
	case "":
		writeJSON(w, 200, update.Account{Email: email, Sync: update.AccountSync{Enabled: true, ItemCount: len(acct.items)}})
	case "/logout":
		w.WriteHeader(204)
	case "/sync/items":
		var push struct {
			Items []update.SyncItem `json:"items"`
		}
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &push)
		res := update.SyncPushResult{Accepted: []string{}, Skipped: []string{}}
		for _, it := range push.Items {
			if cur, ok := acct.items[it.Key]; ok && cur.UpdatedAt >= it.UpdatedAt {
				res.Skipped = append(res.Skipped, it.Key)
				continue
			}
			acct.items[it.Key] = it
			res.Accepted = append(res.Accepted, it.Key)
		}
		writeJSON(w, 200, res)
	case "/sync/pull":
		if str("password") != acct.password {
			apiError(w, 400, "wrong_password", "That password is not correct.")
			return true
		}
		items := []update.SyncItem{}
		for _, it := range acct.items {
			items = append(items, it)
		}
		writeJSON(w, 200, map[string]any{"items": items})
	default:
		w.WriteHeader(404)
	}
	return true
}

// extraWithInstallToken serves notification responses and the privacy policy.
func (p *fakePlatform) extraWithInstallToken(w http.ResponseWriter, r *http.Request) bool {
	e := &p.ext
	switch {
	case strings.HasSuffix(r.URL.Path, "/respond"):
		var body struct {
			ButtonID string `json:"buttonId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.responses = append(e.responses, strings.Split(r.URL.Path, "/")[3]+":"+body.ButtonID)
		w.WriteHeader(204)
	case r.URL.Path == "/v1/policies/privacy":
		if e.policy.Version == 0 && e.policy.Title == "" {
			w.WriteHeader(404)
			return true
		}
		writeJSON(w, 200, e.policy)
	case r.URL.Path == "/v1/policies/privacy/accept":
		var body struct {
			Version int `json:"version"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Version != e.policy.Version {
			apiError(w, 409, "conflict", "A newer version was published")
			return true
		}
		e.policyAccepts = append(e.policyAccepts, body.Version)
		e.policy.AcceptanceRequired = false
		w.WriteHeader(204)
	default:
		return false
	}
	return true
}

// recordEvents captures what the backend would emit to the frontend.
func recordEvents(a *App) func(name string) []any {
	var mu sync.Mutex
	got := map[string][]any{}
	a.update.emit = func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()
		got[event] = append(got[event], data)
	}
	return func(name string) []any {
		mu.Lock()
		defer mu.Unlock()
		return append([]any(nil), got[name]...)
	}
}

func TestNotificationButtonsAndCloseAreReported(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) {
		p.notifications = []update.Notification{{ID: "n1", Title: "Survey", Buttons: []update.NotificationButton{
			{ID: "b1", Label: "Take it", Action: update.ActionOpenLink, URL: "https://example.com", Style: "primary"},
		}}, {ID: "n2", Title: "News"}}
	})
	f := newFixture(t)
	events := recordEvents(f.app)
	f.app.updateTick(f.app.ctx)

	shown := events("notifications:new")
	if len(shown) != 2 || len(shown[0].(update.Notification).Buttons) != 1 {
		t.Fatalf("notifications with buttons not passed on: %+v", shown)
	}
	f.app.RespondNotification("n1", "b1")
	f.app.RespondNotification("n2", "")
	waitFor(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.ext.responses) == 2
	})
	p.mu.Lock()
	got := strings.Join(p.ext.responses, ",")
	p.mu.Unlock()
	if !strings.Contains(got, "n1:b1") || !strings.Contains(got, "n2:close") {
		t.Fatalf("responses = %s", got)
	}
}

func TestPrivacyPolicyPromptsOncePerLaunchAndHandlesANewerVersion(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) {
		p.ext.policy = update.PrivacyPolicy{Version: 2, Title: "Privacy policy", Body: "## Hi", AcceptanceRequired: true}
	})
	f := newFixture(t)
	events := recordEvents(f.app)
	f.app.updateTick(f.app.ctx)
	f.app.updateTick(f.app.ctx)
	if n := len(events("privacy:required")); n != 1 {
		t.Fatalf("prompted %d times, want once per launch", n)
	}
	if !f.app.privacyPending() {
		t.Fatal("policy should be pending")
	}

	// A newer version appears before the user accepts v2: the newer one comes back.
	p.set(func(p *fakePlatform) { p.ext.policy.Version = 3 })
	got, err := f.app.AcceptPrivacyPolicy(2)
	if err != nil || got == nil || got.Version != 3 || !got.AcceptanceRequired {
		t.Fatalf("after conflict: %+v, %v", got, err)
	}
	got, err = f.app.AcceptPrivacyPolicy(3)
	if err != nil || got.AcceptanceRequired || f.app.privacyPending() {
		t.Fatalf("after accept: %+v, %v", got, err)
	}
}

func TestPlatformInfoIsCachedForADayAndSurvivesARestart(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) {
		p.ext.appInfo = &update.AppInfo{Name: "Mango3S", WebsiteURL: "https://mango3s.com", Donation: &update.Donation{URL: "https://donate.example", ButtonLabel: "Donate"}}
	})
	f := newFixture(t)
	if info := f.app.GetPlatformInfo(); info == nil || info.Donation == nil || info.Donation.URL != "https://donate.example" {
		t.Fatalf("info = %+v", info)
	}
	f.app.GetPlatformInfo()
	p.mu.Lock()
	calls := p.ext.appInfoCalls
	p.mu.Unlock()
	if calls != 1 {
		t.Fatalf("app info fetched %d times, want 1 (cached)", calls)
	}

	// A new App on the same config directory, with the server down, still has the links.
	p.set(func(p *fakePlatform) { p.down = true })
	f2 := f
	f2.app = New(Deps{Log: f.app.log, Store: config.NewStore(f.dir), Secrets: f.secrets, Conns: f.app.conns})
	if info := f2.app.GetPlatformInfo(); info == nil || info.WebsiteURL != "https://mango3s.com" {
		t.Fatalf("cached info lost: %+v", info)
	}
}

func TestOpenPlatformLinkOnlyOpensWebLinks(t *testing.T) {
	f := newFixture(t)
	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "mailto:x@y", ""} {
		if err := f.app.OpenPlatformLink(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}

func TestPasswordRulesMatchTheServer(t *testing.T) {
	cases := map[string]bool{
		"Correct-Horse-9": true,
		"short1A!":        false,
		"alllowercase1":   false, // only two classes
		"Ada.Lovelace-1":  false, // contains the email name
		"Paaaaass-word1":  false,
		"Mango3S-secret!": false,
	}
	for pw, ok := range cases {
		if got := len(PasswordProblems(pw, "ada.lovelace@example.com")) == 0; got != ok {
			t.Errorf("%q: ok=%v, want %v (%v)", pw, got, ok, PasswordProblems(pw, "ada.lovelace@example.com"))
		}
	}
}

func TestSignUpThenSyncConnectionsToASecondComputer(t *testing.T) {
	p := withFakePlatform(t)
	laptop := newFixture(t)
	laptop.app.updateTick(laptop.app.ctx) // registered, so sign-up links this install

	if _, err := laptop.app.AccountStartSignUp("ada@example.com", "weak"); err == nil {
		t.Fatal("a weak password must be rejected before anything is sent")
	}
	const pw = "Correct-Horse-9"
	if _, err := laptop.app.AccountStartSignUp("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}
	if _, err := laptop.app.AccountFinishSignUp("ada@example.com", "000000", pw); decodeErr(t, err).Detail != "invalid_code" {
		t.Fatalf("wrong code: %v", err)
	}
	res, err := laptop.app.AccountFinishSignUp("ada@example.com", "123456", pw)
	if err != nil || res.RecoveryKey == "" || !res.State.SignedIn {
		t.Fatalf("sign-up: %+v, %v", res, err)
	}
	p.mu.Lock()
	linked := len(p.ext.installHeader)
	p.mu.Unlock()
	if linked == 0 {
		t.Fatal("sign-up should send this installation's token so the computer is linked")
	}

	prof, err := laptop.app.SaveProfile(ProfileInput{Name: "prod", URI: "mongodb://bob:s3cret@db.example:27017"})
	if err != nil {
		t.Fatal(err)
	}
	if st := laptop.app.GetAccountState(); st.PendingChanges != 1 {
		t.Fatalf("pending = %d, want 1", st.PendingChanges)
	}
	rep, err := laptop.app.SyncNow()
	if err != nil || rep.Uploaded != 1 {
		t.Fatalf("push: %+v, %v", rep, err)
	}
	if st := laptop.app.GetAccountState(); st.PendingChanges != 0 {
		t.Fatalf("pending after push = %d", st.PendingChanges)
	}

	// The desktop signs in and restores: the connection arrives with its password and ID.
	desktop := newFixture(t)
	if _, err := desktop.app.AccountSignIn("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}
	if _, err := desktop.app.SyncRestore("nope"); decodeErr(t, err).Detail != "wrong_password" {
		t.Fatalf("wrong password: %v", err)
	}
	rep, err = desktop.app.SyncRestore(pw)
	if err != nil || rep.Added != 1 {
		t.Fatalf("restore: %+v, %v", rep, err)
	}
	got, err := desktop.app.store.GetProfile(prof.ID)
	if err != nil || got.Name != "prod" || !got.HasPassword {
		t.Fatalf("restored profile: %+v, %v", got, err)
	}
	if v, _ := desktop.secrets.Get(config.SecretKey(prof.ID, config.SecretPassword)); v != "s3cret" {
		t.Fatalf("password not restored: %q", v)
	}
	if st := desktop.app.GetAccountState(); st.PendingChanges != 0 {
		t.Fatalf("a restored connection must not be uploaded again (pending %d)", st.PendingChanges)
	}

	// Deleting on the laptop removes it on the desktop at the next restore.
	if err := laptop.app.DeleteProfile(prof.ID); err != nil {
		t.Fatal(err)
	}
	if rep, err := laptop.app.SyncNow(); err != nil || rep.Deleted != 1 {
		t.Fatalf("delete push: %+v, %v", rep, err)
	}
	if rep, err := desktop.app.SyncRestore(pw); err != nil || rep.Removed != 1 {
		t.Fatalf("delete restore: %+v, %v", rep, err)
	}
	if _, err := desktop.app.store.GetProfile(prof.ID); err == nil {
		t.Fatal("deleted connection still on the desktop")
	}
}

func TestDisabledFeaturesAndExpiredSessions(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) { p.ext.registrationOff = true })
	f := newFixture(t)
	_, err := f.app.AccountStartSignUp("ada@example.com", "Correct-Horse-9")
	if e := decodeErr(t, err); e.Detail != "registration_disabled" {
		t.Fatalf("err = %+v", e)
	}
	if !f.app.GetAccountState().Disabled["registration"] {
		t.Fatal("registration should be remembered as off")
	}

	// A session the server no longer knows signs the app out locally.
	_ = f.secrets.Set(secretAccountToken, "ca_stale")
	if _, err := f.app.RefreshAccount(); decodeErr(t, err).Code != "AUTH_FAILED" {
		t.Fatalf("stale session: %v", err)
	}
	if f.app.GetAccountState().SignedIn {
		t.Fatal("still signed in after a 401")
	}
}

func TestAccountActionsWaitForPrivacyAcceptance(t *testing.T) {
	p := withFakePlatform(t)
	p.set(func(p *fakePlatform) {
		p.ext.policy = update.PrivacyPolicy{Version: 1, Title: "Privacy", AcceptanceRequired: true}
	})
	f := newFixture(t)
	f.app.updateTick(f.app.ctx)
	_, err := f.app.AccountSignIn("ada@example.com", "Correct-Horse-9")
	if e := decodeErr(t, err); e.Detail != "privacy_required" {
		t.Fatalf("err = %+v", e)
	}
}
