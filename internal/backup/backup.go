// Package backup records before-images of documents that an operation is
// about to change, so sync, reschema, masking and AI-generated writes can be
// undone. A backup is an NDJSON file of canonical Extended JSON: a header line,
// then one line per affected document.
package backup

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// Operations recorded per document.
const (
	OpReplaced = "replaced" // before-image of a document that was overwritten
	OpDeleted  = "deleted"  // before-image of a document that was removed
	OpInserted = "inserted" // _id of a document that did not exist before
)

// Header is the first line of a backup file.
type Header struct {
	Format      int       `json:"mango3sBackup"`
	ID          string    `json:"id"`
	Created     time.Time `json:"created"`
	Operation   string    `json:"operation"` // sync, reschema, mask, ai-write ...
	Description string    `json:"description"`
	DB          string    `json:"db"`
	Coll        string    `json:"coll"`
}

// Info describes a backup on disk.
type Info struct {
	Header
	Path     string `json:"path"`
	Replaced int64  `json:"replaced"`
	Deleted  int64  `json:"deleted"`
	Inserted int64  `json:"inserted"`
	Bytes    int64  `json:"bytes"`
}

type line struct {
	Op  string          `json:"op"`
	Doc json.RawMessage `json:"doc,omitempty"`
	ID  json.RawMessage `json:"id,omitempty"`
}

// Writer appends entries to a backup. Safe for concurrent use.
type Writer struct {
	mu   sync.Mutex
	f    *os.File
	w    *bufio.Writer
	info Info
}

// Dir returns the backup directory inside the config directory.
func Dir(configDir string) string { return filepath.Join(configDir, "backups") }

// Create starts a new backup file in dir.
func Create(dir, operation, description, db, coll string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	h := Header{Format: 1, ID: config.NewID(), Created: time.Now().UTC(), Operation: operation, Description: description, DB: db, Coll: coll}
	name := fmt.Sprintf("%s-%s-%s.%s-%s.ndjson", h.Created.Format("20060102-150405"), operation, safe(db), safe(coll), h.ID)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	w := &Writer{f: f, w: bufio.NewWriterSize(f, 64<<10), info: Info{Header: h, Path: path}}
	hb, _ := json.Marshal(h)
	if _, err := w.w.Write(append(hb, '\n')); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == ' ' {
			return '_'
		}
		return r
	}, s)
}

// Info returns the backup's metadata so far.
func (w *Writer) Info() Info {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.info
}

func (w *Writer) write(l line) error {
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.w.Write(append(b, '\n'))
	return err
}

func canonical(v any) (json.RawMessage, error) {
	data, err := bson.MarshalExtJSON(bson.D{{Key: "v", Value: v}}, true, false)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		V json.RawMessage `json:"v"`
	}
	err = json.Unmarshal(data, &wrap)
	return wrap.V, err
}

// Replaced records the before-image of a document about to be overwritten.
func (w *Writer) Replaced(doc bson.D) error { return w.doc(OpReplaced, doc) }

// Deleted records the before-image of a document about to be removed.
func (w *Writer) Deleted(doc bson.D) error { return w.doc(OpDeleted, doc) }

func (w *Writer) doc(op string, doc bson.D) error {
	raw, err := canonical(doc)
	if err != nil {
		return err
	}
	if err := w.write(line{Op: op, Doc: raw}); err != nil {
		return err
	}
	w.mu.Lock()
	if op == OpReplaced {
		w.info.Replaced++
	} else {
		w.info.Deleted++
	}
	w.mu.Unlock()
	return nil
}

// Inserted records the _id of a document that is about to be created.
func (w *Writer) Inserted(id any) error {
	raw, err := canonical(id)
	if err != nil {
		return err
	}
	if err := w.write(line{Op: OpInserted, ID: raw}); err != nil {
		return err
	}
	w.mu.Lock()
	w.info.Inserted++
	w.mu.Unlock()
	return nil
}

// Close flushes the file to disk (fsync) so it survives a crash of the write that follows.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.w.Flush(); err != nil {
		w.f.Close()
		return err
	}
	if err := w.f.Sync(); err != nil {
		w.f.Close()
		return err
	}
	if st, err := w.f.Stat(); err == nil {
		w.info.Bytes = st.Size()
	}
	return w.f.Close()
}

// Flush writes buffered entries and syncs them, keeping the file open.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.w.Flush(); err != nil {
		return err
	}
	return w.f.Sync()
}

