// Package migrate copies tables from relational databases (PostgreSQL,
// MySQL/MariaDB, SQLite) into MongoDB collections: introspect the schema, map
// tables to collections and columns to typed fields, optionally embed child
// rows into parents, then validate counts and sampled rows. The source is
// only ever read, inside one read-only transaction (a consistent snapshot on
// PostgreSQL and MySQL), and every run can be rolled back by dropping the
// collections it created.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL / MariaDB
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL
	_ "modernc.org/sqlite"             // SQLite (pure Go)

	"mango3s/internal/apperr"
	"mango3s/internal/logging"
)

// Drivers.
const (
	Postgres = "postgres"
	MySQL    = "mysql"
	SQLite   = "sqlite"
)

// Source is a relational database to read from. The DSN may contain
// credentials; it is never logged or persisted.
type Source struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// Column describes a table column.
type Column struct {
	Name     string `json:"name"`
	DBType   string `json:"dbType"`
	Nullable bool   `json:"nullable"`
	Suggest  string `json:"suggest"` // suggested target type
}

// ForeignKey describes a reference from this table's columns to another table.
type ForeignKey struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
}

// Table describes a source table.
type Table struct {
	Name        string       `json:"name"` // schema-qualified on PostgreSQL when not in public
	Columns     []Column     `json:"columns"`
	PrimaryKey  []string     `json:"primaryKey"`
	ForeignKeys []ForeignKey `json:"foreignKeys"`
	Rows        int64        `json:"rows"`
}

type dialect struct {
	driverName string
	quote      func(string) string
}

func dialectFor(driver string) (dialect, error) {
	switch driver {
	case Postgres:
		return dialect{"pgx", func(s string) string { return quoteDotted(s, `"`) }}, nil
	case MySQL:
		return dialect{"mysql", func(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }}, nil
	case SQLite:
		return dialect{"sqlite", func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }}, nil
	}
	return dialect{}, apperr.Invalid("Unsupported database %q (supported: postgres, mysql, sqlite).", driver)
}

func quoteDotted(name, q string) string {
	parts := strings.Split(name, ".")
	for i, p := range parts {
		parts[i] = q + strings.ReplaceAll(p, q, q+q) + q
	}
	return strings.Join(parts, ".")
}

// open connects read-only where the driver allows it.
func open(ctx context.Context, src Source) (*sql.DB, dialect, error) {
	d, err := dialectFor(src.Driver)
	if err != nil {
		return nil, d, err
	}
	dsn := strings.TrimSpace(src.DSN)
	if dsn == "" {
		return nil, d, apperr.Invalid("Enter a connection string for the source database.")
	}
	switch src.Driver {
	case SQLite:
		// Read-only at the file level; refuse to create a missing database.
		path := strings.TrimPrefix(dsn, "file:")
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		dsn = "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
	case MySQL:
		// parseTime makes DATETIME columns arrive as time.Time.
		if !strings.Contains(dsn, "parseTime=") {
			sep := "?"
			if strings.Contains(dsn, "?") {
				sep = "&"
			}
			dsn += sep + "parseTime=true&loc=UTC"
		}
	}
	db, err := sql.Open(d.driverName, dsn)
	if err != nil {
		return nil, d, apperr.Invalid("Invalid connection string: %s", logging.Redact(err.Error()))
	}
	db.SetMaxOpenConns(4)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, d, apperr.New(apperr.CodeUnreachable, "Could not connect to the source database: %s", logging.Redact(err.Error()))
	}
	return db, d, nil
}

