package config

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	"mango3s/internal/apperr"
)

// Server connection modes and topology types.
const (
	ServerModeURI    = "uri"    // the connection string is the source of truth
	ServerModeManual = "manual" // the connection string is built from Server

	TypeStandalone = "standalone"
	TypeReplicaSet = "replicaSet"
	TypeSharded    = "sharded"
	TypeSRV        = "srv" // DNS seed list (mongodb+srv://)
)

// HostPort is one seed host.
type HostPort struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ServerConfig is the Server tab.
type ServerConfig struct {
	Mode             string     `json:"mode"`
	Type             string     `json:"type"`
	Hosts            []HostPort `json:"hosts"`
	ReplicaSet       string     `json:"replicaSet"`
	DirectConnection bool       `json:"directConnection"`
}

// TLSConfig is the SSL / TLS tab. The client key password is a secret.
type TLSConfig struct {
	Enabled                  bool   `json:"enabled"`
	CAFile                   string `json:"caFile"`
	CertKeyFile              string `json:"certKeyFile"` // PEM with the client certificate and its private key
	AllowInvalidCertificates bool   `json:"allowInvalidCertificates"`
	AllowInvalidHostnames    bool   `json:"allowInvalidHostnames"`
}

// SSH authentication methods.
const (
	SSHAuthPassword = "password"
	SSHAuthKey      = "privateKey"
	SSHAuthAgent    = "agent"
)

// SSHConfig is the SSH tab. Passwords and key passphrases are secrets.
type SSHConfig struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	AuthMethod     string `json:"authMethod"`
	PrivateKeyFile string `json:"privateKeyFile"`
	// HostKey pins the server's key fingerprint ("SHA256:…") when it is not
	// in ~/.ssh/known_hosts. Set only after the user has verified it.
	HostKey string `json:"hostKey"`
}

// Proxy types.
const (
	ProxyNone   = "none"
	ProxySOCKS5 = "socks5"
	ProxyHTTP   = "http" // HTTP CONNECT
)

// ProxyConfig is the Proxy tab. The proxy password is a secret.
type ProxyConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

// ShellConfig is the Shell tab.
type ShellConfig struct {
	DefaultDB     string `json:"defaultDb"`
	InitScript    string `json:"initScript"`    // shell commands run when a shell tab opens
	RunInitScript bool   `json:"runInitScript"` // run InitScript automatically
}

// ToolsConfig is the MongoDB Tools tab.
type ToolsConfig struct {
	Dir   string            `json:"dir"`   // folder searched before PATH
	Paths map[string]string `json:"paths"` // per-tool overrides (tool name → executable)
}

// AdvancedConfig is the Advanced tab. Zero values mean "driver default" (or
// whatever the connection string says).
type AdvancedConfig struct {
	ServerSelectionTimeoutSeconds int      `json:"serverSelectionTimeoutSeconds"`
	OperationTimeoutSeconds       int      `json:"operationTimeoutSeconds"` // client-side operation timeout (timeoutMS)
	HeartbeatSeconds              int      `json:"heartbeatSeconds"`
	LocalThresholdMS              int      `json:"localThresholdMs"`
	AppName                       string   `json:"appName"`
	ReadPreference                string   `json:"readPreference"`
	ReadPreferenceTags            string   `json:"readPreferenceTags"` // "dc:ny,rack:1"
	MaxStalenessSeconds           int      `json:"maxStalenessSeconds"`
	ReadConcern                   string   `json:"readConcern"`
	WriteConcernW                 string   `json:"writeConcernW"` // "", "majority", a number or a tag set name
	WriteConcernJournal           string   `json:"writeConcernJournal"`
	MaxPoolSize                   int      `json:"maxPoolSize"`
	MinPoolSize                   int      `json:"minPoolSize"`
	MaxConnecting                 int      `json:"maxConnecting"`
	MaxIdleTimeSeconds            int      `json:"maxIdleTimeSeconds"`
	RetryWrites                   string   `json:"retryWrites"` // "", "true", "false"
	RetryReads                    string   `json:"retryReads"`
	Compressors                   []string `json:"compressors"`
}

