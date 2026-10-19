// Package explorer lists databases and collections and inspects collections.
package explorer

import (
	"context"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// DatabaseInfo describes a database.
type DatabaseInfo struct {
	Name       string `json:"name"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Empty      bool   `json:"empty"`
}

// CollectionInfo describes a collection or view.
type CollectionInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // collection | view | timeseries
	ReadOnly bool   `json:"readOnly"`
	System   bool   `json:"system"` // name starts with "system."
}

// IndexInfo describes an index.
type IndexInfo struct {
	Name   string `json:"name"`
	Keys   string `json:"keys"`
	Unique bool   `json:"unique"`
}

// CollectionStats summarizes a collection.
type CollectionStats struct {
	Count          int64       `json:"count"`
	Size           int64       `json:"size"`
	AvgObjSize     int64       `json:"avgObjSize"`
	StorageSize    int64       `json:"storageSize"`
	TotalIndexSize int64       `json:"totalIndexSize"`
	Indexes        []IndexInfo `json:"indexes"`
}

// Service is the explorer API.
type Service interface {
	ListDatabases(ctx context.Context, connID string) ([]DatabaseInfo, error)
	ListCollections(ctx context.Context, connID, db string) ([]CollectionInfo, error)
	CollectionStats(ctx context.Context, connID, db, coll string) (CollectionStats, error)
	SampleFields(ctx context.Context, connID, db, coll string, sampleSize int) ([]string, error)
	CreateCollection(ctx context.Context, connID, db, coll string) error
	DropCollection(ctx context.Context, connID, db, coll string) error
	DropDatabase(ctx context.Context, connID, db string) error
}

type service struct{ src mongox.Source }

// New returns an explorer backed by src.
func New(src mongox.Source) Service { return &service{src: src} }

func (s *service) ListDatabases(ctx context.Context, connID string) ([]DatabaseInfo, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return nil, err
	}
	res, err := sess.Client.ListDatabases(ctx, bson.D{}, options.ListDatabases().SetAuthorizedDatabases(true))
	if err != nil {
		return nil, apperr.From(err)
	}
	out := make([]DatabaseInfo, 0, len(res.Databases))
	for _, d := range res.Databases {
		out = append(out, DatabaseInfo{Name: d.Name, SizeOnDisk: d.SizeOnDisk, Empty: d.Empty})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *service) ListCollections(ctx context.Context, connID, db string) ([]CollectionInfo, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return nil, err
	}
	if db == "" {
		return nil, apperr.Invalid("Database name is required.")
	}
	cur, err := sess.Client.Database(db).ListCollections(ctx, bson.D{}, options.ListCollections().SetAuthorizedCollections(true))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	var specs []struct {
		Name string `bson:"name"`
		Type string `bson:"type"`
		Info struct {
			ReadOnly bool `bson:"readOnly"`
		} `bson:"info"`
	}
	if err := cur.All(ctx, &specs); err != nil {
		return nil, apperr.From(err)
	}
	out := make([]CollectionInfo, 0, len(specs))
	for _, c := range specs {
		t := c.Type
		if t == "" {
			t = "collection"
		}
		// System collections (system.views, system.profile, system.js,
		// system.buckets.*, admin's system.users, …) are listed separately
		// in the explorer.
		out = append(out, CollectionInfo{Name: c.Name, Type: t, ReadOnly: c.Info.ReadOnly, System: strings.HasPrefix(c.Name, "system.")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *service) CollectionStats(ctx context.Context, connID, db, coll string) (CollectionStats, error) {
	_, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return CollectionStats{}, err
	}
	var st CollectionStats
	// $collStats is available on all supported servers; it fails on views, in
	// which case we fall back to a plain count.
	cur, err := c.Aggregate(ctx, mongo.Pipeline{{{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{}}}}}})
	if err == nil {
		defer cur.Close(context.Background())
		var rows []struct {
			Storage struct {
				Count          int64 `bson:"count"`
				Size           int64 `bson:"size"`
				AvgObjSize     int64 `bson:"avgObjSize"`
				StorageSize    int64 `bson:"storageSize"`
				TotalIndexSize int64 `bson:"totalIndexSize"`
			} `bson:"storageStats"`
		}
		if cur.All(ctx, &rows) == nil {
			// Sharded collections return one row per shard.
			for _, r := range rows {
				st.Count += r.Storage.Count
				st.Size += r.Storage.Size
				st.StorageSize += r.Storage.StorageSize
				st.TotalIndexSize += r.Storage.TotalIndexSize
			}
			if st.Count > 0 {
				st.AvgObjSize = st.Size / st.Count
			}
		}
	} else {
		n, cerr := c.EstimatedDocumentCount(ctx)
		if cerr != nil {
			n, cerr = c.CountDocuments(ctx, bson.D{})
		}
		if cerr != nil {
			return CollectionStats{}, apperr.From(cerr)
		}
		st.Count = n
	}

	st.Indexes = []IndexInfo{}
	icur, err := c.Indexes().List(ctx)
	if err == nil {
		defer icur.Close(context.Background())
		var specs []bson.D
		if icur.All(ctx, &specs) == nil {
			for _, spec := range specs {
				ii := IndexInfo{}
				for _, e := range spec {
					switch e.Key {
					case "name":
						ii.Name, _ = e.Value.(string)
					case "key":
						ii.Keys = mql.Compact(e.Value)
					case "unique":
						ii.Unique, _ = e.Value.(bool)
					}
				}
				st.Indexes = append(st.Indexes, ii)
			}
		}
	}
	return st, nil
}

func (s *service) SampleFields(ctx context.Context, connID, db, coll string, sampleSize int) ([]string, error) {
	_, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return nil, err
	}
	if sampleSize <= 0 || sampleSize > 1000 {
		sampleSize = 100
	}
	cur, err := c.Aggregate(ctx, mongo.Pipeline{{{Key: "$sample", Value: bson.D{{Key: "size", Value: sampleSize}}}}})
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	seen := map[string]bool{}
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			continue
		}
		CollectPaths(d, "", seen, 0)
	}
	if err := cur.Err(); err != nil {
		return nil, apperr.From(err)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// CollectPaths records dotted field paths of d (and nested documents, also
// inside arrays) into seen, up to a fixed depth.
func CollectPaths(d bson.D, prefix string, seen map[string]bool, depth int) {
	if depth > 8 {
		return
	}
	for _, e := range d {
		path := e.Key
		if prefix != "" {
			path = prefix + "." + e.Key
		}
		seen[path] = true
		switch v := e.Value.(type) {
		case bson.D:
			CollectPaths(v, path, seen, depth+1)
		case bson.A:
			for _, el := range v {
				if sub, ok := el.(bson.D); ok {
					CollectPaths(sub, path, seen, depth+1)
				}
			}
		}
	}
}

func (s *service) CreateCollection(ctx context.Context, connID, db, coll string) error {
	sess, err := s.src.Session(connID)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	if err := ValidateName(db, coll); err != nil {
		return err
	}
	return apperr.Check(sess.Client.Database(db).CreateCollection(ctx, coll))
}

func (s *service) DropCollection(ctx context.Context, connID, db, coll string) error {
	sess, c, err := mongox.Collection(s.src, connID, db, coll)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	return apperr.Check(c.Drop(ctx))
}

func (s *service) DropDatabase(ctx context.Context, connID, db string) error {
	sess, err := s.src.Session(connID)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	if db == "" {
		return apperr.Invalid("Database name is required.")
	}
	if db == "admin" || db == "local" || db == "config" {
		return apperr.Invalid("Refusing to drop the system database %q.", db)
	}
	return apperr.Check(sess.Client.Database(db).Drop(ctx))
}

// ValidateName applies MongoDB's naming rules for databases and collections.
func ValidateName(db, coll string) error {
	if db == "" || len(db) > 63 || strings.ContainsAny(db, `/\. "$*<>:|?`) {
		return apperr.Invalid("Invalid database name %q.", db)
	}
	if coll == "" || strings.HasPrefix(coll, "system.") || strings.ContainsAny(coll, "$\x00") {
		return apperr.Invalid("Invalid collection name %q.", coll)
	}
	return nil
}
