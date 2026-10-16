package api

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/documents"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// BulkWriteRequest is Studio 3T's Update dialog (Ctrl+U) and Remove dialog
// (F8): change or delete either the selected documents or every document
// matching a query.
type BulkWriteRequest struct {
	ConnID string   `json:"connId"`
	DB     string   `json:"db"`
	Coll   string   `json:"coll"`
	Kind   string   `json:"kind"`   // update | delete
	Scope  string   `json:"scope"`  // selected | query
	IDs    []string `json:"ids"`    // selected: _id values in shell syntax
	Filter string   `json:"filter"` // query: filter in shell syntax
	Update string   `json:"update"` // update: an update document ($set, …) or pipeline
}

// BulkPreview says what a bulk write would touch.
type BulkPreview struct {
	Affected    int64    `json:"affected"`
	SampleIDs   []string `json:"sampleIds"`
	ConfirmText string   `json:"confirmText"` // type this to run it
	Shell       string   `json:"shell"`       // the equivalent mongosh statement
}

// BulkResult is the outcome of a bulk write.
type BulkResult struct {
	Matched    int64  `json:"matched"`
	Modified   int64  `json:"modified"`
	Deleted    int64  `json:"deleted"`
	BackupPath string `json:"backupPath"`
}

func (r BulkWriteRequest) parse() (filter bson.D, update any, shell string, err error) {
	switch r.Scope {
	case "selected":
		if len(r.IDs) == 0 {
			return nil, nil, "", apperr.Invalid("No documents are selected.")
		}
		if len(r.IDs) > documents.MaxBatch {
			return nil, nil, "", apperr.Invalid("At most %d documents can be selected.", documents.MaxBatch)
		}
		ids := bson.A{}
		for _, id := range r.IDs {
			v, err := documents.ParseID(id)
			if err != nil {
				return nil, nil, "", err
			}
			ids = append(ids, v)
		}
		filter = documents.IDsFilter(ids)
	case "query":
		if filter, err = mql.ParseDocument(orEmpty(r.Filter)); err != nil {
			return nil, nil, "", apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
		}
	default:
		return nil, nil, "", apperr.Invalid("Choose the selected documents or all documents matching the query.")
	}
	method := "deleteMany"
	switch r.Kind {
	case "update":
		method = "updateMany"
		if strings.TrimSpace(r.Update) == "" {
			return nil, nil, "", apperr.Invalid("Enter the update, e.g. { $set: { status: \"done\" } }.")
		}
		if update, err = mql.ParseValue(r.Update); err != nil {
			return nil, nil, "", apperr.Wrap(apperr.CodeInvalidInput, err, "Update: "+err.Error())
		}
		switch u := update.(type) {
		case bson.D:
			if len(u) == 0 {
				return nil, nil, "", apperr.Invalid("The update is empty.")
			}
			for _, e := range u {
				if !strings.HasPrefix(e.Key, "$") {
					return nil, nil, "", apperr.Invalid("The update must use operators such as $set or $unset (field %q has none). To replace whole documents, edit them as JSON.", e.Key)
				}
			}
		case bson.A:
			// An aggregation pipeline update.
		default:
			return nil, nil, "", apperr.Invalid("The update must be a document of operators or a pipeline.")
		}
	case "delete":
	default:
		return nil, nil, "", apperr.Invalid("Unknown bulk operation %q.", r.Kind)
	}
	shell = "db.getCollection(" + mql.Quote(r.Coll) + ")." + method + "(" + mql.Compact(filter)
	if update != nil {
		shell += ", " + mql.Compact(update)
	}
	return filter, update, shell + ")", nil
}

func orEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

// PreviewBulkWrite counts what an Update or Remove dialog would touch.
func (a *App) PreviewBulkWrite(r BulkWriteRequest) (BulkPreview, error) {
	return call(a, "PreviewBulkWrite", "", func(ctx context.Context) (BulkPreview, error) {
		filter, _, shell, err := r.parse()
		if err != nil {
			return BulkPreview{}, err
		}
		_, c, err := mongox.Collection(a.conns, r.ConnID, r.DB, r.Coll)
		if err != nil {
			return BulkPreview{}, err
		}
		n, err := c.CountDocuments(ctx, filter)
		if err != nil {
			return BulkPreview{}, apperr.From(err)
		}
		p := BulkPreview{Affected: n, SampleIDs: []string{}, ConfirmText: r.DB + "." + r.Coll, Shell: shell}
		cur, err := c.Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}).SetLimit(5))
		if err == nil {
			defer cur.Close(context.Background())
			for cur.Next(ctx) {
				var d bson.D
				if cur.Decode(&d) == nil {
					if v, ok := mql.Lookup(d, "_id"); ok {
						p.SampleIDs = append(p.SampleIDs, mql.Compact(v))
					}
				}
			}
		}
		return p, nil
	})
}

// ExecuteBulkWrite runs a previewed Update or Remove dialog. It needs the
// target typed as confirmation and refuses to run if the number of matching
// documents changed since the preview. Every touched document is backed up
// first (Library → Backups can restore them).
func (a *App) ExecuteBulkWrite(r BulkWriteRequest, confirm string, expected int64) (BulkResult, error) {
	res, err := call(a, "ExecuteBulkWrite", "", func(ctx context.Context) (BulkResult, error) {
		filter, update, shell, err := r.parse()
		if err != nil {
			return BulkResult{}, err
		}
		if strings.TrimSpace(confirm) != r.DB+"."+r.Coll {
			return BulkResult{}, apperr.Invalid("Type %s.%s to confirm.", r.DB, r.Coll)
		}
		sess, c, err := mongox.Collection(a.conns, r.ConnID, r.DB, r.Coll)
		if err != nil {
			return BulkResult{}, err
		}
		action := mongox.ActionUpdate
		if r.Kind == "delete" {
			action = mongox.ActionRemove
		}
		if err := sess.RequireWrite(ctx, r.DB, r.Coll, action, mongox.ActionFind); err != nil {
			return BulkResult{}, err
		}
		n, err := c.CountDocuments(ctx, filter)
		if err != nil {
			return BulkResult{}, apperr.From(err)
		}
		if n != expected {
			return BulkResult{}, apperr.New(apperr.CodeConflict, "%d documents match now, but the preview showed %d. Preview again before running it.", n, expected)
		}
		op := "update-dialog"
		if r.Kind == "delete" {
			op = "remove-dialog"
		}
		bk, err := backup.Create(backup.Dir(a.store.Dir()), op, shell, r.DB, r.Coll)
		if err != nil {
			return BulkResult{}, apperr.Wrap(apperr.CodeInternal, err, "Could not create the backup; nothing was changed.")
		}
		defer bk.Close()
		m, err := backup.WriteManyWithBackup(ctx, c, filter, update, bk, 0)
		out := BulkResult{Matched: m.Matched, Modified: m.Modified, Deleted: m.Deleted, BackupPath: bk.Info().Path}
		if err != nil {
			return out, err
		}
		return out, bk.Close()
	})
	if err == nil {
		a.log.Info("bulk write", "kind", r.Kind, "scope", r.Scope, "db", r.DB, "coll", r.Coll, "matched", res.Matched, "deleted", res.Deleted)
	}
	return res, err
}