// Secret kinds stored in the credential store next to a profile.
const (
	SecretPassword        = "password"
	SecretSSHPassword     = "sshPassword"
	SecretSSHPassphrase   = "sshPassphrase"
	SecretProxyPassword   = "proxyPassword"
	SecretTLSKeyPassword  = "tlsKeyPassword"
	SecretAWSSessionToken = "awsSessionToken"
)

// SecretKinds lists every secret kind a profile can hold.
var SecretKinds = []string{SecretPassword, SecretSSHPassword, SecretSSHPassphrase, SecretProxyPassword, SecretTLSKeyPassword, SecretAWSSessionToken}

// SecretKey is the credential-store ID for a profile's secret. The password
// keeps the bare profile ID used before other kinds existed.
func SecretKey(profileID, kind string) string {
	if kind == SecretPassword {
		return profileID
	}
	return profileID + ":" + kind
}

// HasSecret reports whether a secret of this kind is stored.
func (p Profile) HasSecret(kind string) bool {
	if kind == SecretPassword {
		return p.HasPassword
	}
	for _, k := range p.StoredSecrets {
		if k == kind {
			return true
		}
	}
	return false
}

// SetSecretFlag records that a secret kind is (or is no longer) stored.
func (p *Profile) SetSecretFlag(kind string, stored bool) {
	if kind == SecretPassword {
		p.HasPassword = stored
		return
	}
	out := p.StoredSecrets[:0:0]
	for _, k := range p.StoredSecrets {
		if k != kind {
			out = append(out, k)
		}
	}
	if stored {
		out = append(out, kind)
	}
	p.StoredSecrets = out
}

// Authentication mechanisms.
const (
	MechDefault  = ""
	MechSCRAM256 = "SCRAM-SHA-256"
	MechSCRAM1   = "SCRAM-SHA-1"
	MechX509     = "MONGODB-X509"
	MechPLAIN    = "PLAIN" // LDAP
	MechAWS      = "MONGODB-AWS"
	MechOIDC     = "MONGODB-OIDC"
	MechGSSAPI   = "GSSAPI" // Kerberos
)

// Normalize fills defaults so older profiles and partial forms behave.
func (p Profile) Normalize() Profile {
	p.Name = strings.TrimSpace(p.Name)
	p.Folder = cleanFolder(p.Folder)
	if p.Server.Mode != ServerModeManual {
		p.Server.Mode = ServerModeURI
	}
	if p.Server.Type == "" {
		p.Server.Type = TypeStandalone
	}
	for i := range p.Server.Hosts {
		p.Server.Hosts[i].Host = strings.TrimSpace(p.Server.Hosts[i].Host)
		if p.Server.Hosts[i].Port == 0 && p.Server.Type != TypeSRV {
			p.Server.Hosts[i].Port = 27017
		}
	}
	if p.SSH.Enabled {
		if p.SSH.Port == 0 {
			p.SSH.Port = 22
		}
		if p.SSH.AuthMethod == "" {
			p.SSH.AuthMethod = SSHAuthPassword
		}
	}
	if p.Proxy.Type == "" {
		p.Proxy.Type = ProxyNone
	}
	if p.Proxy.Type != ProxyNone && p.Proxy.Port == 0 {
		if p.Proxy.Type == ProxySOCKS5 {
			p.Proxy.Port = 1080
		} else {
			p.Proxy.Port = 8080
		}
	}
	if p.ConnectTimeoutSeconds <= 0 {
		p.ConnectTimeoutSeconds = 10
	}
	return p
}

// cleanFolder normalizes "a / b/" to "a/b".
func cleanFolder(f string) string {
	parts := strings.Split(f, "/")
	out := parts[:0]
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "/")
}

