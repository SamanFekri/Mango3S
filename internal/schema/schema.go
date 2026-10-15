// Package schema infers a collection's structure from a sample of documents:
// every field path with its presence, BSON-type distribution, value ranges,
// array shapes, cardinality and example values.
package schema

import (
	"context"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// TypeStat is how often a field had a given BSON type.
type TypeStat struct {
	Type    string  `json:"type"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

// Field describes one dotted path. Values inside arrays of documents are
// reported under the array's path (e.g. "items.sku").
type Field struct {
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Depth    int        `json:"depth"`
	Count    int64      `json:"count"`    // documents containing the field
	Presence float64    `json:"presence"` // percent of sampled documents
	Types    []TypeStat `json:"types"`
	// Arrays
	ElementTypes []TypeStat `json:"elementTypes,omitempty"`
	MinLength    *int64     `json:"minLength,omitempty"` // array length or string length
	MaxLength    *int64     `json:"maxLength,omitempty"`
	AvgLength    *float64   `json:"avgLength,omitempty"`
	// Ranges for numbers and dates, rendered in shell syntax.
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
	// Cardinality within the sample (capped at distinctCap).
	Distinct       int      `json:"distinct"`
	DistinctCapped bool     `json:"distinctCapped"`
	Unique         bool     `json:"unique"`
	Samples        []string `json:"samples"`
}

// Report is the result of an analysis.
type Report struct {
	Sampled       int64   `json:"sampled"`
	EstimatedDocs int64   `json:"estimatedDocs"`
	Fields        []Field `json:"fields"`
	DurationMs    int64   `json:"durationMs"`
}

const (
	distinctCap = 1000
	sampleCap   = 5
)

type acc struct {
	path      string
	count     int64
	types     map[string]int64
	elemTypes map[string]int64
	lenMin    int64
	lenMax    int64
	lenSum    int64
	lenN      int64
	strMin    int64
	strMax    int64
	strSum    int64
	strN      int64
	numMin    float64
	numMax    float64
	numMinV   any
	numMaxV   any
	dateMin   bson.DateTime
	dateMax   bson.DateTime
	hasDate   bool
	distinct  map[string]struct{}
	capped    bool
	samples   []string
	lastDoc   int64
	valueN    int64
}

// Analyzer accumulates statistics document by document.
type Analyzer struct {
	fields map[string]*acc
	order  []string
	docs   int64
}

// NewAnalyzer returns an empty analyzer.
func NewAnalyzer() *Analyzer { return &Analyzer{fields: map[string]*acc{}} }

// Add analyzes one document.
func (a *Analyzer) Add(d bson.D) {
	a.docs++
	a.walk(d, "", 0)
}

func (a *Analyzer) field(path string) *acc {
	f, ok := a.fields[path]
	if !ok {
		f = &acc{
			path: path, types: map[string]int64{}, elemTypes: map[string]int64{},
			distinct: map[string]struct{}{}, numMin: math.Inf(1), numMax: math.Inf(-1),
			lenMin: math.MaxInt64, strMin: math.MaxInt64,
		}
		a.fields[path] = f
		a.order = append(a.order, path)
	}
	return f
}

func (a *Analyzer) walk(d bson.D, prefix string, depth int) {
	if depth > 20 {
		return
	}
	for _, e := range d {
		path := e.Key
		if prefix != "" {
			path = prefix + "." + e.Key
		}
		a.record(path, e.Value, depth)
	}
}

func (a *Analyzer) record(path string, v any, depth int) {
	f := a.field(path)
	if f.lastDoc != a.docs {
		f.lastDoc = a.docs
		f.count++
	}
	t := mql.TypeName(v)
	f.types[t]++
	f.valueN++

	switch x := v.(type) {
	case bson.D:
		a.walk(x, path, depth+1)
		return
	case bson.A:
		n := int64(len(x))
		f.lenMin, f.lenMax = min(f.lenMin, n), max(f.lenMax, n)
		f.lenSum += n
		f.lenN++
		for _, el := range x {
			f.elemTypes[mql.TypeName(el)]++
			if sub, ok := el.(bson.D); ok {
				a.walk(sub, path, depth+1)
			}
		}
		f.sample(v)
		return
	case string:
		n := int64(len([]rune(x)))
		f.strMin, f.strMax = min(f.strMin, n), max(f.strMax, n)
		f.strSum += n
		f.strN++
	case int32, int64, float64, bson.Decimal128:
		if num, ok := toFloat(x); ok && !math.IsNaN(num) {
			if num < f.numMin {
				f.numMin, f.numMinV = num, v
			}
			if num > f.numMax {
				f.numMax, f.numMaxV = num, v
			}
		}
	case bson.DateTime:
		if !f.hasDate || x < f.dateMin {
			f.dateMin = x
		}
		if !f.hasDate || x > f.dateMax {
			f.dateMax = x
		}
		f.hasDate = true
	}
	f.sample(v)
}

func (f *acc) sample(v any) {
	key := mql.Compact(v)
	if len(key) > 120 {
		key = key[:119] + "…"
	}
	if _, seen := f.distinct[key]; !seen {
		if len(f.distinct) >= distinctCap {
			f.capped = true
		} else {
			f.distinct[key] = struct{}{}
			if len(f.samples) < sampleCap {
				f.samples = append(f.samples, key)
			}
		}
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case bson.Decimal128:
		bi, exp, err := n.BigInt()
		if err != nil {
			return 0, false
		}
		f, _ := new(big.Float).SetInt(bi).Float64()
		return f * math.Pow10(exp), true
	}
	return 0, false
}

func typeStats(m map[string]int64) []TypeStat {
	var total int64
	for _, c := range m {
		total += c
	}
	out := make([]TypeStat, 0, len(m))
	for t, c := range m {
		out = append(out, TypeStat{Type: t, Count: c, Percent: round(100 * float64(c) / float64(total))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func round(f float64) float64 { return math.Round(f*10) / 10 }

// Report builds the final report.
func (a *Analyzer) Report() Report {
	r := Report{Sampled: a.docs, Fields: make([]Field, 0, len(a.fields))}
	for _, path := range a.order {
		f := a.fields[path]
		parts := strings.Split(path, ".")
		out := Field{
			Path:           path,
			Name:           parts[len(parts)-1],
			Depth:          len(parts) - 1,
			Count:          f.count,
			Types:          typeStats(f.types),
			Distinct:       len(f.distinct),
			DistinctCapped: f.capped,
			Samples:        f.samples,
		}
		if a.docs > 0 {
			out.Presence = round(100 * float64(f.count) / float64(a.docs))
		}
		out.Unique = !f.capped && f.valueN > 1 && int64(len(f.distinct)) == f.valueN && f.types["array"] == 0 && f.types["document"] == 0
		if len(f.elemTypes) > 0 {
			out.ElementTypes = typeStats(f.elemTypes)
		}
		switch {
		case f.lenN > 0:
			out.MinLength, out.MaxLength = ptr(f.lenMin), ptr(f.lenMax)
			avg := round(float64(f.lenSum) / float64(f.lenN))
			out.AvgLength = &avg
		case f.strN > 0:
			out.MinLength, out.MaxLength = ptr(f.strMin), ptr(f.strMax)
			avg := round(float64(f.strSum) / float64(f.strN))
			out.AvgLength = &avg
		}
		switch {
		case f.numMinV != nil:
			out.Min, out.Max = mql.Compact(f.numMinV), mql.Compact(f.numMaxV)
		case f.hasDate:
			out.Min, out.Max = mql.Compact(f.dateMin), mql.Compact(f.dateMax)
		}
		if out.Samples == nil {
			out.Samples = []string{}
		}
		r.Fields = append(r.Fields, out)
	}
	sort.SliceStable(r.Fields, func(i, j int) bool { return lessPath(r.Fields[i].Path, r.Fields[j].Path) })
	return r
}

// lessPath orders _id first, then parents before their children, then by name.
func lessPath(a, b string) bool {
	if a == "_id" || b == "_id" {
		return a == "_id" && b != "_id"
	}
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

func ptr(n int64) *int64 { return &n }

// Service samples collections.
type Service struct{ src mongox.Source }

// New returns a schema service.
func New(src mongox.Source) *Service { return &Service{src: src} }

// Request selects what to analyze.
type Request struct {
	ConnID     string `json:"connId"`
	DB         string `json:"db"`
	Coll       string `json:"coll"`
	SampleSize int    `json:"sampleSize"`
	Filter     string `json:"filter"`
}

// MaxSample bounds the sample size.
const MaxSample = 100_000

// Analyze samples documents with $sample (after an optional $match) and
// returns the report.
func (s *Service) Analyze(ctx context.Context, req Request) (Report, error) {
	_, c, err := mongox.Collection(s.src, req.ConnID, req.DB, req.Coll)
	if err != nil {
		return Report{}, err
	}
	if req.SampleSize <= 0 {
		req.SampleSize = 1000
	}
	if req.SampleSize > MaxSample {
		return Report{}, apperr.Invalid("Sample size is limited to %d documents.", MaxSample)
	}
	filter, err := mql.ParseDocument(req.Filter)
	if err != nil {
		return Report{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
	}
	start := time.Now()
	pipe := mongo.Pipeline{}
	if len(filter) > 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: filter}})
	}
	pipe = append(pipe, bson.D{{Key: "$sample", Value: bson.D{{Key: "size", Value: req.SampleSize}}}})
	cur, err := c.Aggregate(ctx, pipe)
	if err != nil {
		return Report{}, apperr.From(err)
	}
	defer cur.Close(context.Background())
	a := NewAnalyzer()
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return Report{}, apperr.From(err)
		}
		a.Add(d)
	}
	if err := cur.Err(); err != nil {
		return Report{}, apperr.From(err)
	}
	r := a.Report()
	if n, err := c.EstimatedDocumentCount(ctx); err == nil {
		r.EstimatedDocs = n
	}
	r.DurationMs = time.Since(start).Milliseconds()
	return r, nil
}
