package schema

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func sampleReport() Report {
	a := NewAnalyzer()
	a.Add(bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "name", Value: "Ada"}, {Key: "age", Value: int32(36)},
		{Key: "address", Value: bson.D{{Key: "city", Value: "London"}, {Key: "zip", Value: "N1"}}},
		{Key: "items", Value: bson.A{bson.D{{Key: "sku", Value: "a"}, {Key: "qty", Value: int32(1)}}}},
		{Key: "tags", Value: bson.A{"x", "y"}},
	})
	a.Add(bson.D{
		{Key: "_id", Value: bson.NewObjectID()}, {Key: "name", Value: "Bob"}, {Key: "age", Value: int64(41)},
		{Key: "address", Value: bson.D{{Key: "city", Value: "Paris"}}},
		{Key: "items", Value: bson.A{bson.D{{Key: "sku", Value: "b"}}}},
		{Key: "note", Value: nil},
	})
	return a.Report()
}

func get(t *testing.T, d bson.D, path ...string) any {
	t.Helper()
	var cur any = d
	for _, p := range path {
		doc, ok := cur.(bson.D)
		if !ok {
			t.Fatalf("%v: not a document at %q", path, p)
		}
		v, ok := mql.Lookup(doc, p)
		if !ok {
			t.Fatalf("%v: missing %q in %s", path, p, mql.Compact(doc))
		}
		cur = v
	}
	return cur
}

func TestToJSONSchemaRequiredTypesAndNesting(t *testing.T) {
	v := ToJSONSchema(sampleReport(), JSONSchemaOptions{})
	s := get(t, v, "$jsonSchema").(bson.D)

	// Every document has _id, name, age, address and items; tags and note only one.
	if got := mql.Compact(get(t, s, "required")); got != `["_id", "address", "age", "items", "name"]` {
		t.Errorf("required = %s", got)
	}
	if got := mql.Compact(get(t, s, "properties", "age", "bsonType")); got != `["int", "long"]` {
		t.Errorf("mixed numeric types: %s", got)
	}
	if got := get(t, s, "properties", "note", "bsonType"); got != "null" {
		t.Errorf("note: %v", got)
	}
	// Nested document: city is in both addresses, zip in one.
	if got := mql.Compact(get(t, s, "properties", "address", "required")); got != `["city"]` {
		t.Errorf("address.required = %s", got)
	}
	// Arrays: element types, and documents inside arrays described but never required.
	if got := get(t, s, "properties", "tags", "items", "bsonType"); got != "string" {
		t.Errorf("tags items: %v", got)
	}
	items := get(t, s, "properties", "items", "items").(bson.D)
	if _, has := mql.Lookup(items, "required"); has {
		t.Errorf("array elements must not get required: %s", mql.Compact(items))
	}
	if got := get(t, items, "properties", "sku", "bsonType"); got != "string" {
		t.Errorf("items.sku: %v", got)
	}
	if _, has := mql.Lookup(s, "additionalProperties"); has {
		t.Error("unknown fields are allowed unless asked")
	}
}

func TestToJSONSchemaOptions(t *testing.T) {
	v := ToJSONSchema(sampleReport(), JSONSchemaOptions{RequiredPercent: 50, RejectUnknown: true})
	s := get(t, v, "$jsonSchema").(bson.D)
	if got := mql.Compact(get(t, s, "required")); !strings.Contains(got, `"tags"`) || !strings.Contains(got, `"note"`) {
		t.Errorf("at 50%% fields seen in half the documents are required: %s", got)
	}
	if got := get(t, s, "additionalProperties"); got != false {
		t.Errorf("additionalProperties = %v", got)
	}
	if got := get(t, s, "properties", "address", "additionalProperties"); got != false {
		t.Errorf("nested additionalProperties = %v", got)
	}
}
