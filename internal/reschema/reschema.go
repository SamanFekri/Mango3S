// Package reschema restructures documents: rename, remove, set, copy,
// convert types, flatten sub-documents and nest fields. Operations are pure
// functions over a document, so the preview runs exactly the code that the
// apply runs (see package transform for the collection-level workflow).
package reschema

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/docpath"
	"mango3s/internal/mql"
)

// Operation types.
const (
	OpRename  = "rename"
	OpRemove  = "remove"
	OpSet     = "set"
	OpCopy    = "copy"
	OpConvert = "convert"
	OpFlatten = "flatten"
	OpNest    = "nest"
)

// Op is one step. Which fields matter depends on Type.
type Op struct {
	Type          string   `json:"type"`
	Field         string   `json:"field"`         // rename, remove, set, copy, convert, flatten
	To            string   `json:"to"`            // rename, copy, nest (target path)
	Value         string   `json:"value"`         // set: shell-syntax literal
	OnlyIfMissing bool     `json:"onlyIfMissing"` // set
	TargetType    string   `json:"targetType"`    // convert
	Separator     string   `json:"separator"`     // flatten (default "_")
	Fields        []string `json:"fields"`        // nest
}

// Convertible target types.
var TargetTypes = []string{"string", "int", "long", "double", "decimal", "bool", "date", "objectId"}

// Compiled is a validated list of operations.
type Compiled struct {
	ops    []Op
	values []any
}

