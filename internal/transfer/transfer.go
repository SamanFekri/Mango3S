package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/jobs"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/query"
)

// Import modes.
const (
	ModeInsert = "insert" // insertMany; duplicates are reported as failures
	ModeUpsert = "upsert" // replace by _id, insert when missing
)

// ImportOptions describes an import.
type ImportOptions struct {
	ConnID      string       `json:"connId"`
	DB          string       `json:"db"`
	Coll        string       `json:"coll"`
	Path        string       `json:"path"`
	Format      Format       `json:"format"` // "" = by extension
	Delimiter   string       `json:"delimiter"`
	Sheet       string       `json:"sheet,omitempty"` // Excel: the sheet to read ("" = the first)
	Columns     []ColumnSpec `json:"columns"`
	KeepBlanks  bool         `json:"keepBlanks"`
	Mode        string       `json:"mode"`
	DropFirst   bool         `json:"dropFirst"`
	StopOnError bool         `json:"stopOnError"`
	BatchSize   int          `json:"batchSize"`
}

// ExportOptions describes an export. The query fields mirror query.Request.
type ExportOptions struct {
	ConnID     string   `json:"connId"`
	DB         string   `json:"db"`
	Coll       string   `json:"coll"`
	Filter     string   `json:"filter"`
	Projection string   `json:"projection"`
	Sort       string   `json:"sort"`
	Skip       int64    `json:"skip"`
	Limit      int64    `json:"limit"`
	Path       string   `json:"path"`
	Format     Format   `json:"format"`
	JSONMode   string   `json:"jsonMode"`  // array | lines
	Canonical  bool     `json:"canonical"` // canonical Extended JSON keeps all numeric types
	Pretty     bool     `json:"pretty"`
	Delimiter  string   `json:"delimiter"`
	NoHeader   bool     `json:"noHeader"`
	Fields     []string `json:"fields"` // selected fields; CSV columns
	// Pipeline, when set, exports the aggregation's results instead of a
	// find; Filter, Projection, Sort, Skip, Limit and Fields are then ignored
	// (put them in the pipeline). A pipeline ending in $out or $merge is
	// refused: it writes its own results.
	Pipeline     []aggregate.Stage `json:"pipeline,omitempty"`
	AllowDiskUse bool              `json:"allowDiskUse,omitempty"`
}

// ColumnPreview describes a CSV column in a preview.
type ColumnPreview struct {
	Name     string   `json:"name"`
	Inferred string   `json:"inferred"`
	Samples  []string `json:"samples"`
}

// Preview is the first part of a file, parsed with the given options.
type Preview struct {
	Format    Format           `json:"format"`
	SizeBytes int64            `json:"sizeBytes"`
	Documents []model.Document `json:"documents"`
	Columns   []ColumnPreview  `json:"columns"`
	Sheets    []string         `json:"sheets,omitempty"` // Excel: the workbook's sheets
	Error     string           `json:"error,omitempty"`
}

func openReader(f io.Reader, o ImportOptions) (Reader, error) {
	switch o.Format {
	case FormatCSV:
		d, err := ParseDelimiter(o.Delimiter)
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		r, err := NewCSVReader(f, CSVOptions{Delimiter: d, Columns: o.Columns, KeepBlanks: o.KeepBlanks})
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		return r, nil
	case FormatExcel:
		r, err := NewExcelReader(f, o.Sheet, CSVOptions{Columns: o.Columns, KeepBlanks: o.KeepBlanks})
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		return r, nil
	case FormatJSON:
		r, err := NewJSONReader(f)
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		return r, nil
	}
	return nil, apperr.Invalid("Unsupported format %q.", o.Format)
}

func normalizeImport(o *ImportOptions) error {
	if o.Path == "" {
		return apperr.Invalid("Choose a file to import.")
	}
	if o.Format == "" {
		o.Format = DetectFormat(o.Path)
	}
	if o.Mode == "" {
		o.Mode = ModeInsert
	}
	if o.Mode != ModeInsert && o.Mode != ModeUpsert {
		return apperr.Invalid("Unknown import mode %q.", o.Mode)
	}
	if o.BatchSize <= 0 || o.BatchSize > 10000 {
		o.BatchSize = 1000
	}
	return nil
}

