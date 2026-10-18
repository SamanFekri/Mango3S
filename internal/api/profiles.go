package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"os"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/documents"
	"mango3s/internal/mongox"
	"mango3s/internal/recent"
	"mango3s/internal/secrets"
	"mango3s/internal/tools"
)

// ---- connection profiles --------------------------------------------------

// SecretInput changes one stored secret: a non-empty Value replaces it,
// Clear removes it, and an empty input keeps what is stored.
type SecretInput struct {
	Value string `json:"value"`
	Clear bool   `json:"clear"`
}

// ProfileInput is the connection dialog (all nine tabs). Password semantics
// match SecretInput: a non-empty Password replaces the stored one;
// ClearPassword removes it; otherwise the stored password is kept.
type ProfileInput struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	Folder                string                 `json:"folder"`
	URI                   string                 `json:"uri"`
	Server                config.ServerConfig    `json:"server"`
	Username              string                 `json:"username"`
	Password              string                 `json:"password"`
	ClearPassword         bool                   `json:"clearPassword"`
	AuthSource            string                 `json:"authSource"`
	AuthMechanism         string                 `json:"authMechanism"`
	AuthProperties        string                 `json:"authProperties"`
	ConnectTimeoutSeconds int                    `json:"connectTimeoutSeconds"`
	ReadOnly              bool                   `json:"readOnly"`
	TLS                   config.TLSConfig       `json:"tls"`
	SSH                   config.SSHConfig       `json:"ssh"`
	Proxy                 config.ProxyConfig     `json:"proxy"`
	Shell                 config.ShellConfig     `json:"shell"`
	Tools                 config.ToolsConfig     `json:"tools"`
	Advanced              config.AdvancedConfig  `json:"advanced"`
	Secrets               map[string]SecretInput `json:"secrets"` // sshPassword, sshPassphrase, proxyPassword, tlsKeyPassword, awsSessionToken
}

// toProfile builds the stored profile. In manual mode the connection string
// is generated from the Server tab; in URI mode credentials are stripped out
// of it (the explicit form fields take precedence).
func (in ProfileInput) toProfile() (config.Profile, string, error) {
	username := strings.TrimSpace(in.Username)
	password := in.Password
	uri := strings.TrimSpace(in.URI)
	if in.Server.Mode == config.ServerModeManual {
		n := config.Profile{Server: in.Server}.Normalize()
		built, err := config.BuildURI(n.Server)
		if err != nil {
			return config.Profile{}, "", err
		}
		uri = built
		in.Server = n.Server
	} else {
		stripped, user, pass, err := config.SplitCredentials(uri)
		if err != nil {
			return config.Profile{}, "", err
		}
		uri = stripped
		if username == "" {
			username = user
		} else if user != "" && user != username {
			return config.Profile{}, "", apperr.Invalid("Authentication: the username in the connection string (%q) differs from the Username field (%q).", user, username)
		}
		if password == "" {
			password = pass
		}
	}
	p := config.Profile{
		ID:                    in.ID,
		Name:                  strings.TrimSpace(in.Name),
		Folder:                in.Folder,
		URI:                   uri,
		Server:                in.Server,
		Username:              username,
		AuthSource:            strings.TrimSpace(in.AuthSource),
		AuthMechanism:         strings.TrimSpace(in.AuthMechanism),
		AuthProperties:        strings.TrimSpace(in.AuthProperties),
		ConnectTimeoutSeconds: in.ConnectTimeoutSeconds,
		ReadOnly:              in.ReadOnly,
		TLS:                   in.TLS,
		SSH:                   in.SSH,
		Proxy:                 in.Proxy,
		Shell:                 in.Shell,
		Tools:                 in.Tools,
		Advanced:              in.Advanced,
	}.Normalize()
	if strings.Contains(p.AuthProperties, "AWS_SESSION_TOKEN") {
		return config.Profile{}, "", apperr.Invalid("Authentication: enter the AWS session token in its own (secret) field, not in the mechanism properties.")
	}
	return p, password, p.Validate()
}

