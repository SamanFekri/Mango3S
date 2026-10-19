// Package sqlexport writes MongoDB collections as SQL scripts (CREATE TABLE
// plus batched INSERTs) for PostgreSQL, MySQL and SQLite.
//
// Columns come from a sample of each collection. Nothing is dropped
// silently: fields that are not columns, and values whose type doesn't fit
// their column, are kept as JSON in an "_extra" column and counted in the
// report.
package sqlexport

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Dialects.
const (
	Postgres = "postgres"
	MySQL    = "mysql"
	SQLite   = "sqlite"
)

// ExtraColumn holds fields and values that don't fit the table's columns.
const ExtraColumn = "_extra"

// Options controls an export.
type Options struct {
	Dialect    string `json:"dialect"`
	Table      string `json:"table"`      // default: the collection name
	Filter     string `json:"filter"`     // canonical Extended JSON
	SampleSize int    `json:"sampleSize"` // documents used to choose columns; default 1000
	BatchRows  int    `json:"batchRows"`  // rows per INSERT; default 200
	Flatten    bool   `json:"flatten"`    // nested documents become parent_child columns
	DropTable  bool   `json:"dropTable"`  // emit DROP TABLE IF EXISTS first
}

// Logger receives progress.
type Logger interface {
	Log(format string, args ...any)
	SetTotal(total int64, unit string)
	Add(n int64)
}

// Report summarizes an export.
type Report struct {
	Table        string   `json:"table"`
	Rows         int64    `json:"rows"`
	Columns      []Column `json:"columns"`
	ExtraFields  int64    `json:"extraFields"`  // values kept in _extra because the field isn't a column
	TypeMismatch int64    `json:"typeMismatch"` // values kept in _extra because their type doesn't fit the column
	Notes        []string `json:"notes"`
}

// Column is one table column.
type Column struct {
	Name    string `json:"name"` // SQL column name
	Path    string `json:"path"` // document field path (dot-separated)
	Kind    string `json:"kind"` // int | long | double | decimal | bool | date | objectId | string | binary | json
	SQLType string `json:"sqlType"`
	Mixed   bool   `json:"mixed"` // several BSON types were seen; stored as JSON text
}

func (o *Options) normalize() error {
	switch o.Dialect {
	case Postgres, MySQL, SQLite:
	default:
		return fmt.Errorf("unknown SQL dialect %q (use postgres, mysql or sqlite)", o.Dialect)
	}
	if o.SampleSize <= 0 {
		o.SampleSize = 1000
	}
	if o.SampleSize > 100000 {
		o.SampleSize = 100000
	}
	if o.BatchRows <= 0 {
		o.BatchRows = 200
	}
	if o.BatchRows > 5000 {
		o.BatchRows = 5000
	}
	return nil
}

// kindOf classifies a BSON value for column typing.
func kindOf(v bson.RawValue) string {
	switch v.Type {
	case bson.TypeInt32:
		return "int"
	case bson.TypeInt64:
		return "long"
	case bson.TypeDouble:
		return "double"
	case bson.TypeDecimal128:
		return "decimal"
	case bson.TypeBoolean:
		return "bool"
	case bson.TypeDateTime:
		return "date"
	case bson.TypeObjectID:
		return "objectId"
	case bson.TypeString, bson.TypeSymbol:
		return "string"
	case bson.TypeBinary:
		if sub, _ := v.Binary(); sub == 4 || sub == 3 {
			return "json" // UUIDs keep their subtype in Extended JSON
		}
		return "binary"
	case bson.TypeNull, bson.TypeUndefined:
		return ""
	}
	return "json"
}

// merge combines two column kinds; numbers widen, anything else becomes JSON.
func merge(a, b string) (string, bool) {
	switch {
	case a == "" || a == b:
		return b, false
	case b == "":
		return a, false
	}
	rank := map[string]int{"int": 1, "long": 2, "double": 3}
	if rank[a] > 0 && rank[b] > 0 {
		if rank[a] > rank[b] {
			return a, false
		}
		return b, false
	}
	return "json", true
}

