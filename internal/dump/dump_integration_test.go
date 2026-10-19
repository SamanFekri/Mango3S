//go:build integration

package dump

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/config"
	"mango3s/internal/mongox"
)

func client(t *testing.T) (*mongo.Client, string) {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	cl, err := mongo.Connect(options.Client().ApplyURI(uri).SetDirect(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Disconnect(context.Background()) })
	return cl, uri
}

// seed creates a database with every interesting BSON type, a validator,
// a collation, indexes (unique, partial, TTL) and a view.
func seed(t *testing.T, ctx context.Context, cl *mongo.Client, db string) {
	t.Helper()
	d := cl.Database(db)
	d.Drop(ctx)
	t.Cleanup(func() { cl.Database(db).Drop(context.Background()) })
	err := d.CreateCollection(ctx, "people", options.CreateCollection().
		SetValidator(bson.D{{Key: "$jsonSchema", Value: bson.D{{Key: "required", Value: bson.A{"name"}}}}}).
		SetCollation(&options.Collation{Locale: "en", Strength: 2}))
	if err != nil {
		t.Fatal(err)
	}
	oid := bson.NewObjectID()
	docs := []any{
		bson.D{{Key: "_id", Value: oid}, {Key: "name", Value: "Ann"}, {Key: "age", Value: int32(41)}, {Key: "big", Value: int64(1 << 40)},
			{Key: "price", Value: mustDec(t, "12.50")}, {Key: "when", Value: bson.DateTime(1700000000000)},
			{Key: "bin", Value: bson.Binary{Subtype: 4, Data: bytes.Repeat([]byte{7}, 16)}}, {Key: "re", Value: bson.Regex{Pattern: "^a", Options: "i"}},
			{Key: "ts", Value: bson.Timestamp{T: 1, I: 2}}, {Key: "nested", Value: bson.D{{Key: "arr", Value: bson.A{1.5, nil, true}}}}},
	}
	for i := 0; i < 2500; i++ {
		docs = append(docs, bson.D{{Key: "_id", Value: int32(i)}, {Key: "name", Value: "p"}, {Key: "email", Value: strings.Repeat("x", i%7)}, {Key: "exp", Value: bson.DateTime(1)}})
	}
	if _, err := d.Collection("people").InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	_, err = d.Collection("people").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}, {Key: "age", Value: -1}}, Options: options.Index().SetName("name_age")},
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.D{{Key: "email", Value: bson.D{{Key: "$type", Value: "string"}}}}).SetName("email_partial")},
		{Keys: bson.D{{Key: "exp", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(3600 * 24 * 365 * 50)},
	})
	// email_partial is unique on strings of length 0–6 repeated, so it would
	// fail; recreate it as non-unique when that happens.
	if err != nil {
		_, err = d.Collection("people").Indexes().CreateMany(ctx, []mongo.IndexModel{
			{Keys: bson.D{{Key: "name", Value: 1}, {Key: "age", Value: -1}}, Options: options.Index().SetName("name_age")},
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetPartialFilterExpression(bson.D{{Key: "email", Value: bson.D{{Key: "$type", Value: "string"}}}}).SetName("email_partial")},
			{Keys: bson.D{{Key: "exp", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(3600 * 24 * 365 * 50)},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := d.CreateView(ctx, "adults", "people", bson.A{bson.D{{Key: "$match", Value: bson.D{{Key: "age", Value: bson.D{{Key: "$gte", Value: 18}}}}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Collection("weird/name%").InsertOne(ctx, bson.D{{Key: "x", Value: 1}}); err != nil {
		t.Fatal(err)
	}
}

func mustDec(t *testing.T, s string) bson.Decimal128 {
	d, err := bson.ParseDecimal128(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type tlog struct{ t *testing.T }

func (l tlog) Log(f string, a ...any) { l.t.Logf(f, a...) }
func (l tlog) SetTotal(int64, string) {}
func (l tlog) Add(int64)              {}

func rawDocs(t *testing.T, ctx context.Context, c *mongo.Collection) []bson.Raw {
	cur, err := c.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		t.Fatal(err)
	}
	var out []bson.Raw
	for cur.Next(ctx) {
		out = append(out, append(bson.Raw(nil), cur.Current...))
	}
	return out
}

func indexNames(t *testing.T, ctx context.Context, c *mongo.Collection) map[string]bson.D {
	cur, err := c.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var specs []bson.D
	cur.All(ctx, &specs)
	out := map[string]bson.D{}
	for _, s := range specs {
		out[lookup(s, "name").(string)] = s
	}
	return out
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	for _, gz := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "gzip"}[gz], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cl, _ := client(t)
			src, dst := "mango3s_it_dump_src", "mango3s_it_dump_dst"
			seed(t, ctx, cl, src)
			t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
			dir := filepath.Join(t.TempDir(), "bk")
			m, err := Run(ctx, cl, Options{Dir: dir, Gzip: gz, Selection: Selection{Databases: []string{src}, Views: true}}, tlog{t})
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Namespaces) != 3 {
				t.Fatalf("namespaces: %+v", m.Namespaces)
			}
			// A second backup into the same folder must refuse to overwrite it.
			if _, err := Run(ctx, cl, Options{Dir: dir, Selection: Selection{Databases: []string{src}}}, tlog{t}); err == nil || !strings.Contains(err.Error(), "not empty") {
				t.Fatalf("overwrite not refused: %v", err)
			}

			ro := RestoreOptions{Dir: dir, Mode: ModeInsert, Selection: Selection{Views: true}, Renames: []Rename{{From: src + ".*", To: dst + ".*"}}}
			rep, err := Restore(ctx, cl, ro, tlog{t})
			if err != nil {
				t.Fatalf("%v %+v", err, rep)
			}
			sc, dc := cl.Database(src).Collection("people"), cl.Database(dst).Collection("people")
			a, b := rawDocs(t, ctx, sc), rawDocs(t, ctx, dc)
			if len(a) != 2501 || len(a) != len(b) {
				t.Fatalf("counts %d vs %d", len(a), len(b))
			}
			for i := range a {
				if !bytes.Equal(a[i], b[i]) {
					t.Fatalf("doc %d differs:\n%s\n%s", i, a[i], b[i])
				}
			}
			si, di := indexNames(t, ctx, sc), indexNames(t, ctx, dc)
			if len(si) != 4 || len(di) != len(si) {
				t.Fatalf("indexes %v vs %v", si, di)
			}
			if lookup(di["exp_1"], "expireAfterSeconds") == nil || lookup(di["email_partial"], "partialFilterExpression") == nil {
				t.Fatalf("index options lost: %v", di)
			}
			// Validator and collation came back.
			var info []collInfo
			cur, _ := cl.Database(dst).ListCollections(ctx, bson.D{})
			cur.All(ctx, &info)
			found := map[string]collInfo{}
			for _, ci := range info {
				found[ci.Name] = ci
			}
			if lookup(found["people"].Options, "validator") == nil || lookup(found["people"].Options, "collation") == nil {
				t.Fatalf("options lost: %v", found["people"].Options)
			}
			if found["adults"].Type != "view" || lookup(found["adults"].Options, "viewOn") != "people" {
				t.Fatalf("view lost: %+v", found["adults"])
			}
			if n, _ := cl.Database(dst).Collection("weird/name%").CountDocuments(ctx, bson.D{}); n != 1 {
				t.Fatalf("escaped name not restored: %d", n)
			}

			// Insert again: everything is a duplicate, nothing changes.
			ro.Selection.Views = false
			rep, err = Restore(ctx, cl, ro, tlog{t})
			if err != nil {
				t.Fatal(err)
			}
			if rep.Results[0].Duplicates != 2501 || rep.Results[0].Inserted != 0 {
				t.Fatalf("second insert: %+v", rep.Results[0])
			}
			// Upsert replaces changed documents.
			dc.UpdateOne(ctx, bson.D{{Key: "_id", Value: int32(5)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "name", Value: "changed"}}}})
			ro.Mode = ModeUpsert
			if rep, err = Restore(ctx, cl, ro, tlog{t}); err != nil {
				t.Fatal(err)
			}
			var got bson.M
			dc.FindOne(ctx, bson.D{{Key: "_id", Value: int32(5)}}).Decode(&got)
			if got["name"] != "p" {
				t.Fatalf("upsert did not replace: %v", got)
			}
			// Drop mode replaces the collection entirely.
			dc.InsertOne(ctx, bson.D{{Key: "_id", Value: "extra"}, {Key: "name", Value: "x"}})
			ro.Mode = ModeDrop
			ro.Selection.Include = []string{src + ".people"}
			if _, err = Restore(ctx, cl, ro, tlog{t}); err != nil {
				t.Fatal(err)
			}
			if n, _ := dc.CountDocuments(ctx, bson.D{}); n != 2501 {
				t.Fatalf("drop mode count %d", n)
			}
		})
	}
}

func TestRestoreDetectsTruncatedBackup(t *testing.T) {
	ctx := context.Background()
	cl, _ := client(t)
	src := "mango3s_it_dump_trunc"
	seed(t, ctx, cl, src)
	dir := filepath.Join(t.TempDir(), "bk")
	if _, err := Run(ctx, cl, Options{Dir: dir, Selection: Selection{Databases: []string{src}, Include: []string{src + ".people"}}}, tlog{t}); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, src, "people.bson")
	b, _ := os.ReadFile(f)
	os.WriteFile(f, b[:len(b)-10], 0o600)
	dst := "mango3s_it_dump_trunc2"
	t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
	_, err := Restore(ctx, cl, RestoreOptions{Dir: dir, Mode: ModeInsert, Renames: []Rename{{From: src + ".*", To: dst + ".*"}}}, tlog{t})
	if err == nil {
		t.Fatal("truncated backup restored without error")
	}
}

