// Package mql is the shared query/document model. It parses the relaxed,
// mongosh-style literal syntax users type into editors (unquoted keys, single
// quotes, comments, trailing commas, ObjectId(...), ISODate(...), /regex/ ...)
// as well as strict/relaxed Extended JSON, producing order-preserving BSON
// values. Format performs the inverse so documents round-trip losslessly.
package mql

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// SyntaxError reports a parse failure with a 1-based line and column.
type SyntaxError struct {
	Line, Col int
	Msg       string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("syntax error at line %d, column %d: %s", e.Line, e.Col, e.Msg)
}

// Parser is a recursive-descent parser over a source string. Its primitives
// are exported so the shell package can parse command syntax around values.
type Parser struct {
	src []rune
	pos int
}

// NewParser returns a parser positioned at the start of src.
func NewParser(src string) *Parser { return &Parser{src: []rune(src)} }

// ParseValue parses a single value and requires the whole input to be consumed.
func ParseValue(src string) (any, error) {
	p := NewParser(src)
	if p.EOF() {
		return nil, p.Errorf("expected a value")
	}
	v, err := p.Value()
	if err != nil {
		return nil, err
	}
	if !p.EOF() {
		return nil, p.Errorf("unexpected %q after value", p.Peek())
	}
	return v, nil
}

// ParseDocument parses a document. Blank input yields an empty document.
func ParseDocument(src string) (bson.D, error) {
	if strings.TrimSpace(stripComments(src)) == "" {
		return bson.D{}, nil
	}
	v, err := ParseValue(src)
	if err != nil {
		return nil, err
	}
	d, ok := v.(bson.D)
	if !ok {
		return nil, &SyntaxError{Line: 1, Col: 1, Msg: fmt.Sprintf("expected a document {...}, got %s", TypeName(v))}
	}
	return d, nil
}

// ParseDocuments parses either a single document or an array of documents.
func ParseDocuments(src string) ([]bson.D, error) {
	v, err := ParseValue(src)
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case bson.D:
		return []bson.D{t}, nil
	case bson.A:
		out := make([]bson.D, 0, len(t))
		for i, e := range t {
			d, ok := e.(bson.D)
			if !ok {
				return nil, &SyntaxError{Line: 1, Col: 1, Msg: fmt.Sprintf("element %d is %s, expected a document", i, TypeName(e))}
			}
			out = append(out, d)
		}
		return out, nil
	}
	return nil, &SyntaxError{Line: 1, Col: 1, Msg: fmt.Sprintf("expected a document or array of documents, got %s", TypeName(v))}
}

func stripComments(src string) string {
	p := NewParser(src)
	p.SkipSpace()
	return string(p.src[p.pos:])
}

// Pos returns the current offset in runes.
func (p *Parser) Pos() int { return p.pos }

// Seek moves to a rune offset previously returned by Pos.
func (p *Parser) Seek(pos int) {
	if pos >= 0 && pos <= len(p.src) {
		p.pos = pos
	}
}

// Slice returns the source text between two rune offsets.
func (p *Parser) Slice(from, to int) string {
	if from < 0 {
		from = 0
	}
	if to > len(p.src) {
		to = len(p.src)
	}
	return strings.TrimSpace(string(p.src[from:to]))
}

// Errorf builds a SyntaxError at the current position.
func (p *Parser) Errorf(format string, args ...any) error {
	line, col := 1, 1
	for i := 0; i < p.pos && i < len(p.src); i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &SyntaxError{Line: line, Col: col, Msg: fmt.Sprintf(format, args...)}
}

// SkipSpace skips whitespace and // or /* */ comments.
func (p *Parser) SkipSpace() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case unicode.IsSpace(c) || c == 0xFEFF:
			p.pos++
		case c == '/' && p.at(1) == '/':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == '/' && p.at(1) == '*':
			p.pos += 2
			for p.pos < len(p.src) && !(p.src[p.pos] == '*' && p.at(1) == '/') {
				p.pos++
			}
			p.pos += 2
			if p.pos > len(p.src) {
				p.pos = len(p.src)
			}
		default:
			return
		}
	}
}

// SkipInlineSpace skips spaces and tabs only (not newlines or comments).
func (p *Parser) SkipInlineSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
}

