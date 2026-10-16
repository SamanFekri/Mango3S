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
	AIEnabled        bool   `json:"aiEnabled"`
	AIModel          string `json:"aiModel"`
	AIAllowSamples   bool   `json:"aiAllowSamples"`   // allow sending masked sample documents
	SchedulerEnabled bool   `json:"schedulerEnabled"` // run approved scheduled tasks while the app is open
}

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
		SchedulerEnabled:    true,
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
	switch s.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		s.LogLevel = d.LogLevel
	}
	return s
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
	Folder          string         `json:"folder,omitempty"`
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
	sort.SliceStable(f.Profiles, func(i, j int) bool {
		return strings.ToLower(f.Profiles[i].Name) < strings.ToLower(f.Profiles[j].Name)
	})
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
		list = append(list, p)
	}
	return p, writeJSON(s.path("connections.json"), profilesFile{Version: 1, Profiles: list})
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