// Validate checks the profile for mistakes that can be found without a
// network round trip. Messages say which tab and field to fix.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return apperr.Invalid("Server: a connection name is required.")
	}
	if !strings.HasPrefix(p.URI, "mongodb://") && !strings.HasPrefix(p.URI, "mongodb+srv://") {
		return apperr.Invalid("Server: the connection string must start with mongodb:// or mongodb+srv://")
	}
	if p.Server.Mode == ServerModeManual {
		if err := p.Server.validate(); err != nil {
			return err
		}
	}
	switch p.AuthMechanism {
	case MechDefault, MechSCRAM256, MechSCRAM1, MechPLAIN, MechAWS, MechOIDC:
	case MechX509:
		if p.TLS.CertKeyFile == "" && !strings.Contains(strings.ToLower(p.URI), "tlscertificatekeyfile") {
			return apperr.Invalid("Authentication: X.509 needs a client certificate. Set it on the SSL / TLS tab.")
		}
	case MechGSSAPI:
		return apperr.New(apperr.CodeUnsupported, "Authentication: Kerberos (GSSAPI) is not available in this build of Mango3S (the Go driver needs the gssapi build tag and system Kerberos libraries).")
	default:
		return apperr.Invalid("Authentication: unknown mechanism %q.", p.AuthMechanism)
	}
	if p.AuthMechanism != MechDefault && p.AuthMechanism != MechAWS && p.AuthMechanism != MechX509 && p.AuthMechanism != MechOIDC && p.Username == "" {
		return apperr.Invalid("Authentication: %s needs a username.", p.AuthMechanism)
	}
	if p.SSH.Enabled {
		if p.SSH.Host == "" || p.SSH.Username == "" {
			return apperr.Invalid("SSH: the SSH host and username are required.")
		}
		if err := checkPort("SSH", p.SSH.Port); err != nil {
			return err
		}
		switch p.SSH.AuthMethod {
		case SSHAuthPassword, SSHAuthAgent:
		case SSHAuthKey:
			if p.SSH.PrivateKeyFile == "" {
				return apperr.Invalid("SSH: choose a private key file.")
			}
		default:
			return apperr.Invalid("SSH: unsupported authentication method %q.", p.SSH.AuthMethod)
		}
	}
	switch p.Proxy.Type {
	case ProxyNone, "":
	case ProxySOCKS5, ProxyHTTP:
		if p.Proxy.Host == "" {
			return apperr.Invalid("Proxy: the proxy host is required.")
		}
		if err := checkPort("Proxy", p.Proxy.Port); err != nil {
			return err
		}
	default:
		return apperr.Invalid("Proxy: unsupported proxy type %q.", p.Proxy.Type)
	}
	if p.ConnectTimeoutSeconds > 300 {
		return apperr.Invalid("Advanced: the connect timeout must be at most 300 seconds.")
	}
	return p.Advanced.validate()
}

func checkPort(tab string, port int) error {
	if port < 1 || port > 65535 {
		return apperr.Invalid("%s: port %d is out of range (1–65535).", tab, port)
	}
	return nil
}

func (s ServerConfig) validate() error {
	if len(s.Hosts) == 0 {
		return apperr.Invalid("Server: add at least one host.")
	}
	for _, h := range s.Hosts {
		if h.Host == "" || strings.ContainsAny(h.Host, "/?@, ") {
			return apperr.Invalid("Server: %q is not a valid host name.", h.Host)
		}
		if s.Type != TypeSRV {
			if err := checkPort("Server", h.Port); err != nil {
				return err
			}
		}
	}
	switch s.Type {
	case TypeStandalone:
		if len(s.Hosts) != 1 {
			return apperr.Invalid("Server: a standalone connection has exactly one host. Use Replica Set or Sharded Cluster for several.")
		}
	case TypeReplicaSet:
		if s.ReplicaSet == "" && !s.DirectConnection {
			return apperr.Invalid("Server: enter the replica set name (or enable direct connection to talk to one member).")
		}
	case TypeSharded:
	case TypeSRV:
		if len(s.Hosts) != 1 || s.Hosts[0].Port != 0 {
			return apperr.Invalid("Server: a DNS seed list (SRV) connection has one host name and no port.")
		}
		if s.DirectConnection {
			return apperr.Invalid("Server: direct connection can't be combined with a DNS seed list.")
		}
	default:
		return apperr.Invalid("Server: unknown connection type %q.", s.Type)
	}
	return nil
}