// PreviewFile parses up to n documents (and, for CSV, up to 200 rows for type
// inference). Parse errors are returned inside the preview so the wizard can
// show them next to the documents that did parse.
func PreviewFile(o ImportOptions, n int) (Preview, error) {
	if err := normalizeImport(&o); err != nil {
		return Preview{}, err
	}
	f, err := os.Open(o.Path)
	if err != nil {
		return Preview{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Cannot open the file.")
	}
	defer f.Close()
	info, _ := f.Stat()
	p := Preview{Format: o.Format, Documents: []model.Document{}, Columns: []ColumnPreview{}}
	if info != nil {
		p.SizeBytes = info.Size()
	}

	if o.Format == FormatCSV || o.Format == FormatExcel {
		cols, err := previewColumns(o)
		if err != nil {
			return Preview{}, err
		}
		p.Columns = cols
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Preview{}, err
		}
	}
	r, err := openReader(f, o)
	if err != nil {
		return Preview{}, err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	if x, ok := r.(*ExcelReader); ok {
		p.Sheets = x.Sheets
	}
	for len(p.Documents) < n {
		d, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.Error = fmt.Sprintf("%s: %v", r.Position(), err)
			break
		}
		p.Documents = append(p.Documents, model.NewDocument(d))
	}
	return p, nil
}

func previewColumns(o ImportOptions) ([]ColumnPreview, error) {
	f, err := os.Open(o.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r *CSVReader
	if o.Format == FormatExcel {
		x, err := NewExcelReader(f, o.Sheet, CSVOptions{})
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		defer x.Close()
		r = x.CSVReader
	} else {
		d, err := ParseDelimiter(o.Delimiter)
		if err != nil {
			return nil, apperr.Invalid("%v", err)
		}
		if r, err = NewCSVReader(f, CSVOptions{Delimiter: d}); err != nil {
			return nil, apperr.Invalid("%v", err)
		}
	}
	cols := make([]ColumnPreview, len(r.Header()))
	types := make([]map[string]bool, len(cols))
	for i, h := range r.Header() {
		cols[i] = ColumnPreview{Name: h, Samples: []string{}}
		types[i] = map[string]bool{}
	}
	for row := 0; row < 200; row++ {
		rec, err := r.NextRecord()
		if err != nil {
			break
		}
		for i, cell := range rec {
			if i >= len(cols) {
				break
			}
			if t := InferType(cell); t != "" {
				types[i][t] = true
			}
			if len(cols[i].Samples) < 3 && cell != "" {
				cols[i].Samples = append(cols[i].Samples, truncate(cell, 40))
			}
		}
	}
	for i := range cols {
		cols[i].Inferred = mergeTypes(types[i])
	}
	return cols, nil
}

// mergeTypes picks one type for a column from the types seen in its cells.
func mergeTypes(seen map[string]bool) string {
	switch len(seen) {
	case 0:
		return TypeString
	case 1:
		for t := range seen {
			return t
		}
	}
	numeric := true
	for t := range seen {
		if t != TypeInt && t != TypeLong && t != TypeDouble {
			numeric = false
		}
	}
	if numeric {
		if seen[TypeDouble] {
			return TypeDouble
		}
		return TypeLong
	}
	return TypeAuto // mixed: convert cell by cell
}

// Service runs imports and exports as background jobs.
type Service struct {
	src  mongox.Source
	jobs *jobs.Manager
}

// New returns a transfer service.
func New(src mongox.Source, jm *jobs.Manager) *Service { return &Service{src: src, jobs: jm} }

// StartImport validates the options and queues the import job.
func (s *Service) StartImport(o ImportOptions) (jobs.Job, error) {
	if err := normalizeImport(&o); err != nil {
		return jobs.Job{}, err
	}
	sess, c, err := mongox.Collection(s.src, o.ConnID, o.DB, o.Coll)
	if err != nil {
		return jobs.Job{}, err
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, err
	}
	if _, err := os.Stat(o.Path); err != nil {
		return jobs.Job{}, apperr.Wrap(apperr.CodeInvalidInput, err, "Cannot read the file.")
	}
	title := fmt.Sprintf("Import %s → %s.%s", filepath.Base(o.Path), o.DB, o.Coll)
	return s.jobs.Start("import", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		return RunImport(ctx, c, o, p)
	}), nil
}

// ImportStats summarizes an import.
type ImportStats struct {
	Read, Inserted, Upserted, Replaced, Failed int64
	Errors                                     []string
}

