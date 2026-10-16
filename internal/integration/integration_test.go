//go:build integration

// Package integration runs end-to-end workflows against a real MongoDB.
//
//	MONGO_URI=mongodb://localhost:27017 go test -tags integration ./internal/integration/...
//
// Each run uses a throwaway database that is dropped afterwards.
package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/aggregate"
	"mango3s/internal/api"
	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/documents"
	"mango3s/internal/explorer"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/secrets"
	"mango3s/internal/shell"
)

type env struct {
	ctx  context.Context
	mgr  *mongox.Manager
	conn string
	db   string
}

func setup(t *testing.T) env {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := mongox.NewManager(log)
	p := config.Profile{ID: "it", Name: "integration", URI: uri, ConnectTimeoutSeconds: 5}
	if _, err := mgr.Connect(ctx, p, mongox.Credentials{}); err != nil {
		t.Fatalf("cannot reach MongoDB at %s (set MONGO_URI): %v", uri, err)
	}
	db := "mango3s_it_" + config.NewID()
	t.Cleanup(func() {
		s, _ := mgr.Session("it")
		_ = s.Client.Database(db).Drop(context.Background())
		mgr.CloseAll(context.Background())
	})
	return env{ctx: ctx, mgr: mgr, conn: "it", db: db}
}

func TestDocumentLifecycle(t *testing.T) {
	e := setup(t)
	ex := explorer.New(e.mgr)
	docs := documents.New(e.mgr)
	q := query.New(e.mgr)

	if err := ex.CreateCollection(e.ctx, e.conn, e.db, "people"); err != nil {
		t.Fatal(err)
	}
	dbs, err := ex.ListDatabases(e.ctx, e.conn)
	if err != nil {
		t.Fatal(err)
	}
	if !containsDB(dbs, e.db) {
		t.Fatalf("database %s not listed", e.db)
	}
	colls, err := ex.ListCollections(e.ctx, e.conn, e.db)
	if err != nil || len(colls) != 1 || colls[0].Name != "people" {
		t.Fatalf("collections: %+v %v", colls, err)
	}

	ids, err := docs.Insert(e.ctx, e.conn, e.db, "people", `[
		{name: "Ada", age: 36, born: ISODate("1815-12-10"), tags: ["math"]},
		{name: "Grace", age: 85, big: NumberLong(1), price: NumberDecimal("9.99")},
		{_id: "custom", name: "Linus", age: 54, address: {city: "Helsinki"}}
	]`)
	if err != nil || len(ids) != 3 || ids[2] != `"custom"` {
		t.Fatalf("insert: %v %v", ids, err)
	}

	page, err := q.Find(e.ctx, query.Request{ConnID: e.conn, DB: e.db, Coll: "people",
		Filter: `{age: {$gte: 50}}`, Projection: `{name: 1}`, Sort: `{age: -1}`, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Documents) != 2 || !strings.Contains(page.Documents[0].Text, "Grace") || strings.Contains(page.Documents[0].Text, "age") {
		t.Fatalf("query page: %+v", page)
	}

	// Types survive the round trip through the editor text.
	got, err := docs.Get(e.ctx, e.conn, e.db, "people", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Text, "NumberLong(1)") || !strings.Contains(got.Text, `NumberDecimal("9.99")`) {
		t.Fatalf("types lost: %s", got.Text)
	}

	edited := strings.Replace(got.Text, `"age": 85`, `"age": 86`, 1)
	if _, err := docs.Replace(e.ctx, e.conn, e.db, "people", ids[1], edited); err != nil {
		t.Fatal(err)
	}
	got, _ = docs.Get(e.ctx, e.conn, e.db, "people", ids[1])
	if !strings.Contains(got.Text, `"age": 86`) || !strings.Contains(got.Text, "NumberLong(1)") {
		t.Fatalf("replace: %s", got.Text)
	}

	// Changing _id is refused and leaves the document untouched.
	bad := strings.Replace(got.Text, ids[1], `ObjectId("000000000000000000000000")`, 1)
	if _, err := docs.Replace(e.ctx, e.conn, e.db, "people", ids[1], bad); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("_id change: %v", err)
	}

	// Replacing a vanished document reports NOT_FOUND instead of upserting.
	if _, err := docs.Replace(e.ctx, e.conn, e.db, "people", `"ghost"`, `{a: 1}`); !apperr.Is(err, apperr.CodeNotFound) {
		t.Fatalf("ghost replace: %v", err)
	}

	// Duplicate _id is a CONFLICT.
	if _, err := docs.Insert(e.ctx, e.conn, e.db, "people", `{_id: "custom"}`); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("duplicate: %v", err)
	}

	n, err := docs.Delete(e.ctx, e.conn, e.db, "people", []string{ids[0], ids[2]})
	if err != nil || n != 2 {
		t.Fatalf("delete: %d %v", n, err)
	}
	total, _ := q.Count(e.ctx, query.Request{ConnID: e.conn, DB: e.db, Coll: "people"})
	if total != 1 {
		t.Fatalf("count after delete: %d", total)
	}

	stats, err := ex.CollectionStats(e.ctx, e.conn, e.db, "people")
	if err != nil || stats.Count != 1 || len(stats.Indexes) == 0 {
		t.Fatalf("stats: %+v %v", stats, err)
	}
	fields, err := ex.SampleFields(e.ctx, e.conn, e.db, "people", 10)
	if err != nil || !contains(fields, "name") {
		t.Fatalf("fields: %v %v", fields, err)
	}
}

