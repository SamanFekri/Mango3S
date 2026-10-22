//go:build integration

package integration

import (
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/streams"
)

type streamSink struct {
	mu     sync.Mutex
	events []streams.Event
	token  string
	states []streams.State
}

func (k *streamSink) sink() streams.Sink {
	return streams.Sink{
		Events: func(_ string, ev []streams.Event, tok string) {
			k.mu.Lock()
			k.events = append(k.events, ev...)
			k.token = tok
			k.mu.Unlock()
		},
		State: func(s streams.State) {
			k.mu.Lock()
			k.states = append(k.states, s)
			if s.ResumeToken != "" {
				k.token = s.ResumeToken
			}
			k.mu.Unlock()
		},
	}
}

func (k *streamSink) waitFor(t *testing.T, n int) []streams.Event {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		k.mu.Lock()
		got := append([]streams.Event{}, k.events...)
		k.mu.Unlock()
		if len(got) >= n {
			return got
		}
		time.Sleep(100 * time.Millisecond)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	t.Fatalf("got %d of %d events: %+v", len(k.events), n, k.events)
	return nil
}

func TestChangeStreamsAndResume(t *testing.T) {
	e := setupOptional(t, "MONGO_RS_URI")
	s, _ := e.mgr.Session(e.conn)
	coll := s.Client.Database(e.db).Collection("live")
	if err := s.Client.Database(e.db).CreateCollection(e.ctx, "live"); err != nil {
		t.Fatal(err)
	}
	svc := streams.New(e.mgr)
	sink := &streamSink{}
	svc.SetSink(sink.sink())
	req := streams.Request{ConnID: e.conn, DB: e.db, Coll: "live", FullDocument: true}
	if err := svc.Start("tab1", req); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // the stream is open
	if _, err := coll.InsertOne(e.ctx, bson.D{{Key: "_id", Value: 1}, {Key: "status", Value: "new"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := coll.UpdateOne(e.ctx, bson.D{{Key: "_id", Value: 1}}, bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: "shipped"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := coll.DeleteOne(e.ctx, bson.D{{Key: "_id", Value: 1}}); err != nil {
		t.Fatal(err)
	}
	got := sink.waitFor(t, 3)
	if got[0].Op != "insert" || got[1].Op != "update" || got[2].Op != "delete" || got[1].Updated[0] != "status" || got[1].Document == "" || got[0].Key != "1" {
		t.Fatalf("events: %+v", got)
	}

	// Pause, change something, resume from the token: nothing is missed.
	svc.Stop("tab1")
	time.Sleep(300 * time.Millisecond)
	sink.mu.Lock()
	token := sink.token
	sink.mu.Unlock()
	if token == "" {
		t.Fatal("no resume token after pausing")
	}
	if _, err := coll.InsertOne(e.ctx, bson.D{{Key: "_id", Value: 2}}); err != nil {
		t.Fatal(err)
	}
	req.ResumeToken = token
	if err := svc.Start("tab1", req); err != nil {
		t.Fatal(err)
	}
	defer svc.Stop("tab1")
	got = sink.waitFor(t, 4)
	if got[3].Op != "insert" || got[3].Key != "2" {
		t.Fatalf("after resume: %+v", got[3])
	}

	// A $match filter.
	sink2 := &streamSink{}
	svc2 := streams.New(e.mgr)
	svc2.SetSink(sink2.sink())
	if err := svc2.Start("tab2", streams.Request{ConnID: e.conn, DB: e.db, Coll: "live", Match: `{operationType: "delete"}`}); err != nil {
		t.Fatal(err)
	}
	defer svc2.Stop("tab2")
	time.Sleep(300 * time.Millisecond)
	_, _ = coll.InsertOne(e.ctx, bson.D{{Key: "_id", Value: 3}})
	_, _ = coll.DeleteOne(e.ctx, bson.D{{Key: "_id", Value: 3}})
	if got := sink2.waitFor(t, 1); got[0].Op != "delete" {
		t.Fatalf("filtered: %+v", got)
	}
}

func TestChangeStreamsExplainStandalone(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	if s.Topology != "standalone" {
		t.Skip("MONGO_URI is not a standalone server")
	}
	if err := streams.New(e.mgr).Start("x", streams.Request{ConnID: e.conn, DB: e.db}); !apperr.Is(err, apperr.CodeUnsupported) {
		t.Fatalf("standalone: %v", err)
	}
}
