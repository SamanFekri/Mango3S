//go:build integration

package integration

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/api"
	"mango3s/internal/config"
	"mango3s/internal/jobs"
	"mango3s/internal/mongox"
	"mango3s/internal/secrets"
)

// Generated documents are written in batches as a job, repeatably by seed.
func TestTestDataGeneratorThroughAPI(t *testing.T) {
	e := setup(t)
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := api.New(api.Deps{Log: log, Store: config.NewStore(dir), Secrets: secrets.NewMemoryStore(), Conns: e.mgr})

	tmpl := `{ n: $seq(1), name: $name, age: $int(18, 90), at: $date("-30d", "now"), tags: [$word] }`
	req := api.TestDataRequest{ConnID: e.conn, DB: e.db, Coll: "people", Template: tmpl, Seed: 99, Count: 2500}
	prev, err := app.PreviewTestData(req)
	if err != nil || len(prev.Documents) != 5 || prev.Existing != 0 || !strings.Contains(prev.Documents[0].Text, `"n": 1`) {
		t.Fatalf("preview: %+v %v", prev, err)
	}
	job, err := app.StartTestData(req)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute)
	for {
		var j jobs.Job
		for _, x := range app.ListJobs() {
			if x.ID == job.ID {
				j = x
			}
		}
		if j.State == jobs.Succeeded {
			break
		}
		if j.State == jobs.Failed || time.Now().After(deadline) {
			t.Fatalf("job: %+v", j)
		}
		time.Sleep(100 * time.Millisecond)
	}
	s, _ := e.mgr.Session(e.conn)
	c := s.Client.Database(e.db).Collection("people")
	if n, _ := c.CountDocuments(e.ctx, bson.D{}); n != 2500 {
		t.Fatalf("%d documents", n)
	}
	if n, _ := c.CountDocuments(e.ctx, bson.D{{Key: "age", Value: bson.D{{Key: "$type", Value: "int"}, {Key: "$gte", Value: 18}, {Key: "$lte", Value: 90}}}}); n != 2500 {
		t.Fatalf("%d documents with an int age in range", n)
	}
	if n, _ := c.CountDocuments(e.ctx, bson.D{{Key: "at", Value: bson.D{{Key: "$type", Value: "date"}}}}); n != 2500 {
		t.Fatalf("%d documents with a date", n)
	}

	// The schema-based template resembles what's there.
	tpl, err := app.TestDataTemplateFromSchema(e.conn, e.db, "people")
	if err != nil || !strings.Contains(tpl, "$name") || !strings.Contains(tpl, "$int(18, 90)") {
		t.Fatalf("template from schema: %v\n%s", err, tpl)
	}

	// Read-only connections refuse.
	if _, err := e.mgr.Connect(e.ctx, config.Profile{ID: "ro-gen", Name: "ro", URI: "mongodb://localhost:27017", ReadOnly: true, ConnectTimeoutSeconds: 5}, mongox.Credentials{}); err == nil {
		if _, err := app.StartTestData(api.TestDataRequest{ConnID: "ro-gen", DB: e.db, Coll: "people", Template: tmpl, Count: 1}); err == nil || !strings.Contains(err.Error(), "READ_ONLY") {
			t.Fatalf("read-only: %v", err)
		}
	}
}
