package datagen

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
	"mango3s/internal/schema"
)

var now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

const tmpl = `{
  name: $name, email: $email, age: $int(18, 90), created: $date("-1y", "now"),
  score: $double(0, 10, 1), price: $decimal(1, 99), active: $bool, id: $objectId,
  status: $pick("new", "paid"), n: $seq(100), tags: [$word, "fixed"],
  address: { city: $city, zip: "75001" }, note: "costs $5, keep $name as text", big: $long(1, 5)
}`

func TestTypesMatchTheTemplate(t *testing.T) {
	tp, err := Compile(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	g := tp.NewGenerator(42, now)
	for i := 0; i < 200; i++ {
		d := g.Next()
		get := func(k string) any { v, _ := mql.Lookup(d, k); return v }
		if s, ok := get("name").(string); !ok || !strings.Contains(s, " ") {
			t.Fatalf("name: %v", get("name"))
		}
		if s, _ := get("email").(string); !strings.Contains(s, "@") {
			t.Fatalf("email: %v", get("email"))
		}
		if a, ok := get("age").(int32); !ok || a < 18 || a > 90 {
			t.Fatalf("age: %v (%T)", get("age"), get("age"))
		}
		if c, ok := get("created").(bson.DateTime); !ok || c.Time().Before(now.AddDate(-1, 0, 0)) || c.Time().After(now) {
			t.Fatalf("created: %v", get("created"))
		}
		if _, ok := get("price").(bson.Decimal128); !ok {
			t.Fatalf("price: %T", get("price"))
		}
		if _, ok := get("id").(bson.ObjectID); !ok {
			t.Fatalf("id: %T", get("id"))
		}
		if s := get("status"); s != "new" && s != "paid" {
			t.Fatalf("status: %v", s)
		}
		if n := get("n"); n != int32(100+i) {
			t.Fatalf("seq: %v at %d", n, i)
		}
		if b, ok := get("big").(int64); !ok || b < 1 || b > 5 {
			t.Fatalf("big: %v", get("big"))
		}
		tags := get("tags").(bson.A)
		if len(tags) != 2 || tags[1] != "fixed" {
			t.Fatalf("tags: %v", tags)
		}
		if get("note") != "costs $5, keep $name as text" {
			t.Fatalf("quoted text changed: %v", get("note"))
		}
		addr := get("address").(bson.D)
		if mql.Compact(addr[1].Value) != `"75001"` {
			t.Fatalf("literal: %v", addr)
		}
	}
}

func TestSameSeedSameData(t *testing.T) {
	tp, _ := Compile(tmpl)
	a, b, c := tp.NewGenerator(7, now), tp.NewGenerator(7, now), tp.NewGenerator(8, now)
	same, differ := true, false
	for i := 0; i < 50; i++ {
		x, y, z := mql.Compact(a.Next()), mql.Compact(b.Next()), mql.Compact(c.Next())
		same = same && x == y
		differ = differ || x != z
	}
	if !same || !differ {
		t.Fatalf("same seed equal: %v, other seed different: %v", same, differ)
	}
}

func TestMaybeAndErrors(t *testing.T) {
	tp, err := Compile(`{ a: $maybe(0.25, $email), b: 1 }`)
	if err != nil {
		t.Fatal(err)
	}
	g := tp.NewGenerator(1, now)
	with := 0
	for i := 0; i < 2000; i++ {
		if _, ok := mql.Lookup(g.Next(), "a"); ok {
			with++
		}
	}
	if with < 400 || with > 600 {
		t.Fatalf("$maybe(0.25) present %d of 2000 times", with)
	}
	for _, bad := range []string{`{ a: $int(5) }`, `{ a: $int(9, 1) }`, `{ a: $pick() }`, `{ a: $date("someday") }`, `{ a: $maybe(2, 1) }`, `{ a: "x }`} {
		if _, err := Compile(bad); err == nil {
			t.Errorf("%s should be refused", bad)
		}
	}
}

func TestFromSchemaCompiles(t *testing.T) {
	a := schema.NewAnalyzer()
	for i := 0; i < 10; i++ {
		d := bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "email", Value: "x@y.z"}, {Key: "qty", Value: int32(i)},
			{Key: "status", Value: []string{"a", "b"}[i%2]}, {Key: "addr", Value: bson.D{{Key: "city", Value: "Rome"}}}}
		if i%2 == 0 {
			d = append(d, bson.E{Key: "note", Value: "n"})
		}
		a.Add(d)
	}
	src := FromSchema(a.Report())
	for _, want := range []string{"$objectId", "$email", "$int(0, 9)", `$pick("a", "b")`, "$city", "$maybe(0.50"} {
		if !strings.Contains(src, want) {
			t.Errorf("template lacks %s:\n%s", want, src)
		}
	}
	tp, err := Compile(src)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	if d := tp.NewGenerator(1, now).Next(); len(d) < 5 {
		t.Fatalf("generated %s", mql.Compact(d))
	}
}
