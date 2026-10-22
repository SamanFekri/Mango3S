// Package dump writes and restores full logical backups in the mongodump
// directory layout, so backups move freely between Mango3S and the MongoDB
// Database Tools:
//
//	<dir>/<db>/<coll>.bson[.gz]            raw BSON documents, back to back
//	<dir>/<db>/<coll>.metadata.json[.gz]   indexes, options, type (canonical Extended JSON)
//	<dir>/mango3s-backup.json              Mango3S manifest (counts, for validation)
//
// Documents are streamed as raw BSON (never decoded), so every BSON type is
// preserved exactly and memory use does not grow with the data. Views are
// stored as metadata only (viewOn + pipeline), as mongodump does.
package dump

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ManifestName is the Mango3S manifest at the root of a backup folder.
const ManifestName = "mango3s-backup.json"

// MaxDocSize is the largest BSON document accepted when reading a dump.
const MaxDocSize = 16*1024*1024 + 16*1024

// Metadata is the mongodump <coll>.metadata.json content.
type Metadata struct {
	Indexes        []bson.D `bson:"indexes"`
	UUID           string   `bson:"uuid,omitempty"`
	CollectionName string   `bson:"collectionName"`
	Type           string   `bson:"type,omitempty"` // collection | view | timeseries
	Options        bson.D   `bson:"options"`
}

// Namespace describes one collection or view in a backup.
type Namespace struct {
	DB      string `json:"db"`
	Coll    string `json:"coll"`
	Type    string `json:"type"` // collection | view | timeseries
	Count   int64  `json:"count"`
	Bytes   int64  `json:"bytes"`
	Indexes int    `json:"indexes"`
	Gzip    bool   `json:"gzip"`
	Error   string `json:"error,omitempty"`
	// ShardKey is the collection's shard key (mongosh syntax) when it was
	// backed up from a sharded cluster; a restore can shard it again.
	ShardKey    string `json:"shardKey,omitempty"`
	ShardUnique bool   `json:"shardUnique,omitempty"`
}

// Manifest is the Mango3S summary of a backup folder.
type Manifest struct {
	Format        string      `json:"format"` // "mango3s-backup"
	Version       int         `json:"version"`
	Created       time.Time   `json:"created"`
	Tool          string      `json:"tool"`
	ServerVersion string      `json:"serverVersion,omitempty"`
	Source        string      `json:"source,omitempty"` // connection name (never credentials)
	Gzip          bool        `json:"gzip"`
	Namespaces    []Namespace `json:"namespaces"`
	Notes         []string    `json:"notes,omitempty"`
}

// escapeName makes a collection name safe as a file name, like mongodump
// (percent-encoding path separators and percent signs).
func escapeName(name string) string {
	r := strings.NewReplacer("%", "%25", "/", "%2F", "\\", "%5C", ":", "%3A", "*", "%2A", "?", "%3F", "\"", "%22", "<", "%3C", ">", "%3E", "|", "%7C")
	return r.Replace(name)
}

func unescapeName(name string) string {
	if u, err := url.PathUnescape(name); err == nil {
		return u
	}
	return name
}

func ext(gz bool) string {
	if gz {
		return ".gz"
	}
	return ""
}

// fileWriter writes to a file, optionally gzipped, and fsyncs on Close.
type fileWriter struct {
	f  *os.File
	gz *gzip.Writer
	w  *bufio.Writer
	n  int64
}

func createFile(path string, gz bool) (*fileWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	fw := &fileWriter{f: f}
	var dst io.Writer = f
	if gz {
		fw.gz = gzip.NewWriter(f)
		dst = fw.gz
	}
	fw.w = bufio.NewWriterSize(dst, 256<<10)
	return fw, nil
}

func (w *fileWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

func (w *fileWriter) Close() error {
	err := w.w.Flush()
	if w.gz != nil {
		if e := w.gz.Close(); err == nil {
			err = e
		}
	}
	if e := w.f.Sync(); err == nil {
		err = e
	}
	if e := w.f.Close(); err == nil {
		err = e
	}
	return err
}

// openFile opens a dump file, transparently gunzipping .gz files.
func openFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(path, ".gz") {
		return f, nil
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s is not valid gzip: %w", filepath.Base(path), err)
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, closers{gz, f}}, nil
}

type closers []io.Closer

func (c closers) Close() error {
	var first error
	for _, x := range c {
		if err := x.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// docReader reads concatenated BSON documents from a .bson stream.
type docReader struct {
	r   *bufio.Reader
	buf []byte
	off int64
}

func newDocReader(r io.Reader) *docReader { return &docReader{r: bufio.NewReaderSize(r, 256<<10)} }

// Next returns the next raw document (valid until the next call), or io.EOF.
func (d *docReader) Next() (bson.Raw, error) {
	var head [4]byte
	if _, err := io.ReadFull(d.r, head[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("truncated document at byte %d: %w", d.off, err)
	}
	n := int(binary.LittleEndian.Uint32(head[:]))
	if n < 5 || n > MaxDocSize {
		return nil, fmt.Errorf("invalid document length %d at byte %d (corrupt or not a .bson file)", n, d.off)
	}
	if cap(d.buf) < n {
		d.buf = make([]byte, n)
	}
	d.buf = d.buf[:n]
	copy(d.buf, head[:])
	if _, err := io.ReadFull(d.r, d.buf[4:]); err != nil {
		return nil, fmt.Errorf("truncated document at byte %d: %w", d.off, err)
	}
	d.off += int64(n)
	raw := bson.Raw(d.buf)
	if err := raw.Validate(); err != nil {
		return nil, fmt.Errorf("corrupt document at byte %d: %w", d.off-int64(n), err)
	}
	return raw, nil
}
