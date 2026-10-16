// Package transform runs a per-document transformation (reschema, masking)
// over a collection with a dry-run preview and a recoverable apply:
//
//   - copy mode writes every transformed document into a new, empty
//     collection and leaves the source untouched;
//   - in-place mode backs up each original before replacing it, and writes
//     with an optimistic filter so documents changed by someone else in the
//     meantime are skipped, not clobbered.
//
// Both modes validate the result afterwards.
package transform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/docpath"
	"mango3s/internal/jobs"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Func transforms one document. It must not modify its input and returns
// whether anything changed.
type Func func(bson.D) (bson.D, bool, error)

// Modes.
const (
	ModeCopy    = "copy"
	ModeInPlace = "inPlace"
)

// Spec says where to read and where to write.
type Spec struct {
	ConnID     string `json:"connId"`
	DB         string `json:"db"`
	Coll       string `json:"coll"`
	Filter     string `json:"filter"`
	Mode       string `json:"mode"`       // copy | inPlace
	TargetColl string `json:"targetColl"` // copy mode: new collection in the same database
	OnError    string `json:"onError"`    // skip | fail
}

// Validate checks the spec.
func (s Spec) Validate() error {
	if s.ConnID == "" || s.DB == "" || s.Coll == "" {
		return apperr.Invalid("Choose a source collection.")
	}
	switch s.Mode {
	case ModeCopy:
		if strings.TrimSpace(s.TargetColl) == "" {
			return apperr.Invalid("Copy mode needs a target collection name.")
		}
		if s.TargetColl == s.Coll {
			return apperr.Invalid("The target collection must differ from the source; use in-place mode to modify the source.")
		}
	case ModeInPlace:
	default:
		return apperr.Invalid("Mode must be copy or inPlace.")
	}
	if s.OnError != "" && s.OnError != "skip" && s.OnError != "fail" {
		return apperr.Invalid("onError must be skip or fail.")
	}
	if _, err := mql.ParseDocument(s.Filter); err != nil {
		return apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
	}
	return nil
}

// Namespace returns the collection that will be modified.
func (s Spec) Namespace() string {
	if s.Mode == ModeCopy {
		return s.DB + "." + s.TargetColl
	}
	return s.DB + "." + s.Coll
}

// Sample is one before/after pair.
type Sample struct {
	ID      string         `json:"id"`
	Before  model.Document `json:"before"`
	After   model.Document `json:"after"`
	Changed bool           `json:"changed"`
	Error   string         `json:"error,omitempty"`
}

// Preview is the dry-run result.
type Preview struct {
	Total        int64    `json:"total"`
	Scanned      int64    `json:"scanned"` // documents dry-run (all, unless Estimated)
	Changed      int64    `json:"changed"`
	Errors       int64    `json:"errors"`
	ErrorSamples []string `json:"errorSamples"`
	Samples      []Sample `json:"samples"`
	Estimated    bool     `json:"estimated"` // dry run stopped at DryRunLimit
}

// DryRunLimit bounds the documents a synchronous preview transforms.
var DryRunLimit int64 = 100_000

// Report is the outcome of an apply.
type Report struct {
	Mode          string   `json:"mode"`
	Scanned       int64    `json:"scanned"`
	Changed       int64    `json:"changed"`
	Written       int64    `json:"written"`
	Skipped       int64    `json:"skipped"` // transform errors (onError=skip)
	Drift         int64    `json:"drift"`   // in-place: changed by someone else after being read
	ErrorSamples  []string `json:"errorSamples"`
	BackupPath    string   `json:"backupPath,omitempty"`
	Validation    []Check  `json:"validation"`
	Valid         bool     `json:"valid"`
	DurationMs    int64    `json:"durationMs"`
	Target        string   `json:"target"`
	CreatedTarget bool     `json:"createdTarget"`
}

// Check is one validation result.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func source(src mongox.Source, s Spec) (*mongox.Session, *mongo.Collection, bson.D, error) {
	sess, c, err := mongox.Collection(src, s.ConnID, s.DB, s.Coll)
	if err != nil {
		return nil, nil, nil, err
	}
	f, err := mql.ParseDocument(s.Filter)
	if err != nil {
		return nil, nil, nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
	}
	return sess, c, f, nil
}

