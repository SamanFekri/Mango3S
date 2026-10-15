// Package mongox manages live MongoDB client connections. It knows nothing
// about where profiles or passwords are stored; callers pass them in.
package mongox

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/logging"
)

// Session is an open connection for one profile.
type Session struct {
	ProfileID     string
	Name          string
	ReadOnly      bool
	Client        *mongo.Client
	ServerVersion string
	Topology      string
	ConnectedAt   time.Time
}

// RequireWritable returns a READ_ONLY error for read-only sessions.
func (s *Session) RequireWritable() error {
	if s.ReadOnly {
		return apperr.New(apperr.CodeReadOnly, "Connection %q is read-only. Edit the connection to allow writes.", s.Name)
	}
	return nil
}

// Source resolves a connection ID to an open session. Services depend on this
// interface rather than on Manager.
type Source interface {
	Session(connID string) (*Session, error)
}

// SessionInfo describes an open connection for the UI.
type SessionInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ReadOnly      bool      `json:"readOnly"`
	ServerVersion string    `json:"serverVersion"`
	Topology      string    `json:"topology"`
	ConnectedAt   time.Time `json:"connectedAt"`
}

// TestResult is the outcome of a successful connection test.
type TestResult struct {
	ServerVersion string `json:"serverVersion"`
	Topology      string `json:"topology"`
	LatencyMs     int64  `json:"latencyMs"`
}

// Manager owns all open clients, keyed by profile ID.
type Manager struct {
	log      *slog.Logger
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager returns an empty manager.
func NewManager(log *slog.Logger) *Manager {
	return &Manager{log: log, sessions: map[string]*Session{}}
}

// BuildClientOptions turns a profile and its password into driver options.
func BuildClientOptions(p config.Profile, password string) (*options.ClientOptions, error) {
	timeout := time.Duration(p.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	o := options.Client().ApplyURI(p.URI)
	if err := o.Validate(); err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Invalid connection string.")
	}
	if o.AppName == nil {
		o.SetAppName("Mango3S")
	}
	o.SetConnectTimeout(timeout).SetServerSelectionTimeout(timeout)
	if p.Username != "" {
		cred := options.Credential{}
		if o.Auth != nil {
			cred = *o.Auth
		}
		cred.Username = p.Username
		cred.Password = password
		cred.PasswordSet = password != ""
		if p.AuthSource != "" {
			cred.AuthSource = p.AuthSource
		} else if cred.AuthSource == "" {
			// The driver ignores authSource when the URI itself has no user.
			if cs, err := connstring.Parse(p.URI); err == nil {
				cred.AuthSource = cs.AuthSource
			}
		}
		if p.AuthMechanism != "" {
			cred.AuthMechanism = p.AuthMechanism
		}
		o.SetAuth(cred)
	}
	return o, nil
}

func open(ctx context.Context, p config.Profile, password string) (*mongo.Client, TestResult, error) {
	opts, err := BuildClientOptions(p, password)
	if err != nil {
		return nil, TestResult{}, err
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, TestResult{}, apperr.From(err)
	}
	start := time.Now()
	if err := client.Ping(ctx, readpref.Nearest()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, TestResult{}, apperr.From(err)
	}
	res := TestResult{LatencyMs: time.Since(start).Milliseconds()}
	res.ServerVersion, res.Topology = describe(ctx, client)
	return client, res, nil
}

func describe(ctx context.Context, c *mongo.Client) (version, topology string) {
	admin := c.Database("admin")
	var bi struct {
		Version string `bson:"version"`
	}
	if err := admin.RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&bi); err == nil {
		version = bi.Version
	}
	var hello struct {
		SetName string `bson:"setName"`
		Msg     string `bson:"msg"`
	}
	topology = "standalone"
	if err := admin.RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello); err == nil {
		switch {
		case hello.Msg == "isdbgrid":
			topology = "sharded"
		case hello.SetName != "":
			topology = "replica set " + hello.SetName
		}
	}
	return version, topology
}

// Test opens a temporary client, pings the server and closes it.
func (m *Manager) Test(ctx context.Context, p config.Profile, password string) (TestResult, error) {
	client, res, err := open(ctx, p, password)
	if err != nil {
		m.log.Info("connection test failed", "profile", p.Name, "err", err)
		return TestResult{}, err
	}
	_ = client.Disconnect(context.Background())
	m.log.Info("connection test succeeded", "profile", p.Name, "version", res.ServerVersion)
	return res, nil
}

// Connect opens (or returns the existing) session for a profile.
func (m *Manager) Connect(ctx context.Context, p config.Profile, password string) (*Session, error) {
	if s, err := m.Session(p.ID); err == nil {
		return s, nil
	}
	client, res, err := open(ctx, p, password)
	if err != nil {
		m.log.Warn("connect failed", "profile", p.Name, "uri", logging.Redact(p.URI), "err", err)
		return nil, err
	}
	s := &Session{
		ProfileID:     p.ID,
		Name:          p.Name,
		ReadOnly:      p.ReadOnly,
		Client:        client,
		ServerVersion: res.ServerVersion,
		Topology:      res.Topology,
		ConnectedAt:   time.Now(),
	}
	m.mu.Lock()
	if existing, ok := m.sessions[p.ID]; ok {
		m.mu.Unlock()
		_ = client.Disconnect(context.Background())
		return existing, nil
	}
	m.sessions[p.ID] = s
	m.mu.Unlock()
	m.log.Info("connected", "profile", p.Name, "version", res.ServerVersion, "topology", res.Topology)
	return s, nil
}

// Disconnect closes a session. Closing an unknown session is a no-op.
func (m *Manager) Disconnect(ctx context.Context, id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if !ok {
		return nil
	}
	m.log.Info("disconnected", "profile", s.Name)
	return s.Client.Disconnect(ctx)
}

// Session implements Source.
func (m *Manager) Session(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, apperr.New(apperr.CodeNotConnected, "Not connected. Open the connection first.")
	}
	return s, nil
}

// List describes all open sessions, ordered by name.
func (m *Manager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SessionInfo, 0, len(m.sessions))
	for id, s := range m.sessions {
		out = append(out, SessionInfo{
			ID: id, Name: s.Name, ReadOnly: s.ReadOnly,
			ServerVersion: s.ServerVersion, Topology: s.Topology, ConnectedAt: s.ConnectedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CloseAll disconnects every session (application shutdown).
func (m *Manager) CloseAll(ctx context.Context) {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = map[string]*Session{}
	m.mu.Unlock()
	for _, s := range sessions {
		if err := s.Client.Disconnect(ctx); err != nil {
			m.log.Warn("disconnect on shutdown failed", "profile", s.Name, "err", err)
		}
	}
}

// Collection returns a collection handle for a connection.
func Collection(src Source, connID, db, coll string) (*Session, *mongo.Collection, error) {
	if db == "" || coll == "" {
		return nil, nil, apperr.Invalid("Database and collection names are required.")
	}
	s, err := src.Session(connID)
	if err != nil {
		return nil, nil, err
	}
	return s, s.Client.Database(db).Collection(coll), nil
}
