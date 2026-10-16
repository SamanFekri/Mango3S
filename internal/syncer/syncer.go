// Package syncer makes a target collection match a source collection. A sync
// is always two steps: Preview builds a plan from a comparison (the exact set
// of inserts, replaces and deletes with the document hashes that were seen),
// and Apply executes exactly that plan. Every write is re-verified against the
// previewed state, so documents that changed after the preview are skipped and
// reported instead of being overwritten. Before-images go to a backup first.
package syncer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/compare"
	"mango3s/internal/jobs"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/plans"
)

// Options selects which kinds of differences to synchronize.
type Options struct {
	Insert  bool `json:"insert"`  // copy documents that exist only in the source
	Replace bool `json:"replace"` // overwrite changed documents with the source version
	Delete  bool `json:"delete"`  // remove documents that exist only in the target
}

// Plan is a previewed, not yet applied synchronization.
type Plan struct {
	Compare  compare.Options
	Options  Options
	Inserts  []compare.Entry
	Replaces []compare.Entry
	Deletes  []compare.Entry
	Samples  []compare.DocDiff
	Created  time.Time
}

// Preview is what the user confirms.
type Preview struct {
	PlanID       string            `json:"planId"`
	Target       string            `json:"target"`
	Source       string            `json:"source"`
	Inserts      int               `json:"inserts"`
	Replaces     int               `json:"replaces"`
	Deletes      int               `json:"deletes"`
	Samples      []compare.DocDiff `json:"samples"`
	ConfirmText  string            `json:"confirmText"` // must be typed back when the plan deletes
	ExpiresInSec int               `json:"expiresInSec"`
}

// Outcome is the result of applying a plan.
type Outcome struct {
	Planned         Counts   `json:"planned"`
	Applied         Counts   `json:"applied"`
	Skipped         Counts   `json:"skipped"` // drift: changed since the preview
	SkippedIDs      []string `json:"skippedIds"`
	MatchesPreview  bool     `json:"matchesPreview"`
	BackupPath      string   `json:"backupPath"`
	VerifiedEqual   bool     `json:"verifiedEqual"` // post-apply comparison of the synced kinds
	Remaining       Counts   `json:"remaining"`     // differences of the synced kinds left after apply
	PostFingerprint string   `json:"postFingerprint"`
	DurationMs      int64    `json:"durationMs"`
}

// Counts per operation kind.
type Counts struct {
	Inserts  int `json:"inserts"`
	Replaces int `json:"replaces"`
	Deletes  int `json:"deletes"`
}

// Service previews and applies synchronizations.
type Service struct {
	src       mongox.Source
	jobs      *jobs.Manager
	compare   *compare.Service
	backupDir string
	plans     *plans.Store[Plan]
	mu        sync.Mutex
	outcomes  map[string]*Outcome
}

// New returns a sync service.
func New(src mongox.Source, jm *jobs.Manager, cmp *compare.Service, backupDir string) *Service {
	return &Service{src: src, jobs: jm, compare: cmp, backupDir: backupDir, plans: plans.New[Plan](plans.DefaultTTL), outcomes: map[string]*Outcome{}}
}

// BuildPlan derives a plan from a comparison result.
func BuildPlan(r *compare.Result, o Options) (Plan, error) {
	if !o.Insert && !o.Replace && !o.Delete {
		return Plan{}, apperr.Invalid("Choose at least one of insert, replace or delete.")
	}
	p := Plan{Compare: r.Options, Options: o, Created: time.Now()}
	for _, e := range r.Entries {
		switch {
		case e.Kind == compare.KindOnlySource && o.Insert:
			p.Inserts = append(p.Inserts, e)
		case e.Kind == compare.KindChanged && o.Replace:
			p.Replaces = append(p.Replaces, e)
		case e.Kind == compare.KindOnlyTarget && o.Delete:
			p.Deletes = append(p.Deletes, e)
		}
	}
	for _, s := range r.Samples {
		if (s.Kind == compare.KindOnlySource && o.Insert) || (s.Kind == compare.KindChanged && o.Replace) || (s.Kind == compare.KindOnlyTarget && o.Delete) {
			p.Samples = append(p.Samples, s)
		}
	}
	return p, nil
}

func (p Plan) counts() Counts {
	return Counts{Inserts: len(p.Inserts), Replaces: len(p.Replaces), Deletes: len(p.Deletes)}
}

// Preview builds and registers a plan from a finished comparison job.
func (s *Service) Preview(compareJobID string, o Options) (Preview, error) {
	r, err := s.compare.CollectionResult(compareJobID)
	if err != nil {
		return Preview{}, err
	}
	p, err := BuildPlan(r, o)
	if err != nil {
		return Preview{}, err
	}
	pv := Preview{
		Target: r.Options.Target.String(), Source: r.Options.Source.String(),
		Inserts: len(p.Inserts), Replaces: len(p.Replaces), Deletes: len(p.Deletes),
		Samples: p.Samples, ExpiresInSec: int(plans.DefaultTTL.Seconds()),
	}
	if pv.Samples == nil {
		pv.Samples = []compare.DocDiff{}
	}
	if len(p.Deletes) > 0 {
		pv.ConfirmText = pv.Target
	}
	pv.PlanID = s.plans.Put(p)
	return pv, nil
}

