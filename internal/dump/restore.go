package dump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Restore modes.
const (
	ModeInsert = "insert" // add documents; ones whose _id exists are skipped and counted
	ModeUpsert = "upsert" // replace documents with the same _id, insert the rest
	ModeDrop   = "drop"   // drop each target collection first (destructive)
)

// Entry is one namespace found in a backup folder.
type Entry struct {
	Namespace
	DataFile     string `json:"-"`
	MetadataFile string `json:"-"`
	Expected     int64  `json:"expected"` // from the manifest; -1 when unknown
}

// Inspection describes a backup folder before it is restored.
type Inspection struct {
	Dir      string    `json:"dir"`
	Manifest *Manifest `json:"manifest,omitempty"` // nil for folders written by mongodump
	Entries  []Entry   `json:"entries"`
	Warnings []string  `json:"warnings"`
}

// Inspect reads a backup folder (Mango3S or mongodump layout) without
// touching any database.
func Inspect(dir string) (Inspection, error) {
	in := Inspection{Dir: dir, Entries: []Entry{}, Warnings: []string{}}
	st, err := os.Stat(dir)
	if err != nil {
		return in, err
	}
	if !st.IsDir() {
		return in, fmt.Errorf("%s is a file; built-in restore reads backup folders (use the MongoDB Database Tools for .archive files)", filepath.Base(dir))
	}
	expected := map[string]Namespace{}
	if b, err := os.ReadFile(filepath.Join(dir, ManifestName)); err == nil {
		var m Manifest
		if err := json.Unmarshal(b, &m); err != nil {
			in.Warnings = append(in.Warnings, ManifestName+" is unreadable, so document counts cannot be checked: "+err.Error())
		} else {
			in.Manifest = &m
			for _, ns := range m.Namespaces {
				expected[ns.DB+"."+ns.Coll] = ns
				if ns.Error != "" {
					in.Warnings = append(in.Warnings, fmt.Sprintf("%s.%s failed when the backup was made (%s) and may be incomplete.", ns.DB, ns.Coll, ns.Error))
				}
			}
		}
	}
	dbDirs, err := os.ReadDir(dir)
	if err != nil {
		return in, err
	}
	for _, d := range dbDirs {
		if !d.IsDir() {
			continue
		}
		db := unescapeName(d.Name())
		files, err := os.ReadDir(filepath.Join(dir, d.Name()))
		if err != nil {
			return in, err
		}
		byColl := map[string]*Entry{}
		get := func(name string) *Entry {
			if e := byColl[name]; e != nil {
				return e
			}
			e := &Entry{Namespace: Namespace{DB: db, Coll: name, Type: "collection"}, Expected: -1}
			byColl[name] = e
			return e
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			n := f.Name()
			gz := strings.HasSuffix(n, ".gz")
			base := strings.TrimSuffix(n, ".gz")
			p := filepath.Join(dir, d.Name(), n)
			switch {
			case strings.HasSuffix(base, ".metadata.json"):
				e := get(unescapeName(strings.TrimSuffix(base, ".metadata.json")))
				e.MetadataFile = p
				e.Gzip = e.Gzip || gz
			case strings.HasSuffix(base, ".bson"):
				e := get(unescapeName(strings.TrimSuffix(base, ".bson")))
				e.DataFile = p
				e.Gzip = e.Gzip || gz
				if info, err := f.Info(); err == nil {
					e.Bytes = info.Size()
				}
			}
		}
		for _, e := range byColl {
			if strings.HasPrefix(e.Coll, "system.buckets.") {
				// mongodump writes time series data as system.buckets.<name>;
				// the built-in restore inserts measurements, not buckets.
				in.Warnings = append(in.Warnings, fmt.Sprintf("%s.%s holds raw time series buckets from mongodump; restore it with mongorestore.", e.DB, e.Coll))
				continue
			}
			if e.MetadataFile != "" {
				md, err := readMetadata(e.MetadataFile)
				if err != nil {
					in.Warnings = append(in.Warnings, fmt.Sprintf("%s.%s: metadata unreadable (%v); indexes and options cannot be restored.", e.DB, e.Coll, err))
				} else {
					if md.Type != "" {
						e.Type = md.Type
					}
					if lookup(md.Options, "viewOn") != nil {
						e.Type = "view"
					}
					e.Indexes = len(md.Indexes)
				}
			} else {
				in.Warnings = append(in.Warnings, fmt.Sprintf("%s.%s has no .metadata.json: indexes and collection options will not be restored.", e.DB, e.Coll))
			}
			if e.DataFile == "" && e.Type != "view" {
				in.Warnings = append(in.Warnings, fmt.Sprintf("%s.%s has metadata but no .bson data file.", e.DB, e.Coll))
			}
			if ns, ok := expected[e.DB+"."+e.Coll]; ok {
				e.Expected = ns.Count
				e.Count = ns.Count
			}
			in.Entries = append(in.Entries, *e)
		}
	}
	for k := range expected {
		found := false
		for _, e := range in.Entries {
			if e.DB+"."+e.Coll == k {
				found = true
				break
			}
		}
		if !found && expected[k].Error == "" {
			in.Warnings = append(in.Warnings, k+" is listed in the manifest but its files are missing.")
		}
	}
	sort.Slice(in.Entries, func(i, j int) bool {
		a, b := in.Entries[i], in.Entries[j]
		if a.DB != b.DB {
			return a.DB < b.DB
		}
		// Collections before views: a view may be defined on a restored collection.
		if (a.Type == "view") != (b.Type == "view") {
			return b.Type == "view"
		}
		return a.Coll < b.Coll
	})
	if len(in.Entries) == 0 {
		return in, fmt.Errorf("%s contains no backup (expected <database>/<collection>.bson files)", dir)
	}
	return in, nil
}

