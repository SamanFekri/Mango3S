package documents

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// FieldValue is one field prepared for in-place editing.
type FieldValue struct {
	Exists bool   `json:"exists"`
	Type   string `json:"type"` // mql.TypeName, or "missing"
	// Text is what the cell editor shows: the raw string for strings,
	// digits for numbers, ISO-8601 for dates, hex for ObjectIds, and shell
	// syntax for everything else (documents, arrays, binary, …).
	Text string `json:"text"`
	// Expected identifies the value the user saw (exact, shell syntax); it
	// is sent back with the update so concurrent changes are detected.
	Expected string `json:"expected"`
	Editable bool   `json:"editable"`
	Reason   string `json:"reason,omitempty"` // why not editable
}

// FieldUpdate changes or removes one field with $set / $unset.
type FieldUpdate struct {
	ConnID string   `json:"connId"`
	DB     string   `json:"db"`
	Coll   string   `json:"coll"`
	ID     string   `json:"id"`   // _id in shell syntax (model.Document.ID)
	Path   []string `json:"path"` // field names; array indexes as decimal strings
	// Type is the type to store. It is normally the field's current type,
	// so editing never silently changes it; "auto" parses shell syntax.
	Type string `json:"type"`
	Text string `json:"text"`
	// Value, when set, is an exact shell-syntax value used instead of
	// Text+Type (undo/redo restore values this way).
	Value string `json:"value,omitempty"`
	// Expected is FieldValue.Expected from when editing started; with
	// ExpectedMissing the field must still be absent.
	Expected        string `json:"expected"`
	ExpectedMissing bool   `json:"expectedMissing"`
	Unset           bool   `json:"unset"`
}

// FieldResult is the confirmed outcome of a field update.
type FieldResult struct {
	Document model.Document `json:"document"` // the document as stored after the update
	Previous string         `json:"previous"` // previous value (shell syntax) for undo
	Current  string         `json:"current"`  // new value (shell syntax) for redo
	Existed  bool           `json:"existed"`
}

// ValidatePath rejects paths that $set / $unset can't address unambiguously.
func ValidatePath(path []string) error {
	if len(path) == 0 {
		return apperr.Invalid("No field selected.")
	}
	if path[0] == "_id" {
		return apperr.Invalid("_id can't be changed. Insert a copy of the document with a new _id instead.")
	}
	for _, seg := range path {
		switch {
		case seg == "":
			return apperr.Invalid("Empty field names can't be edited in place. Use Edit document (JSON).")
		case strings.Contains(seg, "."):
			return apperr.Invalid("The field name %q contains a dot, which MongoDB update paths can't address. Use Edit document (JSON).", seg)
		case strings.HasPrefix(seg, "$"):
			return apperr.Invalid("The field name %q starts with $, which update operators can't address. Use Edit document (JSON).", seg)
		}
	}
	return nil
}

