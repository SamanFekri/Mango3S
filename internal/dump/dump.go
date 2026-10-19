package dump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Logger receives progress lines (jobs.Progress satisfies it).
type Logger interface {
	Log(format string, args ...any)
	SetTotal(total int64, unit string)
	Add(n int64)
}

// Selection picks namespaces. Empty Databases means every non-system
// database. Include/Exclude hold "db.coll" patterns where "*" matches any
// run of characters ("sales.*", "*.tmp_*").
type Selection struct {
	Databases     []string `json:"databases"`
	Include       []string `json:"include"`
	Exclude       []string `json:"exclude"`
	IncludeSystem bool     `json:"includeSystem"` // system.* collections such as system.js
	Views         bool     `json:"views"`         // view definitions (metadata only)
}

// Options controls a backup.
type Options struct {
	Dir       string    `json:"dir"`  // empty folder to create the backup in
	Gzip      bool      `json:"gzip"` // .bson.gz + .metadata.json.gz
	Selection Selection `json:"selection"`
	Query     string    `json:"query"` // optional filter (canonical Extended JSON) applied to every collection
	Source    string    `json:"source"`
}

// glob reports whether s matches pattern, where "*" matches any run of
// characters (including dots).
func glob(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		k := strings.Index(s, parts[i])
		if k < 0 {
			return false
		}
		s = s[k+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

// Match reports whether a namespace is selected.
func (s Selection) Match(db, coll string) bool {
	ns := db + "." + coll
	if len(s.Include) > 0 {
		ok := false
		for _, p := range s.Include {
			if glob(strings.TrimSpace(p), ns) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, p := range s.Exclude {
		if glob(strings.TrimSpace(p), ns) {
			return false
		}
	}
	return true
}

// Systemic databases are never part of a logical backup of user data.
var skipDatabases = map[string]bool{"local": true, "config": true}

// collInfo is one listCollections entry.
type collInfo struct {
	Name    string `bson:"name"`
	Type    string `bson:"type"`
	Options bson.D `bson:"options"`
	Info    struct {
		ReadOnly bool         `bson:"readOnly"`
		UUID     *bson.Binary `bson:"uuid"`
	} `bson:"info"`
}

// Plan lists the namespaces a backup would contain, in order.
func Plan(ctx context.Context, cl *mongo.Client, sel Selection) ([]Namespace, error) {
	dbs := sel.Databases
	if len(dbs) == 0 {
		names, err := cl.ListDatabaseNames(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if !skipDatabases[n] && n != "admin" {
				dbs = append(dbs, n)
			}
		}
	}
	var out []Namespace
	for _, db := range dbs {
		if db == "" || strings.ContainsAny(db, "/\\. \"$") {
			return nil, fmt.Errorf("invalid database name %q", db)
		}
		cur, err := cl.Database(db).ListCollections(ctx, bson.D{})
		if err != nil {
			return nil, fmt.Errorf("list collections of %s: %w", db, err)
		}
		var infos []collInfo
		if err := cur.All(ctx, &infos); err != nil {
			return nil, fmt.Errorf("list collections of %s: %w", db, err)
		}
		sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
		for _, ci := range infos {
			typ := ci.Type
			if typ == "" {
				typ = "collection"
			}
			if strings.HasPrefix(ci.Name, "system.") {
				// system.buckets.* backs time series collections and is dumped
				// through them; system.views/profile are server-managed.
				if !sel.IncludeSystem || strings.HasPrefix(ci.Name, "system.buckets.") || ci.Name == "system.views" || ci.Name == "system.profile" {
					continue
				}
			}
			if typ == "view" && !sel.Views {
				continue
			}
			if !sel.Match(db, ci.Name) {
				continue
			}
			out = append(out, Namespace{DB: db, Coll: ci.Name, Type: typ})
		}
	}
	return out, nil
}

// Run writes a backup of the selected namespaces into o.Dir, which must be
// empty or not exist. Collections that fail are recorded in the manifest and
// the run continues; the returned error then summarizes the failures.
func Run(ctx context.Context, cl *mongo.Client, o Options, log Logger) (Manifest, error) {
	m := Manifest{Format: "mango3s-backup", Version: 1, Created: time.Now().UTC(), Tool: "mango3s", Source: o.Source, Gzip: o.Gzip}
	if err := prepareDir(o.Dir); err != nil {
		return m, err
	}
	var filter bson.D
	if strings.TrimSpace(o.Query) != "" {
		if err := bson.UnmarshalExtJSON([]byte(o.Query), false, &filter); err != nil {
			return m, fmt.Errorf("query is not valid Extended JSON: %w", err)
		}
	}
	m.ServerVersion = serverVersion(ctx, cl)
	nss, err := Plan(ctx, cl, o.Selection)
	if err != nil {
		return m, err
	}
	if len(nss) == 0 {
		return m, errors.New("nothing to back up: no collection matches the selection")
	}
	var total int64
	for i := range nss {
		if nss[i].Type != "view" {
			n, err := cl.Database(nss[i].DB).Collection(nss[i].Coll).EstimatedDocumentCount(ctx)
			if err == nil {
				total += n
			}
		}
	}
	log.SetTotal(total, "docs")
	log.Log("Backing up %d namespaces (~%d documents) to %s", len(nss), total, o.Dir)
	if filter != nil {
		m.Notes = append(m.Notes, "Filtered backup: only documents matching "+o.Query+" were saved.")
	}
	failed := 0
	for i := range nss {
		if err := ctx.Err(); err != nil {
			m.Namespaces = nss[:i]
			writeManifest(o.Dir, m)
			return m, err
		}
		ns := &nss[i]
		ns.Gzip = o.Gzip
		if err := dumpOne(ctx, cl, o, filter, ns, log); err != nil {
			if ctx.Err() != nil {
				m.Namespaces = nss[:i+1]
				ns.Error = "canceled"
				writeManifest(o.Dir, m)
				return m, ctx.Err()
			}
			ns.Error = err.Error()
			failed++
			log.Log("FAILED %s.%s: %v", ns.DB, ns.Coll, err)
		}
	}
	m.Namespaces = nss
	if err := writeManifest(o.Dir, m); err != nil {
		return m, err
	}
	if failed > 0 {
		return m, fmt.Errorf("%d of %d namespaces failed; see the job log and %s", failed, len(nss), ManifestName)
	}
	return m, nil
}

func serverVersion(ctx context.Context, cl *mongo.Client) string {
	var r struct {
		Version string `bson:"version"`
	}
	if cl.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&r) == nil {
		return r.Version
	}
	return ""
}

func prepareDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("choose a folder for the backup")
	}
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return os.MkdirAll(dir, 0o700)
	case err != nil:
		return err
	case len(entries) > 0:
		return fmt.Errorf("%s is not empty; choose an empty or new folder so no existing backup is overwritten", dir)
	}
	return nil
}

