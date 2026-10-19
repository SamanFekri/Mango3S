//go:build integration

package manage

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func client(t *testing.T) *mongo.Client {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	cl, err := mongo.Connect(options.Client().ApplyURI(uri).SetDirect(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Disconnect(context.Background()) })
	return cl
}

type tlog struct{ t *testing.T }

func (l tlog) Log(f string, a ...any) { l.t.Logf(f, a...) }

func TestManageAgainstRealServer(t *testing.T) {
	ctx := context.Background()
	cl := client(t)
	dbn := "mango3s_it_manage"
	db := cl.Database(dbn)
	db.Drop(ctx)
	t.Cleanup(func() {
		cl.Database(dbn).Drop(context.Background())
		cl.Database(dbn + "_renamed").Drop(context.Background())
	})

	// Create with collation, validator and capped options.
	err := CreateCollection(ctx, cl, dbn, CreateOptions{Name: "people", Collation: `{ locale: "en", strength: 2 }`,
		Validator: `{ $jsonSchema: { required: ["name"] } }`, ValidationAction: "error", ValidationLevel: "strict"})
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateCollection(ctx, cl, dbn, CreateOptions{Name: "log", Capped: true, SizeBytes: 1 << 20, MaxDocs: 100}); err != nil {
		t.Fatal(err)
	}
	if err := CreateCollection(ctx, cl, dbn, CreateOptions{Name: "metrics", TimeField: "ts", MetaField: "host", Granularity: "minutes", ExpireAfter: 86400}); err != nil {
		t.Fatal(err)
	}
	if err := CreateCollection(ctx, cl, dbn, CreateOptions{Name: "bad", Capped: true}); err == nil {
		t.Fatal("capped without size accepted")
	}
	if _, err := db.Collection("people").InsertOne(ctx, bson.D{{Key: "x", Value: 1}}); err == nil {
		t.Fatal("validator not applied")
	}
	db.Collection("people").InsertMany(ctx, []any{bson.D{{Key: "_id", Value: 1}, {Key: "name", Value: "Ann"}, {Key: "age", Value: 30}}, bson.D{{Key: "_id", Value: 2}, {Key: "name", Value: "Bob"}}})

	// Validation: read, check a stricter rule against existing data, change it.
	v, err := GetValidation(ctx, db, "people")
	if err != nil || !strings.Contains(v.Validator, "$jsonSchema") || v.Level != "strict" {
		t.Fatalf("%+v %v", v, err)
	}
	rep, err := CheckValidator(ctx, db.Collection("people"), `{ age: { $exists: true } }`)
	if err != nil || rep.Checked != 2 || rep.Failing != 1 || len(rep.Sample) != 1 || rep.Sample[0] != "2" {
		t.Fatalf("%+v %v", rep, err)
	}
	if err := SetValidation(ctx, db, "people", Validation{Validator: `{ age: { $exists: true } }`, Level: "moderate", Action: "warn"}); err != nil {
		t.Fatal(err)
	}
	v, _ = GetValidation(ctx, db, "people")
	if v.Level != "moderate" || v.Action != "warn" || !strings.Contains(v.Validator, "age") {
		t.Fatalf("%+v", v)
	}
	if err := SetValidation(ctx, db, "people", Validation{Level: "strict", Action: "error"}); err != nil {
		t.Fatal(err)
	}
	if v, _ = GetValidation(ctx, db, "people"); v.Validator != "" {
		t.Fatalf("validator not removed: %+v", v)
	}

	// Indexes: create (unique+partial, TTL), hide, change TTL, drop.
	ttl := int64(3600)
	name, err := CreateIndex(ctx, db, "people", IndexOptions{Keys: `{ name: 1, age: -1 }`, Unique: true, Partial: `{ age: { $gt: 0 } }`})
	if err != nil || name != "name_1_age_-1" {
		t.Fatal(name, err)
	}
	if _, err := CreateIndex(ctx, db, "people", IndexOptions{Keys: `{ createdAt: 1 }`, Name: "exp", ExpireAfter: &ttl, Collation: `{ locale: "fr" }`}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateIndex(ctx, db, "people", IndexOptions{Keys: `{ a: 1, b: 1 }`, ExpireAfter: &ttl}); err == nil {
		t.Fatal("compound TTL accepted")
	}
	hidden := true
	if err := ModifyIndex(ctx, db, "people", "exp", &hidden, nil); err != nil {
		t.Fatal(err)
	}
	ttl2 := int64(60)
	if err := ModifyIndex(ctx, db, "people", "exp", nil, &ttl2); err != nil {
		t.Fatal(err)
	}
	idx, err := ListIndexes(ctx, db, "people")
	if err != nil || len(idx) != 3 || idx[0].Name != "_id_" {
		t.Fatalf("%+v %v", idx, err)
	}
	for _, ix := range idx {
		switch ix.Name {
		case "exp":
			if !ix.Hidden || ix.TTL == nil || *ix.TTL != 60 || ix.Collation == "" {
				t.Fatalf("exp: %+v", ix)
			}
		case "name_1_age_-1":
			if !ix.Unique || ix.Partial == "" || ix.Size == 0 {
				t.Fatalf("compound: %+v", ix)
			}
		}
	}
	if err := DropIndex(ctx, db, "people", "_id_"); err == nil {
		t.Fatal("_id_ dropped")
	}
	if err := DropIndex(ctx, db, "people", "exp"); err != nil {
		t.Fatal(err)
	}

	// Details and stats.
	d, err := Details(ctx, db, "log")
	if err != nil || !d.Capped || !strings.Contains(d.Options, "capped") || !strings.Contains(d.Shell, "createCollection") {
		t.Fatalf("%+v %v", d, err)
	}
	if d, err = Details(ctx, db, "people"); err != nil || d.Count != 2 || d.UUID == "" {
		t.Fatalf("%+v %v", d, err)
	}
	if dd, err := DBDetails(ctx, db); err != nil || dd.Collections < 3 {
		t.Fatalf("%+v %v", dd, err)
	}

	// Views, rename, clear.
	if err := CreateView(ctx, cl, dbn, "adults", "people", `[ { $match: { age: { $gte: 18 } } } ]`, ""); err != nil {
		t.Fatal(err)
	}
	if err := CreateView(ctx, cl, dbn, "teens", "adults", `[ { $match: { age: { $lt: 20 } } } ]`, ""); err != nil {
		t.Fatal(err)
	}
	if err := RenameCollection(ctx, cl, dbn, "log", "", "events", false); err != nil {
		t.Fatal(err)
	}
	if err := RenameCollection(ctx, cl, dbn, "events", "", "people", false); err == nil {
		t.Fatal("rename onto existing collection without dropTarget")
	}
	n, err := ClearCollection(ctx, db.Collection("people"))
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if idx, _ := ListIndexes(ctx, db, "people"); len(idx) != 2 {
		t.Fatalf("clear dropped indexes: %+v", idx)
	}

	// Time series collections can't be renamed: refused before anything moves.
	if err := RenameDatabase(ctx, cl, dbn, dbn+"_renamed", tlog{t}); err == nil || !strings.Contains(err.Error(), "time series") {
		t.Fatalf("time series rename: %v", err)
	}
	if names, _ := cl.Database(dbn+"_renamed").ListCollectionNames(ctx, bson.D{}); len(names) != 0 {
		t.Fatalf("partially moved: %v", names)
	}
	db.Collection("metrics").Drop(ctx)

	// Rename the database: collections, indexes and (dependent) views move.
	if err := RenameDatabase(ctx, cl, dbn, dbn+"_renamed", tlog{t}); err != nil {
		t.Fatal(err)
	}
	names, _ := cl.Database(dbn+"_renamed").ListCollectionNames(ctx, bson.D{{Key: "name", Value: bson.D{{Key: "$not", Value: bson.Regex{Pattern: "^system\\."}}}}})
	if len(names) != 4 {
		t.Fatalf("renamed db has %v", names)
	}
	if left, _ := cl.Database(dbn).ListCollectionNames(ctx, bson.D{}); len(left) != 0 {
		t.Fatalf("old db still has %v", left)
	}
	if idx, _ := ListIndexes(ctx, cl.Database(dbn+"_renamed"), "people"); len(idx) != 2 {
		t.Fatalf("indexes lost in rename: %+v", idx)
	}
	if err := RenameDatabase(ctx, cl, dbn+"_renamed", dbn+"_renamed", tlog{t}); err == nil {
		t.Fatal("rename to itself")
	}
}

func TestProfiler(t *testing.T) {
	ctx := context.Background()
	cl := client(t)
	dbn := "mango3s_it_profile"
	db := cl.Database(dbn)
	db.Drop(ctx)
	t.Cleanup(func() {
		SetProfiling(context.Background(), cl.Database(dbn), Profiling{Level: 0, SlowMS: 100})
		cl.Database(dbn).Drop(context.Background())
	})
	db.Collection("c").InsertOne(ctx, bson.D{{Key: "a", Value: 1}})
	if err := SetProfiling(ctx, db, Profiling{Level: 2, SlowMS: 100}); err != nil {
		t.Fatal(err)
	}
	p, err := GetProfiling(ctx, db)
	if err != nil || p.Level != 2 {
		t.Fatalf("%+v %v", p, err)
	}
	for i := 0; i < 3; i++ {
		db.Collection("c").Find(ctx, bson.D{{Key: "a", Value: i}})
	}
	entries, err := ListProfile(ctx, db, ProfileQuery{Coll: "c", Op: "query"})
	if err != nil || len(entries) < 3 || entries[0].NS != dbn+".c" || !strings.Contains(entries[0].Command, "find") || entries[0].PlanSummary != "COLLSCAN" {
		t.Fatalf("%+v %v", entries, err)
	}
	if err := SetProfiling(ctx, db, Profiling{Level: 3}); err == nil {
		t.Fatal("level 3 accepted")
	}
	if err := ClearProfile(ctx, db); err != nil {
		t.Fatal(err)
	}
	if p, _ := GetProfiling(ctx, db); p.Level != 2 {
		t.Fatalf("level not restored: %+v", p)
	}
	SetProfiling(ctx, db, Profiling{Level: 0})
	if entries, _ := ListProfile(ctx, db, ProfileQuery{Coll: "c", Op: "query"}); len(entries) != 0 {
		t.Fatalf("not cleared: %d", len(entries))
	}
}
