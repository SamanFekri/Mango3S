package library

import (
	"fmt"
	"testing"
	"time"

	"mango3s/internal/aggregate"
)

func TestStorePersistsAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	q, err := s.Save(Item{Kind: KindQuery, Name: " Active users ", DB: "app", Coll: "users",
		Query: &QueryPayload{Filter: `{active: true}`, Sort: `{name: 1}`, Limit: 50}})
	if err != nil {
		t.Fatal(err)
	}
	if q.ID == "" || q.Name != "Active users" || q.CreatedAt.IsZero() {
		t.Fatalf("saved: %+v", q)
	}
	time.Sleep(2 * time.Millisecond)
	p, err := s.Save(Item{Kind: KindAggregation, Name: "Revenue", Stages: []aggregate.Stage{{Operator: "$group", Body: "{_id: null}", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(Item{Kind: KindScript, Name: "Cleanup", Text: "db.x.deleteMany({})"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(Item{Kind: KindSQL, Name: "Top", Text: "SELECT * FROM users"}); err != nil {
		t.Fatal(err)
	}

	// A new Store instance (= app restart) sees everything.
	s2 := NewStore(dir)
	all, err := s2.List("")
	if err != nil || len(all) != 4 {
		t.Fatalf("reload: %d %v", len(all), err)
	}
	if all[0].Name != "Top" {
		t.Fatalf("newest first: %s", all[0].Name)
	}
	queries, _ := s2.List(KindQuery)
	if len(queries) != 1 || queries[0].Query.Filter != `{active: true}` || queries[0].Query.Limit != 50 {
		t.Fatalf("query payload: %+v", queries)
	}

	p.Name = "Revenue v2"
	updated, err := s2.Save(p)
	if err != nil || !updated.CreatedAt.Equal(p.CreatedAt) || updated.Name != "Revenue v2" {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if err := s2.Delete(q.ID); err != nil {
		t.Fatal(err)
	}
	if left, _ := NewStore(dir).List(""); len(left) != 3 {
		t.Fatalf("after delete: %d", len(left))
	}
}

func TestStoreValidation(t *testing.T) {
	s := NewStore(t.TempDir())
	bad := []Item{
		{Kind: "macro", Name: "x", Text: "y"},
		{Kind: KindQuery, Name: "", Query: &QueryPayload{}},
		{Kind: KindQuery, Name: "x"},
		{Kind: KindAggregation, Name: "x"},
		{Kind: KindScript, Name: "x", Text: "  "},
	}
	for _, it := range bad {
		if _, err := s.Save(it); err == nil {
			t.Errorf("accepted %+v", it)
		}
	}
	if _, err := s.Save(Item{ID: "missing", Kind: KindSQL, Name: "x", Text: "SELECT 1"}); err == nil {
		t.Fatal("update of missing item accepted")
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	h := NewHistory(dir, 5)
	for i := 0; i < 7; i++ {
		if err := h.Add(Entry{Kind: KindQuery, DB: "app", Coll: "users", Summary: fmt.Sprintf("q%d", i), OK: true}); err != nil {
			t.Fatal(err)
		}
	}
	// Re-running the latest query does not duplicate it.
	_ = h.Add(Entry{Kind: KindQuery, DB: "app", Coll: "users", Summary: "q6", OK: false, Error: "boom"})
	_ = h.Add(Entry{Kind: KindScript, DB: "app", Summary: "show dbs"})

	h2 := NewHistory(dir, 5)
	all, err := h2.List("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 || all[0].Summary != "show dbs" || all[1].Summary != "q6" || all[1].OK {
		t.Fatalf("history: %+v", all)
	}
	if hits, _ := h2.List(KindQuery, "Q5", 0); len(hits) != 1 {
		t.Fatalf("search: %+v", hits)
	}
	if hits, _ := h2.List("", "", 2); len(hits) != 2 {
		t.Fatal("limit")
	}
	if err := h2.Add(Entry{Kind: "bogus"}); err == nil {
		t.Fatal("bad kind accepted")
	}
	if err := h2.Clear(); err != nil {
		t.Fatal(err)
	}
	if left, _ := NewHistory(dir, 5).List("", "", 0); len(left) != 0 {
		t.Fatal("clear not persisted")
	}
}