// EOF reports whether only whitespace/comments remain.
func (p *Parser) EOF() bool {
	p.SkipSpace()
	return p.pos >= len(p.src)
}

// Peek returns the next significant rune, or 0 at end of input.
func (p *Parser) Peek() rune {
	p.SkipSpace()
	return p.at(0)
}

func (p *Parser) at(off int) rune {
	if p.pos+off < len(p.src) {
		return p.src[p.pos+off]
	}
	return 0
}

// Consume skips space and consumes r if it is next.
func (p *Parser) Consume(r rune) bool {
	if p.Peek() == r {
		p.pos++
		return true
	}
	return false
}

// Expect consumes r or returns a syntax error.
func (p *Parser) Expect(r rune) error {
	if !p.Consume(r) {
		if p.pos >= len(p.src) {
			return p.Errorf("expected %q but reached end of input", r)
		}
		return p.Errorf("expected %q but found %q", r, p.at(0))
	}
	return nil
}

func isIdentStart(c rune) bool { return c == '_' || c == '$' || unicode.IsLetter(c) }
func isIdentPart(c rune) bool  { return isIdentStart(c) || unicode.IsDigit(c) }

// Ident consumes an identifier if one is next.
func (p *Parser) Ident() (string, bool) {
	p.SkipSpace()
	if !isIdentStart(p.at(0)) {
		return "", false
	}
	start := p.pos
	for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
		p.pos++
	}
	return string(p.src[start:p.pos]), true
}

// Word consumes a bare word on the current line (up to whitespace or ';'),
// used for shell commands like "use my-db".
func (p *Parser) Word() string {
	p.SkipInlineSpace()
	start := p.pos
	for p.pos < len(p.src) && !unicode.IsSpace(p.src[p.pos]) && p.src[p.pos] != ';' {
		p.pos++
	}
	return string(p.src[start:p.pos])
}

// String consumes a quoted string literal if one is next.
func (p *Parser) String() (string, bool, error) {
	q := p.Peek()
	if q != '"' && q != '\'' && q != '`' {
		return "", false, nil
	}
	s, err := p.quoted()
	return s, true, err
}

// Value parses any value.
func (p *Parser) Value() (any, error) {
	c := p.Peek()
	switch {
	case c == 0:
		return nil, p.Errorf("unexpected end of input, expected a value")
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"' || c == '\'' || c == '`':
		return p.quoted()
	case c == '/':
		return p.regex()
	case c == '-' || c == '+' || c == '.' || unicode.IsDigit(c):
		return p.number()
	case isIdentStart(c):
		return p.identValue()
	}
	return nil, p.Errorf("unexpected character %q", c)
}

func (p *Parser) object() (any, error) {
	if err := p.Expect('{'); err != nil {
		return nil, err
	}
	d := bson.D{}
	for {
		if p.Consume('}') {
			break
		}
		key, err := p.key()
		if err != nil {
			return nil, err
		}
		if err := p.Expect(':'); err != nil {
			return nil, err
		}
		val, err := p.Value()
		if err != nil {
			return nil, err
		}
		d = append(d, bson.E{Key: key, Value: val})
		if p.Consume(',') {
			continue
		}
		if err := p.Expect('}'); err != nil {
			return nil, err
		}
		break
	}
	if v, ok := fromExtJSONWrapper(d); ok {
		return v, nil
	}
	return d, nil
}

func (p *Parser) key() (string, error) {
	c := p.Peek()
	if c == '"' || c == '\'' || c == '`' {
		return p.quoted()
	}
	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if isIdentPart(r) || r == '.' {
			p.pos++
			continue
		}
		break
	}
	if start == p.pos {
		if c == 0 {
			return "", p.Errorf("unexpected end of input, expected a field name or '}'")
		}
		return "", p.Errorf("expected a field name but found %q", c)
	}
	return string(p.src[start:p.pos]), nil
}

func (p *Parser) array() (any, error) {
	if err := p.Expect('['); err != nil {
		return nil, err
	}
	a := bson.A{}
	for {
		if p.Consume(']') {
			return a, nil
		}
		v, err := p.Value()
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		if p.Consume(',') {
			continue
		}
		if err := p.Expect(']'); err != nil {
			return nil, err
		}
		return a, nil
	}
}

