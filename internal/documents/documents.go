// Package documents implements single-document CRUD addressed by _id.
package documents

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// MaxBatch caps how many documents a single insert or delete may touch.
const MaxBatch = 10000

// Service is the document CRUD API. Document IDs are _id values in shell
// syntax as produced by model.Document.ID.
type Service interface {
	Get(ctx context.Context, connID, db, coll, id string) (model.Document, error)
	Insert(ctx context.Context, connID, db, coll, text string) ([]string, error)
	Replace(ctx context.Context, connID, db, coll, id, text string) (model.Document, error)
	Delete(ctx context.Context, connID, db, coll string, ids []string) (int64, error)
	// GetField and UpdateField support in-place cell editing with
	// field-level $set / $unset (see fields.go).
	GetField(ctx context.Context, connID, db, coll, id string, path []string) (FieldValue, error)
	UpdateField(ctx context.Context, u FieldUpdate) (FieldResult, error)
}

type service struct{ src mongox.Source }

// New returns a document service backed by src.
func New(src mongox.Source) Service { return &service{src: src} }

// ParseID parses a shell-syntax _id value.
func ParseID(id string) (any, error) {
	if id == "" {
		return nil, apperr.Invalid("This document has no _id and cannot be modified individually.")
	}
	v, err := mql.ParseValue(id)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Invalid _id.")
	}
	return v, nil
}

func (s *service) Get(ctx context.Context, connID, db, coll, id string) (model.Document, error) {
	_, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return model.Document{}, err
	}
	idv, err := ParseID(id)
	if err != nil {
		return model.Document{}, err
	}
	var d bson.D
	if err := c.FindOne(ctx, IDFilter(idv)).Decode(&d); err != nil {
		if err == mongo.ErrNoDocuments {
			return model.Document{}, apperr.New(apperr.CodeNotFound, "Document not found; it may have been deleted.")
		}
		return model.Document{}, apperr.From(err)
	}
	return model.NewDocument(d), nil
}

func (s *service) Insert(ctx context.Context, connID, db, coll, text string) ([]string, error) {
	sess, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return nil, err
	}
	if err := sess.RequireWritable(); err != nil {
		return nil, err
	}
	docs, err := mql.ParseDocuments(text)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, err.Error())
	}
	if len(docs) == 0 {
		return nil, apperr.Invalid("Nothing to insert.")
	}
	if len(docs) > MaxBatch {
		return nil, apperr.Invalid("At most %d documents can be inserted at once.", MaxBatch)
	}
	ids := make([]string, 0, len(docs))
	if len(docs) == 1 {
		res, err := c.InsertOne(ctx, docs[0])
		if err != nil {
			return nil, apperr.From(err)
		}
		return append(ids, mql.Compact(res.InsertedID)), nil
	}
	in := make([]any, len(docs))
	for i, d := range docs {
		in[i] = d
	}
	res, err := c.InsertMany(ctx, in)
	if err != nil {
		return nil, apperr.From(err)
	}
	for _, id := range res.InsertedIDs {
		ids = append(ids, mql.Compact(id))
	}
	return ids, nil
}

func (s *service) Replace(ctx context.Context, connID, db, coll, id, text string) (model.Document, error) {
	sess, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return model.Document{}, err
	}
	if err := sess.RequireWritable(); err != nil {
		return model.Document{}, err
	}
	idv, err := ParseID(id)
	if err != nil {
		return model.Document{}, err
	}
	doc, err := mql.ParseDocument(text)
	if err != nil {
		return model.Document{}, apperr.Wrap(apperr.CodeInvalidInput, err, err.Error())
	}
	doc, err = WithID(doc, idv)
	if err != nil {
		return model.Document{}, err
	}
	res, err := c.ReplaceOne(ctx, IDFilter(idv), doc)
	if err != nil {
		return model.Document{}, apperr.From(err)
	}
	if res.MatchedCount == 0 {
		return model.Document{}, apperr.New(apperr.CodeNotFound, "Document not found; it may have been deleted by someone else.")
	}
	return model.NewDocument(doc), nil
}

// IDFilter matches exactly one _id. $eq is used so that an _id which is itself
// a regular expression is compared literally rather than pattern-matched.
func IDFilter(id any) bson.D {
	return bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}}}
}

// IDsFilter matches any of ids literally.
func IDsFilter(ids bson.A) bson.D {
	for _, v := range ids {
		if _, isRegex := v.(bson.Regex); isRegex {
			or := make(bson.A, len(ids))
			for i, id := range ids {
				or[i] = IDFilter(id)
			}
			return bson.D{{Key: "$or", Value: or}}
		}
	}
	return bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}
}

// WithID ensures doc carries exactly the given _id, placing it first.
// Changing _id is rejected because MongoDB treats _id as immutable.
func WithID(doc bson.D, id any) (bson.D, error) {
	out := bson.D{{Key: "_id", Value: id}}
	for _, e := range doc {
		if e.Key == "_id" {
			if !mql.Equal(e.Value, id) {
				return nil, apperr.Invalid("_id cannot be changed (was %s, now %s). Insert a new document instead.",
					mql.Compact(id), mql.Compact(e.Value))
			}
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *service) Delete(ctx context.Context, connID, db, coll string, ids []string) (int64, error) {
	sess, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return 0, err
	}
	if err := sess.RequireWritable(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, apperr.Invalid("No documents selected.")
	}
	if len(ids) > MaxBatch {
		return 0, apperr.Invalid("At most %d documents can be deleted at once.", MaxBatch)
	}
	vals := make(bson.A, 0, len(ids))
	for _, id := range ids {
		v, err := ParseID(id)
		if err != nil {
			return 0, err
		}
		vals = append(vals, v)
	}
	res, err := c.DeleteMany(ctx, IDsFilter(vals))
	if err != nil {
		return 0, apperr.From(err)
	}
	return res.DeletedCount, nil
}
