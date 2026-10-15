package documents

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

func TestWithID(t *testing.T) {
	id := bson.NewObjectID()
	got, err := WithID(bson.D{{Key: "a", Value: 1}, {Key: "_id", Value: id}}, id)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Key != "_id" || len(got) != 2 {
		t.Fatalf("_id must be first: %v", got)
	}
	got, err = WithID(bson.D{{Key: "a", Value: 1}}, id)
	if err != nil || got[0].Value != id {
		t.Fatalf("missing _id must be restored: %v %v", got, err)
	}
	if _, err := WithID(bson.D{{Key: "_id", Value: bson.NewObjectID()}}, id); !apperr.Is(err, apperr.CodeInvalidInput) {
		t.Fatalf("changed _id must be rejected: %v", err)
	}
	// Same numeric value but different BSON type is a different _id.
	if _, err := WithID(bson.D{{Key: "_id", Value: int64(1)}}, int32(1)); err == nil {
		t.Fatal("type change of _id accepted")
	}
}

func TestParseID(t *testing.T) {
	id := bson.NewObjectID()
	v, err := ParseID(mql.Compact(id))
	if err != nil || v != id {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := ParseID(""); err == nil {
		t.Fatal("empty id accepted")
	}
	if _, err := ParseID("ObjectId("); err == nil {
		t.Fatal("bad id accepted")
	}
}

func TestIDFilters(t *testing.T) {
	re := bson.Regex{Pattern: "a"}
	if got := mql.Compact(IDFilter(re)); got != `{"_id": {"$eq": /a/}}` {
		t.Fatalf("IDFilter: %s", got)
	}
	if got := mql.Compact(IDsFilter(bson.A{int32(1), int32(2)})); got != `{"_id": {"$in": [1, 2]}}` {
		t.Fatalf("IDsFilter: %s", got)
	}
	if got := mql.Compact(IDsFilter(bson.A{int32(1), re})); got != `{"$or": [{"_id": {"$eq": 1}}, {"_id": {"$eq": /a/}}]}` {
		t.Fatalf("IDsFilter with regex: %s", got)
	}
}
