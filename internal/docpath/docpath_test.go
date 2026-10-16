package docpath

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func doc(t *testing.T, s string) bson.D {
	t.Helper()
	d, err := mql.ParseDocument(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGetSetDelete(t *testing.T) {
	d := doc(t, `{a: {b: 1}, s: "x"}`)
	if v, ok := Get(d, "a.b"); !ok || v != int32(1) {
		t.Fatal("get nested")
	}
	if _, ok := Get(d, "s.x"); ok {
		t.Fatal("get through scalar")
	}
	d, ok := Set(d, "a.c.d", "new")
	if !ok || mql.Compact(d) != `{"a": {"b": 1, "c": {"d": "new"}}, "s": "x"}` {
		t.Fatalf("set: %s", mql.Compact(d))
	}
	if _, ok := Set(d, "s.x", 1); ok {
		t.Fatal("set through scalar must fail")
	}
	d, v, found := Delete(d, "a.b")
	if !found || v != int32(1) || Exists(d, "a.b") {
		t.Fatalf("delete: %s", mql.Compact(d))
	}
	if _, _, found := Delete(d, "zz"); found {
		t.Fatal("delete missing")
	}
	if p, l := Parent("a.b.c"); p != "a.b" || l != "c" {
		t.Fatal("parent")
	}
}

func TestUpdateTraversesArrays(t *testing.T) {
	d := doc(t, `{items: [{sku: "a"}, {sku: "b"}, 5, {other: 1}], name: "n"}`)
	d, n, err := Update(d, "items.sku", func(v any) (any, bool, error) {
		return strings.ToUpper(v.(string)), true, nil
	})
	if err != nil || n != 2 || mql.Compact(d) != `{"items": [{"sku": "A"}, {"sku": "B"}, 5, {"other": 1}], "name": "n"}` {
		t.Fatalf("update: %d %s", n, mql.Compact(d))
	}
	d, n, _ = Update(d, "name", func(any) (any, bool, error) { return nil, false, nil })
	if n != 1 || Exists(d, "name") {
		t.Fatal("remove via update")
	}
}

func TestClone(t *testing.T) {
	d := doc(t, `{a: {b: [1, {c: 2}]}}`)
	c := CloneDoc(d)
	c, _ = Set(c, "a.b", "changed")
	if mql.Compact(d) != `{"a": {"b": [1, {"c": 2}]}}` {
		t.Fatal("clone shares memory")
	}
}