// Lookup returns the value at path, following array indexes.
func Lookup(doc bson.D, path []string) (any, bool) {
	var cur any = doc
	for _, seg := range path {
		switch v := cur.(type) {
		case bson.D:
			found := false
			for _, e := range v {
				if e.Key == seg {
					cur, found = e.Value, true
					break
				}
			}
			if !found {
				return nil, false
			}
		case bson.A:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(v) {
				return nil, false
			}
			cur = v[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// checkParents makes sure every parent on the path is a document or array
// (with numeric segments only under arrays), so $set can't hit a scalar or
// turn an array index into an object key.
func checkParents(doc bson.D, path []string) error {
	var cur any = doc
	for i, seg := range path[:len(path)-1] {
		switch v := cur.(type) {
		case bson.D:
			next, ok := Lookup(v, []string{seg})
			if !ok {
				return nil // $set creates missing embedded documents
			}
			cur = next
		case bson.A:
			n, err := strconv.Atoi(seg)
			if err != nil || n < 0 || n >= len(v) {
				return apperr.Invalid("%s is not an element of the array %s.", seg, strings.Join(path[:i], "."))
			}
			cur = v[n]
		default:
			return apperr.Invalid("%s is a %s, not a document; it has no field %q.", strings.Join(path[:i], "."), mql.TypeName(cur), seg)
		}
	}
	last := path[len(path)-1]
	switch v := cur.(type) {
	case bson.D:
	case bson.A:
		n, err := strconv.Atoi(last)
		if err != nil || n < 0 || n >= len(v) {
			return apperr.Invalid("Array elements are addressed by index (0–%d).", len(v)-1)
		}
	default:
		return apperr.Invalid("%s is a %s, not a document; it has no field %q.", strings.Join(path[:len(path)-1], "."), mql.TypeName(cur), last)
	}
	return nil
}

// EditText renders a value for the cell editor (see FieldValue.Text).
func EditText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return mql.FormatDouble(t)
	case bson.Decimal128:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case bson.DateTime:
		return t.Time().UTC().Format("2006-01-02T15:04:05.000Z")
	case bson.ObjectID:
		return t.Hex()
	case nil, bson.Null:
		return "null"
	}
	return mql.Pretty(v)
}

// ParseTyped converts editor text into a value of the given type. Errors
// say what is wrong in terms of the type, for display next to the cell.
func ParseTyped(typ, text string) (any, error) {
	trimmed := strings.TrimSpace(text)
	switch typ {
	case "string":
		return text, nil
	case "int32":
		n, err := strconv.ParseInt(trimmed, 10, 32)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return nil, apperr.Invalid("Out of range for a 32-bit integer (−2147483648 to 2147483647). Change the type with Edit document (JSON) to store a larger number.")
			}
			return nil, apperr.Invalid("%q is not a whole number (this field is a 32-bit integer).", trimmed)
		}
		return int32(n), nil
	case "int64":
		n, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return nil, apperr.Invalid("Out of range for a 64-bit integer.")
			}
			return nil, apperr.Invalid("%q is not a whole number (this field is a 64-bit integer).", trimmed)
		}
		return n, nil
	case "double":
		switch trimmed {
		case "NaN":
			return math.NaN(), nil
		case "Infinity", "+Infinity":
			return math.Inf(1), nil
		case "-Infinity":
			return math.Inf(-1), nil
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, apperr.Invalid("%q is not a number (this field is a double).", trimmed)
		}
		return f, nil
	case "decimal":
		d, err := bson.ParseDecimal128(trimmed)
		if err != nil {
			return nil, apperr.Invalid("%q is not a decimal number (this field is a Decimal128).", trimmed)
		}
		return d, nil
	case "boolean":
		switch strings.ToLower(trimmed) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, apperr.Invalid("Enter true or false (this field is a boolean).")
	case "date":
		t, err := mql.ParseDate(trimmed)
		if err != nil {
			return nil, apperr.Invalid("%q is not a date. Use ISO 8601, e.g. 2024-05-01T12:00:00Z.", trimmed)
		}
		return bson.NewDateTimeFromTime(t), nil
	case "objectId":
		id, err := bson.ObjectIDFromHex(trimmed)
		if err != nil {
			return nil, apperr.Invalid("An ObjectId is 24 hexadecimal characters.")
		}
		return id, nil
	}
	// Documents, arrays, binary, regex, timestamps, null/missing ("auto"):
	// exact shell syntax, as in the document editor.
	if trimmed == "" {
		return nil, apperr.Invalid("Enter a value in shell syntax, e.g. \"text\", 42, true, null, { a: 1 } or [1, 2].")
	}
	v, err := mql.ParseValue(trimmed)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Invalid value: "+err.Error()+". Strings need quotes here.")
	}
	if typ == "document" {
		if _, ok := v.(bson.D); !ok {
			return nil, apperr.Invalid("This field is an embedded document; enter { … }. Use Edit document (JSON) to change its type.")
		}
	}
	if typ == "array" {
		if _, ok := v.(bson.A); !ok {
			return nil, apperr.Invalid("This field is an array; enter [ … ]. Use Edit document (JSON) to change its type.")
		}
	}
	return v, nil
}

func describeField(doc bson.D, path []string) FieldValue {
	v, ok := Lookup(doc, path)
	fv := FieldValue{Exists: ok, Editable: true}
	if !ok {
		fv.Type = "missing"
		return fv
	}
	fv.Type = mql.TypeName(v)
	fv.Text = EditText(v)
	fv.Expected = mql.Compact(v)
	if err := ValidatePath(path); err != nil {
		fv.Editable = false
		fv.Reason = apperr.From(err).Message
	}
	return fv
}

