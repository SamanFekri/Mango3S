// Package compare compares collections (and databases) that may live on
// different connections. Documents are matched by _id, normalized (ignored
// fields removed, optionally key order canonicalized) and hashed, so results
// are repeatable: equivalent inputs always give the same counts, the same
// sorted samples and the same fingerprint.
package compare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/docpath"
	"mango3s/internal/jobs"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Endpoint names a collection on a connection.
type Endpoint struct {
	ConnID string `json:"connId"`
	DB     string `json:"db"`
	Coll   string `json:"coll"`
}

func (e Endpoint) String() string { return e.DB + "." + e.Coll }

// Options configures a collection comparison.
type Options struct {
	Source           Endpoint `json:"source"`
	Target           Endpoint `json:"target"`
	Filter           string   `json:"filter"`           // applied to both sides
	IgnoreFields     []string `json:"ignoreFields"`     // dotted paths excluded from comparison
	IgnoreFieldOrder bool     `json:"ignoreFieldOrder"` // {a,b} equals {b,a}
	SampleSize       int      `json:"sampleSize"`       // differences returned with field detail
	// MatchKey pairs documents by these fields (dotted paths) instead of
	// _id, e.g. ["email"] or ["tenant", "orderNo"]. The key must be unique on
	// each side; a missing field counts as null. _id is then ignored in the
	// comparison, since matched documents may have different _ids.
	MatchKey []string `json:"matchKey,omitempty"`
}

// byKey reports whether documents are matched by a custom key.
func (o Options) byKey() bool { return len(o.MatchKey) > 0 }

// ignored is IgnoreFields, plus _id when matching by a custom key.
func (o Options) ignored() []string {
	if !o.byKey() {
		return o.IgnoreFields
	}
	return append(append([]string{}, o.IgnoreFields...), "_id")
}

// keyOf returns a document's match key: its canonical _id key, or the
// canonical form of its MatchKey values.
func (o Options) keyOf(d bson.D) (string, bool) {
	if !o.byKey() {
		id, ok := lookupID(d)
		if !ok {
			return "", false
		}
		return IDKey(id), true
	}
	vals := make(bson.A, len(o.MatchKey))
	for i, f := range o.MatchKey {
		v, ok := docpath.Get(d, strings.TrimSpace(f))
		if !ok {
			v = nil
		}
		vals[i] = v
	}
	return IDKey(vals), true
}

// Kinds of entries.
const (
	KindChanged    = "changed"
	KindOnlySource = "onlySource"
	KindOnlyTarget = "onlyTarget"
)

// Hash is a digest of a normalized document.
type Hash [16]byte

// Entry is one differing document, identified by its canonical match key
// (the _id key unless Options.MatchKey is set). SourceID and TargetID are the
// _id keys of the document on each side ("" where it is missing).
type Entry struct {
	Key        string `json:"key"`
	Kind       string `json:"kind"`
	SourceID   string `json:"-"`
	TargetID   string `json:"-"`
	SourceHash Hash   `json:"-"`
	TargetHash Hash   `json:"-"`
}

// FieldChange is a difference at one field path.
type FieldChange struct {
	Path   string `json:"path"`
	Change string `json:"change"` // added (only in source) | removed (only in target) | changed
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
}

// DocDiff describes one differing document.
type DocDiff struct {
	ID     string        `json:"id"` // shell syntax
	Kind   string        `json:"kind"`
	Fields []FieldChange `json:"fields"`
}

// Result is the outcome of a comparison.
type Result struct {
	Options     Options   `json:"options"`
	SourceCount int64     `json:"sourceCount"`
	TargetCount int64     `json:"targetCount"`
	Identical   int64     `json:"identical"`
	Changed     int64     `json:"changed"`
	OnlySource  int64     `json:"onlySource"`
	OnlyTarget  int64     `json:"onlyTarget"`
	Samples     []DocDiff `json:"samples"`
	Fingerprint string    `json:"fingerprint"`
	DurationMs  int64     `json:"durationMs"`
	Entries     []Entry   `json:"-"` // every difference, sorted by key (used by sync)
}

