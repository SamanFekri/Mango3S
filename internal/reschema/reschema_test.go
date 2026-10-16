package reschema

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

func run(t *testing.T, src string, ops ...Op) (string, bool, error) {
	t.Helper()
	c, err := Compile(ops)
	if err != nil {
		t.Fatal(err)
	}
	in := doc(t, src)
	before := mql.Compact(in)
	out, changed, err := c.Apply(in)
	if mql.Compact(in) != before {
		t.Fatal("Apply mutated its input")
	}
	if err != nil {
		return "", false, err
	}
	return mql.Compact(out), changed, nil
}

func TestOperations(t *testing.T) {
	cases := []struct {
		name string
		src  string
		ops  []Op
		want string
	}{
		{"rename keeps position", `{_id: 1, fname: "Ada", age: 36}`, []Op{{Type: OpRename, Field: "fname", To: "firstName"}},
			`{"_id": 1, "firstName": "Ada", "age": 36}`},
		{"rename into sub-document", `{_id: 1, city: "Paris", n: 1}`, []Op{{Type: OpRename, Field: "city", To: "address.city"}},
			`{"_id": 1, "n": 1, "address": {"city": "Paris"}}`},
		{"rename nested in place", `{_id: 1, a: {x: 1, y: 2}}`, []Op{{Type: OpRename, Field: "a.x", To: "a.z"}},
			`{"_id": 1, "a": {"z": 1, "y": 2}}`},
		{"remove", `{_id: 1, tmp: 1, keep: 2}`, []Op{{Type: OpRemove, Field: "tmp"}}, `{"_id": 1, "keep": 2}`},
		{"set only if missing", `{_id: 1, status: "A"}`, []Op{{Type: OpSet, Field: "status", Value: `"B"`, OnlyIfMissing: true}, {Type: OpSet, Field: "meta.v", Value: `NumberInt(2)`}},
			`{"_id": 1, "status": "A", "meta": {"v": 2}}`},
		{"copy", `{_id: 1, a: {b: 1}}`, []Op{{Type: OpCopy, Field: "a", To: "a2"}}, `{"_id": 1, "a": {"b": 1}, "a2": {"b": 1}}`},
		{"convert through arrays", `{_id: 1, items: [{qty: "2"}, {qty: "3"}], price: "9.99", when: "2024-01-31"}`,
			[]Op{{Type: OpConvert, Field: "items.qty", TargetType: "int"}, {Type: OpConvert, Field: "price", TargetType: "decimal"}, {Type: OpConvert, Field: "when", TargetType: "date"}},
			`{"_id": 1, "items": [{"qty": 2}, {"qty": 3}], "price": NumberDecimal("9.99"), "when": ISODate("2024-01-31T00:00:00.000Z")}`},
		{"flatten", `{_id: 1, address: {city: "Paris", geo: {lat: 1.5, lng: 2.5}}, z: 0}`, []Op{{Type: OpFlatten, Field: "address"}},
			`{"_id": 1, "address_city": "Paris", "address_geo_lat": 1.5, "address_geo_lng": 2.5, "z": 0}`},
		{"nest", `{_id: 1, street: "Main", city: "X", name: "n"}`, []Op{{Type: OpNest, Fields: []string{"street", "city"}, To: "address"}},
			`{"_id": 1, "name": "n", "address": {"street": "Main", "city": "X"}}`},
		{"missing fields are no-ops", `{_id: 1}`, []Op{{Type: OpRename, Field: "a", To: "b"}, {Type: OpRemove, Field: "c"}, {Type: OpConvert, Field: "d", TargetType: "int"}},
			`{"_id": 1}`},
		{"null converts to null", `{_id: 1, n: null}`, []Op{{Type: OpConvert, Field: "n", TargetType: "int"}}, `{"_id": 1, "n": null}`},
	}
	for _, c := range cases {
		got, _, err := run(t, c.src, c.ops...)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
	if _, changed, _ := run(t, `{_id: 1}`, Op{Type: OpRemove, Field: "x"}); changed {
		t.Fatal("no-op reported a change")
	}
}

func TestPerDocumentErrors(t *testing.T) {
	cases := map[string][]Op{
		"not an integer":  {{Type: OpConvert, Field: "s", TargetType: "int"}},
		"already exists":  {{Type: OpRename, Field: "s", To: "n"}},
		"does not fit":    {{Type: OpConvert, Field: "big", TargetType: "int"}},
		"would overwrite": {{Type: OpFlatten, Field: "a"}},
	}
	for want, ops := range cases {
		_, _, err := run(t, `{_id: 1, s: "abc", n: 1, big: NumberLong("9000000000"), a: {b: 1}, a_b: 2}`, ops...)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestCompileValidation(t *testing.T) {
	bad := [][]Op{
		{},
		{{Type: "explode", Field: "a"}},
		{{Type: OpRename, Field: "a"}},
		{{Type: OpRename, Field: "a", To: "a.b"}},
		{{Type: OpRemove, Field: "_id"}},
		{{Type: OpConvert, Field: "_id", TargetType: "string"}},
		{{Type: OpConvert, Field: "a", TargetType: "uuid"}},
		{{Type: OpSet, Field: "a", Value: "{bad"}},
		{{Type: OpNest, Fields: []string{"a.b"}, To: "x"}},
	}
	for _, ops := range bad {
		if _, err := Compile(ops); err == nil {
			t.Errorf("accepted %+v", ops)
		}
	}
}

func TestConvertMatrix(t *testing.T) {
	oid := bson.NewObjectID()
	ok := []struct {
		in   any
		to   string
		want string
	}{
		{int32(5), "long", "NumberLong(5)"},
		{3.0, "int", "3"},
		{"42", "long", "NumberLong(42)"},
		{"yes", "bool", "true"},
		{int64(0), "bool", "false"},
		{oid.Hex(), "objectId", `ObjectId("` + oid.Hex() + `")`},
		{oid, "string", `"` + oid.Hex() + `"`},
		{int64(1700000000000), "date", `ISODate("2023-11-14T22:13:20.000Z")`},
		{"1e3", "double", "1000.0"},
	}
	for _, c := range ok {
		got, err := Convert(c.in, c.to)
		if err != nil || mql.Compact(got) != c.want {
			t.Errorf("%v -> %s: %v %v (want %s)", c.in, c.to, mql.Compact(got), err, c.want)
		}
	}
	for _, c := range []struct {
		in any
		to string
	}{{3.5, "int"}, {"x", "objectId"}, {"maybe", "bool"}, {bson.D{}, "string"}} {
		if _, err := Convert(c.in, c.to); err == nil {
			t.Errorf("%v -> %s accepted", c.in, c.to)
		}
	}
}
