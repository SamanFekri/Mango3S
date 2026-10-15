package sqlq

import (
	"fmt"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// Translation is the MongoDB equivalent of a SQL query.
type Translation struct {
	Kind       string // "find" | "aggregate"
	Collection string
	Filter     bson.D
	Projection bson.D
	Sort       bson.D
	Skip       int64
	Limit      int64 // 0 = no limit
	Pipeline   []bson.D
}

// Shell renders the translation as mongosh code.
func (t Translation) Shell() string {
	coll := collRef(t.Collection)
	if t.Kind == "aggregate" {
		var b strings.Builder
		b.WriteString(coll + ".aggregate([\n")
		for i, st := range t.Pipeline {
			b.WriteString("  " + mql.Compact(st))
			if i < len(t.Pipeline)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("])")
		return b.String()
	}
	s := fmt.Sprintf("%s.find(%s", coll, mql.Compact(t.Filter))
	if len(t.Projection) > 0 {
		s += ", " + mql.Compact(t.Projection)
	}
	s += ")"
	if len(t.Sort) > 0 {
		s += ".sort(" + mql.Compact(t.Sort) + ")"
	}
	if t.Skip > 0 {
		s += fmt.Sprintf(".skip(%d)", t.Skip)
	}
	if t.Limit > 0 {
		s += fmt.Sprintf(".limit(%d)", t.Limit)
	}
	return s
}

func collRef(name string) string {
	if regexp.MustCompile(`^[A-Za-z_$][\w$]*$`).MatchString(name) {
		return "db." + name
	}
	return "db.getCollection(" + mql.Quote(name) + ")"
}

// Translate parses SQL and converts it. Errors are apperr INVALID_INPUT with
// the line and column of the problem.
func Translate(sql string) (Translation, error) {
	s, err := parse(sql)
	if err != nil {
		return Translation{}, apperr.Invalid("SQL %s", err.Error())
	}
	t, err := (&translator{src: sql, s: s}).run()
	if err != nil {
		if se, ok := err.(*Error); ok {
			return Translation{}, apperr.Invalid("SQL %s", se.Error())
		}
		return Translation{}, err
	}
	return t, nil
}

type translator struct {
	src string
	s   *selectStmt
}

func (tr *translator) run() (Translation, error) {
	s := tr.s
	t := Translation{Collection: s.from, Skip: s.offset}
	if s.hasLimit {
		t.Limit = s.limit
		if t.Limit == 0 {
			t.Limit = -1 // LIMIT 0 returns nothing; handled below
		}
	}
	filter := bson.D{}
	if s.where != nil {
		f, err := tr.filter(s.where, nil)
		if err != nil {
			return t, err
		}
		filter = f
	}
	grouped := len(s.groupBy) > 0
	for _, it := range s.items {
		if it.agg != nil {
			grouped = true
		}
	}
	if s.having != nil && !grouped {
		return t, errAt(tr.src, 0, "HAVING requires GROUP BY or an aggregate function")
	}
	if grouped || s.distinct {
		return tr.aggregate(t, filter)
	}

	// Plain find.
	t.Kind = "find"
	t.Filter = filter
	aliases := map[string]string{}
	star := false
	for _, it := range s.items {
		if it.star {
			star = true
		}
	}
	if !star {
		proj := bson.D{}
		hasID := false
		for _, it := range s.items {
			if it.field == "_id" && (it.alias == "" || it.alias == "_id") {
				hasID = true
			}
			if it.alias != "" && it.alias != it.field {
				proj = append(proj, bson.E{Key: it.alias, Value: "$" + it.field})
				aliases[it.alias] = it.field
			} else {
				proj = append(proj, bson.E{Key: it.field, Value: 1})
			}
		}
		if !hasID {
			proj = append(proj, bson.E{Key: "_id", Value: 0})
		}
		t.Projection = proj
	} else if len(s.items) > 1 {
		return t, errAt(tr.src, s.items[1].pos, "SELECT * cannot be combined with other columns")
	}
	for _, o := range s.orderBy {
		if o.agg != nil {
			return t, errAt(tr.src, o.pos, "ORDER BY an aggregate requires GROUP BY")
		}
		f := o.field
		if src, ok := aliases[f]; ok {
			f = src
		}
		t.Sort = append(t.Sort, bson.E{Key: f, Value: dir(o.desc)})
	}
	if t.Limit < 0 {
		t.Limit, t.Filter = 0, bson.D{{Key: "$expr", Value: false}}
	}
	return t, nil
}

func dir(desc bool) int32 {
	if desc {
		return -1
	}
	return 1
}

// groupKey is a safe $group key for a field path.
func groupKey(i int) string { return fmt.Sprintf("k%d", i) }

func defaultAlias(a *aggCall) string {
	if a.field == "" {
		return a.fn
	}
	return a.fn + "_" + strings.ReplaceAll(a.field, ".", "_")
}

func accumulator(a *aggCall) bson.D {
	switch a.fn {
	case "count":
		if a.field == "" {
			return bson.D{{Key: "$sum", Value: int32(1)}}
		}
		// COUNT(field) counts non-null values; in aggregation null/missing sort below everything.
		return bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
			bson.D{{Key: "$gt", Value: bson.A{"$" + a.field, nil}}}, int32(1), int32(0),
		}}}}}
	default:
		return bson.D{{Key: "$" + a.fn, Value: "$" + a.field}}
	}
}