func (st ImportStats) String() string {
	parts := []string{fmt.Sprintf("%d read", st.Read)}
	if st.Inserted > 0 {
		parts = append(parts, fmt.Sprintf("%d inserted", st.Inserted))
	}
	if st.Upserted > 0 {
		parts = append(parts, fmt.Sprintf("%d upserted", st.Upserted))
	}
	if st.Replaced > 0 {
		parts = append(parts, fmt.Sprintf("%d replaced", st.Replaced))
	}
	if st.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", st.Failed))
	}
	s := strings.Join(parts, ", ")
	if len(st.Errors) > 0 {
		s += ". First error: " + st.Errors[0]
	}
	return s
}

// RunImport streams the file into the collection in batches. p may be nil.
func RunImport(ctx context.Context, c *mongo.Collection, o ImportOptions, p *jobs.Progress) (string, error) {
	if err := normalizeImport(&o); err != nil {
		return "", err
	}
	f, err := os.Open(o.Path)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInvalidInput, err, "Cannot open the file.")
	}
	defer f.Close()
	if p != nil {
		if info, err := f.Stat(); err == nil {
			p.SetTotal(info.Size(), "bytes")
		}
	}
	counter := &countingReader{r: f}
	r, err := openReader(counter, o)
	if err != nil {
		return "", err
	}
	if c, ok := r.(io.Closer); ok {
		defer c.Close()
	}
	if o.DropFirst {
		if err := c.Drop(ctx); err != nil {
			return "", apperr.From(err)
		}
	}

	st := &ImportStats{}
	batch := make([]bson.D, 0, o.BatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := writeBatch(ctx, c, o.Mode, batch, st)
		batch = batch[:0]
		if p != nil {
			p.Set(counter.n)
			p.Message("%s", st.String())
		}
		if err != nil {
			return err
		}
		if o.StopOnError && st.Failed > 0 {
			return apperr.New(apperr.CodeMongo, "Stopped after a write error: %s", st.Errors[0])
		}
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return st.String(), err
		}
		d, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			msg := fmt.Sprintf("%s: %v", r.Position(), err)
			if o.StopOnError || o.Format == FormatJSON {
				// A JSON syntax error leaves the decoder in an unknown state, so it is always fatal.
				_ = flush()
				return st.String(), apperr.Invalid("Import stopped at %s", msg)
			}
			st.Failed++
			if len(st.Errors) < 20 {
				st.Errors = append(st.Errors, msg)
			}
			continue
		}
		st.Read++
		batch = append(batch, d)
		if len(batch) >= o.BatchSize {
			if err := flush(); err != nil {
				return st.String(), err
			}
		}
	}
	if err := flush(); err != nil {
		return st.String(), err
	}
	if st.Read > 0 && st.Inserted+st.Upserted+st.Replaced == 0 && st.Failed > 0 {
		return st.String(), apperr.New(apperr.CodeMongo, "No documents were imported: %s", st.Errors[0])
	}
	return st.String(), nil
}

func writeBatch(ctx context.Context, c *mongo.Collection, mode string, batch []bson.D, st *ImportStats) error {
	var res *mongo.BulkWriteResult
	var err error
	if mode == ModeInsert {
		docs := make([]any, len(batch))
		for i, d := range batch {
			docs[i] = d
		}
		_, err = c.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
		if err == nil {
			st.Inserted += int64(len(batch))
		}
	} else {
		models := make([]mongo.WriteModel, len(batch))
		for i, d := range batch {
			if id, ok := lookupID(d); ok {
				models[i] = mongo.NewReplaceOneModel().SetFilter(bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}}}).SetReplacement(d).SetUpsert(true)
			} else {
				models[i] = mongo.NewInsertOneModel().SetDocument(d)
			}
		}
		res, err = c.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		if res != nil {
			st.Inserted += res.InsertedCount
			st.Upserted += res.UpsertedCount
			st.Replaced += res.MatchedCount
		}
	}
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) && len(bwe.WriteErrors) > 0 {
		if mode == ModeInsert {
			// Unordered: everything except the reported documents was written.
			st.Inserted += int64(len(batch) - len(bwe.WriteErrors))
		}
		st.Failed += int64(len(bwe.WriteErrors))
		for _, we := range bwe.WriteErrors {
			if len(st.Errors) < 20 {
				st.Errors = append(st.Errors, firstLine(we.Message))
			}
		}
		if bwe.WriteConcernError == nil {
			return nil // per-document failures; the rest of the batch was written
		}
	}
	return apperr.From(err)
}

