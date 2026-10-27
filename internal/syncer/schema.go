package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/compare"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Kinds of schema changes.
const (
	CreateIndex  = "createIndex"  // index only in the source
	ReplaceIndex = "replaceIndex" // same name, different definition: drop and recreate
	DropIndex    = "dropIndex"    // index only in the target
	SetValidator = "validator"    // validation rules, level or action differ
)

// SchemaChange is one index or validation difference that can be copied
// from the source collection to the target.
type SchemaChange struct {
	ID     string `json:"id"` // "index:<name>" or "validator"
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"` // mongosh syntax
	Target string `json:"target,omitempty"`
}

// SchemaDiff lists the differences. Fingerprint covers both sides as they
// were read; ApplySchema refuses to run when it no longer matches.
type SchemaDiff struct {
	Changes     []SchemaChange `json:"changes"`
	Fingerprint string         `json:"fingerprint"`
}

// SchemaResult is what ApplySchema did.
type SchemaResult struct {
	Applied []string `json:"applied"`
	Failed  []string `json:"failed"` // "<id>: <reason>"
}

type collSchema struct {
	indexes   map[string]bson.D // by name, without "v" and "ns"; _id_ excluded
	validator bson.D            // {validator, validationLevel, validationAction}
}

func readSchema(ctx context.Context, c *mongo.Collection) (collSchema, error) {
	s := collSchema{indexes: map[string]bson.D{}}
	cur, err := c.Indexes().List(ctx)
	switch {
	case isNamespaceNotFound(err):
		// No collection yet: no indexes.
	case err != nil:
		return s, apperr.From(err)
	default:
		defer cur.Close(context.Background())
		for cur.Next(ctx) {
			var spec bson.D
			if err := cur.Decode(&spec); err != nil {
				return s, apperr.From(err)
			}
			clean := bson.D{}
			for _, e := range spec {
				if e.Key != "v" && e.Key != "ns" {
					clean = append(clean, e)
				}
			}
			name, _ := mql.Lookup(clean, "name")
			if n, ok := name.(string); ok && n != "_id_" {
				s.indexes[n] = clean
			}
		}
		if err := cur.Err(); err != nil {
			return s, apperr.From(err)
		}
	}
	specs, err := c.Database().ListCollectionSpecifications(ctx, bson.D{{Key: "name", Value: c.Name()}})
	if err != nil {
		return s, apperr.From(err)
	}
	if len(specs) == 1 {
		var opts bson.D
		if err := bson.Unmarshal(specs[0].Options, &opts); err == nil {
			for _, k := range []string{"validator", "validationLevel", "validationAction"} {
				if v, ok := mql.Lookup(opts, k); ok {
					s.validator = append(s.validator, bson.E{Key: k, Value: v})
				}
			}
		}
	}
	return s, nil
}

func isNamespaceNotFound(err error) bool {
	var ce mongo.CommandError
	return errors.As(err, &ce) && ce.Code == 26
}

func same(a, b bson.D) bool {
	return compare.HashDoc(compare.Normalize(a, nil, true)) == compare.HashDoc(compare.Normalize(b, nil, true))
}

// effective fills in the server defaults, so "no validator" equals an empty
// validator with level strict and action error.
func effective(v bson.D) bson.D {
	out := bson.D{{Key: "validator", Value: bson.D{}}, {Key: "validationLevel", Value: "strict"}, {Key: "validationAction", Value: "error"}}
	for _, e := range v {
		for i := range out {
			if out[i].Key == e.Key {
				out[i].Value = e.Value
			}
		}
	}
	return out
}

func diffSchema(s, t collSchema) SchemaDiff {
	var changes []SchemaChange
	for name, spec := range s.indexes {
		ts, ok := t.indexes[name]
		switch {
		case !ok:
			changes = append(changes, SchemaChange{ID: "index:" + name, Kind: CreateIndex, Name: name, Source: mql.Compact(spec)})
		case !same(spec, ts):
			changes = append(changes, SchemaChange{ID: "index:" + name, Kind: ReplaceIndex, Name: name, Source: mql.Compact(spec), Target: mql.Compact(ts)})
		}
	}
	for name, spec := range t.indexes {
		if _, ok := s.indexes[name]; !ok {
			changes = append(changes, SchemaChange{ID: "index:" + name, Kind: DropIndex, Name: name, Target: mql.Compact(spec)})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ID < changes[j].ID })
	if sv, tv := effective(s.validator), effective(t.validator); !same(sv, tv) {
		changes = append(changes, SchemaChange{ID: "validator", Kind: SetValidator, Source: mql.Pretty(sv), Target: mql.Pretty(tv)})
	}
	if changes == nil {
		changes = []SchemaChange{}
	}
	h := sha256.New()
	for _, side := range []collSchema{s, t} {
		names := make([]string, 0, len(side.indexes))
		for n := range side.indexes {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			sum := compare.HashDoc(compare.Normalize(side.indexes[n], nil, true))
			h.Write([]byte(n))
			h.Write(sum[:])
		}
		sum := compare.HashDoc(compare.Normalize(effective(side.validator), nil, true))
		h.Write(sum[:])
		h.Write([]byte{0})
	}
	return SchemaDiff{Changes: changes, Fingerprint: hex.EncodeToString(h.Sum(nil)[:12])}
}

