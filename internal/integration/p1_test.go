//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/aggregate"
	"mango3s/internal/api"
	"mango3s/internal/config"
	"mango3s/internal/explain"
	"mango3s/internal/jobs"
	"mango3s/internal/library"
	"mango3s/internal/manage"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/schema"
	"mango3s/internal/secrets"
	"mango3s/internal/transfer"
)

// representative returns documents covering the BSON types people store.
func representative(n int) []any {
	docs := make([]any, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range docs {
		price, _ := bson.ParseDecimal128(fmt.Sprintf("%d.%02d", 10+i%90, i%100))
		d := bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "orderNo", Value: int64(100000 + i)},
			{Key: "qty", Value: int32(1 + i%5)},
			{Key: "price", Value: price},
			{Key: "rating", Value: float64(i%50) / 10},
			{Key: "status", Value: []string{"pending", "shipped", "delivered", "cancelled"}[i%4]},
			{Key: "createdAt", Value: bson.NewDateTimeFromTime(base.Add(time.Duration(i) * time.Hour))},
			{Key: "customer", Value: bson.D{{Key: "name", Value: fmt.Sprintf("Customer %d, \"VIP\"", i%37)}, {Key: "city", Value: []string{"Paris", "Berlin", "Rome"}[i%3]}}},
			{Key: "tags", Value: bson.A{"t" + fmt.Sprint(i%7), int32(i % 3)}},
			{Key: "active", Value: i%2 == 0},
		}
		if i%10 == 0 {
			d = append(d, bson.E{Key: "note", Value: nil})
		}
		docs[i] = d
	}
	return docs
}

func seedDocs(t *testing.T, e env, coll string, docs []any) *mongo.Collection {
	t.Helper()
	s, _ := e.mgr.Session(e.conn)
	c := s.Client.Database(e.db).Collection(coll)
	if _, err := c.InsertMany(e.ctx, docs); err != nil {
		t.Fatal(err)
	}
	return c
}

