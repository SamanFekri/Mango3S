package ai

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/plans"
	"mango3s/internal/schema"
)

// Service generates, validates and (on confirmation) executes proposals.
type Service struct {
	src       mongox.Source
	schema    *schema.Service
	backupDir string
	proposals *plans.Store[Proposal]
}

// NewService returns an AI service.
func NewService(src mongox.Source, sch *schema.Service, backupDir string) *Service {
	return &Service{src: src, schema: sch, backupDir: backupDir, proposals: plans.New[Proposal](plans.DefaultTTL)}
}

// PreviewContext returns exactly what would be sent, without calling the provider.
func (s *Service) PreviewContext(ctx context.Context, req Request) (Context, error) {
	if _, _, err := mongox.Collection(s.src, req.ConnID, req.DB, req.Coll); err != nil {
		return Context{}, err
	}
	rep, err := s.schema.Analyze(ctx, schema.Request{ConnID: req.ConnID, DB: req.DB, Coll: req.Coll, SampleSize: 200})
	if err != nil {
		return Context{}, err
	}
	var samples []bson.D
	if req.IncludeSamples {
		if samples, err = s.maskedSamples(ctx, req, 3); err != nil {
			return Context{}, err
		}
	}
	return BuildContext(req.Coll, rep, samples), nil
}

func (s *Service) maskedSamples(ctx context.Context, req Request, n int) ([]bson.D, error) {
	_, c, err := mongox.Collection(s.src, req.ConnID, req.DB, req.Coll)
	if err != nil {
		return nil, err
	}
	cur, err := c.Aggregate(ctx, mongo.Pipeline{{{Key: "$sample", Value: bson.D{{Key: "size", Value: n}}}}})
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	var out []bson.D
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return nil, apperr.From(err)
		}
		out = append(out, MaskShape(d, rng).(bson.D))
	}
	return out, apperr.Check(cur.Err())
}

// MaskShape keeps a document's structure and types but replaces every
// value: strings keep length and character classes, numbers keep their type
// and magnitude only roughly, dates move by up to a year, binary is emptied.
func MaskShape(v any, rng *rand.Rand) any {
	switch t := v.(type) {
	case bson.D:
		out := make(bson.D, len(t))
		for i, e := range t {
			out[i] = bson.E{Key: e.Key, Value: MaskShape(e.Value, rng)}
		}
		return out
	case bson.A:
		out := make(bson.A, 0, min(len(t), 3))
		for _, e := range t[:min(len(t), 3)] {
			out = append(out, MaskShape(e, rng))
		}
		return out
	case string:
		var b strings.Builder
		for _, c := range t {
			switch {
			case c >= '0' && c <= '9':
				b.WriteByte(byte('0' + rng.IntN(10)))
			case c >= 'A' && c <= 'Z':
				b.WriteByte(byte('A' + rng.IntN(26)))
			case c >= 'a' && c <= 'z', c > 127:
				b.WriteByte(byte('a' + rng.IntN(26)))
			default:
				b.WriteRune(c)
			}
		}
		return b.String()
	case int32:
		return int32(rng.IntN(1000))
	case int64:
		return int64(rng.IntN(100000))
	case float64:
		return float64(rng.IntN(100000)) / 100
	case bson.Decimal128:
		d, _ := bson.ParseDecimal128(fmt.Sprintf("%d.%02d", rng.IntN(1000), rng.IntN(100)))
		return d
	case bson.DateTime:
		return bson.NewDateTimeFromTime(t.Time().Add(time.Duration(rng.IntN(730)-365) * 24 * time.Hour))
	case bson.ObjectID:
		return bson.NewObjectID()
	case bson.Binary:
		return bson.Binary{Subtype: t.Subtype}
	}
	return v // null, bool, MinKey/MaxKey, regex, timestamp: not identifying on their own
}

// Generate asks the provider and returns a validated proposal.
func (s *Service) Generate(ctx context.Context, p Provider, req Request) (*Proposal, error) {
	start := time.Now()
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, apperr.Invalid("Describe what you want to query.")
	}
	if len(req.Prompt) > 4000 {
		return nil, apperr.Invalid("The request is too long (4000 characters max).")
	}
	sent, err := s.PreviewContext(ctx, req)
	if err != nil {
		return nil, err
	}
	raw, usage, err := p.Generate(ctx, SystemPrompt, UserMessage(req, sent), AnswerSchema())
	if err != nil {
		return nil, err
	}
	prop, err := Validate(ExtractJSON(raw), req)
	if err != nil {
		return nil, err
	}
	prop.Sent, prop.Model, prop.InputTokens, prop.OutputTokens = sent, usage.Model, usage.InputTokens, usage.OutputTokens
	prop.ConnID = req.ConnID
	if prop.Valid {
		s.serverValidate(ctx, prop)
	}
	prop.DurationMs = timeSince(start).Milliseconds()
	prop.ID = s.proposals.Put(*prop)
	return prop, nil
}

