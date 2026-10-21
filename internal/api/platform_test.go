package api

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"mango3s/internal/config"
	"mango3s/internal/mongox"
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
	transportKey  *ecdh.PrivateKey        // what sync requests are sealed to
	plainSync     []string                // sync requests that arrived readable (must stay empty)
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
	if e.transportKey == nil {
		e.transportKey, _ = ecdh.X25519().GenerateKey(rand.Reader)
	}
	// open reads an envelope the app sealed to the transport key.
	open := func(raw any, aad string) []byte {
		var env update.Envelope
		b, _ := json.Marshal(raw)
		if json.Unmarshal(b, &env) != nil || env.Kid != "fake-kid" {
			return nil
		}
		plain, err := update.OpenWith(e.transportKey, env, aad)
		if err != nil {
			return nil
		}
		return plain
	}
	// sealedPassword opens a pull or wipe body.
	sealedPassword := func(aad string) (password, responseKey string) {
		if _, plain := body["password"]; plain {
			e.plainSync = append(e.plainSync, r.URL.Path)
		}
		var v struct {
			Password    string `json:"password"`
			ResponseKey string `json:"responseKey"`
		}
		_ = json.Unmarshal(open(body["sealed"], aad), &v)
		return v.Password, v.ResponseKey
	}
	switch strings.TrimPrefix(r.URL.Path, "/v1/account") {
	case "/sync/transport-key":
		writeJSON(w, 200, update.TransportKey{KeyID: "fake-kid", Algorithm: "x25519-hkdf-sha256-aes256gcm", PublicKey: base64.StdEncoding.EncodeToString(e.transportKey.PublicKey().Bytes())})
		return true
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
			if it.Data != nil {
				e.plainSync = append(e.plainSync, it.Key)
			}
			if it.Sealed != nil {
				plain := open(it.Sealed, update.AADItem(it.Key))
				if plain == nil {
					apiError(w, 400, "invalid_envelope", "The encrypted request could not be opened.")
					return true
				}
				data := string(plain)
				it.Data, it.Sealed = &data, nil
			}
			if cur, ok := acct.items[it.Key]; ok && cur.UpdatedAt >= it.UpdatedAt {
				res.Skipped = append(res.Skipped, it.Key)
				continue
			}
			acct.items[it.Key] = it
			res.Accepted = append(res.Accepted, it.Key)
		}
		writeJSON(w, 200, res)
	case "/sync/pull":
		password, responseKey := sealedPassword(update.AADPull)
		if password != acct.password {
			apiError(w, 400, "wrong_password", "That password is not correct.")
			return true
		}
		items := []update.SyncItem{}
		for _, it := range acct.items {
			items = append(items, it)
		}
		raw, _ := base64.StdEncoding.DecodeString(responseKey)
		pub, err := ecdh.X25519().NewPublicKey(raw)
		if err != nil {
			apiError(w, 400, "validation_error", "A sealed pull needs a responseKey")
			return true
		}
		answer, _ := json.Marshal(map[string]any{"items": items})
		env, _ := update.SealTo(pub, answer, update.AADPullResponse)
		writeJSON(w, 200, map[string]any{"sealed": env})
	case "/sync/wipe":
		if password, _ := sealedPassword(update.AADWipe); password != acct.password {
			apiError(w, 400, "wrong_password", "That password is not correct.")
			return true
		}
		acct.items = map[string]update.SyncItem{}
		w.WriteHeader(204)
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
	if _, err := desktop.app.SyncPull("nope"); decodeErr(t, err).Detail != "wrong_password" {
		t.Fatalf("wrong password: %v", err)
	}
	rep, err = desktop.app.SyncPull(pw)
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
	if rep, err := desktop.app.SyncPull(pw); err != nil || rep.Removed != 1 {
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

// A synced connection is a complete copy: settings, secrets, the contents of
// its key files and the known_hosts lines for its SSH server, all sent sealed.
func TestSyncCarriesEverythingAConnectionNeeds(t *testing.T) {
	p := withFakePlatform(t)
	laptopHome, desktopHome := t.TempDir(), t.TempDir()
	t.Setenv("HOME", laptopHome)
	hostKey, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(hostKey)
	knownLine := knownhosts.Line([]string{knownhosts.Normalize("bastion.example:2222")}, sshPub)
	if err := os.MkdirAll(filepath.Join(laptopHome, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(laptopHome, ".ssh", "known_hosts"), []byte(knownLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyFile, caFile := filepath.Join(laptopHome, ".ssh", "id_ed25519"), filepath.Join(laptopHome, "ca.pem")
	_ = os.WriteFile(keyFile, []byte("KEY-1"), 0o600)
	_ = os.WriteFile(caFile, []byte("CA-1"), 0o600)

	const pw = "Correct-Horse-9"
	laptop := newFixture(t)
	if _, err := laptop.app.AccountStartSignUp("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}
	if _, err := laptop.app.AccountFinishSignUp("ada@example.com", "123456", pw); err != nil {
		t.Fatal(err)
	}
	desktop := newFixture(t) // its data folder now holds the app's known_hosts
	if _, err := desktop.app.AccountSignIn("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}

	prof, err := laptop.app.SaveProfile(ProfileInput{
		Name: "prod", URI: "mongodb://bob:s3cret@db.internal:27017",
		SSH:     config.SSHConfig{Enabled: true, Host: "bastion.example", Port: 2222, Username: "ada", AuthMethod: config.SSHAuthKey, PrivateKeyFile: keyFile},
		TLS:     config.TLSConfig{Enabled: true, CAFile: caFile},
		Secrets: map[string]SecretInput{config.SecretSSHPassphrase: {Value: "phrase"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep, err := laptop.app.SyncNow(); err != nil || rep.Uploaded != 1 || len(rep.Errors) != 0 {
		t.Fatalf("push: %+v, %v", rep, err)
	}

	// The desktop has none of the laptop's files: both run on this disk, so
	// hide the laptop's home while the desktop pulls.
	onDesktop := func(fn func()) {
		t.Helper()
		t.Setenv("HOME", desktopHome)
		if err := os.Rename(laptopHome, laptopHome+".away"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Rename(laptopHome+".away", laptopHome)
			t.Setenv("HOME", laptopHome)
		}()
		fn()
	}
	onDesktop(func() {
		if rep, err := desktop.app.SyncPull(pw); err != nil || rep.Added != 1 {
			t.Fatalf("pull: %+v, %v", rep, err)
		}
	})
	got, err := desktop.app.store.GetProfile(prof.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.SSH.PrivateKeyFile, filepath.Join(desktop.dir, syncedFilesDir)) {
		t.Fatalf("key file not placed in the data folder: %s", got.SSH.PrivateKeyFile)
	}
	if data, _ := os.ReadFile(got.SSH.PrivateKeyFile); string(data) != "KEY-1" {
		t.Fatalf("key file contents: %q", data)
	}
	if st, _ := os.Stat(got.SSH.PrivateKeyFile); st == nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file must be readable only by the user: %v", st)
	}
	if data, _ := os.ReadFile(got.TLS.CAFile); string(data) != "CA-1" {
		t.Fatalf("CA contents: %q", data)
	}
	if v, _ := desktop.secrets.Get(config.SecretKey(prof.ID, config.SecretSSHPassphrase)); v != "phrase" {
		t.Fatalf("passphrase not restored: %q", v)
	}
	if v, _ := desktop.secrets.Get(config.SecretKey(prof.ID, config.SecretPassword)); v != "s3cret" {
		t.Fatalf("password not restored: %q", v)
	}
	onDesktop(func() {
		if lines := mongox.KnownHostLines("bastion.example", 2222); len(lines) != 1 || lines[0] != knownLine {
			t.Fatalf("SSH server key not trusted on the desktop: %v", lines)
		}
		if st := desktop.app.GetAccountState(); st.PendingChanges != 0 {
			t.Fatalf("a pulled connection must not count as changed (pending %d)", st.PendingChanges)
		}
	})

	// Editing only the key file is a change too.
	_ = os.WriteFile(keyFile, []byte("KEY-2"), 0o600)
	if st := laptop.app.GetAccountState(); st.PendingChanges != 1 {
		t.Fatalf("pending after key edit = %d, want 1", st.PendingChanges)
	}
	if rep, err := laptop.app.SyncNow(); err != nil || rep.Uploaded != 1 {
		t.Fatalf("push key edit: %+v, %v", rep, err)
	}
	if st := laptop.app.GetAccountState(); st.PendingChanges != 0 {
		t.Fatalf("pending after pushing the key edit = %d", st.PendingChanges)
	}
	onDesktop(func() {
		if rep, err := desktop.app.SyncPull(pw); err != nil || rep.Updated != 1 {
			t.Fatalf("pull key edit: %+v, %v", rep, err)
		}
	})
	got, _ = desktop.app.store.GetProfile(prof.ID)
	if data, _ := os.ReadFile(got.SSH.PrivateKeyFile); string(data) != "KEY-2" {
		t.Fatalf("key file after update: %q", data)
	}

	// A third computer that already has the same file at the same path keeps using it.
	third := newFixture(t)
	if _, err := third.app.AccountSignIn("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}
	if rep, err := third.app.SyncPull(pw); err != nil || rep.Added != 1 {
		t.Fatalf("third pull: %+v, %v", rep, err)
	}
	if got, _ := third.app.store.GetProfile(prof.ID); got.SSH.PrivateKeyFile != keyFile {
		t.Fatalf("an identical local file should be reused, got %s", got.SSH.PrivateKeyFile)
	}

	// A key file the connection needs but can't be read is reported; others still go.
	if _, err := laptop.app.SaveProfile(ProfileInput{Name: "broken", URI: "mongodb://h:1",
		SSH: config.SSHConfig{Enabled: true, Host: "h", Port: 22, Username: "u", AuthMethod: config.SSHAuthKey, PrivateKeyFile: filepath.Join(laptopHome, "missing")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := laptop.app.SaveProfile(ProfileInput{Name: "fine", URI: "mongodb://h:2"}); err != nil {
		t.Fatal(err)
	}
	rep, err := laptop.app.SyncNow()
	if err != nil || rep.Uploaded != 1 || len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "broken") {
		t.Fatalf("partial push: %+v, %v", rep, err)
	}

	p.mu.Lock()
	plain := p.ext.plainSync
	p.mu.Unlock()
	if len(plain) != 0 {
		t.Fatalf("sync data or passwords were sent readable: %v", plain)
	}
}

func TestExcludedConnectionsStayLocalAndAutoPushFollowsChanges(t *testing.T) {
	p := withFakePlatform(t)
	const pw = "Correct-Horse-9"
	f := newFixture(t)
	f.app.update.acct.mu.Lock()
	f.app.update.acct.pushWait = 10 * time.Millisecond
	f.app.update.acct.mu.Unlock()
	if _, err := f.app.AccountStartSignUp("ada@example.com", pw); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.AccountFinishSignUp("ada@example.com", "123456", pw); err != nil {
		t.Fatal(err)
	}
	serverItems := func() int {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.ext.accounts["ada@example.com"].items)
	}

	private, err := f.app.SaveProfile(ProfileInput{Name: "private", URI: "mongodb://h:1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.app.mutateSettings(func(s *config.Settings) { s.SyncExclude = []string{private.ID} }); err != nil {
		t.Fatal(err)
	}
	if st := f.app.GetAccountState(); st.PendingChanges != 0 {
		t.Fatalf("an excluded connection must not be pending (%d)", st.PendingChanges)
	}

	// Manual by default: a change waits for Push.
	if _, err := f.app.SaveProfile(ProfileInput{Name: "shared", URI: "mongodb://h:2"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if n := serverItems(); n != 0 {
		t.Fatalf("pushed without automatic sync: %d items", n)
	}

	if err := f.app.mutateSettings(func(s *config.Settings) { s.SyncAuto = true }); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.SaveProfile(ProfileInput{Name: "another", URI: "mongodb://h:3"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for serverItems() != 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := serverItems(); n != 2 {
		t.Fatalf("automatic push after a change: %d items on the server, want 2 (not the excluded one)", n)
	}

	// Deleting an excluded connection sends no deletion.
	if err := f.app.DeleteProfile(private.ID); err != nil {
		t.Fatal(err)
	}
	if st := f.app.GetAccountState(); st.PendingChanges != 0 {
		t.Fatalf("pending after deleting an excluded connection = %d", st.PendingChanges)
	}
}