var readPrefs = map[string]bool{"": true, "primary": true, "primaryPreferred": true, "secondary": true, "secondaryPreferred": true, "nearest": true}

func (a AdvancedConfig) validate() error {
	if !readPrefs[a.ReadPreference] {
		return apperr.Invalid("Advanced: unknown read preference %q.", a.ReadPreference)
	}
	if (a.ReadPreference == "primary" || a.ReadPreference == "") && (a.ReadPreferenceTags != "" || a.MaxStalenessSeconds != 0) {
		return apperr.Invalid("Advanced: tag sets and max staleness need a read preference other than primary.")
	}
	if a.MaxStalenessSeconds != 0 && a.MaxStalenessSeconds < 90 {
		return apperr.Invalid("Advanced: max staleness must be at least 90 seconds.")
	}
	if _, err := ParseTags(a.ReadPreferenceTags); err != nil {
		return err
	}
	switch a.ReadConcern {
	case "", "local", "available", "majority", "linearizable", "snapshot":
	default:
		return apperr.Invalid("Advanced: unknown read concern %q.", a.ReadConcern)
	}
	if a.WriteConcernW != "" && a.WriteConcernW != "majority" {
		if n, err := strconv.Atoi(a.WriteConcernW); err == nil && n < 0 {
			return apperr.Invalid("Advanced: write concern w must be 0 or more.")
		}
	}
	for _, v := range []struct{ name, val string }{{"journal", a.WriteConcernJournal}, {"retry writes", a.RetryWrites}, {"retry reads", a.RetryReads}} {
		if v.val != "" && v.val != "true" && v.val != "false" {
			return apperr.Invalid("Advanced: %s must be default, true or false.", v.name)
		}
	}
	if a.WriteConcernW == "0" && a.WriteConcernJournal == "true" {
		return apperr.Invalid("Advanced: an unacknowledged write concern (w: 0) can't request journaling.")
	}
	for _, n := range []struct {
		name     string
		val, max int
	}{
		{"server selection timeout", a.ServerSelectionTimeoutSeconds, 600},
		{"operation timeout", a.OperationTimeoutSeconds, 3600},
		{"heartbeat interval", a.HeartbeatSeconds, 3600},
		{"local threshold", a.LocalThresholdMS, 60000},
		{"max pool size", a.MaxPoolSize, 10000},
		{"min pool size", a.MinPoolSize, 10000},
		{"max connecting", a.MaxConnecting, 1000},
		{"max idle time", a.MaxIdleTimeSeconds, 86400},
	} {
		if n.val < 0 || n.val > n.max {
			return apperr.Invalid("Advanced: %s must be between 0 and %d.", n.name, n.max)
		}
	}
	if a.HeartbeatSeconds != 0 && a.HeartbeatSeconds < 1 {
		return apperr.Invalid("Advanced: the heartbeat interval must be at least 1 second.")
	}
	if a.MaxPoolSize != 0 && a.MinPoolSize > a.MaxPoolSize {
		return apperr.Invalid("Advanced: min pool size (%d) is larger than max pool size (%d).", a.MinPoolSize, a.MaxPoolSize)
	}
	for _, c := range a.Compressors {
		if c != "snappy" && c != "zlib" && c != "zstd" {
			return apperr.Invalid("Advanced: unsupported compressor %q (use snappy, zlib or zstd).", c)
		}
	}
	return nil
}

