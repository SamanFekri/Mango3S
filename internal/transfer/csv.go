package transfer

import (
	"bufio"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// Column types for CSV import.
const (
	TypeAuto     = "auto"
	TypeString   = "string"
	TypeInt      = "int"
	TypeLong     = "long"
	TypeDouble   = "double"
	TypeDecimal  = "decimal"
	TypeBool     = "bool"
	TypeDate     = "date"
	TypeObjectID = "objectId"
	TypeJSON     = "json" // the cell holds a shell-syntax / Extended JSON value
)

// ValidTypes lists the accepted column types.
var ValidTypes = []string{TypeAuto, TypeString, TypeInt, TypeLong, TypeDouble, TypeDecimal, TypeBool, TypeDate, TypeObjectID, TypeJSON}

// ColumnSpec overrides how one CSV column is imported.
type ColumnSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Skip bool   `json:"skip"`
}

// CSVOptions configures CSV reading.
type CSVOptions struct {
	Delimiter rune
	Columns   []ColumnSpec // by header name; unspecified columns use TypeAuto
	// KeepBlanks imports empty cells as empty strings instead of omitting the field.
	KeepBlanks bool
}

// CSVReader reads a CSV file with a header row. Dotted header names
// ("address.city") produce nested documents.
type CSVReader struct {
	r      *csv.Reader
	header []string
	types  []string
	skip   []bool
	opts   CSVOptions
	row    int
}

// NewCSVReader reads the header row and prepares column conversions.
func NewCSVReader(r io.Reader, opts CSVOptions) (*CSVReader, error) {
	if opts.Delimiter == 0 {
		opts.Delimiter = ','
	}
	br := bufio.NewReaderSize(r, 64<<10)
	if b, err := br.Peek(3); err == nil && string(b) == string(utf8BOM) {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.Comma = opts.Delimiter
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	header, err := cr.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("the file is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("header row: %w", err)
	}
	c := &CSVReader{r: cr, opts: opts, row: 1}
	seen := map[string]bool{}
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("field%d", i+1)
		}
		if seen[h] {
			return nil, fmt.Errorf("duplicate column %q in header", h)
		}
		seen[h] = true
		c.header = append(c.header, h)
		c.types = append(c.types, TypeAuto)
		c.skip = append(c.skip, false)
	}
	for _, spec := range opts.Columns {
		for i, h := range c.header {
			if h != spec.Name {
				continue
			}
			c.skip[i] = spec.Skip
			if spec.Type != "" {
				if !validType(spec.Type) {
					return nil, fmt.Errorf("column %q: unknown type %q", h, spec.Type)
				}
				c.types[i] = spec.Type
			}
		}
	}
	return c, nil
}

func validType(t string) bool {
	for _, v := range ValidTypes {
		if v == t {
			return true
		}
	}
	return false
}

// Header returns the column names.
func (c *CSVReader) Header() []string { return c.header }

// Position implements Reader.
func (c *CSVReader) Position() string { return fmt.Sprintf("row %d", c.row) }

// NextRecord returns the raw cells of the next row.
func (c *CSVReader) NextRecord() ([]string, error) {
	rec, err := c.r.Read()
	if err != nil {
		return nil, err
	}
	c.row++
	return rec, nil
}

// Next implements Reader.
func (c *CSVReader) Next() (bson.D, error) {
	for {
		rec, err := c.NextRecord()
		if err != nil {
			if pe, ok := err.(*csv.ParseError); ok {
				return nil, fmt.Errorf("line %d: %v", pe.Line, pe.Err)
			}
			return nil, err
		}
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue // blank line
		}
		if len(rec) > len(c.header) {
			return nil, fmt.Errorf("has %d cells but the header has %d columns", len(rec), len(c.header))
		}
		doc := bson.D{}
		for i, cell := range rec {
			if c.skip[i] {
				continue
			}
			if cell == "" && !c.opts.KeepBlanks {
				continue
			}
			v, err := ConvertCell(cell, c.types[i])
			if err != nil {
				return nil, fmt.Errorf("column %q: %w", c.header[i], err)
			}
			doc, err = setPath(doc, strings.Split(c.header[i], "."), v)
			if err != nil {
				return nil, fmt.Errorf("column %q: %w", c.header[i], err)
			}
		}
		return doc, nil
	}
}

// setPath sets a dotted path in d, creating nested documents.
func setPath(d bson.D, path []string, v any) (bson.D, error) {
	for i := range d {
		if d[i].Key != path[0] {
			continue
		}
		if len(path) == 1 {
			return nil, fmt.Errorf("field %q set twice", path[0])
		}
		sub, ok := d[i].Value.(bson.D)
		if !ok {
			return nil, fmt.Errorf("%q is both a value and a document", path[0])
		}
		nested, err := setPath(sub, path[1:], v)
		if err != nil {
			return nil, err
		}
		d[i].Value = nested
		return d, nil
	}
	if len(path) == 1 {
		return append(d, bson.E{Key: path[0], Value: v}), nil
	}
	nested, err := setPath(bson.D{}, path[1:], v)
	if err != nil {
		return nil, err
	}
	return append(d, bson.E{Key: path[0], Value: nested}), nil
}