func sqlType(dialect, kind string) string {
	t := map[string][3]string{ // postgres, mysql, sqlite
		"int":      {"INTEGER", "INT", "INTEGER"},
		"long":     {"BIGINT", "BIGINT", "INTEGER"},
		"double":   {"DOUBLE PRECISION", "DOUBLE", "REAL"},
		"decimal":  {"NUMERIC", "DECIMAL(65,30)", "TEXT"},
		"bool":     {"BOOLEAN", "BOOLEAN", "INTEGER"},
		"date":     {"TIMESTAMP(3)", "DATETIME(3)", "TEXT"},
		"objectId": {"CHAR(24)", "CHAR(24)", "TEXT"},
		"string":   {"TEXT", "LONGTEXT", "TEXT"},
		"binary":   {"BYTEA", "LONGBLOB", "BLOB"},
		"json":     {"JSONB", "JSON", "TEXT"},
	}[kind]
	switch dialect {
	case Postgres:
		return t[0]
	case MySQL:
		return t[1]
	}
	return t[2]
}

// collect walks a document's fields (flattening nested documents when asked).
func collect(doc bson.Raw, prefix string, flatten bool, f func(path string, v bson.RawValue)) {
	elems, err := doc.Elements()
	if err != nil {
		return
	}
	for _, e := range elems {
		path := e.Key()
		if prefix != "" {
			path = prefix + "." + path
		}
		v := e.Value()
		if flatten && v.Type == bson.TypeEmbeddedDocument {
			sub, _ := v.DocumentOK()
			if len(sub) > 5 { // non-empty
				collect(sub, path, flatten, f)
				continue
			}
		}
		f(path, v)
	}
}

// identifier makes a safe SQL column or table name.
func identifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '.':
			b.WriteByte('_')
		case r < 32 || r == '"' || r == '`' || r == '\\':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 63 {
		out = out[:63]
	}
	if out == "" {
		out = "_"
	}
	return out
}

type exporter struct {
	o       Options
	cols    []Column
	byPath  map[string]int
	idCol   int
	w       *bufio.Writer
	rep     Report
	pending []string
}