// ListProfiles returns saved connection profiles (never passwords).
func (a *App) ListProfiles() ([]config.Profile, error) {
	list, err := a.store.ListProfiles()
	return list, a.encode("ListProfiles", err)
}

// SaveProfile creates or updates a profile; secrets go to the credential store.
func (a *App) SaveProfile(in ProfileInput) (config.Profile, error) {
	saved, err := a.saveProfile(in)
	return saved, a.encode("SaveProfile", err)
}

func (a *App) saveProfile(in ProfileInput) (config.Profile, error) {
	p, password, err := in.toProfile()
	if err != nil {
		return config.Profile{}, err
	}
	if p.ID != "" {
		existing, err := a.store.GetProfile(p.ID)
		if err != nil {
			return config.Profile{}, err
		}
		p.HasPassword = existing.HasPassword
		p.StoredSecrets = existing.StoredSecrets
		p.LastConnectedAt = existing.LastConnectedAt
	}
	saved, err := a.store.SaveProfile(p)
	if err != nil {
		return config.Profile{}, err
	}
	changes := map[string]SecretInput{config.SecretPassword: {Value: password, Clear: in.ClearPassword}}
	for k, v := range in.Secrets {
		if k == config.SecretPassword {
			continue
		}
		changes[k] = v
	}
	before := fmt.Sprint(saved.HasPassword, saved.StoredSecrets)
	for _, kind := range config.SecretKinds {
		ch, ok := changes[kind]
		if !ok {
			continue
		}
		key := config.SecretKey(saved.ID, kind)
		switch {
		case ch.Value != "":
			if err := a.secrets.Set(key, ch.Value); err != nil {
				return saved, apperr.Wrap(apperr.CodeSecretBackend, err, "The profile was saved, but a secret could not be stored securely.")
			}
			saved.SetSecretFlag(kind, true)
		case ch.Clear:
			if err := a.secrets.Delete(key); err != nil && !errors.Is(err, secrets.ErrNotFound) {
				return saved, apperr.Wrap(apperr.CodeSecretBackend, err, "A stored secret could not be removed.")
			}
			saved.SetSecretFlag(kind, false)
		}
	}
	if fmt.Sprint(saved.HasPassword, saved.StoredSecrets) != before {
		if saved, err = a.store.SaveProfile(saved); err != nil {
			return config.Profile{}, err
		}
	}
	a.log.Info("profile saved", "profile", saved.Name, "uri", saved.URI, "hasPassword", saved.HasPassword, "secrets", saved.StoredSecrets)
	return saved, nil
}

// DuplicateProfile copies a profile, its stored secrets and its collection
// history under a new name.
func (a *App) DuplicateProfile(id string) (config.Profile, error) {
	src, err := a.store.GetProfile(id)
	if err != nil {
		return config.Profile{}, a.encode("DuplicateProfile", err)
	}
	list, _ := a.store.ListProfiles()
	names := map[string]bool{}
	for _, p := range list {
		names[p.Name] = true
	}
	name := src.Name + " (copy)"
	for i := 2; names[name]; i++ {
		name = fmt.Sprintf("%s (copy %d)", src.Name, i)
	}
	cp := src
	cp.ID, cp.Name, cp.LastConnectedAt = "", name, nil
	cp.HasPassword, cp.StoredSecrets = false, nil
	saved, err := a.store.SaveProfile(cp)
	if err != nil {
		return config.Profile{}, a.encode("DuplicateProfile", err)
	}
	for _, kind := range config.SecretKinds {
		if !src.HasSecret(kind) {
			continue
		}
		v, err := a.secrets.Get(config.SecretKey(src.ID, kind))
		if err != nil {
			continue // a missing secret is reported when connecting
		}
		if err := a.secrets.Set(config.SecretKey(saved.ID, kind), v); err != nil {
			return saved, a.encode("DuplicateProfile", apperr.Wrap(apperr.CodeSecretBackend, err, "The copy was saved, but its secrets could not be stored."))
		}
		saved.SetSecretFlag(kind, true)
	}
	if saved, err = a.store.SaveProfile(saved); err != nil {
		return config.Profile{}, a.encode("DuplicateProfile", err)
	}
	_ = a.recent.Copy(src.ID, saved.ID)
	return saved, nil
}

