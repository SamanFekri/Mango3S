package transfer

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Excel's limits: rows per sheet (the header takes one) and characters per cell.
const (
	MaxExcelRows = 1_048_575
	maxCellChars = 32_767
)

// excelDateFormat shows dates as ISO timestamps in UTC, which the importer
// (InferType) recognizes as dates again.
const excelDateFormat = `yyyy-mm-dd"T"hh:mm:ss.000"Z"`

// ---- reading ---------------------------------------------------------------

// sheetRows feeds an Excel sheet's rows to the CSV reader as text, so
// imports use the same column types and conversions as CSV. It walks the
// sheet twice in step: the text Excel shows, and the stored values. Numbers
// and dates take the stored value, since the shown one is rounded (Excel
// shows 0.1+0.2 as 0.3, and dates to the second at best).
type sheetRows struct{ shown, raw *excelize.Rows }

func (s sheetRows) Read() ([]string, error) {
	if !s.shown.Next() || !s.raw.Next() {
		if err := s.shown.Error(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	cells, err := s.shown.Columns()
	if err != nil {
		return nil, err
	}
	raw, err := s.raw.Columns(excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}
	for i, c := range cells {
		var r string
		if i < len(raw) {
			r = raw[i]
		}
		cells[i] = cellText(c, r)
	}
	return cells, nil
}

// cellText picks the text to import for a cell shown as shown and stored as raw.
func cellText(shown, raw string) string {
	switch {
	case shown == "TRUE" || shown == "FALSE": // Excel's booleans
		return strings.ToLower(shown)
	case raw == "" || raw == shown || !floatRe.MatchString(raw):
		return shown
	case dateTimeRe.MatchString(strings.TrimSpace(shown)):
		if serial, err := strconv.ParseFloat(raw, 64); err == nil {
			return FormatCell(excelSerialTime(serial))
		}
	case intRe.MatchString(shown) || floatRe.MatchString(shown):
		return raw
	}
	return shown
}

// excelSerialTime converts an Excel date serial (days since 1899-12-30, in
// the 1900 date system) to a time, to the millisecond.
func excelSerialTime(serial float64) time.Time {
	ms := int64(math.Round(serial * 86_400_000))
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(ms) * time.Millisecond)
}

// ExcelReader reads one sheet of a workbook with a header row; see CSVReader.
type ExcelReader struct {
	*CSVReader
	Sheet  string
	Sheets []string
	f      *excelize.File
	rows   []*excelize.Rows
}

// NewExcelReader opens a workbook and reads sheet ("" = the first one).
func NewExcelReader(r io.Reader, sheet string, opts CSVOptions) (*ExcelReader, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("not a readable .xlsx workbook: %w", err)
	}
	sheets := f.GetSheetList()
	switch {
	case len(sheets) == 0:
		f.Close()
		return nil, fmt.Errorf("the workbook has no sheets")
	case sheet == "":
		sheet = sheets[0]
	case !slices.Contains(sheets, sheet):
		f.Close()
		return nil, fmt.Errorf("the workbook has no sheet %q", sheet)
	}
	shown, err := f.Rows(sheet)
	if err != nil {
		f.Close()
		return nil, err
	}
	raw, err := f.Rows(sheet)
	if err != nil {
		shown.Close()
		f.Close()
		return nil, err
	}
	x := &ExcelReader{Sheet: sheet, Sheets: sheets, f: f, rows: []*excelize.Rows{shown, raw}}
	if x.CSVReader, err = newTableReader(sheetRows{shown: shown, raw: raw}, opts); err != nil {
		x.Close()
		return nil, err
	}
	return x, nil
}

// Position implements Reader.
func (x *ExcelReader) Position() string { return fmt.Sprintf("sheet %q row %d", x.Sheet, x.row) }

// Close frees the workbook.
func (x *ExcelReader) Close() error {
	for _, r := range x.rows {
		r.Close()
	}
	return x.f.Close()
}

// ---- writing ---------------------------------------------------------------

