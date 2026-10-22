// Package explain runs explain for find queries and aggregation pipelines,
// normalizes the server output (classic and slot-based engines, aggregation
// $cursor stages, sharded results) into a stage tree, and derives diagnostics.
package explain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/query"
)

// Detail is a labeled property of a stage.
type Detail struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Stage is a node of the plan tree.
type Stage struct {
	Name         string   `json:"name"`
	Details      []Detail `json:"details"`
	NReturned    *int64   `json:"nReturned,omitempty"`
	DocsExamined *int64   `json:"docsExamined,omitempty"`
	KeysExamined *int64   `json:"keysExamined,omitempty"`
	TimeMs       *int64   `json:"timeMs,omitempty"`
	Children     []Stage  `json:"children"`
}

// Summary holds whole-query numbers.
type Summary struct {
	ExecutionTimeMs *int64   `json:"executionTimeMs,omitempty"`
	NReturned       *int64   `json:"nReturned,omitempty"`
	DocsExamined    *int64   `json:"docsExamined,omitempty"`
	KeysExamined    *int64   `json:"keysExamined,omitempty"`
	IndexesUsed     []string `json:"indexesUsed"`
	CollScan        bool     `json:"collScan"`
	InMemorySort    bool     `json:"inMemorySort"`
	Covered         bool     `json:"covered"`
	RejectedPlans   int      `json:"rejectedPlans"`
	Engine          string   `json:"engine"` // classic | sbe | unknown
}

