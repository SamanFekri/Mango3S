package dump

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/manage"
	"mango3s/internal/mql"
)

// CopyOptions copies collections between databases, on the same or another
// server. Documents stream as raw BSON, so every type is preserved.
type CopyOptions struct {
	SrcDB       string   `json:"srcDb"`
	Collections []string `json:"collections"` // empty: every collection and view of SrcDB
	DstDB       string   `json:"dstDb"`
	DstColl     string   `json:"dstColl"` // only with exactly one collection: its new name
	Mode        string   `json:"mode"`    // insert | upsert | drop
	Filter      string   `json:"filter"`  // canonical Extended JSON; applied to every collection
	SkipIndexes bool     `json:"skipIndexes"`
	SkipOptions bool     `json:"skipOptions"`
	Views       bool     `json:"views"`
	// ShardCollections shards each copied collection on the target cluster
	// with its source shard key (both must be sharded clusters).
	ShardCollections bool `json:"shardCollections"`
}

// CopyReport lists the per-collection outcome.
type CopyReport struct {
	Results []Result `json:"results"`
}

// Copy copies collections (with options and indexes) and view definitions.
// Copying a collection onto itself is refused.
func Copy(ctx context.Context, src, dst *mongo.Client, sameServer bool, o CopyOptions, log Logger) (CopyReport, error) {
	rep := CopyReport{Results: []Result{}}
	switch o.Mode {
	case ModeInsert, ModeUpsert, ModeDrop:
	default:
		return rep, fmt.Errorf("unknown copy mode %q", o.Mode)
	}
	if o.DstDB == "" {
		o.DstDB = o.SrcDB
	}
	if o.DstColl != "" && len(o.Collections) != 1 {
		return rep, errors.New("a new collection name can only be given when copying one collection")
	}
	var filter bson.D
	if strings.TrimSpace(o.Filter) != "" {
		if err := bson.UnmarshalExtJSON([]byte(o.Filter), false, &filter); err != nil {
			return rep, fmt.Errorf("filter is not valid Extended JSON: %w", err)
		}
	}
	all, err := Plan(ctx, src, Selection{Databases: []string{o.SrcDB}, Views: o.Views || len(o.Collections) > 0})
	if err != nil {
		return rep, err
	}
	// Exact names (not patterns: collection names may contain "*").
	want := map[string]bool{}
	for _, c := range o.Collections {
		want[c] = true
	}
	var nss []Namespace
	for _, ns := range all {
		if len(want) == 0 || want[ns.Coll] {
			nss = append(nss, ns)
		}
	}
	if len(o.Collections) > 0 && len(nss) != len(o.Collections) {
		found := map[string]bool{}
		for _, ns := range nss {
			found[ns.Coll] = true
		}
		for _, c := range o.Collections {
			if !found[c] {
				return rep, fmt.Errorf("%s.%s does not exist", o.SrcDB, c)
			}
		}
	}
	if len(nss) == 0 {
		return rep, errors.New("nothing to copy")
	}
	// Collections first, then views (a view may read a copied collection).
	var ordered []Namespace
	for _, ns := range nss {
		if ns.Type != "view" {
			ordered = append(ordered, ns)
		}
	}
	for _, ns := range nss {
		if ns.Type == "view" {
			ordered = append(ordered, ns)
		}
	}
	var total int64
	for _, ns := range ordered {
		if ns.Type != "view" {
			if n, err := src.Database(ns.DB).Collection(ns.Coll).EstimatedDocumentCount(ctx); err == nil {
				total += n
			}
		}
	}
	log.SetTotal(total, "docs")
	failed := 0
	for _, ns := range ordered {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		to := ns.Coll
		if o.DstColl != "" {
			to = o.DstColl
		}
		if sameServer && o.DstDB == ns.DB && to == ns.Coll {
			return rep, fmt.Errorf("%s.%s can't be copied onto itself; choose another database or name", ns.DB, ns.Coll)
		}
		res := Result{Source: ns.DB + "." + ns.Coll, Target: o.DstDB + "." + to}
		err := copyOne(ctx, src, dst, o, filter, ns, to, &res, log)
		if err == nil && res.Failed > 0 {
			err = fmt.Errorf("%d documents were rejected by the target server", res.Failed)
		}
		if err != nil {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			res.Error = err.Error()
			failed++
			log.Log("FAILED %s → %s: %v", res.Source, res.Target, err)
		} else if ns.Type == "view" {
			log.Log("%s: view copied", res.Target)
		} else {
			log.Log("%s → %s: %d copied, %d replaced, %d already present, %d indexes", res.Source, res.Target, res.Inserted, res.Replaced, res.Duplicates, res.Indexes)
		}
		rep.Results = append(rep.Results, res)
	}
	if failed > 0 {
		return rep, fmt.Errorf("%d of %d namespaces failed; see the job log", failed, len(ordered))
	}
	return rep, nil
}

func copyOne(ctx context.Context, src, dst *mongo.Client, o CopyOptions, filter bson.D, ns Namespace, to string, res *Result, log Logger) error {
	sdb, ddb := src.Database(ns.DB), dst.Database(o.DstDB)
	info, err := collectionInfo(ctx, sdb, ns.Coll)
	if err != nil {
		return err
	}
	isView := ns.Type == "view"
	if err := prepareTarget(ctx, ddb, to, o.Mode, isView, info.Options, o.SkipOptions, log); err != nil {
		return err
	}
	if isView {
		return nil
	}
	if o.ShardCollections {
		key, unique, _, _, err := manage.ShardKeyOf(ctx, src, ns.DB, ns.Coll)
		if err != nil {
			return fmt.Errorf("read the source shard key: %w", err)
		}
		if key != nil {
			if err := shardTarget(ctx, dst, o.DstDB, to, mql.Compact(key), unique, log); err != nil {
				return err
			}
		}
	}
	var specs []bson.D
	if !o.SkipIndexes {
		cur, err := sdb.Collection(ns.Coll).Indexes().List(ctx)
		if err != nil {
			return fmt.Errorf("list indexes: %w", err)
		}
		if err := cur.All(ctx, &specs); err != nil {
			return fmt.Errorf("list indexes: %w", err)
		}
	}
	f := filter
	if f == nil {
		f = bson.D{}
	}
	cur, err := sdb.Collection(ns.Coll).Find(ctx, f, options.Find().SetBatchSize(1000))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer cur.Close(context.Background())
	ro := RestoreOptions{Mode: o.Mode}
	if ro.Mode == ModeDrop {
		ro.Mode = ModeInsert
	}
	target := ddb.Collection(to)
	var batch []bson.Raw
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := writeBatch(ctx, target, ro, batch, res)
		log.Add(int64(len(batch)))
		batch, size = batch[:0], 0
		return err
	}
	for cur.Next(ctx) {
		res.Read++
		batch = append(batch, append(bson.Raw(nil), cur.Current...))
		size += len(cur.Current)
		if len(batch) >= batchDocs || size >= batchBytes {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if err := flush(); err != nil {
		return err
	}
	if !o.SkipIndexes {
		n, err := restoreIndexes(ctx, ddb, to, specs)
		res.Indexes = n
		if err != nil {
			return err
		}
	}
	return nil
}
