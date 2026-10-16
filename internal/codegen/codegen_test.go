package codegen

import (
	"go/parser"
	"go/token"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func richSpec() Spec {
	oid, _ := bson.ObjectIDFromHex("65a1f0c2e4b0a1b2c3d4e5f6")
	dec, _ := bson.ParseDecimal128("9.99")
	return Spec{
		Kind: "find",
		DB:   "shop",
		Coll: "orders",
		Filter: bson.D{
			{Key: "_id", Value: oid},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"A", "B \"quoted\""}}}},
			{Key: "total", Value: bson.D{{Key: "$gte", Value: dec}}},
			{Key: "qty", Value: bson.D{{Key: "$lt", Value: int32(5)}}},
			{Key: "big", Value: int64(3000000000)},
			{Key: "ratio", Value: 1.5},
			{Key: "whole", Value: 2.0},
			{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: bson.NewDateTimeFromTime(time.Date(2024, 1, 31, 12, 30, 0, 123e6, time.UTC))}}},
			{Key: "name", Value: bson.Regex{Pattern: "^a/b", Options: "i"}},
			{Key: "ref", Value: bson.Binary{Subtype: 4, Data: []byte{0x12, 0x3e, 0x45, 0x67, 0xe8, 0x9b, 0x12, 0xd3, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x40, 0x00}}},
			{Key: "raw", Value: bson.Binary{Subtype: 0, Data: []byte("hi")}},
			{Key: "deleted", Value: nil},
			{Key: "active", Value: true},
			{Key: "odd key", Value: math.NaN()},
			{Key: "address.city", Value: "Paris"},
		},
		Projection: bson.D{{Key: "status", Value: int32(1)}, {Key: "_id", Value: int32(0)}},
		Sort:       bson.D{{Key: "createdAt", Value: int32(-1)}, {Key: "status", Value: int32(1)}},
		Skip:       10,
		Limit:      5,
	}
}

func aggSpec() Spec {
	return Spec{
		Kind: "aggregate", DB: "shop", Coll: "my-orders",
		Pipeline: []bson.D{
			{{Key: "$match", Value: bson.D{{Key: "status", Value: "A"}}}},
			{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$customer"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: int32(1)}}}}}},
			{{Key: "$sort", Value: bson.D{{Key: "n", Value: int32(-1)}}}},
			{{Key: "$limit", Value: int64(10)}},
		},
	}
}

func gen(t *testing.T, lang string, s Spec) string {
	t.Helper()
	code, err := Generate(lang, s)
	if err != nil {
		t.Fatalf("%s: %v", lang, err)
	}
	return code
}

func mustContain(t *testing.T, lang, code string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(code, p) {
			t.Errorf("%s output missing %q:\n%s", lang, p, code)
		}
	}
}

func TestTypeRendering(t *testing.T) {
	s := richSpec()
	mustContain(t, "mongosh", gen(t, "mongosh", s),
		`use("shop");`, `db.orders.find(`, `_id: ObjectId("65a1f0c2e4b0a1b2c3d4e5f6")`, `Decimal128("9.99")`,
		`big: Long("3000000000")`, `ISODate("2024-01-31T12:30:00.123Z")`, `/^a\/b/i`,
		`UUID("123e4567-e89b-12d3-a456-426614174000")`, `"odd key": NaN`, `"address.city": "Paris"`,
		`.sort({ createdAt: -1, status: 1 })`, `.skip(10)`, `.limit(5)`)
	mustContain(t, "javascript", gen(t, "javascript", s),
		`import { MongoClient, Binary, Decimal128, Long, ObjectId, UUID } from 'mongodb';`,
		`new ObjectId("65a1f0c2e4b0a1b2c3d4e5f6")`, `Decimal128.fromString("9.99")`, `Long.fromString("3000000000")`,
		`new Date("2024-01-31T12:30:00.123Z")`, `Binary.createFromBase64("aGk=", 0)`, `projection: { status: 1, _id: 0 }`,
		`for await (const doc of cursor)`)
	mustContain(t, "python", gen(t, "python", s),
		`from bson import Binary, Decimal128, Int64, ObjectId, Regex`, `from datetime import datetime, timezone`, `import uuid`,
		`"big": Int64(3000000000)`, `"whole": 2.0`, `datetime(2024, 1, 31, 12, 30, 0, 123000, tzinfo=timezone.utc)`,
		`Regex("^a/b", "i")`, `"deleted": None`, `"active": True`, `float("nan")`, `Binary(bytes.fromhex("6869"), 0)`,
		`.sort([("createdAt", -1), ("status", 1)]).skip(10).limit(5)`)
	mustContain(t, "java", gen(t, "java", s),
		`new Document("_id", new ObjectId("65a1f0c2e4b0a1b2c3d4e5f6"))`, `.append("big", 3000000000L)`,
		`Decimal128.parse("9.99")`, `Date.from(Instant.parse("2024-01-31T12:30:00.123Z"))`,
		`new BsonRegularExpression("^a/b", "i")`, `UUID.fromString("123e4567-e89b-12d3-a456-426614174000")`,
		`Double.NaN`, `.projection(projection).sort(sort).skip(10).limit(5)`)
	mustContain(t, "csharp", gen(t, "csharp", s),
		`{ "_id", new ObjectId("65a1f0c2e4b0a1b2c3d4e5f6") }`, `{ "big", 3000000000L }`,
		`new BsonDecimal128(Decimal128.Parse("9.99"))`, `new DateTime(2024, 1, 31, 12, 30, 0, 123, DateTimeKind.Utc)`,
		`Guid.Parse("123e4567-e89b-12d3-a456-426614174000"), GuidRepresentation.Standard`, `BsonNull.Value`,
		`.Project(projection).Sort(sort).Skip(10).Limit(5)`)
	mustContain(t, "go", gen(t, "go", s),
		`{Key: "_id", Value: oid("65a1f0c2e4b0a1b2c3d4e5f6")}`, `int64(3000000000)`, `dec("9.99")`,
		`time.Date(2024, 1, 31, 12, 30, 0, 123000000, time.UTC)`, `math.NaN()`, `{Key: "qty", Value: bson.D{{Key: "$lt", Value: 5}}}`,
		`SetSkip(10)`, `SetLimit(5)`)
}

