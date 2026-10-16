package mql

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Format renders v in mongosh-style syntax that ParseValue reads back to the
// identical BSON value (int32 vs int64 vs double, dates, ObjectIds, ...).
// indent == "" produces a single line.
func Format(v any, indent string) string {
	var b bytes.Buffer
	f := formatter{b: &b, indent: indent}
	f.value(v, 0)
	return b.String()
}

// Pretty formats with two-space indentation.
func Pretty(v any) string { return Format(v, "  ") }

// Compact formats on a single line.
func Compact(v any) string { return Format(v, "") }

type formatter struct {
	b      *bytes.Buffer
	indent string
}

func (f *formatter) newline(depth int) {
	if f.indent == "" {
		return
	}
	f.b.WriteByte('\n')
	for i := 0; i < depth; i++ {
		f.b.WriteString(f.indent)
	}
}

func (f *formatter) doc(d bson.D, depth int) {
	if len(d) == 0 {
		f.b.WriteString("{}")
		return
	}
	f.b.WriteByte('{')
	for i, e := range d {
		if i > 0 {
			f.b.WriteByte(',')
			if f.indent == "" {
				f.b.WriteByte(' ')
			}
		}
		f.newline(depth + 1)
		f.b.WriteString(Quote(e.Key))
		f.b.WriteString(": ")
		f.value(e.Value, depth+1)
	}
	f.newline(depth)
	f.b.WriteByte('}')
}

func (f *formatter) array(a []any, depth int) {
	if len(a) == 0 {
		f.b.WriteString("[]")
		return
	}
	f.b.WriteByte('[')
	for i, e := range a {
		if i > 0 {
			f.b.WriteByte(',')
			if f.indent == "" {
				f.b.WriteByte(' ')
			}
		}
		f.newline(depth + 1)
		f.value(e, depth+1)
	}
	f.newline(depth)
	f.b.WriteByte(']')
}

func (f *formatter) value(v any, depth int) {
	w := f.b
	switch t := v.(type) {
	case nil, bson.Null:
		w.WriteString("null")
	case bson.Undefined:
		w.WriteString("undefined")
	case bool:
		w.WriteString(strconv.FormatBool(t))
	case int32:
		w.WriteString(strconv.FormatInt(int64(t), 10))
	case int:
		if t >= math.MinInt32 && t <= math.MaxInt32 {
			w.WriteString(strconv.Itoa(t))
		} else {
			fmt.Fprintf(w, "NumberLong(%d)", t)
		}
	case int64:
		fmt.Fprintf(w, "NumberLong(%d)", t)
	case float64:
		w.WriteString(FormatDouble(t))
	case float32:
		w.WriteString(FormatDouble(float64(t)))
	case string:
		w.WriteString(Quote(t))
	case bson.Symbol:
		w.WriteString(Quote(string(t)))
	case bson.ObjectID:
		fmt.Fprintf(w, "ObjectId(%q)", t.Hex())
	case bson.DateTime:
		fmt.Fprintf(w, "ISODate(%q)", formatTime(t.Time()))
	case time.Time:
		fmt.Fprintf(w, "ISODate(%q)", formatTime(t))
	case bson.Decimal128:
		fmt.Fprintf(w, "NumberDecimal(%q)", t.String())
	case bson.Binary:
		if t.Subtype == 4 && len(t.Data) == 16 {
			h := hex.EncodeToString(t.Data)
			fmt.Fprintf(w, "UUID(%q)", h[0:8]+"-"+h[8:12]+"-"+h[12:16]+"-"+h[16:20]+"-"+h[20:])
		} else {
			fmt.Fprintf(w, "BinData(%d, %q)", t.Subtype, base64.StdEncoding.EncodeToString(t.Data))
		}
	case bson.Timestamp:
		fmt.Fprintf(w, "Timestamp(%d, %d)", t.T, t.I)
	case bson.Regex:
		if t.Pattern == "" || strings.ContainsAny(t.Pattern, "\n\r") {
			fmt.Fprintf(w, "RegExp(%s, %s)", Quote(t.Pattern), Quote(t.Options))
		} else {
			w.WriteByte('/')
			w.WriteString(escapeRegexSlashes(t.Pattern))
			w.WriteByte('/')
			w.WriteString(t.Options)
		}
	case bson.MinKey:
		w.WriteString("MinKey()")
	case bson.MaxKey:
		w.WriteString("MaxKey()")
	case bson.JavaScript:
		fmt.Fprintf(w, "Code(%s)", Quote(string(t)))
	case bson.D:
		f.doc(t, depth)
	case bson.M:
		f.doc(sortedMap(t), depth)
	case map[string]any:
		f.doc(sortedMap(t), depth)
	case bson.A:
		f.array(t, depth)
	case []any:
		f.array(t, depth)
	case bson.Raw:
		var d bson.D
		if err := bson.Unmarshal(t, &d); err != nil {
			w.WriteString(Quote("<invalid document>"))
			return
		}
		f.doc(d, depth)
	case bson.RawValue:
		var out any
		if err := t.Unmarshal(&out); err != nil {
			w.WriteString(Quote("<invalid value>"))
			return
		}
		f.value(out, depth)
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			a := make([]any, rv.Len())
			for i := range a {
				a[i] = rv.Index(i).Interface()
			}
			f.array(a, depth)
			return
		}
		// Fall back to the driver's Extended JSON for exotic types
		// (DBPointer, CodeWithScope, ...).
		if data, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: v}}, true, false); err == nil {
			s := string(data)
			w.WriteString(strings.TrimSuffix(strings.TrimPrefix(s, `{"v":`), "}"))
			return
		}
		w.WriteString(Quote(fmt.Sprint(v)))
	}
}

