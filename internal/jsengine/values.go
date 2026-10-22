package jsengine

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// BSON values that JavaScript has no type for are Go wrappers: scripts see
// objects with mongosh-style methods (toString, toHexString…), and the
// conversion back to BSON recognizes them exactly.

// ObjectIDValue is ObjectId("…").
type ObjectIDValue struct{ ID bson.ObjectID }

func (o *ObjectIDValue) ToString() string        { return `ObjectId("` + o.ID.Hex() + `")` }
func (o *ObjectIDValue) ToHexString() string     { return o.ID.Hex() }
func (o *ObjectIDValue) GetTimestamp() time.Time { return o.ID.Timestamp() }
func (o *ObjectIDValue) Equals(other goja.Value) bool {
	if w, ok := other.Export().(*ObjectIDValue); ok {
		return w.ID == o.ID
	}
	return false
}

// LongValue is NumberLong("…") (a 64-bit integer beyond JavaScript's safe range,
// or one created explicitly).
type LongValue struct{ N int64 }

func (l *LongValue) ToString() string  { return strconv.FormatInt(l.N, 10) }
func (l *LongValue) ToNumber() float64 { return float64(l.N) }

// IntValue is NumberInt(…).
type IntValue struct{ N int32 }

func (i *IntValue) ToString() string  { return strconv.FormatInt(int64(i.N), 10) }
func (i *IntValue) ToNumber() float64 { return float64(i.N) }

// DecimalValue is NumberDecimal("…").
type DecimalValue struct{ D bson.Decimal128 }

func (d *DecimalValue) ToString() string { return d.D.String() }

// BinaryValue is BinData / UUID.
type BinaryValue struct{ B bson.Binary }

func (b *BinaryValue) ToString() string { return mql.Compact(b.B) }
func (b *BinaryValue) Hex() string      { return hex.EncodeToString(b.B.Data) }

// TimestampValue is Timestamp(t, i).
type TimestampValue struct{ T bson.Timestamp }

func (t *TimestampValue) ToString() string { return mql.Compact(t.T) }

// fromJS converts a JavaScript value to a BSON value.
func fromJS(vm *goja.Runtime, v goja.Value) (any, error) {
	return fromJSDepth(vm, v, 0)
}

func fromJSDepth(vm *goja.Runtime, v goja.Value, depth int) (any, error) {
	if depth > 100 {
		return nil, apperr.Invalid("The value is nested too deeply (or refers to itself).")
	}
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, nil
	}
	switch x := v.Export().(type) {
	case *ObjectIDValue:
		return x.ID, nil
	case *LongValue:
		return x.N, nil
	case *IntValue:
		return x.N, nil
	case *DecimalValue:
		return x.D, nil
	case *BinaryValue:
		return x.B, nil
	case *TimestampValue:
		return x.T, nil
	case bool:
		return x, nil
	case string:
		return x, nil
	case int64:
		return jsNumber(float64(x)), nil
	case float64:
		return jsNumber(x), nil
	case time.Time:
		return bson.NewDateTimeFromTime(x), nil
	}
	obj, ok := v.(*goja.Object)
	if !ok {
		return nil, apperr.Invalid("Values of this type can't be stored: %s", v.String())
	}
	switch obj.ClassName() {
	case "Array":
		n := int(obj.Get("length").ToInteger())
		out := make(bson.A, 0, n)
		for i := 0; i < n; i++ {
			el, err := fromJSDepth(vm, obj.Get(strconv.Itoa(i)), depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, el)
		}
		return out, nil
	case "Date":
		if t, ok := obj.Export().(time.Time); ok {
			return bson.NewDateTimeFromTime(t), nil
		}
	case "RegExp":
		return bson.Regex{Pattern: obj.Get("source").String(), Options: obj.Get("flags").String()}, nil
	case "Function":
		return nil, apperr.Invalid("Functions can't be stored in documents.")
	}
	if c, ok := obj.Export().(*Cursor); ok {
		docs, err := c.all()
		if err != nil {
			return nil, err
		}
		out := make(bson.A, len(docs))
		for i, d := range docs {
			out[i] = d
		}
		return out, nil
	}
	out := bson.D{}
	for _, k := range obj.Keys() {
		el, err := fromJSDepth(vm, obj.Get(k), depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, bson.E{Key: k, Value: el})
	}
	return out, nil
}

