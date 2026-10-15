// Package codegen turns a query or pipeline into runnable code for MongoDB
// drivers. Each language has its own BSON literal renderer so types survive:
// int64, Decimal128, dates, ObjectIds, regexes and binary are rendered with
// that driver's native constructors.
package codegen

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
)

// Spec is a query or aggregation to generate code for.
type Spec struct {
	Kind       string // "find" | "aggregate"
	DB         string
	Coll       string
	Filter     bson.D
	Projection bson.D
	Sort       bson.D
	Skip       int64
	Limit      int64
	Pipeline   []bson.D
}

// Language describes a supported target.
type Language struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Ext   string `json:"ext"`
}

// Languages lists the supported targets in display order.
var Languages = []Language{
	{ID: "mongosh", Label: "mongosh", Ext: "js"},
	{ID: "javascript", Label: "Node.js", Ext: "mjs"},
	{ID: "python", Label: "Python (PyMongo)", Ext: "py"},
	{ID: "java", Label: "Java (sync driver)", Ext: "java"},
	{ID: "csharp", Label: "C# (.NET driver)", Ext: "cs"},
	{ID: "go", Label: "Go (driver v2)", Ext: "go"},
}

// Generate renders spec in the given language.
func Generate(lang string, s Spec) (string, error) {
	if s.Kind != "find" && s.Kind != "aggregate" {
		return "", apperr.Invalid("Unknown query kind %q.", s.Kind)
	}
	if s.DB == "" || s.Coll == "" {
		return "", apperr.Invalid("Database and collection are required.")
	}
	if s.Filter == nil {
		s.Filter = bson.D{}
	}
	switch lang {
	case "mongosh":
		return genMongosh(s), nil
	case "javascript":
		return genJS(s), nil
	case "python":
		return genPython(s), nil
	case "java":
		return genJava(s), nil
	case "csharp":
		return genCSharp(s), nil
	case "go":
		return genGo(s), nil
	}
	return "", apperr.Invalid("Unsupported language %q.", lang)
}

// ---- shared helpers ---------------------------------------------------------

// renderer turns BSON values into source literals for one language.
type renderer struct {
	doc    func(r *renderer, d bson.D, ind string) string
	arr    func(r *renderer, a bson.A, ind string) string
	scalar func(r *renderer, v any) string
	used   map[string]bool // helpers/imports needed
	indent string
}

func (r *renderer) need(s string) { r.used[s] = true }

func (r *renderer) value(v any, ind string) string {
	switch t := v.(type) {
	case bson.D:
		return r.doc(r, t, ind)
	case bson.A:
		return r.arr(r, t, ind)
	case []any:
		return r.arr(r, bson.A(t), ind)
	case bson.M:
		return r.doc(r, sortedM(t), ind)
	case int:
		if t >= math.MinInt32 && t <= math.MaxInt32 {
			return r.scalar(r, int32(t))
		}
		return r.scalar(r, int64(t))
	}
	return r.scalar(r, v)
}

func sortedM(m bson.M) bson.D {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	d := bson.D{}
	for _, k := range keys {
		d = append(d, bson.E{Key: k, Value: m[k]})
	}
	return d
}

// inline reports whether a container fits on one line.
func inline(parts []string, open, close string, max int) (string, bool) {
	s := open + strings.Join(parts, ", ") + close
	if len(s) <= max && !strings.Contains(s, "\n") {
		return s, true
	}
	return "", false
}

func block(parts []string, open, close, ind, step string) string {
	return open + "\n" + ind + step + strings.Join(parts, ",\n"+ind+step) + "\n" + ind + close
}

// quote renders a double-quoted string literal valid in JS, Java, C#, Python and Go.
func quote(s string) string {
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

var jsIdent = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)

func isoTime(dt bson.DateTime) string { return dt.Time().UTC().Format("2006-01-02T15:04:05.000Z") }

func floatLit(f float64, nan, inf, ninf string) string {
	switch {
	case math.IsNaN(f):
		return nan
	case math.IsInf(f, 1):
		return inf
	case math.IsInf(f, -1):
		return ninf
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return fmt.Sprintf("%.1f", f)
	}
	return fmt.Sprint(f)
}