func readMetadata(path string) (Metadata, error) {
	r, err := openFile(path)
	if err != nil {
		return Metadata{}, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		return Metadata{}, err
	}
	var md Metadata
	if err := bson.UnmarshalExtJSON(b, false, &md); err != nil {
		return Metadata{}, err
	}
	return md, nil
}

func lookup(d bson.D, key string) any {
	for _, e := range d {
		if e.Key == key {
			return e.Value
		}
	}
	return nil
}

// Rename maps source namespaces to targets; "*" captures a run of
// characters and is substituted in order ("sales.*" → "sales_copy.*").
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// apply returns the renamed namespace, or ok=false if From doesn't match.
func (r Rename) apply(ns string) (string, bool) {
	parts := strings.Split(r.From, "*")
	if len(parts) == 1 {
		return r.To, ns == r.From
	}
	if !strings.HasPrefix(ns, parts[0]) {
		return "", false
	}
	rest := ns[len(parts[0]):]
	var caps []string
	for i := 1; i < len(parts); i++ {
		var k int
		if i == len(parts)-1 {
			if !strings.HasSuffix(rest, parts[i]) {
				return "", false
			}
			k = len(rest) - len(parts[i])
		} else if k = strings.Index(rest, parts[i]); k < 0 {
			return "", false
		}
		caps = append(caps, rest[:k])
		rest = rest[k+len(parts[i]):]
	}
	out := r.To
	for _, c := range caps {
		if !strings.Contains(out, "*") {
			break
		}
		out = strings.Replace(out, "*", c, 1)
	}
	return out, true
}

// RestoreOptions controls a restore.
type RestoreOptions struct {
	Dir            string    `json:"dir"`
	Mode           string    `json:"mode"` // insert | upsert | drop
	Selection      Selection `json:"selection"`
	Renames        []Rename  `json:"renames"`
	SkipIndexes    bool      `json:"skipIndexes"`
	SkipOptions    bool      `json:"skipOptions"`    // don't create collections with saved options (validators, collation, capped…)
	StopOnError    bool      `json:"stopOnError"`    // stop at the first failed collection
	BypassValidate bool      `json:"bypassValidate"` // bypassDocumentValidation
}

// Target returns the namespace an entry restores into.
func (o RestoreOptions) Target(db, coll string) (string, string) {
	ns := db + "." + coll
	for _, r := range o.Renames {
		if out, ok := r.apply(ns); ok {
			if i := strings.IndexByte(out, '.'); i > 0 {
				return out[:i], out[i+1:]
			}
		}
	}
	return db, coll
}