// Equal reports whether the two sides have no differences.
func (r *Result) Equal() bool { return r.Changed == 0 && r.OnlySource == 0 && r.OnlyTarget == 0 }

// MaxDocs bounds the number of source documents held in memory (one hash per _id).
var MaxDocs int64 = 5_000_000

// IDKey renders an _id as canonical Extended JSON, a total, type-exact key.
func IDKey(id any) string {
	data, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: id}}, true, false)
	if err != nil {
		return fmt.Sprint(id)
	}
	return string(data)
}

// ParseIDKey reverses IDKey.
func ParseIDKey(key string) (any, error) {
	var d bson.D
	if err := bson.UnmarshalExtJSON([]byte(key), true, &d); err != nil || len(d) != 1 {
		return nil, fmt.Errorf("invalid id key %q", key)
	}
	return d[0].Value, nil
}

// Normalize removes ignored paths and, if requested, sorts keys recursively.
// The input is not modified.
func Normalize(d bson.D, ignore []string, sortKeys bool) bson.D {
	ign := map[string]bool{}
	for _, p := range ignore {
		if p = strings.TrimSpace(p); p != "" {
			ign[p] = true
		}
	}
	return normalize(d, "", ign, sortKeys)
}

func normalize(d bson.D, prefix string, ign map[string]bool, sortKeys bool) bson.D {
	out := make(bson.D, 0, len(d))
	for _, e := range d {
		path := e.Key
		if prefix != "" {
			path = prefix + "." + e.Key
		}
		if ign[path] {
			continue
		}
		out = append(out, bson.E{Key: e.Key, Value: normalizeValue(e.Value, path, ign, sortKeys)})
	}
	if sortKeys {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	}
	return out
}

func normalizeValue(v any, path string, ign map[string]bool, sortKeys bool) any {
	switch t := v.(type) {
	case bson.D:
		return normalize(t, path, ign, sortKeys)
	case bson.A:
		a := make(bson.A, len(t))
		for i, el := range t {
			// Paths inside arrays of documents use the array's path (like queries do).
			a[i] = normalizeValue(el, path, ign, sortKeys)
		}
		return a
	}
	return v
}

// HashDoc digests a normalized document.
func HashDoc(d bson.D) Hash {
	data, err := bson.Marshal(d)
	if err != nil {
		data = []byte(mql.Compact(d))
	}
	sum := sha256.Sum256(data)
	var h Hash
	copy(h[:], sum[:16])
	return h
}

// HashFor normalizes and hashes a document with the comparison options.
func (o Options) HashFor(d bson.D) Hash {
	return HashDoc(Normalize(d, o.ignored(), o.IgnoreFieldOrder))
}

func lookupID(d bson.D) (any, bool) {
	for _, e := range d {
		if e.Key == "_id" {
			return e.Value, true
		}
	}
	return nil, false
}

// fingerprint accumulates per-document digests order-independently (sum mod 2^256).
type fingerprint struct{ sum *big.Int }

var mod = new(big.Int).Lsh(big.NewInt(1), 256)

func newFingerprint() *fingerprint { return &fingerprint{sum: new(big.Int)} }

func (f *fingerprint) add(key string, src, tgt Hash, kind string) {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write([]byte(key))
	h.Write(src[:])
	h.Write(tgt[:])
	f.sum.Add(f.sum, new(big.Int).SetBytes(h.Sum(nil)))
	f.sum.Mod(f.sum, mod)
}

func (f *fingerprint) String() string {
	b := f.sum.FillBytes(make([]byte, 32))
	return hex.EncodeToString(b[:12])
}

// Resolve returns the two collections for the options.
func Resolve(src mongox.Source, o Options) (*mongo.Collection, *mongo.Collection, error) {
	_, sc, err := mongox.Collection(src, o.Source.ConnID, o.Source.DB, o.Source.Coll)
	if err != nil {
		return nil, nil, fmt.Errorf("source: %w", err)
	}
	_, tc, err := mongox.Collection(src, o.Target.ConnID, o.Target.DB, o.Target.Coll)
	if err != nil {
		return nil, nil, fmt.Errorf("target: %w", err)
	}
	return sc, tc, nil
}