func sortedMap(m map[string]any) bson.D {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	d := make(bson.D, 0, len(keys))
	for _, k := range keys {
		d = append(d, bson.E{Key: k, Value: m[k]})
	}
	return d
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// FormatDouble renders a float so that it re-parses as a double: integral
// values keep a trailing ".0".
func FormatDouble(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64) + ".0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func escapeRegexSlashes(p string) string {
	var b strings.Builder
	escaped, inClass := false, false
	for _, c := range p {
		switch {
		case escaped:
			escaped = false
		case c == '\\':
			escaped = true
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '/' && !inClass:
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// Quote renders s as a JSON string literal without HTML escaping.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x2028 || c == 0x2029 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// TypeName returns a human-readable BSON type name for v.
func TypeName(v any) string {
	switch v.(type) {
	case nil, bson.Null:
		return "null"
	case bson.Undefined:
		return "undefined"
	case bool:
		return "boolean"
	case int32, int:
		return "int32"
	case int64:
		return "int64"
	case float64, float32:
		return "double"
	case string, bson.Symbol:
		return "string"
	case bson.ObjectID:
		return "objectId"
	case bson.DateTime, time.Time:
		return "date"
	case bson.Decimal128:
		return "decimal"
	case bson.Binary:
		return "binary"
	case bson.Timestamp:
		return "timestamp"
	case bson.Regex:
		return "regex"
	case bson.D, bson.M, map[string]any, bson.Raw:
		return "document"
	case bson.A, []any:
		return "array"
	case bson.MinKey:
		return "minKey"
	case bson.MaxKey:
		return "maxKey"
	case bson.JavaScript:
		return "javascript"
	}
	return fmt.Sprintf("%T", v)
}

// RelaxedJSON renders a document as relaxed Extended JSON for the grid view.
func RelaxedJSON(d bson.D) string {
	data, err := bson.MarshalExtJSON(d, false, false)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// CanonicalJSON renders a document as canonical Extended JSON: every value
// keeps its exact BSON type ({"$numberInt": "5"}, {"$numberDouble": "5.0"}),
// so the UI can show types and 64-bit integers without precision loss.
func CanonicalJSON(d bson.D) string {
	data, err := bson.MarshalExtJSON(d, true, false)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Equal reports whether two BSON values are identical (type and value).
func Equal(a, b any) bool {
	ab, err1 := bson.Marshal(bson.D{{Key: "v", Value: a}})
	bb, err2 := bson.Marshal(bson.D{{Key: "v", Value: b}})
	return err1 == nil && err2 == nil && bytes.Equal(ab, bb)
}

// Lookup returns the value of a top-level field.
func Lookup(d bson.D, key string) (any, bool) {
	for _, e := range d {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}