// Selected returns the entries a restore would process.
func (o RestoreOptions) Selected(in Inspection) []Entry {
	var out []Entry
	dbs := map[string]bool{}
	for _, d := range o.Selection.Databases {
		dbs[d] = true
	}
	for _, e := range in.Entries {
		if len(dbs) > 0 && !dbs[e.DB] {
			continue
		}
		if !o.Selection.Match(e.DB, e.Coll) {
			continue
		}
		if e.Type == "view" && !o.Selection.Views {
			continue
		}
		if strings.HasPrefix(e.Coll, "system.") && !o.Selection.IncludeSystem {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Result is the outcome for one restored namespace.
type Result struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Read       int64  `json:"read"`
	Inserted   int64  `json:"inserted"`
	Replaced   int64  `json:"replaced"`
	Duplicates int64  `json:"duplicates"` // insert mode: _id already existed
	Failed     int64  `json:"failed"`
	Indexes    int    `json:"indexes"`
	Error      string `json:"error,omitempty"`
}

// RestoreReport summarizes a restore.
type RestoreReport struct {
	Results  []Result `json:"results"`
	Warnings []string `json:"warnings"`
}

// Restore loads the selected namespaces of a backup folder into cl.
func Restore(ctx context.Context, cl *mongo.Client, o RestoreOptions, log Logger) (RestoreReport, error) {
	rep := RestoreReport{Results: []Result{}, Warnings: []string{}}
	switch o.Mode {
	case ModeInsert, ModeUpsert, ModeDrop:
	default:
		return rep, fmt.Errorf("unknown restore mode %q", o.Mode)
	}
	in, err := Inspect(o.Dir)
	if err != nil {
		return rep, err
	}
	rep.Warnings = append(rep.Warnings, in.Warnings...)
	entries := o.Selected(in)
	if len(entries) == 0 {
		return rep, errors.New("nothing to restore: no namespace in the backup matches the selection")
	}
	var total int64
	for _, e := range entries {
		if e.Expected > 0 {
			total += e.Expected
		}
	}
	log.SetTotal(total, "docs")
	log.Log("Restoring %d namespaces from %s (mode: %s)", len(entries), o.Dir, o.Mode)
	for _, w := range in.Warnings {
		log.Log("warning: %s", w)
	}
	failed := 0
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		tdb, tcoll := o.Target(e.DB, e.Coll)
		res := Result{Source: e.DB + "." + e.Coll, Target: tdb + "." + tcoll}
		err := restoreOne(ctx, cl, o, e, tdb, tcoll, &res, log)
		if err == nil && e.Expected >= 0 && res.Read != e.Expected {
			err = fmt.Errorf("read %d documents but the manifest lists %d; the backup file may be truncated", res.Read, e.Expected)
		}
		if err == nil && res.Failed > 0 {
			err = fmt.Errorf("%d documents were rejected by the server", res.Failed)
		}
		if err != nil {
			if ctx.Err() != nil {
				res.Error = "canceled"
				rep.Results = append(rep.Results, res)
				return rep, ctx.Err()
			}
			res.Error = err.Error()
			failed++
			log.Log("FAILED %s → %s: %v", res.Source, res.Target, err)
		} else if e.Type == "view" {
			log.Log("%s: view restored", res.Target)
		} else {
			log.Log("%s: %d read, %d inserted, %d replaced, %d already present, %d indexes", res.Target, res.Read, res.Inserted, res.Replaced, res.Duplicates, res.Indexes)
		}
		rep.Results = append(rep.Results, res)
		if err != nil && o.StopOnError {
			return rep, fmt.Errorf("stopped at %s: %w", res.Source, err)
		}
	}
	if failed > 0 {
		return rep, fmt.Errorf("%d of %d namespaces failed; see the job log", failed, len(entries))
	}
	return rep, nil
}

// prepareTarget drops the target in drop mode, then creates it with the
// saved options (always for views) unless it already exists.
func prepareTarget(ctx context.Context, db *mongo.Database, tcoll, mode string, isView bool, opts bson.D, skipOptions bool, log Logger) error {
	if mode == ModeDrop {
		if err := db.Collection(tcoll).Drop(ctx); err != nil {
			return fmt.Errorf("drop: %w", err)
		}
	}
	exists, err := collectionExists(ctx, db, tcoll)
	if err != nil {
		return err
	}
	switch {
	case exists && isView:
		return errors.New("a collection or view with this name already exists; choose drop mode or another target")
	case exists:
		if !skipOptions && len(opts) > 0 {
			log.Log("%s.%s exists; its current options are kept (saved options were not applied)", db.Name(), tcoll)
		}
	case isView || (!skipOptions && len(opts) > 0):
		cmd := bson.D{{Key: "create", Value: tcoll}}
		for _, opt := range opts {
			// Server-assigned or legacy fields that create rejects.
			if opt.Key == "uuid" || opt.Key == "flags" || opt.Key == "autoIndexId" {
				continue
			}
			cmd = append(cmd, opt)
		}
		if err := db.RunCommand(ctx, cmd).Err(); err != nil {
			return fmt.Errorf("create with saved options: %w", err)
		}
	}
	return nil
}

func collectionExists(ctx context.Context, db *mongo.Database, name string) (bool, error) {
	names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: name}})
	return len(names) > 0, err
}

