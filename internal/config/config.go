// Package config persists application settings and connection profiles as
// JSON files in the user's configuration directory. Passwords are never
// stored here; see package secrets.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"mango3s/internal/apperr"
)

// EnvConfigDir overrides the configuration directory (tests, Docker, portable installs).
const EnvConfigDir = "MANGO3S_CONFIG_DIR"

// Dir returns the configuration directory, creating it with 0700 permissions.
func Dir() (string, error) {
	dir := os.Getenv(EnvConfigDir)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "Mango3S")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Settings are user preferences.
type Settings struct {
	Theme               string `json:"theme"` // system | light | dark
	PageSize            int    `json:"pageSize"`
	QueryTimeoutSeconds int    `json:"queryTimeoutSeconds"`
	ConfirmDeletes      bool   `json:"confirmDeletes"`
	DefaultView         string `json:"defaultView"` // tree | table | json
	// DefaultViewChosen is set when the user picks a default view in
	// Settings; until then Tree is used (older files saved "json" without
	// the user ever choosing it).
	DefaultViewChosen bool   `json:"defaultViewChosen"`
	EditorFontSize    int    `json:"editorFontSize"`
	LogLevel          string `json:"logLevel"` // debug | info | warn | error

	// AI-assisted query generation (off until the user enables it).
	AIEnabled      bool   `json:"aiEnabled"`
	AIModel        string `json:"aiModel"`
	AIAllowSamples bool   `json:"aiAllowSamples"` // allow sending masked sample documents
	// AIProvider picks who generates queries: anthropic (API key),
	// claudeCode / codex (the user's installed CLI and its sign-in) or local
	// (an Ollama or OpenAI-compatible server, e.g. LM Studio, llama.cpp).
	AIProvider       string `json:"aiProvider"`
	AITimeoutSeconds int    `json:"aiTimeoutSeconds"`
	ClaudeCodePath   string `json:"claudeCodePath"`  // empty = find it
	ClaudeCodeModel  string `json:"claudeCodeModel"` // empty = the CLI's default; e.g. sonnet, opus
	CodexPath        string `json:"codexPath"`
	CodexModel       string `json:"codexModel"`
	LocalURL         string `json:"localUrl"` // e.g. http://localhost:11434
	LocalAPI         string `json:"localApi"` // ollama | openai
	LocalModel       string `json:"localModel"`
	SchedulerEnabled bool   `json:"schedulerEnabled"` // run approved scheduled tasks while the app is open

	// Platform check-in (the web admin platform this build was distributed from): automatic
	// update checks/downloads and in-app notifications. DeviceFingerprint and
	// UpdateInstallationId are non-secret identifiers; the client key and installation token
	// themselves live in the OS credential store (see package secrets), never here.
	AutoUpdateEnabled    bool   `json:"autoUpdateEnabled"`
	DeviceFingerprint    string `json:"deviceFingerprint,omitempty"`
	UpdateInstallationID string `json:"updateInstallationId,omitempty"`

	// SyncAuto backs connections up to the signed-in Mango3S account in the
	// background (encrypted on the server; see internal/api/account.go).
	SyncAuto bool `json:"syncAuto"`

	// ShowMascot shows Mango, the animated mascot, at the bottom of the explorer.
	ShowMascot bool `json:"showMascot"`

	// Keymap holds customized shortcuts (Preferences → Keymap) per platform
	// ("mac" | "other") and action id. An entry replaces that action's
	// default bindings; an empty list removes all of them.
	Keymap map[string]map[string][]string `json:"keymap,omitempty"`
}

// AI providers.
const (
	AIProviderAnthropic  = "anthropic"
	AIProviderClaudeCode = "claudeCode"
	AIProviderCodex      = "codex"
	AIProviderLocal      = "local"
)

