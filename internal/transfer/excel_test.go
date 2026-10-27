package transfer

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// Roadmap 6.1: a round trip through .xlsx keeps every supported type.
func TestExcelRoundTrip(t *testing.T) {
	oid := bson.NewObjectID()
	when := time.Date(2024, 1, 31, 13, 45, 6, 789_000_000, time.UTC)
	dec, _ := bson.ParseDecimal128("12.50")
	docs := []bson.D{
		{{Key: "_id", Value: oid}, {Key: "name", Value: "Ann"}, {Key: "n", Value: int32(7)}, {Key: "big", Value: int64(1) << 60},
			{Key: "price", Value: 0.1 + 0.2}, {Key: "ok", Value: true}, {Key: "at", Value: bson.NewDateTimeFromTime(when)},
			{Key: "addr", Value: bson.D{{Key: "city", Value: "Rome"}}}, {Key: "tags", Value: bson.A{"a", "b"}}, {Key: "dec", Value: dec}},
		{{Key: "_id", Value: bson.NewObjectID()}, {Key: "name", Value: "Bob"}, {Key: "n", Value: int32(-3)}, {Key: "ok", Value: false}},
	}
	fields := []string{"_id", "name", "n", "big", "price", "ok", "at", "addr.city", "tags", "dec"}
	var buf bytes.Buffer
	w, err := NewExcelWriter(&buf, "orders: 2024/01 [all]", fields, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if err := w.Write(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// The cells are typed in Excel itself.
	f, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	sheet := f.GetSheetList()[0]
	if sheet != "orders_ 2024_01 _all_" {
		t.Fatalf("sheet name %q", sheet)
	}
	// Numbers are stored without a type attribute, which means number.
	for cell, want := range map[string][]excelize.CellType{"C2": {excelize.CellTypeUnset, excelize.CellTypeNumber}, "E2": {excelize.CellTypeUnset, excelize.CellTypeNumber},
		"F2": {excelize.CellTypeBool}, "B2": {excelize.CellTypeSharedString, excelize.CellTypeInlineString}} {
		if got, _ := f.GetCellType(sheet, cell); !slices.Contains(want, got) {
			t.Errorf("%s: cell type %v, want one of %v", cell, got, want)
		}
	}
	if v, _ := f.GetCellValue(sheet, "G2"); v != "2024-01-31T13:45:07.000Z" {
		t.Errorf("the date shows as %q", v) // Excel shows whole seconds; the stored value keeps the milliseconds
	}
	f.Close()

	r, err := NewExcelReader(bytes.NewReader(buf.Bytes()), "", CSVOptions{Columns: []ColumnSpec{{Name: "tags", Type: TypeJSON}, {Name: "dec", Type: TypeDecimal}}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if strings.Join(r.Header(), ",") != strings.Join(fields, ",") {
		t.Fatalf("header %v", r.Header())
	}
	got, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	want := bson.D{{Key: "_id", Value: oid}, {Key: "name", Value: "Ann"}, {Key: "n", Value: int32(7)}, {Key: "big", Value: int64(1) << 60},
		{Key: "price", Value: 0.1 + 0.2}, {Key: "ok", Value: true}, {Key: "at", Value: bson.NewDateTimeFromTime(when)},
		{Key: "addr", Value: bson.D{{Key: "city", Value: "Rome"}}}, {Key: "tags", Value: bson.A{"a", "b"}}, {Key: "dec", Value: dec}}
	if !mql.Equal(got, want) {
		t.Fatalf("round trip\n got %s\nwant %s", mql.Compact(got), mql.Compact(want))
	}
	second, err := r.Next()
	if err != nil || len(second) != 4 {
		t.Fatalf("missing fields stay missing: %s %v", mql.Compact(second), err)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestExcelLimits(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewExcelWriter(&buf, "x", []string{"s"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(bson.D{{Key: "s", Value: strings.Repeat("x", maxCellChars+1)}}); err == nil || !strings.Contains(err.Error(), "Excel's limit") {
		t.Fatalf("an over-long cell: %v", err)
	}
	if _, err := NewExcelReader(strings.NewReader("not a workbook"), "", CSVOptions{}); err == nil {
		t.Fatal("garbage accepted")
	}
	if got := SheetName(strings.Repeat("é", 40)); len([]rune(got)) != 31 {
		t.Fatalf("sheet name length %d", len([]rune(got)))
	}
	if DetectFormat("/tmp/People.XLSX") != FormatExcel {
		t.Fatal("xlsx not detected")
	}
}
