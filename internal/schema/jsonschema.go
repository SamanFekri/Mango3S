package schema

import (
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// JSONSchemaOptions controls how a validator is built from a report.
type JSONSchemaOptions struct {
	// RequiredPercent: a field is required when it appears in at least this
	// percent of the documents (or of its parent document's occurrences).
	// 0 means 100 — only fields every sampled document has.
	RequiredPercent float64 `json:"requiredPercent"`
	// RejectUnknown adds additionalProperties: false, so fields that weren't
	// seen in the sample are rejected. _id is always allowed.
	RejectUnknown bool `json:"rejectUnknown"`
}

// bsonTypeAlias maps the analyzer's type names to $jsonSchema bsonType aliases.
var bsonTypeAlias = map[string]string{
	"null": "null", "undefined": "undefined", "boolean": "bool", "int32": "int", "int64": "long",
	"double": "double", "string": "string", "objectId": "objectId", "date": "date",
	"decimal": "decimal", "binary": "binData", "timestamp": "timestamp", "regex": "regex",
	"document": "object", "array": "array", "minKey": "minKey", "maxKey": "maxKey",
	"javascript": "javascript",
}

type node struct {
	field    *Field
	children map[string]*node
	order    []string
}

func (n *node) child(name string) *node {
	if n.children == nil {
		n.children = map[string]*node{}
	}
	c, ok := n.children[name]
	if !ok {
		c = &node{}
		n.children[name] = c
		n.order = append(n.order, name)
	}
	return c
}

// ToJSONSchema builds a { $jsonSchema: … } validator from the observed types
// and presence. Nested documents and arrays (of documents) are described
// recursively. Inside arrays nothing is marked required, because the report
// counts documents, not array elements, so the generated rule can't reject a
// document that's already in the sample for that reason.
func ToJSONSchema(r Report, o JSONSchemaOptions) bson.D {
	threshold := o.RequiredPercent
	if threshold <= 0 || threshold > 100 {
		threshold = 100
	}
	root := &node{}
	for i := range r.Fields {
		f := &r.Fields[i]
		n := root
		for _, part := range strings.Split(f.Path, ".") {
			n = n.child(part)
		}
		n.field = f
	}
	schema := objectSchema(root, float64(r.Sampled), threshold, o.RejectUnknown, false, true)
	return bson.D{{Key: "$jsonSchema", Value: schema}}
}

// objectSchema describes the children of n as an object. total is how many
// times the object occurred (documents for the root).
func objectSchema(n *node, total, threshold float64, rejectUnknown, inArray, root bool) bson.D {
	props := bson.D{}
	var required []string
	for _, name := range n.order {
		c := n.children[name]
		if c.field == nil {
			continue
		}
		props = append(props, bson.E{Key: name, Value: fieldSchema(c, threshold, rejectUnknown, inArray)})
		if !inArray && total > 0 && float64(c.field.Count)/total*100 >= threshold {
			required = append(required, name)
		}
	}
	out := bson.D{{Key: "bsonType", Value: "object"}}
	if len(required) > 0 {
		sort.Strings(required)
		req := bson.A{}
		for _, r := range required {
			req = append(req, r)
		}
		out = append(out, bson.E{Key: "required", Value: req})
	}
	if rejectUnknown {
		if root {
			if _, ok := n.children["_id"]; !ok {
				props = append(bson.D{{Key: "_id", Value: bson.D{}}}, props...)
			}
		}
		out = append(out, bson.E{Key: "additionalProperties", Value: false})
	}
	return append(out, bson.E{Key: "properties", Value: props})
}

func fieldSchema(n *node, threshold float64, rejectUnknown, inArray bool) bson.D {
	f := n.field
	types := aliases(f.Types)
	out := bson.D{{Key: "bsonType", Value: oneOrMany(types)}}
	hasChildren := len(n.children) > 0
	if hasChildren && contains(types, "object") {
		obj := objectSchema(n, float64(f.Count), threshold, rejectUnknown, inArray, false)
		// Merge the object description (required, properties…) into this field.
		out = append(out, obj[1:]...)
	}
	if contains(types, "array") && len(f.ElementTypes) > 0 {
		elem := aliases(f.ElementTypes)
		items := bson.D{{Key: "bsonType", Value: oneOrMany(elem)}}
		if hasChildren && contains(elem, "object") {
			obj := objectSchema(n, float64(f.Count), threshold, rejectUnknown, true, false)
			items = append(items, obj[1:]...)
		}
		out = append(out, bson.E{Key: "items", Value: items})
	}
	return out
}

func aliases(ts []TypeStat) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range ts {
		a, ok := bsonTypeAlias[t.Type]
		if !ok || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func oneOrMany(ts []string) any {
	if len(ts) == 1 {
		return ts[0]
	}
	a := bson.A{}
	for _, t := range ts {
		a = append(a, t)
	}
	return a
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
