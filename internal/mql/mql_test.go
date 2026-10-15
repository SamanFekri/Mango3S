package mql

import (
	"math"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParseRelaxedSyntax(t *testing.T) {
	src := `{
		// comment
		name: 'Ada', "age": 36, /* block */ score: 9.5,
		big: 3000000000, neg: -4, active: true, nothing: null,
		tags: ['a', "b",],
		"nested.path": { $gt: 1 },
	}`
	d, err := ParseDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	want := bson.D{
		{Key: "name", Value: "Ada"},
		{Key: "age", Value: int32(36)},
		{Key: "score", Value: 9.5},
		{Key: "big", Value: int64(3000000000)},
		{Key: "neg", Value: int32(-4)},
		{Key: "active", Value: true},
		{Key: "nothing", Value: nil},
		{Key: "tags", Value: bson.A{"a", "b"}},
		{Key: "nested.path", Value: bson.D{{Key: "$gt", Value: int32(1)}}},
	}
	if !Equal(d, want) {
		t.Fatalf("got %s\nwant %s", Compact(d), Compact(want))
	}
}

func TestParseShellHelpers(t *testing.T) {
	oid := "5f1d7f0b2c3a4b5c6d7e8f90"
	cases := map[string]any{
		`ObjectId("` + oid + `")`:                      mustOID(oid),
		`ISODate("2024-01-31T12:00:00Z")`:              bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)),
		`new Date("2024-01-31")`:                       bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)),
		`ISODate("2024-01-31T12:00:00.123+02:00")`:     bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 10, 0, 0, 123e6, time.UTC)),
		`NumberLong("9007199254740993")`:               int64(9007199254740993),
		`NumberLong(5)`:                                int64(5),
		`NumberInt(7)`:                                 int32(7),
		`NumberDecimal("1.10")`:                        mustDec("1.10"),
		`UUID("123e4567-e89b-12d3-a456-426614174000")`: bson.Binary{Subtype: 4, Data: mustHex("123e4567e89b12d3a456426614174000")},
		`BinData(0, "aGk=")`:                           bson.Binary{Subtype: 0, Data: []byte("hi")},
		`Timestamp(10, 2)`:                             bson.Timestamp{T: 10, I: 2},
		`/^ab\/c[/]/mi`:                                bson.Regex{Pattern: `^ab/c[/]`, Options: "im"},
		`RegExp("x", "i")`:                             bson.Regex{Pattern: "x", Options: "i"},
		`MinKey()`:                                     bson.MinKey{},
		`MaxKey`:                                       bson.MaxKey{},
		`{"$oid": "` + oid + `"}`:                      mustOID(oid),
		`{"$date": "2024-01-31T12:00:00Z"}`:            bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)),
		`{"$date": {"$numberLong": "1706702400000"}}`:  bson.DateTime(1706702400000),
		`{"$numberLong": "42"}`:                        int64(42),
		`{"$numberDecimal": "2.5"}`:                    mustDec("2.5"),
		`"esc\"aped\né\x41"`:                           "esc\"aped\né" + "A",
		`0x1F`:                                         int32(31),
		`1e3`:                                          1000.0,
		`-Infinity`:                                    math.Inf(-1),
		`undefined`:                                    bson.Undefined{},
	}
	for src, want := range cases {
		got, err := ParseValue(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if !Equal(got, want) {
			t.Errorf("%s: got %s (%T), want %s (%T)", src, Compact(got), got, Compact(want), want)
		}
	}
}

