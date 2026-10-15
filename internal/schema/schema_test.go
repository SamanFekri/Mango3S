package schema

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func field(t *testing.T, r Report, path string) Field {
	t.Helper()
	for _, f := range r.Fields {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("field %q not in report", path)
	return Field{}
}

func typeCount(f Field, typ string) int64 {
	for _, ts := range f.Types {
		if ts.Type == typ {
			return ts.Count
		}
	}
	return 0
}

func TestAnalyzer(t *testing.T) {
	a := NewAnalyzer()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		d := bson.D{
			{Key: "_id", Value: int32(i)},
			{Key: "name", Value: []string{"ab", "abcd"}[i%2]},
			{Key: "when", Value: bson.NewDateTimeFromTime(base.Add(time.Duration(i) * 24 * time.Hour))},
			{Key: "items", Value: bson.A{bson.D{{Key: "sku", Value: "x"}, {Key: "qty", Value: int32(i)}}, bson.D{{Key: "sku", Value: "y"}}}},
			{Key: "address", Value: bson.D{{Key: "city", Value: "Paris"}}},
		}
		switch {
		case i < 50:
			d = append(d, bson.E{Key: "score", Value: int32(i)})
		case i < 75:
			d = append(d, bson.E{Key: "score", Value: float64(i) + 0.5})
		case i < 90:
			d = append(d, bson.E{Key: "score", Value: nil})
		}
		if i%4 == 0 {
			d = append(d, bson.E{Key: "tags", Value: bson.A{"a", int32(1)}})
		}
		a.Add(d)
	}
	r := a.Report()
	if r.Sampled != 100 {
		t.Fatalf("sampled %d", r.Sampled)
	}
	if r.Fields[0].Path != "_id" {
		t.Fatalf("_id must come first, got %s", r.Fields[0].Path)
	}

	id := field(t, r, "_id")
	if id.Presence != 100 || !id.Unique || id.Min != "0" || id.Max != "99" || id.Distinct != 100 {
		t.Fatalf("_id: %+v", id)
	}

	score := field(t, r, "score")
	if score.Count != 90 || score.Presence != 90 {
		t.Fatalf("score presence: %+v", score)
	}
	if typeCount(score, "int32") != 50 || typeCount(score, "double") != 25 || typeCount(score, "null") != 15 {
		t.Fatalf("score types: %+v", score.Types)
	}
	if score.Types[0].Type != "int32" || score.Types[0].Percent != 55.6 {
		t.Fatalf("score type order/percent: %+v", score.Types)
	}
	if score.Min != "0" || score.Max != "74.5" {
		t.Fatalf("score range: %s..%s", score.Min, score.Max)
	}

	name := field(t, r, "name")
	if *name.MinLength != 2 || *name.MaxLength != 4 || *name.AvgLength != 3 || name.Unique || name.Distinct != 2 {
		t.Fatalf("name: %+v", name)
	}

	when := field(t, r, "when")
	if when.Min != `ISODate("2024-01-01T00:00:00.000Z")` || when.Max != `ISODate("2024-04-09T00:00:00.000Z")` {
		t.Fatalf("when range: %s..%s", when.Min, when.Max)
	}

	items := field(t, r, "items")
	if typeCount(items, "array") != 100 || items.ElementTypes[0].Type != "document" || *items.MinLength != 2 {
		t.Fatalf("items: %+v", items)
	}
	sku := field(t, r, "items.sku")
	if sku.Count != 100 || typeCount(sku, "string") != 200 || sku.Depth != 1 || sku.Name != "sku" {
		t.Fatalf("items.sku: %+v", sku)
	}
	if qty := field(t, r, "items.qty"); qty.Count != 100 || qty.Presence != 100 {
		t.Fatalf("items.qty: %+v", qty)
	}

	tags := field(t, r, "tags")
	if tags.Presence != 25 || len(tags.ElementTypes) != 2 || tags.ElementTypes[0].Percent != 50 {
		t.Fatalf("tags: %+v", tags)
	}
	if city := field(t, r, "address.city"); city.Samples[0] != `"Paris"` || city.Distinct != 1 {
		t.Fatalf("address.city: %+v", city)
	}
	if addr := field(t, r, "address"); typeCount(addr, "document") != 100 {
		t.Fatalf("address: %+v", addr)
	}

	// Parents sort before children; siblings alphabetically.
	var paths []string
	for _, f := range r.Fields {
		paths = append(paths, f.Path)
	}
	want := []string{"_id", "address", "address.city", "items", "items.qty", "items.sku", "name", "score", "tags", "when"}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("order: %v", paths)
		}
	}
}

func TestSamplesAndCap(t *testing.T) {
	a := NewAnalyzer()
	for i := 0; i < distinctCap+10; i++ {
		a.Add(bson.D{{Key: "n", Value: int64(i)}})
	}
	f := a.Report().Fields[0]
	if !f.DistinctCapped || f.Distinct != distinctCap || f.Unique || len(f.Samples) != sampleCap {
		t.Fatalf("cap: %+v", f)
	}
	if f.Min != "NumberLong(0)" {
		t.Fatalf("min keeps original type: %s", f.Min)
	}
}

func TestDecimalRange(t *testing.T) {
	a := NewAnalyzer()
	for _, s := range []string{"10.5", "-2.25", "3"} {
		d, _ := bson.ParseDecimal128(s)
		a.Add(bson.D{{Key: "p", Value: d}})
	}
	f := a.Report().Fields[0]
	if f.Min != `NumberDecimal("-2.25")` || f.Max != `NumberDecimal("10.5")` {
		t.Fatalf("decimal range: %s..%s", f.Min, f.Max)
	}
}

func TestEmpty(t *testing.T) {
	r := NewAnalyzer().Report()
	if r.Sampled != 0 || len(r.Fields) != 0 {
		t.Fatalf("%+v", r)
	}
}