// ParseTags parses "dc:ny,rack:1" into a tag set.
func ParseTags(s string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), ":")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, apperr.Invalid("Advanced: read preference tags must look like dc:ny,rack:1.")
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// BuildURI turns the manual Server fields into a connection string without
// credentials (they never live in the URI).
func BuildURI(s ServerConfig) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	scheme := "mongodb://"
	hosts := make([]string, len(s.Hosts))
	for i, h := range s.Hosts {
		if s.Type == TypeSRV {
			hosts[i] = h.Host
		} else {
			hosts[i] = net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
		}
	}
	if s.Type == TypeSRV {
		scheme = "mongodb+srv://"
	}
	q := url.Values{}
	if s.Type == TypeReplicaSet && s.ReplicaSet != "" {
		q.Set("replicaSet", s.ReplicaSet)
	}
	if s.DirectConnection {
		q.Set("directConnection", "true")
	}
	uri := scheme + strings.Join(hosts, ",") + "/"
	if len(q) > 0 {
		uri += "?" + q.Encode()
	}
	return uri, nil
}

// ParsedURI is what the Server tab can take over from a connection string.
type ParsedURI struct {
	Server        ServerConfig `json:"server"`
	Username      string       `json:"username"`
	HasPassword   bool         `json:"hasPassword"`
	AuthSource    string       `json:"authSource"`
	AuthMechanism string       `json:"authMechanism"`
	TLS           bool         `json:"tls"`
	AppName       string       `json:"appName"`
	ReadPref      string       `json:"readPreference"`
	Database      string       `json:"database"`
	// Options the form doesn't model; they stay in effect only in URI mode.
	Unmapped []string `json:"unmapped"`
}

var mappedOptions = map[string]bool{
	"replicaset": true, "directconnection": true, "authsource": true, "authmechanism": true,
	"tls": true, "ssl": true, "appname": true, "readpreference": true,
}

// ParseURI reads a connection string into Server fields.
func ParseURI(uri string) (ParsedURI, error) {
	cs, err := connstring.Parse(strings.TrimSpace(uri))
	if err != nil {
		return ParsedURI{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Server: this is not a valid MongoDB connection string.")
	}
	out := ParsedURI{
		Username: cs.Username, HasPassword: cs.PasswordSet, AuthSource: cs.AuthSource,
		AuthMechanism: strings.ToUpper(cs.AuthMechanism), TLS: cs.SSL, AppName: cs.AppName,
		ReadPref: cs.ReadPreference, Database: cs.Database,
	}
	srv := strings.HasPrefix(strings.TrimSpace(uri), "mongodb+srv://")
	s := ServerConfig{Mode: ServerModeManual, ReplicaSet: cs.ReplicaSet, DirectConnection: cs.DirectConnection}
	hosts := cs.Hosts
	if srv {
		hosts = []string{cs.Original[len("mongodb+srv://"):]}
		h := hosts[0]
		if at := strings.LastIndex(h, "@"); at >= 0 {
			h = h[at+1:]
		}
		if i := strings.IndexAny(h, "/?"); i >= 0 {
			h = h[:i]
		}
		s.Type = TypeSRV
		s.Hosts = []HostPort{{Host: h}}
	} else {
		for _, h := range hosts {
			host, port, err := net.SplitHostPort(h)
			if err != nil {
				host, port = h, "27017"
			}
			n, _ := strconv.Atoi(port)
			s.Hosts = append(s.Hosts, HostPort{Host: host, Port: n})
		}
		switch {
		case cs.ReplicaSet != "":
			s.Type = TypeReplicaSet
		case len(s.Hosts) > 1:
			s.Type = TypeSharded
		default:
			s.Type = TypeStandalone
		}
	}
	out.Server = s
	if _, rawQuery, ok := strings.Cut(strings.TrimSpace(uri), "?"); ok {
		if q, err := url.ParseQuery(rawQuery); err == nil {
			for k := range q {
				if !mappedOptions[strings.ToLower(k)] {
					out.Unmapped = append(out.Unmapped, k)
				}
			}
			sort.Strings(out.Unmapped)
		}
	}
	return out, nil
}

// DisplayHosts is a short "host:port, …" summary for lists and logs.
func (p Profile) DisplayHosts() string {
	if p.Server.Mode == ServerModeManual && len(p.Server.Hosts) > 0 {
		hs := make([]string, len(p.Server.Hosts))
		for i, h := range p.Server.Hosts {
			if h.Port == 0 {
				hs[i] = h.Host
			} else {
				hs[i] = fmt.Sprintf("%s:%d", h.Host, h.Port)
			}
		}
		return strings.Join(hs, ", ")
	}
	return p.URI
}

// ShardURI turns a mongos connection string into one for a single shard, from
// the shard's host string as listShards reports it ("set/a:27017,b:27017", or
// "a:27017" for a standalone shard). Authentication, TLS and other options are
// kept; a mongodb+srv:// string becomes mongodb:// with the TLS and auth
// source it implied.
func ShardURI(base, shardHost string) (string, error) {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(base), "://")
	if !ok || (scheme != "mongodb" && scheme != "mongodb+srv") {
		return "", apperr.Invalid("The connection string must start with mongodb:// or mongodb+srv://")
	}
	set, hosts, hasSet := strings.Cut(shardHost, "/")
	if !hasSet {
		hosts, set = set, ""
	}
	if strings.TrimSpace(hosts) == "" {
		return "", apperr.Invalid("The shard has no hosts.")
	}
	path := "/"
	query := ""
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		tail := rest[i:]
		if q := strings.IndexByte(tail, '?'); q >= 0 {
			path, query = tail[:q], tail[q+1:]
		} else {
			path = tail
		}
	}
	if path == "" {
		path = "/"
	}
	vals, err := url.ParseQuery(query)
	if err != nil {
		return "", apperr.Invalid("The connection string's options can't be read: %v", err)
	}
	for _, k := range []string{"replicaSet", "directConnection", "srvServiceName", "srvMaxHosts", "loadBalanced"} {
		vals.Del(k)
	}
	if scheme == "mongodb+srv" {
		if vals.Get("tls") == "" && vals.Get("ssl") == "" {
			vals.Set("tls", "true")
		}
		if vals.Get("authSource") == "" {
			vals.Set("authSource", "admin")
		}
	}
	if set != "" {
		vals.Set("replicaSet", set)
	}
	out := "mongodb://" + hosts + path
	if enc := vals.Encode(); enc != "" {
		out += "?" + enc
	}
	return out, nil
}