// beginRead starts the snapshot transaction used for the whole migration.
func beginRead(ctx context.Context, db *sql.DB, driver string) (*sql.Tx, error) {
	opts := &sql.TxOptions{ReadOnly: true}
	switch driver {
	case Postgres, MySQL:
		opts.Isolation = sql.LevelRepeatableRead
	case SQLite:
		opts = nil // the file is opened read-only; the driver does not take ReadOnly options
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, apperr.New(apperr.CodeMongo, "Could not start a read-only transaction on the source: %s", logging.Redact(err.Error()))
	}
	return tx, nil
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Introspect lists tables with columns, keys and row counts.
func Introspect(ctx context.Context, src Source) ([]Table, error) {
	db, _, err := open(ctx, src)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := beginRead(ctx, db, src.Driver)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return introspect(ctx, tx, src.Driver)
}

func introspect(ctx context.Context, q querier, driver string) ([]Table, error) {
	var tables []Table
	var err error
	switch driver {
	case SQLite:
		tables, err = introspectSQLite(ctx, q)
	case Postgres:
		tables, err = introspectPostgres(ctx, q)
	case MySQL:
		tables, err = introspectMySQL(ctx, q)
	}
	if err != nil {
		return nil, apperr.New(apperr.CodeMongo, "Reading the source schema failed: %s", logging.Redact(err.Error()))
	}
	d, _ := dialectFor(driver)
	for i := range tables {
		for j := range tables[i].Columns {
			tables[i].Columns[j].Suggest = SuggestTypeFor(driver, tables[i].Columns[j].DBType)
		}
		if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+d.quote(tables[i].Name)).Scan(&tables[i].Rows); err != nil {
			return nil, apperr.New(apperr.CodeMongo, "Counting %s failed: %s", tables[i].Name, logging.Redact(err.Error()))
		}
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].Name < tables[j].Name })
	return tables, nil
}

