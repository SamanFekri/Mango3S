package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/secrets"
	"mango3s/internal/update"
)

// Optional Mango3S account with encrypted connection sync
// (web/docs/API_CONTRACT.md §7). Nothing here is needed to use the app: it is
// off until the user signs in, every call is user-initiated or a quiet
// background push, and an unreachable server only means "try again later".
//
// What is synced: one item per connection ("conn-<id>"), holding everything
// needed to recreate it as it is here: every setting, every secret
// (passwords, passphrases, tokens), the contents of the files it points to
// (SSH private key, TLS client certificate/key, CA) and the known_hosts lines
// that vouch for its SSH server. Connections listed in Settings.SyncExclude
// stay on this computer. Pushing and pulling are manual unless SyncAuto is on,
// which pushes a few seconds after a connection changes.
//
// Each item is sealed to the server's transport key before it is sent (see
// update/transport.go); the server opens it and encrypts it to the account's
// public key, so uploading needs no password; reading back needs it. Locally,
// sync bookkeeping (which version of each connection the server has) lives in
// account-sync.json; the account token lives in the OS credential store like
// every other secret.

const (
	secretAccountToken = "account:token"
	accountSyncFile    = "account-sync.json"
	syncKeyPrefix      = "conn-"
	// syncItemVersion 2 added Files and KnownHosts; version 1 items still apply.
	syncItemVersion = 2
	// syncedFilesDir (in the data folder) holds key files that arrived with
	// a synced connection, one folder per connection.
	syncedFilesDir = "synced-files"
)

// autoPushDelay lets a burst of edits go up as one push.
const autoPushDelay = 5 * time.Second

// Platform error codes that mean "the admin turned this part off". They are
// remembered (until the next success) so the UI can hide what won't work.
var disabledCodes = map[string]string{
	"registration_disabled":   "registration",
	"login_disabled":          "login",
	"password_reset_disabled": "passwordReset",
	"sync_disabled":           "sync",
}

// accountState is the in-memory side of the account; see newUpdateState.
type accountState struct {
	mu       sync.Mutex // serializes account actions and sync runs
	account  *update.Account
	disabled map[string]bool
	syncing  bool
	autoPush *time.Timer   // pending automatic push after a change
	pushWait time.Duration // delay before it (autoPushDelay; shorter in tests)
}

// syncBook is account-sync.json.
type syncBook struct {
	Email string `json:"email,omitempty"`
	// Items maps each synced key to the version (updatedAt) the server has.
	Items map[string]string `json:"items"`
	// Content maps each synced key to the contentHash of the files and
	// known_hosts lines the server has, so editing a key file is a change too.
	Content    map[string]string `json:"content,omitempty"`
	LastSyncAt string            `json:"lastSyncAt,omitempty"`
	LastError  string            `json:"lastError,omitempty"`
	// RestoreSuggested is set when the server had a newer version of
	// something; "Restore from account" brings it in.
	RestoreSuggested bool `json:"restoreSuggested,omitempty"`
}

// syncPayload is what one synced connection holds: everything needed to
// recreate it as it is here.
type syncPayload struct {
	V       int               `json:"v"`
	Profile config.Profile    `json:"profile"`
	Secrets map[string]string `json:"secrets,omitempty"`
	// Files holds the contents of the files the connection points to, by
	// kind (syncFileSSHKey, ...), so it works where those files don't exist.
	Files map[string]syncFile `json:"files,omitempty"`
	// KnownHosts are the known_hosts lines listing the SSH server's keys, so
	// the tunnel is trusted the same way on another computer.
	KnownHosts []string `json:"knownHosts,omitempty"`
}

// syncFile is one file's contents.
type syncFile struct {
	Name string `json:"name"` // base name, for the copy written on another computer
	Data string `json:"data"` // base64
}

// AccountState is what the Account settings tab shows.
type AccountState struct {
	SignedIn bool            `json:"signedIn"`
	Email    string          `json:"email"`
	Account  *update.Account `json:"account"`
	// Disabled lists features the server said are turned off
	// (registration, login, passwordReset, sync).
	Disabled         map[string]bool `json:"disabled"`
	Syncing          bool            `json:"syncing"`
	LastSyncAt       string          `json:"lastSyncAt"`
	LastSyncError    string          `json:"lastSyncError"`
	PendingChanges   int             `json:"pendingChanges"`
	RestoreSuggested bool            `json:"restoreSuggested"`
	PrivacyPending   bool            `json:"privacyPending"`
}

// AccountResult is returned by the actions that sign in.
type AccountResult struct {
	State AccountState `json:"state"`
	// RecoveryKey is set after sign-up and after a reset that erased the data:
	// it must be shown to the user once — the server can't show it again.
	RecoveryKey string `json:"recoveryKey,omitempty"`
	DataKept    *bool  `json:"dataKept,omitempty"`
}

