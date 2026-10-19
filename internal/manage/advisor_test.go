package manage

import (
	"reflect"
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

func TestClassifyAndSuggest(t *testing.T) {
	cases := []struct {
		filter, sort string
		want         string
		blocked      bool
	}{
		{`{ status: "A", age: { $gt: 5 } }`, `{ created: -1 }`, `{ status: 1, created: -1, age: 1 }`, false},
		{`{ $and: [ { a: 1 }, { b: { $in: [1, 2] } } ] }`, ``, `{ a: 1, b: 1 }`, false},
		{`{ name: /^jo/i, a: { $gte: 1, $eq: 1 } }`, ``, `{ name: 1, a: 1 }`, false},
		{`{ a: 1, a2: { $exists: true } }`, `{ a: 1 }`, `{ a: 1, a2: 1 }`, false},
		{`{ "addr.city": "Oslo" }`, ``, `{ "addr.city": 1 }`, false},
		{`{ $or: [ { a: 1 }, { b: 2 } ] }`, ``, ``, true},
		{`{ loc: { $near: [0, 0] } }`, ``, ``, true},
		{`{ x: { $elemMatch: { y: 1 } } }`, `{ score: { $meta: "textScore" } }`, `{ x: 1 }`, false},
	}
	for _, c := range cases {
		var p predicates
		classify(doc(t, c.filter), &p)
		var s bson.D
		if c.sort != "" {
			s = doc(t, c.sort)
		}
		if (p.blocked != "") != c.blocked {
			t.Errorf("%s: blocked=%q", c.filter, p.blocked)
			continue
		}
		if c.blocked {
			continue
		}
		if got := KeysText(suggest(p, sortFields(s))); got != c.want {
			t.Errorf("%s sort %s: got %s, want %s", c.filter, c.sort, got, c.want)
		}
	}
}

func TestExtractFromProfiledCommands(t *testing.T) {
	f, s, ok := extract("command", doc(t, `{ aggregate: "c", pipeline: [ { $match: { a: 1 } }, { $sort: { b: -1 } }, { $group: { _id: "$a" } } ] }`))
	if !ok || mql.Compact(f) != `{"a": 1}` || mql.Compact(s) != `{"b": -1}` {
		t.Fatal(f, s)
	}
	f, _, ok = extract("update", doc(t, `{ q: { a: 2 }, u: { $set: { b: 1 } } }`))
	if !ok || mql.Compact(f) != `{"a": 2}` {
		t.Fatal(f)
	}
	f, _, ok = extract("command", doc(t, `{ count: "c", query: { z: 1 } }`))
	if !ok || mql.Compact(f) != `{"z": 1}` {
		t.Fatal(f)
	}
	if _, _, ok := extract("command", doc(t, `{ insert: "c" }`)); ok {
		t.Fatal("insert analyzed")
	}
}

func ix(name string, keys string, mod func(*indexInfo)) indexInfo {
	d, _ := mql.ParseDocument(keys)
	i := indexInfo{name: name, keys: keyFields(d)}
	if mod != nil {
		mod(&i)
	}
	return i
}

func TestServes(t *testing.T) {
	var p predicates
	classify(bson.D{{Key: "status", Value: "A"}, {Key: "age", Value: bson.D{{Key: "$gt", Value: 5}}}}, &p)
	s := []KeyField{{Field: "created", Dir: -1}}
	for _, c := range []struct {
		keys string
		want int
	}{
		{`{ status: 1, created: -1, age: 1 }`, 2},
		{`{ status: -1, created: 1, age: 1 }`, 2}, // whole sort reversed is fine
		{`{ status: 1, created: -1 }`, 1},         // range field missing
		{`{ status: 1 }`, 1},
		{`{ created: -1 }`, 0},
		{`{ age: 1, status: 1 }`, 0},
	} {
		if got := serves(ix("x", c.keys, nil), p, s); got != c.want {
			t.Errorf("%s: %d, want %d", c.keys, got, c.want)
		}
	}
	if serves(ix("x", `{ status: 1, created: -1, age: 1 }`, func(i *indexInfo) { i.partial = true }), p, s) != 0 {
		t.Error("partial index counted")
	}
}

func TestRedundant(t *testing.T) {
	all := []indexInfo{
		ix("_id_", `{ _id: 1 }`, nil),
		ix("a_1", `{ a: 1 }`, nil),
		ix("a_1_b_1", `{ a: 1, b: 1 }`, nil),
		ix("a_-1", `{ a: -1 }`, nil),
		ix("u_1", `{ u: 1 }`, func(i *indexInfo) { i.unique = true }),
		ix("u_1_v_1", `{ u: 1, v: 1 }`, nil),
		ix("p_1", `{ p: 1 }`, nil),
		ix("p_1_q_1", `{ p: 1, q: 1 }`, func(i *indexInfo) { i.sparse = true }),
	}
	var got []string
	for _, i := range all {
		if by := redundantWith(i, all); by != "" {
			got = append(got, i.name+"<"+by)
		}
	}
	if !reflect.DeepEqual(got, []string{"a_1<a_1_b_1"}) {
		t.Fatal(got)
	}
}

func TestDecide(t *testing.T) {
	var p predicates
	classify(bson.D{{Key: "status", Value: "A"}}, &p)
	a := &shapeAcc{QueryShape: QueryShape{Plans: []string{"COLLSCAN"}, DocsExamined: 1000, Returned: 2}, preds: p}
	decide(a, []indexInfo{ix("_id_", `{ _id: 1 }`, nil)})
	if a.Status != "missing" || a.Suggested != "{ status: 1 }" || a.Name != "status_1" {
		t.Fatalf("%+v", a.QueryShape)
	}
	a = &shapeAcc{preds: p}
	decide(a, []indexInfo{ix("s", `{ status: 1 }`, func(i *indexInfo) { i.hidden = true })})
	if a.Status != "hidden" || a.ServedBy != "s" {
		t.Fatalf("%+v", a.QueryShape)
	}
}