// serverValidate plans the proposal on the server without executing it.
func (s *Service) serverValidate(ctx context.Context, p *Proposal) {
	sess, err := s.src.Session(p.ConnID)
	if err != nil {
		return
	}
	var inner bson.D
	filter, _ := mql.ParseDocument(p.Filter)
	switch p.Kind {
	case KindFind:
		inner = bson.D{{Key: "find", Value: p.Coll}, {Key: "filter", Value: filter}}
		if proj, _ := mql.ParseDocument(p.Projection); len(proj) > 0 {
			inner = append(inner, bson.E{Key: "projection", Value: proj})
		}
		if sort, _ := mql.ParseDocument(p.Sort); len(sort) > 0 {
			inner = append(inner, bson.E{Key: "sort", Value: sort})
		}
	case KindAggregate:
		pipe, _ := aggregate.Build(p.Pipeline, -1)
		inner = bson.D{{Key: "aggregate", Value: p.Coll}, {Key: "pipeline", Value: pipe}, {Key: "cursor", Value: bson.D{}}}
	case KindUpdateMany:
		upd, _ := mql.ParseValue(p.Update)
		inner = bson.D{{Key: "update", Value: p.Coll}, {Key: "updates", Value: bson.A{bson.D{{Key: "q", Value: filter}, {Key: "u", Value: upd}, {Key: "multi", Value: true}}}}}
	case KindDeleteMany:
		inner = bson.D{{Key: "delete", Value: p.Coll}, {Key: "deletes", Value: bson.A{bson.D{{Key: "q", Value: filter}, {Key: "limit", Value: 0}}}}}
	}
	cmd := bson.D{{Key: "explain", Value: inner}, {Key: "verbosity", Value: "queryPlanner"}}
	err = sess.Client.Database(p.DB).RunCommand(ctx, cmd).Err()
	if err != nil {
		p.Validation = append(p.Validation, Check{Name: "Server validation", OK: false, Detail: apperr.From(err).Message})
		p.Valid = false
		return
	}
	p.Validation = append(p.Validation, Check{Name: "Server validation", OK: true, Detail: "the server accepted and planned the query (not executed)"})
}

// WritePreview describes what a write proposal would change.
type WritePreview struct {
	ProposalID  string   `json:"proposalId"`
	Shell       string   `json:"shell"`
	Affected    int64    `json:"affected"`
	SampleIDs   []string `json:"sampleIds"`
	ConfirmText string   `json:"confirmText"`
}

// PreviewWrite counts the documents a write proposal would touch.
func (s *Service) PreviewWrite(ctx context.Context, id string) (WritePreview, error) {
	p, err := s.proposals.Get(id)
	if err != nil {
		return WritePreview{}, err
	}
	if !p.IsWrite || p.Kind == KindAggregate {
		return WritePreview{}, apperr.Invalid("Only updateMany and deleteMany proposals run through the write path; open pipelines in the aggregation editor.")
	}
	if !p.Valid {
		return WritePreview{}, apperr.Invalid("This proposal failed validation and cannot run.")
	}
	_, c, err := mongox.Collection(s.src, p.ConnID, p.DB, p.Coll)
	if err != nil {
		return WritePreview{}, err
	}
	filter, _ := mql.ParseDocument(p.Filter)
	n, err := c.CountDocuments(ctx, filter)
	if err != nil {
		return WritePreview{}, apperr.From(err)
	}
	wp := WritePreview{ProposalID: id, Shell: p.Shell, Affected: n, ConfirmText: p.DB + "." + p.Coll, SampleIDs: []string{}}
	cur, err := c.Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}).SetLimit(10))
	if err == nil {
		defer cur.Close(context.Background())
		for cur.Next(ctx) {
			var d bson.D
			if cur.Decode(&d) == nil {
				if v, ok := mql.Lookup(d, "_id"); ok {
					wp.SampleIDs = append(wp.SampleIDs, mql.Compact(v))
				}
			}
		}
	}
	return wp, nil
}

// WriteResult is the outcome of an executed write proposal.
type WriteResult struct {
	Matched    int64  `json:"matched"`
	Modified   int64  `json:"modified"`
	Deleted    int64  `json:"deleted"`
	BackupPath string `json:"backupPath"`
}

// ExecuteWrite runs a confirmed write proposal (single use). Documents are
// processed in _id batches: each batch is backed up, then written with the
// original filter AND the batch's ids, so nothing outside the backup changes.
func (s *Service) ExecuteWrite(ctx context.Context, id, confirm string) (WriteResult, error) {
	p, err := s.proposals.Get(id)
	if err != nil {
		return WriteResult{}, err
	}
	if !p.IsWrite || (p.Kind != KindUpdateMany && p.Kind != KindDeleteMany) || !p.Valid {
		return WriteResult{}, apperr.Invalid("This proposal cannot be executed through the write path.")
	}
	if strings.TrimSpace(confirm) != p.DB+"."+p.Coll {
		return WriteResult{}, apperr.Invalid("Type %s.%s to confirm.", p.DB, p.Coll)
	}
	sess, c, err := mongox.Collection(s.src, p.ConnID, p.DB, p.Coll)
	if err != nil {
		return WriteResult{}, err
	}
	action := mongox.ActionUpdate
	if p.Kind == KindDeleteMany {
		action = mongox.ActionRemove
	}
	if err := sess.RequireWrite(ctx, p.DB, p.Coll, action, mongox.ActionFind); err != nil {
		return WriteResult{}, err
	}
	if _, err := s.proposals.Take(id); err != nil {
		return WriteResult{}, err
	}
	filter, _ := mql.ParseDocument(p.Filter)
	var update any
	if p.Kind == KindUpdateMany {
		update, _ = mql.ParseValue(p.Update)
	}
	bk, err := backup.Create(s.backupDir, "ai-write", p.Shell, p.DB, p.Coll)
	if err != nil {
		return WriteResult{}, apperr.Wrap(apperr.CodeInternal, err, "Could not create the backup; nothing was changed.")
	}
	defer bk.Close()
	r, err := backup.WriteManyWithBackup(ctx, c, filter, update, bk, 0)
	res := WriteResult{Matched: r.Matched, Modified: r.Modified, Deleted: r.Deleted, BackupPath: bk.Info().Path}
	if err != nil {
		return res, err
	}
	return res, bk.Close()
}