// The folder layout must be readable by mongorestore, and mongodump folders
// must be readable by the built-in restore.
func TestCompatibleWithDatabaseTools(t *testing.T) {
	if _, err := exec.LookPath("mongorestore"); err != nil {
		t.Skip("mongorestore not installed")
	}
	ctx := context.Background()
	cl, uri := client(t)
	src := "mango3s_it_dump_tools"
	seed(t, ctx, cl, src)
	for _, gz := range []bool{false, true} {
		dir := filepath.Join(t.TempDir(), "bk")
		if _, err := Run(ctx, cl, Options{Dir: dir, Gzip: gz, Selection: Selection{Databases: []string{src}, Views: true}}, tlog{t}); err != nil {
			t.Fatal(err)
		}
		dst := "mango3s_it_dump_tools2"
		cl.Database(dst).Drop(ctx)
		t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
		args := []string{"--uri", uri, "--nsFrom", src + ".*", "--nsTo", dst + ".*", dir}
		if gz {
			args = append([]string{"--gzip"}, args...)
		}
		if out, err := exec.Command("mongorestore", args...).CombinedOutput(); err != nil {
			t.Fatalf("mongorestore: %v\n%s", err, out)
		}
		if n, _ := cl.Database(dst).Collection("people").CountDocuments(ctx, bson.D{}); n != 2501 {
			t.Fatalf("mongorestore restored %d", n)
		}
		if len(indexNames(t, ctx, cl.Database(dst).Collection("people"))) != 4 {
			t.Fatal("mongorestore lost indexes")
		}
	}
	// mongodump → built-in restore.
	dir := filepath.Join(t.TempDir(), "md")
	if out, err := exec.Command("mongodump", "--uri", uri, "--db", src, "--out", dir, "--gzip").CombinedOutput(); err != nil {
		t.Fatalf("mongodump: %v\n%s", err, out)
	}
	dst := "mango3s_it_dump_tools3"
	t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
	rep, err := Restore(ctx, cl, RestoreOptions{Dir: dir, Mode: ModeInsert, Selection: Selection{Views: true}, Renames: []Rename{{From: src + ".*", To: dst + ".*"}}}, tlog{t})
	if err != nil {
		t.Fatalf("%v %+v", err, rep)
	}
	if n, _ := cl.Database(dst).Collection("people").CountDocuments(ctx, bson.D{}); n != 2501 {
		t.Fatalf("built-in restore of mongodump folder: %d", n)
	}
}