func restoreOne(ctx context.Context, cl *mongo.Client, o RestoreOptions, e Entry, tdb, tcoll string, res *Result, log Logger) error {
	db := cl.Database(tdb)
	var md Metadata
	if e.MetadataFile != "" {
		var err error
		if md, err = readMetadata(e.MetadataFile); err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
	}
	isView := e.Type == "view"
	if err := prepareTarget(ctx, db, tcoll, o.Mode, isView, md.Options, o.SkipOptions, log); err != nil {
		return err
	}
	if isView {
		return nil
	}
	if e.DataFile != "" {
		if err := loadData(ctx, db.Collection(tcoll), o, e.DataFile, res, log); err != nil {
			return err
		}
	}
	if !o.SkipIndexes {
		n, err := restoreIndexes(ctx, db, tcoll, md.Indexes)
		res.Indexes = n
		if err != nil {
			return err
		}
	}
	return nil
}

const batchDocs = 1000
const batchBytes = 8 << 20

func loadData(ctx context.Context, c *mongo.Collection, o RestoreOptions, path string, res *Result, log Logger) error {
	f, err := openFile(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := newDocReader(f)
	var batch []bson.Raw
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := writeBatch(ctx, c, o, batch, res)
		log.Add(int64(len(batch)))
		batch, size = batch[:0], 0
		return err
	}
	for {
		raw, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		res.Read++
		batch = append(batch, append(bson.Raw(nil), raw...))
		size += len(raw)
		if len(batch) >= batchDocs || size >= batchBytes {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

func writeBatch(ctx context.Context, c *mongo.Collection, o RestoreOptions, batch []bson.Raw, res *Result) error {
	if o.Mode == ModeUpsert {
		models := make([]mongo.WriteModel, 0, len(batch))
		for _, d := range batch {
			id, err := d.LookupErr("_id")
			if err != nil {
				models = append(models, mongo.NewInsertOneModel().SetDocument(d))
				continue
			}
			models = append(models, mongo.NewReplaceOneModel().SetFilter(bson.D{{Key: "_id", Value: id}}).SetReplacement(d).SetUpsert(true))
		}
		opts := options.BulkWrite().SetOrdered(false)
		if o.BypassValidate {
			opts.SetBypassDocumentValidation(true)
		}
		r, err := c.BulkWrite(ctx, models, opts)
		if r != nil {
			res.Inserted += r.InsertedCount + r.UpsertedCount
			res.Replaced += r.MatchedCount
		}
		var bwe mongo.BulkWriteException
		if errors.As(err, &bwe) && bwe.WriteConcernError == nil {
			res.Failed += int64(len(bwe.WriteErrors))
			return nil
		}
		return err
	}
	docs := make([]any, len(batch))
	for i, d := range batch {
		docs[i] = d
	}
	opts := options.InsertMany().SetOrdered(false)
	if o.BypassValidate {
		opts.SetBypassDocumentValidation(true)
	}
	r, err := c.InsertMany(ctx, docs, opts)
	if err == nil && r != nil {
		res.Inserted += int64(len(r.InsertedIDs))
	}
	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) && bwe.WriteConcernError == nil {
		var dup, bad int64
		for _, we := range bwe.WriteErrors {
			if we.Code == 11000 {
				dup++
			} else {
				bad++
			}
		}
		res.Duplicates += dup
		res.Failed += bad
		res.Inserted += int64(len(batch)) - dup - bad
		return nil
	}
	return err
}

// restoreIndexes creates the saved indexes (except _id). An index that
// conflicts with an existing one is reported as an error, never skipped.
func restoreIndexes(ctx context.Context, db *mongo.Database, coll string, specs []bson.D) (int, error) {
	var idx bson.A
	for _, s := range specs {
		if name, _ := lookup(s, "name").(string); name == "_id_" {
			continue
		}
		clean := bson.D{}
		for _, e := range s {
			if e.Key == "ns" {
				continue
			}
			clean = append(clean, e)
		}
		idx = append(idx, clean)
	}
	if len(idx) == 0 {
		return 0, nil
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "createIndexes", Value: coll}, {Key: "indexes", Value: idx}}).Err(); err != nil {
		return 0, fmt.Errorf("indexes: %w", err)
	}
	return len(idx), nil
}