func idOf(d bson.D) any {
	v, _ := docpath.Get(d, "_id")
	return v
}

// RunPreview dry-runs fn over the matching documents (up to DryRunLimit) and
// returns n before/after samples. Nothing is written.
func RunPreview(ctx context.Context, src mongox.Source, s Spec, fn Func, n int) (Preview, error) {
	if err := s.Validate(); err != nil {
		return Preview{}, err
	}
	_, c, filter, err := source(src, s)
	if err != nil {
		return Preview{}, err
	}
	pv := Preview{Samples: []Sample{}, ErrorSamples: []string{}}
	if pv.Total, err = c.CountDocuments(ctx, filter); err != nil {
		return Preview{}, apperr.From(err)
	}
	cur, err := c.Find(ctx, filter, options.Find().SetBatchSize(1000).SetLimit(DryRunLimit))
	if err != nil {
		return Preview{}, apperr.From(err)
	}
	defer cur.Close(context.Background())
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return Preview{}, apperr.From(err)
		}
		pv.Scanned++
		out, changed, terr := fn(d)
		if terr != nil {
			pv.Errors++
			if len(pv.ErrorSamples) < 20 {
				pv.ErrorSamples = append(pv.ErrorSamples, fmt.Sprintf("%s: %v", mql.Compact(idOf(d)), terr))
			}
		} else if changed {
			pv.Changed++
		}
		if len(pv.Samples) < n {
			smp := Sample{ID: mql.Compact(idOf(d)), Before: model.NewDocument(d), Changed: changed}
			if terr != nil {
				smp.Error = terr.Error()
				smp.After = smp.Before
			} else {
				smp.After = model.NewDocument(out)
			}
			pv.Samples = append(pv.Samples, smp)
		}
	}
	if err := cur.Err(); err != nil {
		return Preview{}, apperr.From(err)
	}
	pv.Estimated = pv.Scanned < pv.Total
	return pv, nil
}

