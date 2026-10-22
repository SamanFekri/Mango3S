// Package streams watches change streams for the live change viewer: a
// collection, a database or the whole deployment, with an optional $match and
// the full document. Events reach the UI in batches; pausing remembers the
// resume token so resuming continues exactly where it stopped.
package streams

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Request selects what to watch. Empty DB watches the deployment; empty
// Coll watches the database.
type Request struct {
	ConnID       string `json:"connId"`
	DB           string `json:"db"`
	Coll         string `json:"coll"`
	Match        string `json:"match"`        // optional $match on change events, mongosh syntax
	FullDocument bool   `json:"fullDocument"` // include the document after updates
	ResumeToken  string `json:"resumeToken"`  // canonical Extended JSON; continue after it
}

// Event is one change.
type Event struct {
	Op       string    `json:"op"` // insert | update | replace | delete | drop | rename | invalidate…
	NS       string    `json:"ns"`
	Key      string    `json:"key"` // documentKey, mongosh syntax
	Time     time.Time `json:"time"`
	Updated  []string  `json:"updated,omitempty"` // changed field paths
	Removed  []string  `json:"removed,omitempty"`
	Document string    `json:"document,omitempty"` // fullDocument, mongosh syntax
	Raw      string    `json:"raw"`                // the whole event, mongosh syntax
}

// State reports a stream's status.
type State struct {
	ID          string `json:"id"`
	Running     bool   `json:"running"`
	Error       string `json:"error,omitempty"`
	ResumeToken string `json:"resumeToken,omitempty"`
}

// Sink receives batches and state changes.
type Sink struct {
	Events func(id string, events []Event, resumeToken string)
	State  func(State)
}

// Service runs streams by ID (one per viewer tab).
type Service struct {
	src  mongox.Source
	mu   sync.Mutex
	runs map[string]*entry
	gen  int
	sink Sink
}

type entry struct {
	cancel context.CancelFunc
	gen    int
}

// New returns a stream service.
func New(src mongox.Source) *Service { return &Service{src: src, runs: map[string]*entry{}} }

// SetSink installs where batches go (the desktop app's events).
func (s *Service) SetSink(k Sink) {
	s.mu.Lock()
	s.sink = k
	s.mu.Unlock()
}

// Check explains why the connection can't watch changes, or returns nil.
func Check(sess *mongox.Session) error {
	if sess.Topology == "standalone" || sess.Topology == "single" {
		return apperr.New(apperr.CodeUnsupported, "Change streams need a replica set or a sharded cluster: they read the oplog, which a standalone server doesn't keep. A single-node replica set is enough for development (start mongod with --replSet and run rs.initiate()).")
	}
	return nil
}