func (tr *translator) aggregate(t Translation, filter bson.D) (Translation, error) {
	s := tr.s
	t.Kind = "aggregate"
	var pipe []bson.D
	if len(filter) > 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: filter}})
	}

	groupFields := s.groupBy
	if s.distinct && len(groupFields) == 0 {
		for _, it := range s.items {
			if it.star {
				return t, errAt(tr.src, it.pos, "SELECT DISTINCT * is not supported; list the fields")
			}
			if it.agg == nil {
				groupFields = append(groupFields, it.field)
			}
		}
	}
	keyOf := map[string]string{} // field path -> _id.kN
	idDoc := bson.D{}
	for i, f := range groupFields {
		k := groupKey(i)
		keyOf[f] = "_id." + k
		idDoc = append(idDoc, bson.E{Key: k, Value: "$" + f})
	}
	var groupID any = idDoc
	if len(idDoc) == 0 {
		groupID = nil
	}

	group := bson.D{{Key: "_id", Value: groupID}}
	aggName := map[string]string{} // aggCall.key() -> output field
	project := bson.D{{Key: "_id", Value: int32(0)}}
	outputs := map[string]bool{}
	hidden := 0

	for _, it := range s.items {
		switch {
		case it.star:
			return t, errAt(tr.src, it.pos, "SELECT * cannot be used with GROUP BY or aggregates")
		case it.agg != nil:
			name := it.alias
			if name == "" {
				name = defaultAlias(it.agg)
			}
			if strings.ContainsAny(name, ".$") {
				return t, errAt(tr.src, it.pos, "alias %q must not contain '.' or '$'", name)
			}
			if outputs[name] {
				return t, errAt(tr.src, it.pos, "duplicate output column %q", name)
			}
			outputs[name] = true
			if _, ok := aggName[it.agg.key()]; !ok {
				aggName[it.agg.key()] = name
			}
			group = append(group, bson.E{Key: name, Value: accumulator(it.agg)})
			project = append(project, bson.E{Key: name, Value: int32(1)})
		default:
			k, ok := keyOf[it.field]
			if !ok {
				return t, errAt(tr.src, it.pos, "%q must appear in GROUP BY or be used in an aggregate function", it.field)
			}
			name := it.field
			if it.alias != "" {
				name = it.alias
			}
			outputs[name] = true
			project = append(project, bson.E{Key: name, Value: "$" + k})
		}
	}

	// HAVING runs on the $group output, before the final projection.
	var having bson.D
	if s.having != nil {
		resolve := func(o operand) (string, error) {
			if o.agg != nil {
				if n, ok := aggName[o.agg.key()]; ok {
					return n, nil
				}
				n := fmt.Sprintf("__having%d", hidden)
				hidden++
				aggName[o.agg.key()] = n
				group = append(group, bson.E{Key: n, Value: accumulator(o.agg)})
				return n, nil
			}
			if k, ok := keyOf[o.field]; ok {
				return k, nil
			}
			for _, it := range s.items {
				if it.alias == o.field && it.agg != nil {
					return aggName[it.agg.key()], nil
				}
				if it.alias == o.field && it.agg == nil {
					return keyOf[it.field], nil
				}
			}
			return "", errAt(tr.src, o.pos, "HAVING can only use grouped fields, aggregates or their aliases (%q)", o.field)
		}
		h, err := tr.filter(s.having, resolve)
		if err != nil {
			return t, err
		}
		having = h
	}

	pipe = append(pipe, bson.D{{Key: "$group", Value: group}})
	if len(having) > 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: having}})
	}
	pipe = append(pipe, bson.D{{Key: "$project", Value: project}})

	if len(s.orderBy) > 0 {
		sort := bson.D{}
		for _, o := range s.orderBy {
			name := o.field
			if o.agg != nil {
				n, ok := aggName[o.agg.key()]
				if !ok || strings.HasPrefix(n, "__") {
					return t, errAt(tr.src, o.pos, "ORDER BY %s(): add it to the SELECT list", strings.ToUpper(o.agg.fn))
				}
				name = n
			} else if !outputs[name] {
				return t, errAt(tr.src, o.pos, "ORDER BY %q: the column is not in the SELECT list", name)
			}
			sort = append(sort, bson.E{Key: name, Value: dir(o.desc)})
		}
		pipe = append(pipe, bson.D{{Key: "$sort", Value: sort}})
	}
	if t.Skip > 0 {
		pipe = append(pipe, bson.D{{Key: "$skip", Value: t.Skip}})
	}
	if t.Limit > 0 {
		pipe = append(pipe, bson.D{{Key: "$limit", Value: t.Limit}})
	} else if t.Limit < 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: false}}}})
	}
	t.Skip, t.Limit = 0, 0
	t.Pipeline = pipe
	return t, nil
}