func TestToolArchiveRoundTrip(t *testing.T) {
	dumpPath, err1 := exec.LookPath("mongodump")
	restorePath, err2 := exec.LookPath("mongorestore")
	if err1 != nil || err2 != nil {
		t.Skip("database tools not installed")
	}
	ctx := context.Background()
	cl, uri := client(t)
	src, dst := "mango3s_it_tool_src", "mango3s_it_tool_dst"
	seed(t, ctx, cl, src)
	t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
	p := config.Profile{Name: "it", URI: uri + "/?directConnection=true"}
	archive := filepath.Join(t.TempDir(), "b.archive")
	ta, err := DumpArgs(p, mongox.Credentials{}, ToolDump{Archive: archive, Gzip: true, DB: src})
	if err != nil {
		t.Fatal(err)
	}
	sum, err := RunTool(ctx, dumpPath, ta, tlog{t})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Documents != 2502 {
		t.Fatalf("dumped %d", sum.Documents)
	}
	if _, err := DumpArgs(p, mongox.Credentials{}, ToolDump{Archive: archive}); err == nil {
		t.Fatal("existing archive would be overwritten")
	}
	ta, err = RestoreArgs(p, mongox.Credentials{}, ToolRestore{Archive: archive, Gzip: true, NsFrom: []string{src + ".*"}, NsTo: []string{dst + ".*"}})
	if err != nil {
		t.Fatal(err)
	}
	if sum, err = RunTool(ctx, restorePath, ta, tlog{t}); err != nil {
		t.Fatal(err)
	}
	if sum.Documents != 2502 {
		t.Fatalf("restored %d", sum.Documents)
	}
	if n, _ := cl.Database(dst).Collection("people").CountDocuments(ctx, bson.D{}); n != 2501 {
		t.Fatalf("count %d", n)
	}
	// Restoring again without drop reports the duplicates as failures.
	ta, _ = RestoreArgs(p, mongox.Credentials{}, ToolRestore{Archive: archive, Gzip: true, NsFrom: []string{src + ".*"}, NsTo: []string{dst + ".*"}})
	if _, err = RunTool(ctx, restorePath, ta, tlog{t}); err == nil {
		t.Fatal("duplicate restore reported success")
	}
}