// Start opens the stream and sends events until Stop.
func (s *Service) Start(id string, r Request) error {
	sess, err := s.src.Session(r.ConnID)
	if err != nil {
		return err
	}
	if err := Check(sess); err != nil {
		return err
	}
	pipe := mongo.Pipeline{}
	if strings.TrimSpace(r.Match) != "" {
		m, err := mql.ParseDocument(r.Match)
		if err != nil {
			return apperr.Invalid("The filter can't be read: %v", err)
		}
		if len(m) > 0 {
			pipe = append(pipe, bson.D{{Key: "$match", Value: m}})
		}
	}
	opts := options.ChangeStream().SetMaxAwaitTime(time.Second)
	if r.FullDocument {
		opts.SetFullDocument(options.UpdateLookup)
	}
	if r.ResumeToken != "" {
		var tok bson.D
		if err := bson.UnmarshalExtJSON([]byte(r.ResumeToken), true, &tok); err != nil {
			return apperr.Invalid("The resume token can't be read.")
		}
		opts.SetResumeAfter(tok)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var cs *mongo.ChangeStream
	switch {
	case r.DB == "":
		cs, err = sess.Client.Watch(ctx, pipe, opts)
	case r.Coll == "":
		cs, err = sess.Client.Database(r.DB).Watch(ctx, pipe, opts)
	default:
		cs, err = sess.Client.Database(r.DB).Collection(r.Coll).Watch(ctx, pipe, opts)
	}
	if err != nil {
		cancel()
		return apperr.From(err)
	}
	s.mu.Lock()
	if old, ok := s.runs[id]; ok {
		old.cancel()
	}
	s.gen++
	gen := s.gen
	s.runs[id] = &entry{cancel: cancel, gen: gen}
	sink := s.sink
	s.mu.Unlock()
	if sink.State != nil {
		sink.State(State{ID: id, Running: true, ResumeToken: r.ResumeToken})
	}
	go s.pump(ctx, id, gen, cs, r.ResumeToken)
	return nil
}

// pump reads events and sends them in batches (every 250 ms or 200 events).
func (s *Service) pump(ctx context.Context, id string, gen int, cs *mongo.ChangeStream, token string) {
	defer cs.Close(context.Background())
	var batch []Event
	last := time.Now()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.mu.Lock()
		sink := s.sink
		cur, ok := s.runs[id]
		stale := ok && cur.gen != gen
		s.mu.Unlock()
		if sink.Events != nil && !stale {
			sink.Events(id, batch, token)
		}
		batch = nil
		last = time.Now()
	}
	for {
		if cs.TryNext(ctx) {
			var raw bson.D
			if err := cs.Decode(&raw); err == nil {
				batch = append(batch, EventFrom(raw))
			}
			token = resumeToken(cs)
			if len(batch) >= 200 {
				flush()
			}
			continue
		}
		if t := resumeToken(cs); t != "" {
			token = t
		}
		if time.Since(last) > 250*time.Millisecond {
			flush()
		}
		if err := cs.Err(); err != nil || ctx.Err() != nil {
			flush()
			state := State{ID: id, Running: false, ResumeToken: token}
			if err != nil && ctx.Err() == nil {
				state.Error = apperr.From(err).Message
			}
			s.mu.Lock()
			cur, ok := s.runs[id]
			stale := ok && cur.gen != gen // restarted: the new run reports from now on
			if ok && !stale {
				delete(s.runs, id)
			}
			sink := s.sink
			s.mu.Unlock()
			if sink.State != nil && !stale {
				sink.State(state)
			}
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func resumeToken(cs *mongo.ChangeStream) string {
	t := cs.ResumeToken()
	if len(t) == 0 {
		return ""
	}
	j, err := bson.MarshalExtJSON(t, true, false)
	if err != nil {
		return ""
	}
	return string(j)
}

// Stop ends a stream; its last resume token was already reported.
func (s *Service) Stop(id string) {
	s.mu.Lock()
	e, ok := s.runs[id]
	delete(s.runs, id)
	s.mu.Unlock()
	if ok {
		e.cancel()
	}
}

// StopAll ends every stream.
func (s *Service) StopAll() {
	s.mu.Lock()
	runs := s.runs
	s.runs = map[string]*entry{}
	s.mu.Unlock()
	for _, e := range runs {
		e.cancel()
	}
}

// EventFrom summarizes a raw change event.
func EventFrom(raw bson.D) Event {
	e := Event{Raw: mql.Pretty(raw)}
	if v, ok := mql.Lookup(raw, "operationType"); ok {
		e.Op, _ = v.(string)
	}
	if v, ok := mql.Lookup(raw, "ns"); ok {
		if ns, ok := v.(bson.D); ok {
			db, _ := mql.Lookup(ns, "db")
			coll, _ := mql.Lookup(ns, "coll")
			e.NS, _ = db.(string)
			if c, ok := coll.(string); ok && c != "" {
				e.NS += "." + c
			}
		}
	}
	if v, ok := mql.Lookup(raw, "documentKey"); ok {
		if d, ok := v.(bson.D); ok {
			if id, ok := mql.Lookup(d, "_id"); ok && len(d) == 1 {
				e.Key = mql.Compact(id)
			} else {
				e.Key = mql.Compact(d)
			}
		}
	}
	if v, ok := mql.Lookup(raw, "clusterTime"); ok {
		if ts, ok := v.(bson.Timestamp); ok {
			e.Time = time.Unix(int64(ts.T), 0).UTC()
		}
	}
	if e.Time.IsZero() {
		if v, ok := mql.Lookup(raw, "wallTime"); ok {
			if dt, ok := v.(bson.DateTime); ok {
				e.Time = dt.Time().UTC()
			}
		}
	}
	if v, ok := mql.Lookup(raw, "updateDescription"); ok {
		if ud, ok := v.(bson.D); ok {
			if u, ok := mql.Lookup(ud, "updatedFields"); ok {
				if ud, ok := u.(bson.D); ok {
					for _, f := range ud {
						e.Updated = append(e.Updated, f.Key)
					}
				}
			}
			if r, ok := mql.Lookup(ud, "removedFields"); ok {
				if arr, ok := r.(bson.A); ok {
					for _, f := range arr {
						if s, ok := f.(string); ok {
							e.Removed = append(e.Removed, s)
						}
					}
				}
			}
		}
	}
	if v, ok := mql.Lookup(raw, "fullDocument"); ok && v != nil {
		if d, ok := v.(bson.D); ok {
			e.Document = mql.Pretty(d)
		}
	}
	return e
}