// filter converts a WHERE/HAVING expression. resolve maps operands to
// document fields (HAVING); nil means operands are plain field paths.
func (tr *translator) filter(e expr, resolve func(operand) (string, error)) (bson.D, error) {
	name := func(o operand) (string, error) {
		if resolve != nil {
			return resolve(o)
		}
		return o.field, nil
	}
	switch x := e.(type) {
	case andExpr:
		parts := make([]bson.D, 0, len(x.items))
		for _, it := range x.items {
			d, err := tr.filter(it, resolve)
			if err != nil {
				return nil, err
			}
			if inner, ok := mql.Lookup(d, "$and"); ok && len(d) == 1 {
				for _, sub := range inner.(bson.A) {
					parts = append(parts, sub.(bson.D))
				}
				continue
			}
			parts = append(parts, d)
		}
		return mergeAnd(parts), nil
	case orExpr:
		arr := bson.A{}
		for _, it := range x.items {
			d, err := tr.filter(it, resolve)
			if err != nil {
				return nil, err
			}
			if inner, ok := mql.Lookup(d, "$or"); ok && len(d) == 1 {
				arr = append(arr, inner.(bson.A)...)
				continue
			}
			arr = append(arr, d)
		}
		return bson.D{{Key: "$or", Value: arr}}, nil
	case notExpr:
		d, err := tr.filter(x.e, resolve)
		if err != nil {
			return nil, err
		}
		return bson.D{{Key: "$nor", Value: bson.A{d}}}, nil
	case cmpExpr:
		left, right, op := x.left, x.right, x.op
		if left.isValue { // 5 < age  ->  age > 5
			left, right, op = right, left, flip(op)
		}
		lf, err := name(left)
		if err != nil {
			return nil, err
		}
		if !right.isValue { // field vs field
			rf, err := name(right)
			if err != nil {
				return nil, err
			}
			return bson.D{{Key: "$expr", Value: bson.D{{Key: aggOp(op), Value: bson.A{"$" + lf, "$" + rf}}}}}, nil
		}
		if op == "=" {
			if _, isRegex := right.value.(bson.Regex); !isRegex {
				return bson.D{{Key: lf, Value: right.value}}, nil
			}
		}
		return bson.D{{Key: lf, Value: bson.D{{Key: aggOp(op), Value: right.value}}}}, nil
	case inExpr:
		f, err := name(x.left)
		if err != nil {
			return nil, err
		}
		op := "$in"
		if x.not {
			op = "$nin"
		}
		return bson.D{{Key: f, Value: bson.D{{Key: op, Value: bson.A(x.values)}}}}, nil
	case likeExpr:
		f, err := name(x.left)
		if err != nil {
			return nil, err
		}
		re := bson.Regex{Pattern: LikeToRegex(x.pattern)}
		if x.ci {
			re.Options = "i"
		}
		if x.not {
			return bson.D{{Key: f, Value: bson.D{{Key: "$not", Value: re}}}}, nil
		}
		return bson.D{{Key: f, Value: re}}, nil
	case nullExpr:
		f, err := name(x.left)
		if err != nil {
			return nil, err
		}
		if x.not {
			return bson.D{{Key: f, Value: bson.D{{Key: "$ne", Value: nil}}}}, nil
		}
		return bson.D{{Key: f, Value: nil}}, nil
	case betweenExpr:
		f, err := name(x.left)
		if err != nil {
			return nil, err
		}
		rng := bson.D{{Key: "$gte", Value: x.lo}, {Key: "$lte", Value: x.hi}}
		if x.not {
			return bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: f, Value: bson.D{{Key: "$lt", Value: x.lo}}}},
				bson.D{{Key: f, Value: bson.D{{Key: "$gt", Value: x.hi}}}},
			}}}, nil
		}
		return bson.D{{Key: f, Value: rng}}, nil
	}
	return nil, fmt.Errorf("internal: unknown expression %T", e)
}