// Apply runs fn over the collection according to the spec. backupDir is
// used for in-place mode; operation labels the backup ("reschema", "mask").
func Apply(ctx context.Context, src mongox.Source, s Spec, fn Func, operation, backupDir string, p *jobs.Progress) (*Report, error) {
	start := time.Now()
	if err := s.Validate(); err != nil {
		return nil, err
	}
	sess, c, filter, err := source(src, s)
	if err != nil {
		return nil, err
	}
	rep := &Report{Mode: s.Mode, Target: s.Namespace(), ErrorSamples: []string{}, Validation: []Check{}}
	total, err := c.CountDocuments(ctx, filter)
	if err != nil {
		return nil, apperr.From(err)
	}
	if p != nil {
		p.SetTotal(total, "documents")
	}

	var target *mongo.Collection
	var bk *backup.Writer
	if s.Mode == ModeCopy {
		if err := sess.RequireWrite(ctx, s.DB, s.TargetColl, mongox.ActionInsert); err != nil {
			return nil, err
		}
		target = sess.Client.Database(s.DB).Collection(s.TargetColl)
		names, err := sess.Client.Database(s.DB).ListCollectionNames(ctx, bson.D{{Key: "name", Value: s.TargetColl}})
		if err != nil {
			return nil, apperr.From(err)
		}
		if len(names) > 0 {
			n, err := target.EstimatedDocumentCount(ctx)
			if err != nil {
				return nil, apperr.From(err)
			}
			if n > 0 {
				return nil, apperr.New(apperr.CodeConflict, "Target %s already contains %d documents; choose a new or empty collection.", s.Namespace(), n)
			}
		} else {
			rep.CreatedTarget = true
		}
	} else {
		if err := sess.RequireWrite(ctx, s.DB, s.Coll, mongox.ActionUpdate); err != nil {
			return nil, err
		}
		target = c
		if bk, err = backup.Create(backupDir, operation, operation+" "+s.DB+"."+s.Coll, s.DB, s.Coll); err != nil {
			return nil, apperr.Wrap(apperr.CodeInternal, err, "Could not create the backup; nothing was changed.")
		}
		rep.BackupPath = bk.Info().Path
		defer bk.Close()
	}

	// Remember a sample of written documents for validation.
	written := map[string]bson.D{}
	remember := func(d bson.D) {
		if len(written) < 100 {
			written[mql.Compact(idOf(d))] = d
		}
	}

	var batch []mongo.WriteModel
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if bk != nil {
			if err := bk.Flush(); err != nil {
				return err
			}
		}
		res, err := target.BulkWrite(ctx, batch, options.BulkWrite().SetOrdered(false))
		n := len(batch)
		batch = batch[:0]
		if err != nil {
			var bwe mongo.BulkWriteException
			if !errors.As(err, &bwe) || bwe.WriteConcernError != nil {
				return apperr.From(err)
			}
			return apperr.New(apperr.CodeMongo, "%d write(s) failed: %s", len(bwe.WriteErrors), bwe.WriteErrors[0].Message)
		}
		if s.Mode == ModeCopy {
			rep.Written += res.InsertedCount
		} else {
			rep.Written += res.MatchedCount
			rep.Drift += int64(n) - res.MatchedCount
		}
		return nil
	}

	cur, err := c.Find(ctx, filter, options.Find().SetBatchSize(1000))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return rep, apperr.From(err)
		}
		rep.Scanned++
		if p != nil {
			p.Add(1)
		}
		out, changed, terr := fn(d)
		if terr != nil {
			if len(rep.ErrorSamples) < 20 {
				rep.ErrorSamples = append(rep.ErrorSamples, fmt.Sprintf("%s: %v", mql.Compact(idOf(d)), terr))
			}
			if s.OnError == "fail" {
				return rep, apperr.Invalid("Stopped at document %s: %v", mql.Compact(idOf(d)), terr)
			}
			rep.Skipped++
			continue
		}
		if changed {
			rep.Changed++
		}
		switch {
		case s.Mode == ModeCopy:
			batch = append(batch, mongo.NewInsertOneModel().SetDocument(out))
			remember(out)
		case changed:
			if err := bk.Replaced(d); err != nil {
				return rep, err
			}
			filter := bson.D{
				{Key: "_id", Value: bson.D{{Key: "$eq", Value: idOf(d)}}},
				{Key: "$expr", Value: bson.D{{Key: "$eq", Value: bson.A{"$$ROOT", bson.D{{Key: "$literal", Value: d}}}}}},
			}
			batch = append(batch, mongo.NewReplaceOneModel().SetFilter(filter).SetReplacement(out))
			remember(out)
		}
		if len(batch) >= 500 {
			if err := flush(); err != nil {
				return rep, err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return rep, apperr.From(err)
	}
	if err := flush(); err != nil {
		return rep, err
	}
	if bk != nil {
		if err := bk.Close(); err != nil {
			return rep, err
		}
	}

	if p != nil {
		p.Message("Validating")
	}
	if err := validate(ctx, target, s, rep, written); err != nil {
		return rep, err
	}
	rep.DurationMs = time.Since(start).Milliseconds()
	return rep, nil
}

func validate(ctx context.Context, target *mongo.Collection, s Spec, rep *Report, written map[string]bson.D) error {
	add := func(name string, ok bool, format string, args ...any) {
		rep.Validation = append(rep.Validation, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
	}
	if s.Mode == ModeCopy {
		n, err := target.CountDocuments(ctx, bson.D{})
		if err != nil {
			return apperr.From(err)
		}
		want := rep.Scanned - rep.Skipped
		add("Document count", n == want, "target has %d documents, expected %d (%d scanned, %d skipped)", n, want, rep.Scanned, rep.Skipped)
	} else {
		add("No concurrent changes", rep.Drift == 0, "%d document(s) changed by someone else were left untouched", rep.Drift)
		add("Backup written", rep.BackupPath != "", "%s", rep.BackupPath)
	}
	// Spot-check: the stored documents equal what was written.
	mismatch := 0
	for _, want := range written {
		var got bson.D
		err := target.FindOne(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: idOf(want)}}}}).Decode(&got)
		if err != nil || !mql.Equal(got, want) {
			mismatch++
		}
	}
	// In place, a sampled document that drifted was (correctly) not written.
	add("Spot check", int64(mismatch) <= rep.Drift, "%d of %d sampled documents match what was written", len(written)-mismatch, len(written))
	rep.Valid = true
	for _, c := range rep.Validation {
		rep.Valid = rep.Valid && c.OK
	}
	return nil
}