func (p *Parser) quoted() (string, error) {
	q := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			return "", p.Errorf("unterminated string")
		}
		c := p.src[p.pos]
		p.pos++
		switch {
		case c == q:
			return b.String(), nil
		case c == '\n' && q != '`':
			return "", p.Errorf("unterminated string (newline in string literal)")
		case c != '\\':
			b.WriteRune(c)
			continue
		}
		if p.pos >= len(p.src) {
			return "", p.Errorf("unterminated escape sequence")
		}
		e := p.src[p.pos]
		p.pos++
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case '\n':
			// line continuation
		case 'x':
			r, err := p.hexRune(2)
			if err != nil {
				return "", err
			}
			b.WriteRune(r)
		case 'u':
			r, err := p.hexRune(4)
			if err != nil {
				return "", err
			}
			if r >= 0xD800 && r < 0xDC00 && p.at(0) == '\\' && p.at(1) == 'u' {
				p.pos += 2
				lo, err := p.hexRune(4)
				if err != nil {
					return "", err
				}
				r = (r-0xD800)<<10 + (lo - 0xDC00) + 0x10000
			}
			b.WriteRune(r)
		default:
			b.WriteRune(e)
		}
	}
}

func (p *Parser) hexRune(n int) (rune, error) {
	if p.pos+n > len(p.src) {
		return 0, p.Errorf("invalid escape sequence")
	}
	v, err := strconv.ParseUint(string(p.src[p.pos:p.pos+n]), 16, 32)
	if err != nil {
		return 0, p.Errorf("invalid escape sequence")
	}
	p.pos += n
	return rune(v), nil
}

func (p *Parser) regex() (any, error) {
	p.pos++ // opening '/'
	var b strings.Builder
	inClass := false
	for {
		if p.pos >= len(p.src) || p.src[p.pos] == '\n' {
			return nil, p.Errorf("unterminated regular expression")
		}
		c := p.src[p.pos]
		p.pos++
		if c == '\\' && p.pos < len(p.src) {
			n := p.src[p.pos]
			p.pos++
			if n == '/' {
				b.WriteRune('/')
			} else {
				b.WriteRune('\\')
				b.WriteRune(n)
			}
			continue
		}
		if c == '[' {
			inClass = true
		} else if c == ']' {
			inClass = false
		} else if c == '/' && !inClass {
			break
		}
		b.WriteRune(c)
	}
	start := p.pos
	for p.pos < len(p.src) && unicode.IsLetter(p.src[p.pos]) {
		p.pos++
	}
	opts, err := regexOptions(string(p.src[start:p.pos]))
	if err != nil {
		return nil, p.Errorf("%s", err)
	}
	return bson.Regex{Pattern: b.String(), Options: opts}, nil
}

func regexOptions(flags string) (string, error) {
	r := []rune(flags)
	for _, f := range r {
		if !strings.ContainsRune("imsxlu", f) {
			if f == 'g' {
				return "", fmt.Errorf("regex flag 'g' is not supported by MongoDB")
			}
			return "", fmt.Errorf("invalid regex flag %q", f)
		}
	}
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	return string(r), nil
}

func (p *Parser) number() (any, error) {
	start := p.pos
	if c := p.at(0); c == '-' || c == '+' {
		p.pos++
	}
	if id, ok := p.peekWord(); ok && (id == "Infinity" || id == "NaN") {
		p.pos += len(id)
		if id == "NaN" {
			return math.NaN(), nil
		}
		if p.src[start] == '-' {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil
	}
	if p.at(0) == '0' && (p.at(1) == 'x' || p.at(1) == 'X') {
		p.pos += 2
		hs := p.pos
		for p.pos < len(p.src) && strings.ContainsRune("0123456789abcdefABCDEF", p.src[p.pos]) {
			p.pos++
		}
		v, err := strconv.ParseInt(string(p.src[hs:p.pos]), 16, 64)
		if err != nil {
			return nil, p.Errorf("invalid hex number")
		}
		if p.src[start] == '-' {
			v = -v
		}
		return narrowInt(v), nil
	}
	isFloat := false
	digits := 0
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case unicode.IsDigit(c):
			digits++
		case c == '.' || c == 'e' || c == 'E':
			isFloat = true
		case (c == '-' || c == '+') && (p.src[p.pos-1] == 'e' || p.src[p.pos-1] == 'E'):
		case c == '_':
		default:
			goto done
		}
		p.pos++
	}
