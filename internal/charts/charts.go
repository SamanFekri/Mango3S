// Package charts turns a chart definition into an aggregation pipeline that
// runs on the server, so a chart covers every document matching the query,
// not one page of results.
package charts

import (
	"context"
	"math"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Spec describes a chart.
type Spec struct {
	Kind     string `json:"kind"`     // bar | line | area | pie | scatter | timeseries
	X        string `json:"x"`        // field path for the x axis / slices
	Y        string `json:"y"`        // field path for the value (not needed for count)
	Agg      string `json:"agg"`      // count | sum | avg | min | max
	DateUnit string `json:"dateUnit"` // day | week | month | year: group a date x by it
	Series   string `json:"series"`   // optional field splitting the data into series
	Limit    int    `json:"limit"`    // most groups (default 200) or scatter points (default 2000)
}

// Request is a chart over a collection, optionally narrowed by a filter.
type Request struct {
	ConnID string `json:"connId"`
	DB     string `json:"db"`
	Coll   string `json:"coll"`
	Filter string `json:"filter"` // mongosh syntax
	Spec   Spec   `json:"spec"`
}

// Point is one value; X is a number (time: Unix milliseconds) or a label.
type Point struct {
	X      any     `json:"x"`
	Y      float64 `json:"y"`
	Series string  `json:"series,omitempty"`
}

// Data is what the chart draws.
type Data struct {
	XKind     string  `json:"xKind"` // time | number | category
	Points    []Point `json:"points"`
	Truncated bool    `json:"truncated"` // the limit cut groups or points off
	Pipeline  string  `json:"pipeline"`  // mongosh syntax, to show or copy
}

var dateUnits = map[string]bool{"day": true, "week": true, "month": true, "year": true}

// validField rejects paths that would be read as operators or expressions.
func validField(name, what string) error {
	if strings.TrimSpace(name) == "" {
		return apperr.Invalid("Choose a field for %s.", what)
	}
	if strings.HasPrefix(name, "$") || strings.Contains(name, "..") {
		return apperr.Invalid("%q is not a field path.", name)
	}
	return nil
}

// Build returns the pipeline for spec, after a $match on filter.
func Build(filter bson.D, s Spec) (mongo.Pipeline, error) {
	pipe := mongo.Pipeline{}
	if len(filter) > 0 {
		pipe = append(pipe, bson.D{{Key: "$match", Value: filter}})
	}
	if err := validField(s.X, "the x axis"); err != nil {
		return nil, err
	}
	if s.Series != "" {
		if err := validField(s.Series, "the series"); err != nil {
			return nil, err
		}
	}
	if s.Kind == "scatter" {
		if err := validField(s.Y, "the y axis"); err != nil {
			return nil, err
		}
		limit := s.Limit
		if limit <= 0 {
			limit = 2000
		}
		proj := bson.D{{Key: "_id", Value: 0}, {Key: "x", Value: "$" + s.X}, {Key: "y", Value: "$" + s.Y}}
		if s.Series != "" {
			proj = append(proj, bson.E{Key: "s", Value: "$" + s.Series})
		}
		return append(pipe,
			bson.D{{Key: "$match", Value: bson.D{{Key: s.Y, Value: bson.D{{Key: "$type", Value: "number"}}}}}},
			bson.D{{Key: "$project", Value: proj}},
			bson.D{{Key: "$limit", Value: limit + 1}},
		), nil
	}

	var value bson.D
	switch s.Agg {
	case "", "count":
		value = bson.D{{Key: "$sum", Value: 1}}
	case "sum", "avg", "min", "max":
		if err := validField(s.Y, "the value"); err != nil {
			return nil, err
		}
		value = bson.D{{Key: "$" + s.Agg, Value: "$" + s.Y}}
	default:
		return nil, apperr.Invalid("Unknown aggregation %q.", s.Agg)
	}
	var x any = "$" + s.X
	if s.DateUnit != "" {
		if !dateUnits[s.DateUnit] {
			return nil, apperr.Invalid("Unknown date unit %q.", s.DateUnit)
		}
		x = truncDate("$"+s.X, s.DateUnit)
		// Documents whose x isn't a date can't be truncated.
		pipe = append(pipe, bson.D{{Key: "$match", Value: bson.D{{Key: s.X, Value: bson.D{{Key: "$type", Value: "date"}}}}}})
	}
	id := bson.D{{Key: "x", Value: x}}
	if s.Series != "" {
		id = append(id, bson.E{Key: "s", Value: "$" + s.Series})
	}
	limit := s.Limit
	if limit <= 0 {
		limit = 200
	}
	sortStage := bson.D{{Key: "_id.x", Value: 1}}
	if s.Kind == "pie" || (s.Kind == "bar" && s.DateUnit == "") {
		sortStage = bson.D{{Key: "v", Value: -1}} // the biggest slices and bars first
	}
	return append(pipe,
		bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: id}, {Key: "v", Value: value}}}},
		bson.D{{Key: "$sort", Value: sortStage}},
		bson.D{{Key: "$limit", Value: limit + 1}},
	), nil
}

