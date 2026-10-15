// Package aggregate builds and runs aggregation pipelines from editable stages.
package aggregate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Stage is one editable pipeline stage: an operator such as "$match" and its
// argument in shell syntax ("{ status: 'A' }", "10", "'$tags'").
type Stage struct {
	Operator string `json:"operator"`
	Body     string `json:"body"`
	Enabled  bool   `json:"enabled"`
}

// Request runs the pipeline. UpTo limits execution to stages[0..UpTo]
// (inclusive) for previewing an intermediate stage; -1 runs all stages.
type Request struct {
	ConnID       string  `json:"connId"`
	DB           string  `json:"db"`
	Coll         string  `json:"coll"`
	Stages       []Stage `json:"stages"`
	UpTo         int     `json:"upTo"`
	Page         int     `json:"page"`
	PageSize     int     `json:"pageSize"`
	AllowDiskUse bool    `json:"allowDiskUse"`
}

// writeStages must be last and produce no output documents.
var writeStages = map[string]bool{"$out": true, "$merge": true}

// Build converts stages into a pipeline, skipping disabled stages. Errors name
// the 1-based stage number and operator.
func Build(stages []Stage, upTo int) (mongo.Pipeline, error) {
	if upTo < 0 || upTo >= len(stages) {
		upTo = len(stages) - 1
	}
	pipe := mongo.Pipeline{}
	for i := 0; i <= upTo; i++ {
		st := stages[i]
		if !st.Enabled {
			continue
		}
		op := strings.TrimSpace(st.Operator)
		label := fmt.Sprintf("Stage %d (%s)", i+1, op)
		if !strings.HasPrefix(op, "$") || len(op) < 2 {
			return nil, apperr.Invalid("Stage %d: operator must start with '$' (got %q).", i+1, op)
		}
		if strings.TrimSpace(st.Body) == "" {
			return nil, apperr.Invalid("%s: stage body is empty.", label)
		}
		v, err := mql.ParseValue(st.Body)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInvalidInput, err, label+": "+err.Error())
		}
		if writeStages[op] && i != lastEnabled(stages, upTo) {
			return nil, apperr.Invalid("%s must be the last stage of the pipeline.", label)
		}
		pipe = append(pipe, bson.D{{Key: op, Value: v}})
	}
	return pipe, nil
}

func lastEnabled(stages []Stage, upTo int) int {
	for i := upTo; i >= 0; i-- {
		if stages[i].Enabled {
			return i
		}
	}
	return -1
}

// IsWrite reports whether the pipeline ends with $out or $merge.
func IsWrite(p mongo.Pipeline) bool {
	return len(p) > 0 && len(p[len(p)-1]) > 0 && writeStages[p[len(p)-1][0].Key]
}

// Service runs pipelines.
type Service interface {
	Run(ctx context.Context, r Request) (model.Page, error)
}

type service struct{ src mongox.Source }

// New returns an aggregation service backed by src.
func New(src mongox.Source) Service { return &service{src: src} }

func (s *service) Run(ctx context.Context, r Request) (model.Page, error) {
	sess, c, err := mongox.Collection(s.src, r.ConnID, r.DB, r.Coll)
	if err != nil {
		return model.Page{}, err
	}
	pipe, err := Build(r.Stages, r.UpTo)
	if err != nil {
		return model.Page{}, err
	}
	pageSize := r.PageSize
	if pageSize <= 0 {
		pageSize = model.DefaultPageSize
	}
	page := model.Page{Documents: []model.Document{}, Page: max(r.Page, 0), PageSize: pageSize}
	opts := options.Aggregate().SetAllowDiskUse(r.AllowDiskUse)
	start := time.Now()

	if IsWrite(pipe) {
		if err := sess.RequireWritable(); err != nil {
			return model.Page{}, err
		}
		cur, err := c.Aggregate(ctx, pipe, opts)
		if err != nil {
			return model.Page{}, apperr.From(err)
		}
		_ = cur.Close(context.Background())
		page.DurationMs = time.Since(start).Milliseconds()
		page.Notice = fmt.Sprintf("Pipeline finished; results were written by %s.", pipe[len(pipe)-1][0].Key)
		return page, nil
	}

	paging := model.ComputePaging(0, 0, page.Page, pageSize)
	paged := append(mongo.Pipeline{}, pipe...)
	if paging.Skip > 0 {
		paged = append(paged, bson.D{{Key: "$skip", Value: paging.Skip}})
	}
	paged = append(paged, bson.D{{Key: "$limit", Value: paging.Limit + 1}})
	opts.SetBatchSize(int32(paging.Limit + 1))

	cur, err := c.Aggregate(ctx, paged, opts)
	if err != nil {
		return model.Page{}, apperr.From(err)
	}
	defer cur.Close(context.Background())
	fetched := 0
	for cur.Next(ctx) {
		fetched++
		if int64(fetched) > paging.Limit {
			break
		}
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return model.Page{}, apperr.From(err)
		}
		page.Documents = append(page.Documents, model.NewDocument(d))
	}
	if err := cur.Err(); err != nil {
		return model.Page{}, apperr.From(err)
	}
	page.HasMore = paging.HasMore(fetched, 0, 0)
	page.DurationMs = time.Since(start).Milliseconds()
	return page, nil
}

// ToText renders stages (including disabled ones, commented out) as a mongosh
// pipeline array.
func ToText(stages []Stage) (string, error) {
	var b strings.Builder
	b.WriteString("[\n")
	for i, st := range stages {
		v, err := mql.ParseValue(st.Body)
		if err != nil {
			return "", apperr.Wrap(apperr.CodeInvalidInput, err, fmt.Sprintf("Stage %d (%s): %s", i+1, st.Operator, err))
		}
		body := indentLines(mql.Pretty(bson.D{{Key: st.Operator, Value: v}}), "  ")
		if !st.Enabled {
			body = indentLines(mql.Pretty(bson.D{{Key: st.Operator, Value: v}}), "  // ")
		}
		b.WriteString(body)
		if i < len(stages)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]")
	return b.String(), nil
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// FromText parses a pipeline array (or a single stage document) into stages.
func FromText(text string) ([]Stage, error) {
	v, err := mql.ParseValue(text)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, err.Error())
	}
	var items bson.A
	switch t := v.(type) {
	case bson.A:
		items = t
	case bson.D:
		items = bson.A{t}
	default:
		return nil, apperr.Invalid("A pipeline must be an array of stage documents.")
	}
	out := make([]Stage, 0, len(items))
	for i, it := range items {
		d, ok := it.(bson.D)
		if !ok || len(d) != 1 || !strings.HasPrefix(d[0].Key, "$") {
			return nil, apperr.Invalid("Stage %d must be a document with exactly one $operator.", i+1)
		}
		out = append(out, Stage{Operator: d[0].Key, Body: mql.Pretty(d[0].Value), Enabled: true})
	}
	return out, nil
}