done:
	text := strings.ReplaceAll(string(p.src[start:p.pos]), "_", "")
	if digits == 0 {
		p.pos = start
		return nil, p.Errorf("invalid number")
	}
	if !isFloat {
		if v, err := strconv.ParseInt(text, 10, 64); err == nil {
			return narrowInt(v), nil
		}
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, p.Errorf("invalid number %q", text)
	}
	return f, nil
}

func narrowInt(v int64) any {
	if v >= math.MinInt32 && v <= math.MaxInt32 {
		return int32(v)
	}
	return v
}

func (p *Parser) peekWord() (string, bool) {
	i := p.pos
	for i < len(p.src) && isIdentPart(p.src[i]) {
		i++
	}
	if i == p.pos {
		return "", false
	}
	return string(p.src[p.pos:i]), true
}

func (p *Parser) identValue() (any, error) {
	start := p.pos
	id, _ := p.Ident()
	switch id {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	case "undefined":
		return bson.Undefined{}, nil
	case "NaN":
		return math.NaN(), nil
	case "Infinity":
		return math.Inf(1), nil
	case "new":
		next, ok := p.Ident()
		if !ok {
			return nil, p.Errorf("expected a constructor after 'new'")
		}
		id = next
	}
	if id == "MinKey" || id == "MaxKey" {
		if p.Consume('(') {
			if err := p.Expect(')'); err != nil {
				return nil, err
			}
		}
		if id == "MinKey" {
			return bson.MinKey{}, nil
		}
		return bson.MaxKey{}, nil
	}
	if p.Peek() != '(' {
		p.pos = start
		return nil, p.Errorf("unknown identifier %q (strings must be quoted)", id)
	}
	p.pos++
	var args []any
	for !p.Consume(')') {
		v, err := p.Value()
		if err != nil {
			return nil, err
		}
		args = append(args, v)
		if !p.Consume(',') {
			if err := p.Expect(')'); err != nil {
				return nil, err
			}
			break
		}
	}
	v, err := construct(id, args)
	if err != nil {
		p.pos = start
		return nil, p.Errorf("%s", err)
	}
	return v, nil
}