// ConnString is a MongoDB connection string split into its parts. Unlike
// net/url it accepts several hosts ("a:27017,b:27017").
type ConnString struct {
	Scheme   string // mongodb | mongodb+srv
	User     string // percent-encoded user name, "" for none
	Password string // percent-encoded password
	HasPass  bool
	Hosts    string // as written, e.g. "a:27017,b:27017"
	Path     string // "/" or "/db"
	Query    url.Values
}

// ParseConnString splits a connection string.
func ParseConnString(uri string) (ConnString, error) {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(uri), "://")
	if !ok || (scheme != "mongodb" && scheme != "mongodb+srv") {
		return ConnString{}, apperr.Invalid("The connection string must start with mongodb:// or mongodb+srv://")
	}
	authority, tail := rest, ""
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		authority, tail = rest[:i], rest[i:]
	}
	path, query, _ := strings.Cut(tail, "?")
	cs := ConnString{Scheme: scheme, Path: path}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		cs.Hosts = authority[at+1:]
		cs.User, cs.Password, cs.HasPass = strings.Cut(authority[:at], ":")
	} else {
		cs.Hosts = authority
	}
	if cs.Hosts == "" {
		return ConnString{}, apperr.Invalid("The connection string has no host.")
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return ConnString{}, apperr.Invalid("The connection string's options can't be read: %v", err)
	}
	cs.Query = q
	return cs, nil
}

// String puts the connection string back together.
func (c ConnString) String() string {
	out := c.Scheme + "://"
	if c.User != "" {
		out += c.User
		if c.HasPass {
			out += ":" + c.Password
		}
		out += "@"
	}
	path := c.Path
	if path == "" {
		path = "/"
	}
	out += c.Hosts + path
	if enc := c.Query.Encode(); enc != "" {
		out += "?" + enc
	}
	return out
}

// EscapeUserinfo percent-encodes a user name or password for a connection
// string (every reserved character, including "@", ":" and "/").
func EscapeUserinfo(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