// DeleteProfile disconnects and removes a profile with its secrets and history.
func (a *App) DeleteProfile(id string) error {
	_ = a.conns.Disconnect(a.ctx, id)
	for _, kind := range config.SecretKinds {
		if err := a.secrets.Delete(config.SecretKey(id, kind)); err != nil && !errors.Is(err, secrets.ErrNotFound) {
			a.log.Warn("could not delete stored secret", "kind", kind, "err", err)
		}
	}
	_ = a.recent.Clear(id)
	return a.encode("DeleteProfile", a.store.DeleteProfile(id))
}

var secretLabels = map[string]string{
	config.SecretPassword:        "password",
	config.SecretSSHPassword:     "SSH password",
	config.SecretSSHPassphrase:   "SSH key passphrase",
	config.SecretProxyPassword:   "proxy password",
	config.SecretTLSKeyPassword:  "TLS key password",
	config.SecretAWSSessionToken: "AWS session token",
}

func (a *App) storedSecret(p config.Profile, kind string) (string, error) {
	if p.ID == "" || !p.HasSecret(kind) {
		return "", nil
	}
	v, err := a.secrets.Get(config.SecretKey(p.ID, kind))
	if errors.Is(err, secrets.ErrNotFound) {
		return "", apperr.New(apperr.CodeSecretBackend, "The saved %s for %q is missing. Edit the connection and enter it again.", secretLabels[kind], p.Name)
	}
	if err != nil {
		return "", apperr.Wrap(apperr.CodeSecretBackend, err, "The saved "+secretLabels[kind]+" could not be read from the secret store.")
	}
	return v, nil
}

// credentials collects a profile's secrets; form values (when testing
// unsaved changes) take precedence over stored ones.
func (a *App) credentials(p config.Profile, form map[string]SecretInput) (mongox.Credentials, error) {
	get := func(kind string) (string, error) {
		if in, ok := form[kind]; ok {
			if in.Value != "" {
				return in.Value, nil
			}
			if in.Clear {
				return "", nil
			}
		}
		return a.storedSecret(p, kind)
	}
	var c mongox.Credentials
	var err error
	for _, f := range []struct {
		kind string
		dst  *string
	}{
		{config.SecretPassword, &c.Password}, {config.SecretSSHPassword, &c.SSHPassword},
		{config.SecretSSHPassphrase, &c.SSHPassphrase}, {config.SecretProxyPassword, &c.ProxyPassword},
		{config.SecretTLSKeyPassword, &c.TLSKeyPassword}, {config.SecretAWSSessionToken, &c.AWSSessionToken},
	} {
		if *f.dst, err = get(f.kind); err != nil {
			return c, err
		}
	}
	return c, nil
}

// TestConnection checks the form values without saving them.
func (a *App) TestConnection(in ProfileInput) (mongox.TestResult, error) {
	return a.testConnection("", in, nil)
}

// TestConnectionWithLog is TestConnection, sending each step (settings,
// DNS, tunnel, reachability, TLS, sign-in, server info) to the UI as a
// "conn:step" event tagged with opID, so the connection log dialog can show
// where it fails.
func (a *App) TestConnectionWithLog(opID string, in ProfileInput) (mongox.TestResult, error) {
	return a.testConnection(opID, in, a.stepReporter(opID, "test", in.Name))
}

// ConnectWithLog is Connect with the same step-by-step log.
func (a *App) ConnectWithLog(opID, profileID string) (mongox.SessionInfo, error) {
	name := profileID
	if p, err := a.store.GetProfile(profileID); err == nil {
		name = p.Name
	}
	return a.connect(profileID, a.stepReporter(opID, "connect", name))
}

// ConnectionStep is the "conn:step" event payload.
type ConnectionStep struct {
	OpID string      `json:"opId"`
	Step mongox.Step `json:"step"`
}

