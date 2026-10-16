//go:build integration

package integration

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/api"
	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/config"
	"mango3s/internal/documents"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/query"
	"mango3s/internal/secrets"
)

func TestRenameAndAppendField(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	c := s.Client.Database(e.db).Collection("ren")
	if _, err := c.InsertOne(e.ctx, bson.D{{Key: "_id", Value: 1}, {Key: "nm", Value: "Ada"}, {Key: "age", Value: int32(36)},
		{Key: "addr", Value: bson.D{{Key: "cty", Value: "London"}}}, {Key: "tags", Value: bson.A{"a", bson.D{{Key: "k", Value: 1}}}}}); err != nil {
		t.Fatal(err)
	}
	docs := documents.New(e.mgr)
	upd := func(path []string, f func(u *documents.FieldUpdate)) (documents.FieldResult, error) {
		fv, err := docs.GetField(e.ctx, e.conn, e.db, "ren", "1", path)
		if err != nil {
			t.Fatal(err)
		}
		u := documents.FieldUpdate{ConnID: e.conn, DB: e.db, Coll: "ren", ID: "1", Path: path, Type: fv.Type, Expected: fv.Expected, ExpectedMissing: !fv.Exists}
		f(&u)
		return docs.UpdateField(e.ctx, u)
	}
	// Rename keeps the value and its type; other fields are untouched.
	if _, err := upd([]string{"nm"}, func(u *documents.FieldUpdate) { u.RenameTo = "name" }); err != nil {
		t.Fatal(err)
	}
	if _, err := upd([]string{"addr", "cty"}, func(u *documents.FieldUpdate) { u.RenameTo = "city" }); err != nil {
		t.Fatal(err)
	}
	got := rawDoc(t, e, "ren", 1)
	if v, _ := mql.Lookup(got, "name"); v != "Ada" {
		t.Fatalf("renamed field: %s", mql.Compact(got))
	}
	if _, ok := mql.Lookup(got, "nm"); ok {
		t.Fatal("old name still there")
	}
	if a, _ := documents.Lookup(got, []string{"addr", "city"}); a != "London" {
		t.Fatalf("nested rename: %s", mql.Compact(got))
	}
	if v, _ := mql.Lookup(got, "age"); v != int32(36) {
		t.Fatal("unrelated field changed")
	}
	// Refused: overwriting an existing field, renaming inside arrays.
	if _, err := upd([]string{"name"}, func(u *documents.FieldUpdate) { u.RenameTo = "age" }); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("rename onto an existing field: %v", err)
	}
	if _, err := upd([]string{"tags", "1", "k"}, func(u *documents.FieldUpdate) { u.RenameTo = "kk" }); err == nil || !strings.Contains(err.Error(), "arrays") {
		t.Fatalf("rename inside an array: %v", err)
	}
	// Append to an array with an explicit type.
	if _, err := upd([]string{"tags"}, func(u *documents.FieldUpdate) { u.Append, u.Type, u.Text = true, "int64", "7" }); err != nil {
		t.Fatal(err)
	}
	tags, _ := mql.Lookup(rawDoc(t, e, "ren", 1), "tags")
	if arr := tags.(bson.A); len(arr) != 3 || arr[2] != int64(7) {
		t.Fatalf("append: %s", mql.Compact(tags))
	}
	if _, err := upd([]string{"age"}, func(u *documents.FieldUpdate) { u.Append, u.Type, u.Text = true, "int32", "1" }); err == nil {
		t.Fatal("append to a non-array must fail")
	}
	// Add a new null field with an explicit type.
	if _, err := upd([]string{"note"}, func(u *documents.FieldUpdate) { u.Type, u.Text = "null", "" }); err != nil {
		t.Fatal(err)
	}
	if v, ok := mql.Lookup(rawDoc(t, e, "ren", 1), "note"); !ok || v != nil {
		t.Fatalf("null field: %v %v", v, ok)
	}
}

func TestUpdateAndRemoveDialogs(t *testing.T) {
	uri := uriFromEnv()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
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
	if _, err := app.InsertDocuments(p.ID, db, "o", `[{_id: 1, s: "A", n: 1}, {_id: 2, s: "A", n: 2}, {_id: 3, s: "B", n: 3}, {_id: 4, s: "B", n: 4}]`); err != nil {
		t.Fatal(err)
	}
	up := api.BulkWriteRequest{ConnID: p.ID, DB: db, Coll: "o", Kind: "update", Scope: "query", Filter: `{ s: "A" }`, Update: `{ $set: { s: "done" }, $inc: { n: NumberLong(10) } }`}
	prev, err := app.PreviewBulkWrite(up)
	if err != nil || prev.Affected != 2 || prev.ConfirmText != db+".o" || !strings.Contains(prev.Shell, "updateMany") {
		t.Fatalf("preview: %+v %v", prev, err)
	}
	if _, err := app.ExecuteBulkWrite(up, "wrong", 2); err == nil {
		t.Fatal("the target must be typed to confirm")
	}
	if _, err := app.ExecuteBulkWrite(up, db+".o", 3); err == nil || !strings.Contains(err.Error(), "Preview again") {
		t.Fatalf("a changed count must refuse to run: %v", err)
	}
	res, err := app.ExecuteBulkWrite(up, db+".o", 2)
	if err != nil || res.Modified != 2 || res.BackupPath == "" {
		t.Fatalf("update: %+v %v", res, err)
	}
	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Fatal("backup missing")
	}
	// Replacement documents and empty updates are refused.
	bad := up
	bad.Update = `{ s: "x" }`
	if _, err := app.PreviewBulkWrite(bad); err == nil || !strings.Contains(err.Error(), "operators") {
		t.Fatalf("replacement document: %v", err)
	}
	// Remove only the selected documents.
	rm := api.BulkWriteRequest{ConnID: p.ID, DB: db, Coll: "o", Kind: "delete", Scope: "selected", IDs: []string{"3", "4"}}
	prev, err = app.PreviewBulkWrite(rm)
	if err != nil || prev.Affected != 2 {
		t.Fatalf("remove preview: %+v %v", prev, err)
	}
	res, err = app.ExecuteBulkWrite(rm, db+".o", 2)
	if err != nil || res.Deleted != 2 {
		t.Fatalf("remove: %+v %v", res, err)
	}
	page, _ := app.CountQuery("c", queryReq(p.ID, db, "o"))
	if page != 2 {
		t.Fatalf("2 documents should remain, got %d", page)
	}
	// The backup restores what the remove dialog deleted.
	infos, _ := backup.List(backup.Dir(dir))
	if len(infos) != 2 {
		t.Fatalf("backups: %d", len(infos))
	}
}

func queryReq(conn, db, coll string) query.Request {
	return query.Request{ConnID: conn, DB: db, Coll: coll, Filter: "{}"}
}
