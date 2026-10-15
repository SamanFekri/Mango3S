package transfer

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// dataset returns n documents covering every commonly used BSON type.
func dataset(n int) []bson.D {
	out := make([]bson.D, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range out {
		dec, _ := bson.ParseDecimal128(fmt.Sprintf("%d.%02d", i, i%100))
		out[i] = bson.D{
			{Key: "_id", Value: bson.NewObjectID()},
			{Key: "i32", Value: int32(i)},
			{Key: "i64", Value: int64(i) * 10_000_000_000},
			{Key: "dbl", Value: float64(i) + 0.5},
			{Key: "whole", Value: float64(i)},
			{Key: "dec", Value: dec},
			{Key: "str", Value: fmt.Sprintf("name, \"quoted\" %d\nline2", i)},
			{Key: "ok", Value: i%2 == 0},
			{Key: "when", Value: bson.NewDateTimeFromTime(base.Add(time.Duration(i) * time.Hour))},
			{Key: "nested", Value: bson.D{{Key: "city", Value: "Paris"}, {Key: "zip", Value: int32(75000 + i)}}},
			{Key: "tags", Value: bson.A{"a", int32(i)}},
			{Key: "nil", Value: nil},
			{Key: "bin", Value: bson.Binary{Subtype: 0, Data: []byte{byte(i), 1, 2}}},
			{Key: "re", Value: bson.Regex{Pattern: "^x", Options: "i"}},
		}
	}
	return out
}

func writeAll(t *testing.T, w Writer, docs []bson.D) {
	t.Helper()
	for _, d := range docs {
		if err := w.Write(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func readAll(t *testing.T, r Reader) []bson.D {
	t.Helper()
	var out []bson.D
	for {
		d, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("%s: %v", r.Position(), err)
		}
		out = append(out, d)
	}
}

func TestJSONRoundTripPreservesTypes(t *testing.T) {
	docs := dataset(500)
	for _, mode := range []string{JSONArray, JSONLines} {
		for _, pretty := range []bool{false, true} {
			var buf bytes.Buffer
			writeAll(t, NewJSONWriter(&buf, mode, true, pretty), docs)
			r, err := NewJSONReader(&buf)
			if err != nil {
				t.Fatal(err)
			}
			back := readAll(t, r)
			if len(back) != len(docs) {
				t.Fatalf("%s pretty=%v: %d docs back", mode, pretty, len(back))
			}
			for i := range docs {
				if !mql.Equal(docs[i], back[i]) {
					t.Fatalf("%s pretty=%v doc %d differs:\n%s\n%s", mode, pretty, i, mql.Compact(docs[i]), mql.Compact(back[i]))
				}
			}
		}
	}
}

func TestRelaxedJSONIsReadable(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSONWriter(&buf, JSONLines, false, false)
	writeAll(t, w, dataset(3))
	if strings.Contains(buf.String(), "$numberInt") {
		t.Fatal("relaxed output should use plain numbers")
	}
	r, _ := NewJSONReader(&buf)
	back := readAll(t, r)
	if v, _ := mql.Lookup(back[1], "when"); mql.TypeName(v) != "date" {
		t.Fatalf("dates must survive relaxed mode, got %T", v)
	}
}

func TestJSONReaderLayouts(t *testing.T) {
	cases := map[string]int{
		"\xEF\xBB\xBF[{\"a\":1},{\"a\":2}]":   2,
		"{\"a\":1}\n{\"a\":2}\n\n{\"a\":3}\n": 3,
		"{\"a\":1}{\"a\":2}":                  2,
		"[]":                                  0,
		"":                                    0,
		"  \n [ {\"_id\": {\"$oid\": \"65a1f0c2e4b0a1b2c3d4e5f6\"}} ]": 1,
	}
	for src, want := range cases {
		r, err := NewJSONReader(strings.NewReader(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if got := len(readAll(t, r)); got != want {
			t.Errorf("%q: got %d docs, want %d", src, got, want)
		}
	}
}

func TestJSONReaderErrors(t *testing.T) {
	if _, err := NewJSONReader(strings.NewReader("hello")); err == nil {
		t.Fatal("non-JSON accepted")
	}
	for _, src := range []string{`[{"a":1}, 5]`, `{"a":1}{"a":`, `[{"a":1}`, `{"a": {"$oid": "nope"}}`} {
		r, err := NewJSONReader(strings.NewReader(src))
		if err != nil {
			continue
		}
		var lastErr error
		for {
			_, err := r.Next()
			if err != nil {
				lastErr = err
				break
			}
		}
		if lastErr == io.EOF {
			t.Errorf("%q: expected an error", src)
		}
	}
}

func TestCSVReaderTypesAndNesting(t *testing.T) {
	src := "name,age,score,big,active,joined,id,address.city,address.zip,tags,notes,skipme\n" +
		"Ada,36,9.5,3000000000,true,2024-01-31T10:00:00Z,65a1f0c2e4b0a1b2c3d4e5f6,London,00123,\"[1, 2]\",,x\n" +
		"\n" +
		"\"Smith, J\",,1e3,1,false,2024-01-31,abc,Paris,75001,plain,\"multi\nline\",y\n"
	r, err := NewCSVReader(strings.NewReader(src), CSVOptions{
		Columns: []ColumnSpec{{Name: "address.zip", Type: TypeString}, {Name: "skipme", Skip: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	docs := readAll(t, r)
	if len(docs) != 2 {
		t.Fatalf("got %d docs", len(docs))
	}
	want := `{"name": "Ada", "age": 36, "score": 9.5, "big": NumberLong(3000000000), "active": true, ` +
		`"joined": ISODate("2024-01-31T10:00:00.000Z"), "id": ObjectId("65a1f0c2e4b0a1b2c3d4e5f6"), ` +
		`"address": {"city": "London", "zip": "00123"}, "tags": [1, 2]}`
	if got := mql.Compact(docs[0]); got != want {
		t.Fatalf("row 1:\n got %s\nwant %s", got, want)
	}
	want = `{"name": "Smith, J", "score": 1000.0, "big": 1, "active": false, "joined": "2024-01-31", "id": "abc", ` +
		`"address": {"city": "Paris", "zip": "75001"}, "tags": "plain", "notes": "multi\nline"}`
	if got := mql.Compact(docs[1]); got != want {
		t.Fatalf("row 2:\n got %s\nwant %s", got, want)
	}
}

func TestCSVReaderErrors(t *testing.T) {
	if _, err := NewCSVReader(strings.NewReader(""), CSVOptions{}); err == nil {
		t.Fatal("empty file accepted")
	}
	if _, err := NewCSVReader(strings.NewReader("a,a\n"), CSVOptions{}); err == nil {
		t.Fatal("duplicate header accepted")
	}
	if _, err := NewCSVReader(strings.NewReader("a\n"), CSVOptions{Columns: []ColumnSpec{{Name: "a", Type: "uuid"}}}); err == nil {
		t.Fatal("unknown type accepted")
	}
	r, _ := NewCSVReader(strings.NewReader("a,b\n1,2,3\n"), CSVOptions{})
	if _, err := r.Next(); err == nil || r.Position() != "row 2" {
		t.Fatalf("extra cells: %v at %s", err, r.Position())
	}
	r, _ = NewCSVReader(strings.NewReader("n\nabc\n"), CSVOptions{Columns: []ColumnSpec{{Name: "n", Type: TypeInt}}})
	if _, err := r.Next(); err == nil || !strings.Contains(err.Error(), "not a 32-bit integer") {
		t.Fatalf("bad int: %v", err)
	}
	r, _ = NewCSVReader(strings.NewReader("a,a.b\n1,2\n"), CSVOptions{})
	if _, err := r.Next(); err == nil {
		t.Fatal("conflicting paths accepted")
	}
}

func TestCSVTSVAndBlanks(t *testing.T) {
	r, err := NewCSVReader(strings.NewReader("a\tb\nx\t\n"), CSVOptions{Delimiter: '\t', KeepBlanks: true})
	if err != nil {
		t.Fatal(err)
	}
	d, _ := r.Next()
	if mql.Compact(d) != `{"a": "x", "b": ""}` {
		t.Fatalf("got %s", mql.Compact(d))
	}
}

func TestCSVExportImportRoundTrip(t *testing.T) {
	docs := dataset(200)
	fields := []string{"_id", "i32", "dbl", "str", "ok", "when", "nested.city", "nested.zip", "tags", "missing"}
	var buf bytes.Buffer
	w, err := NewCSVWriter(&buf, fields, ';', true)
	if err != nil {
		t.Fatal(err)
	}
	writeAll(t, w, docs)
	if !strings.HasPrefix(buf.String(), "_id;i32;dbl;str;ok;when;nested.city;nested.zip;tags;missing\n") {
		t.Fatalf("header: %q", buf.String()[:80])
	}
	r, err := NewCSVReader(&buf, CSVOptions{Delimiter: ';', Columns: []ColumnSpec{{Name: "str", Type: TypeString}}})
	if err != nil {
		t.Fatal(err)
	}
	back := readAll(t, r)
	if len(back) != len(docs) {
		t.Fatalf("got %d rows", len(back))
	}
	for i, d := range back {
		for _, f := range []string{"_id", "i32", "str", "ok", "when", "nested.city", "nested.zip"} {
			a, _ := LookupPath(docs[i], f)
			b, _ := LookupPath(d, f)
			if !mql.Equal(a, b) {
				t.Fatalf("doc %d field %s: %s vs %s", i, f, mql.Compact(a), mql.Compact(b))
			}
		}
		// Integral doubles come back as integers: CSV cannot carry the double type.
		a, _ := LookupPath(docs[i], "dbl")
		b, _ := LookupPath(d, "dbl")
		if bf, ok := b.(float64); ok && bf != a.(float64) {
			t.Fatalf("dbl changed value: %v vs %v", a, b)
		}
		if _, ok := LookupPath(d, "missing"); ok {
			t.Fatal("blank cell for a missing field should not create the field")
		}
	}
}

func TestFormatCell(t *testing.T) {
	oid, _ := bson.ObjectIDFromHex("65a1f0c2e4b0a1b2c3d4e5f6")
	cases := map[string]any{
		"":                         nil,
		"65a1f0c2e4b0a1b2c3d4e5f6": oid,
		"2024-01-31T10:00:00.000Z": bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC)),
		"0.1":                      0.1,
		"NaN":                      math.NaN(),
		`[1,"x"]`:                  bson.A{int32(1), "x"},
		`{"a":{"$oid":"65a1f0c2e4b0a1b2c3d4e5f6"}}`: bson.D{{Key: "a", Value: oid}},
		"AQI=": bson.Binary{Data: []byte{1, 2}},
	}
	for want, v := range cases {
		if got := FormatCell(v); got != want {
			t.Errorf("FormatCell(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestInferAndMerge(t *testing.T) {
	cases := map[string]string{
		"12": TypeInt, "-3000000000": TypeLong, "1.5": TypeDouble, "1e9": TypeDouble, "true": TypeBool,
		"65a1f0c2e4b0a1b2c3d4e5f6": TypeObjectID, "2024-01-31T10:00:00+02:00": TypeDate, "2024-01-31": TypeString,
		"[1,2]": TypeJSON, "[not json": TypeString, "hello": TypeString, " ": "",
		"99999999999999999999999": TypeString,
	}
	for in, want := range cases {
		if got := InferType(in); got != want {
			t.Errorf("InferType(%q) = %q, want %q", in, got, want)
		}
	}
	if mergeTypes(map[string]bool{TypeInt: true, TypeDouble: true}) != TypeDouble {
		t.Error("int+double should merge to double")
	}
	if mergeTypes(map[string]bool{TypeInt: true, TypeString: true}) != TypeAuto {
		t.Error("mixed types should merge to auto")
	}
	if mergeTypes(map[string]bool{}) != TypeString {
		t.Error("empty column should be string")
	}
}

func TestFieldsProjection(t *testing.T) {
	got := mql.Compact(fieldsProjection([]string{"a", "a.b", "c"}))
	if got != `{"a": 1, "c": 1, "_id": 0}` {
		t.Fatalf("got %s", got)
	}
	if got := mql.Compact(fieldsProjection([]string{"_id", "x"})); got != `{"_id": 1, "x": 1}` {
		t.Fatalf("got %s", got)
	}
}

func TestParseDelimiter(t *testing.T) {
	for in, want := range map[string]rune{"": ',', ";": ';', "tab": '\t', `\t`: '\t', "|": '|'} {
		if got, err := ParseDelimiter(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{`"`, ";;", "\n"} {
		if _, err := ParseDelimiter(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestPreviewFile(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "people.csv")
	_ = os.WriteFile(csvPath, []byte("name,age,score\nAda,36,1\nBob,x,2.5\nCy,40,3\n"), 0o600)
	p, err := PreviewFile(ImportOptions{Path: csvPath}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Format != FormatCSV || len(p.Documents) != 2 || len(p.Columns) != 3 {
		t.Fatalf("preview: %+v", p)
	}
	if p.Columns[1].Inferred != TypeAuto || p.Columns[2].Inferred != TypeDouble || p.Columns[0].Inferred != TypeString {
		t.Fatalf("inferred: %+v", p.Columns)
	}

	jsonPath := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(jsonPath, []byte("{\"a\":1}\n{\"a\":\n"), 0o600)
	p, err = PreviewFile(ImportOptions{Path: jsonPath}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Documents) != 1 || p.Error == "" {
		t.Fatalf("partial preview: %+v", p)
	}
	if _, err := PreviewFile(ImportOptions{Path: filepath.Join(dir, "missing.json")}, 1); err == nil {
		t.Fatal("missing file accepted")
	}
}