// ExcelWriter writes documents as rows of one sheet, one column per field.
// Numbers, booleans and dates become typed cells; other values are written
// as text, like in CSV (sub-documents and arrays as Extended JSON).
type ExcelWriter struct {
	out       io.Writer
	f         *excelize.File
	sw        *excelize.StreamWriter
	fields    []string
	row       int
	dateStyle int
	cells     []any
}

// SheetName makes a valid sheet name: at most 31 characters, none of : \ / ? * [ ].
func SheetName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:\/?*[]`, r) {
			return '_'
		}
		return r
	}, strings.Trim(s, "'"))
	if r := []rune(s); len(r) > 31 {
		s = string(r[:31])
	}
	if s == "" {
		s = "Sheet1"
	}
	return s
}

// NewExcelWriter starts a workbook with one sheet; it is written to w on Close.
func NewExcelWriter(w io.Writer, sheet string, fields []string, header bool) (*ExcelWriter, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("Excel export needs at least one field")
	}
	f := excelize.NewFile()
	name := SheetName(sheet)
	if err := f.SetSheetName("Sheet1", name); err != nil {
		f.Close()
		return nil, err
	}
	sw, err := f.NewStreamWriter(name)
	if err != nil {
		f.Close()
		return nil, err
	}
	numFmt := excelDateFormat
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})
	if err != nil {
		f.Close()
		return nil, err
	}
	x := &ExcelWriter{out: w, f: f, sw: sw, fields: fields, row: 1, dateStyle: dateStyle, cells: make([]any, len(fields))}
	if header {
		bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		if err != nil {
			f.Close()
			return nil, err
		}
		if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			f.Close()
			return nil, err
		}
		head := make([]any, len(fields))
		for i, name := range fields {
			head[i] = excelize.Cell{Value: name, StyleID: bold}
		}
		if err := sw.SetRow("A1", head); err != nil {
			f.Close()
			return nil, err
		}
		x.row = 2
	}
	return x, nil
}

// Write implements Writer.
func (x *ExcelWriter) Write(d bson.D) error {
	if x.row > MaxExcelRows+1 {
		return fmt.Errorf("an Excel sheet holds at most %d rows; filter or limit the export, or use CSV or JSON", MaxExcelRows)
	}
	for i, f := range x.fields {
		v, ok := LookupPath(d, f)
		if !ok {
			x.cells[i] = nil
			continue
		}
		c, err := x.cell(v)
		if err != nil {
			return fmt.Errorf("field %q: %w", f, err)
		}
		x.cells[i] = c
	}
	cell, err := excelize.CoordinatesToCellName(1, x.row)
	if err != nil {
		return err
	}
	x.row++
	return x.sw.SetRow(cell, x.cells)
}

// cell converts a value to what the sheet stores.
func (x *ExcelWriter) cell(v any) (any, error) {
	switch t := v.(type) {
	case nil, bson.Null, bson.Undefined:
		return nil, nil
	case bool:
		return t, nil
	case int32:
		return int64(t), nil
	case int64:
		if t > 1<<53 || t < -(1<<53) {
			return FormatCell(t), nil // beyond a double's exact range: keep every digit as text
		}
		return t, nil
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return FormatCell(t), nil
		}
		return t, nil
	case bson.DateTime:
		return x.date(t.Time())
	case time.Time:
		return x.date(t)
	}
	s := FormatCell(v)
	if len([]rune(s)) > maxCellChars {
		return nil, fmt.Errorf("a value is longer than %d characters, Excel's limit for a cell; export as JSON or CSV instead", maxCellChars)
	}
	return s, nil
}

func (x *ExcelWriter) date(t time.Time) (any, error) {
	t = t.UTC()
	if t.Year() < 1900 || t.Year() > 9999 { // outside what Excel can show as a date
		return FormatCell(t), nil
	}
	return excelize.Cell{Value: t, StyleID: x.dateStyle}, nil
}

// Close implements Writer: it finishes the sheet and writes the workbook.
func (x *ExcelWriter) Close() error {
	defer x.f.Close()
	if err := x.sw.Flush(); err != nil {
		return err
	}
	_, err := x.f.WriteTo(x.out)
	return err
}
