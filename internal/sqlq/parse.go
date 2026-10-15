package sqlq

import (
	"math"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

type aggCall struct {
	fn    string // count | sum | avg | min | max
	field string // "" for COUNT(*)
	pos   int
}

func (a aggCall) key() string { return a.fn + "(" + a.field + ")" }

type selItem struct {
	star  bool
	field string
	agg   *aggCall
	alias string
	pos   int
}

type orderItem struct {
	field string
	agg   *aggCall
	desc  bool
	pos   int
}

// operand is a field, an aggregate (HAVING) or a literal value.
type operand struct {
	field   string
	agg     *aggCall
	value   any
	isValue bool
	pos     int
}

type expr interface{}

type andExpr struct{ items []expr }
type orExpr struct{ items []expr }
type notExpr struct{ e expr }
type cmpExpr struct {
	left, right operand
	op          string
	pos         int
}
type inExpr struct {
	left   operand
	values []any
	not    bool
}
type likeExpr struct {
	left    operand
	pattern string
	not     bool
	ci      bool
}
type nullExpr struct {
	left operand
	not  bool
}
type betweenExpr struct {
	left   operand
	lo, hi any
	not    bool
}

type selectStmt struct {
	distinct bool
	items    []selItem
	from     string
	where    expr
	groupBy  []string
	having   expr
	orderBy  []orderItem
	limit    int64
	offset   int64
	hasLimit bool
}

var unsupported = map[string]string{
	"JOIN": "JOIN", "INNER": "JOIN", "LEFT": "JOIN", "RIGHT": "JOIN", "FULL": "JOIN", "CROSS": "JOIN",
	"UNION": "UNION", "INTERSECT": "INTERSECT", "EXCEPT": "EXCEPT", "WITH": "WITH (CTEs)",
	"INSERT": "INSERT", "UPDATE": "UPDATE", "DELETE": "DELETE", "CREATE": "CREATE", "DROP": "DROP",
	"ALTER": "ALTER", "TRUNCATE": "TRUNCATE", "MERGE": "MERGE", "CASE": "CASE expressions",
	"EXISTS": "EXISTS", "OVER": "window functions", "INTO": "SELECT INTO", "WINDOW": "window functions",
}

var reserved = map[string]bool{
	"SELECT": true, "DISTINCT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true, "NOT": true,
	"IN": true, "LIKE": true, "ILIKE": true, "IS": true, "NULL": true, "BETWEEN": true, "GROUP": true,
	"BY": true, "HAVING": true, "ORDER": true, "ASC": true, "DESC": true, "LIMIT": true, "OFFSET": true,
	"AS": true, "TRUE": true, "FALSE": true,
}

var aggFuncs = map[string]bool{"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true}

type parser struct {
	src  string
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) isKw(t token, kw string) bool {
	return t.kind == tIdent && strings.EqualFold(t.text, kw)
}

func (p *parser) acceptKw(kw string) bool {
	if p.isKw(p.peek(), kw) {
		p.i++
		return true
	}
	return false
}

func (p *parser) expectKw(kw string) error {
	if !p.acceptKw(kw) {
		return p.errExpected(kw)
	}
	return nil
}

func (p *parser) acceptPunct(s string) bool {
	if t := p.peek(); t.kind == tPunct && t.text == s {
		p.i++
		return true
	}
	return false
}

func (p *parser) errExpected(what string) error {
	t := p.peek()
	if t.kind == tEOF {
		return errAt(p.src, t.pos, "expected %s but reached the end of the query", what)
	}
	if t.kind == tIdent {
		if feature, ok := unsupported[strings.ToUpper(t.text)]; ok {
			return errAt(p.src, t.pos, "%s is not supported", feature)
		}
	}
	return errAt(p.src, t.pos, "expected %s but found %q", what, t.text)
}

func parse(src string) (*selectStmt, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, toks: toks}
	for _, t := range toks {
		if t.kind == tIdent {
			if feature, ok := unsupported[strings.ToUpper(t.text)]; ok {
				return nil, errAt(src, t.pos, "%s is not supported", feature)
			}
		}
	}
	if p.peek().kind == tEOF {
		return nil, errAt(src, 0, "the query is empty")
	}
	if !p.isKw(p.peek(), "SELECT") {
		return nil, errAt(src, p.peek().pos, "only SELECT statements are supported")
	}
	p.next()
	s := &selectStmt{}
	s.distinct = p.acceptKw("DISTINCT")
	if err := p.selectList(s); err != nil {
		return nil, err
	}
	if err := p.expectKw("FROM"); err != nil {
		return nil, err
	}
	name, pos, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if strings.Contains(name, ".") {
		return nil, errAt(src, pos, "use the collection name only (switch databases in the toolbar)")
	}
	s.from = name
	if t := p.peek(); t.kind == tPunct && t.text == "," {
		return nil, errAt(src, t.pos, "multiple tables (implicit joins) are not supported")
	}
	if t := p.peek(); t.kind == tIdent && !reserved[strings.ToUpper(t.text)] {
		return nil, errAt(src, t.pos, "table aliases are not supported")
	}
	if p.acceptKw("WHERE") {
		if s.where, err = p.orExpr(false); err != nil {
			return nil, err
		}
	}
	if p.acceptKw("GROUP") {
		if err := p.expectKw("BY"); err != nil {
			return nil, err
		}
		for {
			f, _, err := p.identifier()
			if err != nil {
				return nil, err
			}
			s.groupBy = append(s.groupBy, f)
			if !p.acceptPunct(",") {
				break
			}
		}
	}
	if p.acceptKw("HAVING") {
		if s.having, err = p.orExpr(true); err != nil {
			return nil, err
		}
	}
	if p.acceptKw("ORDER") {
		if err := p.expectKw("BY"); err != nil {
			return nil, err
		}
		for {
			it := orderItem{pos: p.peek().pos}
			if agg, ok, err := p.tryAggregate(); err != nil {
				return nil, err
			} else if ok {
				it.agg = agg
			} else if it.field, _, err = p.identifier(); err != nil {
				return nil, err
			}
			if p.acceptKw("DESC") {
				it.desc = true
			} else {
				p.acceptKw("ASC")
			}
			s.orderBy = append(s.orderBy, it)
			if !p.acceptPunct(",") {
				break
			}
		}
	}
	if p.acceptKw("LIMIT") {
		n, err := p.integer()
		if err != nil {
			return nil, err
		}
		s.limit, s.hasLimit = n, true
		if p.acceptPunct(",") { // MySQL: LIMIT offset, count
			m, err := p.integer()
			if err != nil {
				return nil, err
			}
			s.offset, s.limit = n, m
		}
	}
	if p.acceptKw("OFFSET") {
		n, err := p.integer()
		if err != nil {
			return nil, err
		}
		s.offset = n
	}
	p.acceptPunct(";")
	if t := p.peek(); t.kind != tEOF {
		return nil, p.errExpected("end of query")
	}
	return s, nil
}

func (p *parser) selectList(s *selectStmt) error {
	for {
		it := selItem{pos: p.peek().pos}
		switch {
		case p.acceptPunct("*"):
			it.star = true
		default:
			agg, ok, err := p.tryAggregate()
			if err != nil {
				return err
			}
			if ok {
				it.agg = agg
			} else {
				if t := p.peek(); t.kind == tString || t.kind == tNumber {
					return errAt(p.src, t.pos, "constant expressions in SELECT are not supported")
				}
				if it.field, _, err = p.identifier(); err != nil {
					return err
				}
				if t := p.peek(); t.kind == tPunct && t.text == "(" {
					return errAt(p.src, t.pos, "function %s() is not supported (supported: COUNT, SUM, AVG, MIN, MAX)", strings.ToUpper(it.field))
				}
			}
			if p.acceptKw("AS") {
				if it.alias, _, err = p.identifier(); err != nil {
					return err
				}
			} else if t := p.peek(); (t.kind == tIdent && !reserved[strings.ToUpper(t.text)]) || t.kind == tQuotedIdent {
				it.alias, _, _ = p.identifier()
			}
		}
		s.items = append(s.items, it)
		if !p.acceptPunct(",") {
			return nil
		}
	}
}

func (p *parser) tryAggregate() (*aggCall, bool, error) {
	t := p.peek()
	if t.kind != tIdent || !aggFuncs[strings.ToUpper(t.text)] {
		return nil, false, nil
	}
	if n := p.toks[p.i+1]; n.kind != tPunct || n.text != "(" {
		return nil, false, nil
	}
	p.i += 2
	a := &aggCall{fn: strings.ToLower(t.text), pos: t.pos}
	if p.acceptKw("DISTINCT") {
		return nil, false, errAt(p.src, t.pos, "%s(DISTINCT ...) is not supported", strings.ToUpper(t.text))
	}
	if p.acceptPunct("*") {
		if a.fn != "count" {
			return nil, false, errAt(p.src, t.pos, "%s(*) is not valid", strings.ToUpper(t.text))
		}
	} else {
		f, _, err := p.identifier()
		if err != nil {
			return nil, false, err
		}
		a.field = f
	}
	if !p.acceptPunct(")") {
		return nil, false, p.errExpected("')'")
	}
	return a, true, nil
}

// identifier reads a possibly dotted, possibly quoted field path.
func (p *parser) identifier() (string, int, error) {
	t := p.peek()
	var parts []string
	for {
		t2 := p.peek()
		switch {
		case t2.kind == tQuotedIdent:
			parts = append(parts, t2.text)
		case t2.kind == tIdent && !reserved[strings.ToUpper(t2.text)]:
			parts = append(parts, t2.text)
		default:
			return "", t.pos, p.errExpected("a field name")
		}
		p.next()
		if !p.acceptPunct(".") {
			return strings.Join(parts, "."), t.pos, nil
		}
	}
}

func (p *parser) integer() (int64, error) {
	t := p.peek()
	if t.kind != tNumber {
		return 0, p.errExpected("a number")
	}
	n, err := strconv.ParseInt(t.text, 10, 64)
	if err != nil || n < 0 {
		return 0, errAt(p.src, t.pos, "expected a non-negative integer")
	}
	p.next()
	return n, nil
}

func (p *parser) orExpr(having bool) (expr, error) {
	left, err := p.andExpr(having)
	if err != nil {
		return nil, err
	}
	items := []expr{left}
	for p.acceptKw("OR") {
		r, err := p.andExpr(having)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if len(items) == 1 {
		return left, nil
	}
	return orExpr{items}, nil
}

func (p *parser) andExpr(having bool) (expr, error) {
	left, err := p.notExpr(having)
	if err != nil {
		return nil, err
	}
	items := []expr{left}
	for p.acceptKw("AND") {
		r, err := p.notExpr(having)
		if err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	if len(items) == 1 {
		return left, nil
	}
	return andExpr{items}, nil
}

func (p *parser) notExpr(having bool) (expr, error) {
	if p.acceptKw("NOT") {
		e, err := p.notExpr(having)
		if err != nil {
			return nil, err
		}
		return notExpr{e}, nil
	}
	return p.predicate(having)
}

func (p *parser) predicate(having bool) (expr, error) {
	if p.acceptPunct("(") {
		if p.isKw(p.peek(), "SELECT") {
			return nil, errAt(p.src, p.peek().pos, "subqueries are not supported")
		}
		e, err := p.orExpr(having)
		if err != nil {
			return nil, err
		}
		if !p.acceptPunct(")") {
			return nil, p.errExpected("')'")
		}
		return e, nil
	}
	left, err := p.operand(having)
	if err != nil {
		return nil, err
	}
	t := p.peek()
	switch {
	case t.kind == tOp:
		p.next()
		right, err := p.operand(having)
		if err != nil {
			return nil, err
		}
		if left.isValue && right.isValue {
			return nil, errAt(p.src, t.pos, "comparisons need at least one field")
		}
		if (left.isValue && left.value == nil) || (right.isValue && right.value == nil) {
			return nil, errAt(p.src, t.pos, "use IS NULL / IS NOT NULL to compare with NULL")
		}
		return cmpExpr{left: left, right: right, op: t.text, pos: t.pos}, nil
	case p.isKw(t, "IS"):
		p.next()
		not := p.acceptKw("NOT")
		if err := p.expectKw("NULL"); err != nil {
			return nil, err
		}
		return nullExpr{left: left, not: not}, p.requireField(left)
	}
	not := p.acceptKw("NOT")
	t = p.peek()
	switch {
	case p.isKw(t, "IN"):
		p.next()
		if !p.acceptPunct("(") {
			return nil, p.errExpected("'('")
		}
		if p.isKw(p.peek(), "SELECT") {
			return nil, errAt(p.src, p.peek().pos, "subqueries are not supported")
		}
		var vals []any
		for {
			v, err := p.literal()
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
			if !p.acceptPunct(",") {
				break
			}
		}
		if !p.acceptPunct(")") {
			return nil, p.errExpected("')'")
		}
		return inExpr{left: left, values: vals, not: not}, p.requireField(left)
	case p.isKw(t, "LIKE") || p.isKw(t, "ILIKE"):
		p.next()
		pt := p.peek()
		if pt.kind != tString {
			return nil, p.errExpected("a quoted LIKE pattern")
		}
		p.next()
		return likeExpr{left: left, pattern: pt.text, not: not, ci: p.isKw(t, "ILIKE")}, p.requireField(left)
	case p.isKw(t, "BETWEEN"):
		p.next()
		lo, err := p.literal()
		if err != nil {
			return nil, err
		}
		if err := p.expectKw("AND"); err != nil {
			return nil, err
		}
		hi, err := p.literal()
		if err != nil {
			return nil, err
		}
		return betweenExpr{left: left, lo: lo, hi: hi, not: not}, p.requireField(left)
	}
	if not {
		return nil, p.errExpected("IN, LIKE or BETWEEN after NOT")
	}
	return nil, p.errExpected("a comparison operator")
}

func (p *parser) requireField(o operand) error {
	if o.isValue {
		return errAt(p.src, o.pos, "the left side must be a field")
	}
	return nil
}

func (p *parser) operand(having bool) (operand, error) {
	t := p.peek()
	agg, ok, err := p.tryAggregate()
	if err != nil {
		return operand{}, err
	}
	if ok {
		if !having {
			return operand{}, errAt(p.src, t.pos, "aggregate functions are only allowed in SELECT, HAVING and ORDER BY")
		}
		return operand{agg: agg, pos: t.pos}, nil
	}
	if t.kind == tIdent && !reserved[strings.ToUpper(t.text)] && !isLiteralFunc(t.text) {
		if n := p.toks[p.i+1]; n.kind == tPunct && n.text == "(" {
			return operand{}, errAt(p.src, t.pos, "function %s() is not supported", strings.ToUpper(t.text))
		}
	}
	if t.kind == tQuotedIdent || (t.kind == tIdent && !reserved[strings.ToUpper(t.text)] && !isLiteralFunc(t.text)) {
		f, pos, err := p.identifier()
		return operand{field: f, pos: pos}, err
	}
	v, err := p.literal()
	return operand{value: v, isValue: true, pos: t.pos}, err
}

func isLiteralFunc(s string) bool {
	switch strings.ToUpper(s) {
	case "OBJECTID", "DATE", "TIMESTAMP", "ISODATE":
		return true
	}
	return false
}

func (p *parser) literal() (any, error) {
	t := p.peek()
	switch {
	case t.kind == tString:
		p.next()
		return t.text, nil
	case t.kind == tNumber:
		p.next()
		if !strings.ContainsAny(t.text, ".eE") {
			if n, err := strconv.ParseInt(t.text, 10, 64); err == nil {
				if n >= math.MinInt32 && n <= math.MaxInt32 {
					return int32(n), nil
				}
				return n, nil
			}
		}
		f, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, errAt(p.src, t.pos, "invalid number %q", t.text)
		}
		return f, nil
	case p.isKw(t, "TRUE"):
		p.next()
		return true, nil
	case p.isKw(t, "FALSE"):
		p.next()
		return false, nil
	case p.isKw(t, "NULL"):
		p.next()
		return nil, nil
	case t.kind == tIdent && isLiteralFunc(t.text):
		p.next()
		fn := strings.ToUpper(t.text)
		var arg token
		if fn != "OBJECTID" && p.peek().kind == tString { // DATE '2024-01-01'
			arg = p.next()
		} else {
			if !p.acceptPunct("(") {
				return nil, p.errExpected("'('")
			}
			arg = p.peek()
			if arg.kind != tString {
				return nil, p.errExpected("a quoted string")
			}
			p.next()
			if !p.acceptPunct(")") {
				return nil, p.errExpected("')'")
			}
		}
		if fn == "OBJECTID" {
			oid, err := bson.ObjectIDFromHex(arg.text)
			if err != nil {
				return nil, errAt(p.src, arg.pos, "invalid ObjectId %q", arg.text)
			}
			return oid, nil
		}
		tm, err := mql.ParseDate(arg.text)
		if err != nil {
			return nil, errAt(p.src, arg.pos, "invalid date %q", arg.text)
		}
		return bson.NewDateTimeFromTime(tm), nil
	}
	if t.kind == tIdent && !reserved[strings.ToUpper(t.text)] {
		return nil, errAt(p.src, t.pos, "expected a value but found the name %q (quote strings with single quotes)", t.text)
	}
	return nil, p.errExpected("a value")
}