func uuidString(b []byte) string {
	h := fmt.Sprintf("%x", b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func hexString(b []byte) string { return fmt.Sprintf("%x", b) }

func timeParts(dt bson.DateTime) time.Time { return dt.Time().UTC() }

func sortedUsed(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func escapeRegex(p string) string {
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

// ---- mongosh & JavaScript --------------------------------------------------

func jsRenderer(shell bool) *renderer {
	r := &renderer{used: map[string]bool{}, indent: "  "}
	r.doc = func(r *renderer, d bson.D, ind string) string {
		if len(d) == 0 {
			return "{}"
		}
		parts := make([]string, len(d))
		for i, e := range d {
			k := e.Key
			if !jsIdent.MatchString(k) {
				k = quote(k)
			}
			parts[i] = k + ": " + r.value(e.Value, ind+r.indent)
		}
		if s, ok := inline(parts, "{ ", " }", 72); ok {
			return s
		}
		return block(parts, "{", "}", ind, r.indent)
	}
	r.arr = func(r *renderer, a bson.A, ind string) string {
		if len(a) == 0 {
			return "[]"
		}
		parts := make([]string, len(a))
		for i, e := range a {
			parts[i] = r.value(e, ind+r.indent)
		}
		if s, ok := inline(parts, "[", "]", 72); ok {
			return s
		}
		return block(parts, "[", "]", ind, r.indent)
	}
	r.scalar = func(r *renderer, v any) string {
		switch t := v.(type) {
		case nil, bson.Null, bson.Undefined:
			return "null"
		case bool:
			return fmt.Sprint(t)
		case int32:
			return fmt.Sprint(t)
		case int64:
			if shell {
				return fmt.Sprintf("Long(%q)", fmt.Sprint(t))
			}
			r.need("Long")
			return fmt.Sprintf("Long.fromString(%q)", fmt.Sprint(t))
		case float64:
			s := floatLit(t, "NaN", "Infinity", "-Infinity")
			return strings.TrimSuffix(s, ".0")
		case string:
			return quote(t)
		case bson.ObjectID:
			if shell {
				return fmt.Sprintf("ObjectId(%q)", t.Hex())
			}
			r.need("ObjectId")
			return fmt.Sprintf("new ObjectId(%q)", t.Hex())
		case bson.DateTime:
			if shell {
				return fmt.Sprintf("ISODate(%q)", isoTime(t))
			}
			return fmt.Sprintf("new Date(%q)", isoTime(t))
		case bson.Decimal128:
			if shell {
				return fmt.Sprintf("Decimal128(%q)", t.String())
			}
			r.need("Decimal128")
			return fmt.Sprintf("Decimal128.fromString(%q)", t.String())
		case bson.Regex:
			if t.Pattern == "" {
				return fmt.Sprintf("new RegExp(\"\", %q)", t.Options)
			}
			return "/" + escapeRegex(t.Pattern) + "/" + t.Options
		case bson.Binary:
			if t.Subtype == 4 && len(t.Data) == 16 {
				if shell {
					return fmt.Sprintf("UUID(%q)", uuidString(t.Data))
				}
				r.need("UUID")
				return fmt.Sprintf("new UUID(%q)", uuidString(t.Data))
			}
			if shell {
				return fmt.Sprintf("BinData(%d, %q)", t.Subtype, b64(t.Data))
			}
			r.need("Binary")
			return fmt.Sprintf("Binary.createFromBase64(%q, %d)", b64(t.Data), t.Subtype)
		case bson.Timestamp:
			if shell {
				return fmt.Sprintf("Timestamp({ t: %d, i: %d })", t.T, t.I)
			}
			r.need("Timestamp")
			return fmt.Sprintf("new Timestamp({ t: %d, i: %d })", t.T, t.I)
		case bson.MinKey:
			if shell {
				return "MinKey()"
			}
			r.need("MinKey")
			return "new MinKey()"
		case bson.MaxKey:
			if shell {
				return "MaxKey()"
			}
			r.need("MaxKey")
			return "new MaxKey()"
		case bson.JavaScript:
			if shell {
				return fmt.Sprintf("Code(%s)", quote(string(t)))
			}
			r.need("Code")
			return fmt.Sprintf("new Code(%s)", quote(string(t)))
		}
		return quote(fmt.Sprint(v))
	}
	return r
}

func genMongosh(s Spec) string {
	r := jsRenderer(true)
	var b strings.Builder
	fmt.Fprintf(&b, "use(%s);\n\n", quote(s.DB))
	coll := "db." + s.Coll
	if !jsIdent.MatchString(s.Coll) {
		coll = "db.getCollection(" + quote(s.Coll) + ")"
	}
	if s.Kind == "aggregate" {
		fmt.Fprintf(&b, "%s.aggregate(%s);\n", coll, r.value(pipelineA(s.Pipeline), ""))
		return b.String()
	}
	fmt.Fprintf(&b, "%s.find(%s", coll, r.value(s.Filter, ""))
	if len(s.Projection) > 0 {
		fmt.Fprintf(&b, ", %s", r.value(s.Projection, ""))
	}
	b.WriteString(")")
	if len(s.Sort) > 0 {
		fmt.Fprintf(&b, "\n  .sort(%s)", r.value(s.Sort, "  "))
	}
	if s.Skip > 0 {
		fmt.Fprintf(&b, "\n  .skip(%d)", s.Skip)
	}
	if s.Limit > 0 {
		fmt.Fprintf(&b, "\n  .limit(%d)", s.Limit)
	}
	b.WriteString(";\n")
	return b.String()
}

func pipelineA(p []bson.D) bson.A {
	a := make(bson.A, len(p))
	for i, st := range p {
		a[i] = st
	}
	return a
}

func genJS(s Spec) string {
	r := jsRenderer(false)
	var body strings.Builder
	in := "    "
	fmt.Fprintf(&body, "%sconst coll = client.db(%s).collection(%s);\n", in, quote(s.DB), quote(s.Coll))
	if s.Kind == "aggregate" {
		fmt.Fprintf(&body, "%sconst pipeline = %s;\n", in, r.value(pipelineA(s.Pipeline), in))
		fmt.Fprintf(&body, "%sconst cursor = coll.aggregate(pipeline);\n", in)
	} else {
		fmt.Fprintf(&body, "%sconst filter = %s;\n", in, r.value(s.Filter, in))
		var opts []string
		if len(s.Projection) > 0 {
			opts = append(opts, "projection: "+r.value(s.Projection, in+"  "))
		}
		if len(s.Sort) > 0 {
			opts = append(opts, "sort: "+r.value(s.Sort, in+"  "))
		}
		if s.Skip > 0 {
			opts = append(opts, fmt.Sprintf("skip: %d", s.Skip))
		}
		if s.Limit > 0 {
			opts = append(opts, fmt.Sprintf("limit: %d", s.Limit))
		}
		if len(opts) > 0 {
			fmt.Fprintf(&body, "%sconst cursor = coll.find(filter, {\n%s  %s\n%s});\n", in, in, strings.Join(opts, ",\n"+in+"  "), in)
		} else {
			fmt.Fprintf(&body, "%sconst cursor = coll.find(filter);\n", in)
		}
	}
	fmt.Fprintf(&body, "%sfor await (const doc of cursor) {\n%s  console.log(doc);\n%s}\n", in, in, in)

	imports := append([]string{"MongoClient"}, sortedUsed(r.used)...)
	var b strings.Builder
	fmt.Fprintf(&b, "// npm install mongodb   ·   run with: node query.mjs\n")
	fmt.Fprintf(&b, "import { %s } from 'mongodb';\n\n", strings.Join(imports, ", "))
	b.WriteString("const client = new MongoClient(process.env.MONGODB_URI ?? 'mongodb://localhost:27017');\n\n")
	b.WriteString("async function main() {\n  try {\n")
	b.WriteString(body.String())
	b.WriteString("  } finally {\n    await client.close();\n  }\n}\n\nmain().catch(console.error);\n")
	return b.String()
}

func b64(data []byte) string {
	const enc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	for i := 0; i < len(data); i += 3 {
		var n uint32
		rem := len(data) - i
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < rem {
				n |= uint32(data[i+j])
			}
		}
		for j := 0; j < 4; j++ {
			if j <= rem {
				b.WriteByte(enc[(n>>(18-6*uint(j)))&0x3F])
			} else {
				b.WriteByte('=')
			}
		}
	}
	return b.String()
}

// ---- Python -----------------------------------------------------------------

func pyRenderer() *renderer {
	r := &renderer{used: map[string]bool{}, indent: "    "}
	r.doc = func(r *renderer, d bson.D, ind string) string {
		if len(d) == 0 {
			return "{}"
		}
		parts := make([]string, len(d))
		for i, e := range d {
			parts[i] = quote(e.Key) + ": " + r.value(e.Value, ind+r.indent)
		}
		if s, ok := inline(parts, "{", "}", 76); ok {
			return s
		}
		return block(parts, "{", "}", ind, r.indent)
	}
	r.arr = func(r *renderer, a bson.A, ind string) string {
		if len(a) == 0 {
			return "[]"
		}
		parts := make([]string, len(a))
		for i, e := range a {
			parts[i] = r.value(e, ind+r.indent)
		}
		if s, ok := inline(parts, "[", "]", 76); ok {
			return s
		}
		return block(parts, "[", "]", ind, r.indent)
	}
	r.scalar = func(r *renderer, v any) string {
		switch t := v.(type) {
		case nil, bson.Null, bson.Undefined:
			return "None"
		case bool:
			if t {
				return "True"
			}
			return "False"
		case int32:
			return fmt.Sprint(t)
		case int64:
			r.need("from bson import Int64")
			return fmt.Sprintf("Int64(%d)", t)
		case float64:
			return floatLit(t, `float("nan")`, `float("inf")`, `float("-inf")`)
		case string:
			return quote(t)
		case bson.ObjectID:
			r.need("from bson import ObjectId")
			return fmt.Sprintf("ObjectId(%q)", t.Hex())
		case bson.DateTime:
			r.need("from datetime import datetime, timezone")
			tm := timeParts(t)
			return fmt.Sprintf("datetime(%d, %d, %d, %d, %d, %d, %d, tzinfo=timezone.utc)",
				tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond()/1000)
		case bson.Decimal128:
			r.need("from bson import Decimal128")
			return fmt.Sprintf("Decimal128(%q)", t.String())
		case bson.Regex:
			r.need("from bson import Regex")
			return fmt.Sprintf("Regex(%s, %q)", quote(t.Pattern), t.Options)
		case bson.Binary:
			r.need("from bson import Binary")
			if t.Subtype == 4 && len(t.Data) == 16 {
				r.need("import uuid")
				return fmt.Sprintf("Binary.from_uuid(uuid.UUID(%q))", uuidString(t.Data))
			}
			return fmt.Sprintf("Binary(bytes.fromhex(%q), %d)", hexString(t.Data), t.Subtype)
		case bson.Timestamp:
			r.need("from bson import Timestamp")
			return fmt.Sprintf("Timestamp(%d, %d)", t.T, t.I)
		case bson.MinKey:
			r.need("from bson import MinKey")
			return "MinKey()"
		case bson.MaxKey:
			r.need("from bson import MaxKey")
			return "MaxKey()"
		case bson.JavaScript:
			r.need("from bson import Code")
			return fmt.Sprintf("Code(%s)", quote(string(t)))
		}
		return quote(fmt.Sprint(v))
	}
	return r
}

func genPython(s Spec) string {
	r := pyRenderer()
	var body strings.Builder
	fmt.Fprintf(&body, "coll = client[%s][%s]\n\n", quote(s.DB), quote(s.Coll))
	if s.Kind == "aggregate" {
		fmt.Fprintf(&body, "pipeline = %s\n\ncursor = coll.aggregate(pipeline)\n", r.value(pipelineA(s.Pipeline), ""))
	} else {
		fmt.Fprintf(&body, "query = %s\n", r.value(s.Filter, ""))
		args := "query"
		if len(s.Projection) > 0 {
			fmt.Fprintf(&body, "projection = %s\n", r.value(s.Projection, ""))
			args += ", projection"
		}
		chain := ""
		if len(s.Sort) > 0 {
			pairs := make([]string, len(s.Sort))
			for i, e := range s.Sort {
				pairs[i] = fmt.Sprintf("(%s, %s)", quote(e.Key), r.value(e.Value, ""))
			}
			chain += fmt.Sprintf(".sort([%s])", strings.Join(pairs, ", "))
		}
		if s.Skip > 0 {
			chain += fmt.Sprintf(".skip(%d)", s.Skip)
		}
		if s.Limit > 0 {
			chain += fmt.Sprintf(".limit(%d)", s.Limit)
		}
		fmt.Fprintf(&body, "\ncursor = coll.find(%s)%s\n", args, chain)
	}
	body.WriteString("for doc in cursor:\n    print(doc)\n")

	var imports, froms []string
	for _, u := range sortedUsed(r.used) {
		if strings.HasPrefix(u, "import ") {
			imports = append(imports, u)
		} else {
			froms = append(froms, u)
		}
	}
	var b strings.Builder
	b.WriteString("# pip install pymongo\nimport os\n")
	for _, i := range imports {
		b.WriteString(i + "\n")
	}
	for _, f := range mergeFroms(froms) {
		b.WriteString(f + "\n")
	}
	b.WriteString("from pymongo import MongoClient\n\n")
	b.WriteString(`client = MongoClient(os.environ.get("MONGODB_URI", "mongodb://localhost:27017"))` + "\n")
	b.WriteString(body.String())
	return b.String()
}

// mergeFroms combines "from bson import A" lines per module.
func mergeFroms(lines []string) []string {
	mods := map[string][]string{}
	var order []string
	for _, l := range lines {
		parts := strings.SplitN(strings.TrimPrefix(l, "from "), " import ", 2)
		if _, ok := mods[parts[0]]; !ok {
			order = append(order, parts[0])
		}
		mods[parts[0]] = append(mods[parts[0]], parts[1])
	}
	sort.Strings(order)
	out := make([]string, len(order))
	for i, m := range order {
		names := mods[m]
		sort.Strings(names)
		out[i] = "from " + m + " import " + strings.Join(names, ", ")
	}
	return out
}

// ---- Java -------------------------------------------------------------------

func javaRenderer() *renderer {
	r := &renderer{used: map[string]bool{}, indent: "    "}
	r.doc = func(r *renderer, d bson.D, ind string) string {
		if len(d) == 0 {
			return "new Document()"
		}
		s := fmt.Sprintf("new Document(%s, %s)", quote(d[0].Key), r.value(d[0].Value, ind+r.indent))
		for _, e := range d[1:] {
			next := fmt.Sprintf(".append(%s, %s)", quote(e.Key), r.value(e.Value, ind+r.indent))
			if len(s)+len(next) > 90 || strings.Contains(s, "\n") || strings.Contains(next, "\n") {
				s += "\n" + ind + r.indent + next
			} else {
				s += next
			}
		}
		return s
	}
	r.arr = func(r *renderer, a bson.A, ind string) string {
		parts := make([]string, len(a))
		for i, e := range a {
			parts[i] = r.value(e, ind+r.indent)
		}
		if s, ok := inline(parts, "Arrays.asList(", ")", 80); ok {
			return s
		}
		return block(parts, "Arrays.asList(", ")", ind, r.indent)
	}
	r.scalar = func(r *renderer, v any) string {
		switch t := v.(type) {
		case nil, bson.Null:
			return "null"
		case bson.Undefined:
			return "new BsonUndefined()"
		case bool:
			return fmt.Sprint(t)
		case int32:
			return fmt.Sprint(t)
		case int64:
			return fmt.Sprintf("%dL", t)
		case float64:
			return floatLit(t, "Double.NaN", "Double.POSITIVE_INFINITY", "Double.NEGATIVE_INFINITY")
		case string:
			return quote(t)
		case bson.ObjectID:
			return fmt.Sprintf("new ObjectId(%q)", t.Hex())
		case bson.DateTime:
			return fmt.Sprintf("Date.from(Instant.parse(%q))", isoTime(t))
		case bson.Decimal128:
			return fmt.Sprintf("Decimal128.parse(%q)", t.String())
		case bson.Regex:
			return fmt.Sprintf("new BsonRegularExpression(%s, %q)", quote(t.Pattern), t.Options)
		case bson.Binary:
			if t.Subtype == 4 && len(t.Data) == 16 {
				return fmt.Sprintf("UUID.fromString(%q)", uuidString(t.Data))
			}
			return fmt.Sprintf("new Binary((byte) %d, Base64.getDecoder().decode(%q))", t.Subtype, b64(t.Data))
		case bson.Timestamp:
			return fmt.Sprintf("new BSONTimestamp(%d, %d)", t.T, t.I)
		case bson.MinKey:
			return "new MinKey()"
		case bson.MaxKey:
			return "new MaxKey()"
		case bson.JavaScript:
			return fmt.Sprintf("new Code(%s)", quote(string(t)))
		}
		return quote(fmt.Sprint(v))
	}
	return r
}

func genJava(s Spec) string {
	r := javaRenderer()
	in := "            "
	var body strings.Builder
	fmt.Fprintf(&body, "%sMongoCollection<Document> coll = client.getDatabase(%s).getCollection(%s);\n\n", in, quote(s.DB), quote(s.Coll))
	if s.Kind == "aggregate" {
		stages := make([]string, len(s.Pipeline))
		for i, st := range s.Pipeline {
			stages[i] = r.value(st, in+"    ")
		}
		fmt.Fprintf(&body, "%sList<Document> pipeline = Arrays.asList(\n%s    %s\n%s);\n", in, in, strings.Join(stages, ",\n"+in+"    "), in)
		fmt.Fprintf(&body, "%sfor (Document doc : coll.aggregate(pipeline)) {\n", in)
	} else {
		fmt.Fprintf(&body, "%sDocument filter = %s;\n", in, r.value(s.Filter, in))
		chain := "coll.find(filter)"
		if len(s.Projection) > 0 {
			fmt.Fprintf(&body, "%sDocument projection = %s;\n", in, r.value(s.Projection, in))
			chain += ".projection(projection)"
		}
		if len(s.Sort) > 0 {
			fmt.Fprintf(&body, "%sDocument sort = %s;\n", in, r.value(s.Sort, in))
			chain += ".sort(sort)"
		}
		if s.Skip > 0 {
			chain += fmt.Sprintf(".skip(%d)", s.Skip)
		}
		if s.Limit > 0 {
			chain += fmt.Sprintf(".limit(%d)", s.Limit)
		}
		fmt.Fprintf(&body, "\n%sfor (Document doc : %s) {\n", in, chain)
	}
	fmt.Fprintf(&body, "%s    System.out.println(doc.toJson());\n%s}\n", in, in)

	var b strings.Builder
	b.WriteString(`// Maven: org.mongodb:mongodb-driver-sync
import com.mongodb.client.MongoClient;
import com.mongodb.client.MongoClients;
import com.mongodb.client.MongoCollection;
import org.bson.BsonRegularExpression;
import org.bson.BsonUndefined;
import org.bson.Document;
import org.bson.types.*;

import java.time.Instant;
import java.util.*;

public class Query {
    public static void main(String[] args) {
        String uri = System.getenv().getOrDefault("MONGODB_URI", "mongodb://localhost:27017");
        try (MongoClient client = MongoClients.create(uri)) {
`)
	b.WriteString(body.String())
	b.WriteString("        }\n    }\n}\n")
	return b.String()
}

// ---- C# ---------------------------------------------------------------------

func csRenderer() *renderer {
	r := &renderer{used: map[string]bool{}, indent: "    "}
	r.doc = func(r *renderer, d bson.D, ind string) string {
		if len(d) == 0 {
			return "new BsonDocument()"
		}
		parts := make([]string, len(d))
		for i, e := range d {
			parts[i] = "{ " + quote(e.Key) + ", " + r.value(e.Value, ind+r.indent) + " }"
		}
		if s, ok := inline(parts, "new BsonDocument { ", " }", 90); ok {
			return s
		}
		return block(parts, "new BsonDocument\n"+ind+"{", "}", ind, r.indent)
	}
	r.arr = func(r *renderer, a bson.A, ind string) string {
		if len(a) == 0 {
			return "new BsonArray()"
		}
		parts := make([]string, len(a))
		for i, e := range a {
			parts[i] = r.value(e, ind+r.indent)
		}
		if s, ok := inline(parts, "new BsonArray { ", " }", 90); ok {
			return s
		}
		return block(parts, "new BsonArray\n"+ind+"{", "}", ind, r.indent)
	}
	r.scalar = func(r *renderer, v any) string {
		switch t := v.(type) {
		case nil, bson.Null:
			return "BsonNull.Value"
		case bson.Undefined:
			return "BsonUndefined.Value"
		case bool:
			return fmt.Sprint(t)
		case int32:
			return fmt.Sprint(t)
		case int64:
			return fmt.Sprintf("%dL", t)
		case float64:
			return floatLit(t, "double.NaN", "double.PositiveInfinity", "double.NegativeInfinity")
		case string:
			return quote(t)
		case bson.ObjectID:
			return fmt.Sprintf("new ObjectId(%q)", t.Hex())
		case bson.DateTime:
			tm := timeParts(t)
			return fmt.Sprintf("new DateTime(%d, %d, %d, %d, %d, %d, %d, DateTimeKind.Utc)",
				tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond()/1_000_000)
		case bson.Decimal128:
			return fmt.Sprintf("new BsonDecimal128(Decimal128.Parse(%q))", t.String())
		case bson.Regex:
			return fmt.Sprintf("new BsonRegularExpression(%s, %q)", quote(t.Pattern), t.Options)
		case bson.Binary:
			if t.Subtype == 4 && len(t.Data) == 16 {
				return fmt.Sprintf("new BsonBinaryData(Guid.Parse(%q), GuidRepresentation.Standard)", uuidString(t.Data))
			}
			return fmt.Sprintf("new BsonBinaryData(Convert.FromBase64String(%q), (BsonBinarySubType)%d)", b64(t.Data), t.Subtype)
		case bson.Timestamp:
			return fmt.Sprintf("new BsonTimestamp(%d, %d)", t.T, t.I)
		case bson.MinKey:
			return "BsonMinKey.Value"
		case bson.MaxKey:
			return "BsonMaxKey.Value"
		case bson.JavaScript:
			return fmt.Sprintf("new BsonJavaScript(%s)", quote(string(t)))
		}
		return quote(fmt.Sprint(v))
	}
	return r
}

func genCSharp(s Spec) string {
	r := csRenderer()
	var body strings.Builder
	fmt.Fprintf(&body, "var coll = client.GetDatabase(%s).GetCollection<BsonDocument>(%s);\n\n", quote(s.DB), quote(s.Coll))
	if s.Kind == "aggregate" {
		stages := make([]string, len(s.Pipeline))
		for i, st := range s.Pipeline {
			stages[i] = r.value(st, "    ")
		}
		fmt.Fprintf(&body, "PipelineDefinition<BsonDocument, BsonDocument> pipeline = new BsonDocument[]\n{\n    %s\n};\n", strings.Join(stages, ",\n    "))
		body.WriteString("var cursor = coll.Aggregate(pipeline);\n")
	} else {
		fmt.Fprintf(&body, "var filter = %s;\n", r.value(s.Filter, ""))
		chain := "coll.Find(filter)"
		if len(s.Projection) > 0 {
			fmt.Fprintf(&body, "var projection = %s;\n", r.value(s.Projection, ""))
			chain += ".Project(projection)"
		}
		if len(s.Sort) > 0 {
			fmt.Fprintf(&body, "var sort = %s;\n", r.value(s.Sort, ""))
			chain += ".Sort(sort)"
		}
		if s.Skip > 0 {
			chain += fmt.Sprintf(".Skip(%d)", s.Skip)
		}
		if s.Limit > 0 {
			chain += fmt.Sprintf(".Limit(%d)", s.Limit)
		}
		fmt.Fprintf(&body, "\nvar cursor = %s;\n", chain)
	}
	body.WriteString("foreach (var doc in cursor.ToEnumerable())\n{\n    Console.WriteLine(doc);\n}\n")

	var b strings.Builder
	b.WriteString("// dotnet add package MongoDB.Driver\nusing MongoDB.Bson;\nusing MongoDB.Driver;\n\n")
	b.WriteString(`var uri = Environment.GetEnvironmentVariable("MONGODB_URI") ?? "mongodb://localhost:27017";` + "\n")
	b.WriteString("var client = new MongoClient(uri);\n")
	b.WriteString(body.String())
	return b.String()
}

// ---- Go ---------------------------------------------------------------------

func goRenderer() *renderer {
	r := &renderer{used: map[string]bool{}, indent: "\t"}
	r.doc = func(r *renderer, d bson.D, ind string) string {
		if len(d) == 0 {
			return "bson.D{}"
		}
		parts := make([]string, len(d))
		for i, e := range d {
			parts[i] = fmt.Sprintf("{Key: %s, Value: %s}", quote(e.Key), r.value(e.Value, ind+r.indent))
		}
		if s, ok := inline(parts, "bson.D{", "}", 90); ok {
			return s
		}
		return goBlock(parts, "bson.D{", ind)
	}
	r.arr = func(r *renderer, a bson.A, ind string) string {
		if len(a) == 0 {
			return "bson.A{}"
		}
		parts := make([]string, len(a))
		for i, e := range a {
			parts[i] = r.value(e, ind+r.indent)
		}
		if s, ok := inline(parts, "bson.A{", "}", 90); ok {
			return s
		}
		return goBlock(parts, "bson.A{", ind)
	}
	r.scalar = func(r *renderer, v any) string {
		switch t := v.(type) {
		case nil, bson.Null:
			return "nil"
		case bson.Undefined:
			return "bson.Undefined{}"
		case bool:
			return fmt.Sprint(t)
		case int32:
			return fmt.Sprint(t) // Go int literals encode as int32 when they fit
		case int64:
			return fmt.Sprintf("int64(%d)", t)
		case float64:
			r.needIf(math.IsNaN(t) || math.IsInf(t, 0), "math")
			return floatLit(t, "math.NaN()", "math.Inf(1)", "math.Inf(-1)")
		case string:
			return quote(t)
		case bson.ObjectID:
			r.need("oid")
			return fmt.Sprintf("oid(%q)", t.Hex())
		case bson.DateTime:
			r.need("time")
			tm := timeParts(t)
			return fmt.Sprintf("time.Date(%d, %d, %d, %d, %d, %d, %d, time.UTC)",
				tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond())
		case bson.Decimal128:
			r.need("dec")
			return fmt.Sprintf("dec(%q)", t.String())
		case bson.Regex:
			return fmt.Sprintf("bson.Regex{Pattern: %s, Options: %q}", quote(t.Pattern), t.Options)
		case bson.Binary:
			r.need("hexBytes")
			return fmt.Sprintf("bson.Binary{Subtype: %d, Data: hexBytes(%q)}", t.Subtype, hexString(t.Data))
		case bson.Timestamp:
			return fmt.Sprintf("bson.Timestamp{T: %d, I: %d}", t.T, t.I)
		case bson.MinKey:
			return "bson.MinKey{}"
		case bson.MaxKey:
			return "bson.MaxKey{}"
		case bson.JavaScript:
			return fmt.Sprintf("bson.JavaScript(%s)", quote(string(t)))
		}
		return quote(fmt.Sprint(v))
	}
	return r
}

// goBlock is a multi-line Go composite literal; Go requires the trailing comma.
func goBlock(parts []string, open, ind string) string {
	return open + "\n" + ind + "\t" + strings.Join(parts, ",\n"+ind+"\t") + ",\n" + ind + "}"
}

func (r *renderer) needIf(cond bool, s string) {
	if cond {
		r.need(s)
	}
}

func genGo(s Spec) string {
	r := goRenderer()
	var body strings.Builder
	in := "\t"
	fmt.Fprintf(&body, "%scoll := client.Database(%s).Collection(%s)\n", in, quote(s.DB), quote(s.Coll))
	if s.Kind == "aggregate" {
		stages := make([]string, len(s.Pipeline))
		for i, st := range s.Pipeline {
			stages[i] = r.value(st, in+"\t")
		}
		fmt.Fprintf(&body, "%spipeline := mongo.Pipeline{\n%s\t%s,\n%s}\n", in, in, strings.Join(stages, ",\n"+in+"\t"), in)
		fmt.Fprintf(&body, "%scursor, err := coll.Aggregate(ctx, pipeline)\n", in)
	} else {
		fmt.Fprintf(&body, "%sfilter := %s\n", in, r.value(s.Filter, in))
		opts := "options.Find()"
		if len(s.Projection) > 0 {
			opts += ".\n" + in + "\tSetProjection(" + r.value(s.Projection, in+"\t") + ")"
		}
		if len(s.Sort) > 0 {
			opts += ".\n" + in + "\tSetSort(" + r.value(s.Sort, in+"\t") + ")"
		}
		if s.Skip > 0 {
			opts += fmt.Sprintf(".\n%s\tSetSkip(%d)", in, s.Skip)
		}
		if s.Limit > 0 {
			opts += fmt.Sprintf(".\n%s\tSetLimit(%d)", in, s.Limit)
		}
		fmt.Fprintf(&body, "%sopts := %s\n", in, opts)
		fmt.Fprintf(&body, "%scursor, err := coll.Find(ctx, filter, opts)\n", in)
	}
	body.WriteString(`	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Fatal(err)
		}
		fmt.Println(doc)
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
}
`)
	imports := []string{`"context"`, `"fmt"`, `"log"`}
	if r.used["math"] {
		imports = append(imports, `"math"`)
	}
	imports = append(imports, `"os"`)
	if r.used["time"] {
		imports = append(imports, `"time"`)
	}
	if r.used["hexBytes"] {
		imports = append(imports, `"encoding/hex"`)
	}
	sort.Strings(imports)
	var b strings.Builder
	b.WriteString("// go get go.mongodb.org/mongo-driver/v2\npackage main\n\nimport (\n")
	for _, i := range imports {
		b.WriteString("\t" + i + "\n")
	}
	b.WriteString("\n\t\"go.mongodb.org/mongo-driver/v2/bson\"\n\t\"go.mongodb.org/mongo-driver/v2/mongo\"\n\t\"go.mongodb.org/mongo-driver/v2/mongo/options\"\n)\n\n")
	b.WriteString(`func main() {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	defer client.Disconnect(ctx)

`)
	b.WriteString(body.String())
	if r.used["oid"] {
		b.WriteString(`
func oid(s string) bson.ObjectID {
	id, err := bson.ObjectIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return id
}
`)
	}
	if r.used["dec"] {
		b.WriteString(`
func dec(s string) bson.Decimal128 {
	d, err := bson.ParseDecimal128(s)
	if err != nil {
		panic(err)
	}
	return d
}
`)
	}
	if r.used["hexBytes"] {
		b.WriteString(`
func hexBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
`)
	}
	return b.String()
}