// Read parses a backup file.
func Read(path string, each func(op string, doc bson.D, id any) error) (Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return Header{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	var h Header
	n := 0
	for sc.Scan() {
		n++
		if n == 1 {
			if err := json.Unmarshal(sc.Bytes(), &h); err != nil || h.Format != 1 {
				return h, fmt.Errorf("%s is not a Mango3S backup", filepath.Base(path))
			}
			continue
		}
		var l line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return h, fmt.Errorf("line %d: %w", n, err)
		}
		switch l.Op {
		case OpReplaced, OpDeleted:
			var d bson.D
			if err := bson.UnmarshalExtJSON(l.Doc, true, &d); err != nil {
				return h, fmt.Errorf("line %d: %w", n, err)
			}
			if err := each(l.Op, d, nil); err != nil {
				return h, err
			}
		case OpInserted:
			var wrap bson.D
			if err := bson.UnmarshalExtJSON([]byte(`{"v":`+string(l.ID)+`}`), true, &wrap); err != nil {
				return h, fmt.Errorf("line %d: %w", n, err)
			}
			if err := each(l.Op, nil, wrap[0].Value); err != nil {
				return h, err
			}
		default:
			return h, fmt.Errorf("line %d: unknown operation %q", n, l.Op)
		}
	}
	if err := sc.Err(); err != nil {
		return h, err
	}
	if n == 0 {
		return h, fmt.Errorf("%s is empty", filepath.Base(path))
	}
	return h, nil
}

// List returns the backups in dir, newest first.
func List(dir string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Info{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ndjson") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info := Info{Path: path}
		h, err := Read(path, func(op string, _ bson.D, _ any) error {
			switch op {
			case OpReplaced:
				info.Replaced++
			case OpDeleted:
				info.Deleted++
			case OpInserted:
				info.Inserted++
			}
			return nil
		})
		if err != nil {
			continue
		}
		info.Header = h
		if st, err := e.Info(); err == nil {
			info.Bytes = st.Size()
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// RestoreStats summarizes a restore.
type RestoreStats struct {
	Restored int64 `json:"restored"` // replaced or deleted documents put back
	Removed  int64 `json:"removed"`  // inserted documents deleted again
}

// Restore undoes the recorded operation on coll: documents that were replaced
// or deleted are written back (upsert by _id) and inserted documents are
// deleted. It is idempotent, so a partially applied restore can be re-run.
func Restore(ctx context.Context, coll *mongo.Collection, path string) (RestoreStats, error) {
	var st RestoreStats
	var models []mongo.WriteModel
	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		res, err := coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		models = models[:0]
		if res != nil {
			st.Restored += res.UpsertedCount + res.MatchedCount
			st.Removed += res.DeletedCount
		}
		return apperr.Check(err)
	}
	_, err := Read(path, func(op string, doc bson.D, id any) error {
		switch op {
		case OpReplaced, OpDeleted:
			var docID any
			for _, e := range doc {
				if e.Key == "_id" {
					docID = e.Value
				}
			}
			models = append(models, mongo.NewReplaceOneModel().
				SetFilter(bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: docID}}}}).
				SetReplacement(doc).SetUpsert(true))
		case OpInserted:
			models = append(models, mongo.NewDeleteOneModel().SetFilter(bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}}}))
		}
		if len(models) >= 500 {
			return flush()
		}
		return nil
	})
	if err != nil {
		return st, err
	}
	return st, flush()
}

// ManyResult counts the effect of WriteManyWithBackup.
type ManyResult struct {
	Matched  int64 `json:"matched"`
	Modified int64 `json:"modified"`
	Deleted  int64 `json:"deleted"`
}

// WriteManyWithBackup applies updateMany (update != nil) or deleteMany
// (update == nil) to the documents matching filter, in _id batches: each
// batch's before-images are flushed to bk before the batch is written with
// filter AND _id in batch, so no document changes without a backup.
// maxDocs > 0 aborts before writing anything if more documents match.
func WriteManyWithBackup(ctx context.Context, c *mongo.Collection, filter bson.D, update any, bk *Writer, maxDocs int64) (ManyResult, error) {
	var res ManyResult
	if maxDocs > 0 {
		n, err := c.CountDocuments(ctx, filter)
		if err != nil {
			return res, apperr.From(err)
		}
		if n > maxDocs {
			return res, apperr.Invalid("%d documents match; the limit for this operation is %d.", n, maxDocs)
		}
	}
	cur, err := c.Find(ctx, filter, options.Find().SetBatchSize(500))
	if err != nil {
		return res, apperr.From(err)
	}
	defer cur.Close(context.Background())
	var ids bson.A
	flush := func() error {
		if len(ids) == 0 {
			return nil
		}
		if err := bk.Flush(); err != nil {
			return err
		}
		scoped := bson.D{{Key: "$and", Value: bson.A{filter, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}}}}
		if update != nil {
			r, err := c.UpdateMany(ctx, scoped, update)
			if err != nil {
				return apperr.From(err)
			}
			res.Matched += r.MatchedCount
			res.Modified += r.ModifiedCount
		} else {
			r, err := c.DeleteMany(ctx, scoped)
			if err != nil {
				return apperr.From(err)
			}
			res.Deleted += r.DeletedCount
		}
		ids = ids[:0]
		return nil
	}
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return res, apperr.From(err)
		}
		if update != nil {
			err = bk.Replaced(d)
		} else {
			err = bk.Deleted(d)
		}
		if err != nil {
			return res, err
		}
		for _, e := range d {
			if e.Key == "_id" {
				ids = append(ids, e.Value)
			}
		}
		if len(ids) >= 500 {
			if err := flush(); err != nil {
				return res, err
			}
		}
	}
	if err := cur.Err(); err != nil {
		return res, apperr.From(err)
	}
	return res, flush()
}