// DefaultSettings returns the factory defaults.
func DefaultSettings() Settings {
	return Settings{
		Theme:               "system",
		PageSize:            50,
		QueryTimeoutSeconds: 60,
		ConfirmDeletes:      true,
		DefaultView:         "tree",
		EditorFontSize:      13,
		LogLevel:            "info",
		AIModel:             "claude-opus-5-5",
		AIProvider:          AIProviderAnthropic,
		AITimeoutSeconds:    180,
		LocalURL:            "http://localhost:11434",
		LocalAPI:            "ollama",
		SchedulerEnabled:    true,
		AutoUpdateEnabled:   true,
		SyncAuto:            true,
		ShowMascot:          true,
	}
}

// Normalize clamps values into supported ranges.
func (s Settings) Normalize() Settings {
	d := DefaultSettings()
	if s.Theme != "light" && s.Theme != "dark" {
		s.Theme = d.Theme
	}
	if s.PageSize <= 0 {
		s.PageSize = d.PageSize
	}
	if s.PageSize > 1000 {
		s.PageSize = 1000
	}
	if s.QueryTimeoutSeconds <= 0 {
		s.QueryTimeoutSeconds = d.QueryTimeoutSeconds
	}
	if s.QueryTimeoutSeconds > 3600 {
		s.QueryTimeoutSeconds = 3600
	}
	if !s.DefaultViewChosen || (s.DefaultView != "table" && s.DefaultView != "json") {
		s.DefaultView = "tree"
	}
	if s.EditorFontSize < 9 || s.EditorFontSize > 28 {
		s.EditorFontSize = d.EditorFontSize
	}
	s.AIModel = strings.TrimSpace(s.AIModel)
	if s.AIModel == "" {
		s.AIModel = d.AIModel
	}
	s.Keymap = normalizeKeymap(s.Keymap)
	switch s.AIProvider {
	case AIProviderAnthropic, AIProviderClaudeCode, AIProviderCodex, AIProviderLocal:
	default:
		s.AIProvider = d.AIProvider
	}
	if s.AITimeoutSeconds < 10 || s.AITimeoutSeconds > 1800 {
		s.AITimeoutSeconds = d.AITimeoutSeconds
	}
	s.LocalURL = strings.TrimRight(strings.TrimSpace(s.LocalURL), "/")
	if s.LocalURL == "" {
		s.LocalURL = d.LocalURL
	}
	if s.LocalAPI != "openai" {
		s.LocalAPI = "ollama"
	}
	for _, f := range []*string{
		&s.ClaudeCodePath, &s.ClaudeCodeModel, &s.CodexPath, &s.CodexModel, &s.LocalModel,
		&s.DeviceFingerprint, &s.UpdateInstallationID,
	} {
		*f = strings.TrimSpace(*f)
	}
	switch s.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		s.LogLevel = d.LogLevel
	}
	return s
}

// MaxBindings is how many shortcuts one action can have.
const MaxBindings = 3