func TestPagination(t *testing.T) {
	e := setup(t)
	seed(t, e, "nums", 125)
	q := query.New(e.mgr)
	req := query.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Sort: `{n: 1}`, PageSize: 50}

	var seen []int
	for page := 0; ; page++ {
		req.Page = page
		p, err := q.Find(e.ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range p.Documents {
			seen = append(seen, nField(t, d.JSON))
		}
		if !p.HasMore {
			break
		}
		if page > 5 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != 125 || seen[0] != 0 || seen[124] != 124 {
		t.Fatalf("saw %d docs: first %v last %v", len(seen), seen[0], seen[len(seen)-1])
	}

	// User skip/limit combine with paging.
	req = query.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Sort: `{n: 1}`, Skip: 10, Limit: 60, PageSize: 50, Page: 1}
	p, err := q.Find(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Documents) != 10 || p.HasMore || nField(t, p.Documents[0].JSON) != 60 {
		t.Fatalf("skip/limit page: %d docs, more=%v", len(p.Documents), p.HasMore)
	}
	n, _ := q.Count(e.ctx, req)
	if n != 60 {
		t.Fatalf("count with skip/limit = %d", n)
	}
}

func TestAggregation(t *testing.T) {
	e := setup(t)
	seed(t, e, "nums", 100)
	agg := aggregate.New(e.mgr)
	stages := []aggregate.Stage{
		{Operator: "$match", Body: `{n: {$lt: 50}}`, Enabled: true},
		{Operator: "$group", Body: `{_id: {$mod: ["$n", 2]}, count: {$sum: 1}, total: {$sum: "$n"}}`, Enabled: true},
		{Operator: "$sort", Body: `{_id: 1}`, Enabled: true},
	}
	p, err := agg.Run(e.ctx, aggregate.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Stages: stages, UpTo: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Documents) != 2 {
		t.Fatalf("groups: %+v", p.Documents)
	}
	// even numbers 0..48: 25 docs summing to 600
	var even struct {
		Count int `bson:"count"`
		Total int `bson:"total"`
	}
	if err := bson.UnmarshalExtJSON([]byte(p.Documents[0].JSON), true, &even); err != nil || even.Count != 25 || even.Total != 600 {
		t.Fatalf("even group: %+v from %s (%v)", even, p.Documents[0].JSON, err)
	}

	// Preview up to the first stage, paged.
	p, err = agg.Run(e.ctx, aggregate.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Stages: stages, UpTo: 0, PageSize: 20, Page: 2})
	if err != nil || len(p.Documents) != 10 || p.HasMore {
		t.Fatalf("preview page: %d docs more=%v err=%v", len(p.Documents), p.HasMore, err)
	}

	// $out writes and reports a notice.
	out := append(stages[:1:1], aggregate.Stage{Operator: "$out", Body: `"small"`, Enabled: true})
	p, err = agg.Run(e.ctx, aggregate.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Stages: out, UpTo: -1})
	if err != nil || p.Notice == "" {
		t.Fatalf("$out: %+v %v", p, err)
	}
	n, _ := query.New(e.mgr).Count(e.ctx, query.Request{ConnID: e.conn, DB: e.db, Coll: "small"})
	if n != 50 {
		t.Fatalf("$out wrote %d docs", n)
	}

	// Server errors surface as MONGO_ERROR with the stage problem.
	_, err = agg.Run(e.ctx, aggregate.Request{ConnID: e.conn, DB: e.db, Coll: "nums",
		Stages: []aggregate.Stage{{Operator: "$bogus", Body: "{}", Enabled: true}}, UpTo: -1})
	if !apperr.Is(err, apperr.CodeMongo) {
		t.Fatalf("bad stage: %v", err)
	}
}

