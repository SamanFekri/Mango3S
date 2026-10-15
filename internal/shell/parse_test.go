package shell

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParseStatements(t *testing.T) {
	script := `
show dbs
use my-db;
// comment line
db.users.find({age: {$gt: 21}}, {name: 1})
  .sort({name: 1})
  .limit(5);  db.getCollection("odd name").countDocuments()
db["x"].insertOne({a: ObjectId("5f1d7f0b2c3a4b5c6d7e8f90")})
db.orders.items.findOne()
db.runCommand({ping: 1})
help`
	stmts, err := Parse(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 8 {
		for _, s := range stmts {
			t.Logf("%q", s.text)
		}
		t.Fatalf("got %d statements", len(stmts))
	}
	if stmts[0].kind != "show" || stmts[0].word != "dbs" {
		t.Errorf("show: %+v", stmts[0])
	}
	if stmts[1].kind != "use" || stmts[1].word != "my-db" {
		t.Errorf("use: %+v", stmts[1])
	}
	find := stmts[2]
	if describe(find.segs) != "db.users.find().sort().limit()" {
		t.Errorf("find chain: %s", describe(find.segs))
	}
	if !strings.HasPrefix(find.text, "db.users.find(") || !strings.HasSuffix(find.text, ".limit(5)") {
		t.Errorf("statement text: %q", find.text)
	}
	if f, ok := find.segs[1].args[0].(bson.D); !ok || f[0].Key != "age" {
		t.Errorf("find args: %#v", find.segs[1].args)
	}
	if describe(stmts[3].segs) != "db.getCollection().countDocuments()" {
		t.Errorf("getCollection: %s", describe(stmts[3].segs))
	}
	if stmts[4].segs[0].name != "x" {
		t.Errorf("bracket access: %+v", stmts[4].segs)
	}
	if k := firstCall(stmts[5].segs); k != 2 {
		t.Errorf("dotted collection name: first call at %d", k)
	}
	if stmts[7].kind != "help" {
		t.Errorf("help: %+v", stmts[7])
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		`var x = 1`:                         "not supported",
		`db.users.find({a: 1}`:              "end of input",
		`db.users.find(function() {})`:      "functions are not supported",
		`foo()`:                             "unknown command",
		`use`:                               "requires an argument",
		`it`:                                "not supported",
		`db.users.find()()`:                 "unexpected '('",
		`db.users.find({a: 1}).sort({b: 1}`: "end of input",
		`db.`:                               "expected a name",
	}
	for src, want := range cases {
		_, err := Parse(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want error containing %q, got %v", src, want, err)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	if humanBytes(512) != "512 B" || humanBytes(2048) != "2.00 KiB" || humanBytes(5<<20) != "5.00 MiB" {
		t.Fatal(humanBytes(512), humanBytes(2048), humanBytes(5<<20))
	}
}
