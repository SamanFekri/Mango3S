package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

// SeedSQLite creates a small shop schema with the awkward cases: decimals,
// NULLs, blobs, JSON text, dates, a composite key and foreign keys.
func SeedSQLite(t testing.TB, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email VARCHAR(200), vip BOOLEAN, balance DECIMAL(10,2), joined DATETIME, prefs JSON, avatar BLOB)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER REFERENCES customers(id), total NUMERIC(12,2), placed_at DATE, note TEXT)`,
		`CREATE TABLE order_lines (order_id INTEGER REFERENCES orders(id), line INTEGER, sku TEXT, qty INTEGER, PRIMARY KEY (order_id, line))`,
		`CREATE TABLE logs (msg TEXT, level REAL)`,
		`INSERT INTO customers VALUES (1, 'Ada', 'ada@example.com', 1, 1234.50, '2024-01-31 10:00:00', '{"theme":"dark","tags":["a","b"]}', x'00ff10')`,
		`INSERT INTO customers VALUES (2, 'Grace', NULL, 0, 0.10, '2023-06-01 08:30:00', NULL, NULL)`,
		`INSERT INTO customers VALUES (3, 'Linus', 'linus@example.com', NULL, 99999999.99, NULL, '{}', NULL)`,
		`INSERT INTO orders VALUES (10, 1, 19.99, '2024-02-01', 'first')`,
		`INSERT INTO orders VALUES (11, 1, 5.00, '2024-02-02', NULL)`,
		`INSERT INTO orders VALUES (12, 2, 100.00, '2024-03-01', 'bulk')`,
		`INSERT INTO orders VALUES (13, 99, 1.00, '2024-03-02', 'orphan')`,
		`INSERT INTO order_lines VALUES (10, 1, 'KB-001', 1), (10, 2, 'MS-002', 2), (12, 1, 'HD-004', 10)`,
		`INSERT INTO logs VALUES ('boot', 1.5), ('ready', NULL)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

func TestIntrospectSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shop.db")
	SeedSQLite(t, path)
	tables, err := Introspect(context.Background(), Source{Driver: SQLite, DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Table{}
	for _, tb := range tables {
		byName[tb.Name] = tb
	}
	if len(tables) != 4 {
		t.Fatalf("tables: %+v", tables)
	}
	c := byName["customers"]
	if c.Rows != 3 || len(c.PrimaryKey) != 1 || c.PrimaryKey[0] != "id" || len(c.Columns) != 8 {
		t.Fatalf("customers: %+v", c)
	}
	suggest := map[string]string{}
	for _, col := range c.Columns {
		suggest[col.Name] = col.Suggest
	}
	want := map[string]string{"id": TLong, "name": TString, "email": TString, "vip": TBool, "balance": TDecimal, "joined": TDate, "prefs": TJSON, "avatar": TBinary}
	for k, v := range want {
		if suggest[k] != v {
			t.Errorf("suggest %s = %s, want %s", k, suggest[k], v)
		}
	}
	o := byName["order_lines"]
	if strings.Join(o.PrimaryKey, ",") != "order_id,line" || len(o.ForeignKeys) != 1 || o.ForeignKeys[0].RefTable != "orders" {
		t.Fatalf("order_lines keys: %+v", o)
	}
	if len(byName["logs"].PrimaryKey) != 0 {
		t.Fatal("logs has no primary key")
	}
}

func TestSourceIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shop.db")
	SeedSQLite(t, path)
	db, _, err := open(context.Background(), Source{Driver: SQLite, DSN: path})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM customers`); err == nil {
		t.Fatal("source connection allowed a write")
	}
	if _, _, err := open(context.Background(), Source{Driver: SQLite, DSN: filepath.Join(t.TempDir(), "missing.db")}); err == nil {
		t.Fatal("missing SQLite file was created instead of rejected")
	}
}

func TestSuggestType(t *testing.T) {
	cases := map[string]string{
		"integer": TInt, "int(11)": TInt, "bigint": TLong, "bigint unsigned": TLong, "int unsigned": TLong, "tinyint(1)": TBool,
		"numeric(10,2)": TDecimal, "decimal": TDecimal, "double precision": TDouble, "real": TDouble,
		"timestamp with time zone": TDate, "datetime": TDate, "jsonb": TJSON, "uuid": TUUID, "bytea": TBinary,
		"varchar(20)": TString, "character varying": TString, "text": TString, "boolean": TBool, "INTEGER": TInt,
	}
	for in, want := range cases {
		if got := SuggestType(in); got != want {
			t.Errorf("SuggestType(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestConvertValue(t *testing.T) {
	ts := time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC)
	ok := []struct {
		in   any
		typ  string
		want string
	}{
		{int64(5), TAuto, "5"},
		{int64(5_000_000_000), TAuto, "NumberLong(5000000000)"},
		{int64(5), TLong, "NumberLong(5)"},
		{[]byte("42"), TInt, "42"},
		{"1234.50", TDecimal, `NumberDecimal("1234.50")`},
		{[]byte("9.99"), TDecimal, `NumberDecimal("9.99")`},
		{1.5, TDouble, "1.5"},
		{int64(1), TBool, "true"},
		{[]byte("\x00"), TBool, "false"},
		{ts, TDate, `ISODate("2024-01-31T10:00:00.000Z")`},
		{"2024-01-31 10:00:00", TDate, `ISODate("2024-01-31T10:00:00.000Z")`},
		{"2024-02-01", TDate, `ISODate("2024-02-01T00:00:00.000Z")`},
		{`{"a":[1,2]}`, TJSON, `{"a": [1, 2]}`},
		{"123e4567-e89b-12d3-a456-426614174000", TUUID, `UUID("123e4567-e89b-12d3-a456-426614174000")`},
		{[]byte{1, 2}, TBinary, `BinData(0, "AQI=")`},
		{nil, TInt, "null"},
		{ts, TString, `"2024-01-31T10:00:00Z"`},
	}
	for _, c := range ok {
		got, err := ConvertValue(c.in, c.typ)
		if err != nil || mql.Compact(got) != c.want {
			t.Errorf("%v as %s: %s %v (want %s)", c.in, c.typ, mql.Compact(got), err, c.want)
		}
	}
	for _, c := range []struct {
		in  any
		typ string
	}{{int64(5_000_000_000), TInt}, {"abc", TDecimal}, {"maybe", TBool}, {"{", TJSON}, {"nope", TUUID}, {"x", TDate}} {
		if _, err := ConvertValue(c.in, c.typ); err == nil {
			t.Errorf("%v as %s accepted", c.in, c.typ)
		}
	}
}

func TestRequestValidate(t *testing.T) {
	tables := []Table{
		{Name: "a", PrimaryKey: []string{"id"}, Columns: []Column{{Name: "id", Suggest: TLong}, {Name: "x", Suggest: TString}}},
		{Name: "b", Columns: []Column{{Name: "v", Suggest: TString}}, ForeignKeys: []ForeignKey{{Columns: []string{"v"}, RefTable: "a", RefColumns: []string{"id"}}}},
	}
	good := Request{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "a"}}}
	if err := good.Validate(tables); err != nil || len(good.Tables[0].Columns) != 2 || good.Tables[0].IDStrategy != IDPrimaryKey {
		t.Fatalf("defaults: %+v %v", good, err)
	}
	bad := []Request{
		{ConnID: "c", DB: "d"},
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "zz", Collection: "x"}}},
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "b", Collection: "b"}}}, // no pk
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "same"}, {Table: "b", Collection: "same", IDStrategy: IDObjectID}}},
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "a", Columns: []ColumnMapping{{Column: "x", Field: "_id"}}}}},
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "a", Columns: []ColumnMapping{{Column: "x", Type: "money"}}}}},
		{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "a"}}, Embeds: []Embed{{Child: "b", Parent: "a"}}},
	}
	for i, r := range bad {
		if err := r.Validate(tables); err == nil {
			t.Errorf("bad request %d accepted", i)
		}
	}
	emb := Request{ConnID: "c", DB: "d", Tables: []TableMapping{{Table: "a", Collection: "a"}, {Table: "b", IDStrategy: IDObjectID, EmbedOnly: true}},
		Embeds: []Embed{{Child: "b", Parent: "a"}}}
	if err := emb.Validate(tables); err != nil || emb.Embeds[0].ChildColumns[0] != "v" || emb.Embeds[0].As != "b" {
		t.Fatalf("embed defaults: %+v %v", emb.Embeds, err)
	}
}

func TestRowToDocCompositeKey(t *testing.T) {
	m := &TableMapping{Table: "ol", IDStrategy: IDPrimaryKey, Columns: []ColumnMapping{
		{Column: "order_id", Field: "order_id", Type: TLong}, {Column: "line", Field: "line", Type: TInt}, {Column: "sku", Field: "item.sku", Type: TString},
	}}
	d, err := rowToDoc(m, []string{"order_id", "line"}, []string{"order_id", "line", "sku"}, []any{int64(10), int64(2), "KB"}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"_id": {"order_id": NumberLong(10), "line": 2}, "order_id": NumberLong(10), "line": 2, "item": {"sku": "KB"}}`
	if got := mql.Compact(d); got != want {
		t.Fatalf("got %s", got)
	}
	_ = bson.D{}
}
