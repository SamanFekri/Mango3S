// Package gridfs browses GridFS buckets: it finds them from matching
// <bucket>.files / <bucket>.chunks pairs, lists files page by page, streams
// uploads and downloads with progress, previews small images and text, and
// renames and deletes files (deleting a file removes its chunks).
package gridfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Preview limits.
const (
	MaxImagePreview = 8 << 20
	MaxTextPreview  = 256 << 10
)

// Bucket is one GridFS bucket in a database.
type Bucket struct {
	Name       string `json:"name"`
	Files      int64  `json:"files"`
	ChunkBytes int64  `json:"chunkBytes"` // storage used by the chunks collection
}

// File is one entry of a bucket's files collection.
type File struct {
	ID          string    `json:"id"` // the _id in mongosh syntax
	Filename    string    `json:"filename"`
	Length      int64     `json:"length"`
	ChunkSize   int64     `json:"chunkSize"`
	UploadDate  time.Time `json:"uploadDate"`
	ContentType string    `json:"contentType"`
	Metadata    string    `json:"metadata,omitempty"` // mongosh syntax
}

// Page is a page of files.
type Page struct {
	Files []File `json:"files"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
}

// ListRequest selects files.
type ListRequest struct {
	ConnID   string `json:"connId"`
	DB       string `json:"db"`
	Bucket   string `json:"bucket"`
	Name     string `json:"name"` // filename contains (case-insensitive)
	Sort     string `json:"sort"` // uploadDate (default, newest first) | filename | length
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// Service does GridFS work on open connections.
type Service struct{ src mongox.Source }

// New returns a GridFS service.
func New(src mongox.Source) *Service { return &Service{src: src} }

func (s *Service) db(connID, db string) (*mongox.Session, *mongo.Database, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return nil, nil, err
	}
	if db == "" {
		return nil, nil, apperr.Invalid("Choose a database.")
	}
	return sess, sess.Client.Database(db), nil
}

func bucketOf(d *mongo.Database, name string) *mongo.GridFSBucket {
	return d.GridFSBucket(options.GridFSBucket().SetName(name))
}

// BucketNames finds bucket names from collection names: a bucket is a
// "<name>.files" collection with a matching "<name>.chunks".
func BucketNames(collections []string) []string {
	has := map[string]bool{}
	for _, c := range collections {
		has[c] = true
	}
	var out []string
	for _, c := range collections {
		if name, ok := strings.CutSuffix(c, ".files"); ok && name != "" && has[name+".chunks"] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// ListBuckets lists the buckets of a database with their file counts.
func (s *Service) ListBuckets(ctx context.Context, connID, db string) ([]Bucket, error) {
	_, d, err := s.db(connID, db)
	if err != nil {
		return nil, err
	}
	names, err := d.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, apperr.From(err)
	}
	out := []Bucket{}
	for _, name := range BucketNames(names) {
		b := Bucket{Name: name}
		b.Files, _ = d.Collection(name + ".files").EstimatedDocumentCount(ctx)
		var st bson.D
		if d.RunCommand(ctx, bson.D{{Key: "collStats", Value: name + ".chunks"}}).Decode(&st) == nil {
			if v, ok := mql.Lookup(st, "size"); ok {
				b.ChunkBytes = toInt(v)
			}
		}
		out = append(out, b)
	}
	return out, nil
}

// List returns a page of files.
func (s *Service) List(ctx context.Context, r ListRequest) (Page, error) {
	_, d, err := s.db(r.ConnID, r.DB)
	if err != nil {
		return Page{}, err
	}
	if r.Bucket == "" {
		return Page{}, apperr.Invalid("Choose a bucket.")
	}
	if r.PageSize <= 0 || r.PageSize > 500 {
		r.PageSize = 50
	}
	if r.Page < 0 {
		r.Page = 0
	}
	filter := bson.D{}
	if n := strings.TrimSpace(r.Name); n != "" {
		filter = bson.D{{Key: "filename", Value: bson.D{{Key: "$regex", Value: regexp.QuoteMeta(n)}, {Key: "$options", Value: "i"}}}}
	}
	sortDoc := bson.D{{Key: "uploadDate", Value: -1}}
	switch r.Sort {
	case "filename":
		sortDoc = bson.D{{Key: "filename", Value: 1}}
	case "length":
		sortDoc = bson.D{{Key: "length", Value: -1}}
	}
	files := d.Collection(r.Bucket + ".files")
	total, err := files.CountDocuments(ctx, filter)
	if err != nil {
		return Page{}, apperr.From(err)
	}
	cur, err := files.Find(ctx, filter, options.Find().SetSort(sortDoc).SetSkip(int64(r.Page*r.PageSize)).SetLimit(int64(r.PageSize)))
	if err != nil {
		return Page{}, apperr.From(err)
	}
	defer cur.Close(ctx)
	page := Page{Files: []File{}, Total: total, Page: r.Page}
	for cur.Next(ctx) {
		var doc bson.D
		if err := cur.Decode(&doc); err != nil {
			return Page{}, apperr.From(err)
		}
		page.Files = append(page.Files, fileFrom(doc))
	}
	if err := cur.Err(); err != nil {
		return Page{}, apperr.From(err)
	}
	return page, nil
}

func fileFrom(doc bson.D) File {
	f := File{}
	if v, ok := mql.Lookup(doc, "_id"); ok {
		f.ID = mql.Compact(v)
	}
	if v, ok := mql.Lookup(doc, "filename"); ok {
		f.Filename, _ = v.(string)
	}
	if v, ok := mql.Lookup(doc, "length"); ok {
		f.Length = toInt(v)
	}
	if v, ok := mql.Lookup(doc, "chunkSize"); ok {
		f.ChunkSize = toInt(v)
	}
	if v, ok := mql.Lookup(doc, "uploadDate"); ok {
		if dt, ok := v.(bson.DateTime); ok {
			f.UploadDate = dt.Time().UTC()
		}
	}
	if v, ok := mql.Lookup(doc, "metadata"); ok && v != nil {
		f.Metadata = mql.Compact(v)
		if md, ok := v.(bson.D); ok {
			if ct, ok := mql.Lookup(md, "contentType"); ok {
				f.ContentType, _ = ct.(string)
			}
		}
	}
	if f.ContentType == "" { // the deprecated top-level field older drivers wrote
		if v, ok := mql.Lookup(doc, "contentType"); ok {
			f.ContentType, _ = v.(string)
		}
	}
	if f.ContentType == "" {
		f.ContentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(f.Filename)))
	}
	return f
}

func toInt(v any) int64 {
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	}
	return 0
}

// parseID turns a file ID shown in the UI back into its BSON value.
func parseID(id string) (any, error) {
	v, err := mql.ParseValue(id)
	if err != nil {
		return nil, apperr.Invalid("The file ID %q can't be read.", id)
	}
	return v, nil
}

// Upload streams a local file into a bucket and returns the new file. onBytes
// reports progress.
func (s *Service) Upload(ctx context.Context, connID, db, bucket, path, filename string, onBytes func(int64)) (File, error) {
	sess, d, err := s.db(connID, db)
	if err != nil {
		return File{}, err
	}
	if err := sess.RequireWritable(); err != nil {
		return File{}, err
	}
	if bucket == "" {
		bucket = "fs"
	}
	in, err := os.Open(path)
	if err != nil {
		return File{}, apperr.Wrap(apperr.CodeInvalidInput, err, "The file can't be opened.")
	}
	defer in.Close()
	if filename == "" {
		filename = filepath.Base(path)
	}
	md := bson.D{}
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); ct != "" {
		md = append(md, bson.E{Key: "contentType", Value: ct})
	}
	up, err := bucketOf(d, bucket).OpenUploadStream(ctx, filename, options.GridFSUpload().SetMetadata(md))
	if err != nil {
		return File{}, apperr.From(err)
	}
	buf := make([]byte, 1<<20)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := up.Write(buf[:n]); werr != nil {
				_ = up.Abort()
				return File{}, apperr.From(werr)
			}
			if onBytes != nil {
				onBytes(int64(n))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = up.Abort()
			return File{}, apperr.Wrap(apperr.CodeInternal, rerr, "Reading the file failed.")
		}
		if ctx.Err() != nil {
			_ = up.Abort()
			return File{}, apperr.From(ctx.Err())
		}
	}
	if err := up.Close(); err != nil {
		return File{}, apperr.From(err)
	}
	var doc bson.D
	if err := d.Collection(bucket+".files").FindOne(ctx, bson.D{{Key: "_id", Value: up.FileID}}).Decode(&doc); err != nil {
		return File{}, apperr.From(err)
	}
	return fileFrom(doc), nil
}

// Download streams a file to dest (written to a temporary file and renamed
// when complete, so a failed or canceled download leaves nothing behind).
// onStart receives the file's length.
func (s *Service) Download(ctx context.Context, connID, db, bucket, id, dest string, onStart func(int64), onBytes func(int64)) (int64, error) {
	_, d, err := s.db(connID, db)
	if err != nil {
		return 0, err
	}
	fid, err := parseID(id)
	if err != nil {
		return 0, err
	}
	ds, err := bucketOf(d, bucket).OpenDownloadStream(ctx, fid)
	if err != nil {
		return 0, notFound(err)
	}
	defer ds.Close()
	if onStart != nil {
		onStart(ds.GetFile().Length)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*.part")
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeInternal, err, "The file can't be written there.")
	}
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	var n int64
	buf := make([]byte, 1<<20)
	for {
		if ctx.Err() != nil {
			return n, apperr.From(ctx.Err())
		}
		k, rerr := ds.Read(buf)
		if k > 0 {
			if _, werr := tmp.Write(buf[:k]); werr != nil {
				return n, apperr.Wrap(apperr.CodeInternal, werr, "Writing the file failed.")
			}
			n += int64(k)
			if onBytes != nil {
				onBytes(int64(k))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return n, apperr.From(rerr)
		}
	}
	if err := tmp.Close(); err != nil {
		return n, apperr.Wrap(apperr.CodeInternal, err, "Writing the file failed.")
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return n, apperr.Wrap(apperr.CodeInternal, err, "The file can't be saved there.")
	}
	ok = true
	return n, nil
}

// Preview is a small file's content for display.
type Preview struct {
	Kind        string `json:"kind"` // image | text | none
	ContentType string `json:"contentType"`
	DataURL     string `json:"dataUrl,omitempty"`
	Text        string `json:"text,omitempty"`
	Truncated   bool   `json:"truncated"`
	Reason      string `json:"reason,omitempty"`
}

// Preview reads an image (up to MaxImagePreview) or the start of a text file.
func (s *Service) Preview(ctx context.Context, connID, db, bucket, id string) (Preview, error) {
	_, d, err := s.db(connID, db)
	if err != nil {
		return Preview{}, err
	}
	fid, err := parseID(id)
	if err != nil {
		return Preview{}, err
	}
	ds, err := bucketOf(d, bucket).OpenDownloadStream(ctx, fid)
	if err != nil {
		return Preview{}, notFound(err)
	}
	defer ds.Close()
	length := ds.GetFile().Length
	head := make([]byte, 512)
	n, _ := io.ReadFull(ds, head)
	head = head[:n]
	ct := http.DetectContentType(head)
	switch {
	case strings.HasPrefix(ct, "image/"):
		if length > MaxImagePreview {
			return Preview{Kind: "none", ContentType: ct, Reason: "The image is too large to preview; download it instead."}, nil
		}
		rest, err := io.ReadAll(io.LimitReader(ds, MaxImagePreview))
		if err != nil {
			return Preview{}, apperr.From(err)
		}
		data := append(head, rest...)
		return Preview{Kind: "image", ContentType: ct, DataURL: "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)}, nil
	case strings.HasPrefix(ct, "text/") || ct == "application/json" || looksText(head):
		rest, err := io.ReadAll(io.LimitReader(ds, MaxTextPreview-int64(len(head))))
		if err != nil {
			return Preview{}, apperr.From(err)
		}
		data := append(head, rest...)
		for !utf8.Valid(data) && len(data) > 0 { // don't cut a character in half
			data = data[:len(data)-1]
		}
		return Preview{Kind: "text", ContentType: ct, Text: string(data), Truncated: length > int64(len(data))}, nil
	}
	return Preview{Kind: "none", ContentType: ct, Reason: "This type of file can't be previewed; download it instead."}, nil
}

func looksText(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	return utf8.Valid(b) || utf8.Valid(b[:len(b)-min(3, len(b))])
}

// Delete removes a file and its chunks.
func (s *Service) Delete(ctx context.Context, connID, db, bucket, id string) error {
	sess, d, err := s.db(connID, db)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	fid, err := parseID(id)
	if err != nil {
		return err
	}
	return notFound(bucketOf(d, bucket).Delete(ctx, fid))
}

// Rename changes a file's name.
func (s *Service) Rename(ctx context.Context, connID, db, bucket, id, name string) error {
	sess, d, err := s.db(connID, db)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return apperr.Invalid("The new name can't be empty.")
	}
	fid, err := parseID(id)
	if err != nil {
		return err
	}
	return notFound(bucketOf(d, bucket).Rename(ctx, fid, name))
}

func notFound(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "file with given parameters not found") || err == mongo.ErrFileNotFound {
		return apperr.New(apperr.CodeNotFound, "The file no longer exists.")
	}
	return apperr.From(err)
}
