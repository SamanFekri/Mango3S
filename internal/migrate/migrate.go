package migrate

import (
	"context"
	"database/sql"
	"fmt"
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
	"mango3s/internal/docpath"
	"mango3s/internal/jobs"
	"mango3s/internal/logging"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// ID strategies.
const (
	IDPrimaryKey = "pk"       // _id = primary key value (or a document of a composite key)
	IDObjectID   = "objectId" // new ObjectId; key columns stay as ordinary fields
)

// ColumnMapping maps one column to a (possibly dotted) field.
type ColumnMapping struct {
	Column string `json:"column"`
	Field  string `json:"field"`
	Type   string `json:"type"`
}

// TableMapping maps one table to one collection.
type TableMapping struct {
	Table      string          `json:"table"`
	Collection string          `json:"collection"`
	IDStrategy string          `json:"idStrategy"`
	Columns    []ColumnMapping `json:"columns"`
	// Skip copying the table itself (e.g. a child that is only embedded).
	EmbedOnly bool `json:"embedOnly"`
}

// Embed puts child rows into their parent documents as an array.
type Embed struct {
	Child        string   `json:"child"`        // child table (must have a TableMapping for its columns)
	Parent       string   `json:"parent"`       // parent table; its mapping must use the pk id strategy
	ChildColumns []string `json:"childColumns"` // foreign key columns in the child
	As           string   `json:"as"`           // array field in the parent
}

// Request describes a migration.
type Request struct {
	Source     Source         `json:"source"`
	ConnID     string         `json:"connId"`
	DB         string         `json:"db"`
	Tables     []TableMapping `json:"tables"`
	Embeds     []Embed        `json:"embeds"`
	OnRowError string         `json:"onRowError"` // fail (default) | skip
	BatchSize  int            `json:"batchSize"`
}

// TableReport is the result for one table.
type TableReport struct {
	Table            string   `json:"table"`
	Collection       string   `json:"collection"`
	Rows             int64    `json:"rows"`
	Inserted         int64    `json:"inserted"`
	Skipped          int64    `json:"skipped"`
	Errors           []string `json:"errors"`
	CountOK          bool     `json:"countOk"`
	SampleChecked    int      `json:"sampleChecked"`
	SampleMismatches []string `json:"sampleMismatches"`
}

// EmbedReport is the result for one embedding.
type EmbedReport struct {
	Child     string `json:"child"`
	Parent    string `json:"parent"`
	As        string `json:"as"`
	ChildRows int64  `json:"childRows"`
	Embedded  int64  `json:"embedded"`
	Orphans   int64  `json:"orphans"` // child rows whose parent does not exist
	CountOK   bool   `json:"countOk"`
}

// Report is the outcome of a migration.
type Report struct {
	RunID      string        `json:"runId"`
	Tables     []TableReport `json:"tables"`
	Embeds     []EmbedReport `json:"embeds"`
	Valid      bool          `json:"valid"`
	DurationMs int64         `json:"durationMs"`
}

// Run is a persisted record of a migration, kept for rollback.
type Run struct {
	ID                 string    `json:"id"`
	Created            time.Time `json:"created"`
	Source             string    `json:"source"` // credential-free description
	ConnID             string    `json:"connId"`
	DB                 string    `json:"db"`
	CreatedCollections []string  `json:"createdCollections"`
	State              string    `json:"state"` // running | succeeded | failed | rolledBack
	Summary            string    `json:"summary"`
	Report             *Report   `json:"report,omitempty"`
}

func (r Request) mapping(table string) *TableMapping {
	for i := range r.Tables {
		if r.Tables[i].Table == table {
			return &r.Tables[i]
		}
	}
	return nil
}

// Validate checks the request against the introspected schema.
func (r *Request) Validate(tables []Table) error {
	if r.ConnID == "" || strings.TrimSpace(r.DB) == "" {
		return apperr.Invalid("Choose a target connection and database.")
	}
	if len(r.Tables) == 0 {
		return apperr.Invalid("Select at least one table.")
	}
	if r.OnRowError == "" {
		r.OnRowError = "fail"
	}
	if r.BatchSize <= 0 || r.BatchSize > 5000 {
		r.BatchSize = 1000
	}
	byName := map[string]Table{}
	for _, t := range tables {
		byName[t.Name] = t
	}
	colls := map[string]string{}
	for i := range r.Tables {
		m := &r.Tables[i]
		t, ok := byName[m.Table]
		if !ok {
			return apperr.Invalid("Table %q does not exist in the source.", m.Table)
		}
		if m.IDStrategy == "" {
			m.IDStrategy = IDPrimaryKey
		}
		if m.IDStrategy == IDPrimaryKey && len(t.PrimaryKey) == 0 {
			return apperr.Invalid("Table %s has no primary key; use the objectId strategy.", m.Table)
		}
		if !m.EmbedOnly {
			if m.Collection == "" || strings.ContainsAny(m.Collection, "$\x00") || strings.HasPrefix(m.Collection, "system.") {
				return apperr.Invalid("Table %s: invalid collection name %q.", m.Table, m.Collection)
			}
			if prev, dup := colls[m.Collection]; dup {
				return apperr.Invalid("Tables %s and %s map to the same collection %s.", prev, m.Table, m.Collection)
			}
			colls[m.Collection] = m.Table
		}
		if len(m.Columns) == 0 {
			for _, c := range t.Columns {
				m.Columns = append(m.Columns, ColumnMapping{Column: c.Name, Field: c.Name, Type: c.Suggest})
			}
		}
		fields := map[string]bool{}
		known := map[string]bool{}
		for _, c := range t.Columns {
			known[c.Name] = true
		}
		for j := range m.Columns {
			cm := &m.Columns[j]
			if !known[cm.Column] {
				return apperr.Invalid("Table %s has no column %q.", m.Table, cm.Column)
			}
			if cm.Type == "" {
				cm.Type = TAuto
			}
			if !contains(TargetTypes, cm.Type) {
				return apperr.Invalid("%s.%s: unknown type %q.", m.Table, cm.Column, cm.Type)
			}
			if cm.Type == TSkip {
				continue
			}
			if cm.Field == "" {
				cm.Field = cm.Column
			}
			if cm.Field == "_id" || strings.HasPrefix(cm.Field, "$") || fields[cm.Field] {
				return apperr.Invalid("%s.%s: field %q is reserved or used twice.", m.Table, cm.Column, cm.Field)
			}
			fields[cm.Field] = true
		}
	}
	for i := range r.Embeds {
		e := &r.Embeds[i]
		pm, cm := r.mapping(e.Parent), r.mapping(e.Child)
		if pm == nil || cm == nil {
			return apperr.Invalid("Embedding %s into %s: both tables must be selected.", e.Child, e.Parent)
		}
		if pm.EmbedOnly {
			return apperr.Invalid("Embedding into %s: the parent must be copied, not embed-only.", e.Parent)
		}
		if pm.IDStrategy != IDPrimaryKey || len(byName[e.Parent].PrimaryKey) != 1 {
			return apperr.Invalid("Embedding into %s needs a single-column primary key used as _id.", e.Parent)
		}
		if len(e.ChildColumns) != 1 {
			// Default to the foreign key that points at the parent.
			for _, fk := range byName[e.Child].ForeignKeys {
				if fk.RefTable == e.Parent && len(fk.Columns) == 1 {
					e.ChildColumns = fk.Columns
				}
			}
			if len(e.ChildColumns) != 1 {
				return apperr.Invalid("Embedding %s into %s: name the single foreign key column.", e.Child, e.Parent)
			}
		}
		if e.As == "" {
			e.As = e.Child
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// rowToDoc converts a scanned row.
func rowToDoc(m *TableMapping, pk []string, cols []string, vals []any, withID bool) (bson.D, error) {
	doc := bson.D{}
	byCol := map[string]any{}
	for i, c := range cols {
		byCol[c] = vals[i]
	}
	if withID {
		switch {
		case m.IDStrategy == IDObjectID:
			doc = append(doc, bson.E{Key: "_id", Value: bson.NewObjectID()})
		case len(pk) == 1:
			v, err := ConvertValue(byCol[pk[0]], typeOf(m, pk[0]))
			if err != nil {
				return nil, fmt.Errorf("%s: %v", pk[0], err)
			}
			doc = append(doc, bson.E{Key: "_id", Value: v})
		default:
			id := bson.D{}
			for _, k := range pk {
				v, err := ConvertValue(byCol[k], typeOf(m, k))
				if err != nil {
					return nil, fmt.Errorf("%s: %v", k, err)
				}
				id = append(id, bson.E{Key: k, Value: v})
			}
			doc = append(doc, bson.E{Key: "_id", Value: id})
		}
	}
	for _, cm := range m.Columns {
		if cm.Type == TSkip {
			continue
		}
		// With the pk strategy a single key column becomes _id only.
		if withID && m.IDStrategy == IDPrimaryKey && len(pk) == 1 && cm.Column == pk[0] {
			continue
		}
		v, err := ConvertValue(byCol[cm.Column], cm.Type)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", cm.Column, err)
		}
		var ok bool
		if doc, ok = docpath.Set(doc, cm.Field, v); !ok {
			return nil, fmt.Errorf("%s: field %s conflicts with another column", cm.Column, cm.Field)
		}
	}
	return doc, nil
}

func typeOf(m *TableMapping, col string) string {
	for _, c := range m.Columns {
		if c.Column == col && c.Type != TSkip {
			return c.Type
		}
	}
	return TAuto
}

func selectSQL(d dialect, m *TableMapping, pk []string, order []string) (string, []string) {
	cols := []string{}
	seen := map[string]bool{}
	add := func(c string) {
		if !seen[c] {
			seen[c] = true
			cols = append(cols, c)
		}
	}
	for _, k := range pk {
		add(k)
	}
	for _, c := range m.Columns {
		if c.Type != TSkip {
			add(c.Column)
		}
	}
	for _, c := range order {
		add(c)
	}
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = d.quote(c)
	}
	q := "SELECT " + strings.Join(quoted, ", ") + " FROM " + d.quote(m.Table)
	if len(order) > 0 {
		o := make([]string, len(order))
		for i, c := range order {
			o[i] = d.quote(c)
		}
		q += " ORDER BY " + strings.Join(o, ", ")
	}
	return q, cols
}

func scanRow(rows *sql.Rows, n int) ([]any, error) {
	vals := make([]any, n)
	ptrs := make([]any, n)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	return vals, rows.Scan(ptrs...)
}

// Execute runs a migration. It records created collections in run as it
// goes, so a failed run can still be rolled back.
func Execute(ctx context.Context, src mongox.Source, req Request, run *Run, p *jobs.Progress) (*Report, error) {
	start := time.Now()
	sess, err := src.Session(req.ConnID)
	if err != nil {
		return nil, err
	}
	db, d, err := open(ctx, req.Source)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := beginRead(ctx, db, req.Source.Driver)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	tables, err := introspect(ctx, tx, req.Source.Driver)
	if err != nil {
		return nil, err
	}
	if err := req.Validate(tables); err != nil {
		return nil, err
	}
	schema := map[string]Table{}
	var totalRows int64
	for _, t := range tables {
		schema[t.Name] = t
	}
	for _, m := range req.Tables {
		totalRows += schema[m.Table].Rows
	}
	if p != nil {
		p.SetTotal(totalRows, "rows")
	}

	target := sess.Client.Database(req.DB)
	existing, err := target.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, apperr.From(err)
	}
	exists := map[string]bool{}
	for _, n := range existing {
		exists[n] = true
	}
	for _, m := range req.Tables {
		if m.EmbedOnly {
			continue
		}
		if err := sess.RequireWrite(ctx, req.DB, m.Collection, mongox.ActionInsert, mongox.ActionCreateCollection); err != nil {
			return nil, err
		}
		if exists[m.Collection] {
			n, err := target.Collection(m.Collection).EstimatedDocumentCount(ctx)
			if err != nil {
				return nil, apperr.From(err)
			}
			if n > 0 {
				return nil, apperr.New(apperr.CodeConflict, "Collection %s.%s already has %d documents; migrations only write to new or empty collections.", req.DB, m.Collection, n)
			}
		}
	}
	for _, e := range req.Embeds {
		if err := sess.RequireWrite(ctx, req.DB, req.mapping(e.Parent).Collection, mongox.ActionUpdate); err != nil {
			return nil, err
		}
	}

	rep := &Report{RunID: run.ID}
	for i := range req.Tables {
		m := &req.Tables[i]
		if m.EmbedOnly {
			continue
		}
		if !exists[m.Collection] {
			if err := target.CreateCollection(ctx, m.Collection); err != nil {
				return rep, apperr.From(err)
			}
			run.CreatedCollections = append(run.CreatedCollections, m.Collection)
		}
		tr, err := copyTable(ctx, tx, d, target.Collection(m.Collection), m, schema[m.Table], req, p)
		rep.Tables = append(rep.Tables, tr)
		if err != nil {
			return rep, err
		}
	}
	for _, e := range req.Embeds {
		er, err := embed(ctx, tx, d, target, req, e, schema)
		rep.Embeds = append(rep.Embeds, er)
		if err != nil {
			return rep, err
		}
	}
	if p != nil {
		p.Message("Validating")
	}
	if err := validateRun(ctx, tx, d, target, req, schema, rep); err != nil {
		return rep, err
	}
	rep.DurationMs = time.Since(start).Milliseconds()
	return rep, nil
}

func copyTable(ctx context.Context, tx *sql.Tx, d dialect, coll *mongo.Collection, m *TableMapping, t Table, req Request, p *jobs.Progress) (TableReport, error) {
	tr := TableReport{Table: m.Table, Collection: m.Collection, Rows: t.Rows, Errors: []string{}, SampleMismatches: []string{}}
	q, cols := selectSQL(d, m, t.PrimaryKey, t.PrimaryKey)
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return tr, apperr.New(apperr.CodeMongo, "Reading %s failed: %s", m.Table, logging.Redact(err.Error()))
	}
	defer rows.Close()
	batch := make([]any, 0, req.BatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		res, err := coll.InsertMany(ctx, batch, options.InsertMany().SetOrdered(true))
		if res != nil {
			tr.Inserted += int64(len(res.InsertedIDs))
		}
		batch = batch[:0]
		return apperr.Check(err)
	}
	var rowNum int64
	for rows.Next() {
		rowNum++
		vals, err := scanRow(rows, len(cols))
		if err != nil {
			return tr, apperr.New(apperr.CodeMongo, "Reading %s row %d failed: %s", m.Table, rowNum, logging.Redact(err.Error()))
		}
		doc, err := rowToDoc(m, t.PrimaryKey, cols, vals, true)
		if p != nil {
			p.Add(1)
		}
		if err != nil {
			msg := fmt.Sprintf("row %d: %v", rowNum, err)
			if req.OnRowError != "skip" {
				return tr, apperr.Invalid("%s %s", m.Table, msg)
			}
			tr.Skipped++
			if len(tr.Errors) < 20 {
				tr.Errors = append(tr.Errors, msg)
			}
			continue
		}
		batch = append(batch, doc)
		if len(batch) >= req.BatchSize {
			if err := flush(); err != nil {
				return tr, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return tr, apperr.New(apperr.CodeMongo, "Reading %s failed: %s", m.Table, logging.Redact(err.Error()))
	}
	return tr, flush()
}

func embed(ctx context.Context, tx *sql.Tx, d dialect, target *mongo.Database, req Request, e Embed, schema map[string]Table) (EmbedReport, error) {
	er := EmbedReport{Child: e.Child, Parent: e.Parent, As: e.As}
	pm, cm := req.mapping(e.Parent), req.mapping(e.Child)
	parent := target.Collection(pm.Collection)
	parentPK := schema[e.Parent].PrimaryKey[0]
	fkCol := e.ChildColumns[0]
	order := append([]string{fkCol}, schema[e.Child].PrimaryKey...)
	q, cols := selectSQL(d, cm, nil, order)
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return er, apperr.New(apperr.CodeMongo, "Reading %s failed: %s", e.Child, logging.Redact(err.Error()))
	}
	defer rows.Close()
	fkIdx := -1
	for i, c := range cols {
		if c == fkCol {
			fkIdx = i
		}
	}
	var models []mongo.WriteModel
	var groupKey any
	var group bson.A
	flushGroup := func() {
		if groupKey != nil && len(group) > 0 {
			models = append(models, mongo.NewUpdateOneModel().
				SetFilter(bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: groupKey}}}}).
				SetUpdate(bson.D{{Key: "$set", Value: bson.D{{Key: e.As, Value: group}}}}))
		}
		group = nil
	}
	flushWrites := func() error {
		if len(models) == 0 {
			return nil
		}
		_, err := parent.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		models = models[:0]
		return apperr.Check(err)
	}
	childMapping := *cm
	childMapping.IDStrategy = IDObjectID // embedded documents carry the child's columns, no _id
	for rows.Next() {
		vals, err := scanRow(rows, len(cols))
		if err != nil {
			return er, apperr.From(err)
		}
		er.ChildRows++
		if vals[fkIdx] == nil {
			er.Orphans++
			continue
		}
		key, err := ConvertValue(vals[fkIdx], typeOf(pm, parentPK))
		if err != nil {
			return er, apperr.Invalid("%s.%s: %v", e.Child, fkCol, err)
		}
		if groupKey == nil || !mql.Equal(key, groupKey) {
			flushGroup()
			groupKey = key
		}
		doc, err := rowToDoc(&childMapping, nil, cols, vals, false)
		if err != nil {
			return er, apperr.Invalid("%s: %v", e.Child, err)
		}
		group = append(group, doc)
		if len(models) >= 500 {
			if err := flushWrites(); err != nil {
				return er, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return er, apperr.From(err)
	}
	flushGroup()
	if err := flushWrites(); err != nil {
		return er, err
	}
	// Count what actually landed in parents (orphan groups match nothing).
	cur, err := parent.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: e.As, Value: bson.D{{Key: "$type", Value: "array"}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: nil}, {Key: "n", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$size", Value: "$" + e.As}}}}}}}},
	})
	if err != nil {
		return er, apperr.From(err)
	}
	defer cur.Close(context.Background())
	var res []struct {
		N int64 `bson:"n"`
	}
	if err := cur.All(ctx, &res); err != nil {
		return er, apperr.From(err)
	}
	if len(res) == 1 {
		er.Embedded = res[0].N
	}
	er.Orphans = er.ChildRows - er.Embedded
	// Validate against the source: every child row whose parent exists must be embedded.
	var expected int64
	join := fmt.Sprintf("SELECT COUNT(*) FROM %s c JOIN %s p ON c.%s = p.%s", d.quote(e.Child), d.quote(e.Parent), d.quote(fkCol), d.quote(parentPK))
	if err := tx.QueryRowContext(ctx, join).Scan(&expected); err != nil {
		return er, apperr.New(apperr.CodeMongo, "Validating %s failed: %s", e.Child, logging.Redact(err.Error()))
	}
	er.CountOK = er.Embedded == expected
	return er, nil
}

