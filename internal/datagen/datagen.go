// Package datagen generates test documents from a template with generator
// markers, e.g. { name: $name, email: $email, age: $int(18,90),
// created: $date("-1y","now") }, or from a schema report. A seed makes the
// output repeatable.
package datagen

import (
	"fmt"
	"math"
	"math/rand/v2"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
	"mango3s/internal/schema"
)

// node is one part of a compiled template.
type node struct {
	kind     string // literal | doc | array | gen
	value    any
	fields   []field
	elems    []*node
	gen      string
	args     []any
	presence float64 // 0..1; fields only appear this often (1 = always)
}

type field struct {
	name string
	node *node
}

// Template is a compiled template.
type Template struct{ root *node }

// Generators lists the markers a template can use, with an example, for the UI.
var Generators = []GeneratorInfo{
	{"$name", "a full name", "$name"},
	{"$firstName", "a first name", "$firstName"},
	{"$lastName", "a last name", "$lastName"},
	{"$email", "an email address", "$email"},
	{"$phone", "a phone number", "$phone"},
	{"$city", "a city", "$city"},
	{"$country", "a country", "$country"},
	{"$street", "a street address", "$street"},
	{"$company", "a company name", "$company"},
	{"$word", "a word", "$word"},
	{"$words", "n words", "$words(3)"},
	{"$sentence", "a sentence", "$sentence"},
	{"$int", "an integer between min and max", "$int(18, 90)"},
	{"$long", "a 64-bit integer between min and max", "$long(1, 1000000000000)"},
	{"$double", "a decimal number between min and max", "$double(0, 100, 2)"},
	{"$decimal", "a Decimal128 between min and max", "$decimal(1, 500, 2)"},
	{"$bool", "true or false", "$bool"},
	{"$date", "a date between two moments (now, -30d, -1y, 2024-01-01…)", `$date("-1y", "now")`},
	{"$objectId", "a new ObjectId", "$objectId"},
	{"$uuid", "a random UUID", "$uuid"},
	{"$pick", "one of the given values", `$pick("new", "paid", "shipped")`},
	{"$seq", "a counter starting at start", "$seq(1)"},
	{"$maybe", "the value, or no field, with a probability (0–1)", "$maybe(0.3, $email)"},
}

// GeneratorInfo describes one marker.
type GeneratorInfo struct {
	Name    string `json:"name"`
	Help    string `json:"help"`
	Example string `json:"example"`
}

func known(name string) bool {
	for _, g := range Generators {
		if g.Name == "$"+name {
			return true
		}
	}
	return false
}

var markerHead = regexp.MustCompile(`^\$([A-Za-z][A-Za-z0-9]*)`)