// jsNumber stores a JavaScript number the way mongosh does: an integer that
// fits 32 bits as int32, anything else as a double.
func jsNumber(f float64) any {
	if f == math.Trunc(f) && f >= math.MinInt32 && f <= math.MaxInt32 && !math.IsInf(f, 0) && !(f == 0 && math.Signbit(f)) {
		return int32(f)
	}
	return f
}

// docFromJS converts a value that must be a document (a filter, an update…).
func docFromJS(vm *goja.Runtime, v goja.Value, what string) (bson.D, error) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return bson.D{}, nil
	}
	x, err := fromJS(vm, v)
	if err != nil {
		return nil, err
	}
	d, ok := x.(bson.D)
	if !ok {
		return nil, apperr.Invalid("%s must be a document, not %s.", what, v.String())
	}
	return d, nil
}

// toJS converts a BSON value to a JavaScript value.
func toJS(vm *goja.Runtime, v any) goja.Value {
	switch x := v.(type) {
	case nil, bson.Null:
		return goja.Null()
	case bson.Undefined:
		return goja.Undefined()
	case bson.D:
		obj := vm.NewObject()
		for _, e := range x {
			_ = obj.Set(e.Key, toJS(vm, e.Value))
		}
		return obj
	case bson.M:
		obj := vm.NewObject()
		for k, el := range x {
			_ = obj.Set(k, toJS(vm, el))
		}
		return obj
	case bson.A:
		arr := make([]any, len(x))
		for i, el := range x {
			arr[i] = toJS(vm, el)
		}
		return vm.NewArray(arr...)
	case bson.ObjectID:
		return vm.ToValue(&ObjectIDValue{ID: x})
	case int32:
		return vm.ToValue(int64(x))
	case int64:
		if x > 1<<53 || x < -(1<<53) {
			return vm.ToValue(&LongValue{N: x})
		}
		return vm.ToValue(x)
	case float64:
		return vm.ToValue(x)
	case bson.Decimal128:
		return vm.ToValue(&DecimalValue{D: x})
	case bson.DateTime:
		return newDate(vm, x.Time())
	case bson.Binary:
		return vm.ToValue(&BinaryValue{B: x})
	case bson.Timestamp:
		return vm.ToValue(&TimestampValue{T: x})
	case bson.Regex:
		re, err := vm.New(vm.Get("RegExp"), vm.ToValue(x.Pattern), vm.ToValue(x.Options))
		if err == nil {
			return re
		}
		return vm.ToValue(x.Pattern)
	case string, bool:
		return vm.ToValue(x)
	}
	return vm.ToValue(v)
}

func newDate(vm *goja.Runtime, t time.Time) goja.Value {
	d, err := vm.New(vm.Get("Date"), vm.ToValue(t.UnixMilli()))
	if err != nil {
		return vm.ToValue(t.String())
	}
	return d
}

// display formats a value the way the shell shows it: documents and arrays
// in mongosh syntax; strings as they are when raw is set (print).
func display(vm *goja.Runtime, v goja.Value, raw bool) string {
	if v == nil || goja.IsUndefined(v) {
		return "undefined"
	}
	if s, ok := v.Export().(string); ok && raw {
		return s
	}
	if obj, ok := v.(*goja.Object); ok && obj.ClassName() == "Function" {
		return "[Function]"
	}
	if c, ok := v.Export().(*Cursor); ok {
		return fmt.Sprintf("[cursor on %s]", c.ns())
	}
	x, err := fromJS(vm, v)
	if err != nil {
		return v.String()
	}
	if d, ok := x.(bson.D); ok {
		return mql.Pretty(d)
	}
	if a, ok := x.(bson.A); ok {
		return mql.Pretty(a)
	}
	return mql.Compact(x)
}

func errText(err error) string {
	if ex, ok := err.(*goja.Exception); ok {
		return strings.TrimPrefix(ex.Error(), "Error: ")
	}
	if ie, ok := err.(*goja.InterruptedError); ok {
		return fmt.Sprint(ie.Value())
	}
	return apperr.From(err).Message
}
