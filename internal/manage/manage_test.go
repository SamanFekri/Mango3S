package manage

import (
	"strings"
	"testing"

	"mango3s/internal/mql"
)

func TestCreateCommand(t *testing.T) {
	cmd, err := CreateCommand(CreateOptions{Name: "c", Capped: true, SizeBytes: 4096, MaxDocs: 10, Collation: `{ locale: "en" }`, ValidationLevel: "moderate"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mql.Compact(cmd); got != `{"create": "c", "capped": true, "size": NumberLong(4096), "max": NumberLong(10), "collation": {"locale": "en"}, "validationLevel": "moderate"}` {
		t.Fatal(got)
	}
	bad := []CreateOptions{
		{Name: "c", Capped: true},
		{Name: "c", Capped: true, SizeBytes: 1, TimeField: "t"},
		{Name: "c", MetaField: "m"},
		{Name: "c", ExpireAfter: 10},
		{Name: "c", Validator: "{ oops"},
		{Name: "c", ValidationAction: "explode"},
		{Name: "system.x"},
		{Name: ""},
	}
	for _, o := range bad {
		if _, err := CreateCommand(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}
	cmd, err = CreateCommand(CreateOptions{Name: "m", TimeField: "ts", Granularity: "hours", ExpireAfter: 60})
	if err != nil || !strings.Contains(mql.Compact(cmd), `"timeseries": {"timeField": "ts", "granularity": "hours"}`) {
		t.Fatal(mql.Compact(cmd), err)
	}
}

func TestIndexSpec(t *testing.T) {
	ttl := int64(30)
	spec, err := IndexSpec(IndexOptions{Keys: `{ a: 1, "b.c": -1 }`, Unique: true, Partial: `{ a: { $exists: true } }`})
	if err != nil {
		t.Fatal(err)
	}
	if got := mql.Compact(spec); got != `{"key": {"a": 1, "b.c": -1}, "name": "a_1_b.c_-1", "unique": true, "partialFilterExpression": {"a": {"$exists": true}}}` {
		t.Fatal(got)
	}
	spec, err = IndexSpec(IndexOptions{Keys: `{ title: "text" }`, Name: "search", Extra: `{ weights: { title: 10 }, default_language: "english" }`})
	if err != nil || !strings.Contains(mql.Compact(spec), "weights") {
		t.Fatal(mql.Compact(spec), err)
	}
	for _, o := range []IndexOptions{{}, {Keys: `{ a: 1, b: 1 }`, ExpireAfter: &ttl}, {Keys: `{ a: 1 }`, Unique: true, Extra: `{ name: "x" }`}} {
		if _, err := IndexSpec(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}
}
