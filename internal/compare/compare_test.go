package compare

import (
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

func TestNormalizeAndHash(t *testing.T) {
	a := doc(t, `{_id: 1, b: {y: 2, x: 1}, a: 1, updatedAt: ISODate("2024-01-01"), items: [{sku: "a", ts: 1}]}`)
	b := doc(t, `{_id: 1, a: 1, b: {x: 1, y: 2}, updatedAt: ISODate("2025-01-01"), items: [{sku: "a", ts: 2}]}`)
	if HashDoc(a) == HashDoc(b) {
		t.Fatal("different documents hash equal")
	}
	o := Options{IgnoreFields: []string{"updatedAt", "items.ts"}, IgnoreFieldOrder: true}
	if o.HashFor(a) != o.HashFor(b) {
		t.Fatalf("normalized documents differ:\n%s\n%s", mql.Compact(Normalize(a, o.IgnoreFields, true)), mql.Compact(Normalize(b, o.IgnoreFields, true)))
	}
	// Field order matters unless ignored; type matters always.
	if (Options{}).HashFor(doc(t, `{a: 1, b: 2}`)) == (Options{}).HashFor(doc(t, `{b: 2, a: 1}`)) {
		t.Fatal("field order ignored by default")
	}
	if o.HashFor(doc(t, `{a: 1}`)) == o.HashFor(doc(t, `{a: NumberLong(1)}`)) {
		t.Fatal("int32 and int64 must differ")
	}
	// Normalize must not mutate its input.
	_ = Normalize(a, o.IgnoreFields, true)
	if mql.Compact(a) != `{"_id": 1, "b": {"y": 2, "x": 1}, "a": 1, "updatedAt": ISODate("2024-01-01T00:00:00.000Z"), "items": [{"sku": "a", "ts": 1}]}` {
		t.Fatalf("input mutated: %s", mql.Compact(a))
	}
}

func TestIDKeyRoundTrip(t *testing.T) {
	ids := []any{bson.NewObjectID(), "x", int32(1), int64(1), 1.5, bson.D{{Key: "a", Value: int32(1)}}, bson.Binary{Subtype: 4, Data: make([]byte, 16)}}
	seen := map[string]bool{}
	for _, id := range ids {
		k := IDKey(id)
		if seen[k] {
			t.Fatalf("key collision for %v", id)
		}
		seen[k] = true
		back, err := ParseIDKey(k)
		if err != nil || !mql.Equal(back, id) {
			t.Fatalf("round trip %v -> %s -> %v (%v)", id, k, back, err)
		}
	}
}

func TestFingerprintOrderIndependent(t *testing.T) {
	h := func(s string) Hash { return HashDoc(bson.D{{Key: "s", Value: s}}) }
	a, b := newFingerprint(), newFingerprint()
	a.add("1", h("x"), h("x"), "same")
	a.add("2", h("y"), h("z"), KindChanged)
	b.add("2", h("y"), h("z"), KindChanged)
	b.add("1", h("x"), h("x"), "same")
	if a.String() != b.String() {
		t.Fatal("fingerprint depends on order")
	}
	c := newFingerprint()
	c.add("1", h("x"), h("x"), "same")
	c.add("2", h("y"), h("w"), KindChanged)
	if a.String() == c.String() {
		t.Fatal("different inputs, same fingerprint")
	}
}

func TestDiff(t *testing.T) {
	s := doc(t, `{_id: 1, name: "Ada", address: {city: "London", zip: "N1"}, tags: [1, 2], extra: true}`)
	tg := doc(t, `{_id: 1, name: "Ada L.", address: {city: "London"}, tags: [2, 1], legacy: 1}`)
	got := Diff(s, tg, "")
	want := map[string]string{"name": "changed", "address.zip": "added", "tags": "changed", "extra": "added", "legacy": "removed"}
	if len(got) != len(want) {
		t.Fatalf("diff: %+v", got)
	}
	for _, c := range got {
		if want[c.Path] != c.Change {
			t.Errorf("%s: %s, want %s", c.Path, c.Change, want[c.Path])
		}
	}
	order := Diff(doc(t, `{a: 1, b: 2}`), doc(t, `{b: 2, a: 1}`), "")
	if len(order) != 1 || order[0].Source != "field order differs" {
		t.Fatalf("order-only diff: %+v", order)
	}
}