func TestCopyCollectionsAndDatabases(t *testing.T) {
	ctx := context.Background()
	cl, _ := client(t)
	src, dst := "mango3s_it_copy_src", "mango3s_it_copy_dst"
	seed(t, ctx, cl, src)
	t.Cleanup(func() { cl.Database(dst).Drop(context.Background()) })
	// Whole database with views.
	rep, err := Copy(ctx, cl, cl, true, CopyOptions{SrcDB: src, DstDB: dst, Mode: ModeInsert, Views: true}, tlog{t})
	if err != nil {
		t.Fatalf("%v %+v", err, rep)
	}
	a, b := rawDocs(t, ctx, cl.Database(src).Collection("people")), rawDocs(t, ctx, cl.Database(dst).Collection("people"))
	if len(a) != len(b) || !bytes.Equal(a[0], b[0]) {
		t.Fatal("documents differ")
	}
	if len(indexNames(t, ctx, cl.Database(dst).Collection("people"))) != 4 {
		t.Fatal("indexes not copied")
	}
	if names, _ := cl.Database(dst).ListCollectionNames(ctx, bson.D{{Key: "type", Value: "view"}}); len(names) != 1 {
		t.Fatalf("views: %v", names)
	}
	// One collection under a new name, filtered.
	_, err = Copy(ctx, cl, cl, true, CopyOptions{SrcDB: src, Collections: []string{"people"}, DstDB: src, DstColl: "people_small", Mode: ModeDrop, Filter: `{"_id":{"$lt":10}}`}, tlog{t})
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := cl.Database(src).Collection("people_small").CountDocuments(ctx, bson.D{}); n != 10 {
		t.Fatalf("filtered copy %d", n)
	}
	// Onto itself is refused.
	if _, err := Copy(ctx, cl, cl, true, CopyOptions{SrcDB: src, Collections: []string{"people"}, Mode: ModeInsert}, tlog{t}); err == nil {
		t.Fatal("copy onto itself")
	}
	// Missing collection is reported, not skipped.
	if _, err := Copy(ctx, cl, cl, true, CopyOptions{SrcDB: src, Collections: []string{"nope"}, DstDB: dst, Mode: ModeInsert}, tlog{t}); err == nil {
		t.Fatal("missing collection ignored")
	}
}