// Diagnostic is a finding about the plan.
type Diagnostic struct {
	Level  string `json:"level"` // good | info | warning
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Plan is the normalized explain result.
type Plan struct {
	Namespace   string       `json:"namespace"`
	Root        *Stage       `json:"root"`
	Pipeline    []Stage      `json:"pipeline"` // aggregation stages after the query layer
	Summary     Summary      `json:"summary"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Raw         string       `json:"raw"`
}

// Request selects what to explain. Kind "find" uses Query; "aggregate" uses Stages.
type Request struct {
	Kind   string            `json:"kind"`
	Query  query.Request     `json:"query"`
	ConnID string            `json:"connId"`
	DB     string            `json:"db"`
	Coll   string            `json:"coll"`
	Stages []aggregate.Stage `json:"stages"`
	// Command (kind "command") is a find, aggregate, count or distinct command
	// in mongosh syntax, e.g. a profiled query from the index advisor.
	Command string `json:"command,omitempty"`
}

// Service runs explain.
type Service struct{ src mongox.Source }

// New returns an explain service.
func New(src mongox.Source) *Service { return &Service{src: src} }

// Explain runs the query with executionStats verbosity (pipelines that write
// with $out/$merge are only planned, never executed) and analyzes the output.
func (s *Service) Explain(ctx context.Context, req Request) (Plan, error) {
	var cmd bson.D
	var filter bson.D
	var sortDoc bson.D
	verbosity := "executionStats"
	switch req.Kind {
	case "find":
		q := req.Query
		req.ConnID, req.DB, req.Coll = q.ConnID, q.DB, q.Coll
		p, err := query.Parse(q)
		if err != nil {
			return Plan{}, err
		}
		filter, sortDoc = p.Filter, p.Sort
		find := bson.D{{Key: "find", Value: q.Coll}, {Key: "filter", Value: p.Filter}}
		if len(p.Projection) > 0 {
			find = append(find, bson.E{Key: "projection", Value: p.Projection})
		}
		if len(p.Sort) > 0 {
			find = append(find, bson.E{Key: "sort", Value: p.Sort})
		}
		if q.Skip > 0 {
			find = append(find, bson.E{Key: "skip", Value: q.Skip})
		}
		if q.Limit > 0 {
			find = append(find, bson.E{Key: "limit", Value: q.Limit})
		}
		cmd = bson.D{{Key: "explain", Value: find}}
	case "aggregate":
		pipe, err := aggregate.Build(req.Stages, -1)
		if err != nil {
			return Plan{}, err
		}
		if aggregate.IsWrite(pipe) {
			verbosity = "queryPlanner"
		}
		if len(pipe) > 0 {
			if m, ok := mql.Lookup(pipe[0], "$match"); ok {
				filter, _ = m.(bson.D)
			}
		}
		cmd = bson.D{{Key: "explain", Value: bson.D{
			{Key: "aggregate", Value: req.Coll}, {Key: "pipeline", Value: pipe}, {Key: "cursor", Value: bson.D{}},
		}}}
	case "command":
		inner, coll, f, so, write, err := commandToExplain(req.Command)
		if err != nil {
			return Plan{}, err
		}
		if write {
			verbosity = "queryPlanner"
		}
		req.Coll, filter, sortDoc = coll, f, so
		cmd = bson.D{{Key: "explain", Value: inner}}
	default:
		return Plan{}, apperr.Invalid("Unknown explain kind %q.", req.Kind)
	}
	cmd = append(cmd, bson.E{Key: "verbosity", Value: verbosity})

	sess, err := s.src.Session(req.ConnID)
	if err != nil {
		return Plan{}, err
	}
	if req.DB == "" || req.Coll == "" {
		return Plan{}, apperr.Invalid("Database and collection are required.")
	}
	var out bson.D
	if err := sess.Client.Database(req.DB).RunCommand(ctx, cmd).Decode(&out); err != nil {
		return Plan{}, apperr.From(err)
	}
	return Analyze(out, filter, sortDoc), nil
}

// m is a convenience accessor over bson.D.
type m bson.D

func (d m) get(key string) any {
	v, _ := mql.Lookup(bson.D(d), key)
	return v
}

func (d m) doc(key string) m {
	if v, ok := d.get(key).(bson.D); ok {
		return m(v)
	}
	return nil
}

func (d m) str(key string) string {
	s, _ := d.get(key).(string)
	return s
}

func (d m) num(key string) *int64 {
	switch n := d.get(key).(type) {
	case int32:
		v := int64(n)
		return &v
	case int64:
		return &n
	case float64:
		v := int64(n)
		return &v
	}
	return nil
}

// Analyze normalizes raw explain output. filter and sort (optional) improve
// index suggestions.
func Analyze(out bson.D, filter, sortDoc bson.D) Plan {
	p := Plan{Raw: mql.Pretty(out), Pipeline: []Stage{}, Diagnostics: []Diagnostic{}}
	root := m(out)

	// Sharded output: analyze the first shard and say so.
	if shards := root.doc("shards"); shards != nil && root.doc("queryPlanner") == nil {
		for _, e := range shards {
			if sd, ok := e.Value.(bson.D); ok {
				p.Diagnostics = append(p.Diagnostics, Diagnostic{Level: "info", Title: "Sharded collection",
					Detail: fmt.Sprintf("Showing the plan of shard %q (%d shards answered).", e.Key, len(shards))})
				root = m(sd)
				break
			}
		}
	}

	var qp, es m
	if stages, ok := root.get("stages").(bson.A); ok {
		for i, st := range stages {
			sd, ok := st.(bson.D)
			if !ok || len(sd) == 0 {
				continue
			}
			sm := m(sd)
			if i == 0 && sd[0].Key == "$cursor" {
				cur := sm.doc("$cursor")
				qp, es = cur.doc("queryPlanner"), cur.doc("executionStats")
				continue
			}
			name := sd[0].Key
			stage := Stage{Name: name, Children: []Stage{}, NReturned: sm.num("nReturned"), TimeMs: sm.num("executionTimeMillisEstimate")}
			if v := sd[0].Value; v != nil {
				stage.Details = append(stage.Details, Detail{Key: "spec", Value: clip(mql.Compact(v), 300)})
			}
			if sm.get("usedDisk") == true {
				stage.Details = append(stage.Details, Detail{Key: "usedDisk", Value: "true"})
			}
			p.Pipeline = append(p.Pipeline, stage)
		}
	} else {
		qp, es = root.doc("queryPlanner"), root.doc("executionStats")
	}

	if qp != nil {
		p.Namespace = qp.str("namespace")
		if rej, ok := qp.get("rejectedPlans").(bson.A); ok {
			p.Summary.RejectedPlans = len(rej)
		}
	}
	winning := qp.doc("winningPlan")
	p.Summary.Engine = "classic"
	planTree := winning
	if qplan := winning.doc("queryPlan"); qplan != nil {
		planTree = qplan
		p.Summary.Engine = "sbe"
	}
	if winning == nil {
		p.Summary.Engine = "unknown"
	}

	if es != nil {
		p.Summary.ExecutionTimeMs = es.num("executionTimeMillis")
		p.Summary.NReturned = es.num("nReturned")
		p.Summary.DocsExamined = es.num("totalDocsExamined")
		p.Summary.KeysExamined = es.num("totalKeysExamined")
	}
	// Prefer executionStages (it carries per-stage stats) when it uses classic stage names.
	if exec := es.doc("executionStages"); exec != nil && isClassic(exec.str("stage")) {
		t := buildTree(exec)
		p.Root = &t
	} else if planTree != nil {
		t := buildTree(planTree)
		p.Root = &t
	}
	if p.Root != nil {
		walk(*p.Root, func(s Stage) {
			switch s.Name {
			case "COLLSCAN":
				p.Summary.CollScan = true
			case "SORT":
				p.Summary.InMemorySort = true
			case "IXSCAN", "DISTINCT_SCAN", "COUNT_SCAN", "EXPRESS_IXSCAN":
				for _, d := range s.Details {
					if d.Key == "indexName" && !contains(p.Summary.IndexesUsed, d.Value) {
						p.Summary.IndexesUsed = append(p.Summary.IndexesUsed, d.Value)
					}
				}
			}
		})
		p.Summary.Covered = len(p.Summary.IndexesUsed) > 0 && !hasStage(*p.Root, "FETCH") && !p.Summary.CollScan &&
			p.Summary.DocsExamined != nil && *p.Summary.DocsExamined == 0
	}
	if p.Summary.IndexesUsed == nil {
		p.Summary.IndexesUsed = []string{}
	}
	p.Diagnostics = append(p.Diagnostics, diagnose(p, filter, sortDoc)...)
	return p
}

var classicStages = map[string]bool{
	"COLLSCAN": true, "IXSCAN": true, "FETCH": true, "SORT": true, "PROJECTION_SIMPLE": true,
	"PROJECTION_DEFAULT": true, "PROJECTION_COVERED": true, "LIMIT": true, "SKIP": true, "SHARDING_FILTER": true,
	"OR": true, "AND_SORTED": true, "AND_HASH": true, "SORT_MERGE": true, "COUNT_SCAN": true, "DISTINCT_SCAN": true,
	"IDHACK": true, "EOF": true, "TEXT_MATCH": true, "SUBPLAN": true, "CACHED_PLAN": true, "MULTI_PLAN": true,
	"EXPRESS_IXSCAN": true, "EXPRESS_CLUSTERED_IXSCAN": true, "BATCHED_DELETE": true, "COUNT": true,
	"SORT_KEY_GENERATOR": true, "CLUSTERED_IXSCAN": true,
}

func isClassic(name string) bool { return classicStages[name] }

var detailKeys = []string{"indexName", "keyPattern", "direction", "isMultiKey", "indexBounds", "filter",
	"sortPattern", "memLimit", "limitAmount", "skipAmount", "transformBy", "usedDisk", "spills", "type"}

func buildTree(d m) Stage {
	s := Stage{Name: d.str("stage"), Children: []Stage{}, Details: []Detail{}}
	if s.Name == "" {
		s.Name = "?"
	}
	s.NReturned = d.num("nReturned")
	s.DocsExamined = d.num("docsExamined")
	s.KeysExamined = d.num("keysExamined")
	s.TimeMs = d.num("executionTimeMillisEstimate")
	for _, k := range detailKeys {
		if v := d.get(k); v != nil {
			val := ""
			if str, ok := v.(string); ok {
				val = str
			} else {
				val = mql.Compact(v)
			}
			s.Details = append(s.Details, Detail{Key: k, Value: clip(val, 300)})
		}
	}
	for _, key := range []string{"inputStage", "outerStage", "innerStage", "thenStage", "elseStage"} {
		if c := d.doc(key); c != nil {
			s.Children = append(s.Children, buildTree(c))
		}
	}
	if arr, ok := d.get("inputStages").(bson.A); ok {
		for _, c := range arr {
			if cd, ok := c.(bson.D); ok {
				s.Children = append(s.Children, buildTree(m(cd)))
			}
		}
	}
	return s
}

func walk(s Stage, f func(Stage)) {
	f(s)
	for _, c := range s.Children {
		walk(c, f)
	}
}

func hasStage(s Stage, name string) bool {
	found := false
	walk(s, func(x Stage) {
		if x.Name == name {
			found = true
		}
	})
	return found
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func diagnose(p Plan, filter, sortDoc bson.D) []Diagnostic {
	var out []Diagnostic
	s := p.Summary
	val := func(n *int64) int64 {
		if n == nil {
			return -1
		}
		return *n
	}
	returned, docs, keys, ms := val(s.NReturned), val(s.DocsExamined), val(s.KeysExamined), val(s.ExecutionTimeMs)

	if s.CollScan {
		detail := "MongoDB read every document in the collection."
		if docs >= 0 {
			detail = fmt.Sprintf("MongoDB examined %d documents to return %d.", docs, max(returned, 0))
		}
		if idx := suggestIndex(filter, sortDoc); idx != "" {
			detail += " Consider an index such as " + idx + "."
		}
		level := "warning"
		if docs >= 0 && docs < 1000 {
			level = "info" // small collections are fine to scan
		}
		out = append(out, Diagnostic{Level: level, Title: "Collection scan", Detail: detail})
	}
	if s.InMemorySort {
		detail := "Results are sorted in memory instead of being read in index order; large sorts are slow and limited to 100 MB without allowDiskUse."
		if idx := suggestIndex(filter, sortDoc); idx != "" && len(sortDoc) > 0 {
			detail += " An index ending in the sort fields, e.g. " + idx + ", avoids the sort."
		}
		out = append(out, Diagnostic{Level: "warning", Title: "In-memory sort", Detail: detail})
	}
	for _, st := range p.Pipeline {
		for _, d := range st.Details {
			if d.Key == "usedDisk" {
				out = append(out, Diagnostic{Level: "warning", Title: st.Name + " spilled to disk", Detail: "The stage exceeded its memory limit and wrote temporary files."})
			}
		}
	}
	if returned >= 0 && docs > 1000 && docs > 10*max(returned, 1) {
		out = append(out, Diagnostic{Level: "warning", Title: "Low selectivity",
			Detail: fmt.Sprintf("%.0f documents examined per document returned. A more selective index would help.", float64(docs)/float64(max(returned, 1)))})
	} else if !s.CollScan && len(s.IndexesUsed) > 0 && keys > 1000 && keys > 10*max(returned, 1) {
		out = append(out, Diagnostic{Level: "info", Title: "Many index keys scanned",
			Detail: fmt.Sprintf("%d index keys examined for %d results; the index prefix may not match the filter well.", keys, max(returned, 0))})
	}
	if s.Covered {
		out = append(out, Diagnostic{Level: "good", Title: "Covered query", Detail: "The index alone answered the query; no documents were fetched."})
	} else if len(s.IndexesUsed) > 0 && !s.CollScan && (docs < 0 || docs <= 2*max(returned, 1)+10) {
		out = append(out, Diagnostic{Level: "good", Title: "Efficient index use",
			Detail: "Uses " + strings.Join(s.IndexesUsed, ", ") + " and examines about as many documents as it returns."})
	} else if len(s.IndexesUsed) > 0 && !s.CollScan {
		out = append(out, Diagnostic{Level: "good", Title: "Index used", Detail: "Uses " + strings.Join(s.IndexesUsed, ", ") + "."})
	}
	if ms >= 100 {
		out = append(out, Diagnostic{Level: "info", Title: "Slow execution", Detail: fmt.Sprintf("Execution took %d ms on the server.", ms)})
	}
	if s.RejectedPlans > 0 {
		out = append(out, Diagnostic{Level: "info", Title: "Plan selection",
			Detail: fmt.Sprintf("The planner compared %d alternative plan(s) before choosing this one.", s.RejectedPlans)})
	}
	if hasUnanchoredRegex(filter) {
		out = append(out, Diagnostic{Level: "info", Title: "Unanchored regular expression",
			Detail: "Regexes that don't start with ^ (or are case-insensitive) cannot use an index efficiently."})
	}
	if p.Root == nil && len(p.Pipeline) == 0 {
		out = append(out, Diagnostic{Level: "info", Title: "No plan details", Detail: "The server returned an explain format this version does not recognize; see the raw output."})
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].Level) < rank(out[j].Level) })
	return out
}

func rank(level string) int {
	switch level {
	case "warning":
		return 0
	case "info":
		return 1
	}
	return 2
}

// suggestIndex proposes an ESR (equality, sort, range) index for the filter and sort.
func suggestIndex(filter, sortDoc bson.D) string {
	var equality, ranges []string
	for _, e := range filter {
		if strings.HasPrefix(e.Key, "$") {
			continue
		}
		if d, ok := e.Value.(bson.D); ok && len(d) > 0 && strings.HasPrefix(d[0].Key, "$") {
			switch d[0].Key {
			case "$eq", "$in":
				equality = append(equality, e.Key)
			default:
				ranges = append(ranges, e.Key)
			}
			continue
		}
		if _, isRegex := e.Value.(bson.Regex); isRegex {
			ranges = append(ranges, e.Key)
			continue
		}
		equality = append(equality, e.Key)
	}
	keys := bson.D{}
	seen := map[string]bool{}
	add := func(k string, dir int32) {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, bson.E{Key: k, Value: dir})
		}
	}
	for _, k := range equality {
		add(k, 1)
	}
	for _, e := range sortDoc {
		dir := int32(1)
		if n, ok := e.Value.(int32); ok && n < 0 {
			dir = -1
		}
		add(e.Key, dir)
	}
	for _, k := range ranges {
		add(k, 1)
	}
	if len(keys) == 0 {
		return ""
	}
	return mql.Compact(keys)
}

func hasUnanchoredRegex(filter bson.D) bool {
	found := false
	var visit func(v any)
	visit = func(v any) {
		switch t := v.(type) {
		case bson.Regex:
			if !strings.HasPrefix(t.Pattern, "^") || strings.Contains(t.Options, "i") {
				found = true
			}
		case bson.D:
			for _, e := range t {
				if e.Key == "$regex" {
					if s, ok := e.Value.(string); ok && !strings.HasPrefix(s, "^") {
						found = true
					}
				}
				visit(e.Value)
			}
		case bson.A:
			for _, x := range t {
				visit(x)
			}
		}
	}
	visit(filter)
	return found
}

// explainable lists, per command, the fields explain accepts. Everything
// else in a profiled command (lsid, $db, $clusterTime, readConcern…) is
// dropped.
var explainable = map[string][]string{
	"find":      {"filter", "projection", "sort", "skip", "limit", "hint", "collation", "min", "max", "let"},
	"aggregate": {"pipeline", "hint", "collation", "let", "allowDiskUse"},
	"count":     {"query", "limit", "skip", "hint", "collation"},
	"distinct":  {"key", "query", "collation"},
}

// commandToExplain turns a command document into the command to explain,
// and returns its collection and the filter and sort used for diagnostics.
// write reports a pipeline with $out or $merge, which must only be planned.
func commandToExplain(text string) (cmd bson.D, coll string, filter, sortDoc bson.D, write bool, err error) {
	doc, err := mql.ParseDocument(text)
	if err != nil {
		return nil, "", nil, nil, false, apperr.Invalid("The command can't be read: %v", err)
	}
	if len(doc) == 0 {
		return nil, "", nil, nil, false, apperr.Invalid("The command is empty.")
	}
	name := doc[0].Key
	keep, ok := explainable[name]
	if !ok {
		return nil, "", nil, nil, false, apperr.Invalid("Only find, aggregate, count and distinct commands can be explained here, not %q.", name)
	}
	coll, _ = doc[0].Value.(string)
	if coll == "" {
		return nil, "", nil, nil, false, apperr.Invalid("The command names no collection.")
	}
	cmd = bson.D{{Key: name, Value: coll}}
	for _, k := range keep {
		if v, ok := mql.Lookup(doc, k); ok {
			cmd = append(cmd, bson.E{Key: k, Value: v})
		}
	}
	switch name {
	case "find":
		if v, ok := mql.Lookup(doc, "filter"); ok {
			filter, _ = v.(bson.D)
		}
		if v, ok := mql.Lookup(doc, "sort"); ok {
			sortDoc, _ = v.(bson.D)
		}
	case "aggregate":
		cmd = append(cmd, bson.E{Key: "cursor", Value: bson.D{}})
		pipe, _ := mql.Lookup(doc, "pipeline")
		if arr, ok := pipe.(bson.A); ok {
			for i, st := range arr {
				sd, _ := st.(bson.D)
				if i == 0 {
					if m, ok := mql.Lookup(sd, "$match"); ok {
						filter, _ = m.(bson.D)
					}
				}
				if _, ok := mql.Lookup(sd, "$out"); ok {
					write = true
				}
				if _, ok := mql.Lookup(sd, "$merge"); ok {
					write = true
				}
			}
		}
	case "count", "distinct":
		if v, ok := mql.Lookup(doc, "query"); ok {
			filter, _ = v.(bson.D)
		}
	}
	return cmd, coll, filter, sortDoc, write, nil
}