// Run compares the two collections. p may be nil.
func Run(ctx context.Context, src mongox.Source, o Options, p *jobs.Progress) (*Result, error) {
	start := time.Now()
	if o.Source == o.Target {
		return nil, apperr.Invalid("Source and target are the same collection.")
	}
	sc, tc, err := Resolve(src, o)
	if err != nil {
		return nil, err
	}
	filter, err := mql.ParseDocument(o.Filter)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
	}
	if o.SampleSize <= 0 {
		o.SampleSize = 50
	}
	r := &Result{Options: o, Samples: []DocDiff{}}
	srcTotal, err := sc.CountDocuments(ctx, filter)
	if err != nil {
		return nil, apperr.From(err)
	}
	if srcTotal > MaxDocs {
		return nil, apperr.Invalid("The source has %d documents; comparisons are limited to %d. Narrow it with a filter.", srcTotal, MaxDocs)
	}
	tgtTotal, err := tc.CountDocuments(ctx, filter)
	if err != nil {
		return nil, apperr.From(err)
	}
	if p != nil {
		p.SetTotal(srcTotal+tgtTotal, "documents")
	}

	type seen struct {
		hash Hash
		id   string
	}
	hashes := make(map[string]seen, srcTotal)
	err = scan(ctx, sc, filter, func(d bson.D) error {
		id, ok := lookupID(d)
		if !ok {
			return nil
		}
		key, _ := o.keyOf(d)
		if _, dup := hashes[key]; dup && o.byKey() {
			return duplicateKey("source", o, key)
		}
		hashes[key] = seen{hash: o.HashFor(d), id: IDKey(id)}
		r.SourceCount++
		if p != nil {
			p.Add(1)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	fp := newFingerprint()
	var zero Hash
	targetKeys := map[string]bool{}
	err = scan(ctx, tc, filter, func(d bson.D) error {
		id, ok := lookupID(d)
		if !ok {
			return nil
		}
		key, _ := o.keyOf(d)
		if o.byKey() {
			if targetKeys[key] {
				return duplicateKey("target", o, key)
			}
			targetKeys[key] = true
		}
		tid := IDKey(id)
		th := o.HashFor(d)
		r.TargetCount++
		if p != nil {
			p.Add(1)
		}
		sh, inSource := hashes[key]
		switch {
		case !inSource:
			r.OnlyTarget++
			r.Entries = append(r.Entries, Entry{Key: key, Kind: KindOnlyTarget, TargetID: tid, TargetHash: th})
			fp.add(key, zero, th, KindOnlyTarget)
		case sh.hash == th:
			r.Identical++
			fp.add(key, sh.hash, th, "same")
			delete(hashes, key)
		default:
			r.Changed++
			r.Entries = append(r.Entries, Entry{Key: key, Kind: KindChanged, SourceID: sh.id, TargetID: tid, SourceHash: sh.hash, TargetHash: th})
			fp.add(key, sh.hash, th, KindChanged)
			delete(hashes, key)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for key, sh := range hashes {
		r.OnlySource++
		r.Entries = append(r.Entries, Entry{Key: key, Kind: KindOnlySource, SourceID: sh.id, SourceHash: sh.hash})
		fp.add(key, sh.hash, zero, KindOnlySource)
	}
	sort.Slice(r.Entries, func(i, j int) bool { return r.Entries[i].Key < r.Entries[j].Key })
	r.Fingerprint = fp.String()

	if p != nil {
		p.Message("Collecting field-level details")
	}
	if err := r.loadSamples(ctx, sc, tc); err != nil {
		return nil, err
	}
	r.DurationMs = time.Since(start).Milliseconds()
	return r, nil
}

func duplicateKey(side string, o Options, key string) error {
	v, _ := ParseIDKey(key)
	return apperr.Invalid("Several documents in the %s have the match key %s = %s. The match key must be unique on both sides; add fields to it or filter the documents.",
		side, strings.Join(o.MatchKey, " + "), mql.Compact(v))
}

func scan(ctx context.Context, c *mongo.Collection, filter bson.D, f func(bson.D) error) error {
	cur, err := c.Find(ctx, filter, options.Find().SetBatchSize(1000))
	if err != nil {
		return apperr.From(err)
	}
	defer cur.Close(context.Background())
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return apperr.From(err)
		}
		if err := f(d); err != nil {
			return err
		}
	}
	return apperr.Check(cur.Err())
}

// FetchByKeys loads documents by _id keys, keyed by IDKey.
func FetchByKeys(ctx context.Context, c *mongo.Collection, keys []string) (map[string]bson.D, error) {
	ids := make(bson.A, 0, len(keys))
	for _, k := range keys {
		id, err := ParseIDKey(k)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	out := map[string]bson.D{}
	if len(ids) == 0 {
		return out, nil
	}
	err := scan(ctx, c, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}, func(d bson.D) error {
		id, _ := lookupID(d)
		out[IDKey(id)] = d
		return nil
	})
	return out, err
}

func (r *Result) loadSamples(ctx context.Context, sc, tc *mongo.Collection) error {
	n := min(len(r.Entries), r.Options.SampleSize)
	if n == 0 {
		return nil
	}
	var srcIDs, tgtIDs []string
	for _, e := range r.Entries[:n] {
		if e.SourceID != "" {
			srcIDs = append(srcIDs, e.SourceID)
		}
		if e.TargetID != "" {
			tgtIDs = append(tgtIDs, e.TargetID)
		}
	}
	srcDocs, err := FetchByKeys(ctx, sc, srcIDs)
	if err != nil {
		return err
	}
	tgtDocs, err := FetchByKeys(ctx, tc, tgtIDs)
	if err != nil {
		return err
	}
	ign := r.Options.ignored()
	for _, e := range r.Entries[:n] {
		id, _ := ParseIDKey(e.Key)
		dd := DocDiff{ID: mql.Compact(id), Kind: e.Kind, Fields: []FieldChange{}}
		s, t := srcDocs[e.SourceID], tgtDocs[e.TargetID]
		if s != nil && t != nil {
			dd.Fields = Diff(Normalize(s, ign, r.Options.IgnoreFieldOrder), Normalize(t, ign, r.Options.IgnoreFieldOrder), "")
		}
		r.Samples = append(r.Samples, dd)
	}
	return nil
}

// Diff lists field-level differences between two documents. Sub-documents
// are compared recursively; arrays and scalars as whole values. A pure
// field-order difference is reported on the document's own path.
func Diff(source, target bson.D, prefix string) []FieldChange {
	out := []FieldChange{}
	path := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + "." + k
	}
	tmap := map[string]any{}
	for _, e := range target {
		tmap[e.Key] = e.Value
	}
	smap := map[string]bool{}
	for _, e := range source {
		smap[e.Key] = true
		tv, ok := tmap[e.Key]
		if !ok {
			out = append(out, FieldChange{Path: path(e.Key), Change: "added", Source: clip(mql.Compact(e.Value))})
			continue
		}
		sd, sIsDoc := e.Value.(bson.D)
		td, tIsDoc := tv.(bson.D)
		if sIsDoc && tIsDoc {
			out = append(out, Diff(sd, td, path(e.Key))...)
			continue
		}
		if !mql.Equal(e.Value, tv) {
			out = append(out, FieldChange{Path: path(e.Key), Change: "changed", Source: clip(mql.Compact(e.Value)), Target: clip(mql.Compact(tv))})
		}
	}
	for _, e := range target {
		if !smap[e.Key] {
			out = append(out, FieldChange{Path: path(e.Key), Change: "removed", Target: clip(mql.Compact(e.Value))})
		}
	}
	if len(out) == 0 && !mql.Equal(source, target) {
		p := prefix
		if p == "" {
			p = "(document)"
		}
		out = append(out, FieldChange{Path: p, Change: "changed", Source: "field order differs", Target: "field order differs"})
	}
	return out
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:199] + "…"
	}
	return s
}