func validateRun(ctx context.Context, tx *sql.Tx, d dialect, target *mongo.Database, req Request, schema map[string]Table, rep *Report) error {
	rep.Valid = true
	for i := range rep.Tables {
		tr := &rep.Tables[i]
		m := req.mapping(tr.Table)
		t := schema[tr.Table]
		coll := target.Collection(m.Collection)
		var rows int64
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+d.quote(m.Table)).Scan(&rows); err != nil {
			return apperr.New(apperr.CodeMongo, "Counting %s failed: %s", m.Table, logging.Redact(err.Error()))
		}
		docs, err := coll.CountDocuments(ctx, bson.D{})
		if err != nil {
			return apperr.From(err)
		}
		tr.Rows = rows
		tr.CountOK = docs == rows-tr.Skipped
		if !tr.CountOK {
			tr.SampleMismatches = append(tr.SampleMismatches, fmt.Sprintf("count: %d rows, %d skipped, %d documents", rows, tr.Skipped, docs))
		}

		// Re-read up to 50 rows and compare them field by field with the stored documents.
		q, cols := selectSQL(d, m, t.PrimaryKey, t.PrimaryKey)
		srows, err := tx.QueryContext(ctx, q+" LIMIT 50")
		if err != nil {
			return apperr.New(apperr.CodeMongo, "Sampling %s failed: %s", m.Table, logging.Redact(err.Error()))
		}
		for srows.Next() {
			vals, err := scanRow(srows, len(cols))
			if err != nil {
				srows.Close()
				return apperr.From(err)
			}
			want, err := rowToDoc(m, t.PrimaryKey, cols, vals, true)
			if err != nil {
				continue // skipped row
			}
			filter := bson.D{}
			if m.IDStrategy == IDPrimaryKey {
				id, _ := docpath.Get(want, "_id")
				filter = bson.D{{Key: "_id", Value: bson.D{{Key: "$eq", Value: id}}}}
			} else {
				for _, k := range t.PrimaryKey {
					v, ok := docpath.Get(want, fieldFor(m, k))
					if !ok {
						continue
					}
					filter = append(filter, bson.E{Key: fieldFor(m, k), Value: bson.D{{Key: "$eq", Value: v}}})
				}
			}
			var got bson.D
			tr.SampleChecked++
			if len(filter) == 0 {
				continue
			}
			if err := coll.FindOne(ctx, filter).Decode(&got); err != nil {
				tr.SampleMismatches = append(tr.SampleMismatches, fmt.Sprintf("%s: missing", mql.Compact(filter)))
				continue
			}
			for _, cm := range m.Columns {
				if cm.Type == TSkip || (m.IDStrategy == IDPrimaryKey && len(t.PrimaryKey) == 1 && cm.Column == t.PrimaryKey[0]) {
					continue
				}
				wv, _ := docpath.Get(want, cm.Field)
				gv, _ := docpath.Get(got, cm.Field)
				if !mql.Equal(wv, gv) && len(tr.SampleMismatches) < 20 {
					tr.SampleMismatches = append(tr.SampleMismatches, fmt.Sprintf("%s %s: source %s, stored %s", mql.Compact(filter), cm.Field, mql.Compact(wv), mql.Compact(gv)))
				}
			}
		}
		srows.Close()
		if !tr.CountOK || len(tr.SampleMismatches) > 0 {
			rep.Valid = false
		}
	}
	for _, e := range rep.Embeds {
		if !e.CountOK {
			rep.Valid = false
		}
	}
	return nil
}