func writeManifest(dir string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ManifestName), b, 0o600)
}

func dumpOne(ctx context.Context, cl *mongo.Client, o Options, filter bson.D, ns *Namespace, log Logger) error {
	db := cl.Database(ns.DB)
	info, err := collectionInfo(ctx, db, ns.Coll)
	if err != nil {
		return err
	}
	md := Metadata{CollectionName: ns.Coll, Type: ns.Type, Options: info.Options, Indexes: []bson.D{}}
	if md.Options == nil {
		md.Options = bson.D{}
	}
	if info.Info.UUID != nil {
		md.UUID = fmt.Sprintf("%x", info.Info.UUID.Data)
	}
	if ns.Type != "view" {
		cur, err := db.Collection(ns.Coll).Indexes().List(ctx)
		if err != nil {
			return fmt.Errorf("list indexes: %w", err)
		}
		if err := cur.All(ctx, &md.Indexes); err != nil {
			return fmt.Errorf("list indexes: %w", err)
		}
		ns.Indexes = len(md.Indexes)
	}
	dir := filepath.Join(o.Dir, escapeName(ns.DB))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	base := filepath.Join(dir, escapeName(ns.Coll))
	if err := writeMetadata(base+".metadata.json"+ext(o.Gzip), md, o.Gzip); err != nil {
		return err
	}
	if ns.Type == "view" {
		log.Log("%s.%s: view definition saved", ns.DB, ns.Coll)
		return nil
	}
	f := filter
	if f == nil {
		f = bson.D{}
	}
	cur, err := db.Collection(ns.Coll).Find(ctx, f, options.Find().SetBatchSize(1000).SetNoCursorTimeout(false))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer cur.Close(context.Background())
	w, err := createFile(base+".bson"+ext(o.Gzip), o.Gzip)
	if err != nil {
		return err
	}
	var n int64
	for cur.Next(ctx) {
		if _, err := w.Write(cur.Current); err != nil {
			w.Close()
			return fmt.Errorf("write: %w", err)
		}
		n++
		if n%1000 == 0 {
			log.Add(1000)
		}
	}
	log.Add(n % 1000)
	if err := cur.Err(); err != nil {
		w.Close()
		return fmt.Errorf("read: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	ns.Count, ns.Bytes = n, w.n
	log.Log("%s.%s: %d documents, %d indexes", ns.DB, ns.Coll, n, ns.Indexes)
	return nil
}

func collectionInfo(ctx context.Context, db *mongo.Database, name string) (collInfo, error) {
	cur, err := db.ListCollections(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil {
		return collInfo{}, fmt.Errorf("collection info: %w", err)
	}
	var infos []collInfo
	if err := cur.All(ctx, &infos); err != nil {
		return collInfo{}, fmt.Errorf("collection info: %w", err)
	}
	if len(infos) == 0 {
		return collInfo{}, errors.New("collection no longer exists")
	}
	return infos[0], nil
}

func writeMetadata(path string, md Metadata, gz bool) error {
	b, err := bson.MarshalExtJSON(md, true, false)
	if err != nil {
		return err
	}
	w, err := createFile(path, gz)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