// ---- database comparison ----------------------------------------------------

// DBOptions compares two databases.
type DBOptions struct {
	SourceConnID string `json:"sourceConnId"`
	SourceDB     string `json:"sourceDb"`
	TargetConnID string `json:"targetConnId"`
	TargetDB     string `json:"targetDb"`
	Deep         bool   `json:"deep"` // compare documents of common collections too
}

// CollectionRow is the per-collection result of a database comparison.
type CollectionRow struct {
	Name        string   `json:"name"`
	InSource    bool     `json:"inSource"`
	InTarget    bool     `json:"inTarget"`
	SourceCount int64    `json:"sourceCount"`
	TargetCount int64    `json:"targetCount"`
	IndexDiffs  []string `json:"indexDiffs"`
	Compared    bool     `json:"compared"`
	Identical   int64    `json:"identical"`
	Changed     int64    `json:"changed"`
	OnlySource  int64    `json:"onlySource"`
	OnlyTarget  int64    `json:"onlyTarget"`
	Fingerprint string   `json:"fingerprint,omitempty"`
}

// DBResult is the outcome of a database comparison.
type DBResult struct {
	Options     DBOptions       `json:"options"`
	Collections []CollectionRow `json:"collections"`
	DurationMs  int64           `json:"durationMs"`
}