// Compile parses a template. Generator markers may appear wherever a value
// can; quoted strings are left alone.
func Compile(src string) (*Template, error) {
	var gens []string
	var b strings.Builder
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '"' || c == '\'' { // copy quoted strings as they are
			j := i + 1
			for j < len(src) && src[j] != c {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(src) {
				return nil, apperr.Invalid("A quoted string isn't closed.")
			}
			b.WriteString(src[i : j+1])
			i = j + 1
			continue
		}
		if c == '$' {
			if m := markerHead.FindStringSubmatch(src[i:]); m != nil && known(m[1]) && !keyFollows(src, i+len(m[0])) {
				end := i + len(m[0])
				if end < len(src) && src[end] == '(' {
					close, err := matchParen(src, end)
					if err != nil {
						return nil, err
					}
					end = close + 1
				}
				gens = append(gens, src[i:end])
				fmt.Fprintf(&b, `"\u0000gen%d"`, len(gens)-1)
				i = end
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	doc, err := mql.ParseDocument(b.String())
	if err != nil {
		return nil, apperr.Invalid("The template can't be read: %v", err)
	}
	root, err := build(doc, gens)
	if err != nil {
		return nil, err
	}
	return &Template{root: root}, nil
}

// keyFollows reports a ":" right after a marker: "$name:" is a field name.
func keyFollows(src string, at int) bool {
	for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
		at++
	}
	return at < len(src) && src[at] == ':'
}

func matchParen(src string, open int) (int, error) {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			q := src[i]
			for i++; i < len(src) && src[i] != q; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, apperr.Invalid("A generator's parentheses aren't closed.")
}

func build(v any, gens []string) (*node, error) {
	switch x := v.(type) {
	case bson.D:
		n := &node{kind: "doc"}
		for _, e := range x {
			child, err := build(e.Value, gens)
			if err != nil {
				return nil, err
			}
			n.fields = append(n.fields, field{name: e.Key, node: child})
		}
		return n, nil
	case bson.A:
		n := &node{kind: "array"}
		for _, el := range x {
			child, err := build(el, gens)
			if err != nil {
				return nil, err
			}
			n.elems = append(n.elems, child)
		}
		return n, nil
	case string:
		if strings.HasPrefix(x, "\u0000gen") {
			idx, _ := strconv.Atoi(strings.TrimPrefix(x, "\u0000gen"))
			return compileMarker(gens[idx], gens)
		}
	}
	return &node{kind: "literal", value: v}, nil
}

// compileMarker turns "$int(1, 5)" into a generator node.
func compileMarker(text string, gens []string) (*node, error) {
	m := markerHead.FindStringSubmatch(text)
	n := &node{kind: "gen", gen: m[1], presence: 1}
	rest := strings.TrimSpace(text[len(m[0]):])
	if rest != "" {
		inner := rest[1 : len(rest)-1]
		if n.gen == "maybe" {
			// $maybe(p, <value or marker>)
			comma := strings.IndexByte(inner, ',')
			if comma < 0 {
				return nil, apperr.Invalid("$maybe needs a probability and a value, e.g. $maybe(0.3, $email).")
			}
			p, err := strconv.ParseFloat(strings.TrimSpace(inner[:comma]), 64)
			if err != nil || p < 0 || p > 1 {
				return nil, apperr.Invalid("$maybe's probability must be between 0 and 1.")
			}
			child, err := Compile("{v: " + strings.TrimSpace(inner[comma+1:]) + "}")
			if err != nil {
				return nil, err
			}
			out := child.root.fields[0].node
			out.presence = p
			return out, nil
		}
		args, err := mql.ParseValue("[" + inner + "]")
		if err != nil {
			return nil, apperr.Invalid("The arguments of %s can't be read: %v", m[0], err)
		}
		n.args = args.(bson.A)
	}
	if err := checkArgs(n); err != nil {
		return nil, err
	}
	return n, nil
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func checkArgs(n *node) error {
	needRange := func() error {
		if len(n.args) < 2 {
			return apperr.Invalid("$%s needs a minimum and a maximum, e.g. $%s(1, 100).", n.gen, n.gen)
		}
		lo, ok1 := num(n.args[0])
		hi, ok2 := num(n.args[1])
		if !ok1 || !ok2 || lo > hi {
			return apperr.Invalid("$%s needs a minimum and a maximum that is not smaller.", n.gen)
		}
		return nil
	}
	switch n.gen {
	case "int", "long", "double", "decimal":
		return needRange()
	case "pick":
		if len(n.args) == 0 {
			return apperr.Invalid(`$pick needs values to choose from, e.g. $pick("a", "b").`)
		}
	case "date":
		for _, a := range n.args {
			s, ok := a.(string)
			if !ok {
				if _, isDate := a.(bson.DateTime); !isDate {
					return apperr.Invalid(`$date takes moments like "now", "-30d", "-1y" or "2024-01-01".`)
				}
				continue
			}
			if _, err := moment(s, time.Now()); err != nil {
				return err
			}
		}
	}
	return nil
}

// moment reads "now", "-30d", "+2w", "-1y", "6h" or a date.
func moment(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "now" || s == "" {
		return now, nil
	}
	if m := regexp.MustCompile(`^([+-]?\d+)([smhdwy])$`).FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "s":
			return now.Add(time.Duration(n) * time.Second), nil
		case "m":
			return now.Add(time.Duration(n) * time.Minute), nil
		case "h":
			return now.Add(time.Duration(n) * time.Hour), nil
		case "d":
			return now.AddDate(0, 0, n), nil
		case "w":
			return now.AddDate(0, 0, 7*n), nil
		case "y":
			return now.AddDate(n, 0, 0), nil
		}
	}
	t, err := mql.ParseDate(s)
	if err != nil {
		return time.Time{}, apperr.Invalid("%q is not a moment (now, -30d, -1y, 2024-01-01…).", s)
	}
	return t, nil
}

// ---- generating ----------------------------------------------------------------

// Generator produces documents from a template, repeatably for a seed.
type Generator struct {
	t   *Template
	rng *rand.Rand
	now time.Time
	seq map[*node]int64
}

// NewGenerator returns a generator. now anchors relative dates, so a seed
// gives the same documents only with the same now.
func (t *Template) NewGenerator(seed uint64, now time.Time) *Generator {
	return &Generator{t: t, rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), now: now, seq: map[*node]int64{}}
}

// Next returns the next document.
func (g *Generator) Next() bson.D {
	v, _ := g.value(g.t.root)
	d, _ := v.(bson.D)
	return d
}