func (x *exporter) quote(id string) string {
	if x.o.Dialect == MySQL {
		return "`" + strings.ReplaceAll(id, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

func (x *exporter) str(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	if x.o.Dialect == MySQL {
		s = strings.ReplaceAll(s, `\`, `\\`)
	}
	if strings.ContainsRune(s, 0) {
		s = strings.ReplaceAll(s, "\x00", "")
		x.rep.Notes = appendOnce(x.rep.Notes, "NUL characters in strings were removed (SQL text can't hold them).")
	}
	return "'" + s + "'"
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// extJSON renders a value as canonical Extended JSON (exact types).
func extJSON(v bson.RawValue) string {
	b, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: v}}, true, false)
	if err != nil {
		return "null"
	}
	s := string(b)
	// {"v":<value>}
	return strings.TrimSuffix(strings.TrimPrefix(s, `{"v":`), "}")
}

// literal renders v for column kind; ok=false when the value doesn't fit.
func (x *exporter) literal(kind string, v bson.RawValue) (string, bool) {
	if v.Type == bson.TypeNull || v.Type == bson.TypeUndefined {
		return "NULL", true
	}
	switch kind {
	case "int", "long", "double":
		// Only exact conversions: a double never goes into an integer column,
		// and integers beyond 2^53 never go into a double column.
		switch v.Type {
		case bson.TypeInt32:
			return strconv.FormatInt(int64(v.Int32()), 10), true
		case bson.TypeInt64:
			n := v.Int64()
			if kind == "int" && (n < math.MinInt32 || n > math.MaxInt32) {
				return "", false
			}
			if kind == "double" && (n > 1<<53 || n < -(1<<53)) {
				return "", false
			}
			return strconv.FormatInt(n, 10), true
		case bson.TypeDouble:
			f := v.Double()
			if kind != "double" || math.IsNaN(f) || math.IsInf(f, 0) {
				return "", false
			}
			return strconv.FormatFloat(f, 'g', -1, 64), true
		}
	case "decimal":
		if v.Type == bson.TypeDecimal128 {
			s := v.Decimal128().String()
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				return "", false // NaN / Infinity
			}
			plain, ok := plainDecimal(s)
			if !ok {
				return "", false
			}
			if x.o.Dialect == MySQL {
				intPart, frac, _ := strings.Cut(strings.TrimPrefix(plain, "-"), ".")
				if len(intPart) > 35 || len(frac) > 30 {
					return "", false // DECIMAL(65,30) would round it
				}
			}
			if x.o.Dialect == SQLite {
				return "'" + plain + "'", true // TEXT keeps every digit
			}
			return plain, true
		}
	case "bool":
		if v.Type == bson.TypeBoolean {
			b := v.Boolean()
			if x.o.Dialect == SQLite {
				return map[bool]string{true: "1", false: "0"}[b], true
			}
			return strings.ToUpper(strconv.FormatBool(b)), true
		}
	case "date":
		if v.Type == bson.TypeDateTime {
			t := time.UnixMilli(v.DateTime()).UTC()
			if t.Year() < 1 || t.Year() > 9999 {
				return "", false
			}
			return "'" + t.Format("2006-01-02 15:04:05.000") + "'", true
		}
	case "objectId":
		if v.Type == bson.TypeObjectID {
			return "'" + v.ObjectID().Hex() + "'", true
		}
	case "string":
		switch v.Type {
		case bson.TypeString:
			return x.str(v.StringValue()), true
		case bson.TypeSymbol:
			return x.str(v.Symbol()), true
		}
	case "binary":
		if v.Type == bson.TypeBinary {
			_, data := v.Binary()
			h := hex.EncodeToString(data)
			if x.o.Dialect == Postgres {
				return `'\x` + h + `'::bytea`, true
			}
			return "X'" + h + "'", true
		}
	case "json":
		s := x.str(extJSON(v))
		if x.o.Dialect == Postgres {
			return s + "::jsonb", true
		}
		return s, true
	}
	return "", false
}

// plan samples documents to choose columns.
func plan(ctx context.Context, c *mongo.Collection, filter bson.D, o Options) ([]Column, []string, error) {
	cur, err := c.Find(ctx, filter, options.Find().SetLimit(int64(o.SampleSize)).SetSort(bson.D{{Key: "$natural", Value: 1}}))
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(context.Background())
	kinds := map[string]string{}
	mixed := map[string]bool{}
	var order []string
	for cur.Next(ctx) {
		collect(cur.Current, "", o.Flatten, func(path string, v bson.RawValue) {
			k := kindOf(v)
			prev, seen := kinds[path]
			if !seen {
				order = append(order, path)
			}
			m, mix := merge(prev, k)
			kinds[path] = m
			if mix {
				mixed[path] = true
			}
		})
	}
	if err := cur.Err(); err != nil {
		return nil, nil, err
	}
	// _id first, then fields in first-seen order.
	sort.SliceStable(order, func(i, j int) bool { return order[i] == "_id" && order[j] != "_id" })
	var notes []string
	cols := []Column{}
	names := map[string]bool{strings.ToLower(ExtraColumn): true}
	for _, p := range order {
		k := kinds[p]
		if k == "" {
			k = "string" // only nulls seen
		}
		name := identifier(p)
		for i := 2; names[strings.ToLower(name)]; i++ {
			name = fmt.Sprintf("%s_%d", identifier(p), i)
		}
		names[strings.ToLower(name)] = true
		cols = append(cols, Column{Name: name, Path: p, Kind: k, SQLType: sqlType(o.Dialect, k), Mixed: mixed[p]})
		if mixed[p] {
			notes = append(notes, fmt.Sprintf("%s holds several types; it is stored as Extended JSON text.", p))
		}
	}
	return cols, notes, nil
}

// Export writes one collection as SQL.
func Export(ctx context.Context, c *mongo.Collection, out io.Writer, o Options, log Logger) (Report, error) {
	if err := o.normalize(); err != nil {
		return Report{}, err
	}
	var filter bson.D
	if strings.TrimSpace(o.Filter) != "" {
		if err := bson.UnmarshalExtJSON([]byte(o.Filter), false, &filter); err != nil {
			return Report{}, fmt.Errorf("filter is not valid Extended JSON: %w", err)
		}
	} else {
		filter = bson.D{}
	}
	table := o.Table
	if table == "" {
		table = c.Name()
	}
	x := &exporter{o: o, w: bufio.NewWriterSize(out, 256<<10), byPath: map[string]int{}, idCol: -1}
	x.rep.Table = identifier(table)
	cols, notes, err := plan(ctx, c, filter, o)
	if err != nil {
		return x.rep, err
	}
	if len(cols) == 0 {
		return x.rep, errors.New("the collection has no documents to describe a table")
	}
	x.cols = cols
	x.rep.Columns = cols
	x.rep.Notes = append(x.rep.Notes, notes...)
	for i, col := range cols {
		x.byPath[col.Path] = i
		if col.Path == "_id" && !col.Mixed && col.Kind != "json" {
			x.idCol = i
		}
	}
	if n, err := c.CountDocuments(ctx, filter); err == nil {
		log.SetTotal(n, "docs")
	}
	x.header(c)
	cur, err := c.Find(ctx, filter, options.Find().SetBatchSize(1000))
	if err != nil {
		return x.rep, err
	}
	defer cur.Close(context.Background())
	for cur.Next(ctx) {
		if err := x.row(cur.Current); err != nil {
			return x.rep, err
		}
		x.rep.Rows++
		if x.rep.Rows%1000 == 0 {
			log.Add(1000)
		}
	}
	log.Add(x.rep.Rows % 1000)
	if err := cur.Err(); err != nil {
		return x.rep, err
	}
	if err := x.flush(); err != nil {
		return x.rep, err
	}
	x.footer()
	if x.rep.ExtraFields > 0 {
		x.rep.Notes = append(x.rep.Notes, fmt.Sprintf("%d values of fields missing from the sample were kept in %s.", x.rep.ExtraFields, ExtraColumn))
	}
	if x.rep.TypeMismatch > 0 {
		x.rep.Notes = append(x.rep.Notes, fmt.Sprintf("%d values whose type didn't fit their column were kept in %s.", x.rep.TypeMismatch, ExtraColumn))
	}
	if x.rep.Notes == nil {
		x.rep.Notes = []string{}
	}
	return x.rep, x.w.Flush()
}

func (x *exporter) header(c *mongo.Collection) {
	w := x.w
	fmt.Fprintf(w, "-- Exported by Mango3S from %s.%s\n", c.Database().Name(), c.Name())
	fmt.Fprintf(w, "-- Dialect: %s. Fields that are not columns, and values whose type doesn't fit, are kept as JSON in %s.\n", x.o.Dialect, ExtraColumn)
	if x.o.Dialect == MySQL {
		fmt.Fprintln(w, "SET NAMES utf8mb4;")
		fmt.Fprintln(w, "SET time_zone = '+00:00';")
	}
	if x.o.Dialect != SQLite {
		fmt.Fprintln(w, "BEGIN;")
	} else {
		fmt.Fprintln(w, "BEGIN TRANSACTION;")
	}
	tbl := x.quote(x.rep.Table)
	if x.o.DropTable {
		fmt.Fprintf(w, "DROP TABLE IF EXISTS %s;\n", tbl)
	}
	fmt.Fprintf(w, "CREATE TABLE %s (\n", tbl)
	for i, col := range x.cols {
		def := "  " + x.quote(col.Name) + " " + col.SQLType
		if i == x.idCol {
			def += " NOT NULL PRIMARY KEY"
			if x.o.Dialect == MySQL && (col.Kind == "string" || col.Kind == "binary") {
				// MySQL can't index LONGTEXT/LONGBLOB without a prefix length.
				def = "  " + x.quote(col.Name) + " VARCHAR(768) NOT NULL PRIMARY KEY"
				if col.Kind == "binary" {
					def = "  " + x.quote(col.Name) + " VARBINARY(768) NOT NULL PRIMARY KEY"
				}
			}
		}
		fmt.Fprintln(w, def+",")
	}
	fmt.Fprintf(w, "  %s %s\n);\n", x.quote(ExtraColumn), sqlType(x.o.Dialect, "json"))
}

func (x *exporter) footer() { fmt.Fprintln(x.w, "COMMIT;") }

func (x *exporter) row(doc bson.Raw) error {
	vals := make([]string, len(x.cols))
	for i := range vals {
		vals[i] = "NULL"
	}
	var extra []string
	collect(doc, "", x.o.Flatten, func(path string, v bson.RawValue) {
		i, ok := x.byPath[path]
		if !ok {
			x.rep.ExtraFields++
			k, _ := json.Marshal(path)
			extra = append(extra, string(k)+":"+extJSON(v))
			return
		}
		col := x.cols[i]
		lit, fits := x.literal(col.Kind, v)
		if !fits {
			x.rep.TypeMismatch++
			k, _ := json.Marshal(path)
			extra = append(extra, string(k)+":"+extJSON(v))
			return
		}
		vals[i] = lit
	})
	if x.idCol >= 0 && vals[x.idCol] == "NULL" {
		return fmt.Errorf("a document's _id doesn't fit the %s primary key column; export with a larger sample so _id is typed as JSON", x.cols[x.idCol].Kind)
	}
	ex := "NULL"
	if len(extra) > 0 {
		lit := x.str("{" + strings.Join(extra, ",") + "}")
		if x.o.Dialect == Postgres {
			lit += "::jsonb"
		}
		ex = lit
	}
	x.pending = append(x.pending, "("+strings.Join(vals, ", ")+", "+ex+")")
	if len(x.pending) >= x.o.BatchRows {
		return x.flush()
	}
	return nil
}

func (x *exporter) flush() error {
	if len(x.pending) == 0 {
		return nil
	}
	names := make([]string, 0, len(x.cols)+1)
	for _, c := range x.cols {
		names = append(names, x.quote(c.Name))
	}
	names = append(names, x.quote(ExtraColumn))
	fmt.Fprintf(x.w, "INSERT INTO %s (%s) VALUES\n%s;\n", x.quote(x.rep.Table), strings.Join(names, ", "), strings.Join(x.pending, ",\n"))
	x.pending = x.pending[:0]
	return nil
}

// plainDecimal rewrites a Decimal128 string ("1.5E+3", "-0.00012") without
// an exponent, so every SQL dialect reads it exactly.
func plainDecimal(s string) (string, bool) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, expStr, hasExp := strings.Cut(strings.ToUpper(s), "E")
	exp := 0
	if hasExp {
		n, err := strconv.Atoi(expStr)
		if err != nil {
			return "", false
		}
		exp = n
	}
	intPart, frac, _ := strings.Cut(mant, ".")
	digits := intPart + frac
	point := len(intPart) + exp
	if point > 80 || point < -80 {
		return "", false
	}
	switch {
	case point <= 0:
		digits = strings.Repeat("0", -point+1) + digits
		point = 1
	case point > len(digits):
		digits += strings.Repeat("0", point-len(digits))
	}
	ip, fp := strings.TrimLeft(digits[:point], "0"), strings.TrimRight(digits[point:], "0")
	if ip == "" {
		ip = "0"
	}
	out := ip
	if fp != "" {
		out += "." + fp
	}
	if neg && out != "0" {
		out = "-" + out
	}
	return out, true
}