// RunDB compares the collections of two databases.
func RunDB(ctx context.Context, src mongox.Source, o DBOptions, p *jobs.Progress) (*DBResult, error) {
	start := time.Now()
	ss, err := src.Session(o.SourceConnID)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	ts, err := src.Session(o.TargetConnID)
	if err != nil {
		return nil, fmt.Errorf("target: %w", err)
	}
	if o.SourceConnID == o.TargetConnID && o.SourceDB == o.TargetDB {
		return nil, apperr.Invalid("Source and target are the same database.")
	}
	sdb, tdb := ss.Client.Database(o.SourceDB), ts.Client.Database(o.TargetDB)
	userColls := bson.D{{Key: "name", Value: bson.D{{Key: "$not", Value: bson.Regex{Pattern: "^system\\."}}}}, {Key: "type", Value: "collection"}}
	sNames, err := sdb.ListCollectionNames(ctx, userColls)
	if err != nil {
		return nil, apperr.From(err)
	}
	tNames, err := tdb.ListCollectionNames(ctx, userColls)
	if err != nil {
		return nil, apperr.From(err)
	}
	rows := map[string]*CollectionRow{}
	for _, n := range sNames {
		rows[n] = &CollectionRow{Name: n, InSource: true, IndexDiffs: []string{}}
	}
	for _, n := range tNames {
		if r, ok := rows[n]; ok {
			r.InTarget = true
		} else {
			rows[n] = &CollectionRow{Name: n, InTarget: true, IndexDiffs: []string{}}
		}
	}
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	if p != nil {
		p.SetTotal(int64(len(names)), "collections")
	}
	res := &DBResult{Options: o, Collections: make([]CollectionRow, 0, len(names))}
	for _, n := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := rows[n]
		if p != nil {
			p.Message("Comparing %s", n)
		}
		if r.InSource {
			if r.SourceCount, err = sdb.Collection(n).CountDocuments(ctx, bson.D{}); err != nil {
				return nil, apperr.From(err)
			}
		}
		if r.InTarget {
			if r.TargetCount, err = tdb.Collection(n).CountDocuments(ctx, bson.D{}); err != nil {
				return nil, apperr.From(err)
			}
		}
		if r.InSource && r.InTarget {
			r.IndexDiffs, err = indexDiffs(ctx, sdb.Collection(n), tdb.Collection(n))
			if err != nil {
				return nil, err
			}
			if o.Deep {
				cr, err := Run(ctx, src, Options{
					Source: Endpoint{ConnID: o.SourceConnID, DB: o.SourceDB, Coll: n},
					Target: Endpoint{ConnID: o.TargetConnID, DB: o.TargetDB, Coll: n}, SampleSize: 1,
				}, nil)
				if err != nil {
					return nil, err
				}
				r.Compared = true
				r.Identical, r.Changed, r.OnlySource, r.OnlyTarget, r.Fingerprint = cr.Identical, cr.Changed, cr.OnlySource, cr.OnlyTarget, cr.Fingerprint
			}
		}
		res.Collections = append(res.Collections, *r)
		if p != nil {
			p.Add(1)
		}
	}
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func indexKeys(ctx context.Context, c *mongo.Collection) (map[string]string, error) {
	cur, err := c.Indexes().List(ctx)
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	out := map[string]string{}
	for cur.Next(ctx) {
		var spec bson.D
		if err := cur.Decode(&spec); err != nil {
			return nil, apperr.From(err)
		}
		name, _ := mql.Lookup(spec, "name")
		key, _ := mql.Lookup(spec, "key")
		unique, _ := mql.Lookup(spec, "unique")
		desc := mql.Compact(key)
		if unique == true {
			desc += " unique"
		}
		out[fmt.Sprint(name)] = desc
	}
	return out, apperr.Check(cur.Err())
}