// value generates n; ok is false when an optional field is left out.
func (g *Generator) value(n *node) (any, bool) {
	if n.presence > 0 && n.presence < 1 && g.rng.Float64() >= n.presence {
		return nil, false
	}
	switch n.kind {
	case "literal":
		return n.value, true
	case "doc":
		d := bson.D{}
		for _, f := range n.fields {
			if v, ok := g.value(f.node); ok {
				d = append(d, bson.E{Key: f.name, Value: v})
			}
		}
		return d, true
	case "array":
		a := bson.A{}
		for _, el := range n.elems {
			if v, ok := g.value(el); ok {
				a = append(a, v)
			}
		}
		return a, true
	}
	return g.generate(n), true
}

func (g *Generator) pick(list []string) string { return list[g.rng.IntN(len(list))] }

func (g *Generator) between(n *node) (float64, float64) {
	lo, _ := num(n.args[0])
	hi, _ := num(n.args[1])
	return lo, hi
}

func (g *Generator) generate(n *node) any {
	r := g.rng
	switch n.gen {
	case "name":
		return g.pick(firstNames) + " " + g.pick(lastNames)
	case "firstName":
		return g.pick(firstNames)
	case "lastName":
		return g.pick(lastNames)
	case "email":
		return strings.ToLower(g.pick(firstNames)+"."+g.pick(lastNames)) + strconv.Itoa(r.IntN(100)) + "@" + g.pick(domains)
	case "phone":
		return fmt.Sprintf("+1 555 %03d %04d", r.IntN(1000), r.IntN(10000))
	case "city":
		return g.pick(cities)
	case "country":
		return g.pick(countries)
	case "street":
		return fmt.Sprintf("%d %s %s", 1+r.IntN(999), g.pick(lastNames), g.pick([]string{"Street", "Road", "Avenue", "Lane"}))
	case "company":
		return g.pick(lastNames) + " " + g.pick([]string{"Ltd", "Inc", "Group", "Labs", "& Co"})
	case "word":
		return g.pick(words)
	case "words":
		count := 3
		if len(n.args) > 0 {
			if c, ok := num(n.args[0]); ok && c > 0 {
				count = int(c)
			}
		}
		parts := make([]string, count)
		for i := range parts {
			parts[i] = g.pick(words)
		}
		return strings.Join(parts, " ")
	case "sentence":
		parts := make([]string, 5+r.IntN(8))
		for i := range parts {
			parts[i] = g.pick(words)
		}
		s := strings.Join(parts, " ")
		return strings.ToUpper(s[:1]) + s[1:] + "."
	case "int":
		lo, hi := g.between(n)
		return int32(int64(lo) + r.Int64N(int64(hi)-int64(lo)+1))
	case "long":
		lo, hi := g.between(n)
		return int64(lo) + r.Int64N(int64(hi)-int64(lo)+1)
	case "double":
		lo, hi := g.between(n)
		v := lo + r.Float64()*(hi-lo)
		if len(n.args) > 2 {
			if p, ok := num(n.args[2]); ok {
				f := math.Pow10(int(p))
				v = math.Round(v*f) / f
			}
		}
		return v
	case "decimal":
		lo, hi := g.between(n)
		places := 2
		if len(n.args) > 2 {
			if p, ok := num(n.args[2]); ok {
				places = int(p)
			}
		}
		d, _ := bson.ParseDecimal128(strconv.FormatFloat(lo+r.Float64()*(hi-lo), 'f', places, 64))
		return d
	case "bool":
		return r.IntN(2) == 1
	case "date":
		from, to := g.now.AddDate(-1, 0, 0), g.now
		if len(n.args) > 0 {
			from = g.argMoment(n.args[0])
		}
		if len(n.args) > 1 {
			to = g.argMoment(n.args[1])
		}
		if to.Before(from) {
			from, to = to, from
		}
		span := to.Sub(from).Milliseconds()
		if span <= 0 {
			return bson.NewDateTimeFromTime(from)
		}
		return bson.NewDateTimeFromTime(from.Add(time.Duration(r.Int64N(span+1)) * time.Millisecond))
	case "objectId":
		var b [12]byte
		ts := uint32(g.now.Unix())
		b[0], b[1], b[2], b[3] = byte(ts>>24), byte(ts>>16), byte(ts>>8), byte(ts)
		for i := 4; i < 12; i++ {
			b[i] = byte(r.IntN(256))
		}
		return bson.ObjectID(b)
	case "uuid":
		var b [16]byte
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return bson.Binary{Subtype: 4, Data: b[:]}
	case "pick":
		return n.args[r.IntN(len(n.args))]
	case "seq":
		start := int64(1)
		if len(n.args) > 0 {
			if s, ok := num(n.args[0]); ok {
				start = int64(s)
			}
		}
		v := start + g.seq[n]
		g.seq[n]++
		if v >= math.MinInt32 && v <= math.MaxInt32 {
			return int32(v)
		}
		return v
	}
	return nil
}