func scanStrings(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func introspectSQLite(ctx context.Context, q querier) ([]Table, error) {
	rows, err := q.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	names, err := scanStrings(rows)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, n := range names {
		t := Table{Name: n}
		cr, err := q.QueryContext(ctx, `SELECT name, type, "notnull", pk FROM pragma_table_info(?) ORDER BY cid`, n)
		if err != nil {
			return nil, err
		}
		type pkCol struct {
			pos  int
			name string
		}
		var pks []pkCol
		for cr.Next() {
			var c Column
			var notNull, pk int
			if err := cr.Scan(&c.Name, &c.DBType, &notNull, &pk); err != nil {
				cr.Close()
				return nil, err
			}
			c.Nullable = notNull == 0
			t.Columns = append(t.Columns, c)
			if pk > 0 {
				pks = append(pks, pkCol{pk, c.Name})
			}
		}
		cr.Close()
		sort.Slice(pks, func(i, j int) bool { return pks[i].pos < pks[j].pos })
		for _, p := range pks {
			t.PrimaryKey = append(t.PrimaryKey, p.name)
		}
		fr, err := q.QueryContext(ctx, `SELECT id, "table", "from", "to" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, n)
		if err != nil {
			return nil, err
		}
		byID := map[int]*ForeignKey{}
		var order []int
		for fr.Next() {
			var id int
			var ref, from string
			var to sql.NullString
			if err := fr.Scan(&id, &ref, &from, &to); err != nil {
				fr.Close()
				return nil, err
			}
			fk, ok := byID[id]
			if !ok {
				fk = &ForeignKey{RefTable: ref}
				byID[id] = fk
				order = append(order, id)
			}
			fk.Columns = append(fk.Columns, from)
			fk.RefColumns = append(fk.RefColumns, to.String)
		}
		fr.Close()
		for _, id := range order {
			t.ForeignKeys = append(t.ForeignKeys, *byID[id])
		}
		out = append(out, t)
	}
	return out, nil
}

func introspectPostgres(ctx context.Context, q querier) ([]Table, error) {
	qualified := func(schema, name string) string {
		if schema == "public" {
			return name
		}
		return schema + "." + name
	}
	rows, err := q.QueryContext(ctx, `SELECT table_schema, table_name FROM information_schema.tables
		WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name`)
	if err != nil {
		return nil, err
	}
	type ref struct{ schema, name string }
	var refs []ref
	for rows.Next() {
		var r ref
		if err := rows.Scan(&r.schema, &r.name); err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, r)
	}
	rows.Close()
	var out []Table
	for _, r := range refs {
		t := Table{Name: qualified(r.schema, r.name)}
		cr, err := q.QueryContext(ctx, `SELECT column_name, CASE WHEN data_type = 'USER-DEFINED' OR data_type = 'ARRAY' THEN udt_name ELSE data_type END, is_nullable = 'YES'
			FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, r.schema, r.name)
		if err != nil {
			return nil, err
		}
		for cr.Next() {
			var c Column
			if err := cr.Scan(&c.Name, &c.DBType, &c.Nullable); err != nil {
				cr.Close()
				return nil, err
			}
			t.Columns = append(t.Columns, c)
		}
		cr.Close()
		// pg_catalog, not information_schema: the latter hides constraints from
		// users that only hold SELECT, which is exactly the least-privilege setup.
		pr, err := q.QueryContext(ctx, `SELECT a.attname FROM pg_constraint con
			JOIN pg_class cl ON cl.oid = con.conrelid JOIN pg_namespace n ON n.oid = cl.relnamespace
			CROSS JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
			JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
			WHERE con.contype = 'p' AND n.nspname = $1 AND cl.relname = $2 ORDER BY k.ord`, r.schema, r.name)
		if err != nil {
			return nil, err
		}
		if t.PrimaryKey, err = scanStrings(pr); err != nil {
			return nil, err
		}
		fr, err := q.QueryContext(ctx, `SELECT con.conname, a.attname, rn.nspname, rc.relname, ra.attname
			FROM pg_constraint con
			JOIN pg_class cl ON cl.oid = con.conrelid JOIN pg_namespace n ON n.oid = cl.relnamespace
			JOIN pg_class rc ON rc.oid = con.confrelid JOIN pg_namespace rn ON rn.oid = rc.relnamespace
			CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS k(attnum, refnum, ord)
			JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
			JOIN pg_attribute ra ON ra.attrelid = con.confrelid AND ra.attnum = k.refnum
			WHERE con.contype = 'f' AND n.nspname = $1 AND cl.relname = $2 ORDER BY con.conname, k.ord`, r.schema, r.name)
		if err != nil {
			return nil, err
		}
		byName := map[string]*ForeignKey{}
		var order []string
		for fr.Next() {
			var cname, col, rschema, rtable, rcol string
			if err := fr.Scan(&cname, &col, &rschema, &rtable, &rcol); err != nil {
				fr.Close()
				return nil, err
			}
			fk, ok := byName[cname]
			if !ok {
				fk = &ForeignKey{RefTable: qualified(rschema, rtable)}
				byName[cname] = fk
				order = append(order, cname)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, rcol)
		}
		fr.Close()
		for _, n := range order {
			t.ForeignKeys = append(t.ForeignKeys, *byName[n])
		}
		out = append(out, t)
	}
	return out, nil
}

func introspectMySQL(ctx context.Context, q querier) ([]Table, error) {
	rows, err := q.QueryContext(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	names, err := scanStrings(rows)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, n := range names {
		t := Table{Name: n}
		cr, err := q.QueryContext(ctx, `SELECT column_name, column_type, is_nullable = 'YES' FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? ORDER BY ordinal_position`, n)
		if err != nil {
			return nil, err
		}
		for cr.Next() {
			var c Column
			if err := cr.Scan(&c.Name, &c.DBType, &c.Nullable); err != nil {
				cr.Close()
				return nil, err
			}
			t.Columns = append(t.Columns, c)
		}
		cr.Close()
		pr, err := q.QueryContext(ctx, `SELECT column_name FROM information_schema.key_column_usage
			WHERE table_schema = DATABASE() AND table_name = ? AND constraint_name = 'PRIMARY' ORDER BY ordinal_position`, n)
		if err != nil {
			return nil, err
		}
		if t.PrimaryKey, err = scanStrings(pr); err != nil {
			return nil, err
		}
		fr, err := q.QueryContext(ctx, `SELECT constraint_name, column_name, referenced_table_name, referenced_column_name
			FROM information_schema.key_column_usage
			WHERE table_schema = DATABASE() AND table_name = ? AND referenced_table_name IS NOT NULL
			ORDER BY constraint_name, ordinal_position`, n)
		if err != nil {
			return nil, err
		}
		byName := map[string]*ForeignKey{}
		var order []string
		for fr.Next() {
			var cname, col, rtable, rcol string
			if err := fr.Scan(&cname, &col, &rtable, &rcol); err != nil {
				fr.Close()
				return nil, err
			}
			fk, ok := byName[cname]
			if !ok {
				fk = &ForeignKey{RefTable: rtable}
				byName[cname] = fk
				order = append(order, cname)
			}
			fk.Columns = append(fk.Columns, col)
			fk.RefColumns = append(fk.RefColumns, rcol)
		}
		fr.Close()
		for _, cn := range order {
			t.ForeignKeys = append(t.ForeignKeys, *byName[cn])
		}
		out = append(out, t)
	}
	return out, nil
}

// SafeDSNHint returns a description of a DSN without credentials (for logs and UI).
func SafeDSNHint(src Source) string {
	if u, err := url.Parse(src.DSN); err == nil && u.Host != "" {
		return fmt.Sprintf("%s://%s%s", src.Driver, u.Host, u.Path)
	}
	return src.Driver + " " + logging.Redact(src.DSN)
}
