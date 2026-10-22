//go:build integration

package integration

import (
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/gridfs"
	"mango3s/internal/mongox"
)

func TestGridFSRoundTripPreviewRenameDelete(t *testing.T) {
	e := setup(t)
	g := gridfs.New(e.mgr)
	dir := t.TempDir()

	// A 50 MB file comes back byte-identical, with progress for both ways.
	big := make([]byte, 50<<20)
	if _, err := rand.Read(big); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(src, big, 0o600); err != nil {
		t.Fatal(err)
	}
	var uploaded int64
	f, err := g.Upload(e.ctx, e.conn, e.db, "media", src, "", func(n int64) { uploaded += n })
	if err != nil {
		t.Fatal(err)
	}
	if f.Length != int64(len(big)) || uploaded != int64(len(big)) || f.Filename != "big.bin" {
		t.Fatalf("upload: %+v (%d bytes reported)", f, uploaded)
	}
	dest := filepath.Join(dir, "back.bin")
	var total, got int64
	if _, err := g.Download(e.ctx, e.conn, e.db, "media", f.ID, dest, func(n int64) { total = n }, func(n int64) { got += n }); err != nil {
		t.Fatal(err)
	}
	back, _ := os.ReadFile(dest)
	if sha256.Sum256(back) != sha256.Sum256(big) || total != int64(len(big)) || got != total {
		t.Fatalf("download differs: %d bytes, total %d, progress %d", len(back), total, got)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".back.bin.*.part")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}

	// Buckets, listing, search and sort.
	txt := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(txt, []byte("hello GridFS\n"), 0o600)
	tf, err := g.Upload(e.ctx, e.conn, e.db, "media", txt, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tf.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("content type from the extension: %q", tf.ContentType)
	}
	buckets, err := g.ListBuckets(e.ctx, e.conn, e.db)
	if err != nil || len(buckets) != 1 || buckets[0].Name != "media" || buckets[0].Files != 2 || buckets[0].ChunkBytes < int64(len(big)) {
		t.Fatalf("buckets: %+v %v", buckets, err)
	}
	page, err := g.List(e.ctx, gridfs.ListRequest{ConnID: e.conn, DB: e.db, Bucket: "media", Name: "NOTES", PageSize: 10})
	if err != nil || page.Total != 1 || len(page.Files) != 1 || page.Files[0].Filename != "notes.txt" {
		t.Fatalf("search: %+v %v", page, err)
	}
	page, err = g.List(e.ctx, gridfs.ListRequest{ConnID: e.conn, DB: e.db, Bucket: "media", Sort: "length", PageSize: 1})
	if err != nil || page.Total != 2 || len(page.Files) != 1 || page.Files[0].Filename != "big.bin" {
		t.Fatalf("paging and sort: %+v %v", page, err)
	}

	// Previews.
	pv, err := g.Preview(e.ctx, e.conn, e.db, "media", tf.ID)
	if err != nil || pv.Kind != "text" || pv.Text != "hello GridFS\n" || pv.Truncated {
		t.Fatalf("text preview: %+v %v", pv, err)
	}
	png := filepath.Join(dir, "dot.png")
	_ = os.WriteFile(png, []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"), 0o600)
	pf, _ := g.Upload(e.ctx, e.conn, e.db, "media", png, "", nil)
	if pv, err := g.Preview(e.ctx, e.conn, e.db, "media", pf.ID); err != nil || pv.Kind != "image" || !strings.HasPrefix(pv.DataURL, "data:image/png;base64,") {
		t.Fatalf("image preview: %+v %v", pv.Kind, err)
	}
	if pv, err := g.Preview(e.ctx, e.conn, e.db, "media", f.ID); err != nil || pv.Kind != "none" {
		t.Fatalf("binary preview: %+v %v", pv.Kind, err)
	}

	// Rename, then delete removes the chunks too.
	if err := g.Rename(e.ctx, e.conn, e.db, "media", tf.ID, "renamed.txt"); err != nil {
		t.Fatal(err)
	}
	if page, _ := g.List(e.ctx, gridfs.ListRequest{ConnID: e.conn, DB: e.db, Bucket: "media", Name: "renamed"}); page.Total != 1 {
		t.Fatalf("rename: %+v", page)
	}
	s, _ := e.mgr.Session(e.conn)
	chunks := s.Client.Database(e.db).Collection("media.chunks")
	fid, _ := bson.ObjectIDFromHex(strings.TrimSuffix(strings.TrimPrefix(f.ID, `ObjectId("`), `")`))
	before, _ := chunks.CountDocuments(e.ctx, bson.D{{Key: "files_id", Value: fid}})
	if before == 0 {
		t.Fatalf("no chunks found for %s", f.ID)
	}
	if err := g.Delete(e.ctx, e.conn, e.db, "media", f.ID); err != nil {
		t.Fatal(err)
	}
	if after, _ := chunks.CountDocuments(e.ctx, bson.D{{Key: "files_id", Value: fid}}); after != 0 {
		t.Fatalf("%d chunks left after delete", after)
	}
	if err := g.Delete(e.ctx, e.conn, e.db, "media", f.ID); !apperr.Is(err, apperr.CodeNotFound) {
		t.Fatalf("deleting twice: %v", err)
	}

	// Read-only connections can't change anything.
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if _, err := e.mgr.Connect(e.ctx, config.Profile{ID: "ro-gridfs", Name: "ro", URI: uri, ReadOnly: true, ConnectTimeoutSeconds: 5}, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Upload(e.ctx, "ro-gridfs", e.db, "media", txt, "", nil); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("read-only upload: %v", err)
	}
	if err := g.Delete(e.ctx, "ro-gridfs", e.db, "media", tf.ID); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("read-only delete: %v", err)
	}
}
