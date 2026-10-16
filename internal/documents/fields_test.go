package documents

import (
	"math"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

func TestParseTypedPreservesTypes(t *testing.T) {
	oid := bson.NewObjectID()
	cases := []struct {
		typ, text string
		want      any
	}{
		{"string", "  keeps spaces ", "  keeps spaces "},
		{"string", "42", "42"},
		{"int32", " 7 ", int32(7)},
		{"int64", "9007199254740993", int64(9007199254740993)},
		{"double", "5", float64(5)},
		{"double", "-1.5e3", -1500.0},
		{"decimal", "12.340", mustDec("12.340")},
		{"boolean", "TRUE", true},
		{"date", "2024-05-01T12:00:00Z", bson.NewDateTimeFromTime(time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC))},
		{"objectId", oid.Hex(), oid},
		{"document", `{ a: NumberLong(1), b: [1, "x"] }`, bson.D{{Key: "a", Value: int64(1)}, {Key: "b", Value: bson.A{int32(1), "x"}}}},
		{"array", `[1, { c: true }]`, bson.A{int32(1), bson.D{{Key: "c", Value: true}}}},
		{"missing", `"new"`, "new"},
		{"null", `null`, nil},
	}
	for _, c := range cases {
		got, err := ParseTyped(c.typ, c.text)
		if err != nil {
			t.Errorf("%s %q: %v", c.typ, c.text, err)
			continue
		}
		if !mql.Equal(got, c.want) {
			t.Errorf("%s %q = %s (%T), want %s", c.typ, c.text, mql.Compact(got), got, mql.Compact(c.want))
		}
	}
	if v, _ := ParseTyped("double", "NaN"); !math.IsNaN(v.(float64)) {
		t.Fatal("NaN")
	}
}

func mustDec(s string) bson.Decimal128 {
	d, err := bson.ParseDecimal128(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestParseTypedErrorsAreUseful(t *testing.T) {
	cases := map[[2]string]string{
		{"int32", "3000000000"}:   "Out of range for a 32-bit integer",
		{"int32", "1.5"}:          "not a whole number",
		{"int64", "abc"}:          "not a whole number",
		{"double", "1,5"}:         "not a number",
		{"decimal", "x"}:          "not a decimal",
		{"boolean", "yes"}:        "true or false",
		{"date", "yesterday"}:     "not a date",
		{"objectId", "123"}:       "24 hexadecimal",
		{"document", "[1]"}:       "embedded document",
		{"array", "{a:1}"}:        "array",
		{"missing", "bare words"}: "Strings need quotes",
		{"missing", ""}:           "shell syntax",
	}
	for in, want := range cases {
		_, err := ParseTyped(in[0], in[1])
		if !apperr.Is(err, apperr.CodeInvalidInput) || !strings.Contains(err.Error(), want) {
			t.Errorf("%v: want %q, got %v", in, want, err)
		}
	}
}

func TestEditTextRoundTrips(t *testing.T) {
	oid := bson.NewObjectID()
	vals := []any{"x", int32(-3), int64(1 << 60), 2.5, float64(3), true, mustDec("1.10"), bson.NewDateTimeFromTime(time.Date(2020, 1, 2, 3, 4, 5, 6e6, time.UTC)), oid,
		bson.D{{Key: "a", Value: int64(1)}}, bson.A{"q", int32(2)}}
	for _, v := range vals {
		text := EditText(v)
		got, err := ParseTyped(mql.TypeName(v), text)
		if err != nil || !mql.Equal(got, v) {
			t.Errorf("%T %s → %q → %v (%v)", v, mql.Compact(v), text, got, err)
		}
	}
}

func TestPathsAndLookup(t *testing.T) {
	doc := bson.D{{Key: "_id", Value: 1}, {Key: "a", Value: bson.D{{Key: "b", Value: bson.A{int32(10), bson.D{{Key: "c", Value: "deep"}}}}}}}
	if v, ok := Lookup(doc, []string{"a", "b", "1", "c"}); !ok || v != "deep" {
		t.Fatalf("lookup: %v %v", v, ok)
	}
	if _, ok := Lookup(doc, []string{"a", "b", "9"}); ok {
		t.Fatal("out of range index")
	}
	for _, bad := range [][]string{{}, {"_id"}, {"a.b"}, {"$set"}, {"a", ""}} {
		if err := ValidatePath(bad); err == nil {
			t.Errorf("path %q should be rejected", bad)
		}
	}
	if err := checkParents(doc, []string{"a", "b", "0", "x"}); err == nil || !strings.Contains(err.Error(), "not a document") {
		t.Fatalf("scalar parent: %v", err)
	}
	if err := checkParents(doc, []string{"a", "b", "x"}); err == nil {
		t.Fatal("non-numeric key under an array must be rejected")
	}
	if err := checkParents(doc, []string{"a", "new", "field"}); err != nil {
		t.Fatalf("missing parents are created by $set: %v", err)
	}
}