func normalizeKeymap(k map[string]map[string][]string) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for plat, actions := range k {
		if plat != "mac" && plat != "other" {
			continue
		}
		m := map[string][]string{}
		for id, binds := range actions {
			if id == "" || len(id) > 64 {
				continue
			}
			clean := []string{}
			seen := map[string]bool{}
			for _, b := range binds {
				b = strings.TrimSpace(b)
				if b == "" || len(b) > 40 || seen[b] || len(clean) == MaxBindings {
					continue
				}
				seen[b] = true
				clean = append(clean, b)
			}
			m[id] = clean
		}
		if len(m) > 0 {
			out[plat] = m
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Profile is a saved connection. URI never contains a password.
type Profile struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	URI                   string    `json:"uri"`
	Username              string    `json:"username,omitempty"`
	AuthSource            string    `json:"authSource,omitempty"`
	AuthMechanism         string    `json:"authMechanism,omitempty"`
	ConnectTimeoutSeconds int       `json:"connectTimeoutSeconds"`
	ReadOnly              bool      `json:"readOnly"`
	HasPassword           bool      `json:"hasPassword"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`

	// Connection manager tabs (see profile.go). Zero values keep the
	// behavior of profiles saved before these existed.
	Folder string `json:"folder,omitempty"`
	// Position is the user's order (drag to reorder); 0 = not placed yet,
	// listed after placed profiles, by name.
	Position        int            `json:"position,omitempty"`
	Server          ServerConfig   `json:"server"`
	AuthProperties  string         `json:"authProperties,omitempty"` // authMechanismProperties, e.g. ENVIRONMENT:azure
	TLS             TLSConfig      `json:"tls"`
	SSH             SSHConfig      `json:"ssh"`
	Proxy           ProxyConfig    `json:"proxy"`
	Shell           ShellConfig    `json:"shell"`
	Tools           ToolsConfig    `json:"tools"`
	Advanced        AdvancedConfig `json:"advanced"`
	StoredSecrets   []string       `json:"storedSecrets,omitempty"` // secret kinds other than the password
	LastConnectedAt *time.Time     `json:"lastConnectedAt,omitempty"`
}

// SplitCredentials removes user info from a MongoDB URI and returns it
// separately, so the password can be moved to the secret store.
func SplitCredentials(uri string) (stripped, username, password string, err error) {
	uri = strings.TrimSpace(uri)
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return "", "", "", apperr.Invalid("Connection string must start with mongodb:// or mongodb+srv://")
	}
	rest := uri[schemeEnd+3:]
	authorityEnd := strings.IndexAny(rest, "/?")
	if authorityEnd < 0 {
		authorityEnd = len(rest)
	}
	authority := rest[:authorityEnd]
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return uri, "", "", nil
	}
	userInfo := authority[:at]
	user, pass, hasPass := strings.Cut(userInfo, ":")
	if username, err = url.PathUnescape(user); err != nil {
		return "", "", "", apperr.Invalid("Invalid percent-encoding in username.")
	}
	if hasPass {
		if password, err = url.PathUnescape(pass); err != nil {
			return "", "", "", apperr.Invalid("Invalid percent-encoding in password.")
		}
	}
	return uri[:schemeEnd+3] + authority[at+1:] + rest[authorityEnd:], username, password, nil
}

// Store reads and writes settings.json and connections.json.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore returns a store rooted at dir.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// Dir returns the store's directory.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

// LoadSettings returns saved settings or defaults.
func (s *Store) LoadSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := DefaultSettings()
	if err := readJSON(s.path("settings.json"), &st); err != nil {
		return DefaultSettings(), err
	}
	return st.Normalize(), nil
}

// SaveSettings normalizes and persists settings.
func (s *Store) SaveSettings(st Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st = st.Normalize()
	return st, writeJSON(s.path("settings.json"), st)
}

type profilesFile struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// ListProfiles returns all profiles sorted by name.
func (s *Store) ListProfiles() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *Store) listLocked() ([]Profile, error) {
	var f profilesFile
	if err := readJSON(s.path("connections.json"), &f); err != nil {
		return nil, err
	}
	sortProfiles(f.Profiles)
	if f.Profiles == nil {
		f.Profiles = []Profile{}
	}
	return f.Profiles, nil
}

// GetProfile returns one profile.
func (s *Store) GetProfile(id string) (Profile, error) {
	list, err := s.ListProfiles()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, apperr.New(apperr.CodeNotFound, "Connection profile not found.")
}

// SaveProfile inserts (empty ID) or updates a profile and returns it.
func (s *Store) SaveProfile(p Profile) (Profile, error) {
	p = p.Normalize()
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return Profile{}, err
	}
	now := time.Now().UTC()
	p.UpdatedAt = now
	if p.ConnectTimeoutSeconds <= 0 {
		p.ConnectTimeoutSeconds = 10
	}
	found := false
	if p.ID != "" {
		for i := range list {
			if list[i].ID == p.ID {
				p.CreatedAt = list[i].CreatedAt
				if p.LastConnectedAt == nil {
					p.LastConnectedAt = list[i].LastConnectedAt
				}
				if p.Position == 0 {
					p.Position = list[i].Position // edits keep the place in the list
				}
				list[i] = p
				found = true
			}
		}
		if !found {
			return Profile{}, apperr.New(apperr.CodeNotFound, "Connection profile not found.")
		}
	} else {
		p.ID = NewID()
		p.CreatedAt = now
		p.Position = 0
		if maxPos(list) > 0 {
			p.Position = maxPos(list) + 1 // new connections go to the end of a custom order
		}
		list = append(list, p)
	}
	return p, writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
}