// stepReporter sends steps to the UI and writes failures to the log (the
// steps carry no secrets).
func (a *App) stepReporter(opID, action, name string) mongox.Reporter {
	return func(st mongox.Step) {
		switch st.State {
		case "failed":
			a.log.Warn("connection step failed", "action", action, "profile", name, "step", st.ID, "error", st.Error, "ms", st.DurationMs)
		case "ok", "warning":
			a.log.Debug("connection step", "action", action, "profile", name, "step", st.ID, "state", st.State, "ms", st.DurationMs)
		}
		if a.emitStep != nil {
			a.emitStep(ConnectionStep{OpID: opID, Step: st})
		}
	}
}

// SetStepListener receives connection steps (the desktop runtime forwards
// them to the UI; tests record them). A function, not a method, so Wails
// does not bind it.
func SetStepListener(a *App, f func(ConnectionStep)) { a.emitStep = f }

func (a *App) testConnection(opID string, in ProfileInput, rep mongox.Reporter) (mongox.TestResult, error) {
	return call(a, "TestConnection", opID, func(ctx context.Context) (mongox.TestResult, error) {
		p, password, err := in.toProfile()
		if err != nil {
			return mongox.TestResult{}, err
		}
		stored := config.Profile{ID: p.ID, Name: p.Name}
		if p.ID != "" {
			if existing, err := a.store.GetProfile(p.ID); err == nil {
				stored = existing
			}
		}
		form := map[string]SecretInput{config.SecretPassword: {Value: password, Clear: in.ClearPassword}}
		for k, v := range in.Secrets {
			form[k] = v
		}
		c, err := a.credentials(stored, form)
		if err != nil {
			return mongox.TestResult{}, err
		}
		if p.Username == "" {
			c.Password = ""
		}
		return a.conns.TestWithLog(ctx, p, c, rep)
	})
}

// Connect opens a saved profile.
func (a *App) Connect(profileID string) (mongox.SessionInfo, error) {
	return a.connect(profileID, nil)
}

func (a *App) connect(profileID string, rep mongox.Reporter) (mongox.SessionInfo, error) {
	return call(a, "Connect", "connect:"+profileID, func(ctx context.Context) (mongox.SessionInfo, error) {
		p, err := a.store.GetProfile(profileID)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		c, err := a.credentials(p, nil)
		if err != nil {
			if rep != nil {
				rep(mongox.Step{ID: "settings", Label: "Read saved credentials", State: "failed", Error: apperr.From(err).Message, Code: string(apperr.From(err).Code)})
			}
			return mongox.SessionInfo{}, err
		}
		s, err := a.conns.ConnectWithLog(ctx, p, c, rep)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		if err := a.store.TouchConnected(p.ID, time.Now()); err != nil {
			a.log.Warn("could not record last connection time", "err", err)
		}
		return mongox.SessionInfo{ID: s.ProfileID, Name: s.Name, ReadOnly: s.ReadOnly,
			ServerVersion: s.ServerVersion, Topology: s.Topology, ConnectedAt: s.ConnectedAt}, nil
	})
}

// connectProfile opens (or reuses) a session with the stored credentials.
// Used by Connect and by background work (scheduled tasks, MCP).
func (a *App) connectProfile(ctx context.Context, p config.Profile) (*mongox.Session, error) {
	c, err := a.credentials(p, nil)
	if err != nil {
		return nil, err
	}
	return a.conns.Connect(ctx, p, c)
}

// ParseConnectionString reads a connection string into Server tab fields.
// The password, if present, is reported but never returned.
func (a *App) ParseConnectionString(uri string) (config.ParsedURI, error) {
	p, err := config.ParseURI(uri)
	return p, a.encode("ParseConnectionString", err)
}

// BuildConnectionString renders the manual Server fields as a URI (no credentials).
func (a *App) BuildConnectionString(s config.ServerConfig) (string, error) {
	uri, err := config.BuildURI(config.Profile{Server: s}.Normalize().Server)
	return uri, a.encode("BuildConnectionString", err)
}