func TestShell(t *testing.T) {
	e := setup(t)
	sh := shell.New(e.mgr)
	script := `
		use ` + e.db + `
		db.items.insertMany([{sku: "a", qty: 5}, {sku: "b", qty: 15}, {sku: "c", qty: 25}])
		db.items.updateOne({sku: "a"}, {$inc: {qty: 1}})
		db.items.find({qty: {$gt: 5}}).sort({qty: -1}).limit(2)
		db.items.countDocuments({})
		db.items.distinct("sku")
		db.items.aggregate([{$group: {_id: null, total: {$sum: "$qty"}}}])
		db.items.createIndex({sku: 1}, {unique: true})
		show collections
		db.runCommand({ping: 1})
	`
	res, err := sh.Execute(e.ctx, e.conn, "", script, 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.CurrentDB != e.db {
		t.Fatalf("current db %q", res.CurrentDB)
	}
	if len(res.Outputs) != 10 {
		for _, o := range res.Outputs {
			t.Logf("%s => %s %s", o.Statement, o.Kind, o.Text)
		}
		t.Fatalf("got %d outputs", len(res.Outputs))
	}
	for _, o := range res.Outputs {
		if o.Kind == "error" {
			t.Fatalf("%s failed: %s", o.Statement, o.Text)
		}
	}
	find := res.Outputs[3]
	if len(find.Documents) != 2 || !strings.Contains(find.Documents[0].Text, `"c"`) {
		t.Fatalf("find: %+v", find)
	}
	if res.Outputs[4].Text != "3" {
		t.Fatalf("count: %q", res.Outputs[4].Text)
	}
	if !strings.Contains(res.Outputs[6].Documents[0].Text, `"total": 46`) {
		t.Fatalf("aggregate: %+v", res.Outputs[6])
	}

	// Execution stops at the first error.
	res, _ = sh.Execute(e.ctx, e.conn, e.db, "db.items.insertOne({_id: 1})\ndb.items.insertOne({_id: 1})\ndb.items.drop()", 20)
	if len(res.Outputs) != 2 || res.Outputs[1].Kind != "error" {
		t.Fatalf("stop on error: %+v", res.Outputs)
	}

	// Output is capped and flagged.
	res, _ = sh.Execute(e.ctx, e.conn, e.db, "db.items.find()", 2)
	if len(res.Outputs[0].Documents) != 2 || !res.Outputs[0].HasMore {
		t.Fatalf("cap: %+v", res.Outputs[0])
	}
}

func TestReadOnlyConnection(t *testing.T) {
	e := setup(t)
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ro := config.Profile{ID: "ro", Name: "ro", URI: uri, ReadOnly: true, ConnectTimeoutSeconds: 5}
	if _, err := e.mgr.Connect(e.ctx, ro, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
	docs := documents.New(e.mgr)
	if _, err := docs.Insert(e.ctx, "ro", e.db, "x", `{a: 1}`); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("insert on read-only: %v", err)
	}
	res, _ := shell.New(e.mgr).Execute(e.ctx, "ro", e.db, `db.x.deleteMany({})`, 20)
	if res.Outputs[0].Kind != "error" || !strings.Contains(res.Outputs[0].Text, "read-only") {
		t.Fatalf("shell write on read-only: %+v", res.Outputs[0])
	}
	res, _ = shell.New(e.mgr).Execute(e.ctx, "ro", e.db, `db.runCommand({drop: "x"})`, 20)
	if res.Outputs[0].Kind != "error" {
		t.Fatalf("runCommand write on read-only: %+v", res.Outputs[0])
	}
	res, _ = shell.New(e.mgr).Execute(e.ctx, "ro", e.db, `db.runCommand({ping: 1})`, 20)
	if res.Outputs[0].Kind == "error" {
		t.Fatalf("ping on read-only: %+v", res.Outputs[0])
	}
}

func TestCancellationAndTimeout(t *testing.T) {
	e := setup(t)
	seed(t, e, "nums", 10)
	q := query.New(e.mgr)
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	_, err := q.Find(ctx, query.Request{ConnID: e.conn, DB: e.db, Coll: "nums"})
	if !apperr.Is(err, apperr.CodeCanceled) && !apperr.Is(err, apperr.CodeTimeout) {
		t.Fatalf("canceled: %v", err)
	}
	ctx, cancel = context.WithTimeout(e.ctx, 50*time.Millisecond)
	defer cancel()
	_, err = q.Find(ctx, query.Request{ConnID: e.conn, DB: e.db, Coll: "nums", Filter: `{$where: "sleep(500) || true"}`})
	if err == nil {
		t.Skip("server does not support sleep() in $where; skipping timeout check")
	}
	if !apperr.Is(err, apperr.CodeTimeout) && !apperr.Is(err, apperr.CodeMongo) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestAPIFacadeEndToEnd(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := api.New(api.Deps{Log: log, Store: config.NewStore(dir), Secrets: secrets.NewMemoryStore(), Conns: mongox.NewManager(log)})
	p, err := app.SaveProfile(api.ProfileInput{Name: "it", URI: uri, ConnectTimeoutSeconds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.TestConnection(api.ProfileInput{ID: p.ID, Name: "it", URI: uri}); err != nil {
		t.Fatal(err)
	}
	info, err := app.Connect(p.ID)
	if err != nil || info.ServerVersion == "" {
		t.Fatalf("connect: %+v %v", info, err)
	}
	db := "mango3s_it_" + config.NewID()
	defer app.DropDatabase(p.ID, db)
	if _, err := app.InsertDocuments(p.ID, db, "c", `{a: 1}`); err != nil {
		t.Fatal(err)
	}
	page, err := app.RunQuery("q1", query.Request{ConnID: p.ID, DB: db, Coll: "c"})
	if err != nil || len(page.Documents) != 1 {
		t.Fatalf("query: %+v %v", page, err)
	}
	// A bad filter returns a JSON-encoded INVALID_INPUT error.
	_, err = app.RunQuery("q1", query.Request{ConnID: p.ID, DB: db, Coll: "c", Filter: "{a:"})
	var ae apperr.Error
	if err == nil || json.Unmarshal([]byte(err.Error()), &ae) != nil || ae.Code != apperr.CodeInvalidInput {
		t.Fatalf("bad filter: %v", err)
	}
	if err := app.Disconnect(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ListDatabases(p.ID); err == nil {
		t.Fatal("query after disconnect succeeded")
	}
}

func seed(t *testing.T, e env, coll string, n int) {
	t.Helper()
	s, _ := e.mgr.Session(e.conn)
	docs := make([]any, n)
	for i := range docs {
		docs[i] = bson.D{{Key: "n", Value: i}}
	}
	if _, err := s.Client.Database(e.db).Collection(coll).InsertMany(e.ctx, docs); err != nil {
		t.Fatal(err)
	}
}

func nField(t *testing.T, js string) int {
	t.Helper()
	d, err := mql.ParseDocument(js)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := mql.Lookup(d, "n")
	switch n := v.(type) {
	case int32:
		return int(n)
	case int64:
		return int(n)
	}
	t.Fatalf("n has type %T in %s", v, js)
	return 0
}

func containsDB(dbs []explorer.DatabaseInfo, name string) bool {
	for _, d := range dbs {
		if d.Name == name {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// TestAuthentication needs a server with auth enabled, e.g.
// MONGO_AUTH_URI=mongodb://admin:change-me@localhost:27018/?authSource=admin
func TestAuthentication(t *testing.T) {
	uri := os.Getenv("MONGO_AUTH_URI")
	if uri == "" {
		t.Skip("MONGO_AUTH_URI not set")
	}
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sec := secrets.NewMemoryStore()
	app := api.New(api.Deps{Log: log, Store: config.NewStore(dir), Secrets: sec, Conns: mongox.NewManager(log)})

	p, err := app.SaveProfile(api.ProfileInput{Name: "auth", URI: uri, ConnectTimeoutSeconds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !p.HasPassword || strings.Contains(p.URI, "@") {
		t.Fatalf("credentials not split out of URI: %+v", p)
	}
	if _, err := app.Connect(p.ID); err != nil {
		t.Fatalf("connect with stored password: %v", err)
	}
	if _, err := app.ListDatabases(p.ID); err != nil {
		t.Fatal(err)
	}
	_ = app.Disconnect(p.ID)

	_, err = app.TestConnection(api.ProfileInput{ID: p.ID, Name: "auth", URI: p.URI, Username: p.Username, Password: "wrong-password", AuthSource: "admin", ConnectTimeoutSeconds: 5})
	var ae apperr.Error
	if err == nil || json.Unmarshal([]byte(err.Error()), &ae) != nil || ae.Code != apperr.CodeAuthFailed {
		t.Fatalf("wrong password: want AUTH_FAILED, got %v", err)
	}
	if strings.Contains(err.Error(), "wrong-password") {
		t.Fatal("password leaked into error")
	}
}
