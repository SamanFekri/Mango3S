//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/monitor"
)

// setupOptional connects to the server named by envVar, or skips when it's
// unset (e.g. MONGO_RS_URI for the replica set from `make rs-up`).
func setupOptional(t *testing.T, envVar string) env {
	t.Helper()
	uri := os.Getenv(envVar)
	if uri == "" {
		t.Skipf("%s not set", envVar)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	mgr := mongox.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := mgr.Connect(ctx, config.Profile{ID: "opt", Name: envVar, URI: uri, ConnectTimeoutSeconds: 5}, mongox.Credentials{}); err != nil {
		t.Fatalf("cannot reach %s (%s): %v", uri, envVar, err)
	}
	db := "mango3s_it_" + config.NewID()
	t.Cleanup(func() {
		s, _ := mgr.Session("opt")
		_ = s.Client.Database(db).Drop(context.Background())
		mgr.CloseAll(context.Background())
	})
	return env{ctx: ctx, mgr: mgr, conn: "opt", db: db}
}

func TestMonitorSamplesAndOverview(t *testing.T) {
	e := setup(t)
	got := make(chan monitor.Sample, 10)
	m := monitor.New(e.mgr, func(_ string, s monitor.Sample) { got <- s })
	if _, err := m.Start(e.conn, time.Second); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(e.conn)
	first := <-got
	second := <-got
	if !first.First || second.First || second.ConnsCur == 0 || second.Command <= 0 {
		t.Fatalf("samples: %+v / %+v", first, second)
	}
	snap, err := m.Start(e.conn, time.Second) // a second tab joins the same run
	if err != nil || len(snap.Samples) < 2 {
		t.Fatalf("history: %d samples, %v", len(snap.Samples), err)
	}
	s, _ := e.mgr.Session(e.conn)
	ov, err := monitor.ReadOverview(e.ctx, s.Client)
	if err != nil || ov.Process != "mongod" || ov.Version == "" || ov.UptimeSecs <= 0 {
		t.Fatalf("overview: %+v %v", ov, err)
	}
}

func TestMonitorListsAndKillsASlowOperation(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	coll := s.Client.Database(e.db).Collection("slow")
	if _, err := coll.InsertOne(e.ctx, bson.D{{Key: "x", Value: 1}}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		// Runs ~20 s unless killed.
		done <- coll.FindOne(context.Background(), bson.D{{Key: "$where", Value: "sleep(20000) || true"}}).Err()
	}()

	m := monitor.New(e.mgr, nil)
	var op monitor.Op
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && op.OpID == "" {
		ops, err := m.CurrentOps(e.ctx, e.conn, monitor.OpsFilter{NS: e.db + ".slow"})
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range ops {
			if strings.Contains(o.Command, "$where") {
				op = o
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if op.OpID == "" {
		t.Fatal("the slow $where operation was not listed")
	}
	if ops, _ := m.CurrentOps(e.ctx, e.conn, monitor.OpsFilter{NS: e.db + ".slow", MinSeconds: 3600}); len(ops) != 0 {
		t.Fatalf("minimum duration filter: %+v", ops)
	}
	if err := m.KillOp(e.ctx, e.conn, op.OpID, "nope"); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("kill without typing the ID: %v", err)
	}

	// A read-only connection can't kill anything.
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if _, err := e.mgr.Connect(e.ctx, config.Profile{ID: "ro", Name: "ro", URI: uri, ReadOnly: true, ConnectTimeoutSeconds: 5}, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if err := m.KillOp(e.ctx, "ro", op.OpID, op.OpID); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("read-only kill: %v", err)
	}

	if err := m.KillOp(e.ctx, e.conn, op.OpID, op.OpID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "interrupt") {
			t.Fatalf("the killed query should fail as interrupted, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the operation was not killed")
	}
}

func TestMonitorReplicaSetLagAndOplog(t *testing.T) {
	e := setupOptional(t, "MONGO_RS_URI")
	s, _ := e.mgr.Session(e.conn)
	ov, err := monitor.ReadOverview(e.ctx, s.Client)
	if err != nil {
		t.Fatal(err)
	}
	r := ov.Replication
	if r == nil || r.Set != "rs0" || len(r.Members) != 1 || r.Problem != "" {
		t.Fatalf("replication: %+v", r)
	}
	if m := r.Members[0]; m.State != "PRIMARY" || m.LagSecs == nil || *m.LagSecs != 0 {
		t.Fatalf("member: %+v", m)
	}
	if r.OplogHours == nil || r.OplogSizeMB == nil || *r.OplogSizeMB <= 0 {
		t.Fatalf("oplog window: %+v", r)
	}
}