// Service runs charts on open connections.
type Service struct{ src mongox.Source }

// New returns a chart service.
func New(src mongox.Source) *Service { return &Service{src: src} }

// Run builds and runs the chart's pipeline.
func (s *Service) Run(ctx context.Context, r Request) (Data, error) {
	sess, err := s.src.Session(r.ConnID)
	if err != nil {
		return Data{}, err
	}
	var filter bson.D
	if strings.TrimSpace(r.Filter) != "" {
		if filter, err = mql.ParseDocument(r.Filter); err != nil {
			return Data{}, apperr.Invalid("The filter can't be read: %v", err)
		}
	}
	pipe, err := Build(filter, r.Spec)
	if err != nil {
		return Data{}, err
	}
	arr := bson.A{}
	for _, st := range pipe {
		arr = append(arr, st)
	}
	data := Data{Points: []Point{}, Pipeline: mql.Pretty(arr)}
	cur, err := sess.Client.Database(r.DB).Collection(r.Coll).Aggregate(ctx, pipe, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return Data{}, apperr.From(err)
	}
	defer cur.Close(ctx)
	limit := r.Spec.Limit
	if limit <= 0 {
		limit = 200
		if r.Spec.Kind == "scatter" {
			limit = 2000
		}
	}
	kinds := map[string]int{}
	for cur.Next(ctx) {
		if len(data.Points) >= limit {
			data.Truncated = true
			break
		}
		var row bson.D
		if err := cur.Decode(&row); err != nil {
			return Data{}, apperr.From(err)
		}
		var x, y, series any
		if r.Spec.Kind == "scatter" {
			x, _ = mql.Lookup(row, "x")
			y, _ = mql.Lookup(row, "y")
			series, _ = mql.Lookup(row, "s")
		} else {
			id, _ := mql.Lookup(row, "_id")
			if idd, ok := id.(bson.D); ok {
				x, _ = mql.Lookup(idd, "x")
				series, _ = mql.Lookup(idd, "s")
			}
			y, _ = mql.Lookup(row, "v")
		}
		px, kind := axisValue(x)
		kinds[kind]++
		p := Point{X: px, Y: number(y)}
		if series != nil {
			p.Series = label(series)
		}
		data.Points = append(data.Points, p)
	}
	if err := cur.Err(); err != nil {
		return Data{}, apperr.From(err)
	}
	// One kind of x for the whole chart: a mix falls back to labels.
	switch {
	case len(kinds) == 1 && kinds["time"] > 0:
		data.XKind = "time"
	case len(kinds) == 1 && kinds["number"] > 0 && r.Spec.Kind != "bar" && r.Spec.Kind != "pie":
		data.XKind = "number"
	default:
		data.XKind = "category"
		for i := range data.Points {
			data.Points[i].X = label(rawX(data.Points[i].X))
		}
	}
	return data, nil
}

// axisValue returns x as the chart sees it and its kind.
func axisValue(x any) (any, string) {
	switch v := x.(type) {
	case bson.DateTime:
		return float64(v.Time().UnixMilli()), "time"
	case time.Time:
		return float64(v.UnixMilli()), "time"
	case int32, int64, float64, bson.Decimal128:
		return number(v), "number"
	}
	return label(x), "category"
}

func rawX(x any) any {
	if f, ok := x.(float64); ok && f == math.Trunc(f) {
		return int64(f)
	}
	return x
}

// label is a value as text for a category axis or a series name.
func label(v any) string {
	switch x := v.(type) {
	case nil:
		return "(none)"
	case string:
		return x
	}
	return mql.Compact(v)
}

func number(v any) float64 {
	switch x := v.(type) {
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case bson.Decimal128:
		f, err := mql.ParseValue(x.String())
		if err == nil {
			if n, ok := f.(float64); ok {
				return n
			}
		}
		bi, exp, err := x.BigInt()
		if err == nil {
			f, _ := bi.Float64()
			return f * math.Pow10(exp)
		}
	}
	return 0
}

// truncDate rounds a date down to its day, ISO week (Monday), month or year.
// $dateFromParts works on every server version ($dateTrunc needs 5.0).
func truncDate(field, unit string) bson.D {
	part := func(op string) bson.D { return bson.D{{Key: op, Value: field}} }
	var parts bson.D
	switch unit {
	case "week":
		parts = bson.D{{Key: "isoWeekYear", Value: part("$isoWeekYear")}, {Key: "isoWeek", Value: part("$isoWeek")}}
	case "year":
		parts = bson.D{{Key: "year", Value: part("$year")}}
	case "month":
		parts = bson.D{{Key: "year", Value: part("$year")}, {Key: "month", Value: part("$month")}}
	default: // day
		parts = bson.D{{Key: "year", Value: part("$year")}, {Key: "month", Value: part("$month")}, {Key: "day", Value: part("$dayOfMonth")}}
	}
	return bson.D{{Key: "$dateFromParts", Value: parts}}
}