// sortProfiles orders by the user's position, then name (unplaced last).
func sortProfiles(l []Profile) {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if (a.Position == 0) != (b.Position == 0) {
			return a.Position != 0
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

func maxPos(l []Profile) int {
	m := 0
	for _, p := range l {
		if p.Position > m {
			m = p.Position
		}
	}
	return m
}

// ReorderProfiles stores the user's order: ids first, in that order; any
// profile not listed keeps its relative place after them.
func (s *Store) ReorderProfiles(ids []string) ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	pos := map[string]int{}
	for _, id := range ids {
		if _, dup := pos[id]; !dup {
			pos[id] = len(pos) + 1
		}
	}
	next := len(pos)
	for i := range list {
		if p, ok := pos[list[i].ID]; ok {
			list[i].Position = p
		} else {
			next++
			list[i].Position = next
		}
	}
	sortProfiles(list)
	return list, writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
}

// SetProfileFolder moves a profile into a folder ("" = top level).
func (s *Store) SetProfileFolder(id, folder string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return Profile{}, err
	}
	for i := range list {
		if list[i].ID == id {
			list[i].Folder = cleanFolder(folder)
			list[i].UpdatedAt = time.Now().UTC()
			return list[i], writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
		}
	}
	return Profile{}, apperr.New(apperr.CodeNotFound, "Connection profile not found.")
}

// TouchConnected records a successful connection (for "recent connections").
func (s *Store) TouchConnected(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID == id {
			t := at.UTC()
			list[i].LastConnectedAt = &t
			return writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
		}
	}
	return nil
}

// DeleteProfile removes a profile; deleting a missing profile is not an error.
// PutProfile stores a profile under its own ID and UpdatedAt, adding it if
// it doesn't exist yet. It is for profiles that come from elsewhere (account
// sync), where the ID and timestamp must match the other devices; the local
// list position and last-connected time are kept.
func (s *Store) PutProfile(p Profile) (Profile, error) {
	p = p.Normalize()
	if p.ID == "" {
		return Profile{}, apperr.Invalid("A synced connection has no ID.")
	}
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return Profile{}, err
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now().UTC()
	}
	if p.ConnectTimeoutSeconds <= 0 {
		p.ConnectTimeoutSeconds = 10
	}
	p.LastConnectedAt, p.Position = nil, 0
	for i := range list {
		if list[i].ID == p.ID {
			p.Position, p.LastConnectedAt = list[i].Position, list[i].LastConnectedAt
			if p.CreatedAt.IsZero() {
				p.CreatedAt = list[i].CreatedAt
			}
			list[i] = p
			return p, writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
		}
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = p.UpdatedAt
	}
	if maxPos(list) > 0 {
		p.Position = maxPos(list) + 1
	}
	list = append(list, p)
	return p, writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
}

func (s *Store) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return err
	}
	out := list[:0]
	for _, p := range list {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: out})
}

// NewID returns a random 16-hex-character identifier.
func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ReadJSON loads path into v; a missing or empty file leaves v unchanged.
func ReadJSON(path string, v any) error { return readJSON(path, v) }

// WriteJSON atomically writes v to path with 0600 permissions.
func WriteJSON(path string, v any) error { return writeJSON(path, v) }

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s is corrupt: %w", filepath.Base(path), err)
	}
	return nil
}

// writeJSON writes atomically via a temp file + rename with 0600 permissions.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
