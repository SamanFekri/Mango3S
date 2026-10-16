//go:build integration

package integration

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/documents"
	"mango3s/internal/explorer"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// rawDoc reads a document straight from the server, bypassing Mango3S.
func rawDoc(t *testing.T, e env, coll string, id any) bson.D {
	t.Helper()
	s, _ := e.mgr.Session(e.conn)
	var d bson.D
	if err := s.Client.Database(e.db).Collection(coll).FindOne(e.ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFieldLevelEditing(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	c := s.Client.Database(e.db).Collection("cells")
	oid := bson.NewObjectID()
	orig := bson.D{
		{Key: "_id", Value: oid},
		{Key: "name", Value: "Ada"},
		{Key: "age", Value: int32(36)},
		{Key: "big", Value: int64(9007199254740993)},
		{Key: "score", Value: 4.0},
		{Key: "joined", Value: bson.NewDateTimeFromTime(mustTime("2020-01-02T03:04:05Z"))},
		{Key: "address", Value: bson.D{{Key: "city", Value: "London"}, {Key: "zip", Value: "N1"}}},
		{Key: "tags", Value: bson.A{"a", bson.D{{Key: "k", Value: int32(1)}}}},
	}
	if _, err := c.InsertOne(e.ctx, orig); err != nil {
		t.Fatal(err)
	}
	docs := documents.New(e.mgr)
	id := mql.Compact(oid)

	edit := func(path []string, text string) (documents.FieldResult, error) {
		fv, err := docs.GetField(e.ctx, e.conn, e.db, "cells", id, path)
		if err != nil {
			t.Fatal(err)
		}
		return docs.UpdateField(e.ctx, documents.FieldUpdate{ConnID: e.conn, DB: e.db, Coll: "cells", ID: id, Path: path,
			Type: fv.Type, Text: text, Expected: fv.Expected, ExpectedMissing: !fv.Exists})
	}
	unchangedExcept := func(changed string) {
		t.Helper()
		got := rawDoc(t, e, "cells", oid)
		for _, f := range orig {
			if f.Key == changed {
				continue
			}
			v, _ := mql.Lookup(got, f.Key)
			if !mql.Equal(v, f.Value) {
				t.Fatalf("unrelated field %s changed: %s → %s", f.Key, mql.Compact(f.Value), mql.Compact(v))
			}
		}
	}

	// Types are preserved: the int32 stays an int32, the int64 keeps all digits.
	if _, err := edit([]string{"age"}, "37"); err != nil {
		t.Fatal(err)
	}
	if v, _ := mql.Lookup(rawDoc(t, e, "cells", oid), "age"); v != int32(37) {
		t.Fatalf("age = %#v", v)
	}
	unchangedExcept("age")
	orig[2].Value = int32(37)

	if _, err := edit([]string{"big"}, "9007199254740995"); err != nil {
		t.Fatal(err)
	}
	if v, _ := mql.Lookup(rawDoc(t, e, "cells", oid), "big"); v != int64(9007199254740995) {
		t.Fatalf("big = %#v", v)
	}
	orig[3].Value = int64(9007199254740995)

	if _, err := edit([]string{"score"}, "5"); err != nil {
		t.Fatal(err)
	}
	if v, _ := mql.Lookup(rawDoc(t, e, "cells", oid), "score"); v != 5.0 {
		t.Fatalf("double must stay double: %#v", v)
	}
	orig[4].Value = 5.0

	// Nested document field and array element, by path.
	res, err := edit([]string{"address", "city"}, "Paris")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Document.Text, `"city": "Paris"`) || !strings.Contains(res.Document.Text, `"zip": "N1"`) || res.Previous != `"London"` {
		t.Fatalf("nested edit result: %s / prev %s", res.Document.Text, res.Previous)
	}
	orig[6].Value = bson.D{{Key: "city", Value: "Paris"}, {Key: "zip", Value: "N1"}}
	unchangedExcept("")

	if _, err := edit([]string{"tags", "1", "k"}, "2"); err != nil {
		t.Fatal(err)
	}
	tags, _ := mql.Lookup(rawDoc(t, e, "cells", oid), "tags")
	if mql.Compact(tags) != `["a",{k:2}]` && !mql.Equal(tags, bson.A{"a", bson.D{{Key: "k", Value: int32(2)}}}) {
		t.Fatalf("array element edit: %s", mql.Compact(tags))
	}
	orig[7].Value = tags

	// Invalid input is rejected before anything is written.
	if _, err := edit([]string{"age"}, "old"); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("invalid int: %v", err)
	}
	if _, err := edit([]string{"joined"}, "someday"); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("invalid date: %v", err)
	}
	unchangedExcept("")

	// A concurrent change makes the edit fail instead of overwriting it.
	fv, _ := docs.GetField(e.ctx, e.conn, e.db, "cells", id, []string{"name"})
	if _, err := c.UpdateOne(e.ctx, bson.D{{Key: "_id", Value: oid}}, bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: "Grace"}}}}); err != nil {
		t.Fatal(err)
	}
	_, err = docs.UpdateField(e.ctx, documents.FieldUpdate{ConnID: e.conn, DB: e.db, Coll: "cells", ID: id, Path: []string{"name"}, Type: fv.Type, Text: "Ada L.", Expected: fv.Expected})
	if !apperr.Is(err, apperr.CodeConflict) {
		t.Fatalf("concurrent change must conflict: %v", err)
	}
	if v, _ := mql.Lookup(rawDoc(t, e, "cells", oid), "name"); v != "Grace" {
		t.Fatalf("the other writer's value was overwritten: %v", v)
	}
	orig[1].Value = "Grace"

	// A new field on a document that lacks it, and $unset.
	if _, err := edit([]string{"nickname"}, `"Gracie"`); err != nil {
		t.Fatal(err)
	}
	fv, _ = docs.GetField(e.ctx, e.conn, e.db, "cells", id, []string{"nickname"})
	if _, err := docs.UpdateField(e.ctx, documents.FieldUpdate{ConnID: e.conn, DB: e.db, Coll: "cells", ID: id, Path: []string{"nickname"}, Expected: fv.Expected, Unset: true}); err != nil {
		t.Fatal(err)
	}
	if _, ok := mql.Lookup(rawDoc(t, e, "cells", oid), "nickname"); ok {
		t.Fatal("unset failed")
	}
	unchangedExcept("")

	// _id and read-only connections.
	if _, err := edit([]string{"_id"}, oid.Hex()); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("_id edit: %v", err)
	}
	ro := mongox.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
	p := config.Profile{ID: "ro", Name: "ro", URI: uriFromEnv(), ReadOnly: true, ConnectTimeoutSeconds: 5}
	if _, err := ro.Connect(e.ctx, p, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
	defer ro.CloseAll(e.ctx)
	_, err = documents.New(ro).UpdateField(e.ctx, documents.FieldUpdate{ConnID: "ro", DB: e.db, Coll: "cells", ID: id, Path: []string{"age"}, Type: "int32", Text: "1", Expected: "37"})
	if !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("read-only: %v", err)
	}
}

func TestSystemCollectionsAreListedSeparately(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	db := s.Client.Database(e.db)
	if err := db.CreateCollection(e.ctx, "items"); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateView(e.ctx, "items_view", "items", bson.A{}); err != nil {
		t.Fatal(err)
	}
	list, err := explorer.New(e.mgr).ListCollections(e.ctx, e.conn, e.db)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]explorer.CollectionInfo{}
	for _, c := range list {
		kinds[c.Name] = c
	}
	if c := kinds["system.views"]; !c.System {
		t.Fatalf("system.views must be listed and flagged: %+v", list)
	}
	if c := kinds["items_view"]; c.Type != "view" || c.System {
		t.Fatalf("view: %+v", c)
	}
	if c := kinds["items"]; c.Type != "collection" || c.System {
		t.Fatalf("collection: %+v", c)
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func uriFromEnv() string {
	if u := os.Getenv("MONGO_URI"); u != "" {
		return u
	}
	return "mongodb://localhost:27017"
}
