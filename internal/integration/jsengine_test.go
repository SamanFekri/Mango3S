//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/jsengine"
	"mango3s/internal/mongox"
	"mango3s/internal/shell"
)

func outputsText(res shell.Result) string {
	var b strings.Builder
	for _, o := range res.Outputs {
		b.WriteString(o.Kind + ": " + o.Text + "\n")
		for _, d := range o.Documents {
			b.WriteString(d.Text + "\n")
		}
	}
	return b.String()
}

func TestJavaScriptEngineOnRealServer(t *testing.T) {
	e := setup(t)
	js := jsengine.New(e.mgr, 50)
	script := `
use ` + e.db + `
function square(x) { return x * x }
const docs = []
for (let i = 1; i <= 120; i++) docs.push({ i, sq: square(i), even: i % 2 === 0, at: ISODate("2026-01-01T00:00:00Z"), big: NumberLong("9007199254740993") })
db.nums.insertMany(docs)
let sum = 0
db.nums.find({ even: true }).forEach(d => { sum += d.i })
print("sum of evens", sum)
const one = db.nums.findOne({ i: 3 })
printjson({ i: one.i, sq: one.sq, big: one.big, isDate: one.at instanceof Date })
print("count", db.nums.countDocuments({ sq: { $gt: 100 } }))
print("agg", db.nums.aggregate([{ $group: { _id: "$even", n: { $sum: 1 } } }, { $sort: { _id: 1 } }]).toArray().map(g => g.n).join(","))
db.nums.find({}, { _id: 0, i: 1 }).sort({ i: -1 })
`
	res, err := js.Run(e.ctx, e.conn, "test", script)
	if err != nil {
		t.Fatal(err)
	}
	out := outputsText(res)
	for _, want := range []string{"sum of evens 3660", `"i": 3`, `"sq": 9`, `NumberLong(9007199254740993)`, `"isDate": true`, "count 110", "agg 60,60"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	last := res.Outputs[len(res.Outputs)-1]
	if last.Kind != "documents" || len(last.Documents) != 50 || !last.HasMore || !strings.Contains(last.Documents[0].Text, "120") || res.CurrentDB != e.db {
		t.Fatalf("result page: kind %s, %d docs, more %v, db %s", last.Kind, len(last.Documents), last.HasMore, res.CurrentDB)
	}

	// Errors are reported with the script's message, not as a failed run.
	res, err = js.Run(e.ctx, e.conn, e.db, "let x = 1\nthrow new Error('nope ' + x)")
	if err != nil || res.Outputs[0].Kind != "error" || !strings.Contains(res.Outputs[0].Text, "nope 1") {
		t.Fatalf("error output: %+v %v", res.Outputs, err)
	}

	// Read-only connections refuse writes but can read.
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if _, err := e.mgr.Connect(e.ctx, config.Profile{ID: "ro-js", Name: "ro", URI: uri, ReadOnly: true, ConnectTimeoutSeconds: 5}, mongox.Credentials{}); err != nil {
		t.Fatal(err)
	}
	res, _ = js.Run(e.ctx, "ro-js", e.db, "print(db.nums.countDocuments({}))\ndb.nums.deleteMany({})")
	if out := outputsText(res); !strings.Contains(out, "text: 120") || !strings.Contains(out, "read-only") {
		t.Fatalf("read-only: %s", out)
	}
	res, _ = js.Run(e.ctx, "ro-js", e.db, "db.runCommand({ drop: 'nums' })")
	if out := outputsText(res); !strings.Contains(out, "read-only") {
		t.Fatalf("read-only runCommand: %s", out)
	}

	// Canceling stops an endless loop.
	ctx, cancel := context.WithTimeout(e.ctx, time.Second)
	defer cancel()
	start := time.Now()
	if _, err := js.Run(ctx, e.conn, e.db, "while (true) {}"); !apperr.Is(err, apperr.CodeTimeout) && !apperr.Is(err, apperr.CodeCanceled) {
		t.Fatalf("endless loop: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the loop was not interrupted")
	}
}