func lookupID(d bson.D) (any, bool) {
	for _, e := range d {
		if e.Key == "_id" {
			return e.Value, true
		}
	}
	return nil, false
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// StartExport validates the options and queues the export job.
func (s *Service) StartExport(o ExportOptions) (jobs.Job, error) {
	if err := normalizeExport(&o); err != nil {
		return jobs.Job{}, err
	}
	_, c, err := mongox.Collection(s.src, o.ConnID, o.DB, o.Coll)
	if err != nil {
		return jobs.Job{}, err
	}
	if _, err := query.Parse(exportQuery(o)); err != nil {
		return jobs.Job{}, err
	}
	title := fmt.Sprintf("Export %s.%s → %s", o.DB, o.Coll, filepath.Base(o.Path))
	return s.jobs.Start("export", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		return RunExport(ctx, c, o, p)
	}), nil
}

func normalizeExport(o *ExportOptions) error {
	if o.Path == "" {
		return apperr.Invalid("Choose a destination file.")
	}
	if o.Format == "" {
		o.Format = DetectFormat(o.Path)
	}
	if o.Format != FormatJSON && o.Format != FormatCSV && o.Format != FormatExcel {
		return apperr.Invalid("Unsupported format %q.", o.Format)
	}
	if _, err := ParseDelimiter(o.Delimiter); err != nil {
		return apperr.Invalid("%v", err)
	}
	if len(o.Pipeline) > 0 {
		pipe, err := aggregate.Build(o.Pipeline, -1)
		if err != nil {
			return err
		}
		if aggregate.IsWrite(pipe) {
			return apperr.Invalid("This pipeline ends with %s, which writes its own results. Remove that stage to export the results to a file.", pipe[len(pipe)-1][0].Key)
		}
	}
	dir := filepath.Dir(o.Path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return apperr.Invalid("The folder %q does not exist.", dir)
	}
	return nil
}

func exportQuery(o ExportOptions) query.Request {
	return query.Request{ConnID: o.ConnID, DB: o.DB, Coll: o.Coll, Filter: o.Filter, Projection: o.Projection, Sort: o.Sort, Skip: o.Skip, Limit: o.Limit}
}

// fieldsProjection builds a projection from selected fields; _id is excluded
// unless selected. Overlapping paths ("a" and "a.b") collapse to the parent.
func fieldsProjection(fields []string) bson.D {
	proj := bson.D{}
	hasID := false
	for _, f := range fields {
		if f == "_id" {
			hasID = true
		}
		covered := false
		for _, g := range fields {
			if g != f && strings.HasPrefix(f, g+".") {
				covered = true
			}
		}
		if !covered {
			proj = append(proj, bson.E{Key: f, Value: 1})
		}
	}
	if !hasID {
		proj = append(proj, bson.E{Key: "_id", Value: 0})
	}
	return proj
}

// RunExport streams query results into the file. It writes to a temporary
// file and renames it on success, so a failed or canceled export never leaves
// a truncated file behind.
func RunExport(ctx context.Context, c *mongo.Collection, o ExportOptions, p *jobs.Progress) (string, error) {
	if err := normalizeExport(&o); err != nil {
		return "", err
	}
	src, err := newExportSource(ctx, c, o)
	if err != nil {
		return "", err
	}
	if p != nil {
		// An aggregation's result count is unknown until it has run (0).
		total, err := src.count(ctx)
		if err != nil {
			return "", err
		}
		p.SetTotal(total, "documents")
	}

	fields := o.Fields
	if len(o.Pipeline) > 0 {
		fields = nil
	}
	if (o.Format == FormatCSV || o.Format == FormatExcel) && len(fields) == 0 {
		if fields, err = src.sampleLeafPaths(ctx, 1000); err != nil {
			return "", err
		}
		if len(fields) == 0 {
			fields = []string{"_id"}
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(o.Path), "."+filepath.Base(o.Path)+".*.tmp")
	if err != nil {
		return "", apperr.Wrap(apperr.CodeInvalidInput, err, "Cannot create the output file.")
	}
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	var w Writer
	switch o.Format {
	case FormatCSV:
		delim, _ := ParseDelimiter(o.Delimiter)
		if w, err = NewCSVWriter(tmp, fields, delim, !o.NoHeader); err != nil {
			return "", apperr.Invalid("%v", err)
		}
	case FormatExcel:
		if w, err = NewExcelWriter(tmp, o.Coll, fields, !o.NoHeader); err != nil {
			return "", apperr.Invalid("%v", err)
		}
	default:
		w = NewJSONWriter(tmp, o.JSONMode, o.Canonical, o.Pretty)
	}

	cur, err := src.open(ctx, 0)
	if err != nil {
		return "", err
	}
	defer cur.Close(context.Background())

	var n int64
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return "", apperr.From(err)
		}
		if err := w.Write(d); err != nil {
			return "", apperr.Wrap(apperr.CodeInternal, err, "Writing the file failed.")
		}
		n++
		if p != nil {
			p.Set(n)
		}
	}
	if err := cur.Err(); err != nil {
		return "", apperr.From(err)
	}
	if err := w.Close(); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "Writing the file failed.")
	}
	if err := tmp.Close(); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "Writing the file failed.")
	}
	if err := os.Rename(tmp.Name(), o.Path); err != nil {
		return "", apperr.Wrap(apperr.CodeInternal, err, "Could not move the export into place.")
	}
	committed = true
	return fmt.Sprintf("%d documents written to %s", n, filepath.Base(o.Path)), nil
}