// DetectTools finds the MongoDB utilities using the dialog's Tools settings.
// connID, when connected, adds compatibility notes for that server.
func (a *App) DetectTools(cfg config.ToolsConfig, connID string) []tools.Info {
	sv := ""
	if s, err := a.conns.Session(connID); err == nil {
		sv = s.ServerVersion
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return tools.Detect(ctx, cfg, sv)
}

// Capability says whether an option works in this build and environment.
type Capability struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Supported bool   `json:"supported"`
	Note      string `json:"note,omitempty"`
}

// ConnectionCapabilities lists options the dialog offers, marking the ones
// this build can't support so the UI never pretends they work.
func (a *App) ConnectionCapabilities() map[string][]Capability {
	return map[string][]Capability{
		"authMechanisms": {
			{config.MechDefault, "Default (SCRAM negotiated)", true, ""},
			{config.MechSCRAM256, "SCRAM-SHA-256", true, ""},
			{config.MechSCRAM1, "SCRAM-SHA-1", true, ""},
			{config.MechX509, "X.509 certificate", true, "Needs a client certificate on the SSL / TLS tab."},
			{config.MechPLAIN, "LDAP (PLAIN)", true, "MongoDB Enterprise / Atlas. Sends the password in clear text unless TLS is on."},
			{config.MechAWS, "AWS IAM", true, "Access key ID as username, secret key as password; or leave both empty to use the environment / instance role."},
			{config.MechOIDC, "OIDC (workload identity)", true, "Machine workflows only: set ENVIRONMENT:azure, gcp or k8s (and TOKEN_RESOURCE) in mechanism properties. Interactive browser login is not supported."},
			{config.MechGSSAPI, "Kerberos (GSSAPI)", false, "Not available: the Go driver needs the gssapi build tag and system Kerberos libraries."},
		},
		"sshAuth": {
			{config.SSHAuthPassword, "Password", true, "Also answers keyboard-interactive password prompts."},
			{config.SSHAuthKey, "Private key", true, "OpenSSH or PEM keys (RSA, ECDSA, Ed25519), optionally passphrase-protected."},
			{config.SSHAuthAgent, "SSH agent", agentSupported(), agentNote()},
			{"keyboardInteractive", "Keyboard-interactive (2FA, OTP)", false, "Not supported: there is no prompt flow for one-time codes."},
		},
		"proxy": {
			{config.ProxyNone, "None", true, ""},
			{config.ProxySOCKS5, "SOCKS5", true, "Host names are resolved by the proxy."},
			{config.ProxyHTTP, "HTTP (CONNECT)", true, "Basic authentication only."},
			{"socks4", "SOCKS4", false, "Not supported."},
		},
		"advanced": {
			{"socketTimeout", "Socket timeout", false, "Removed in Go driver v2; use the operation timeout (timeoutMS) instead."},
			{"wtimeout", "Write concern timeout (wtimeout)", false, "Removed in Go driver v2; use the operation timeout."},
		},
	}
}

func agentSupported() bool { return goos() != "windows" }
func agentNote() string {
	if agentSupported() {
		return "Uses the agent at SSH_AUTH_SOCK."
	}
	return "The Windows OpenSSH agent pipe is not supported; use a private key file."
}

// ---- collection history ---------------------------------------------------

// RecordCollectionOpened adds db.coll to a connection's collection history.
func (a *App) RecordCollectionOpened(connID, db, coll string) error {
	return a.encode("RecordCollectionOpened", a.recent.Record(connID, db, coll))
}

// ListCollectionHistory returns a connection's recently opened collections.
func (a *App) ListCollectionHistory(profileID string) ([]recent.Entry, error) {
	l, err := a.recent.List(profileID)
	return l, a.encode("ListCollectionHistory", err)
}

// ClearCollectionHistory forgets a connection's collection history.
func (a *App) ClearCollectionHistory(profileID string) error {
	return a.encode("ClearCollectionHistory", a.recent.Clear(profileID))
}

// ---- field-level editing ----------------------------------------------------

