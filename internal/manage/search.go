package manage

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// Search index types.
const (
	SearchTypeSearch = "search"       // Atlas Search (full text, $search)
	SearchTypeVector = "vectorSearch" // Atlas Vector Search ($vectorSearch)
)

// SearchIndex is an Atlas Search or Vector Search index. Building one is
// asynchronous: Status goes PENDING → BUILDING → READY (or FAILED), and
// Queryable says whether queries can use it yet.
type SearchIndex struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Queryable  bool   `json:"queryable"`
	Definition string `json:"definition"` // mongosh syntax
	Message    string `json:"message,omitempty"`
}

// SearchIndexList is a collection's search indexes. Supported is false on
// servers without Atlas Search (a plain mongod); Reason says why.
type SearchIndexList struct {
	Supported bool          `json:"supported"`
	Reason    string        `json:"reason,omitempty"`
	Indexes   []SearchIndex `json:"indexes"`
}

// noSearch reports whether err means the server has no search support, as
// opposed to a real failure.
func noSearch(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) && (ce.Code == 40324 || ce.Code == 31082 || ce.Code == 6047401 || ce.Code == 115 || ce.Code == 59) {
		return true // unknown stage, search not enabled, Atlas only, not supported, no such command
	}
	s := err.Error()
	return strings.Contains(s, "Unrecognized pipeline stage name: '$listSearchIndexes'") || strings.Contains(s, "SearchNotEnabled")
}

const noSearchReason = "This server has no Atlas Search. Search and Vector Search indexes need MongoDB Atlas, or a local Atlas deployment (the mongodb/mongodb-atlas-local Docker image or atlas deployments setup)."

// ListSearchIndexes lists a collection's search indexes.
func ListSearchIndexes(ctx context.Context, db *mongo.Database, coll string) (SearchIndexList, error) {
	cur, err := db.Collection(coll).SearchIndexes().List(ctx, options.SearchIndexes())
	if err != nil {
		if noSearch(err) {
			return SearchIndexList{Reason: noSearchReason, Indexes: []SearchIndex{}}, nil
		}
		return SearchIndexList{}, apperr.From(err)
	}
	defer cur.Close(context.Background())
	out := SearchIndexList{Supported: true, Indexes: []SearchIndex{}}
	for cur.Next(ctx) {
		var raw struct {
			Name             string `bson:"name"`
			Type             string `bson:"type"`
			Status           string `bson:"status"`
			Queryable        bool   `bson:"queryable"`
			LatestDefinition bson.D `bson:"latestDefinition"`
			Message          string `bson:"message"`
		}
		if err := cur.Decode(&raw); err != nil {
			return SearchIndexList{}, apperr.From(err)
		}
		if raw.Type == "" {
			raw.Type = SearchTypeSearch
		}
		out.Indexes = append(out.Indexes, SearchIndex{Name: raw.Name, Type: raw.Type, Status: raw.Status, Queryable: raw.Queryable,
			Definition: mql.Pretty(raw.LatestDefinition), Message: raw.Message})
	}
	if err := cur.Err(); err != nil {
		if noSearch(err) {
			return SearchIndexList{Reason: noSearchReason, Indexes: []SearchIndex{}}, nil
		}
		return SearchIndexList{}, apperr.From(err)
	}
	return out, nil
}

// SearchIndexInput creates or updates a search index. Definition is a
// document in mongosh syntax, e.g. { mappings: { dynamic: true } }.
type SearchIndexInput struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Definition string `json:"definition"`
}

func searchDefinition(text string) (bson.D, error) {
	v, err := mql.ParseValue(text)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Definition: "+err.Error())
	}
	d, ok := v.(bson.D)
	if !ok || len(d) == 0 {
		return nil, apperr.Invalid("The definition must be a document, e.g. { mappings: { dynamic: true } }.")
	}
	return d, nil
}

// CreateSearchIndex starts building a search index and returns its name.
func CreateSearchIndex(ctx context.Context, db *mongo.Database, coll string, in SearchIndexInput) (string, error) {
	def, err := searchDefinition(in.Definition)
	if err != nil {
		return "", err
	}
	if in.Type == "" {
		in.Type = SearchTypeSearch
	}
	if in.Type != SearchTypeSearch && in.Type != SearchTypeVector {
		return "", apperr.Invalid("Unknown search index type %q.", in.Type)
	}
	opts := options.SearchIndexes().SetType(in.Type)
	if name := strings.TrimSpace(in.Name); name != "" {
		opts.SetName(name)
	}
	name, err := db.Collection(coll).SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{Definition: def, Options: opts})
	if err != nil {
		if noSearch(err) {
			return "", apperr.New(apperr.CodeUnsupported, "%s", noSearchReason)
		}
		return "", apperr.From(err)
	}
	return name, nil
}

// UpdateSearchIndex replaces a search index's definition; the server rebuilds
// it in the background and keeps serving the old version meanwhile.
func UpdateSearchIndex(ctx context.Context, db *mongo.Database, coll, name, definition string) error {
	def, err := searchDefinition(definition)
	if err != nil {
		return err
	}
	return apperr.Check(db.Collection(coll).SearchIndexes().UpdateOne(ctx, name, def))
}

// DropSearchIndex deletes a search index.
func DropSearchIndex(ctx context.Context, db *mongo.Database, coll, name string) error {
	return apperr.Check(db.Collection(coll).SearchIndexes().DropOne(ctx, name))
}
