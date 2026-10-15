package transfer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// JSONReader reads Extended JSON (canonical or relaxed) documents from either
// a top-level array or a stream of documents (NDJSON / mongoexport output).
type JSONReader struct {
	dec     *json.Decoder
	inArray bool
	started bool
	n       int
}

// NewJSONReader detects the layout from the first significant byte.
func NewJSONReader(r io.Reader) (*JSONReader, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	if b, err := br.Peek(3); err == nil && bytes.Equal(b, utf8BOM) {
		_, _ = br.Discard(3)
	}
	jr := &JSONReader{}
scan:
	for {
		b, err := br.Peek(1)
		if err == io.EOF {
			jr.dec = json.NewDecoder(br)
			return jr, nil
		}
		if err != nil {
			return nil, err
		}
		switch b[0] {
		case ' ', '\t', '\r', '\n':
			_, _ = br.Discard(1)
			continue
		case '[':
			jr.inArray = true
		case '{':
		default:
			return nil, fmt.Errorf("not a JSON document or array (starts with %q)", b[0])
		}
		break scan
	}
	jr.dec = json.NewDecoder(br)
	return jr, nil
}

// Position implements Reader.
func (j *JSONReader) Position() string {
	return fmt.Sprintf("document %d (byte %d)", j.n, j.dec.InputOffset())
}

// Next implements Reader.
func (j *JSONReader) Next() (bson.D, error) {
	if j.inArray && !j.started {
		if _, err := j.dec.Token(); err != nil { // '['
			return nil, err
		}
		j.started = true
	}
	if j.inArray && !j.dec.More() {
		if _, err := j.dec.Token(); err != nil { // ']'
			return nil, fmt.Errorf("unterminated array: %w", err)
		}
		return nil, io.EOF
	}
	var raw json.RawMessage
	if err := j.dec.Decode(&raw); err != nil {
		if err == io.EOF && !j.inArray {
			return nil, io.EOF
		}
		j.n++
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	j.n++
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("expected a document, got %s", truncate(string(raw), 40))
	}
	var d bson.D
	if err := bson.UnmarshalExtJSON(raw, false, &d); err != nil {
		return nil, fmt.Errorf("invalid Extended JSON: %w", err)
	}
	return d, nil
}

// JSONWriter writes Extended JSON documents.
type JSONWriter struct {
	w         *bufio.Writer
	mode      string
	canonical bool
	pretty    bool
	n         int
}

// NewJSONWriter writes to w. mode is JSONArray or JSONLines; canonical keeps
// every numeric type exact ({"$numberLong": ...}); relaxed is friendlier for
// other tools but folds int64/double into plain numbers.
func NewJSONWriter(w io.Writer, mode string, canonical, pretty bool) *JSONWriter {
	if mode != JSONLines {
		mode = JSONArray
	}
	if mode == JSONLines {
		pretty = false // one document per line
	}
	return &JSONWriter{w: bufio.NewWriterSize(w, 64<<10), mode: mode, canonical: canonical, pretty: pretty}
}

// Write implements Writer.
func (j *JSONWriter) Write(d bson.D) error {
	data, err := bson.MarshalExtJSON(d, j.canonical, false)
	if err != nil {
		return err
	}
	if j.pretty {
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "  ", "  "); err == nil {
			data = buf.Bytes()
		}
	}
	if j.mode == JSONArray {
		sep := ",\n  "
		if j.n == 0 {
			sep = "[\n  "
		}
		if _, err := j.w.WriteString(sep); err != nil {
			return err
		}
	}
	if _, err := j.w.Write(data); err != nil {
		return err
	}
	if j.mode == JSONLines {
		if err := j.w.WriteByte('\n'); err != nil {
			return err
		}
	}
	j.n++
	return nil
}

// Close implements Writer.
func (j *JSONWriter) Close() error {
	if j.mode == JSONArray {
		end := "\n]\n"
		if j.n == 0 {
			end = "[]\n"
		}
		if _, err := j.w.WriteString(end); err != nil {
			return err
		}
	}
	return j.w.Flush()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
