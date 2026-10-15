// Package query executes find queries with server-side pagination.
package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Request is a find query as entered in the query editor. Filter, Projection
// and Sort are shell-syntax documents; blank means "none".
type Request struct {
	ConnID     string `json:"connId"`
	DB         string `json:"db"`
	Coll       string `json:"coll"`
	Filter     string `json:"filter"`
	Projection string `json:"projection"`
	Sort       string `json:"sort"`
	Skip       int64  `json:"skip"`
	Limit      int64  `json:"limit"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// Parsed is a validated request.
type Parsed struct {
	Filter     bson.D
	Projection bson.D
	Sort       bson.D
}

// Parse validates the request's text fields.
func Parse(r Request) (Parsed, error) {
	var p Parsed
	var err error
	if r.Skip < 0 || r.Limit < 0 {
		return p, apperr.Invalid("Skip and limit must not be negative.")
	}
	if p.Filter, err = parseField("Filter", r.Filter); err != nil {
		return p, err
	}
	if p.Projection, err = parseField("Projection", r.Projection); err != nil {
		return p, err
	}
	if p.Sort, err = parseField("Sort", r.Sort); err != nil {
		return p, err
	}
	return p, nil
}

func parseField(name, text string) (bson.D, error) {
	d, err := mql.ParseDocument(text)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, name+": "+err.Error())
	}
	return d, nil
}

// Service runs queries.
type Service interface {
	Find(ctx context.Context, r Request) (model.Page, error)
	Count(ctx context.Context, r Request) (int64, error)
}

type service struct{ src mongox.Source }

// New returns a query service backed by src.
func New(src mongox.Source) Service { return &service{src: src} }

func (s *service) Find(ctx context.Context, r Request) (model.Page, error) {
	_, c, err := mongox.Collection(s.src, r.ConnID, r.DB, r.Coll)
	if err != nil {
		return model.Page{}, err
	}
	p, err := Parse(r)
	if err != nil {
		return model.Page{}, err
	}
	paging := model.ComputePaging(r.Skip, r.Limit, r.Page, r.PageSize)
	page := model.Page{Documents: []model.Document{}, Page: max(r.Page, 0), PageSize: r.PageSize}
	if page.PageSize <= 0 {
		page.PageSize = model.DefaultPageSize
	}
	if paging.Limit == 0 {
		return page, nil
	}

	start := time.Now()
	opts := options.Find().SetSkip(paging.Skip).SetLimit(paging.Limit + 1).SetBatchSize(int32(paging.Limit + 1))
	if len(p.Projection) > 0 {
		opts.SetProjection(p.Projection)
	}
	if len(p.Sort) > 0 {
		opts.SetSort(p.Sort)
	}
	cur, err := c.Find(ctx, p.Filter, opts)
	if err != nil {
		return model.Page{}, apperr.From(err)
	}
	// Close with a fresh context so the server cursor is released even if ctx was canceled.
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
	page.HasMore = paging.HasMore(fetched, r.Skip, r.Limit)
	page.DurationMs = time.Since(start).Milliseconds()
	return page, nil
}

func (s *service) Count(ctx context.Context, r Request) (int64, error) {
	_, c, err := mongox.Collection(s.src, r.ConnID, r.DB, r.Coll)
	if err != nil {
		return 0, err
	}
	p, err := Parse(r)
	if err != nil {
		return 0, err
	}
	opts := options.Count()
	if r.Skip > 0 {
		opts.SetSkip(r.Skip)
	}
	if r.Limit > 0 {
		opts.SetLimit(r.Limit)
	}
	n, err := c.CountDocuments(ctx, p.Filter, opts)
	if err != nil {
		return 0, apperr.From(err)
	}
	return n, nil
}

// ToShell renders the request as mongosh code (for "copy as shell").
func ToShell(r Request) (string, error) {
	p, err := Parse(r)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s.find(%s", CollectionRef(r.Coll), mql.Compact(p.Filter))
	if len(p.Projection) > 0 {
		fmt.Fprintf(&b, ", %s", mql.Compact(p.Projection))
	}
	b.WriteString(")")
	if len(p.Sort) > 0 {
		fmt.Fprintf(&b, "\n  .sort(%s)", mql.Compact(p.Sort))
	}
	if r.Skip > 0 {
		fmt.Fprintf(&b, "\n  .skip(%d)", r.Skip)
	}
	if r.Limit > 0 {
		fmt.Fprintf(&b, "\n  .limit(%d)", r.Limit)
	}
	return b.String(), nil
}

// CollectionRef renders db.<name>, falling back to db.getCollection("...")
// for names that are not valid identifiers.
func CollectionRef(name string) string {
	ok := name != ""
	for i, c := range name {
		if !(c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			ok = false
			break
		}
	}
	if ok {
		return "db." + name
	}
	return "db.getCollection(" + mql.Quote(name) + ")"
}