var (
	intRe      = regexp.MustCompile(`^[-+]?\d+$`)
	floatRe    = regexp.MustCompile(`^[-+]?(\d+\.\d*|\.\d+|\d+)([eE][-+]?\d+)?$`)
	objectIDRe = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)
	// Only full timestamps are auto-detected as dates; a bare "2024-01-31"
	// stays a string unless the column type says date.
	dateTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})$`)
)

// InferType returns the type TypeAuto would pick for a cell.
func InferType(cell string) string {
	s := strings.TrimSpace(cell)
	switch {
	case s == "":
		return ""
	case s == "true" || s == "false":
		return TypeBool
	case intRe.MatchString(s):
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			if n >= math.MinInt32 && n <= math.MaxInt32 {
				return TypeInt
			}
			return TypeLong
		}
		return TypeString
	case floatRe.MatchString(s):
		return TypeDouble
	case objectIDRe.MatchString(s):
		return TypeObjectID
	case dateTimeRe.MatchString(s):
		return TypeDate
	case (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) || (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")):
		if _, err := mql.ParseValue(s); err == nil {
			return TypeJSON
		}
	}
	return TypeString
}

// ConvertCell converts one CSV cell to a BSON value of the given type.
func ConvertCell(cell, typ string) (any, error) {
	if typ == TypeAuto || typ == "" {
		typ = InferType(cell)
		if typ == "" {
			return "", nil
		}
	}
	s := strings.TrimSpace(cell)
	switch typ {
	case TypeString:
		return cell, nil
	case TypeInt:
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%q is not a 32-bit integer", cell)
		}
		return int32(n), nil
	case TypeLong:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a 64-bit integer", cell)
		}
		return n, nil
	case TypeDouble:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", cell)
		}
		return f, nil
	case TypeDecimal:
		d, err := bson.ParseDecimal128(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not a decimal", cell)
		}
		return d, nil
	case TypeBool:
		switch strings.ToLower(s) {
		case "true", "1", "yes", "y":
			return true, nil
		case "false", "0", "no", "n":
			return false, nil
		}
		return nil, fmt.Errorf("%q is not a boolean", cell)
	case TypeDate:
		if t, err := mql.ParseDate(s); err == nil {
			return bson.NewDateTimeFromTime(t), nil
		}
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			return bson.DateTime(ms), nil
		}
		return nil, fmt.Errorf("%q is not a date (use ISO-8601)", cell)
	case TypeObjectID:
		oid, err := bson.ObjectIDFromHex(s)
		if err != nil {
			return nil, fmt.Errorf("%q is not an ObjectId", cell)
		}
		return oid, nil
	case TypeJSON:
		v, err := mql.ParseValue(s)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, fmt.Errorf("unknown type %q", typ)
}

// CSVWriter writes documents as CSV rows with the given dotted field paths
// as columns. Values are rendered spreadsheet-friendly: ObjectIds as hex,
// dates as ISO-8601, nested documents and arrays as relaxed Extended JSON.
type CSVWriter struct {
	w      *csv.Writer
	fields []string
	row    []string
}

// NewCSVWriter writes the header immediately when header is true.
func NewCSVWriter(w io.Writer, fields []string, delimiter rune, header bool) (*CSVWriter, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("CSV export needs at least one field")
	}
	if delimiter == 0 {
		delimiter = ','
	}
	cw := csv.NewWriter(w)
	cw.Comma = delimiter
	if header {
		if err := cw.Write(fields); err != nil {
			return nil, err
		}
	}
	return &CSVWriter{w: cw, fields: fields, row: make([]string, len(fields))}, nil
}

// Write implements Writer.
func (c *CSVWriter) Write(d bson.D) error {
	for i, f := range c.fields {
		v, ok := LookupPath(d, f)
		if !ok {
			c.row[i] = ""
			continue
		}
		c.row[i] = FormatCell(v)
	}
	return c.w.Write(c.row)
}

// Close implements Writer.
func (c *CSVWriter) Close() error {
	c.w.Flush()
	return c.w.Error()
}

// LookupPath resolves a dotted path through nested documents.
func LookupPath(d bson.D, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = d
	for _, p := range parts {
		doc, ok := cur.(bson.D)
		if !ok {
			return nil, false
		}
		v, found := mql.Lookup(doc, p)
		if !found {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// FormatCell renders a value for a CSV cell.
func FormatCell(v any) string {
	switch t := v.(type) {
	case nil, bson.Null, bson.Undefined:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return mql.FormatDouble(t)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bson.Decimal128:
		return t.String()
	case bson.ObjectID:
		return t.Hex()
	case bson.DateTime:
		return t.Time().UTC().Format("2006-01-02T15:04:05.000Z")
	case time.Time:
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	case bson.Binary:
		return base64.StdEncoding.EncodeToString(t.Data)
	case bson.Regex:
		return "/" + t.Pattern + "/" + t.Options
	case bson.Timestamp:
		return fmt.Sprintf("%d:%d", t.T, t.I)
	case bson.Symbol:
		return string(t)
	case bson.JavaScript:
		return string(t)
	}
	data, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: v}}, false, false)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(data)
	return s[len(`{"v":`) : len(s)-1]
}

// LeafPaths collects dotted paths of non-document values (arrays count as
// leaves) in first-seen order, used to pick CSV columns.
func LeafPaths(d bson.D, prefix string, seen map[string]bool, out *[]string) {
	for _, e := range d {
		p := e.Key
		if prefix != "" {
			p = prefix + "." + e.Key
		}
		if sub, ok := e.Value.(bson.D); ok && len(sub) > 0 {
			LeafPaths(sub, p, seen, out)
			continue
		}
		if !seen[p] {
			seen[p] = true
			*out = append(*out, p)
		}
	}
}

// ParseDelimiter accepts ",", ";", "\t", "tab", "|" and the like.
func ParseDelimiter(s string) (rune, error) {
	switch s {
	case "", ",":
		return ',', nil
	case "\\t", "\t", "tab", "TAB":
		return '\t', nil
	}
	r, size := utf8.DecodeRuneInString(s)
	if size != len(s) || r == '"' || r == '\n' || r == '\r' {
		return 0, fmt.Errorf("invalid delimiter %q", s)
	}
	return r, nil
}
