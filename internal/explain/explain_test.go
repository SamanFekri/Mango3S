package explain

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func doc(t *testing.T, src string) bson.D {
	t.Helper()
	d, err := mql.ParseDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func hasDiag(p Plan, title string) bool {
	for _, d := range p.Diagnostics {
		if d.Title == title {
			return true
		}
	}
	return false
}

const collScanSort = `{
  queryPlanner: { namespace: "shop.orders", winningPlan: {
    stage: "SORT", sortPattern: { createdAt: -1 }, memLimit: 104857600,
    inputStage: { stage: "COLLSCAN", filter: { status: { $eq: "A" } }, direction: "forward" } },
    rejectedPlans: [] },
  executionStats: { nReturned: 25, executionTimeMillis: 140, totalKeysExamined: 0, totalDocsExamined: 50000,
    executionStages: { stage: "SORT", nReturned: 25, executionTimeMillisEstimate: 130, sortPattern: { createdAt: -1 },
      inputStage: { stage: "COLLSCAN", nReturned: 25, docsExamined: 50000, executionTimeMillisEstimate: 120 } } },
  ok: 1
}`

func TestCollScanWithSort(t *testing.T) {
	p := Analyze(doc(t, collScanSort), doc(t, `{status: "A", total: {$gt: 5}}`), doc(t, `{createdAt: -1}`))
	if p.Namespace != "shop.orders" || p.Summary.Engine != "classic" {
		t.Fatalf("summary: %+v", p.Summary)
	}
	if p.Root == nil || p.Root.Name != "SORT" || p.Root.Children[0].Name != "COLLSCAN" {
		t.Fatalf("tree: %+v", p.Root)
	}
	if *p.Root.Children[0].DocsExamined != 50000 || *p.Root.TimeMs != 130 {
		t.Fatal("per-stage stats missing")
	}
	if !p.Summary.CollScan || !p.Summary.InMemorySort || *p.Summary.ExecutionTimeMs != 140 {
		t.Fatalf("flags: %+v", p.Summary)
	}
	for _, title := range []string{"Collection scan", "In-memory sort", "Low selectivity", "Slow execution"} {
		if !hasDiag(p, title) {
			t.Errorf("missing diagnostic %q in %+v", title, p.Diagnostics)
		}
	}
	if p.Diagnostics[0].Level != "warning" {
		t.Fatal("warnings must come first")
	}
	var scan Diagnostic
	for _, d := range p.Diagnostics {
		if d.Title == "Collection scan" {
			scan = d
		}
	}
	// ESR: equality, sort, range.
	if !strings.Contains(scan.Detail, `{"status": 1, "createdAt": -1, "total": 1}`) {
		t.Fatalf("index suggestion: %s", scan.Detail)
	}
}

func TestCoveredIndexScan(t *testing.T) {
	out := doc(t, `{
	  queryPlanner: { namespace: "shop.users", winningPlan: { stage: "PROJECTION_COVERED", transformBy: { email: 1, _id: 0 },
	    inputStage: { stage: "IXSCAN", indexName: "email_1", keyPattern: { email: 1 }, isMultiKey: false, direction: "forward",
	      indexBounds: { email: ["[\"a@x\", \"a@x\"]"] } } },
	    rejectedPlans: [ { stage: "COLLSCAN" } ] },
	  executionStats: { nReturned: 1, executionTimeMillis: 0, totalKeysExamined: 1, totalDocsExamined: 0,
	    executionStages: { stage: "PROJECTION_COVERED", nReturned: 1,
	      inputStage: { stage: "IXSCAN", nReturned: 1, keysExamined: 1, indexName: "email_1", keyPattern: { email: 1 } } } }
	}`)
	p := Analyze(out, doc(t, `{email: "a@x"}`), nil)
	if !p.Summary.Covered || p.Summary.CollScan || p.Summary.IndexesUsed[0] != "email_1" || p.Summary.RejectedPlans != 1 {
		t.Fatalf("summary: %+v", p.Summary)
	}
	if !hasDiag(p, "Covered query") || !hasDiag(p, "Plan selection") || hasDiag(p, "Collection scan") {
		t.Fatalf("diagnostics: %+v", p.Diagnostics)
	}
	ix := p.Root.Children[0]
	found := false
	for _, d := range ix.Details {
		if d.Key == "keyPattern" && d.Value == `{"email": 1}` {
			found = true
		}
	}
	if !found {
		t.Fatalf("IXSCAN details: %+v", ix.Details)
	}
}

func TestSBEUsesQueryPlan(t *testing.T) {
	out := doc(t, `{
	  queryPlanner: { namespace: "shop.orders", winningPlan: {
	    queryPlan: { stage: "GROUP", inputStage: { stage: "FETCH", inputStage: { stage: "IXSCAN", indexName: "status_1" } } },
	    slotBasedPlan: { slots: "...", stages: "..." } }, rejectedPlans: [] },
	  executionStats: { nReturned: 3, executionTimeMillis: 2, totalKeysExamined: 40, totalDocsExamined: 40,
	    executionStages: { stage: "project", nReturned: 3, inputStage: { stage: "group" } } }
	}`)
	p := Analyze(out, nil, nil)
	if p.Summary.Engine != "sbe" || p.Root.Name != "GROUP" || p.Root.Children[0].Children[0].Name != "IXSCAN" {
		t.Fatalf("sbe tree: %+v / %+v", p.Summary, p.Root)
	}
	if p.Summary.IndexesUsed[0] != "status_1" || !hasDiag(p, "Index used") || hasDiag(p, "Collection scan") {
		t.Fatalf("sbe summary: %+v %+v", p.Summary, p.Diagnostics)
	}
}

func TestAggregationStages(t *testing.T) {
	out := doc(t, `{
	  stages: [
	    { $cursor: { queryPlanner: { namespace: "shop.orders", winningPlan: { stage: "COLLSCAN" }, rejectedPlans: [] },
	                 executionStats: { nReturned: 500, executionTimeMillis: 3, totalDocsExamined: 500, totalKeysExamined: 0,
	                   executionStages: { stage: "COLLSCAN", nReturned: 500, docsExamined: 500 } } },
	      nReturned: 500, executionTimeMillisEstimate: 1 },
	    { $group: { _id: "$status", n: { $sum: 1 } }, nReturned: 4, executionTimeMillisEstimate: 2, usedDisk: true },
	    { $sort: { sortKey: { n: -1 } }, nReturned: 4, executionTimeMillisEstimate: 2 }
	  ], ok: 1 }`)
	p := Analyze(out, nil, nil)
	if p.Root == nil || p.Root.Name != "COLLSCAN" || len(p.Pipeline) != 2 {
		t.Fatalf("plan: root=%+v pipeline=%+v", p.Root, p.Pipeline)
	}
	if p.Pipeline[0].Name != "$group" || *p.Pipeline[0].NReturned != 4 {
		t.Fatalf("pipeline stage: %+v", p.Pipeline[0])
	}
	if !hasDiag(p, "$group spilled to disk") {
		t.Fatalf("diagnostics: %+v", p.Diagnostics)
	}
	// 500 docs: a scan of a small collection is informational, not a warning.
	for _, d := range p.Diagnostics {
		if d.Title == "Collection scan" && d.Level != "info" {
			t.Fatalf("small collscan level %s", d.Level)
		}
	}
}

func TestShardedAndUnknown(t *testing.T) {
	out := doc(t, `{ shards: { rs0: { queryPlanner: { namespace: "a.b", winningPlan: { stage: "COLLSCAN" } } }, rs1: {} } }`)
	p := Analyze(out, nil, nil)
	if !hasDiag(p, "Sharded collection") || p.Root == nil || p.Namespace != "a.b" {
		t.Fatalf("sharded: %+v", p)
	}
	p = Analyze(doc(t, `{ ok: 1 }`), nil, nil)
	if p.Root != nil || !hasDiag(p, "No plan details") || p.Raw == "" {
		t.Fatalf("unknown: %+v", p)
	}
}

func TestSuggestIndexAndRegex(t *testing.T) {
	cases := map[string]string{
		`{a: 1, b: {$gt: 2}, c: {$in: [1, 2]}}`: `{"a": 1, "c": 1, "b": 1}`,
		`{$or: [{a: 1}]}`:                       ``,
		`{name: /^x/}`:                          `{"name": 1}`,
	}
	for f, want := range cases {
		if got := suggestIndex(doc(t, f), nil); got != want {
			t.Errorf("%s: got %q want %q", f, got, want)
		}
	}
	if !hasUnanchoredRegex(doc(t, `{a: /x/}`)) || !hasUnanchoredRegex(doc(t, `{$or: [{a: {$regex: "x"}}]}`)) ||
		hasUnanchoredRegex(doc(t, `{a: /^x/}`)) || !hasUnanchoredRegex(doc(t, `{a: /^x/i}`)) {
		t.Fatal("regex detection")
	}
}

func TestCommandToExplainKeepsOnlyExplainableFields(t *testing.T) {
	cmd, coll, filter, sortDoc, write, err := commandToExplain(`{find: "orders", filter: {status: "A"}, sort: {created: -1}, limit: 5, lsid: {id: 1}, $db: "shop", readConcern: {level: "local"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if coll != "orders" || write || mql.Compact(filter) != `{"status": "A"}` || mql.Compact(sortDoc) != `{"created": -1}` {
		t.Fatalf("coll %q filter %s sort %s write %v", coll, mql.Compact(filter), mql.Compact(sortDoc), write)
	}
	if got := mql.Compact(cmd); got != `{"find": "orders", "filter": {"status": "A"}, "sort": {"created": -1}, "limit": 5}` {
		t.Fatalf("cmd = %s", got)
	}
	if _, _, _, _, write, _ := commandToExplain(`{aggregate: "o", pipeline: [{$match: {a: 1}}, {$out: "x"}]}`); !write {
		t.Fatal("$out must be planned only")
	}
	if _, _, _, _, _, err := commandToExplain(`{delete: "o", deletes: []}`); err == nil {
		t.Fatal("write commands are refused")
	}
}
