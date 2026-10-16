//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/ai"
	"mango3s/internal/api"
	"mango3s/internal/backup"
	"mango3s/internal/compare"
	"mango3s/internal/config"
	"mango3s/internal/jobs"
	"mango3s/internal/masking"
	"mango3s/internal/mcpserver"
	"mango3s/internal/migrate"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/reschema"
	"mango3s/internal/scheduler"
	"mango3s/internal/secrets"
	"mango3s/internal/syncer"
	"mango3s/internal/transform"
)

func mongoURI() string {
	if u := os.Getenv("MONGO_URI"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}

// secondConn opens another session to the same server under a different
// profile ID, so comparisons and syncs run "across connections".
func secondConn(t *testing.T, e env, id string, readOnly bool) {
	t.Helper()
	p := config.Profile{ID: id, Name: id, URI: mongoURI(), ConnectTimeoutSeconds: 5, ReadOnly: readOnly}
	if _, err := e.mgr.Connect(e.ctx, p, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
}

func coll(t *testing.T, e env, conn, db, name string) *mongo.Collection {
	t.Helper()
	s, err := e.mgr.Session(conn)
	if err != nil {
		t.Fatal(err)
	}
	return s.Client.Database(db).Collection(name)
}

func snapshot(t *testing.T, ctx context.Context, c *mongo.Collection) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, d := range allDocs(t, ctx, c) {
		id, _ := mql.Lookup(d, "_id")
		out[compare.IDKey(id)] = mql.Compact(d)
	}
	return out
}

func sameSnapshot(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func seedPair(t *testing.T, e env) (src, dst *mongo.Collection) {
	t.Helper()
	secondConn(t, e, "it-target", false)
	src = coll(t, e, e.conn, e.db, "people")
	dst = coll(t, e, "it-target", e.db+"_copy", "people")
	t.Cleanup(func() { _ = dst.Database().Drop(context.Background()) })
	var a, b []any
	for i := 0; i < 300; i++ {
		d := bson.D{{Key: "_id", Value: int32(i)}, {Key: "name", Value: fmt.Sprintf("p%d", i)}, {Key: "n", Value: int64(i)},
			{Key: "meta", Value: bson.D{{Key: "x", Value: int32(1)}, {Key: "y", Value: int32(2)}}}, {Key: "updatedAt", Value: bson.DateTime(1000)}}
		a = append(a, d)
		switch {
		case i < 250: // identical, except updatedAt and key order on some
			c := mql.Compact(d)
			dd, _ := mql.ParseDocument(c)
			if i%10 == 0 {
				dd[4].Value = bson.DateTime(2000)
			}
			if i%25 == 0 {
				dd[3].Value = bson.D{{Key: "y", Value: int32(2)}, {Key: "x", Value: int32(1)}}
			}
			b = append(b, dd)
		case i < 270: // changed
			b = append(b, bson.D{{Key: "_id", Value: int32(i)}, {Key: "name", Value: "CHANGED"}, {Key: "n", Value: int64(i)},
				{Key: "meta", Value: bson.D{{Key: "x", Value: int32(1)}, {Key: "y", Value: int32(2)}}}, {Key: "updatedAt", Value: bson.DateTime(1000)}})
		}
		// 270..299 only in source
	}
	for i := 1000; i < 1015; i++ { // only in target
		b = append(b, bson.D{{Key: "_id", Value: int32(i)}, {Key: "name", Value: "extra"}})
	}
	if _, err := src.InsertMany(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.InsertMany(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	return src, dst
}

func TestCompareIsRepeatable(t *testing.T) {
	e := setup(t)
	seedPair(t, e)
	o := compare.Options{
		Source: compare.Endpoint{ConnID: e.conn, DB: e.db, Coll: "people"},
		Target: compare.Endpoint{ConnID: "it-target", DB: e.db + "_copy", Coll: "people"},
	}
	r1, err := compare.Run(e.ctx, e.mgr, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Strict: updatedAt (every 10th: 25 docs) and key order (every 25th: 10 docs, every 50th
	// overlaps: 5) count as changes, plus the 20 real changes.
	if r1.OnlySource != 30 || r1.OnlyTarget != 15 || r1.Changed != 20+25+10-5 || r1.Identical != 250-30 {
		t.Fatalf("strict counts: identical=%d changed=%d onlySource=%d onlyTarget=%d", r1.Identical, r1.Changed, r1.OnlySource, r1.OnlyTarget)
	}
	r2, _ := compare.Run(e.ctx, e.mgr, o, nil)
	if r1.Fingerprint != r2.Fingerprint || mql.Compact(bson.A{r1.Changed, r1.OnlySource}) != mql.Compact(bson.A{r2.Changed, r2.OnlySource}) {
		t.Fatal("comparison is not repeatable")
	}
	for i := range r1.Samples {
		if r1.Samples[i].ID != r2.Samples[i].ID {
			t.Fatal("sample order is not deterministic")
		}
	}
	o.IgnoreFields, o.IgnoreFieldOrder = []string{"updatedAt"}, true
	r3, _ := compare.Run(e.ctx, e.mgr, o, nil)
	if r3.Changed != 20 || r3.Identical != 250 {
		t.Fatalf("normalized counts: %+v", r3)
	}
	var nameDiff bool
	for _, s := range r3.Samples {
		for _, f := range s.Fields {
			if f.Path == "name" && f.Change == "changed" && f.Target == `"CHANGED"` {
				nameDiff = true
			}
		}
	}
	if !nameDiff {
		t.Fatalf("field-level diff missing: %+v", r3.Samples[:2])
	}

	db, err := compare.RunDB(e.ctx, e.mgr, compare.DBOptions{SourceConnID: e.conn, SourceDB: e.db, TargetConnID: "it-target", TargetDB: e.db + "_copy", Deep: true}, nil)
	if err != nil || len(db.Collections) != 1 || !db.Collections[0].Compared || db.Collections[0].OnlyTarget != 15 {
		t.Fatalf("db compare: %+v %v", db, err)
	}
}

func TestSyncPreviewMatchesApplyAndRestores(t *testing.T) {
	e := setup(t)
	_, dst := seedPair(t, e)
	before := snapshot(t, e.ctx, dst)
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 2)
	cmp := compare.New(e.mgr, jm)
	bdir := t.TempDir()
	svc := syncer.New(e.mgr, jm, cmp, bdir)
	o := compare.Options{
		Source:       compare.Endpoint{ConnID: e.conn, DB: e.db, Coll: "people"},
		Target:       compare.Endpoint{ConnID: "it-target", DB: e.db + "_copy", Coll: "people"},
		IgnoreFields: []string{"updatedAt"}, IgnoreFieldOrder: true,
	}
	cj, _ := cmp.StartCollections(o)
	runJob(t, jm, cj)

	pv, err := svc.Preview(cj.ID, syncer.Options{Insert: true, Replace: true, Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	if pv.Inserts != 30 || pv.Replaces != 20 || pv.Deletes != 15 || pv.ConfirmText == "" {
		t.Fatalf("preview: %+v", pv)
	}
	if _, err := svc.StartApply(pv.PlanID, "wrong"); err == nil {
		t.Fatal("deletes applied without typed confirmation")
	}
	// Drift: change one planned replace and one planned delete after the preview.
	_, _ = dst.UpdateOne(e.ctx, bson.D{{Key: "_id", Value: int32(255)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: "edited after preview"}}}})
	_, _ = dst.UpdateOne(e.ctx, bson.D{{Key: "_id", Value: int32(1003)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: "edited too"}}}})
	beforeApply := snapshot(t, e.ctx, dst)

	j, err := svc.StartApply(pv.PlanID, pv.ConfirmText)
	if err != nil {
		t.Fatal(err)
	}
	done := runJob(t, jm, j)
	out, _ := svc.Outcome(j.ID)
	if done.State != jobs.Succeeded || out == nil {
		t.Fatalf("apply: %+v", done)
	}
	if out.Applied != (syncer.Counts{Inserts: 30, Replaces: 19, Deletes: 14}) || out.Skipped != (syncer.Counts{Replaces: 1, Deletes: 1}) || out.MatchesPreview {
		t.Fatalf("drift handling: %+v", out)
	}
	if out.Remaining != (syncer.Counts{Replaces: 1, Deletes: 1}) {
		t.Fatalf("verification should show exactly the drifted docs: %+v", out.Remaining)
	}
	var d bson.D
	_ = dst.FindOne(e.ctx, bson.D{{Key: "_id", Value: int32(255)}}).Decode(&d)
	if v, _ := mql.Lookup(d, "name"); v != "edited after preview" {
		t.Fatal("a document changed after the preview was overwritten")
	}
	if _, err := svc.StartApply(pv.PlanID, pv.ConfirmText); err == nil {
		t.Fatal("plan applied twice")
	}

	// A clean second round: preview counts equal applied counts and the result verifies equal.
	cj2, _ := cmp.StartCollections(o)
	runJob(t, jm, cj2)
	pv2, _ := svc.Preview(cj2.ID, syncer.Options{Insert: true, Replace: true, Delete: true})
	j2, _ := svc.StartApply(pv2.PlanID, pv2.ConfirmText)
	runJob(t, jm, j2)
	out2, _ := svc.Outcome(j2.ID)
	if !out2.MatchesPreview || !out2.VerifiedEqual || out2.Applied != out2.Planned {
		t.Fatalf("clean sync: %+v", out2)
	}

	// Undo both syncs (newest first) and get the exact pre-sync state back.
	infos, _ := backup.List(bdir)
	if len(infos) != 2 {
		t.Fatalf("backups: %d", len(infos))
	}
	for _, b := range infos {
		if _, err := backup.Restore(e.ctx, dst, b.Path); err != nil {
			t.Fatal(err)
		}
	}
	if !sameSnapshot(snapshot(t, e.ctx, dst), beforeApply) {
		t.Fatal("restoring the backups did not reproduce the pre-sync state")
	}
	_ = before
}

func TestReschemaAndMaskingIntegrity(t *testing.T) {
	e := setup(t)
	src := coll(t, e, e.conn, e.db, "customers")
	var docs []any
	for i := 0; i < 500; i++ {
		docs = append(docs, bson.D{{Key: "_id", Value: int32(i)}, {Key: "fname", Value: fmt.Sprintf("Name%d", i)},
			{Key: "email", Value: fmt.Sprintf("user%d@corp.example", i)}, {Key: "ssn", Value: fmt.Sprintf("123-45-%04d", i)},
			{Key: "age", Value: fmt.Sprint(20 + i%50)}, {Key: "address", Value: bson.D{{Key: "city", Value: "Paris"}, {Key: "zip", Value: "75001"}}}})
	}
	docs = append(docs, bson.D{{Key: "_id", Value: int32(999)}, {Key: "fname", Value: "Bad"}, {Key: "age", Value: "unknown"}})
	if _, err := src.InsertMany(e.ctx, docs); err != nil {
		t.Fatal(err)
	}
	original := snapshot(t, e.ctx, src)
	bdir := t.TempDir()
	ops, _ := reschema.Compile([]reschema.Op{
		{Type: reschema.OpRename, Field: "fname", To: "firstName"},
		{Type: reschema.OpConvert, Field: "age", TargetType: "int"},
		{Type: reschema.OpFlatten, Field: "address"},
	})
	spec := transform.Spec{ConnID: e.conn, DB: e.db, Coll: "customers", Mode: transform.ModeCopy, TargetColl: "customers_v2", OnError: "skip"}
	pv, err := transform.RunPreview(e.ctx, e.mgr, spec, ops.Apply, 3)
	if err != nil || pv.Total != 501 || pv.Changed != 500 || pv.Errors != 1 || len(pv.Samples) != 3 {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	rep, err := transform.Apply(e.ctx, e.mgr, spec, ops.Apply, "reschema", bdir, nil)
	if err != nil || !rep.Valid || rep.Written != 500 || rep.Skipped != 1 {
		t.Fatalf("copy apply: %+v %v", rep, err)
	}
	var d bson.D
	_ = coll(t, e, e.conn, e.db, "customers_v2").FindOne(e.ctx, bson.D{{Key: "_id", Value: int32(7)}}).Decode(&d)
	if got := mql.Compact(d); got != `{"_id": 7, "firstName": "Name7", "email": "user7@corp.example", "ssn": "123-45-0007", "age": 27, "address_city": "Paris", "address_zip": "75001"}` {
		t.Fatalf("transformed doc: %s", got)
	}
	if !sameSnapshot(original, snapshot(t, e.ctx, src)) {
		t.Fatal("copy mode modified the source")
	}
	// Target must be empty: a second copy run refuses.
	if _, err := transform.Apply(e.ctx, e.mgr, spec, ops.Apply, "reschema", bdir, nil); err == nil {
		t.Fatal("copy into a non-empty target accepted")
	}

	// In place, then undo with the backup.
	spec.Mode, spec.TargetColl = transform.ModeInPlace, ""
	rep, err = transform.Apply(e.ctx, e.mgr, spec, ops.Apply, "reschema", bdir, nil)
	if err != nil || !rep.Valid || rep.Written != 500 || rep.BackupPath == "" {
		t.Fatalf("in-place: %+v %v", rep, err)
	}
	if _, err := backup.Restore(e.ctx, src, rep.BackupPath); err != nil {
		t.Fatal(err)
	}
	if !sameSnapshot(original, snapshot(t, e.ctx, src)) {
		t.Fatal("restore after in-place reschema is not exact")
	}

	// Masked copy: no configured sensitive value may appear anywhere in the target.
	key := []byte("integration-test-masking-key-123")
	m, err := masking.Compile([]masking.Rule{
		{Field: "fname", Method: masking.MethodRandomize},
		{Field: "email", Method: masking.MethodEmail},
		{Field: "ssn", Method: masking.MethodPartial, KeepLast: 2},
		{Field: "address.zip", Method: masking.MethodHash},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	mspec := transform.Spec{ConnID: e.conn, DB: e.db, Coll: "customers", Mode: transform.ModeCopy, TargetColl: "customers_masked"}
	rep, err = transform.Apply(e.ctx, e.mgr, mspec, m.Apply, "mask", bdir, nil)
	if err != nil || !rep.Valid || rep.Written != 501 {
		t.Fatalf("mask: %+v %v", rep, err)
	}
	masked := allDocs(t, e.ctx, coll(t, e, e.conn, e.db, "customers_masked"))
	var raw bytes.Buffer
	for _, d := range masked {
		b, _ := bson.Marshal(d)
		raw.Write(b)
	}
	for i := 0; i < 500; i += 37 {
		for _, s := range []string{fmt.Sprintf("Name%d\x00", i), fmt.Sprintf("user%d@", i), fmt.Sprintf("123-45-%04d", i), "75001"} {
			if bytes.Contains(raw.Bytes(), []byte(s)) {
				t.Fatalf("sensitive value %q present in masked copy", s)
			}
		}
	}
	if !bytes.Contains(raw.Bytes(), []byte("Paris")) {
		t.Fatal("unmasked field lost")
	}
}

func TestMigrationFromSQLite(t *testing.T) {
	e := setup(t)
	path := filepath.Join(t.TempDir(), "shop.db")
	seedSQLite(t, path)
	src := migrate.Source{Driver: migrate.SQLite, DSN: path}
	runMigration(t, e, src)
}

// TestMigrationFromPostgres and MySQL run when the compose services are up:
// PG_DSN=postgres://mango:mango@localhost:5433/shop MYSQL_DSN='mango:mango@tcp(localhost:3307)/shop'
func TestMigrationFromPostgres(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN not set")
	}
	runMigration(t, setup(t), migrate.Source{Driver: migrate.Postgres, DSN: dsn})
}

func TestMigrationFromMySQL(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set")
	}
	runMigration(t, setup(t), migrate.Source{Driver: migrate.MySQL, DSN: dsn})
}

func runMigration(t *testing.T, e env, src migrate.Source) {
	t.Helper()
	tables, err := migrate.Introspect(e.ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 3 {
		t.Fatalf("introspection: %+v", tables)
	}
	jm := jobs.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	svc := migrate.NewService(e.mgr, jm, t.TempDir())
	req := migrate.Request{
		Source: src, ConnID: e.conn, DB: e.db,
		Tables: []migrate.TableMapping{
			{Table: "customers", Collection: "customers"},
			{Table: "orders", Collection: "orders"},
			{Table: "order_lines", Collection: "order_lines"},
		},
		Embeds: []migrate.Embed{{Child: "orders", Parent: "customers", As: "orders"}},
	}
	j, err := svc.Start(req)
	if err != nil {
		t.Fatal(err)
	}
	done := runJob(t, jm, j)
	runs, _ := svc.Runs()
	if done.State != jobs.Succeeded || runs[0].State != "succeeded" || !runs[0].Report.Valid {
		t.Fatalf("migration: %+v %+v %+v", done, runs[0], runs[0].Report)
	}
	rep := runs[0].Report
	for _, tr := range rep.Tables {
		if !tr.CountOK || len(tr.SampleMismatches) > 0 || tr.SampleChecked == 0 {
			t.Fatalf("table %s: %+v", tr.Table, tr)
		}
	}
	if rep.Embeds[0].Embedded != 3 || rep.Embeds[0].Orphans != 1 || !rep.Embeds[0].CountOK {
		t.Fatalf("embed: %+v", rep.Embeds[0])
	}
	var ada bson.D
	if err := coll(t, e, e.conn, e.db, "customers").FindOne(e.ctx, bson.D{{Key: "name", Value: "Ada"}}).Decode(&ada); err != nil {
		t.Fatal(err)
	}
	get := func(k string) any { v, _ := mql.Lookup(ada, k); return v }
	if mql.Compact(get("balance")) != `NumberDecimal("1234.50")` && mql.Compact(get("balance")) != `NumberDecimal("1234.5")` {
		t.Errorf("decimal: %s", mql.Compact(get("balance")))
	}
	if mql.TypeName(get("joined")) != "date" || get("vip") != true || mql.TypeName(get("avatar")) != "binary" {
		t.Errorf("types: joined=%s vip=%v avatar=%s", mql.TypeName(get("joined")), get("vip"), mql.TypeName(get("avatar")))
	}
	// Key order may be normalized by the source (MySQL JSON), so compare content.
	prefs, ok := get("prefs").(bson.D)
	theme, _ := mql.Lookup(prefs, "theme")
	tags, _ := mql.Lookup(prefs, "tags")
	if !ok || theme != "dark" || mql.Compact(tags) != `["a", "b"]` {
		t.Errorf("json column: %s", mql.Compact(get("prefs")))
	}
	if orders, ok := get("orders").(bson.A); !ok || len(orders) != 2 {
		t.Errorf("embedded orders: %s", mql.Compact(get("orders")))
	}
	var line bson.D
	_ = coll(t, e, e.conn, e.db, "order_lines").FindOne(e.ctx, bson.D{{Key: "sku", Value: "MS-002"}}).Decode(&line)
	if id, _ := mql.Lookup(line, "_id"); mql.TypeName(id) != "document" {
		t.Errorf("composite key: %s", mql.Compact(line))
	}

	// Migrating again into the same (now non-empty) collections is refused.
	j2, _ := svc.Start(req)
	if st := runJob(t, jm, j2); st.State != jobs.Failed || !strings.Contains(st.Error, "already has") {
		t.Fatalf("re-run into non-empty target: %+v", st)
	}
	// Roll back the first run: its collections are gone.
	if _, err := svc.Rollback(e.ctx, runs[0].ID); err != nil {
		t.Fatal(err)
	}
	s, _ := e.mgr.Session(e.conn)
	names, _ := s.Client.Database(e.db).ListCollectionNames(e.ctx, bson.D{})
	for _, n := range names {
		if n == "customers" || n == "orders" || n == "order_lines" {
			t.Fatalf("rollback left %s", n)
		}
	}
}

// seedSQLite mirrors migrate.SeedSQLite (test helpers are not exported across packages).
func seedSQLite(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sqlText, err := os.ReadFile(filepath.Join("..", "..", "docker", "sql-init", "shop.sqlite.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(sqlText), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

func newApp(t *testing.T) (*api.App, string) {
	t.Helper()
	dir := t.TempDir()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := api.New(api.Deps{Log: log, Store: config.NewStore(dir), Secrets: secrets.NewMemoryStore(), Conns: mongox.NewManager(log)})
	return app, dir
}

func TestScheduledExportThroughAPI(t *testing.T) {
	e := setup(t)
	seed(t, e, "nums", 40)
	app, _ := newApp(t)
	p, _ := app.SaveProfile(api.ProfileInput{Name: "it", URI: mongoURI(), ConnectTimeoutSeconds: 5})
	out := t.TempDir()
	params, _ := json.Marshal(map[string]any{"connId": p.ID, "db": e.db, "coll": "nums", "path": filepath.Join(out, "nums-{date}.json"), "format": "json", "canonical": true})
	task, err := app.SaveTask(scheduler.Task{Name: "nightly", Type: "export", Params: params, Schedule: scheduler.Schedule{Kind: scheduler.KindCron, Cron: "0 2 * * *"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.RunTaskNow(task.ID); err == nil {
		t.Fatal("unapproved task ran")
	}
	if _, err := app.ApproveTask(task.ID); err != nil {
		t.Fatal(err)
	}
	// The app is not connected yet: the task connects on its own with stored credentials.
	if _, err := app.RunTaskNow(task.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := app.ListTaskRuns(task.ID, 1)
		if len(runs) == 1 && runs[0].State != scheduler.RunRunning {
			if runs[0].State != scheduler.RunSucceeded {
				t.Fatalf("run: %+v", runs[0])
			}
			files, _ := filepath.Glob(filepath.Join(out, "nums-*.json"))
			if len(files) != 1 {
				t.Fatalf("export file: %v", files)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("task did not finish")
}

type fakeProvider struct{ answer string }

func (f fakeProvider) Generate(ctx context.Context, system, user string, schema map[string]any) (string, ai.Usage, error) {
	if strings.Contains(user, "mongodb://") || strings.Contains(user, "password") {
		return "", ai.Usage{}, fmt.Errorf("connection details leaked into the prompt")
	}
	return f.answer, ai.Usage{Model: "fake", InputTokens: 10, OutputTokens: 5}, nil
}

func TestAIProposalsValidatedAndGated(t *testing.T) {
	e := setup(t)
	seed(t, e, "nums", 20)
	app, _ := newApp(t)
	p, _ := app.SaveProfile(api.ProfileInput{Name: "it", URI: mongoURI(), ConnectTimeoutSeconds: 5})
	if _, err := app.Connect(p.ID); err != nil {
		t.Fatal(err)
	}
	st := app.GetSettings()
	req := ai.Request{ConnID: p.ID, DB: e.db, Coll: "nums", Prompt: "numbers above 10"}
	api.SetAIProvider(app, fakeProvider{answer: `{"kind":"find","filter":"{ n: { $gt: 10 } }","projection":"","sort":"{ n: 1 }","limit":5,"pipeline":[],"update":"","explanation":"n > 10","warnings":[]}`})
	if _, err := app.GenerateAI("ai", req); err == nil {
		t.Fatal("AI ran while disabled")
	}
	st.AIEnabled = true
	_, _ = app.SaveSettings(st)
	prop, err := app.GenerateAI("ai", req)
	if err != nil || !prop.Valid || prop.IsWrite || prop.Validation[len(prop.Validation)-1].Name != "Server validation" {
		t.Fatalf("read proposal: %+v %v", prop, err)
	}
	if len(prop.Sent.Samples) != 0 {
		t.Fatal("samples sent without being allowed")
	}

	// The server catches operators the parser cannot know about.
	api.SetAIProvider(app, fakeProvider{answer: `{"kind":"find","filter":"{ n: { $gtx: 10 } }","projection":"","sort":"","limit":0,"pipeline":[],"update":"","explanation":"","warnings":[]}`})
	bad, err := app.GenerateAI("ai", req)
	if err != nil || bad.Valid {
		t.Fatalf("invalid operator passed validation: %+v", bad)
	}

	// Writes: blocked unless allowed; then preview, confirm, backup, undo.
	api.SetAIProvider(app, fakeProvider{answer: `{"kind":"updateMany","filter":"{ n: { $lt: 5 } }","projection":"","sort":"","limit":0,"pipeline":[],"update":"{ $set: { small: true } }","explanation":"flag","warnings":[]}`})
	w, _ := app.GenerateAI("ai", req)
	if w.Valid {
		t.Fatal("write proposal valid without allowWrites")
	}
	req.AllowWrites = true
	w, _ = app.GenerateAI("ai", req)
	if !w.Valid || !w.RequiresConfirmation {
		t.Fatalf("write proposal: %+v", w)
	}
	wp, err := app.PreviewAIWrite(w.ID)
	if err != nil || wp.Affected != 5 {
		t.Fatalf("write preview: %+v %v", wp, err)
	}
	if _, err := app.ExecuteAIWrite(w.ID, "nope"); err == nil {
		t.Fatal("write executed without confirmation")
	}
	res, err := app.ExecuteAIWrite(w.ID, wp.ConfirmText)
	if err != nil || res.Modified != 5 || res.BackupPath == "" {
		t.Fatalf("execute: %+v %v", res, err)
	}
	if _, err := app.ExecuteAIWrite(w.ID, wp.ConfirmText); err == nil {
		t.Fatal("write proposal executed twice")
	}
	c := coll(t, e, e.conn, e.db, "nums")
	if n, _ := c.CountDocuments(e.ctx, bson.D{{Key: "small", Value: true}}); n != 5 {
		t.Fatalf("modified docs: %d", n)
	}
	if _, err := backup.Restore(e.ctx, c, res.BackupPath); err != nil {
		t.Fatal(err)
	}
	if n, _ := c.CountDocuments(e.ctx, bson.D{{Key: "small", Value: true}}); n != 0 {
		t.Fatal("restore did not undo the AI write")
	}
}

func TestMCPAgainstRealServer(t *testing.T) {
	e := setup(t)
	people := coll(t, e, e.conn, e.db, "people")
	_, _ = people.InsertMany(e.ctx, []any{
		bson.D{{Key: "_id", Value: int32(1)}, {Key: "email", Value: "ada@example.com"}, {Key: "age", Value: int32(36)}},
		bson.D{{Key: "_id", Value: int32(2)}, {Key: "email", Value: "grace@example.com"}, {Key: "age", Value: int32(85)}},
	})
	app, _ := newApp(t)
	ro, _ := app.SaveProfile(api.ProfileInput{Name: "ro", URI: mongoURI(), ConnectTimeoutSeconds: 5, ReadOnly: true})
	rw, _ := app.SaveProfile(api.ProfileInput{Name: "rw", URI: mongoURI(), ConnectTimeoutSeconds: 5})
	rs, err := app.SaveMaskRuleSet(masking.RuleSet{Name: "PII", Rules: []masking.Rule{{Field: "email", Method: masking.MethodEmail}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveMCPPolicy(mcpserver.Policy{Enabled: true, Profiles: []string{ro.ID, rw.ID}, Databases: []string{e.db},
		Tools: []string{mcpserver.ToolFind, mcpserver.ToolAggregate, mcpserver.ToolDeleteMany}, AllowWrites: true, MaxDocs: 1, MaskRuleSetID: rs.ID}); err != nil {
		t.Fatal(err)
	}
	srv, err := api.NewMCPServer(app)
	if err != nil {
		t.Fatal(err)
	}
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.MCP().Connect(e.ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "it", Version: "1"}, nil).Connect(e.ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	callTool := func(name string, args map[string]any) (string, bool) {
		res, err := cs.CallTool(e.ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			return err.Error(), true
		}
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String(), res.IsError
	}
	base := map[string]any{"connection": "ro", "database": e.db, "collection": "people"}
	text, isErr := callTool(mcpserver.ToolFind, base)
	if isErr || strings.Contains(text, "ada@example.com") || !strings.Contains(text, "@example.com") || strings.Count(text, "_id") != 1 {
		t.Fatalf("find must be masked and capped at 1: %s", text)
	}
	agg := map[string]any{"connection": "ro", "database": e.db, "collection": "people", "pipeline": `[{ $match: {} }, { $out: "stolen" }]`}
	if text, isErr := callTool(mcpserver.ToolAggregate, agg); !isErr || !strings.Contains(text, "$out") {
		t.Fatalf("$out must be refused: %s", text)
	}
	del := map[string]any{"connection": "ro", "database": e.db, "collection": "people", "filter": "{ age: { $gt: 50 } }"}
	if text, isErr := callTool(mcpserver.ToolDeleteMany, del); !isErr || !strings.Contains(text, "read-only") {
		t.Fatalf("write through a read-only profile: %s", text)
	}
	del["connection"] = "rw"
	del["filter"] = "{}"
	if text, isErr := callTool(mcpserver.ToolDeleteMany, del); !isErr || !strings.Contains(text, "empty filter") {
		t.Fatalf("empty-filter delete: %s", text)
	}
	del["filter"] = "{ age: { $gt: 50 } }"
	if text, isErr := callTool(mcpserver.ToolDeleteMany, del); isErr || !strings.Contains(text, `"deleted": 1`) {
		t.Fatalf("allowed delete: %s", text)
	}
	backups, _ := app.ListBackups()
	if len(backups) != 1 || backups[0].Deleted != 1 {
		t.Fatalf("MCP delete must be backed up: %+v", backups)
	}
}