func allDocs(t *testing.T, ctx context.Context, c *mongo.Collection) []bson.D {
	t.Helper()
	cur, err := c.Find(ctx, bson.D{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []bson.D
	if err := cur.All(ctx, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func byID(docs []bson.D) map[string]bson.D {
	m := map[string]bson.D{}
	for _, d := range docs {
		id, _ := mql.Lookup(d, "_id")
		m[mql.Compact(id)] = d
	}
	return m
}

func runJob(t *testing.T, m *jobs.Manager, j jobs.Job) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ := m.Get(j.ID)
		if cur.State.Finished() {
			return cur
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", j.ID)
	return jobs.Job{}
}

func TestExportImportRoundTrip(t *testing.T) {
	e := setup(t)
	src := seedDocs(t, e, "orders", representative(2500))
	original := byID(allDocs(t, e.ctx, src))
	dir := t.TempDir()
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	svc := transfer.New(e.mgr, jm)

	cases := []struct {
		name  string
		file  string
		opts  transfer.ExportOptions
		exact bool
	}{
		{"canonical-array", "orders.json", transfer.ExportOptions{JSONMode: transfer.JSONArray, Canonical: true, Pretty: true}, true},
		{"canonical-ndjson", "orders.ndjson", transfer.ExportOptions{JSONMode: transfer.JSONLines, Canonical: true}, true},
		{"relaxed-ndjson", "orders.jsonl", transfer.ExportOptions{JSONMode: transfer.JSONLines}, false},
	}
	for _, c := range cases {
		path := filepath.Join(dir, c.file)
		o := c.opts
		o.ConnID, o.DB, o.Coll, o.Path, o.Format = e.conn, e.db, "orders", path, transfer.FormatJSON
		j, err := svc.StartExport(o)
		if err != nil {
			t.Fatal(err)
		}
		done := runJob(t, jm, j)
		if done.State != jobs.Succeeded || done.Done != 2500 || done.Total != 2500 {
			t.Fatalf("%s export: %+v", c.name, done)
		}
		if leftovers, _ := filepath.Glob(filepath.Join(dir, ".*.tmp")); len(leftovers) > 0 {
			t.Fatalf("temp files left: %v", leftovers)
		}

		target := "imp_" + strings.ReplaceAll(c.name, "-", "_")
		j, err = svc.StartImport(transfer.ImportOptions{ConnID: e.conn, DB: e.db, Coll: target, Path: path, BatchSize: 700})
		if err != nil {
			t.Fatal(err)
		}
		done = runJob(t, jm, j)
		if done.State != jobs.Succeeded || !strings.Contains(done.Result, "2500 inserted") || done.Done != done.Total {
			t.Fatalf("%s import: %+v", c.name, done)
		}
		s, _ := e.mgr.Session(e.conn)
		back := allDocs(t, e.ctx, s.Client.Database(e.db).Collection(target))
		if len(back) != 2500 {
			t.Fatalf("%s: %d docs imported", c.name, len(back))
		}
		for _, d := range back {
			id, _ := mql.Lookup(d, "_id")
			orig := original[mql.Compact(id)]
			if c.exact {
				if !mql.Equal(orig, d) {
					t.Fatalf("%s: document changed:\n%s\n%s", c.name, mql.Compact(orig), mql.Compact(d))
				}
				continue
			}
			// Relaxed JSON keeps values but folds small int64 into int32.
			for _, f := range []string{"price", "createdAt", "status", "customer", "_id"} {
				a, _ := mql.Lookup(orig, f)
				b, _ := mql.Lookup(d, f)
				if !mql.Equal(a, b) {
					t.Fatalf("%s: field %s changed: %s vs %s", c.name, f, mql.Compact(a), mql.Compact(b))
				}
			}
		}
	}
}

func TestCSVExportImport(t *testing.T) {
	e := setup(t)
	seedDocs(t, e, "orders", representative(1200))
	dir := t.TempDir()
	path := filepath.Join(dir, "orders.csv")
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	svc := transfer.New(e.mgr, jm)

	// Filtered, sorted export of selected (nested) fields.
	j, err := svc.StartExport(transfer.ExportOptions{
		ConnID: e.conn, DB: e.db, Coll: "orders", Path: path, Format: transfer.FormatCSV,
		Filter: `{status: "shipped"}`, Sort: `{orderNo: 1}`, Fields: []string{"orderNo", "qty", "customer.city", "createdAt", "tags"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if done := runJob(t, jm, j); done.State != jobs.Succeeded || done.Done != 300 {
		t.Fatalf("csv export: %+v", done)
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "orderNo,qty,customer.city,createdAt,tags" || len(lines) != 301 {
		t.Fatalf("csv: header %q, %d lines", lines[0], len(lines))
	}
	if !strings.HasPrefix(lines[1], "100001,2,Berlin,2024-01-01T01:00:00.000Z,") {
		t.Fatalf("first row: %q", lines[1])
	}

	preview, err := transfer.PreviewFile(transfer.ImportOptions{Path: path}, 5)
	if err != nil || len(preview.Documents) != 5 || preview.Columns[0].Inferred != transfer.TypeInt {
		t.Fatalf("preview: %+v %v", preview, err)
	}

	j, err = svc.StartImport(transfer.ImportOptions{ConnID: e.conn, DB: e.db, Coll: "from_csv", Path: path,
		Columns: []transfer.ColumnSpec{{Name: "orderNo", Type: transfer.TypeLong}}})
	if err != nil {
		t.Fatal(err)
	}
	if done := runJob(t, jm, j); done.State != jobs.Succeeded {
		t.Fatalf("csv import: %+v", done)
	}
	s, _ := e.mgr.Session(e.conn)
	var first bson.D
	if err := s.Client.Database(e.db).Collection("from_csv").FindOne(e.ctx, bson.D{{Key: "orderNo", Value: int64(100001)}}).Decode(&first); err != nil {
		t.Fatal(err)
	}
	want := `"orderNo": NumberLong(100001), "qty": 2, "customer": {"city": "Berlin"}, "createdAt": ISODate("2024-01-01T01:00:00.000Z"), "tags": ["t1", 1]}`
	if got := mql.Compact(first); !strings.HasSuffix(got, want) {
		t.Fatalf("csv round trip:\n got %s\nwant ...%s", got, want)
	}
}

// Roadmap 1.3: exporting an aggregation's results streams the pipeline, and
// the file imports back.
func TestExportAggregationResults(t *testing.T) {
	e := setup(t)
	seedDocs(t, e, "orders", representative(1200))
	dir := t.TempDir()
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	svc := transfer.New(e.mgr, jm)
	stages := []aggregate.Stage{
		{Operator: "$match", Body: `{status: "shipped"}`, Enabled: true},
		{Operator: "$group", Body: `{_id: "$customer.city", n: {$sum: 1}}`, Enabled: true},
		{Operator: "$limit", Body: `1`, Enabled: false}, // disabled stages are skipped
		{Operator: "$sort", Body: `{_id: 1}`, Enabled: true},
	}
	s, _ := e.mgr.Session(e.conn)
	db := s.Client.Database(e.db)

	jsonPath := filepath.Join(dir, "by-city.json")
	j, err := svc.StartExport(transfer.ExportOptions{ConnID: e.conn, DB: e.db, Coll: "orders", Path: jsonPath, Format: transfer.FormatJSON,
		Pipeline: stages, Filter: `{status: "pending"}`}) // the filter is ignored with a pipeline
	if err != nil {
		t.Fatal(err)
	}
	done := runJob(t, jm, j)
	if done.State != jobs.Succeeded || done.Total != 0 || done.Done == 0 {
		t.Fatalf("pipeline export: %+v", done)
	}
	j, err = svc.StartImport(transfer.ImportOptions{ConnID: e.conn, DB: e.db, Coll: "by_city", Path: jsonPath})
	if err != nil {
		t.Fatal(err)
	}
	if imp := runJob(t, jm, j); imp.State != jobs.Succeeded {
		t.Fatalf("import: %+v", imp)
	}
	back := allDocs(t, e.ctx, db.Collection("by_city"))
	if int64(len(back)) != done.Done {
		t.Fatalf("imported %d of %d exported", len(back), done.Done)
	}
	var total int64
	for _, d := range back {
		switch n, _ := mql.Lookup(d, "n"); v := n.(type) {
		case int32:
			total += int64(v)
		case int64:
			total += v
		default:
			t.Fatalf("n = %T %v", n, n)
		}
	}
	if shipped, _ := db.Collection("orders").CountDocuments(e.ctx, bson.D{{Key: "status", Value: "shipped"}}); total != shipped {
		t.Fatalf("group counts add up to %d, want %d shipped orders", total, shipped)
	}

	// CSV takes its columns from the results.
	csvPath := filepath.Join(dir, "by-city.csv")
	j, err = svc.StartExport(transfer.ExportOptions{ConnID: e.conn, DB: e.db, Coll: "orders", Path: csvPath, Format: transfer.FormatCSV, Pipeline: stages})
	if err != nil {
		t.Fatal(err)
	}
	if d := runJob(t, jm, j); d.State != jobs.Succeeded {
		t.Fatalf("csv pipeline export: %+v", d)
	}
	data, _ := os.ReadFile(csvPath)
	if lines := strings.Split(strings.TrimSpace(string(data)), "\n"); lines[0] != "_id,n" || int64(len(lines)-1) != done.Done {
		t.Fatalf("csv: header %q, %d rows", lines[0], len(lines)-1)
	}

	// A pipeline that writes its own results is refused up front.
	_, err = svc.StartExport(transfer.ExportOptions{ConnID: e.conn, DB: e.db, Coll: "orders", Path: filepath.Join(dir, "x.json"),
		Pipeline: append(stages, aggregate.Stage{Operator: "$out", Body: `"elsewhere"`, Enabled: true})})
	if err == nil || !strings.Contains(err.Error(), "$out") {
		t.Fatalf("$out export: %v", err)
	}
}

func TestImportConflictsAndModes(t *testing.T) {
	e := setup(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "people.ndjson")
	var b strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "{\"_id\": %d, \"name\": \"v1-%d\"}\n", i, i)
	}
	_ = os.WriteFile(path, []byte(b.String()), 0o600)
	s, _ := e.mgr.Session(e.conn)
	c := s.Client.Database(e.db).Collection("people")
	_, _ = c.InsertMany(e.ctx, []any{bson.D{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "old"}}, bson.D{{Key: "_id", Value: int32(5)}, {Key: "name", Value: "old"}}})

	// Insert mode: duplicates fail, the rest is written.
	st, err := transfer.RunImport(e.ctx, c, transfer.ImportOptions{Path: path, BatchSize: 4}, nil)
	if err != nil || !strings.Contains(st, "8 inserted") || !strings.Contains(st, "2 failed") || !strings.Contains(st, "duplicate key") {
		t.Fatalf("insert mode: %q %v", st, err)
	}
	// Stop on error.
	_, err = transfer.RunImport(e.ctx, c, transfer.ImportOptions{Path: path, StopOnError: true}, nil)
	if err == nil {
		t.Fatal("stopOnError did not stop")
	}
	// Upsert mode replaces existing documents.
	_ = os.WriteFile(path, []byte(strings.ReplaceAll(b.String(), "v1-", "v2-")+"{\"name\": \"no id\"}\n"), 0o600)
	st, err = transfer.RunImport(e.ctx, c, transfer.ImportOptions{Path: path, Mode: transfer.ModeUpsert}, nil)
	if err != nil || !strings.Contains(st, "10 replaced") || !strings.Contains(st, "1 inserted") {
		t.Fatalf("upsert: %q %v", st, err)
	}
	if n, _ := c.CountDocuments(e.ctx, bson.D{{Key: "name", Value: bson.D{{Key: "$regex", Value: "^v2-"}}}}); n != 10 {
		t.Fatalf("upserted docs: %d", n)
	}
	// Drop first + a malformed file reports the location.
	_ = os.WriteFile(path, []byte("{\"a\": 1}\n{\"a\": \n"), 0o600)
	_, err = transfer.RunImport(e.ctx, c, transfer.ImportOptions{Path: path, DropFirst: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "document 2") {
		t.Fatalf("malformed: %v", err)
	}
	if n, _ := c.CountDocuments(e.ctx, bson.D{}); n != 1 {
		t.Fatalf("drop first + partial import: %d docs", n)
	}
}

func TestExportCancelLeavesNoFile(t *testing.T) {
	e := setup(t)
	seedDocs(t, e, "big", representative(20000))
	path := filepath.Join(t.TempDir(), "big.json")
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	j, err := transfer.New(e.mgr, jm).StartExport(transfer.ExportOptions{ConnID: e.conn, DB: e.db, Coll: "big", Path: path, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	for {
		cur, _ := jm.Get(j.ID)
		if cur.Done > 0 || cur.State.Finished() {
			break
		}
		time.Sleep(time.Millisecond)
	}
	jm.Cancel(j.ID)
	done := runJob(t, jm, j)
	if done.State == jobs.Succeeded {
		t.Skip("export finished before it could be canceled")
	}
	if done.State != jobs.Canceled {
		t.Fatalf("state %s: %s", done.State, done.Error)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("canceled export left a file")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 0 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestSchemaOnRealData(t *testing.T) {
	e := setup(t)
	seedDocs(t, e, "orders", representative(400))
	rep, err := schema.New(e.mgr).Analyze(e.ctx, schema.Request{ConnID: e.conn, DB: e.db, Coll: "orders", SampleSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Sampled != 400 || rep.EstimatedDocs != 400 {
		t.Fatalf("sampled %d of %d", rep.Sampled, rep.EstimatedDocs)
	}
	get := func(p string) schema.Field {
		for _, f := range rep.Fields {
			if f.Path == p {
				return f
			}
		}
		t.Fatalf("missing %s", p)
		return schema.Field{}
	}
	if f := get("note"); f.Presence != 10 || f.Types[0].Type != "null" {
		t.Fatalf("note: %+v", f)
	}
	if f := get("orderNo"); f.Types[0].Type != "int64" || f.Min != "NumberLong(100000)" || f.Max != "NumberLong(100399)" || !f.Unique {
		t.Fatalf("orderNo: %+v", f)
	}
	if f := get("customer.city"); f.Distinct != 3 || f.Presence != 100 {
		t.Fatalf("customer.city: %+v", f)
	}
	if f := get("tags"); len(f.ElementTypes) != 2 {
		t.Fatalf("tags: %+v", f)
	}
	// Filtered sample.
	rep, err = schema.New(e.mgr).Analyze(e.ctx, schema.Request{ConnID: e.conn, DB: e.db, Coll: "orders", SampleSize: 50, Filter: `{status: "pending"}`})
	if err != nil || rep.Sampled != 50 {
		t.Fatalf("filtered: %d %v", rep.Sampled, err)
	}
}

// The validator generated from a full sample accepts every existing document,
// even when unknown fields are rejected.
func TestValidatorFromSchemaAcceptsExistingDocuments(t *testing.T) {
	e := setup(t)
	coll := seedDocs(t, e, "orders_validated", representative(400))
	rep, err := schema.New(e.mgr).Analyze(e.ctx, schema.Request{ConnID: e.conn, DB: e.db, Coll: "orders_validated", SampleSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range []schema.JSONSchemaOptions{{}, {RejectUnknown: true}} {
		v := mql.Pretty(schema.ToJSONSchema(rep, o))
		r, err := manage.CheckValidator(e.ctx, coll, v)
		if err != nil {
			t.Fatalf("%+v: %v\n%s", o, err, v)
		}
		if r.Checked != 400 || r.Failing != 0 {
			t.Fatalf("%+v: %d of %d documents fail the generated rule (%v)\n%s", o, r.Failing, r.Checked, r.Sample, v)
		}
	}
}

// A profiled command (with the session fields the profiler records) can be
// explained before and after an index, as the index advisor's Compare plan does.
func TestExplainCommandBeforeAndAfterIndex(t *testing.T) {
	e := setup(t)
	c := seedDocs(t, e, "orders_cmd", representative(2000))
	ex := explain.New(e.mgr)
	cmd := `{find: "orders_cmd", filter: {status: "shipped"}, sort: {createdAt: -1}, limit: 10, lsid: {id: UUID("123e4567-e89b-12d3-a456-426614174000")}, $db: "` + e.db + `"}`
	before, err := ex.Explain(e.ctx, explain.Request{Kind: "command", ConnID: e.conn, DB: e.db, Command: cmd})
	if err != nil {
		t.Fatal(err)
	}
	if !before.Summary.CollScan {
		t.Fatalf("before: %+v", before.Summary)
	}
	if _, err := c.Indexes().CreateOne(e.ctx, mongo.IndexModel{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}}); err != nil {
		t.Fatal(err)
	}
	after, err := ex.Explain(e.ctx, explain.Request{Kind: "command", ConnID: e.conn, DB: e.db, Command: cmd})
	if err != nil {
		t.Fatal(err)
	}
	if after.Summary.CollScan || *after.Summary.DocsExamined >= *before.Summary.DocsExamined {
		t.Fatalf("after: %+v (before %+v)", after.Summary, before.Summary)
	}
}

func TestExplainOnRealServer(t *testing.T) {
	e := setup(t)
	c := seedDocs(t, e, "orders", representative(3000))
	ex := explain.New(e.mgr)
	q := query.Request{ConnID: e.conn, DB: e.db, Coll: "orders", Filter: `{status: "shipped", qty: {$gte: 3}}`, Sort: `{createdAt: -1}`}

	p, err := ex.Explain(e.ctx, explain.Request{Kind: "find", Query: q})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Summary.CollScan || p.Root == nil || *p.Summary.DocsExamined != 3000 || p.Namespace != e.db+".orders" {
		t.Fatalf("collscan plan: %+v\n%s", p.Summary, p.Raw)
	}
	titles := map[string]bool{}
	for _, d := range p.Diagnostics {
		titles[d.Title] = true
	}
	if !titles["Collection scan"] || !titles["In-memory sort"] {
		t.Fatalf("diagnostics: %+v", p.Diagnostics)
	}

	if _, err := c.Indexes().CreateOne(e.ctx, mongo.IndexModel{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}, {Key: "qty", Value: 1}}}); err != nil {
		t.Fatal(err)
	}
	p, err = ex.Explain(e.ctx, explain.Request{Kind: "find", Query: q})
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.CollScan || p.Summary.InMemorySort || len(p.Summary.IndexesUsed) != 1 {
		t.Fatalf("indexed plan: %+v\n%s", p.Summary, p.Raw)
	}

	stages := []aggregate.Stage{
		{Operator: "$match", Body: `{status: "shipped"}`, Enabled: true},
		{Operator: "$group", Body: `{_id: "$customer.city", n: {$sum: 1}}`, Enabled: true},
		{Operator: "$sort", Body: `{n: -1}`, Enabled: true},
	}
	p, err = ex.Explain(e.ctx, explain.Request{Kind: "aggregate", ConnID: e.conn, DB: e.db, Coll: "orders", Stages: stages})
	if err != nil {
		t.Fatal(err)
	}
	if p.Root == nil || len(p.Summary.IndexesUsed) == 0 {
		t.Fatalf("aggregate plan: root=%v %+v\n%s", p.Root, p.Summary, p.Raw)
	}
	// $out pipelines are planned, not executed.
	out := append(stages[:1:1], aggregate.Stage{Operator: "$out", Body: `"shouldnotexist"`, Enabled: true})
	if _, err := ex.Explain(e.ctx, explain.Request{Kind: "aggregate", ConnID: e.conn, DB: e.db, Coll: "orders", Stages: out}); err != nil {
		t.Fatal(err)
	}
	s, _ := e.mgr.Session(e.conn)
	names, _ := s.Client.Database(e.db).ListCollectionNames(e.ctx, bson.D{{Key: "name", Value: "shouldnotexist"}})
	if len(names) != 0 {
		t.Fatal("explain executed $out")
	}
}

func TestSQLHistoryAndLibraryThroughAPI(t *testing.T) {
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
	if _, err := app.Connect(p.ID); err != nil {
		t.Fatal(err)
	}
	db := "mango3s_it_" + config.NewID()
	defer app.DropDatabase(p.ID, db)
	docs := representative(200)
	var arr []string
	for _, d := range docs {
		arr = append(arr, mql.Compact(d))
	}
	if _, err := app.InsertDocuments(p.ID, db, "orders", "["+strings.Join(arr, ",")+"]"); err != nil {
		t.Fatal(err)
	}

	res, err := app.RunSQL("sql1", api.SQLRequest{ConnID: p.ID, DB: db, SQL: `SELECT orderNo, status FROM orders WHERE status IN ('shipped', 'delivered') AND qty >= 4 ORDER BY orderNo DESC LIMIT 5`, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Translation.Kind != "find" || len(res.Page.Documents) != 5 || !strings.Contains(res.Page.Documents[0].Text, `"orderNo": NumberLong(100198)`) {
		t.Fatalf("sql find: %+v %s", res.Translation, res.Page.Documents[0].Text)
	}
	res, err = app.RunSQL("sql2", api.SQLRequest{ConnID: p.ID, DB: db, SQL: `SELECT status, COUNT(*) AS n, SUM(qty) AS units FROM orders GROUP BY status HAVING COUNT(*) >= 50 ORDER BY status`, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if res.Translation.Kind != "aggregate" || len(res.Page.Documents) != 4 || !strings.Contains(res.Page.Documents[0].Text, `"status": "cancelled"`) || !strings.Contains(res.Page.Documents[0].Text, `"n": 50`) {
		t.Fatalf("sql aggregate: %+v", res.Page.Documents)
	}
	if _, err := app.RunSQL("sql3", api.SQLRequest{ConnID: p.ID, DB: db, SQL: `SELECT * FROM a JOIN b`}); err == nil {
		t.Fatal("JOIN accepted")
	}

	// Generated code for the same SQL.
	code, err := app.GenerateCode(api.CodeRequest{Language: "python", Source: "sql", DB: db, SQL: `SELECT * FROM orders WHERE qty > 2 LIMIT 3`})
	if err != nil || !strings.Contains(code, `{"qty": {"$gt": 2}}`) || !strings.Contains(code, ".limit(3)") {
		t.Fatalf("codegen: %v\n%s", err, code)
	}

	if _, err := app.RunQuery("q", query.Request{ConnID: p.ID, DB: db, Coll: "orders", Filter: `{qty: 1}`}); err != nil {
		t.Fatal(err)
	}
	hist, err := app.ListHistory("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, h := range hist {
		kinds[h.Kind]++
	}
	if kinds[library.KindSQL] != 3 || kinds[library.KindQuery] != 1 {
		t.Fatalf("history: %+v", kinds)
	}
	if hist[0].Kind != library.KindQuery || hist[0].Query.Filter != `{qty: 1}` || !hist[0].OK {
		t.Fatalf("latest history entry: %+v", hist[0])
	}
	failed := hist[1]
	if failed.OK || !strings.Contains(failed.Error, "JOIN") {
		t.Fatalf("failed SQL entry: %+v", failed)
	}

	saved, err := app.SaveItem(library.Item{Kind: library.KindSQL, Name: "Statuses", Text: "SELECT status FROM orders"})
	if err != nil {
		t.Fatal(err)
	}
	// Restart: a fresh App over the same config dir sees saved items and history.
	app2 := api.New(api.Deps{Log: log, Store: config.NewStore(dir), Secrets: secrets.NewMemoryStore(), Conns: mongox.NewManager(log)})
	items, _ := app2.ListSaved("")
	if len(items) != 1 || items[0].ID != saved.ID {
		t.Fatalf("saved items after restart: %+v", items)
	}
	if h, _ := app2.ListHistory("", "", 0); len(h) != len(hist) {
		t.Fatalf("history after restart: %d vs %d", len(h), len(hist))
	}
}

// Roadmap 6.1: export to .xlsx and import it back with the CSV column
// mapping; typed cells come back typed.
func TestExcelExportImport(t *testing.T) {
	e := setup(t)
	src := seedDocs(t, e, "orders", representative(600))
	original := byID(allDocs(t, e.ctx, src))
	path := filepath.Join(t.TempDir(), "orders.xlsx")
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	svc := transfer.New(e.mgr, jm)

	j, err := svc.StartExport(transfer.ExportOptions{ConnID: e.conn, DB: e.db, Coll: "orders", Path: path}) // format from the extension
	if err != nil {
		t.Fatal(err)
	}
	if done := runJob(t, jm, j); done.State != jobs.Succeeded || done.Done != 600 {
		t.Fatalf("export: %+v", done)
	}
	preview, err := transfer.PreviewFile(transfer.ImportOptions{Path: path}, 3)
	if err != nil || preview.Format != transfer.FormatExcel || len(preview.Sheets) != 1 || preview.Sheets[0] != "orders" || len(preview.Documents) != 3 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	inferred := map[string]string{}
	for _, c := range preview.Columns {
		inferred[c.Name] = c.Inferred
	}
	for col, want := range map[string]string{"_id": transfer.TypeObjectID, "qty": transfer.TypeInt, "orderNo": transfer.TypeInt, "rating": transfer.TypeDouble,
		"createdAt": transfer.TypeDate, "active": transfer.TypeBool, "customer.city": transfer.TypeString} {
		if inferred[col] != want {
			t.Errorf("column %s inferred as %q, want %q", col, inferred[col], want)
		}
	}

	j, err = svc.StartImport(transfer.ImportOptions{ConnID: e.conn, DB: e.db, Coll: "from_xlsx", Path: path,
		Columns: []transfer.ColumnSpec{{Name: "orderNo", Type: transfer.TypeLong}, {Name: "price", Type: transfer.TypeDecimal}, {Name: "tags", Type: transfer.TypeJSON},
			{Name: "rating", Type: transfer.TypeDouble}}}) // 0.0 is stored as 0: a column type keeps it a double, as with CSV
	if err != nil {
		t.Fatal(err)
	}
	if done := runJob(t, jm, j); done.State != jobs.Succeeded || !strings.Contains(done.Result, "600 inserted") {
		t.Fatalf("import: %+v", done)
	}
	s, _ := e.mgr.Session(e.conn)
	back := allDocs(t, e.ctx, s.Client.Database(e.db).Collection("from_xlsx"))
	for _, d := range back {
		id, _ := mql.Lookup(d, "_id")
		orig := original[mql.Compact(id)]
		if orig == nil {
			t.Fatalf("unknown _id %v", mql.Compact(id))
		}
		for _, f := range []string{"orderNo", "qty", "price", "rating", "status", "createdAt", "customer", "tags", "active"} {
			a, _ := mql.Lookup(orig, f)
			b, _ := mql.Lookup(d, f)
			if !mql.Equal(a, b) {
				t.Fatalf("field %s changed: %s vs %s", f, mql.Compact(a), mql.Compact(b))
			}
		}
	}
}