// mergeAnd combines conjuncts into one document when their keys don't clash,
// otherwise it keeps an explicit $and.
func mergeAnd(parts []bson.D) bson.D {
	seen := map[string]bool{}
	merged := bson.D{}
	for _, p := range parts {
		for _, e := range p {
			if seen[e.Key] {
				arr := make(bson.A, len(parts))
				for i := range parts {
					arr[i] = parts[i]
				}
				return bson.D{{Key: "$and", Value: arr}}
			}
			seen[e.Key] = true
			merged = append(merged, e)
		}
	}
	return merged
}

func flip(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	}
	return op
}

func aggOp(op string) string {
	switch op {
	case "=":
		return "$eq"
	case "!=", "<>":
		return "$ne"
	case "<":
		return "$lt"
	case "<=":
		return "$lte"
	case ">":
		return "$gt"
	case ">=":
		return "$gte"
	}
	return "$eq"
}

// LikeToRegex converts a SQL LIKE pattern (% and _ wildcards, \ escape) to an
// anchored regular expression.
func LikeToRegex(p string) string {
	var b strings.Builder
	b.WriteByte('^')
	runes := []rune(p)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '\\' && i+1 < len(runes):
			i++
			b.WriteString(regexp.QuoteMeta(string(runes[i])))
		case c == '%':
			b.WriteString(".*")
		case c == '_':
			b.WriteByte('.')
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteByte('$')
	s := b.String()
	// Simplify the common prefix/suffix forms: '^abc.*$' -> '^abc'.
	s = strings.TrimSuffix(s, ".*$")
	if strings.HasPrefix(s, "^.*") {
		s = strings.TrimPrefix(s, "^.*")
	}
	return s
}