func (g *Generator) argMoment(a any) time.Time {
	switch x := a.(type) {
	case string:
		t, err := moment(x, g.now)
		if err == nil {
			return t
		}
	case bson.DateTime:
		return x.Time()
	}
	return g.now
}

// ---- from a schema report ------------------------------------------------------

// FromSchema writes a template that resembles the sampled documents: each
// field's most common type, ranges from the sample, typical generators for
// names like "email" or "city", and optional fields kept optional.
func FromSchema(r schema.Report) string {
	type tnode struct {
		f        *schema.Field
		children map[string]*tnode
		order    []string
	}
	root := &tnode{children: map[string]*tnode{}}
	for i := range r.Fields {
		f := &r.Fields[i]
		cur := root
		for _, part := range strings.Split(f.Path, ".") {
			nx, ok := cur.children[part]
			if !ok {
				nx = &tnode{children: map[string]*tnode{}}
				cur.children[part] = nx
				cur.order = append(cur.order, part)
			}
			cur = nx
		}
		cur.f = f
	}
	var render func(n *tnode, total int64, indent string) string
	render = func(n *tnode, total int64, indent string) string {
		var b strings.Builder
		b.WriteString("{\n")
		for i, name := range n.order {
			c := n.children[name]
			if c.f == nil {
				continue
			}
			val := valueFor(name, c.f)
			if t := mainType(c.f.Types); t == "document" && len(c.children) > 0 {
				val = render(c, c.f.Count, indent+"  ")
			} else if t == "array" && len(c.children) > 0 {
				val = "[" + render(c, c.f.Count, indent+"  ") + "]"
			}
			if total > 0 && c.f.Count < total && name != "_id" {
				p := float64(c.f.Count) / float64(total)
				val = fmt.Sprintf("$maybe(%.2f, %s)", p, val)
			}
			fmt.Fprintf(&b, "%s  %s: %s", indent, mql.Quote(name), val)
			if i < len(n.order)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + "}")
		return b.String()
	}
	return render(root, r.Sampled, "")
}

func mainType(ts []schema.TypeStat) string {
	if len(ts) == 0 {
		return ""
	}
	best := ts[0]
	for _, t := range ts {
		if t.Count > best.Count {
			best = t
		}
	}
	return best.Type
}

func valueFor(name string, f *schema.Field) string {
	lower := strings.ToLower(name)
	rng := func(def string) string {
		if f.Min != "" && f.Max != "" {
			lo, hi := stripNum(f.Min), stripNum(f.Max)
			if lo != "" && hi != "" {
				return lo + ", " + hi
			}
		}
		return def
	}
	switch mainType(f.Types) {
	case "objectId":
		return "$objectId"
	case "int32":
		return "$int(" + rng("0, 1000") + ")"
	case "int64":
		return "$long(" + rng("0, 1000000") + ")"
	case "double":
		return "$double(" + rng("0, 1000") + ", 2)"
	case "decimal":
		return "$decimal(" + rng("0, 1000") + ", 2)"
	case "boolean":
		return "$bool"
	case "date":
		if f.Min != "" && f.Max != "" {
			return fmt.Sprintf("$date(%s, %s)", f.Min, f.Max)
		}
		return `$date("-1y", "now")`
	case "binary":
		return "$uuid"
	case "null":
		return "null"
	case "array":
		return "[$word, $word]"
	case "document":
		return "{}"
	case "string":
		switch {
		case strings.Contains(lower, "email"):
			return "$email"
		case lower == "name" || strings.HasSuffix(lower, "name") && !strings.Contains(lower, "user"):
			if strings.Contains(lower, "first") {
				return "$firstName"
			}
			if strings.Contains(lower, "last") {
				return "$lastName"
			}
			return "$name"
		case strings.Contains(lower, "phone"):
			return "$phone"
		case strings.Contains(lower, "city"):
			return "$city"
		case strings.Contains(lower, "country"):
			return "$country"
		case strings.Contains(lower, "street") || strings.Contains(lower, "address"):
			return "$street"
		case strings.Contains(lower, "company"):
			return "$company"
		}
		if f.Distinct > 0 && f.Distinct <= 10 && !f.DistinctCapped && len(f.Samples) > 0 {
			samples := append([]string{}, f.Samples...)
			sort.Strings(samples)
			return "$pick(" + strings.Join(samples, ", ") + ")"
		}
		return "$words(2)"
	}
	return "null"
}

// stripNum turns a shell value like NumberLong(5) into its number.
func stripNum(s string) string {
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		s = strings.Trim(s[i+1:len(s)-1], `"'`)
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return ""
	}
	return s
}