// StartApply validates confirmation and queues the apply job.
func (s *Service) StartApply(planID, confirm string) (jobs.Job, error) {
	p, err := s.plans.Get(planID)
	if err != nil {
		return jobs.Job{}, err
	}
	if len(p.Deletes) > 0 && strings.TrimSpace(confirm) != p.Compare.Target.String() {
		return jobs.Job{}, apperr.Invalid("This sync deletes %d documents. Type %q to confirm.", len(p.Deletes), p.Compare.Target.String())
	}
	if len(p.Inserts)+len(p.Replaces)+len(p.Deletes) == 0 {
		return jobs.Job{}, apperr.Invalid("Nothing to synchronize.")
	}
	ts, err := s.src.Session(p.Compare.Target.ConnID)
	if err != nil {
		return jobs.Job{}, err
	}
	if err := ts.RequireWritable(); err != nil {
		return jobs.Job{}, err
	}
	if p, err = s.plans.Take(planID); err != nil { // single use
		return jobs.Job{}, err
	}
	title := fmt.Sprintf("Sync %s → %s", p.Compare.Source, p.Compare.Target)
	return s.jobs.Start("sync", title, func(ctx context.Context, pr *jobs.Progress) (string, error) {
		out, err := Apply(ctx, s.src, p, s.backupDir, pr)
		if out != nil {
			s.mu.Lock()
			s.outcomes[pr.ID()] = out
			s.mu.Unlock()
		}
		if err != nil {
			return "", err
		}
		summary := fmt.Sprintf("Inserted %d, replaced %d, deleted %d", out.Applied.Inserts, out.Applied.Replaces, out.Applied.Deletes)
		if !out.MatchesPreview {
			summary += fmt.Sprintf("; skipped %d changed since the preview", out.Skipped.Inserts+out.Skipped.Replaces+out.Skipped.Deletes)
		}
		return summary, nil
	}), nil
}

// Outcome returns the result of an apply job.
func (s *Service) Outcome(jobID string) (*Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.outcomes[jobID]; ok {
		return o, nil
	}
	return nil, apperr.New(apperr.CodeNotFound, "Sync result not found.")
}

const batchSize = 500

