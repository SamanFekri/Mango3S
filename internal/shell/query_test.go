package shell

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
)

func TestParseQueryForms(t *testing.T) {
	q, err := ParseQuery(`db.getCollection("agent_definitions").find({})`, "shop")
	if err != nil || q.Kind != "find" || q.Coll != "agent_definitions" || q.DB != "shop" || q.Filter != "{}" {
		t.Fatalf("%+v %v", q, err)
	}
	q, err = ParseQuery(`db.orders.find({ status: "A", qty: { $gt: NumberLong(5) } }, { _id: 0, status: 1 })
  .sort({ qty: -1 })
  .skip(10)
  .limit(25)`, "shop")
	if err != nil {
		t.Fatal(err)
	}
	if q.Coll != "orders" || !strings.Contains(q.Filter, "NumberLong(5)") || !strings.Contains(q.Projection, "status") ||
		!strings.Contains(q.Sort, "qty") || q.Skip != 10 || q.Limit != 25 {
		t.Fatalf("%+v", q)
	}
	q, err = ParseQuery(`db.getSiblingDB("hr").getCollection("staff").aggregate([{ $match: { active: true } }, { $count: "n" }])`, "shop")
	if err != nil || q.Kind != "aggregate" || q.DB != "hr" || q.Coll != "staff" || len(q.Stages) != 2 || q.Stages[1].Operator != "$count" || q.Stages[1].Body != `"n"` {
		t.Fatalf("%+v %v", q, err)
	}
	q, err = ParseQuery(`db.system.views.find().pretty()`, "shop")
	if err != nil || q.Coll != "system.views" {
		t.Fatalf("dotted names: %+v %v", q, err)
	}
}

func TestParseQueryRejectsWithGuidance(t *testing.T) {
	cases := map[string]string{
		``:                                  "Enter a query",
		`show dbs`:                          "Shell",
		`db.orders.find(); db.x.find()`:     "one query at a time",
		`db.orders.deleteMany({})`:          "Only find() and aggregate()",
		`db.orders.find().count()`:          "Count or Explain",
		`db.orders.find().foo()`:            "not supported",
		`db.getCollection("o")`:             "Add .find",
		`db.orders.aggregate([{ a: 1 }])`:   "one $operator",
		`db.orders.aggregate([]).toArray()`: "inside the pipeline",
		`db.orders.find({ a: `:              "",
	}
	for in, want := range cases {
		_, err := ParseQuery(in, "shop")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want %q, got %v", in, want, err)
		}
		if err != nil && !apperr.Is(err, apperr.CodeInvalidInput) && !apperr.Is(err, apperr.CodeUnsupported) {
			t.Errorf("%q: unexpected code %v", in, err)
		}
	}
}
