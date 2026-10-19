//go:build integration

package sqlexport

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	_ "modernc.org/sqlite"
)

type tlog struct{}

func (tlog) Log(string, ...any)     {}
func (tlog) SetTotal(int64, string) {}
func (tlog) Add(int64)              {}

func seed(t *testing.T) *mongo.Collection {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	cl, err := mongo.Connect(options.Client().ApplyURI(uri).SetDirect(true))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db := cl.Database("mango3s_it_sqlexport")
	db.Drop(ctx)
	t.Cleanup(func() { db.Drop(context.Background()); cl.Disconnect(context.Background()) })
	c := db.Collection("people")
	dec, _ := bson.ParseDecimal128("12345678901234567890.123456789")
	docs := []any{}
	for i := 0; i < 450; i++ {
		docs = append(docs, bson.D{
			{Key: "_id", Value: bson.NewObjectID()}, {Key: "n", Value: int32(i)}, {Key: "big", Value: int64(1) << 40},
			{Key: "price", Value: 1.25}, {Key: "dec", Value: dec}, {Key: "ok", Value: i%2 == 0},
			{Key: "when", Value: bson.DateTime(1700000000123)}, {Key: "name", Value: "O'Brien \\ \"q\" ✓"},
			{Key: "tags", Value: bson.A{"a", int32(1)}}, {Key: "addr", Value: bson.D{{Key: "city", Value: "Oslo"}, {Key: "zip", Value: "0150"}}},
			{Key: "bin", Value: bson.Binary{Subtype: 0, Data: []byte{0, 1, 2, 255}}},
		})
	}
	// Outliers beyond the sample: a field the sample never saw and a type that doesn't fit.
	docs = append(docs, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "n", Value: 2.5}, {Key: "late", Value: "x"}})
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
	return c
}

func export(t *testing.T, c *mongo.Collection, dialect string) (string, Report) {
	var buf bytes.Buffer
	rep, err := Export(context.Background(), c, &buf, Options{Dialect: dialect, SampleSize: 100, BatchRows: 100, Flatten: true, DropTable: true}, tlog{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rows != 451 || rep.ExtraFields != 1 || rep.TypeMismatch != 1 {
		t.Fatalf("%s report: %+v", dialect, rep)
	}
	return buf.String(), rep
}

func check(t *testing.T, db *sql.DB, script, dialect string) {
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, script); err != nil {
		t.Fatalf("%s: %v", dialect, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM people`).Scan(&n); err != nil || n != 451 {
		t.Fatalf("%s count %d %v", dialect, n, err)
	}
	var name, city, dec string
	q := map[string]string{
		SQLite:   `SELECT name, addr_city, dec FROM people WHERE n = 7`,
		Postgres: `SELECT name, addr_city, dec::text FROM people WHERE n = 7`,
		MySQL:    "SELECT name, addr_city, CAST(`dec` AS CHAR) FROM people WHERE n = 7",
	}[dialect]
	if err := db.QueryRow(q).Scan(&name, &city, &dec); err != nil {
		t.Fatalf("%s: %v", dialect, err)
	}
	if name != "O'Brien \\ \"q\" ✓" || city != "Oslo" || !strings.HasPrefix(dec, "12345678901234567890.123456789") {
		t.Fatalf("%s values: %q %q %q", dialect, name, city, dec)
	}
	var extra string
	q = map[string]string{
		SQLite:   `SELECT _extra FROM people WHERE n IS NULL`,
		Postgres: `SELECT _extra::text FROM people WHERE n IS NULL`,
		MySQL:    "SELECT CAST(`_extra` AS CHAR) FROM people WHERE n IS NULL",
	}[dialect]
	if err := db.QueryRow(q).Scan(&extra); err != nil {
		t.Fatalf("%s extra: %v", dialect, err)
	}
	if !strings.Contains(extra, `"late"`) || !strings.Contains(extra, `2.5`) {
		t.Fatalf("%s extra: %s", dialect, extra)
	}
}

func TestSQLiteRoundTrip(t *testing.T) {
	c := seed(t)
	script, _ := export(t, c, SQLite)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	check(t, db, script, SQLite)
}

func TestPostgresRoundTrip(t *testing.T) {
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		t.Skip("PG_DSN not set")
	}
	c := seed(t)
	script, _ := export(t, c, Postgres)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(func() { db.Exec(`DROP TABLE IF EXISTS people`) })
	check(t, db, script, Postgres)
}

func TestMySQLRoundTrip(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		t.Skip("MYSQL_DSN not set")
	}
	c := seed(t)
	script, _ := export(t, c, MySQL)
	if !strings.Contains(dsn, "multiStatements") {
		if strings.Contains(dsn, "?") {
			dsn += "&multiStatements=true"
		} else {
			dsn += "?multiStatements=true"
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS people") })
	check(t, db, script, MySQL)
}
