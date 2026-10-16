package recent

import (
	"fmt"
	"testing"
	"time"
)

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { now = now.Add(time.Minute); return now })
	_ = s.Record("p1", "shop", "orders")
	_ = s.Record("p1", "shop", "customers")
	_ = s.Record("p1", "shop", "orders")
	_ = s.Record("p2", "hr", "staff")
	list, _ := New(dir).List("p1") // persisted
	if len(list) != 2 || list[0].Coll != "orders" || list[0].Count != 2 || list[1].Coll != "customers" {
		t.Fatalf("history: %+v", list)
	}
	for i := 0; i < MaxPerProfile+5; i++ {
		_ = s.Record("p1", "db", fmt.Sprint("c", i))
	}
	if list, _ = s.List("p1"); len(list) != MaxPerProfile {
		t.Fatalf("cap: %d", len(list))
	}
	_ = s.Copy("p2", "p3")
	_ = s.Clear("p1")
	if list, _ = s.List("p1"); len(list) != 0 {
		t.Fatal("clear")
	}
	if list, _ = s.List("p3"); len(list) != 1 || list[0].Coll != "staff" {
		t.Fatalf("copy: %+v", list)
	}
	if err := s.Record("", "a", "b"); err == nil {
		t.Fatal("validation")
	}
}
