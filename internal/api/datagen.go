package api

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/datagen"
	"mango3s/internal/jobs"
	"mango3s/internal/model"
	"mango3s/internal/schema"
)

// Test data generator (Explorer › collection › Generate test data…).

// TestDataPreview is what the dialog shows before generating.
type TestDataPreview struct {
	Documents  []model.Document        `json:"documents"`
	Existing   int64                   `json:"existing"` // documents already in the collection (estimate)
	Generators []datagen.GeneratorInfo `json:"generators"`
}

// TestDataRequest describes a run.
type TestDataRequest struct {
	ConnID   string `json:"connId"`
	DB       string `json:"db"`
	Coll     string `json:"coll"`
	Template string `json:"template"`
	Seed     uint64 `json:"seed"`
	Count    int    `json:"count"`
}

const maxTestDocs = 10_000_000

// TestDataTemplateFromSchema samples the collection and writes a template
// that resembles its documents (empty collections get a starter template).
func (a *App) TestDataTemplateFromSchema(connID, db, coll string) (string, error) {
	return call(a, "TestDataTemplateFromSchema", "", func(ctx context.Context) (string, error) {
		rep, err := a.schema.Analyze(ctx, schema.Request{ConnID: connID, DB: db, Coll: coll, SampleSize: 1000})
		if err != nil {
			return "", err
		}
		if rep.Sampled == 0 {
			return "{\n  name: $name,\n  email: $email,\n  age: $int(18, 90),\n  city: $city,\n  createdAt: $date(\"-1y\", \"now\")\n}", nil
		}
		return datagen.FromSchema(rep), nil
	})
}

// PreviewTestData generates 5 documents without writing anything.
func (a *App) PreviewTestData(r TestDataRequest) (TestDataPreview, error) {
	return call(a, "PreviewTestData", "", func(ctx context.Context) (TestDataPreview, error) {
		out := TestDataPreview{Documents: []model.Document{}, Generators: datagen.Generators}
		tp, err := datagen.Compile(r.Template)
		if err != nil {
			return out, err
		}
		g := tp.NewGenerator(r.Seed, time.Now())
		for i := 0; i < 5; i++ {
			out.Documents = append(out.Documents, model.NewDocument(g.Next()))
		}
		if sess, err := a.conns.Session(r.ConnID); err == nil {
			out.Existing, _ = sess.Client.Database(r.DB).Collection(r.Coll).EstimatedDocumentCount(ctx)
		}
		return out, nil
	})
}

// StartTestData writes Count generated documents as a background job.
func (a *App) StartTestData(r TestDataRequest) (jobs.Job, error) {
	sess, err := a.conns.Session(r.ConnID)
	if err != nil {
		return jobs.Job{}, a.encode("StartTestData", err)
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, a.encode("StartTestData", err)
	}
	if r.Count <= 0 || r.Count > maxTestDocs {
		return jobs.Job{}, a.encode("StartTestData", apperr.Invalid("Choose between 1 and %d documents.", maxTestDocs))
	}
	tp, err := datagen.Compile(r.Template)
	if err != nil {
		return jobs.Job{}, a.encode("StartTestData", err)
	}
	coll := sess.Client.Database(r.DB).Collection(r.Coll)
	title := fmt.Sprintf("Generate %d documents into %s.%s", r.Count, r.DB, r.Coll)
	return a.jobs.Start("datagen", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		p.SetTotal(int64(r.Count), "documents")
		g := tp.NewGenerator(r.Seed, time.Now())
		const batch = 1000
		written := 0
		for written < r.Count {
			n := min(batch, r.Count-written)
			docs := make([]any, n)
			for i := range docs {
				docs[i] = g.Next()
			}
			if _, err := coll.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false)); err != nil {
				return "", apperr.From(err)
			}
			written += n
			p.Add(int64(n))
		}
		return fmt.Sprintf("Inserted %d generated documents (seed %d)", written, r.Seed), nil
	}), nil
}