// exportSource is what an export reads: a find with the export's query, or
// an aggregation pipeline.
type exportSource struct {
	c        *mongo.Collection
	o        ExportOptions
	q        query.Parsed
	pipeline mongo.Pipeline
}

func newExportSource(ctx context.Context, c *mongo.Collection, o ExportOptions) (*exportSource, error) {
	s := &exportSource{c: c, o: o}
	if len(o.Pipeline) > 0 {
		pipe, err := aggregate.Build(o.Pipeline, -1)
		if err != nil {
			return nil, err
		}
		s.pipeline = pipe
		return s, nil
	}
	q, err := query.Parse(exportQuery(o))
	if err != nil {
		return nil, err
	}
	if len(o.Fields) > 0 && len(q.Projection) == 0 {
		q.Projection = fieldsProjection(o.Fields)
	}
	s.q = q
	return s, nil
}

// count is the number of documents a find export writes; 0 (unknown) for a
// pipeline.
func (s *exportSource) count(ctx context.Context) (int64, error) {
	if s.pipeline != nil {
		return 0, nil
	}
	opts := options.Count()
	if s.o.Skip > 0 {
		opts.SetSkip(s.o.Skip)
	}
	if s.o.Limit > 0 {
		opts.SetLimit(s.o.Limit)
	}
	n, err := s.c.CountDocuments(ctx, s.q.Filter, opts)
	return n, apperr.Check(err)
}

// open returns a cursor over the documents to write; limit > 0 caps it (for
// sampling CSV columns).
func (s *exportSource) open(ctx context.Context, limit int64) (*mongo.Cursor, error) {
	if s.pipeline != nil {
		pipe := s.pipeline
		if limit > 0 {
			pipe = append(append(mongo.Pipeline{}, pipe...), bson.D{{Key: "$limit", Value: limit}})
		}
		cur, err := s.c.Aggregate(ctx, pipe, options.Aggregate().SetAllowDiskUse(s.o.AllowDiskUse).SetBatchSize(1000))
		return cur, apperr.Check(err)
	}
	opts := options.Find().SetBatchSize(1000)
	if len(s.q.Projection) > 0 {
		opts.SetProjection(s.q.Projection)
	}
	if limit > 0 {
		opts.SetLimit(limit)
	} else {
		if len(s.q.Sort) > 0 {
			opts.SetSort(s.q.Sort)
		}
		if s.o.Skip > 0 {
			opts.SetSkip(s.o.Skip)
		}
		if s.o.Limit > 0 {
			opts.SetLimit(s.o.Limit)
		}
	}
	cur, err := s.c.Find(ctx, s.q.Filter, opts)
	return cur, apperr.Check(err)
}

func (s *exportSource) sampleLeafPaths(ctx context.Context, n int64) ([]string, error) {
	cur, err := s.open(ctx, n)
	if err != nil {
		return nil, err
	}
	defer cur.Close(context.Background())
	seen := map[string]bool{}
	var out []string
	for cur.Next(ctx) {
		var d bson.D
		if cur.Decode(&d) == nil {
			LeafPaths(d, "", seen, &out)
		}
	}
	return out, apperr.Check(cur.Err())
}
