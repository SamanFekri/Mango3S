package query

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
)

func TestParse(t *testing.T) {
	p, err := Parse(Request{Filter: `{age: {$gte: 21}}`, Projection: `{name: 1}`, Sort: `{age: -1, name: 1}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Filter) != 1 || len(p.Projection) != 1 || len(p.Sort) != 2 || p.Sort[0].Key != "age" {
		t.Fatalf("unexpected parse: %+v", p)
	}
	p, err = Parse(Request{})
	if err != nil || len(p.Filter) != 0 {
		t.Fatalf("blank request: %+v %v", p, err)
	}
}

func TestParseErrorsNameTheField(t *testing.T) {
	_, err := Parse(Request{Sort: `{a: }`})
	if !apperr.Is(err, apperr.CodeInvalidInput) || !strings.HasPrefix(apperr.From(err).Message, "Sort:") {
		t.Fatalf("got %v", err)
	}
	if _, err := Parse(Request{Limit: -1}); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("negative limit: %v", err)
	}
}

func TestToShell(t *testing.T) {
	got, err := ToShell(Request{Coll: "users", Filter: `{a: 1}`, Projection: `{b: 0}`, Sort: `{c: -1}`, Skip: 5, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	want := "db.users.find({\"a\": 1}, {\"b\": 0})\n  .sort({\"c\": -1})\n  .skip(5)\n  .limit(10)"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if got, _ := ToShell(Request{Coll: "my-coll"}); got != `db.getCollection("my-coll").find({})` {
		t.Fatalf("got %s", got)
	}
}