// Apply executes a plan. The returned outcome is non-nil even on error so the
// UI can show what was done before the failure.
func Apply(ctx context.Context, src mongox.Source, p Plan, backupDir string, pr *jobs.Progress) (*Outcome, error) {
	start := time.Now()
	out := &Outcome{Planned: p.counts(), SkippedIDs: []string{}}
	sc, tc, err := compare.Resolve(src, p.Compare)
	if err != nil {
		return out, err
	}
	ts, _ := src.Session(p.Compare.Target.ConnID)
	var actions []string
	if len(p.Inserts) > 0 {
		actions = append(actions, mongox.ActionInsert)
	}
	if len(p.Replaces) > 0 {
		actions = append(actions, mongox.ActionUpdate)
	}
	if len(p.Deletes) > 0 {
		actions = append(actions, mongox.ActionRemove)
	}
	if err := ts.RequireWrite(ctx, p.Compare.Target.DB, p.Compare.Target.Coll, actions...); err != nil {
		return out, err
	}

	bk, err := backup.Create(backupDir, "sync", fmt.Sprintf("sync %s -> %s", p.Compare.Source, p.Compare.Target), p.Compare.Target.DB, p.Compare.Target.Coll)
	if err != nil {
		return out, apperr.Wrap(apperr.CodeInternal, err, "Could not create the backup; nothing was changed.")
	}
	out.BackupPath = bk.Info().Path
	defer bk.Close()
	if pr != nil {
		pr.SetTotal(int64(out.Planned.Inserts+out.Planned.Replaces+out.Planned.Deletes), "documents")
	}
	skip := func(key string) {
		if len(out.SkippedIDs) < 100 {
			id, _ := compare.ParseIDKey(key)
			out.SkippedIDs = append(out.SkippedIDs, mql.Compact(id))
		}
	}

	// Inserts: source must still hash to the previewed value; insert fails if the target gained the _id.
	for i := 0; i < len(p.Inserts); i += batchSize {
		batch := p.Inserts[i:min(i+batchSize, len(p.Inserts))]
		srcDocs, err := compare.FetchByKeys(ctx, sc, keys(batch))
		if err != nil {
			return out, err
		}
		var models []mongo.WriteModel
		for _, e := range batch {
			d, ok := srcDocs[e.Key]
			if !ok || p.Compare.HashFor(d) != e.SourceHash {
				out.Skipped.Inserts++
				skip(e.Key)
				continue
			}
			id, _ := compare.ParseIDKey(e.Key)
			if err := bk.Inserted(id); err != nil {
				return out, err
			}
			models = append(models, mongo.NewInsertOneModel().SetDocument(d))
		}
		if err := bk.Flush(); err != nil {
			return out, err
		}
		n, failed, err := bulk(ctx, tc, models)
		if err != nil {
			return out, err
		}
		out.Applied.Inserts += n
		out.Skipped.Inserts += failed
		if pr != nil {
			pr.Add(int64(len(batch)))
		}
	}

	// Replaces and deletes: target must still be the previewed version; the
	// write filter repeats that check atomically with $$ROOT equality.
	for _, kind := range []string{"replace", "delete"} {
		entries := p.Replaces
		if kind == "delete" {
			entries = p.Deletes
		}
		for i := 0; i < len(entries); i += batchSize {
			batch := entries[i:min(i+batchSize, len(entries))]
			tgtDocs, err := compare.FetchByKeys(ctx, tc, keys(batch))
			if err != nil {
				return out, err
			}
			var srcDocs map[string]bson.D
			if kind == "replace" {
				if srcDocs, err = compare.FetchByKeys(ctx, sc, keys(batch)); err != nil {
					return out, err
				}
			}
			var models []mongo.WriteModel
			for _, e := range batch {
				t, ok := tgtDocs[e.Key]
				drift := !ok || p.Compare.HashFor(t) != e.TargetHash
				var sd bson.D
				if kind == "replace" {
					sd, ok = srcDocs[e.Key]
					drift = drift || !ok || p.Compare.HashFor(sd) != e.SourceHash
				}
				if drift {
					if kind == "replace" {
						out.Skipped.Replaces++
					} else {
						out.Skipped.Deletes++
					}
					skip(e.Key)
					continue
				}
				id, _ := compare.ParseIDKey(e.Key)
				filter := unchangedFilter(id, t)
				if kind == "replace" {
					if err := bk.Replaced(t); err != nil {
						return out, err
					}
					models = append(models, mongo.NewReplaceOneModel().SetFilter(filter).SetReplacement(sd))
				} else {
					if err := bk.Deleted(t); err != nil {
						return out, err
					}
					models = append(models, mongo.NewDeleteOneModel().SetFilter(filter))
				}
			}
			if err := bk.Flush(); err != nil {
				return out, err
			}
			n, missed, err := bulk(ctx, tc, models)
			if err != nil {
				return out, err
			}
			if kind == "replace" {
				out.Applied.Replaces += n
				out.Skipped.Replaces += missed
			} else {
				out.Applied.Deletes += n
				out.Skipped.Deletes += missed
			}
			if pr != nil {
				pr.Add(int64(len(batch)))
			}
		}
	}
	if err := bk.Close(); err != nil {
		return out, err
	}
	out.MatchesPreview = out.Applied == out.Planned

	// Verify: compare again and count what is left of the kinds we synced.
	if pr != nil {
		pr.Message("Verifying")
	}
	post, err := compare.Run(ctx, src, p.Compare, nil)
	if err != nil {
		return out, fmt.Errorf("verification failed: %w", err)
	}
	if p.Options.Insert {
		out.Remaining.Inserts = int(post.OnlySource)
	}
	if p.Options.Replace {
		out.Remaining.Replaces = int(post.Changed)
	}
	if p.Options.Delete {
		out.Remaining.Deletes = int(post.OnlyTarget)
	}
	out.VerifiedEqual = out.Remaining == Counts{}
	out.PostFingerprint = post.Fingerprint
	out.DurationMs = time.Since(start).Milliseconds()
	return out, nil
}

// unchangedFilter matches the document only if it is still exactly t.
func unchangedFilter(id any, t bson.D) bson.D {
	return bson.D{
		{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}},
		{Key: "$expr", Value: bson.D{{Key: "$eq", Value: bson.A{"$$ROOT", bson.D{{Key: "$literal", Value: t}}}}}},
	}
}

func keys(es []compare.Entry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Key
	}
	return out
}

// bulk runs unordered writes and returns (applied, not applied).
func bulk(ctx context.Context, c *mongo.Collection, models []mongo.WriteModel) (int, int, error) {
	if len(models) == 0 {
		return 0, 0, nil
	}
	res, err := c.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	failed := 0
	if err != nil {
		var bwe mongo.BulkWriteException
		if !errors.As(err, &bwe) || bwe.WriteConcernError != nil {
			return 0, 0, apperr.From(err)
		}
		failed = len(bwe.WriteErrors)
	}
	applied := 0
	if res != nil {
		applied = int(res.InsertedCount + res.ModifiedCount + res.DeletedCount)
		// A replace that matched an identical document reports Modified 0; count matches.
		applied += int(res.MatchedCount - res.ModifiedCount)
	}
	notApplied := len(models) - applied
	if notApplied < failed {
		notApplied = failed
	}
	return applied, notApplied, nil
}