func (s *service) fetch(ctx context.Context, c *mongo.Collection, idv any) (bson.D, error) {
	var d bson.D
	if err := c.FindOne(ctx, IDFilter(idv)).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperr.New(apperr.CodeNotFound, "Document not found; it may have been deleted.")
		}
		return nil, apperr.From(err)
	}
	return d, nil
}

// GetField reads one field fresh from the server for editing.
func (s *service) GetField(ctx context.Context, connID, db, coll, id string, path []string) (FieldValue, error) {
	_, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return FieldValue{}, err
	}
	idv, err := ParseID(id)
	if err != nil {
		return FieldValue{}, err
	}
	if len(path) == 0 {
		return FieldValue{}, apperr.Invalid("No field selected.")
	}
	d, err := s.fetch(ctx, c, idv)
	if err != nil {
		return FieldValue{}, err
	}
	return describeField(d, path), nil
}

// UpdateField applies one field change. It succeeds only if the field still
// holds the value the user started from and nothing else changed between
// reading and writing; the rest of the document is never rewritten.
func (s *service) UpdateField(ctx context.Context, u FieldUpdate) (FieldResult, error) {
	sess, c, err := mongox.Collection(s.src, u.ConnID, u.DB, u.Coll)
	if err != nil {
		return FieldResult{}, err
	}
	if err := sess.RequireWritable(); err != nil {
		return FieldResult{}, err
	}
	if err := ValidatePath(u.Path); err != nil {
		return FieldResult{}, err
	}
	idv, err := ParseID(u.ID)
	if err != nil {
		return FieldResult{}, err
	}
	var newVal any
	if !u.Unset {
		if u.Value != "" {
			if newVal, err = mql.ParseValue(u.Value); err != nil {
				return FieldResult{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Invalid value.")
			}
		} else if newVal, err = ParseTyped(u.Type, u.Text); err != nil {
			return FieldResult{}, err
		}
	}

	current, err := s.fetch(ctx, c, idv)
	if err != nil {
		return FieldResult{}, err
	}
	old, existed := Lookup(current, u.Path)
	switch {
	case u.ExpectedMissing && existed:
		return FieldResult{}, apperr.New(apperr.CodeConflict, "Someone added this field since you loaded the page (it is now %s). Reload and try again.", mql.Compact(old))
	case !u.ExpectedMissing:
		want, err := mql.ParseValue(u.Expected)
		if err != nil {
			return FieldResult{}, apperr.Invalid("Missing the original value; reload and try again.")
		}
		if !existed {
			return FieldResult{}, apperr.New(apperr.CodeConflict, "This field was removed since you loaded the page. Reload and try again.")
		}
		if !mql.Equal(old, want) {
			return FieldResult{}, apperr.New(apperr.CodeConflict, "This field changed since you loaded the page (it is now %s). Reload and try again.", truncate(mql.Compact(old), 120))
		}
	}
	if err := checkParents(current, u.Path); err != nil {
		return FieldResult{}, err
	}

	key := strings.Join(u.Path, ".")
	var update bson.D
	if u.Unset {
		if !existed {
			return FieldResult{}, apperr.Invalid("This field doesn't exist.")
		}
		update = bson.D{{Key: "$unset", Value: bson.D{{Key: key, Value: ""}}}}
	} else {
		update = bson.D{{Key: "$set", Value: bson.D{{Key: key, Value: newVal}}}}
	}
	// Match the exact document just read: a write that raced in between
	// makes this match nothing instead of being overwritten.
	filter := bson.D{
		{Key: "_id", Value: bson.D{{Key: "$eq", Value: idv}}},
		{Key: "$expr", Value: bson.D{{Key: "$eq", Value: bson.A{"$$ROOT", bson.D{{Key: "$literal", Value: current}}}}}},
	}
	res, err := c.UpdateOne(ctx, filter, update)
	if err != nil {
		return FieldResult{}, apperr.From(err)
	}
	if res.MatchedCount == 0 {
		return FieldResult{}, apperr.New(apperr.CodeConflict, "The document changed while saving. Reload and try again.")
	}
	after, err := s.fetch(ctx, c, idv)
	if err != nil {
		return FieldResult{}, err
	}
	out := FieldResult{Document: model.NewDocument(after), Existed: existed}
	if existed {
		out.Previous = mql.Compact(old)
	}
	if v, ok := Lookup(after, u.Path); ok {
		out.Current = mql.Compact(v)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
