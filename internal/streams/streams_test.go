package streams

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestEventFromUpdate(t *testing.T) {
	raw := bson.D{
		{Key: "operationType", Value: "update"},
		{Key: "clusterTime", Value: bson.Timestamp{T: 1700000000, I: 1}},
		{Key: "ns", Value: bson.D{{Key: "db", Value: "shop"}, {Key: "coll", Value: "orders"}}},
		{Key: "documentKey", Value: bson.D{{Key: "_id", Value: int32(7)}}},
		{Key: "updateDescription", Value: bson.D{
			{Key: "updatedFields", Value: bson.D{{Key: "status", Value: "shipped"}, {Key: "items.0.qty", Value: int32(2)}}},
			{Key: "removedFields", Value: bson.A{"note"}},
		}},
		{Key: "fullDocument", Value: bson.D{{Key: "_id", Value: int32(7)}, {Key: "status", Value: "shipped"}}},
	}
	e := EventFrom(raw)
	if e.Op != "update" || e.NS != "shop.orders" || e.Key != "7" || e.Time.Unix() != 1700000000 {
		t.Fatalf("%+v", e)
	}
	if len(e.Updated) != 2 || e.Updated[1] != "items.0.qty" || len(e.Removed) != 1 || e.Document == "" {
		t.Fatalf("changes: %+v", e)
	}
}
