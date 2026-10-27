//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/manage"
	"mango3s/internal/mql"
)

// Roadmap 5.3: a plain mongod reports search indexes as unsupported instead
// of failing.
func TestSearchIndexesUnsupported(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	list, err := manage.ListSearchIndexes(e.ctx, s.Client.Database(e.db), "orders")
	if err != nil || list.Supported || !strings.Contains(list.Reason, "Atlas Search") || list.Indexes == nil {
		t.Fatalf("plain server: %+v %v", list, err)
	}
	_, err = manage.CreateSearchIndex(e.ctx, s.Client.Database(e.db), "orders", manage.SearchIndexInput{Definition: `{mappings: {dynamic: true}}`})
	if err == nil || !strings.Contains(err.Error(), "Atlas Search") {
		t.Fatalf("create on a plain server: %v", err)
	}
}

// Roadmap 5.3 on a local Atlas deployment (make search-up):
// MONGO_SEARCH_URI=mongodb://localhost:27022/?directConnection=true
func TestSearchIndexes(t *testing.T) {
	uri := os.Getenv("MONGO_SEARCH_URI")
	if uri == "" {
		t.Skip("MONGO_SEARCH_URI not set")
	}
	c := mustClient(t, uri)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	db := c.Database("m3s_search_" + hex.EncodeToString(b))
	t.Cleanup(func() { _ = db.Drop(context.Background()) })
	movies := db.Collection("movies")
	if _, err := movies.InsertMany(ctx, []any{
		bson.D{{Key: "title", Value: "The Mango Tree"}, {Key: "embedding", Value: bson.A{0.1, 0.2, 0.3}}},
		bson.D{{Key: "title", Value: "Apple Pie"}, {Key: "embedding", Value: bson.A{0.9, 0.1, 0.0}}},
	}); err != nil {
		t.Fatal(err)
	}

	name, err := manage.CreateSearchIndex(ctx, db, "movies", manage.SearchIndexInput{Definition: `{mappings: {dynamic: true}}`})
	if err != nil || name != "default" {
		t.Fatalf("create search index: %q %v", name, err)
	}
	if _, err := manage.CreateSearchIndex(ctx, db, "movies", manage.SearchIndexInput{Name: "vec", Type: manage.SearchTypeVector,
		Definition: `{fields: [{type: "vector", path: "embedding", numDimensions: 3, similarity: "cosine"}]}`}); err != nil {
		t.Fatalf("create vector index: %v", err)
	}
	if _, err := manage.CreateSearchIndex(ctx, db, "movies", manage.SearchIndexInput{Definition: `[1]`}); err == nil {
		t.Fatal("a non-document definition was accepted")
	}

	// Building is asynchronous: wait until both can be queried.
	waitFor := func(what string, ok func([]manage.SearchIndex) bool) []manage.SearchIndex {
		for {
			list, err := manage.ListSearchIndexes(ctx, db, "movies")
			if err != nil || !list.Supported {
				t.Fatalf("list: %+v %v", list, err)
			}
			if ok(list.Indexes) {
				return list.Indexes
			}
			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s: %+v", what, list.Indexes)
			case <-time.After(time.Second):
			}
		}
	}
	idx := waitFor("queryable indexes", func(l []manage.SearchIndex) bool {
		return len(l) == 2 && l[0].Queryable && l[1].Queryable
	})
	vi := slices.IndexFunc(idx, func(s manage.SearchIndex) bool { return s.Name == "vec" })
	if vi < 0 || idx[vi].Type != manage.SearchTypeVector || !strings.Contains(idx[vi].Definition, "numDimensions") {
		t.Fatalf("vector index: %+v", idx)
	}

	cur, err := movies.Aggregate(ctx, bson.A{bson.D{{Key: "$search", Value: bson.D{{Key: "text", Value: bson.D{{Key: "query", Value: "mango"}, {Key: "path", Value: "title"}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	var found []bson.M
	if err := cur.All(ctx, &found); err != nil || len(found) != 1 || found[0]["title"] != "The Mango Tree" {
		t.Fatalf("$search: %v %v", found, err)
	}

	if err := manage.UpdateSearchIndex(ctx, db, "movies", "default", `{mappings: {dynamic: false, fields: {title: {type: "string"}}}}`); err != nil {
		t.Fatal(err)
	}
	waitFor("the new definition", func(l []manage.SearchIndex) bool {
		i := slices.IndexFunc(l, func(s manage.SearchIndex) bool { return s.Name == "default" })
		if i < 0 {
			return false
		}
		v, err := mql.ParseValue(l[i].Definition)
		if err != nil {
			t.Fatalf("definition %q: %v", l[i].Definition, err)
		}
		m, _ := mql.Lookup(v.(bson.D), "mappings")
		dyn, _ := mql.Lookup(m.(bson.D), "dynamic")
		return dyn == false
	})
	if err := manage.DropSearchIndex(ctx, db, "movies", "vec"); err != nil {
		t.Fatal(err)
	}
	waitFor("the drop", func(l []manage.SearchIndex) bool {
		return !slices.ContainsFunc(l, func(s manage.SearchIndex) bool { return s.Name == "vec" && s.Status != "DELETING" })
	})
}
