package mongox

import "go.mongodb.org/mongo-driver/v2/bson"

// UnchangedFilter matches the document with this _id only if it is still
// exactly doc (same fields, order and values), so a write that raced in since
// doc was read makes an update match nothing instead of overwriting it.
//
// $$ROOT goes through $let because MongoDB 4.4 fails to optimize
// {$eq: ["$$ROOT", …]} ("FieldPath::tail() called on single element path");
// the comparison is the same on every version.
func UnchangedFilter(id any, doc any) bson.D {
	return bson.D{
		{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}},
		{Key: "$expr", Value: bson.D{{Key: "$let", Value: bson.D{
			{Key: "vars", Value: bson.D{{Key: "doc", Value: "$$ROOT"}}},
			{Key: "in", Value: bson.D{{Key: "$eq", Value: bson.A{"$$doc", bson.D{{Key: "$literal", Value: doc}}}}}},
		}}}},
	}
}