func TestAggregateAllLanguages(t *testing.T) {
	s := aggSpec()
	want := map[string]string{
		"mongosh":    `db.getCollection("my-orders").aggregate(`,
		"javascript": `coll.aggregate(pipeline)`,
		"python":     `cursor = coll.aggregate(pipeline)`,
		"java":       `coll.aggregate(pipeline)`,
		"csharp":     `coll.Aggregate(pipeline)`,
		"go":         `coll.Aggregate(ctx, pipeline)`,
	}
	for _, l := range Languages {
		code := gen(t, l.ID, s)
		mustContain(t, l.ID, code, want[l.ID], "$group")
	}
}

func TestErrors(t *testing.T) {
	if _, err := Generate("cobol", richSpec()); err == nil {
		t.Fatal("unknown language accepted")
	}
	if _, err := Generate("go", Spec{Kind: "find"}); err == nil {
		t.Fatal("missing db/coll accepted")
	}
	if _, err := Generate("go", Spec{Kind: "update", DB: "a", Coll: "b"}); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestBalancedDelimiters(t *testing.T) {
	strip := regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|/\^a\\/b/i`)
	for _, s := range []Spec{richSpec(), aggSpec(), {Kind: "find", DB: "d", Coll: "c"}} {
		for _, l := range Languages {
			code := strip.ReplaceAllString(gen(t, l.ID, s), `""`)
			for _, pair := range []string{"()", "[]", "{}"} {
				if strings.Count(code, pair[:1]) != strings.Count(code, pair[1:]) {
					t.Errorf("%s: unbalanced %s", l.ID, pair)
				}
			}
		}
	}
}

// TestGoCompiles builds the generated Go programs against the real driver.
func TestGoCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles generated code")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Skip("go.mod not found")
	}
	// Reuse the project's requirements so the generated module resolves the
	// same (already downloaded) versions; GOPROXY=off below forbids fetching.
	mod := regexp.MustCompile(`(?m)^module .*$`).ReplaceAllString(string(gomod), "module gen")
	gosum, _ := os.ReadFile(filepath.Join(root, "go.sum"))
	for name, s := range map[string]Spec{"find": richSpec(), "agg": aggSpec(), "empty": {Kind: "find", DB: "d", Coll: "c"}} {
		code := gen(t, "go", s)
		if _, err := parser.ParseFile(token.NewFileSet(), "main.go", code, 0); err != nil {
			t.Fatalf("%s: generated Go does not parse: %v\n%s", name, err, code)
		}
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600)
		_ = os.WriteFile(filepath.Join(dir, "go.sum"), gosum, 0o600)
		_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0o600)
		cmd := exec.Command("go", "vet", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: generated Go does not compile: %v\n%s\n%s", name, err, out, code)
		}
	}
}

// TestScriptsParse syntax-checks JavaScript and Python output when the
// interpreters are installed.
func TestScriptsParse(t *testing.T) {
	check := func(lang, tool string, args func(path string) []string, ext string) {
		if _, err := exec.LookPath(tool); err != nil {
			t.Logf("%s not installed; skipping %s syntax check", tool, lang)
			return
		}
		for _, s := range []Spec{richSpec(), aggSpec()} {
			path := filepath.Join(t.TempDir(), "q."+ext)
			_ = os.WriteFile(path, []byte(gen(t, lang, s)), 0o600)
			if out, err := exec.Command(tool, args(path)...).CombinedOutput(); err != nil {
				t.Errorf("%s does not parse: %v\n%s\n%s", lang, err, out, gen(t, lang, s))
			}
		}
	}
	check("javascript", "node", func(p string) []string { return []string{"--check", p} }, "mjs")
	check("python", "python3", func(p string) []string { return []string{"-m", "py_compile", p} }, "py")
}
