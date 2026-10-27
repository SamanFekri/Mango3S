// Package transfer imports and exports collections through streaming format
// adapters. Readers yield one document at a time and writers consume one at a
// time, so memory stays bounded regardless of dataset size.
package transfer

import (
	"io"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Format identifies a file format.
type Format string

const (
	FormatJSON  Format = "json" // a JSON array, or newline-delimited / concatenated documents
	FormatCSV   Format = "csv"
	FormatExcel Format = "xlsx" // one sheet, a header row and one column per field
)

// JSON output layouts.
const (
	JSONArray = "array"
	JSONLines = "lines"
)

// DetectFormat guesses the format from the file extension.
func DetectFormat(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".tsv", ".txt":
		return FormatCSV
	case ".xlsx":
		return FormatExcel
	}
	return FormatJSON
}

// Reader yields documents; Next returns io.EOF when exhausted.
type Reader interface {
	Next() (bson.D, error)
	// Position describes the current location for error messages ("row 12").
	Position() string
}

// Writer consumes documents. Close flushes and finishes the output.
type Writer interface {
	Write(bson.D) error
	Close() error
}

// countingReader tracks bytes consumed for progress reporting.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}