func TestQueryOperatorsAreNotTreatedAsExtJSON(t *testing.T) {
	d, err := ParseDocument(`{ name: { $regex: "^a", $options: "i" }, n: { $type: "string" } }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d[0].Value.(bson.D); !ok {
		t.Fatalf("$regex document was converted: %#v", d[0].Value)
	}
	if _, ok := d[1].Value.(bson.D); !ok {
		t.Fatalf("$type document was converted: %#v", d[1].Value)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		`{ a: 1`:                 "end of input",
		`{ a 1 }`:                `expected ':'`,
		`{ a: bogus }`:           "unknown identifier",
		`{ a: ObjectId("zz") }`:  "invalid ObjectId",
		`{ a: 'unterminated }`:   "unterminated string",
		`{ a: 1 } trailing`:      "after value",
		`/abc/g`:                 "'g' is not supported",
		`{ a: ISODate("nope") }`: "invalid date",
		`[1, 2`:                  "end of input",
	}
	for src, want := range cases {
		_, err := ParseValue(src)
		if err == nil {
			t.Errorf("%s: expected error", src)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not contain %q", src, err, want)
		}
	}
}

func TestErrorPosition(t *testing.T) {
	_, err := ParseValue("{\n  a: 1,\n  b: ?\n}")
	se, ok := err.(*SyntaxError)
	if !ok {
		t.Fatalf("want *SyntaxError, got %T", err)
	}
	if se.Line != 3 || se.Col != 6 {
		t.Fatalf("got line %d col %d", se.Line, se.Col)
	}
}

func TestParseDocumentVariants(t *testing.T) {
	d, err := ParseDocument("   // only a comment\n ")
	if err != nil || len(d) != 0 {
		t.Fatalf("blank input: %v %v", d, err)
	}
	if _, err := ParseDocument("[1]"); err == nil {
		t.Fatal("array accepted as document")
	}
	docs, err := ParseDocuments(`[{a: 1}, {b: 2}]`)
	if err != nil || len(docs) != 2 {
		t.Fatalf("ParseDocuments: %v %v", docs, err)
	}
	if _, err := ParseDocuments(`[{a: 1}, 2]`); err == nil {
		t.Fatal("non-document element accepted")
	}
}

func TestRoundTrip(t *testing.T) {
	oid := bson.NewObjectID()
	doc := bson.D{
		{Key: "_id", Value: oid},
		{Key: "i32", Value: int32(1)},
		{Key: "i64", Value: int64(1)},
		{Key: "dbl", Value: 1.0},
		{Key: "frac", Value: 0.1},
		{Key: "tiny", Value: 1.5e-7},
		{Key: "huge", Value: 1e300},
		{Key: "nan", Value: math.NaN()},
		{Key: "str", Value: "quote \" backslash \\ newline \n tab \t ünï"},
		{Key: "date", Value: bson.DateTime(1706702400123)},
		{Key: "dec", Value: mustDec("123.4500")},
		{Key: "uuid", Value: bson.Binary{Subtype: 4, Data: mustHex("00112233445566778899aabbccddeeff")}},
		{Key: "bin", Value: bson.Binary{Subtype: 128, Data: []byte{0, 1, 2}}},
		{Key: "ts", Value: bson.Timestamp{T: 1700000000, I: 3}},
		{Key: "re", Value: bson.Regex{Pattern: `a/b[/]\d`, Options: "i"}},
		{Key: "emptyRe", Value: bson.Regex{Pattern: "", Options: ""}},
		{Key: "null", Value: nil},
		{Key: "undef", Value: bson.Undefined{}},
		{Key: "min", Value: bson.MinKey{}},
		{Key: "max", Value: bson.MaxKey{}},
		{Key: "code", Value: bson.JavaScript("function() { return 1 }")},
		{Key: "arr", Value: bson.A{int32(1), "two", bson.D{{Key: "x", Value: bson.A{}}}, bson.D{}}},
		{Key: "weird key with spaces", Value: true},
		{Key: "$dollar", Value: false},
	}
	for _, indent := range []string{"", "  "} {
		text := Format(doc, indent)
		back, err := ParseDocument(text)
		if err != nil {
			t.Fatalf("indent %q: parse formatted: %v\n%s", indent, err, text)
		}
		if !Equal(doc, back) {
			t.Fatalf("indent %q: round trip mismatch\n%s\n%s", indent, text, Compact(back))
		}
	}
}

func TestFormatShape(t *testing.T) {
	d := bson.D{{Key: "a", Value: int32(1)}, {Key: "b", Value: bson.A{"x"}}, {Key: "c", Value: bson.D{}}}
	if got := Compact(d); got != `{"a": 1, "b": ["x"], "c": {}}` {
		t.Fatalf("compact: %s", got)
	}
	want := "{\n  \"a\": 1,\n  \"b\": [\n    \"x\"\n  ],\n  \"c\": {}\n}"
	if got := Pretty(d); got != want {
		t.Fatalf("pretty:\n%s", got)
	}
	if got := Compact(bson.M{"z": 1, "a": 2}); got != `{"a": 2, "z": 1}` {
		t.Fatalf("map keys must be sorted: %s", got)
	}
}

func mustOID(s string) bson.ObjectID {
	o, err := bson.ObjectIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return o
}

func mustDec(s string) bson.Decimal128 {
	d, err := bson.ParseDecimal128(s)
	if err != nil {
		panic(err)
	}
	return d
}

func mustHex(s string) []byte {
	b := make([]byte, len(s)/2)
	for i := range b {
		var v byte
		for _, c := range s[2*i : 2*i+2] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= byte(c - '0')
			default:
				v |= byte(c-'a') + 10
			}
		}
		b[i] = v
	}
	return b
}