// DiffSchema compares the indexes and validation rules of two collections.
func DiffSchema(ctx context.Context, src mongox.Source, o compare.Options) (SchemaDiff, error) {
	if o.Source == o.Target {
		return SchemaDiff{}, apperr.Invalid("Source and target are the same collection.")
	}
	sc, tc, err := compare.Resolve(src, o)
	if err != nil {
		return SchemaDiff{}, err
	}
	s, err := readSchema(ctx, sc)
	if err != nil {
		return SchemaDiff{}, err
	}
	t, err := readSchema(ctx, tc)
	if err != nil {
		return SchemaDiff{}, err
	}
	return diffSchema(s, t), nil
}

// ApplySchema copies the chosen changes (by ID) to the target. It re-reads
// both sides first and refuses if anything changed since the preview
// (fingerprint). Dropping or rebuilding an index needs confirm to repeat the
// target namespace.
func ApplySchema(ctx context.Context, src mongox.Source, o compare.Options, ids []string, fingerprint, confirm string) (SchemaResult, error) {
	res := SchemaResult{Applied: []string{}, Failed: []string{}}
	ts, err := src.Session(o.Target.ConnID)
	if err != nil {
		return res, err
	}
	if err := ts.RequireWritable(); err != nil {
		return res, err
	}
	diff, err := DiffSchema(ctx, src, o)
	if err != nil {
		return res, err
	}
	if diff.Fingerprint != fingerprint {
		return res, apperr.New(apperr.CodeConflict, "The indexes or validation rules changed since the preview. Compare again.")
	}
	var chosen []SchemaChange
	destructive := false
	for _, c := range diff.Changes {
		if slices.Contains(ids, c.ID) {
			chosen = append(chosen, c)
			destructive = destructive || c.Kind == DropIndex || c.Kind == ReplaceIndex
		}
	}
	if len(chosen) == 0 {
		return res, apperr.Invalid("Choose at least one change.")
	}
	if destructive && strings.TrimSpace(confirm) != o.Target.String() {
		return res, apperr.Invalid("This drops or rebuilds indexes on %s. Type %q to confirm.", o.Target, o.Target.String())
	}
	sc, tc, _ := compare.Resolve(src, o)
	s, err := readSchema(ctx, sc)
	if err != nil {
		return res, err
	}
	db := tc.Database()
	run := func(cmd bson.D) error { return db.RunCommand(ctx, cmd).Err() }
	for _, c := range chosen {
		var err error
		switch c.Kind {
		case CreateIndex:
			err = run(bson.D{{Key: "createIndexes", Value: tc.Name()}, {Key: "indexes", Value: bson.A{s.indexes[c.Name]}}})
		case ReplaceIndex:
			if err = run(bson.D{{Key: "dropIndexes", Value: tc.Name()}, {Key: "index", Value: c.Name}}); err == nil {
				err = run(bson.D{{Key: "createIndexes", Value: tc.Name()}, {Key: "indexes", Value: bson.A{s.indexes[c.Name]}}})
			}
		case DropIndex:
			err = run(bson.D{{Key: "dropIndexes", Value: tc.Name()}, {Key: "index", Value: c.Name}})
		case SetValidator:
			if err = ensureCollection(ctx, db, tc.Name()); err == nil {
				err = run(append(bson.D{{Key: "collMod", Value: tc.Name()}}, effective(s.validator)...))
			}
		}
		if err != nil {
			res.Failed = append(res.Failed, c.ID+": "+apperr.From(err).Message)
		} else {
			res.Applied = append(res.Applied, c.ID)
		}
	}
	return res, nil
}

// ensureCollection creates the collection when it does not exist (collMod
// needs one).
func ensureCollection(ctx context.Context, db *mongo.Database, name string) error {
	names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil || len(names) > 0 {
		return err
	}
	err = db.CreateCollection(ctx, name)
	var ce mongo.CommandError
	if errors.As(err, &ce) && ce.Code == 48 { // created meanwhile
		return nil
	}
	return err
}