// Compile validates the operations and parses literals once.
func Compile(ops []Op) (*Compiled, error) {
	if len(ops) == 0 {
		return nil, apperr.Invalid("Add at least one operation.")
	}
	c := &Compiled{ops: ops, values: make([]any, len(ops))}
	for i, op := range ops {
		label := fmt.Sprintf("Step %d (%s)", i+1, op.Type)
		need := func(cond bool, msg string) error {
			if !cond {
				return apperr.Invalid("%s: %s", label, msg)
			}
			return nil
		}
		var err error
		switch op.Type {
		case OpRename, OpCopy:
			err = need(op.Field != "" && op.To != "", "needs a field and a new name")
			if err == nil {
				err = need(op.Field != op.To && !strings.HasPrefix(op.To, op.Field+"."), "the new path must differ from and not be inside the field")
			}
		case OpRemove, OpFlatten:
			err = need(op.Field != "", "needs a field")
		case OpSet:
			if err = need(op.Field != "", "needs a field"); err == nil {
				v, perr := mql.ParseValue(op.Value)
				if perr != nil {
					return nil, apperr.Invalid("%s: value: %v", label, perr)
				}
				c.values[i] = v
			}
		case OpConvert:
			err = need(op.Field != "" && contains(TargetTypes, op.TargetType), "needs a field and one of "+strings.Join(TargetTypes, ", "))
		case OpNest:
			err = need(len(op.Fields) > 0 && op.To != "", "needs fields and a target sub-document")
			for _, f := range op.Fields {
				if err == nil {
					err = need(!strings.Contains(f, ".") && f != op.To, "nest moves top-level fields; "+f+" is not one")
				}
			}
		default:
			err = apperr.Invalid("%s: unknown operation", label)
		}
		if err != nil {
			return nil, err
		}
		if op.Field == "_id" && op.Type != OpCopy && op.Type != OpConvert {
			return nil, apperr.Invalid("%s: _id cannot be renamed, removed or replaced.", label)
		}
		if op.Type == OpConvert && op.Field == "_id" {
			return nil, apperr.Invalid("%s: converting _id would create a different document; copy it to another field first.", label)
		}
	}
	return c, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Apply transforms a copy of d. It reports whether anything changed.
func (c *Compiled) Apply(d bson.D) (bson.D, bool, error) {
	out := docpath.CloneDoc(d)
	var err error
	for i, op := range c.ops {
		out, err = c.apply(out, i, op)
		if err != nil {
			return nil, false, fmt.Errorf("step %d (%s %s): %w", i+1, op.Type, op.Field, err)
		}
	}
	return out, !mql.Equal(out, d), nil
}

func (c *Compiled) apply(d bson.D, i int, op Op) (bson.D, error) {
	switch op.Type {
	case OpRemove:
		d, _, _ = docpath.Delete(d, op.Field)
	case OpSet:
		if op.OnlyIfMissing && docpath.Exists(d, op.Field) {
			return d, nil
		}
		nd, ok := docpath.Set(d, op.Field, docpath.Clone(c.values[i]))
		if !ok {
			return d, fmt.Errorf("a parent of %s is not a document", op.Field)
		}
		d = nd
	case OpRename:
		v, found := docpath.Get(d, op.Field)
		if !found {
			return d, nil
		}
		if docpath.Exists(d, op.To) {
			return d, fmt.Errorf("%s already exists", op.To)
		}
		fp, fl := docpath.Parent(op.Field)
		tp, tl := docpath.Parent(op.To)
		if fp == tp { // same parent: rename in place, keeping field position
			return renameInPlace(d, fp, fl, tl), nil
		}
		d, _, _ = docpath.Delete(d, op.Field)
		nd, ok := docpath.Set(d, op.To, v)
		if !ok {
			return d, fmt.Errorf("a parent of %s is not a document", op.To)
		}
		d = nd
	case OpCopy:
		v, found := docpath.Get(d, op.Field)
		if !found {
			return d, nil
		}
		nd, ok := docpath.Set(d, op.To, docpath.Clone(v))
		if !ok {
			return d, fmt.Errorf("a parent of %s is not a document", op.To)
		}
		d = nd
	case OpConvert:
		var err error
		d, _, err = docpath.Update(d, op.Field, func(v any) (any, bool, error) {
			nv, err := Convert(v, op.TargetType)
			return nv, true, err
		})
		return d, err
	case OpFlatten:
		sep := op.Separator
		if sep == "" {
			sep = "_"
		}
		parent, leaf := docpath.Parent(op.Field)
		return flatten(d, parent, leaf, sep)
	case OpNest:
		return nest(d, op.Fields, op.To)
	}
	return d, nil
}

func renameInPlace(d bson.D, parent, from, to string) bson.D {
	if parent == "" {
		for i := range d {
			if d[i].Key == from {
				d[i].Key = to
			}
		}
		return d
	}
	d, _, _ = docpath.Update(d, parent, func(v any) (any, bool, error) {
		if sub, ok := v.(bson.D); ok {
			return renameInPlace(sub, "", from, to), true, nil
		}
		return v, true, nil
	})
	return d
}

// flatten replaces sub-document leaf of parent with its leaves joined by sep.
func flatten(d bson.D, parent, leaf, sep string) (bson.D, error) {
	doFlatten := func(container bson.D) (bson.D, error) {
		out := make(bson.D, 0, len(container))
		for _, e := range container {
			sub, ok := e.Value.(bson.D)
			if e.Key != leaf || !ok {
				out = append(out, e)
				continue
			}
			for _, fe := range flattenDoc(sub, e.Key, sep) {
				for _, existing := range container {
					if existing.Key == fe.Key {
						return nil, fmt.Errorf("flattening would overwrite field %s", fe.Key)
					}
				}
				out = append(out, fe)
			}
		}
		return out, nil
	}
	if parent == "" {
		return doFlatten(d)
	}
	var err error
	d, _, uerr := docpath.Update(d, parent, func(v any) (any, bool, error) {
		sub, ok := v.(bson.D)
		if !ok {
			return v, true, nil
		}
		nd, e := doFlatten(sub)
		if e != nil {
			err = e
			return v, true, nil
		}
		return nd, true, nil
	})
	if uerr != nil {
		return d, uerr
	}
	return d, err
}

func flattenDoc(sub bson.D, prefix, sep string) bson.D {
	var out bson.D
	for _, e := range sub {
		key := prefix + sep + e.Key
		if inner, ok := e.Value.(bson.D); ok && len(inner) > 0 {
			out = append(out, flattenDoc(inner, key, sep)...)
			continue
		}
		out = append(out, bson.E{Key: key, Value: e.Value})
	}
	return out
}

func nest(d bson.D, fields []string, to string) (bson.D, error) {
	moved := bson.D{}
	for _, f := range fields {
		if f == "_id" {
			return d, fmt.Errorf("_id cannot be nested")
		}
		var v any
		var found bool
		d, v, found = docpath.Delete(d, f)
		if found {
			moved = append(moved, bson.E{Key: f, Value: v})
		}
	}
	if len(moved) == 0 {
		return d, nil
	}
	existing, ok := docpath.Get(d, to)
	if ok {
		sub, isDoc := existing.(bson.D)
		if !isDoc {
			return d, fmt.Errorf("%s exists and is not a document", to)
		}
		for _, m := range moved {
			for _, e := range sub {
				if e.Key == m.Key {
					return d, fmt.Errorf("%s.%s already exists", to, m.Key)
				}
			}
		}
		nd, _ := docpath.Set(d, to, append(sub, moved...))
		return nd, nil
	}
	nd, okSet := docpath.Set(d, to, moved)
	if !okSet {
		return d, fmt.Errorf("a parent of %s is not a document", to)
	}
	return nd, nil
}

// Convert changes a value's BSON type. null stays null; impossible
// conversions return an error.
func Convert(v any, to string) (any, error) {
	if v == nil {
		return nil, nil
	}
	if _, isNull := v.(bson.Null); isNull {
		return nil, nil
	}
	switch to {
	case "string":
		switch t := v.(type) {
		case string:
			return t, nil
		case bson.ObjectID:
			return t.Hex(), nil
		case bson.DateTime:
			return t.Time().UTC().Format(time.RFC3339Nano), nil
		case bson.Decimal128:
			return t.String(), nil
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64), nil
		case int32, int64, bool:
			return fmt.Sprint(t), nil
		}
	case "int", "long":
		n, err := toInt(v)
		if err != nil {
			return nil, err
		}
		if to == "int" {
			if n < math.MinInt32 || n > math.MaxInt32 {
				return nil, fmt.Errorf("%d does not fit in a 32-bit int", n)
			}
			return int32(n), nil
		}
		return n, nil
	case "double":
		switch t := v.(type) {
		case float64:
			return t, nil
		case int32:
			return float64(t), nil
		case int64:
			return float64(t), nil
		case bson.Decimal128:
			f, err := strconv.ParseFloat(t.String(), 64)
			if err == nil {
				return f, nil
			}
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err == nil {
				return f, nil
			}
		case bool:
			if t {
				return 1.0, nil
			}
			return 0.0, nil
		}
	case "decimal":
		var s string
		switch t := v.(type) {
		case bson.Decimal128:
			return t, nil
		case int32, int64:
			s = fmt.Sprint(t)
		case float64:
			s = strconv.FormatFloat(t, 'f', -1, 64)
		case string:
			s = strings.TrimSpace(t)
		}
		if s != "" {
			if d, err := bson.ParseDecimal128(s); err == nil {
				return d, nil
			}
		}
	case "bool":
		switch t := v.(type) {
		case bool:
			return t, nil
		case int32:
			return t != 0, nil
		case int64:
			return t != 0, nil
		case float64:
			return t != 0, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "true", "1", "yes", "y":
				return true, nil
			case "false", "0", "no", "n", "":
				return false, nil
			}
		}
	case "date":
		switch t := v.(type) {
		case bson.DateTime:
			return t, nil
		case string:
			tm, err := mql.ParseDate(t)
			if err == nil {
				return bson.NewDateTimeFromTime(tm), nil
			}
		case int64:
			return bson.DateTime(t), nil
		case int32:
			return bson.DateTime(int64(t)), nil
		case bson.ObjectID:
			return bson.NewDateTimeFromTime(t.Timestamp()), nil
		}
	case "objectId":
		switch t := v.(type) {
		case bson.ObjectID:
			return t, nil
		case string:
			oid, err := bson.ObjectIDFromHex(strings.TrimSpace(t))
			if err == nil {
				return oid, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot convert %s %s to %s", mql.TypeName(v), clip(mql.Compact(v)), to)
}

func toInt(v any) (int64, error) {
	switch t := v.(type) {
	case int32:
		return int64(t), nil
	case int64:
		return t, nil
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) && math.Abs(t) < 9.2e18 {
			return int64(t), nil
		}
		return 0, fmt.Errorf("%v is not a whole number", t)
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%q is not an integer", t)
		}
		return n, nil
	case bson.Decimal128:
		n, err := strconv.ParseInt(t.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s is not an integer", t.String())
		}
		return n, nil
	}
	return 0, fmt.Errorf("cannot convert %s to an integer", mql.TypeName(v))
}

func clip(s string) string {
	if len(s) > 60 {
		return s[:59] + "…"
	}
	return s
}
