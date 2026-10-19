// Package connio reads and writes connection files: the Mango3S format
// (folders and every connection setting, secrets only when the user opts in,
// sealed with a passphrase), MongoDB Compass exports, and plain lists of
// connection strings.
package connio

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"mango3s/internal/config"
)

// Format names.
const (
	FormatMango3S = "mango3s"
	FormatCompass = "compass"
	FormatURIList = "uris"
)

// FileFormat identifies a Mango3S connections file.
const FileFormat = "mango3s-connections"

// MinPassphrase is the shortest passphrase accepted for exported secrets.
const MinPassphrase = 10

// File is a Mango3S connections file.
type File struct {
	Format      string           `json:"format"`
	Version     int              `json:"version"`
	Exported    time.Time        `json:"exported"`
	Connections []config.Profile `json:"connections"`
	// Secrets holds passwords and other secrets, sealed with a passphrase.
	// Absent unless the user explicitly chose to include them.
	Secrets *Sealed `json:"secrets,omitempty"`
}

// Sealed is AES-256-GCM ciphertext under a scrypt-derived key.
type Sealed struct {
	KDF   string `json:"kdf"` // "scrypt"
	N     int    `json:"n"`
	R     int    `json:"r"`
	P     int    `json:"p"`
	Salt  []byte `json:"salt"`
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

// Secrets maps profile ID → secret kind → value.
type Secrets map[string]map[string]string

// ErrPassphrase means the passphrase is wrong or the secrets were altered.
var ErrPassphrase = errors.New("the passphrase is wrong or the file was modified")

func deriveKey(pass string, salt []byte, n, r, p int) ([]byte, error) {
	return scrypt.Key([]byte(pass), salt, n, r, p, 32)
}

// Seal encrypts secrets with a passphrase.
func Seal(s Secrets, passphrase string) (*Sealed, error) {
	if len([]rune(passphrase)) < MinPassphrase {
		return nil, fmt.Errorf("use a passphrase of at least %d characters", MinPassphrase)
	}
	plain, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	out := &Sealed{KDF: "scrypt", N: 1 << 15, R: 8, P: 1, Salt: make([]byte, 16), Nonce: make([]byte, 12)}
	if _, err := rand.Read(out.Salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(out.Nonce); err != nil {
		return nil, err
	}
	key, err := deriveKey(passphrase, out.Salt, out.N, out.R, out.P)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	out.Data = gcm.Seal(nil, out.Nonce, plain, []byte(FileFormat))
	return out, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// Open decrypts sealed secrets.
func (s *Sealed) Open(passphrase string) (Secrets, error) {
	if s.KDF != "scrypt" || s.N < 1<<14 || s.N > 1<<20 || s.R < 1 || s.R > 32 || s.P < 1 || s.P > 16 || len(s.Nonce) != 12 {
		return nil, errors.New("the encrypted secrets use unsupported settings")
	}
	key, err := deriveKey(passphrase, s.Salt, s.N, s.R, s.P)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, s.Nonce, s.Data, []byte(FileFormat))
	if err != nil {
		return nil, ErrPassphrase
	}
	var out Secrets
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Export builds a connections file. Machine-local state (IDs are kept for
// re-import matching; last-used times, order and stored-secret flags are not)
// is dropped. secrets is nil unless the user chose to include them.
func Export(profiles []config.Profile, secrets Secrets, passphrase string, now time.Time) ([]byte, error) {
	f := File{Format: FileFormat, Version: 1, Exported: now.UTC(), Connections: make([]config.Profile, 0, len(profiles))}
	for _, p := range profiles {
		p.LastConnectedAt = nil
		p.Position = 0
		p.HasPassword = false
		p.StoredSecrets = nil
		// Profiles never store passwords in the URI, but older ones might.
		if stripped, _, _, err := config.SplitCredentials(p.URI); err == nil {
			p.URI = stripped
		}
		f.Connections = append(f.Connections, p)
	}
	if secrets != nil {
		sealed, err := Seal(secrets, passphrase)
		if err != nil {
			return nil, err
		}
		f.Secrets = sealed
	}
	return json.MarshalIndent(f, "", "  ")
}

// Candidate is one connection found in a file.
type Candidate struct {
	Profile config.Profile    `json:"profile"`
	Secrets map[string]string `json:"-"` // kind → value (decrypted or from the URI)
	Kinds   []string          `json:"secretKinds"`
	Notes   []string          `json:"notes"`
	Error   string            `json:"error,omitempty"`
}

// Parsed is the content of a connections file.
type Parsed struct {
	Format          string      `json:"format"`
	Candidates      []Candidate `json:"candidates"`
	HasSecrets      bool        `json:"hasSecrets"`      // the file carries encrypted secrets
	SecretsUnlocked bool        `json:"secretsUnlocked"` // and the passphrase opened them
	Warnings        []string    `json:"warnings"`
}

// MaxFileSize bounds connection files.
const MaxFileSize = 8 << 20

// Parse reads a connections file in any supported format. A passphrase is
// needed only to import secrets from a Mango3S file; without it the
// connections import without secrets.
func Parse(data []byte, passphrase string) (Parsed, error) {
	if len(data) > MaxFileSize {
		return Parsed{}, errors.New("the file is too large to be a connections file")
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	trim := bytes.TrimSpace(data)
	var out Parsed
	var err error
	switch {
	case len(trim) > 0 && (trim[0] == '{' || trim[0] == '['):
		out, err = parseJSON(trim, passphrase)
	default:
		out, err = parseURIList(data)
	}
	if err != nil {
		return out, err
	}
	if len(out.Candidates) == 0 {
		return out, errors.New("no connections were found in the file")
	}
	for i := range out.Candidates {
		c := &out.Candidates[i]
		c.Profile = c.Profile.Normalize()
		if c.Profile.Name == "" {
			c.Profile.Name = nameFromURI(c.Profile.URI)
		}
		for k := range c.Secrets {
			c.Kinds = append(c.Kinds, k)
		}
		if c.Notes == nil {
			c.Notes = []string{}
		}
		if c.Error == "" {
			if err := c.Profile.Validate(); err != nil {
				c.Error = err.Error()
			}
		}
		if c.Profile.TLS.CAFile != "" || c.Profile.TLS.CertKeyFile != "" || c.Profile.SSH.PrivateKeyFile != "" {
			c.Notes = append(c.Notes, "Uses certificate or key files by path; they must exist on this computer.")
		}
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	return out, nil
}

func parseJSON(data []byte, passphrase string) (Parsed, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return Parsed{}, fmt.Errorf("the file is not valid JSON: %w", err)
	}
	var format string
	_ = json.Unmarshal(probe["format"], &format)
	var compassType string
	_ = json.Unmarshal(probe["type"], &compassType)
	switch {
	case format == FileFormat:
		return parseMango3S(data, passphrase)
	case strings.Contains(strings.ToLower(compassType), "compass") || hasCompassShape(probe["connections"]):
		return parseCompass(data)
	}
	return Parsed{}, errors.New("unrecognized connections file: expected a Mango3S or MongoDB Compass export, or a list of connection strings")
}

func parseMango3S(data []byte, passphrase string) (Parsed, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return Parsed{}, fmt.Errorf("the connections file is damaged: %w", err)
	}
	if f.Version > 1 {
		return Parsed{}, fmt.Errorf("this file was written by a newer version of Mango3S (format version %d)", f.Version)
	}
	out := Parsed{Format: FormatMango3S, HasSecrets: f.Secrets != nil}
	var sec Secrets
	if f.Secrets != nil {
		if passphrase == "" {
			out.Warnings = append(out.Warnings, "The file contains encrypted passwords. Enter its passphrase to import them, or import the connections without passwords.")
		} else {
			s, err := f.Secrets.Open(passphrase)
			if err != nil {
				return out, err
			}
			sec, out.SecretsUnlocked = s, true
		}
	}
	for _, p := range f.Connections {
		c := Candidate{Profile: p, Secrets: map[string]string{}}
		for k, v := range sec[p.ID] {
			if v != "" && isSecretKind(k) {
				c.Secrets[k] = v
			}
		}
		out.Candidates = append(out.Candidates, c)
	}
	return out, nil
}

func isSecretKind(k string) bool {
	for _, s := range config.SecretKinds {
		if s == k {
			return true
		}
	}
	return false
}

func hasCompassShape(raw json.RawMessage) bool {
	var conns []map[string]json.RawMessage
	if json.Unmarshal(raw, &conns) != nil || len(conns) == 0 {
		return false
	}
	_, ok := conns[0]["connectionOptions"]
	return ok
}

// compassConn is the part of a Compass export Mango3S understands.
type compassConn struct {
	ID       string `json:"id"`
	Favorite *struct {
		Name string `json:"name"`
	} `json:"favorite"`
	ConnectionOptions struct {
		ConnectionString string `json:"connectionString"`
		SSHTunnel        *struct {
			Host         string `json:"host"`
			Port         any    `json:"port"`
			Username     string `json:"username"`
			IdentityFile string `json:"identityKeyFile"`
		} `json:"sshTunnel"`
	} `json:"connectionOptions"`
	ConnectionSecrets string `json:"connectionSecrets"`
}

func parseCompass(data []byte) (Parsed, error) {
	var f struct {
		Connections []compassConn `json:"connections"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return Parsed{}, fmt.Errorf("the Compass export is damaged: %w", err)
	}
	out := Parsed{Format: FormatCompass}
	encrypted := false
	for _, cc := range f.Connections {
		c := Candidate{Secrets: map[string]string{}}
		uri := strings.TrimSpace(cc.ConnectionOptions.ConnectionString)
		if cc.Favorite != nil {
			c.Profile.Name = strings.TrimSpace(cc.Favorite.Name)
		}
		if uri == "" {
			c.Error = "No connection string."
			out.Candidates = append(out.Candidates, c)
			continue
		}
		c.Profile, c.Secrets, c.Error = fromURI(c.Profile.Name, uri)
		if t := cc.ConnectionOptions.SSHTunnel; t != nil && t.Host != "" {
			c.Profile.SSH = config.SSHConfig{Enabled: true, Host: t.Host, Port: toInt(t.Port), Username: t.Username, PrivateKeyFile: t.IdentityFile}
			if t.IdentityFile != "" {
				c.Profile.SSH.AuthMethod = config.SSHAuthKey
			}
			c.Notes = append(c.Notes, "SSH tunnel imported; enter its password or key passphrase after import if it needs one.")
		}
		if cc.ConnectionSecrets != "" {
			encrypted = true
		}
		out.Candidates = append(out.Candidates, c)
	}
	if encrypted {
		out.Warnings = append(out.Warnings, "Passwords in this Compass export are encrypted with Compass's own passphrase and are not imported. Enter them in each connection after importing.")
	}
	return out, nil
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		var n int
		fmt.Sscan(x, &n)
		return n
	case map[string]any: // Extended JSON {"$numberInt": "22"}
		for _, k := range []string{"$numberInt", "$numberLong", "$numberDouble"} {
			if s, ok := x[k].(string); ok {
				var n int
				fmt.Sscan(s, &n)
				return n
			}
		}
	}
	return 0
}

// fromURI builds a profile from a connection string; an embedded password
// is returned as a secret so it goes to the credential store, not the file.
func fromURI(name, uri string) (config.Profile, map[string]string, string) {
	sec := map[string]string{}
	stripped, user, pass, err := config.SplitCredentials(uri)
	if err != nil {
		return config.Profile{Name: name, URI: redact(uri)}, sec, err.Error()
	}
	if err := checkURI(stripped); err != nil {
		return config.Profile{Name: name, URI: stripped}, sec, err.Error()
	}
	if pass != "" {
		sec[config.SecretPassword] = pass
	}
	return config.Profile{Name: name, URI: stripped, Username: user, Server: config.ServerConfig{Mode: config.ServerModeURI}}, sec, ""
}

// redact hides credentials in a string that failed to parse.
func redact(uri string) string {
	if i := strings.Index(uri, "://"); i >= 0 {
		rest := uri[i+3:]
		if at := strings.LastIndex(strings.SplitN(rest, "/", 2)[0], "@"); at >= 0 {
			return uri[:i+3] + "****@" + rest[at+1:]
		}
	}
	return uri
}

// parseURIList reads one connection per line: "uri", "name = uri" or
// "name<TAB>uri". Blank lines and lines starting with # are skipped.
func parseURIList(data []byte) (Parsed, error) {
	out := Parsed{Format: FormatURIList}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			continue
		}
		i := strings.Index(s, "mongodb://")
		if j := strings.Index(s, "mongodb+srv://"); j >= 0 && (i < 0 || j < i) {
			i = j
		}
		if i < 0 {
			out.Candidates = append(out.Candidates, Candidate{Profile: config.Profile{Name: fmt.Sprintf("Line %d", line)}, Error: fmt.Sprintf("Line %d is not a mongodb:// or mongodb+srv:// connection string.", line)})
			continue
		}
		name := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s[:i]), "=:\t,"))
		name = strings.Trim(name, `"'`)
		uri := strings.Trim(strings.TrimSpace(s[i:]), `"'`)
		c := Candidate{}
		c.Profile, c.Secrets, c.Error = fromURI(name, uri)
		out.Candidates = append(out.Candidates, c)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return out, err
	}
	return out, nil
}

// hostsOf lists the hosts of a connection string without any network
// lookup (the driver's parser resolves mongodb+srv:// names via DNS).
func hostsOf(uri string) ([]string, error) {
	rest, ok := strings.CutPrefix(uri, "mongodb://")
	if !ok {
		if rest, ok = strings.CutPrefix(uri, "mongodb+srv://"); !ok {
			return nil, errors.New("the connection string must start with mongodb:// or mongodb+srv://")
		}
	}
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[at+1:]
	}
	var hosts []string
	for _, h := range strings.Split(rest, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return nil, errors.New("the connection string has no host")
	}
	return hosts, nil
}

// checkURI validates a connection string offline.
func checkURI(uri string) error {
	if _, err := hostsOf(uri); err != nil {
		return err
	}
	if strings.HasPrefix(uri, "mongodb+srv://") {
		if _, err := url.Parse(uri); err != nil {
			return fmt.Errorf("not a valid connection string: %w", err)
		}
		return nil
	}
	_, err := config.ParseURI(uri) // no DNS for mongodb://
	return err
}

func nameFromURI(uri string) string {
	hosts, err := hostsOf(uri)
	if err != nil {
		return "Imported connection"
	}
	h := strings.TrimSuffix(hosts[0], ":27017")
	if len(hosts) > 1 {
		h += fmt.Sprintf(" +%d", len(hosts)-1)
	}
	return h
}