func indexDiffs(ctx context.Context, s, t *mongo.Collection) ([]string, error) {
	si, err := indexKeys(ctx, s)
	if err != nil {
		return nil, err
	}
	ti, err := indexKeys(ctx, t)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for n, d := range si {
		td, ok := ti[n]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("index %s %s only in source", n, d))
		case td != d:
			out = append(out, fmt.Sprintf("index %s differs: %s vs %s", n, d, td))
		}
	}
	for n, d := range ti {
		if _, ok := si[n]; !ok {
			out = append(out, fmt.Sprintf("index %s %s only in target", n, d))
		}
	}
	sort.Strings(out)
	return out, nil
}

// ---- service -------------------------------------------------------------------

// Service runs comparisons as background jobs and keeps their results.
type Service struct {
	src  mongox.Source
	jobs *jobs.Manager
	mu   sync.Mutex
	res  map[string]any
	keys []string
}

// New returns a compare service.
func New(src mongox.Source, jm *jobs.Manager) *Service {
	return &Service{src: src, jobs: jm, res: map[string]any{}}
}

func (s *Service) store(id string, v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.res[id] = v
	s.keys = append(s.keys, id)
	for len(s.keys) > 20 {
		delete(s.res, s.keys[0])
		s.keys = s.keys[1:]
	}
}

// StartCollections queues a collection comparison.
func (s *Service) StartCollections(o Options) (jobs.Job, error) {
	if _, _, err := Resolve(s.src, o); err != nil {
		return jobs.Job{}, err
	}
	if _, err := mql.ParseDocument(o.Filter); err != nil {
		return jobs.Job{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Filter: "+err.Error())
	}
	title := fmt.Sprintf("Compare %s ↔ %s", o.Source, o.Target)
	return s.jobs.Start("compare", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		r, err := Run(ctx, s.src, o, p)
		if err != nil {
			return "", err
		}
		s.store(p.ID(), r)
		if r.Equal() {
			return fmt.Sprintf("Identical: %d documents", r.Identical), nil
		}
		return fmt.Sprintf("%d identical, %d changed, %d only in source, %d only in target", r.Identical, r.Changed, r.OnlySource, r.OnlyTarget), nil
	}), nil
}

// StartDatabases queues a database comparison.
func (s *Service) StartDatabases(o DBOptions) (jobs.Job, error) {
	title := fmt.Sprintf("Compare database %s ↔ %s", o.SourceDB, o.TargetDB)
	return s.jobs.Start("compare", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		r, err := RunDB(ctx, s.src, o, p)
		if err != nil {
			return "", err
		}
		s.store(p.ID(), r)
		diff := 0
		for _, c := range r.Collections {
			if !c.InSource || !c.InTarget || c.SourceCount != c.TargetCount || len(c.IndexDiffs) > 0 || c.Changed+c.OnlySource+c.OnlyTarget > 0 {
				diff++
			}
		}
		return fmt.Sprintf("%d collections, %d with differences", len(r.Collections), diff), nil
	}), nil
}

// CollectionResult returns a stored collection comparison.
func (s *Service) CollectionResult(jobID string) (*Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.res[jobID].(*Result); ok {
		return r, nil
	}
	return nil, apperr.New(apperr.CodeNotFound, "Comparison result not found; run the comparison again.")
}

// DatabaseResult returns a stored database comparison.
func (s *Service) DatabaseResult(jobID string) (*DBResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.res[jobID].(*DBResult); ok {
		return r, nil
	}
	return nil, apperr.New(apperr.CodeNotFound, "Comparison result not found; run the comparison again.")
}