// SyncReport summarizes one sync run.
type SyncReport struct {
	Uploaded int      `json:"uploaded"`
	Deleted  int      `json:"deleted"`
	Added    int      `json:"added"`
	Updated  int      `json:"updated"`
	Removed  int      `json:"removed"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// CodeSent tells the user where the code went.
type CodeSent = update.CodeSent

// ResetPasswordInput is the "forgot password" form.
type ResetPasswordInput struct {
	Email             string `json:"email"`
	Code              string `json:"code"`
	NewPassword       string `json:"newPassword"`
	RecoveryKey       string `json:"recoveryKey"`
	DiscardSyncedData bool   `json:"discardSyncedData"`
}

func (a *App) bookPath() string { return filepath.Join(a.store.Dir(), accountSyncFile) }

func (a *App) readBook() syncBook {
	b := syncBook{}
	if err := config.ReadJSON(a.bookPath(), &b); err != nil {
		a.log.Warn("account sync state unreadable; starting fresh", "err", err)
	}
	if b.Items == nil {
		b.Items = map[string]string{}
	}
	if b.Content == nil {
		b.Content = map[string]string{}
	}
	return b
}

func (a *App) writeBook(b syncBook) {
	if err := config.WriteJSON(a.bookPath(), b); err != nil {
		a.log.Warn("could not save account sync state", "err", err)
	}
}

func (a *App) accountToken() string {
	t, err := a.secrets.Get(secretAccountToken)
	if err != nil {
		return ""
	}
	return t
}

func (a *App) emitAccount() { a.update.emit("account:changed", a.accountState()) }

func (a *App) accountState() AccountState {
	book := a.readBook()
	st := &a.update.acct
	st.mu.Lock()
	acct, syncing := st.account, st.syncing
	disabled := make(map[string]bool, len(st.disabled))
	for k, v := range st.disabled {
		disabled[k] = v
	}
	st.mu.Unlock()
	pending := 0
	if token := a.accountToken(); token != "" {
		if items, err := a.localChanges(book); err == nil {
			pending = len(items)
		}
		return AccountState{
			SignedIn: true, Email: book.Email, Account: acct, Disabled: disabled, Syncing: syncing,
			LastSyncAt: book.LastSyncAt, LastSyncError: book.LastError, PendingChanges: pending,
			RestoreSuggested: book.RestoreSuggested, PrivacyPending: a.privacyPending(),
		}
	}
	return AccountState{Disabled: disabled, PrivacyPending: a.privacyPending()}
}

// accountErr turns a platform answer into a message for the user. The
// platform's own error code goes into Detail so the UI can react to it
// (e.g. "invalid_recovery_key", "wrong_password").
func (a *App) accountErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, update.ErrTransportUnsupported) {
		return &apperr.Error{Code: apperr.CodeUnsupported, Message: "The Mango3S server can't take encrypted sync yet, so nothing was sent. Ask the administrator to update it.", Detail: "transport_unsupported"}
	}
	var se *update.StatusError
	if !errors.As(err, &se) {
		return apperr.Wrap(apperr.CodeUnreachable, err, "Couldn't reach the Mango3S server. Check your connection and try again.")
	}
	if feature, ok := disabledCodes[se.Code]; ok {
		a.setDisabled(feature, true)
	}
	msg := strings.TrimSpace(se.Message)
	switch {
	case se.Status == 401:
		a.clearSession()
		return &apperr.Error{Code: apperr.CodeAuthFailed, Message: "You were signed out. Sign in again.", Detail: "unauthorized"}
	case se.Code == "weak_password":
		var d struct {
			Problems []string `json:"problems"`
		}
		if json.Unmarshal(se.Details, &d) == nil && len(d.Problems) > 0 {
			msg = strings.Join(d.Problems, " ")
		}
	case se.Status == 429:
		msg = "Too many attempts. Wait a few minutes and try again."
	case se.Status >= 500:
		return apperr.Wrap(apperr.CodeUnreachable, err, "The Mango3S server had a problem. Try again in a moment.")
	}
	if msg == "" {
		msg = fmt.Sprintf("The server refused the request (HTTP %d).", se.Status)
	}
	code := apperr.CodeInvalidInput
	if se.Status == 403 {
		code = apperr.CodeUnsupported
	}
	return &apperr.Error{Code: code, Message: msg, Detail: se.Code}
}

func (a *App) setDisabled(feature string, off bool) {
	st := &a.update.acct
	st.mu.Lock()
	if off {
		st.disabled[feature] = true
	} else {
		delete(st.disabled, feature)
	}
	st.mu.Unlock()
}

func (a *App) clearSession() {
	_ = a.secrets.Delete(secretAccountToken)
	st := &a.update.acct
	st.mu.Lock()
	st.account = nil
	st.mu.Unlock()
	book := a.readBook()
	book.Email, book.LastError = "", ""
	a.writeBook(book)
}

// startSession stores a new session and resets sync bookkeeping when a
// different account signs in on this computer.
func (a *App) startSession(s update.AccountSession) error {
	if err := a.secrets.Set(secretAccountToken, s.AccountToken); err != nil {
		return apperr.Wrap(apperr.CodeSecretBackend, err, "Signed in, but the session could not be stored securely.")
	}
	book := a.readBook()
	if !strings.EqualFold(book.Email, s.Account.Email) {
		book = syncBook{Items: map[string]string{}}
	}
	book.Email, book.LastError = s.Account.Email, ""
	a.writeBook(book)
	acct := s.Account
	st := &a.update.acct
	st.mu.Lock()
	st.account = &acct
	st.mu.Unlock()
	a.log.Info("signed in to Mango3S account")
	return nil
}

func requirePrivacy(a *App) error {
	if a.privacyPending() {
		return &apperr.Error{Code: apperr.CodeConflict, Message: "Accept the updated privacy policy first.", Detail: "privacy_required"}
	}
	return nil
}

func deviceLabel() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(h, ".local")
}

func (a *App) deviceInfo() update.DeviceInfo {
	token, _, _ := a.updateCredentials()
	return update.DeviceInfo{InstallationToken: token, Label: deviceLabel()}
}

func (a *App) accountCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(a.ctx, 45*time.Second) // argon2 on the server makes some calls slow
}

func (a *App) finish(op string, s update.AccountSession, err error) (AccountResult, error) {
	if err != nil {
		return AccountResult{}, a.encode(op, a.accountErr(err))
	}
	if err := a.startSession(s); err != nil {
		return AccountResult{}, a.encode(op, err)
	}
	a.emitAccount()
	return AccountResult{State: a.accountState(), RecoveryKey: s.RecoveryKey, DataKept: s.DataKept}, nil
}

// ---- bound methods: account ------------------------------------------------

// GetAccountState returns the account as last known (no network).
func (a *App) GetAccountState() AccountState { return a.accountState() }

// RefreshAccount reloads the account from the server.
func (a *App) RefreshAccount() (AccountState, error) {
	token := a.accountToken()
	if token == "" {
		return a.accountState(), nil
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	acct, err := a.platformClient().GetAccount(ctx, token)
	if err != nil {
		err = a.accountErr(err)
		a.emitAccount()
		return a.accountState(), a.encode("RefreshAccount", err)
	}
	st := &a.update.acct
	st.mu.Lock()
	st.account = &acct
	st.mu.Unlock()
	if !acct.Sync.Enabled {
		a.setDisabled("sync", true)
	} else {
		a.setDisabled("sync", false)
	}
	return a.accountState(), nil
}

// AccountStartSignUp sends a 6-digit code to email. The account is created
// by AccountFinishSignUp.
func (a *App) AccountStartSignUp(email, password string) (CodeSent, error) {
	if err := requirePrivacy(a); err != nil {
		return CodeSent{}, a.encode("AccountStartSignUp", err)
	}
	if err := checkPassword(password, email); err != nil {
		return CodeSent{}, a.encode("AccountStartSignUp", err)
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	sent, err := a.platformClient().AccountRegister(ctx, strings.TrimSpace(email), password)
	if err != nil {
		return CodeSent{}, a.encode("AccountStartSignUp", a.accountErr(err))
	}
	a.setDisabled("registration", false)
	return sent, nil
}

// AccountFinishSignUp creates the account with the emailed code. The result
// carries the recovery key, which must be shown to the user once.
func (a *App) AccountFinishSignUp(email, code, password string) (AccountResult, error) {
	ctx, cancel := a.accountCtx()
	defer cancel()
	s, err := a.platformClient().AccountVerify(ctx, strings.TrimSpace(email), strings.TrimSpace(code), password, a.deviceInfo())
	return a.finish("AccountFinishSignUp", s, err)
}

// AccountSignIn signs in and links this computer to the account.
func (a *App) AccountSignIn(email, password string) (AccountResult, error) {
	if err := requirePrivacy(a); err != nil {
		return AccountResult{}, a.encode("AccountSignIn", err)
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	s, err := a.platformClient().AccountLogin(ctx, strings.TrimSpace(email), password, a.deviceInfo())
	if err == nil {
		a.setDisabled("login", false)
	}
	return a.finish("AccountSignIn", s, err)
}

// AccountSignOut ends the session. Local connections are untouched.
func (a *App) AccountSignOut() AccountState {
	if token := a.accountToken(); token != "" {
		ctx, cancel := context.WithTimeout(a.ctx, userRequestTimeout)
		defer cancel()
		if err := a.platformClient().AccountLogout(ctx, token); err != nil {
			a.log.Debug("server sign-out failed; signing out locally", "err", err)
		}
	}
	a.clearSession()
	a.emitAccount()
	return a.accountState()
}

// AccountChangePassword changes the password. Synced data is kept and other
// devices are signed out.
func (a *App) AccountChangePassword(current, next string) error {
	token := a.accountToken()
	if token == "" {
		return a.encode("AccountChangePassword", apperr.Invalid("Sign in first."))
	}
	if err := checkPassword(next, a.readBook().Email); err != nil {
		return a.encode("AccountChangePassword", err)
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	return a.encode("AccountChangePassword", a.accountErr(a.platformClient().AccountChangePassword(ctx, token, current, next)))
}

// AccountForgotPassword emails a reset code.
func (a *App) AccountForgotPassword(email string) (CodeSent, error) {
	ctx, cancel := a.accountCtx()
	defer cancel()
	sent, err := a.platformClient().AccountForgotPassword(ctx, strings.TrimSpace(email))
	if err != nil {
		return CodeSent{}, a.encode("AccountForgotPassword", a.accountErr(err))
	}
	a.setDisabled("passwordReset", false)
	return sent, nil
}

// AccountResetPassword sets a new password with the emailed code. With the
// recovery key nothing is lost; with DiscardSyncedData the synced data is
// erased and a new recovery key is returned.
func (a *App) AccountResetPassword(in ResetPasswordInput) (AccountResult, error) {
	if strings.TrimSpace(in.RecoveryKey) == "" && !in.DiscardSyncedData {
		return AccountResult{}, a.encode("AccountResetPassword", apperr.Invalid("Enter your recovery key, or choose to reset without it."))
	}
	if err := checkPassword(in.NewPassword, in.Email); err != nil {
		return AccountResult{}, a.encode("AccountResetPassword", err)
	}
	req := update.ResetPasswordRequest{
		Email: strings.TrimSpace(in.Email), Code: strings.TrimSpace(in.Code), NewPassword: in.NewPassword,
	}
	if strings.TrimSpace(in.RecoveryKey) != "" {
		req.RecoveryKey = strings.TrimSpace(in.RecoveryKey)
	} else {
		req.DiscardSyncedData = true
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	s, err := a.platformClient().AccountResetPassword(ctx, req)
	res, ferr := a.finish("AccountResetPassword", s, err)
	if ferr == nil && s.DataKept != nil && !*s.DataKept {
		// The server's copies are gone: everything local counts as not uploaded.
		book := a.readBook()
		book.Items, book.Content, book.RestoreSuggested = map[string]string{}, map[string]string{}, false
		a.writeBook(book)
		res.State = a.accountState()
	}
	return res, ferr
}

// AccountRotateRecoveryKey issues a new recovery key (the old one stops working).
func (a *App) AccountRotateRecoveryKey(password string) (string, error) {
	token := a.accountToken()
	if token == "" {
		return "", a.encode("AccountRotateRecoveryKey", apperr.Invalid("Sign in first."))
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	key, err := a.platformClient().AccountRotateRecoveryKey(ctx, token, password)
	return key, a.encode("AccountRotateRecoveryKey", a.accountErr(err))
}

// AccountDelete deletes the account and its synced data. Connections on this
// computer stay.
func (a *App) AccountDelete(password string) error {
	token := a.accountToken()
	if token == "" {
		return a.encode("AccountDelete", apperr.Invalid("Sign in first."))
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	if err := a.platformClient().AccountDelete(ctx, token, password); err != nil {
		return a.encode("AccountDelete", a.accountErr(err))
	}
	a.clearSession()
	a.writeBook(syncBook{Items: map[string]string{}})
	a.emitAccount()
	a.log.Info("Mango3S account deleted")
	return nil
}

// ---- bound methods: sync ---------------------------------------------------

// SyncNow pushes every connection changed (or deleted) since the last sync.
func (a *App) SyncNow() (SyncReport, error) {
	ctx, cancel := a.accountCtx()
	defer cancel()
	r, err := a.pushChanges(ctx, a.platformClient())
	a.emitAccount()
	return r, a.encode("SyncNow", err)
}

// SyncPull downloads the account's connections (this needs the password)
// and applies the newer ones here: new connections are added, changed ones
// updated, and ones deleted on another device removed — unless they changed
// here since (the next push sends those). Nothing is uploaded.
func (a *App) SyncPull(password string) (SyncReport, error) {
	ctx, cancel := a.accountCtx()
	defer cancel()
	report := SyncReport{Errors: []string{}}
	if err := requirePrivacy(a); err != nil {
		return report, a.encode("SyncPull", err)
	}
	err := a.pullAndApply(ctx, a.platformClient(), password, &report)
	a.emitAccount()
	return report, a.encode("SyncPull", err)
}

// SyncWipe erases everything synced to the account on the server.
func (a *App) SyncWipe(password string) error {
	token := a.accountToken()
	if token == "" {
		return a.encode("SyncWipe", apperr.Invalid("Sign in first."))
	}
	ctx, cancel := a.accountCtx()
	defer cancel()
	if err := a.platformClient().SyncWipe(ctx, token, password); err != nil {
		return a.encode("SyncWipe", a.accountErr(err))
	}
	book := a.readBook()
	book.Items, book.Content, book.RestoreSuggested, book.LastError = map[string]string{}, map[string]string{}, false, ""
	a.writeBook(book)
	if s := a.currentSettings(); s.SyncAuto {
		// Otherwise the next background run would upload everything again.
		_ = a.mutateSettings(func(s *config.Settings) { s.SyncAuto = false })
	}
	a.emitAccount()
	return nil
}

// ---- sync engine -----------------------------------------------------------

// stamp formats a version time the way the server stores it (milliseconds),
// so local and server versions compare equal after a round trip.
func stamp(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}

func parseStamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC().Truncate(time.Millisecond), true
}

// Kinds of files a synced connection carries the contents of.
const (
	syncFileSSHKey  = "sshPrivateKey"
	syncFileTLSCert = "tlsCertKey"
	syncFileTLSCA   = "tlsCA"
)

// maxSyncFileBytes caps one key or certificate file; real ones are a few KB.
const maxSyncFileBytes = 256 << 10

// connFile is one file a profile points to.
type connFile struct {
	kind  string
	path  *string
	inUse bool // the connection needs it to work
}

func connFiles(p *config.Profile) []connFile {
	return []connFile{
		{syncFileSSHKey, &p.SSH.PrivateKeyFile, p.SSH.Enabled && p.SSH.AuthMethod == config.SSHAuthKey},
		{syncFileTLSCert, &p.TLS.CertKeyFile, p.TLS.Enabled},
		{syncFileTLSCA, &p.TLS.CAFile, p.TLS.Enabled},
	}
}

var syncFileLabels = map[string]string{
	syncFileSSHKey:  "SSH private key",
	syncFileTLSCert: "TLS client certificate",
	syncFileTLSCA:   "TLS CA file",
}

func readSmallFile(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > maxSyncFileBytes {
		return nil, fmt.Errorf("%s is larger than %d KB", path, maxSyncFileBytes>>10)
	}
	return os.ReadFile(path)
}

// connContent reads what a connection carries besides its settings: the
// files it points to and the known_hosts lines for its SSH server. A file
// the connection needs that can't be read is an error; unused ones are left
// out.
func connContent(p config.Profile) (map[string]syncFile, []string, error) {
	files := map[string]syncFile{}
	for _, f := range connFiles(&p) {
		path := strings.TrimSpace(*f.path)
		if path == "" {
			continue
		}
		data, err := readSmallFile(path)
		if err != nil {
			if f.inUse {
				return nil, nil, apperr.Invalid("The %s of %q can't be read (%s), so it can't be synced. Fix the path or the file, then push again.", syncFileLabels[f.kind], p.Name, err)
			}
			continue
		}
		files[f.kind] = syncFile{Name: filepath.Base(path), Data: base64.StdEncoding.EncodeToString(data)}
	}
	var known []string
	if p.SSH.Enabled && p.SSH.Host != "" {
		known = mongox.KnownHostLines(p.SSH.Host, p.SSH.Port)
	}
	return files, known, nil
}

// contentHash fingerprints a connection's files and known_hosts lines.
func contentHash(files map[string]syncFile, known []string) string {
	h := sha256.New()
	kinds := make([]string, 0, len(files))
	for k := range files {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Fprintf(h, "f\x00%s\x00%s\x00%s\x00", k, files[k].Name, files[k].Data)
	}
	for _, l := range known {
		fmt.Fprintf(h, "k\x00%s\x00", l)
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// localContentHash is contentHash for a profile as it is on disk now; an
// unreadable file gives a value that never matches, so it shows as pending.
func localContentHash(p config.Profile) string {
	files, known, err := connContent(p)
	if err != nil {
		return "unreadable"
	}
	return contentHash(files, known)
}

func excludedSet(s config.Settings) map[string]bool {
	out := make(map[string]bool, len(s.SyncExclude))
	for _, id := range s.SyncExclude {
		out[id] = true
	}
	return out
}

// localChanges lists what the server doesn't have yet: connections changed
// since their last sync (settings, or the contents of their files), and
// tombstones for connections deleted here. Excluded connections are skipped.
func (a *App) localChanges(book syncBook) ([]update.SyncItem, error) {
	list, err := a.store.ListProfiles()
	if err != nil {
		return nil, err
	}
	excluded := excludedSet(a.currentSettings())
	var items []update.SyncItem
	seen := map[string]bool{}
	for _, p := range list {
		key := syncKeyPrefix + p.ID
		seen[key] = true
		if excluded[p.ID] {
			continue
		}
		if book.Items[key] == stamp(p.UpdatedAt) && book.Content[key] == localContentHash(p) {
			continue
		}
		items = append(items, update.SyncItem{Key: key, UpdatedAt: stamp(p.UpdatedAt)})
	}
	keys := make([]string, 0, len(book.Items))
	for key := range book.Items {
		if !seen[key] && !excluded[strings.TrimPrefix(key, syncKeyPrefix)] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	now := stamp(time.Now())
	for _, key := range keys {
		items = append(items, update.SyncItem{Key: key, Deleted: true, UpdatedAt: now})
	}
	return items, nil
}

// payload builds the data of one connection item: the profile, its secrets,
// its files and known_hosts lines. It also returns the content hash to
// remember once the server has it.
func (a *App) payload(id string) (string, string, error) {
	p, err := a.store.GetProfile(id)
	if err != nil {
		return "", "", err
	}
	sec := map[string]string{}
	for _, kind := range config.SecretKinds {
		if !p.HasSecret(kind) {
			continue
		}
		v, err := a.secrets.Get(config.SecretKey(p.ID, kind))
		if errors.Is(err, secrets.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", "", apperr.Wrap(apperr.CodeSecretBackend, err, fmt.Sprintf("A saved secret of %q could not be read.", p.Name))
		}
		sec[kind] = v
	}
	files, known, err := connContent(p)
	if err != nil {
		return "", "", err
	}
	p.LastConnectedAt, p.Position = nil, 0 // per-device details
	data, err := json.Marshal(syncPayload{V: syncItemVersion, Profile: p, Secrets: sec, Files: files, KnownHosts: known})
	return string(data), contentHash(files, known), err
}

func (a *App) beginSync() (func(), error) {
	st := &a.update.acct
	st.mu.Lock()
	if st.syncing {
		st.mu.Unlock()
		return nil, apperr.New(apperr.CodeConflict, "A sync is already running.")
	}
	st.syncing = true
	st.mu.Unlock()
	return func() {
		st.mu.Lock()
		st.syncing = false
		st.mu.Unlock()
	}, nil
}

// touchContentChanges gives connections whose files changed (but not their
// settings) a new version, so the server takes them as newer.
func (a *App) touchContentChanges(book syncBook, items []update.SyncItem) {
	for i, it := range items {
		if it.Deleted || book.Items[it.Key] != it.UpdatedAt {
			continue
		}
		p, err := a.store.GetProfile(strings.TrimPrefix(it.Key, syncKeyPrefix))
		if err != nil {
			continue
		}
		p.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
		if saved, err := a.store.PutProfile(p); err == nil {
			items[i].UpdatedAt = stamp(saved.UpdatedAt)
		}
	}
}

// pushChanges uploads local changes. No password is needed. A connection
// that can't be packed (an unreadable key file, too large) is reported and
// left out; the rest still go up.
func (a *App) pushChanges(ctx context.Context, client *update.Client) (SyncReport, error) {
	report := SyncReport{Errors: []string{}}
	token := a.accountToken()
	if token == "" {
		return report, apperr.Invalid("Sign in first.")
	}
	if err := requirePrivacy(a); err != nil {
		return report, err
	}
	done, err := a.beginSync()
	if err != nil {
		return report, err
	}
	defer done()

	book := a.readBook()
	all, err := a.localChanges(book)
	if err != nil {
		return report, err
	}
	a.touchContentChanges(book, all)
	maxBytes := 0
	a.update.acct.mu.Lock()
	if acct := a.update.acct.account; acct != nil {
		maxBytes = acct.Sync.MaxItemKB * 1024
	}
	a.update.acct.mu.Unlock()

	items := make([]update.SyncItem, 0, len(all))
	hashes := map[string]string{}
	for _, it := range all {
		if !it.Deleted {
			id := strings.TrimPrefix(it.Key, syncKeyPrefix)
			data, hash, err := a.payload(id)
			if err == nil && maxBytes > 0 && len(data) > maxBytes {
				err = apperr.Invalid("It is larger than the server's limit of %d KB per connection.", maxBytes/1024)
			}
			if err != nil {
				name := id
				if p, perr := a.store.GetProfile(id); perr == nil {
					name = p.Name
				}
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %s", name, apperr.From(err).Message))
				continue
			}
			it.Data, hashes[it.Key] = &data, hash
		}
		items = append(items, it)
	}
	versions := map[string]update.SyncItem{}
	for _, it := range items {
		versions[it.Key] = it
	}

	for start := 0; start < len(items); start += 500 {
		end := min(start+500, len(items))
		res, err := client.SyncPush(ctx, token, items[start:end])
		if err != nil {
			err = a.accountErr(err)
			book.LastError = apperr.From(err).Message
			a.writeBook(book)
			return report, err
		}
		for _, key := range res.Accepted {
			it := versions[key]
			if it.Deleted {
				delete(book.Items, key)
				delete(book.Content, key)
				report.Deleted++
			} else {
				book.Items[key], book.Content[key] = it.UpdatedAt, hashes[key]
				report.Uploaded++
			}
		}
		for _, key := range res.Skipped {
			report.Skipped++
			book.RestoreSuggested = true
			if versions[key].Deleted {
				delete(book.Items, key) // the server has a newer version; a pull brings it back
				delete(book.Content, key)
			}
		}
	}
	book.LastSyncAt, book.LastError = time.Now().UTC().Format(time.RFC3339), ""
	if len(report.Errors) > 0 {
		book.LastError = report.Errors[0]
	}
	a.writeBook(book)
	a.setDisabled("sync", false)
	return report, nil
}

// pullAndApply downloads everything and applies what is newer than here.
func (a *App) pullAndApply(ctx context.Context, client *update.Client, password string, report *SyncReport) error {
	token := a.accountToken()
	if token == "" {
		return apperr.Invalid("Sign in first.")
	}
	done, err := a.beginSync()
	if err != nil {
		return err
	}
	defer done()

	items, err := client.SyncPull(ctx, token, password)
	if err != nil {
		return a.accountErr(err)
	}
	book := a.readBook()
	excluded := excludedSet(a.currentSettings())
	changed := false
	for _, it := range items {
		if !strings.HasPrefix(it.Key, syncKeyPrefix) {
			continue // something a newer app version syncs; leave it alone
		}
		id := strings.TrimPrefix(it.Key, syncKeyPrefix)
		if excluded[id] {
			continue // kept on this computer only
		}
		remote, ok := parseStamp(it.UpdatedAt)
		if !ok {
			continue
		}
		local, lerr := a.store.GetProfile(id)
		exists := lerr == nil
		if exists && !remote.After(local.UpdatedAt.UTC().Truncate(time.Millisecond)) {
			continue // this computer has the same or a newer version; the next push sends it
		}
		if it.Deleted {
			if exists {
				if err := a.DeleteProfile(id); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("%s: %s", local.Name, apperr.From(err).Message))
					continue
				}
				report.Removed++
				changed = true
			}
			delete(book.Items, it.Key)
			delete(book.Content, it.Key)
			continue
		}
		if it.Data == nil {
			continue
		}
		var pl syncPayload
		if err := json.Unmarshal([]byte(*it.Data), &pl); err != nil || pl.Profile.Name == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: this item could not be read.", it.Key))
			continue
		}
		if pl.V > syncItemVersion {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: it was synced by a newer Mango3S. Update the app to bring it in.", pl.Profile.Name))
			continue
		}
		var prev *config.Profile
		if exists {
			prev = &local
		}
		saved, err := a.applySynced(id, remote, pl, prev)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %s", pl.Profile.Name, apperr.From(err).Message))
			continue
		}
		if exists {
			report.Updated++
		} else {
			report.Added++
		}
		book.Items[it.Key], book.Content[it.Key] = stamp(remote), localContentHash(saved)
		changed = true
	}
	book.RestoreSuggested = false
	book.LastSyncAt, book.LastError = time.Now().UTC().Format(time.RFC3339), ""
	a.writeBook(book)
	if changed {
		a.update.emit("profiles:changed", nil)
	}
	a.log.Info("pulled connections from account", "added", report.Added, "updated", report.Updated, "removed", report.Removed, "errors", len(report.Errors))
	return nil
}

// applySynced stores one connection from the account with its secrets, its
// files and its known_hosts lines. prev is the version already here, if any.
func (a *App) applySynced(id string, version time.Time, pl syncPayload, prev *config.Profile) (config.Profile, error) {
	p := pl.Profile
	p.ID, p.UpdatedAt = id, version
	p.HasPassword, p.StoredSecrets = false, nil
	for kind := range pl.Secrets {
		p.SetSecretFlag(kind, true)
	}
	if err := a.placeSyncedFiles(id, &p, pl.Files, prev); err != nil {
		return config.Profile{}, err
	}
	if err := mongox.AddKnownHostLines(pl.KnownHosts); err != nil {
		a.log.Warn("could not save synced known_hosts lines", "err", err)
	}
	saved, err := a.store.PutProfile(p)
	if err != nil {
		return config.Profile{}, err
	}
	for _, kind := range config.SecretKinds {
		key := config.SecretKey(saved.ID, kind)
		if v, ok := pl.Secrets[kind]; ok && v != "" {
			if err := a.secrets.Set(key, v); err != nil {
				return config.Profile{}, apperr.Wrap(apperr.CodeSecretBackend, err, "A secret could not be stored securely.")
			}
		} else if err := a.secrets.Delete(key); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			a.log.Warn("could not remove a stored secret", "kind", kind, "err", err)
		}
	}
	return saved, nil
}

// placeSyncedFiles points p at a local copy of each synced file. A file
// this computer already has with the same contents (at the path the item
// names, or where this connection had it before) is used as it is;
// otherwise the contents are written to the data folder, readable only by
// this user.
func (a *App) placeSyncedFiles(id string, p *config.Profile, files map[string]syncFile, prev *config.Profile) error {
	prevPaths := map[string]string{}
	if prev != nil {
		for _, f := range connFiles(prev) {
			prevPaths[f.kind] = *f.path
		}
	}
	for _, f := range connFiles(p) {
		sf, ok := files[f.kind]
		if !ok {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(sf.Data)
		if err != nil {
			return apperr.Invalid("The synced %s could not be read.", syncFileLabels[f.kind])
		}
		reused := false
		for _, candidate := range []string{*f.path, prevPaths[f.kind]} {
			if candidate == "" {
				continue
			}
			if have, err := readSmallFile(candidate); err == nil && bytes.Equal(have, data) {
				*f.path, reused = candidate, true
				break
			}
		}
		if reused {
			continue
		}
		dst, err := a.writeSyncedFile(id, f.kind, sf.Name, data)
		if err != nil {
			return apperr.Wrap(apperr.CodeInternal, err, "The synced "+syncFileLabels[f.kind]+" could not be saved.")
		}
		*f.path = dst
	}
	return nil
}

func (a *App) syncedFilesPath(id string) string {
	return filepath.Join(a.store.Dir(), syncedFilesDir, filepath.Base(id))
}

// writeSyncedFile replaces the connection's copy of one kind of file.
func (a *App) writeSyncedFile(id, kind, name string, data []byte) (string, error) {
	dir := a.syncedFilesPath(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if old, _ := filepath.Glob(filepath.Join(dir, kind+"-*")); len(old) > 0 {
		for _, f := range old {
			_ = os.Remove(f)
		}
	}
	name = strings.TrimLeft(filepath.Base(strings.ReplaceAll(name, "\\", "/")), ".")
	if name == "" || name == "/" {
		name = "file"
	}
	dst := filepath.Join(dir, kind+"-"+name)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		a.log.Debug("could not restrict synced file permissions", "err", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return dst, os.Rename(tmp.Name(), dst)
}

// removeSyncedFiles deletes the key files that came with a connection.
func (a *App) removeSyncedFiles(id string) {
	if id == "" {
		return
	}
	if err := os.RemoveAll(a.syncedFilesPath(id)); err != nil {
		a.log.Warn("could not remove synced key files", "err", err)
	}
}

// scheduleAutoPush runs after any change to a connection: the account view
// is refreshed and, with automatic sync on, a push follows shortly.
func (a *App) scheduleAutoPush() {
	if a.accountToken() == "" {
		return
	}
	a.emitAccount()
	if !a.currentSettings().SyncAuto {
		return
	}
	st := &a.update.acct
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.autoPush != nil {
		st.autoPush.Stop()
	}
	wait := st.pushWait
	if wait == 0 {
		wait = autoPushDelay
	}
	st.autoPush = time.AfterFunc(wait, func() {
		defer a.guard("automatic sync")
		ctx, cancel := a.accountCtx()
		defer cancel()
		a.backgroundSync(ctx, a.platformClient())
	})
}

// backgroundSync pushes local changes when the user is signed in and
// automatic sync is on: after a change, and on the regular check-in (which
// catches edited key files). Failures are recorded quietly.
func (a *App) backgroundSync(ctx context.Context, client *update.Client) {
	if a.accountToken() == "" || !a.currentSettings().SyncAuto || a.privacyPending() {
		return
	}
	a.update.acct.mu.Lock()
	off := a.update.acct.disabled["sync"]
	a.update.acct.mu.Unlock()
	if off {
		return
	}
	book := a.readBook()
	if items, err := a.localChanges(book); err != nil || len(items) == 0 {
		return
	}
	if _, err := a.pushChanges(ctx, client); err != nil {
		a.log.Debug("background sync failed; will retry", "err", err)
	}
	a.emitAccount()
}

// ---- password rules ----------------------------------------------------------

// minPasswordLength is the floor the server never goes below; the admin may
// require more, which the server then reports as weak_password.
const minPasswordLength = 10

var commonPasswordWords = []string{"password", "qwerty", "123456", "mango3s"}

// PasswordProblems mirrors the server's rules, so most mistakes show up
// before anything is sent.
func PasswordProblems(password, email string) []string {
	var problems []string
	if len([]rune(password)) < minPasswordLength {
		problems = append(problems, fmt.Sprintf("Use at least %d characters (12 or more is best).", minPasswordLength))
	}
	classes := 0
	for _, f := range []func(rune) bool{
		func(r rune) bool { return r >= 'a' && r <= 'z' },
		func(r rune) bool { return r >= 'A' && r <= 'Z' },
		func(r rune) bool { return r >= '0' && r <= '9' },
		func(r rune) bool {
			return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
		},
	} {
		if strings.IndexFunc(password, f) >= 0 {
			classes++
		}
	}
	if classes < 3 {
		problems = append(problems, "Mix at least three of: lowercase, uppercase, numbers, symbols.")
	}
	lower := strings.ToLower(password)
	if local, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@"); len(local) >= 3 && strings.Contains(lower, local) {
		problems = append(problems, "Don't include your email name.")
	}
	runes := []rune(password)
	for i := 0; i+3 < len(runes); i++ {
		if runes[i] == runes[i+1] && runes[i] == runes[i+2] && runes[i] == runes[i+3] {
			problems = append(problems, "Avoid repeating the same character.")
			break
		}
	}
	for _, w := range commonPasswordWords {
		if strings.Contains(lower, w) {
			problems = append(problems, "Avoid common words and sequences.")
			break
		}
	}
	return problems
}

func checkPassword(password, email string) error {
	if p := PasswordProblems(password, email); len(p) > 0 {
		return &apperr.Error{Code: apperr.CodeInvalidInput, Message: strings.Join(p, " "), Detail: "weak_password"}
	}
	return nil
}