// GetField reads one field of a document for in-place editing.
func (a *App) GetField(connID, db, coll, id string, path []string) (documents.FieldValue, error) {
	return call(a, "GetField", "", func(ctx context.Context) (documents.FieldValue, error) {
		return a.docs.GetField(ctx, connID, db, coll, id, path)
	})
}

// UpdateField changes or removes one field with $set / $unset.
func (a *App) UpdateField(u documents.FieldUpdate) (documents.FieldResult, error) {
	res, err := call(a, "UpdateField", "", func(ctx context.Context) (documents.FieldResult, error) {
		return a.docs.UpdateField(ctx, u)
	})
	if err == nil {
		a.log.Info("field updated", "db", u.DB, "coll", u.Coll, "path", strings.Join(u.Path, "."), "unset", u.Unset)
	}
	return res, err
}

// ---- unsaved work -------------------------------------------------------------

// SetUnsavedWork tells the backend whether any tab has unsaved changes, so
// closing the window can ask first.
func (a *App) SetUnsavedWork(dirty bool) { a.unsavedWork.Store(dirty) }

func goos() string { return runtime.GOOS }

// MenuEvent forwards a native menu click to the UI as "menu:<name>". It is a
// function, not a method, so Wails does not bind it.
func MenuEvent(a *App, name string) {
	wruntime.EventsEmit(a.ctx, "menu:"+name)
}

// BeforeClose asks before quitting while the UI reports unsaved work.
// It returns true to keep the window open.
func BeforeClose(a *App) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		if !a.unsavedWork.Load() {
			return false
		}
		choice, err := wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
			Type:          wruntime.QuestionDialog,
			Title:         "Unsaved changes",
			Message:       "Some tabs have unsaved changes. Quit and discard them?",
			Buttons:       []string{"Quit", "Cancel"},
			DefaultButton: "Cancel",
			CancelButton:  "Cancel",
		})
		if err != nil {
			return true
		}
		return choice != "Quit" && choice != "Yes"
	}
}

// ChooseFile opens a native picker for connection files. kind is "cert",
// "key", "tool" or "folder". No extension filter is applied: SSH keys
// (id_ed25519), certificates and tools often have no or unusual extensions,
// and macOS treats filters as a strict allow-list. An empty result means canceled.
func (a *App) ChooseFile(kind, title string) (string, error) {
	if kind == "folder" {
		p, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title, ShowHiddenFiles: true})
		return p, a.encode("ChooseFile", err)
	}
	// Hidden files are shown so ~/.ssh and dotfiles can be browsed.
	p, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: title, ShowHiddenFiles: true})
	return p, a.encode("ChooseFile", err)
}

// ExportTextFile asks where to save content (e.g. an exported keymap) and
// writes it. An empty path means the user canceled.
func (a *App) ExportTextFile(title, defaultName, content string) (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{Title: title, DefaultFilename: defaultName})
	if err != nil || path == "" {
		return "", a.encode("ExportTextFile", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", a.encode("ExportTextFile", apperr.Wrap(apperr.CodeInternal, err, "Could not write the file."))
	}
	return path, nil
}

// ImportTextFile asks for a file and returns its content (at most 1 MB).
// An empty result means the user canceled.
func (a *App) ImportTextFile(title string) (string, error) {
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
	if err != nil || path == "" {
		return "", a.encode("ImportTextFile", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", a.encode("ImportTextFile", apperr.Wrap(apperr.CodeNotFound, err, "Could not read the file."))
	}
	if st.Size() > 1<<20 {
		return "", a.encode("ImportTextFile", apperr.Invalid("The file is larger than 1 MB; it is not a keymap."))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", a.encode("ImportTextFile", apperr.Wrap(apperr.CodeInternal, err, "Could not read the file."))
	}
	return string(b), nil
}

// QuitApp quits like ⌘Q, asking first when tabs have unsaved work
// (BeforeClose). A function, not a method, so Wails does not bind it.
func QuitApp(a *App) { wruntime.Quit(a.ctx) }

// HideApp hides the application (macOS ⌘H).
func HideApp(a *App) { wruntime.Hide(a.ctx) }
