package syncer

import (
	"testing"
	"time"

	"mango3s/internal/compare"
	"mango3s/internal/plans"
)

func result() *compare.Result {
	return &compare.Result{
		Options: compare.Options{Target: compare.Endpoint{DB: "d", Coll: "c"}},
		Entries: []compare.Entry{
			{Key: "a", Kind: compare.KindOnlySource},
			{Key: "b", Kind: compare.KindChanged},
			{Key: "c", Kind: compare.KindOnlyTarget},
			{Key: "d", Kind: compare.KindChanged},
		},
		Samples: []compare.DocDiff{{ID: "a", Kind: compare.KindOnlySource}, {ID: "c", Kind: compare.KindOnlyTarget}},
	}
}

func TestBuildPlan(t *testing.T) {
	p, err := BuildPlan(result(), Options{Insert: true, Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	if c := p.counts(); c != (Counts{Inserts: 1, Replaces: 2}) {
		t.Fatalf("counts: %+v", c)
	}
	if len(p.Samples) != 1 || p.Samples[0].ID != "a" {
		t.Fatalf("samples must only show planned kinds: %+v", p.Samples)
	}
	p, _ = BuildPlan(result(), Options{Delete: true})
	if c := p.counts(); c != (Counts{Deletes: 1}) {
		t.Fatalf("delete only: %+v", c)
	}
	if _, err := BuildPlan(result(), Options{}); err == nil {
		t.Fatal("empty options accepted")
	}
}

func TestPlanStoreSingleUseAndExpiry(t *testing.T) {
	now := time.Now()
	s := plans.New[Plan](time.Minute)
	s.SetClock(func() time.Time { return now })
	id := s.Put(Plan{})
	if _, err := s.Get(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Take(id); err == nil {
		t.Fatal("plan applied twice")
	}
	id = s.Put(Plan{})
	now = now.Add(2 * time.Minute)
	if _, err := s.Get(id); err == nil {
		t.Fatal("expired plan accepted")
	}
}