func construct(name string, args []any) (any, error) {
	argStr := func(i int) (string, bool) {
		if i < len(args) {
			s, ok := args[i].(string)
			return s, ok
		}
		return "", false
	}
	switch name {
	case "ObjectId", "ObjectID":
		if len(args) == 0 {
			return bson.NewObjectID(), nil
		}
		s, ok := argStr(0)
		if !ok {
			return nil, fmt.Errorf("ObjectId expects a 24-character hex string")
		}
		oid, err := bson.ObjectIDFromHex(s)
		if err != nil {
			return nil, fmt.Errorf("invalid ObjectId %q: must be 24 hex characters", s)
		}
		return oid, nil
	case "ISODate", "Date":
		if len(args) == 0 {
			return bson.NewDateTimeFromTime(time.Now()), nil
		}
		if s, ok := argStr(0); ok {
			t, err := ParseDate(s)
			if err != nil {
				return nil, err
			}
			return bson.NewDateTimeFromTime(t), nil
		}
		if n, ok := toInt64(args[0]); ok {
			return bson.DateTime(n), nil
		}
		return nil, fmt.Errorf("%s expects an ISO-8601 string or milliseconds", name)
	case "NumberLong", "Long":
		if len(args) != 1 {
			return nil, fmt.Errorf("NumberLong expects one argument")
		}
		if s, ok := argStr(0); ok {
			v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid NumberLong %q", s)
			}
			return v, nil
		}
		if n, ok := toInt64(args[0]); ok {
			return n, nil
		}
		return nil, fmt.Errorf("NumberLong expects an integer")
	case "NumberInt", "Int32":
		if len(args) != 1 {
			return nil, fmt.Errorf("NumberInt expects one argument")
		}
		var n int64
		if s, ok := argStr(0); ok {
			v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid NumberInt %q", s)
			}
			n = v
		} else if v, ok := toInt64(args[0]); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
			n = v
		} else {
			return nil, fmt.Errorf("NumberInt expects a 32-bit integer")
		}
		return int32(n), nil
	case "Double":
		if len(args) == 1 {
			switch v := args[0].(type) {
			case float64:
				return v, nil
			case int32:
				return float64(v), nil
			case int64:
				return float64(v), nil
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f, nil
				}
			}
		}
		return nil, fmt.Errorf("Double expects a number")
	case "NumberDecimal", "Decimal128":
		if len(args) != 1 {
			return nil, fmt.Errorf("NumberDecimal expects one argument")
		}
		s, ok := argStr(0)
		if !ok {
			s = fmt.Sprint(args[0])
		}
		d, err := bson.ParseDecimal128(s)
		if err != nil {
			return nil, fmt.Errorf("invalid NumberDecimal %q", s)
		}
		return d, nil
	case "UUID":
		if len(args) == 0 {
			return nil, fmt.Errorf("UUID expects a string")
		}
		s, _ := argStr(0)
		b, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
		if err != nil || len(b) != 16 {
			return nil, fmt.Errorf("invalid UUID %q", s)
		}
		return bson.Binary{Subtype: 4, Data: b}, nil
	case "BinData":
		if len(args) != 2 {
			return nil, fmt.Errorf("BinData expects (subtype, base64)")
		}
		sub, ok := toInt64(args[0])
		s, ok2 := argStr(1)
		if !ok || !ok2 || sub < 0 || sub > 255 {
			return nil, fmt.Errorf("BinData expects (subtype, base64)")
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 in BinData")
		}
		return bson.Binary{Subtype: byte(sub), Data: b}, nil
	case "Timestamp":
		if len(args) == 1 {
			if d, ok := args[0].(bson.D); ok {
				var t, i int64
				for _, e := range d {
					switch e.Key {
					case "t":
						t, _ = toInt64(e.Value)
					case "i":
						i, _ = toInt64(e.Value)
					}
				}
				return bson.Timestamp{T: uint32(t), I: uint32(i)}, nil
			}
		}
		if len(args) == 2 {
			t, ok1 := toInt64(args[0])
			i, ok2 := toInt64(args[1])
			if ok1 && ok2 {
				return bson.Timestamp{T: uint32(t), I: uint32(i)}, nil
			}
		}
		return nil, fmt.Errorf("Timestamp expects (seconds, increment)")
	case "RegExp":
		s, ok := argStr(0)
		if !ok {
			return nil, fmt.Errorf("RegExp expects a pattern string")
		}
		flags, _ := argStr(1)
		opts, err := regexOptions(flags)
		if err != nil {
			return nil, err
		}
		return bson.Regex{Pattern: s, Options: opts}, nil
	case "Code":
		s, ok := argStr(0)
		if !ok {
			return nil, fmt.Errorf("Code expects a string")
		}
		return bson.JavaScript(s), nil
	}
	return nil, fmt.Errorf("unknown function %s()", name)
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		if n == math.Trunc(n) && !math.IsInf(n, 0) {
			return int64(n), true
		}
	}
	return 0, false
}

var dateLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999Z0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
}

// ParseDate parses the date formats accepted by ISODate(). Inputs without a
// zone are interpreted as UTC, matching mongosh.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use ISO-8601, e.g. 2024-01-31T12:00:00Z)", s)
}

var extJSONKeys = map[string]bool{
	"$oid": true, "$date": true, "$numberLong": true, "$numberInt": true,
	"$numberDouble": true, "$numberDecimal": true, "$binary": true,
	"$uuid": true, "$timestamp": true, "$regularExpression": true,
	"$minKey": true, "$maxKey": true, "$symbol": true, "$code": true,
	"$undefined": true,
}

// fromExtJSONWrapper converts Extended JSON type wrappers such as
// {"$oid": "..."} into native BSON values by delegating to the driver.
func fromExtJSONWrapper(d bson.D) (any, bool) {
	if len(d) == 0 || len(d) > 2 || !extJSONKeys[d[0].Key] {
		return nil, false
	}
	data, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: d}}, true, false)
	if err != nil {
		return nil, false
	}
	var out bson.D
	if err := bson.UnmarshalExtJSON(data, false, &out); err != nil || len(out) != 1 {
		return nil, false
	}
	if _, still := out[0].Value.(bson.D); still {
		return nil, false
	}
	return out[0].Value, true
}