func fieldFor(m *TableMapping, col string) string {
	for _, c := range m.Columns {
		if c.Column == col && c.Type != TSkip {
			return c.Field
		}
	}
	return col
}

// ---- service, run records, rollback ----------------------------------------

// Service runs migrations as jobs and persists run records.
type Service struct {
	src  mongox.Source
	jobs *jobs.Manager
	mu   sync.Mutex
	path string
}

// NewService keeps run records in dir/migrations.json.
func NewService(src mongox.Source, jm *jobs.Manager, dir string) *Service {
	return &Service{src: src, jobs: jm, path: filepath.Join(dir, "migrations.json")}
}

type runsFile struct {
	Version int   `json:"version"`
	Runs    []Run `json:"runs"`
}

func (s *Service) save(run Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var f runsFile
	if err := config.ReadJSON(s.path, &f); err != nil {
		return err
	}
	found := false
	for i := range f.Runs {
		if f.Runs[i].ID == run.ID {
			f.Runs[i] = run
			found = true
		}
	}
	if !found {
		f.Runs = append(f.Runs, run)
	}
	if len(f.Runs) > 50 {
		f.Runs = f.Runs[len(f.Runs)-50:]
	}
	f.Version = 1
	return config.WriteJSON(s.path, f)
}

// Runs lists recorded migrations, newest first.
func (s *Service) Runs() ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var f runsFile
	if err := config.ReadJSON(s.path, &f); err != nil {
		return nil, err
	}
	out := append([]Run{}, f.Runs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// Get returns one run.
func (s *Service) Get(id string) (Run, error) {
	runs, err := s.Runs()
	if err != nil {
		return Run{}, err
	}
	for _, r := range runs {
		if r.ID == id {
			return r, nil
		}
	}
	return Run{}, apperr.New(apperr.CodeNotFound, "Migration run not found.")
}

// Start validates the request and queues the migration job.
func (s *Service) Start(req Request) (jobs.Job, error) {
	sess, err := s.src.Session(req.ConnID)
	if err != nil {
		return jobs.Job{}, err
	}
	if err := sess.RequireWritable(); err != nil {
		return jobs.Job{}, err
	}
	if _, err := dialectFor(req.Source.Driver); err != nil {
		return jobs.Job{}, err
	}
	run := Run{ID: config.NewID(), Created: time.Now().UTC(), Source: SafeDSNHint(req.Source), ConnID: req.ConnID, DB: req.DB, State: "running", CreatedCollections: []string{}}
	if err := s.save(run); err != nil {
		return jobs.Job{}, err
	}
	title := fmt.Sprintf("Migrate %s → %s", run.Source, req.DB)
	return s.jobs.Start("migrate", title, func(ctx context.Context, p *jobs.Progress) (string, error) {
		rep, err := Execute(ctx, s.src, req, &run, p)
		run.Report = rep
		switch {
		case err != nil:
			run.State = "failed"
			run.Summary = err.Error()
		case !rep.Valid:
			run.State = "failed"
			run.Summary = "Validation found differences; see the report. Roll back to remove the collections."
		default:
			run.State = "succeeded"
			var docs int64
			for _, t := range rep.Tables {
				docs += t.Inserted
			}
			run.Summary = fmt.Sprintf("%d tables, %d documents, validated", len(rep.Tables), docs)
		}
		if serr := s.save(run); serr != nil && err == nil {
			err = serr
		}
		if err != nil {
			return "", err
		}
		if !rep.Valid {
			return "", apperr.New(apperr.CodeConflict, "%s", run.Summary)
		}
		return run.Summary + " (run " + run.ID + ")", nil
	}), nil
}

// Rollback drops the collections a run created. Collections that existed
// before the run (empty targets) are emptied instead of dropped.
func (s *Service) Rollback(ctx context.Context, runID string) (Run, error) {
	run, err := s.Get(runID)
	if err != nil {
		return Run{}, err
	}
	if run.State == "running" {
		return Run{}, apperr.Invalid("The migration is still running; cancel it first.")
	}
	if run.State == "rolledBack" {
		return run, nil
	}
	sess, err := s.src.Session(run.ConnID)
	if err != nil {
		return Run{}, err
	}
	db := sess.Client.Database(run.DB)
	for _, c := range run.CreatedCollections {
		if err := sess.RequireWrite(ctx, run.DB, c, mongox.ActionDropCollection); err != nil {
			return Run{}, err
		}
		if err := db.Collection(c).Drop(ctx); err != nil {
			return Run{}, apperr.From(err)
		}
	}
	if run.Report != nil {
		created := map[string]bool{}
		for _, c := range run.CreatedCollections {
			created[c] = true
		}
		for _, t := range run.Report.Tables {
			if !created[t.Collection] {
				if _, err := db.Collection(t.Collection).DeleteMany(ctx, bson.D{}); err != nil {
					return Run{}, apperr.From(err)
				}
			}
		}
	}
	run.State = "rolledBack"
	run.Summary = fmt.Sprintf("Rolled back: removed %d collection(s)", len(run.CreatedCollections))
	return run, s.save(run)
}
