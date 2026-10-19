package manage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// The index advisor groups profiled operations by query shape (which fields
// are matched exactly, by range, and sorted), proposes an index for each
// shape following the equality → sort → range rule, and checks the existing
// indexes for ones that are unused or made redundant by a longer index.

// KeyField is one field of an index key pattern.
type KeyField struct {
	Field string `json:"field"`
	Dir   any    `json:"dir"` // 1, -1, or a special type such as "text"
}

// QueryShape is a group of profiled operations with the same shape.
type QueryShape struct {
	Coll         string    `json:"coll"`
	Op           string    `json:"op"`
	Shape        string    `json:"shape"` // e.g. {status: =, age: >} sort {created: -1}
	Count        int       `json:"count"`
	TotalMillis  int64     `json:"totalMillis"`
	AvgMillis    int64     `json:"avgMillis"`
	MaxMillis    int64     `json:"maxMillis"`
	DocsExamined int64     `json:"docsExamined"`
	KeysExamined int64     `json:"keysExamined"`
	Returned     int64     `json:"returned"`
	Plans        []string  `json:"plans"`
	Example      string    `json:"example"`
	LastSeen     time.Time `json:"lastSeen"`
	// Suggestion. Status: missing (no index serves it), partial (an index
	// serves part of it), covered, hidden (a hidden index would serve it),
	// none (nothing to suggest, see Reason).
	Status     string `json:"status"`
	Suggested  string `json:"suggested,omitempty"` // key pattern in mongosh syntax
	Name       string `json:"name,omitempty"`      // the index name the server would give it
	ServedBy   string `json:"servedBy,omitempty"`  // existing index for covered/partial/hidden
	Reason     string `json:"reason"`
	suggestion []KeyField
}

// IndexFinding is an index that may not be needed.
type IndexFinding struct {
	Coll   string `json:"coll"`
	Name   string `json:"name"`
	Keys   string `json:"keys"`
	Reason string `json:"reason"`
	Size   int64  `json:"size"`
	Hidden bool   `json:"hidden"`
}

// Advice is the advisor's report.
type Advice struct {
	ProfilingLevel int            `json:"profilingLevel"`
	Analyzed       int            `json:"analyzed"` // profiled operations read
	Since          *time.Time     `json:"since,omitempty"`
	Shapes         []QueryShape   `json:"shapes"`
	Unused         []IndexFinding `json:"unused"`
	Redundant      []IndexFinding `json:"redundant"`
	Notes          []string       `json:"notes"`
}

// AdviceOptions narrows the analysis.
type AdviceOptions struct {
	Coll      string `json:"coll"`      // empty: every collection in the database
	MinMillis int    `json:"minMillis"` // ignore faster operations
	Limit     int    `json:"limit"`     // newest profiled operations to read; default 5000
}

// ---- query shapes ---------------------------------------------------------------

type predicates struct {
	eq, rng []string
	notes   []string
	blocked string // why no single index can serve it
}

func (p *predicates) add(field, kind string) {
	for _, f := range p.eq {
		if f == field {
			return
		}
	}
	if kind == "eq" {
		// An equality wins over a range on the same field.
		for i, f := range p.rng {
			if f == field {
				p.rng = append(p.rng[:i], p.rng[i+1:]...)
				break
			}
		}
		p.eq = append(p.eq, field)
		return
	}
	for _, f := range p.rng {
		if f == field {
			return
		}
	}
	p.rng = append(p.rng, field)
}

var eqOps = map[string]bool{"$eq": true, "$in": true, "$all": true, "$elemMatch": true, "$size": true}
var rangeOps = map[string]bool{"$gt": true, "$gte": true, "$lt": true, "$lte": true, "$ne": true, "$nin": true, "$exists": true,
	"$regex": true, "$options": true, "$type": true, "$mod": true, "$not": true, "$bitsAllSet": true, "$bitsAnySet": true, "$bitsAllClear": true, "$bitsAnyClear": true}
var geoOps = map[string]bool{"$near": true, "$nearSphere": true, "$geoWithin": true, "$geoIntersects": true}

// classify sorts a filter's fields into equality and range predicates.
func classify(filter bson.D, p *predicates) {
	for _, e := range filter {
		switch e.Key {
		case "$and":
			if arr, ok := e.Value.(bson.A); ok {
				for _, x := range arr {
					if d, ok := x.(bson.D); ok {
						classify(d, p)
					}
				}
			}
			continue
		case "$or", "$nor":
			p.blocked = "uses " + e.Key + ": each branch is served by its own index, so look at the branches separately"
			continue
		case "$text":
			p.blocked = "uses $text, which needs a text index"
			continue
		case "$where", "$expr", "$jsonSchema", "$function":
			p.notes = append(p.notes, e.Key+" can't use indexes for its condition")
			continue
		case "$comment":
			continue
		}
		if strings.HasPrefix(e.Key, "$") {
			continue
		}
		kind := "eq"
		switch v := e.Value.(type) {
		case bson.D:
			if len(v) > 0 && strings.HasPrefix(v[0].Key, "$") {
				kind = ""
				for _, op := range v {
					switch {
					case eqOps[op.Key]:
						if kind == "" {
							kind = "eq"
						}
					case rangeOps[op.Key]:
						kind = "range"
					case geoOps[op.Key]:
						p.blocked = "uses " + op.Key + " on " + e.Key + ", which needs a 2dsphere index"
						kind = "skip"
					}
				}
			}
		case bson.Regex:
			kind = "range"
		}
		if kind == "eq" || kind == "range" {
			p.add(e.Key, kind)
		}
	}
}

func sortFields(s bson.D) []KeyField {
	var out []KeyField
	for _, e := range s {
		dir := 0
		switch v := e.Value.(type) {
		case int32:
			dir = int(v)
		case int64:
			dir = int(v)
		case float64:
			dir = int(v)
		}
		if dir > 0 {
			dir = 1
		} else if dir < 0 {
			dir = -1
		} else {
			continue // {$meta: …}
		}
		out = append(out, KeyField{Field: e.Key, Dir: dir})
	}
	return out
}

func lookupD(d bson.D, key string) (any, bool) {
	for _, e := range d {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

func docAt(d bson.D, key string) bson.D {
	v, _ := lookupD(d, key)
	x, _ := v.(bson.D)
	return x
}

// extract finds the filter and sort of a profiled command.
func extract(op string, cmd bson.D) (filter, sortDoc bson.D, ok bool) {
	if len(cmd) == 0 {
		return nil, nil, false
	}
	switch cmd[0].Key {
	case "find":
		return docAt(cmd, "filter"), docAt(cmd, "sort"), true
	case "count", "distinct":
		return docAt(cmd, "query"), nil, true
	case "findAndModify", "findandmodify":
		return docAt(cmd, "query"), docAt(cmd, "sort"), true
	case "aggregate":
		pipe, _ := lookupD(cmd, "pipeline")
		arr, _ := pipe.(bson.A)
		var f, s bson.D
		for i, st := range arr {
			d, ok := st.(bson.D)
			if !ok || len(d) == 0 {
				break
			}
			switch d[0].Key {
			case "$match":
				if i == 0 || (i == 1 && s != nil) {
					f, _ = d[0].Value.(bson.D)
					continue
				}
			case "$sort":
				if s == nil && i <= 1 {
					s, _ = d[0].Value.(bson.D)
					continue
				}
			}
			break
		}
		return f, s, f != nil || s != nil
	}
	// update / remove ops record the statement as {q: …, u: …}.
	if op == "update" || op == "remove" {
		if q, ok := lookupD(cmd, "q"); ok {
			d, _ := q.(bson.D)
			return d, nil, true
		}
	}
	return nil, nil, false
}

// suggest orders an index by equality, then sort, then range fields.
func suggest(p predicates, s []KeyField) []KeyField {
	var out []KeyField
	seen := map[string]bool{}
	add := func(f string, dir int) {
		if !seen[f] {
			seen[f] = true
			out = append(out, KeyField{Field: f, Dir: dir})
		}
	}
	for _, f := range p.eq {
		add(f, 1)
	}
	for _, k := range s {
		add(k.Field, k.Dir.(int))
	}
	for _, f := range p.rng {
		add(f, 1)
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func shapeText(p predicates, s []KeyField) string {
	var parts []string
	for _, f := range p.eq {
		parts = append(parts, keyText(f)+": =")
	}
	for _, f := range p.rng {
		parts = append(parts, keyText(f)+": range")
	}
	out := "{" + strings.Join(parts, ", ") + "}"
	if len(s) > 0 {
		var sp []string
		for _, k := range s {
			sp = append(sp, fmt.Sprintf("%s: %d", keyText(k.Field), k.Dir))
		}
		out += " sort {" + strings.Join(sp, ", ") + "}"
	}
	return out
}

func keyText(f string) string {
	for i, r := range f {
		if !(r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return mql.Quote(f)
		}
	}
	return f
}

// KeysText renders a key pattern in mongosh syntax.
func KeysText(keys []KeyField) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		switch d := k.Dir.(type) {
		case string:
			parts[i] = fmt.Sprintf("%s: %s", keyText(k.Field), mql.Quote(d))
		default:
			parts[i] = fmt.Sprintf("%s: %v", keyText(k.Field), d)
		}
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

// defaultName is the name the server gives an index without an explicit one.
func defaultName(keys []KeyField) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s_%v", k.Field, k.Dir)
	}
	return strings.Join(parts, "_")
}

// ---- existing indexes ----------------------------------------------------------

type indexInfo struct {
	name                   string
	keys                   []KeyField
	unique, sparse, hidden bool
	partial, ttl, special  bool // partial filter, TTL, collation or non-B-tree keys
	size, ops              int64
	since                  time.Time
	hasStats               bool
}

func keyFields(d bson.D) []KeyField {
	out := make([]KeyField, 0, len(d))
	for _, e := range d {
		var dir any
		switch v := e.Value.(type) {
		case int32:
			dir = sign(int64(v))
		case int64:
			dir = sign(v)
		case float64:
			dir = sign(int64(v))
		case string:
			dir = v
		default:
			dir = fmt.Sprint(v)
		}
		out = append(out, KeyField{Field: e.Key, Dir: dir})
	}
	return out
}

func sign(n int64) int {
	if n < 0 {
		return -1
	}
	return 1
}

func loadIndexes(ctx context.Context, db *mongo.Database, coll string) ([]indexInfo, error) {
	c := db.Collection(coll)
	cur, err := c.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	var specs []bson.D
	if err := cur.All(ctx, &specs); err != nil {
		return nil, err
	}
	sizes := map[string]int64{}
	var st struct {
		IndexSizes bson.M `bson:"indexSizes"`
	}
	if db.RunCommand(ctx, bson.D{{Key: "collStats", Value: coll}}).Decode(&st) == nil {
		for k, v := range st.IndexSizes {
			sizes[k] = toInt64(v)
		}
	}
	type usage struct {
		ops   int64
		since time.Time
	}
	stats := map[string]usage{}
	if ucur, err := c.Aggregate(ctx, mongo.Pipeline{{{Key: "$indexStats", Value: bson.D{}}}}); err == nil {
		var rows []struct {
			Name     string `bson:"name"`
			Accesses struct {
				Ops   any           `bson:"ops"`
				Since bson.DateTime `bson:"since"`
			} `bson:"accesses"`
		}
		if ucur.All(ctx, &rows) == nil {
			for _, r := range rows {
				u := stats[r.Name] // one row per shard
				u.ops += toInt64(r.Accesses.Ops)
				if t := r.Accesses.Since.Time(); u.since.IsZero() || t.After(u.since) {
					u.since = t
				}
				stats[r.Name] = u
			}
		}
	}
	out := make([]indexInfo, 0, len(specs))
	for _, s := range specs {
		ix := indexInfo{}
		for _, e := range s {
			switch e.Key {
			case "name":
				ix.name, _ = e.Value.(string)
			case "key":
				d, _ := e.Value.(bson.D)
				ix.keys = keyFields(d)
			case "unique":
				ix.unique, _ = e.Value.(bool)
			case "sparse":
				ix.sparse, _ = e.Value.(bool)
			case "hidden":
				ix.hidden, _ = e.Value.(bool)
			case "partialFilterExpression":
				ix.partial = true
			case "expireAfterSeconds":
				ix.ttl = true
			case "collation":
				ix.special = true
			}
		}
		for _, k := range ix.keys {
			if _, ok := k.Dir.(int); !ok {
				ix.special = true
			}
		}
		ix.size = sizes[ix.name]
		if u, ok := stats[ix.name]; ok {
			ix.ops, ix.since, ix.hasStats = u.ops, u.since, true
		}
		out = append(out, ix)
	}
	return out, nil
}

// serves reports how well an index serves a suggested key pattern:
// 2 = fully (equality fields first in any order, then the sort in the same or
// fully reversed direction, then the range fields), 1 = its leading fields
// are equality fields of the query, 0 = not at all.
func serves(ix indexInfo, p predicates, s []KeyField) int {
	if ix.special || ix.partial || len(ix.keys) == 0 {
		return 0
	}
	n := len(p.eq)
	eqSet := map[string]bool{}
	for _, f := range p.eq {
		eqSet[f] = true
	}
	// Leading equality fields.
	i := 0
	for i < len(ix.keys) && i < n && eqSet[ix.keys[i].Field] {
		i++
	}
	if i < n {
		if i > 0 {
			return 1
		}
		return 0
	}
	// Sort, in order, all in the same or all in the reversed direction.
	if len(s) > 0 {
		flip := 0
		for j, k := range s {
			if eqSet[k.Field] {
				continue // sorting on an equality field is free
			}
			if i >= len(ix.keys) || ix.keys[i].Field != k.Field {
				if n > 0 || j > 0 {
					return 1
				}
				return 0
			}
			same := ix.keys[i].Dir == k.Dir
			if flip == 0 {
				flip = map[bool]int{true: 1, false: -1}[same]
			} else if (flip == 1) != same {
				return 1
			}
			i++
		}
	}
	// Every range field somewhere after that.
	rest := map[string]bool{}
	for _, k := range ix.keys[i:] {
		rest[k.Field] = true
	}
	for _, f := range p.rng {
		if !rest[f] {
			if n > 0 || len(s) > 0 {
				return 1
			}
			return 0
		}
	}
	return 2
}

// ---- analysis ---------------------------------------------------------------------

type shapeAcc struct {
	QueryShape
	preds predicates
	sort  []KeyField
	plans map[string]bool
}

// Analyze reads the database profiler and the indexes and returns advice.
func Analyze(ctx context.Context, db *mongo.Database, o AdviceOptions) (Advice, error) {
	adv := Advice{Shapes: []QueryShape{}, Unused: []IndexFinding{}, Redundant: []IndexFinding{}, Notes: []string{}}
	prof, err := GetProfiling(ctx, db)
	if err != nil {
		return adv, err
	}
	adv.ProfilingLevel = prof.Level
	limit := int64(o.Limit)
	if limit <= 0 {
		limit = 5000
	}
	if limit > 50000 {
		limit = 50000
	}
	filter := bson.D{{Key: "op", Value: bson.D{{Key: "$in", Value: bson.A{"query", "getmore", "update", "remove", "command"}}}}}
	if o.Coll != "" {
		filter = append(filter, bson.E{Key: "ns", Value: db.Name() + "." + o.Coll})
	}
	if o.MinMillis > 0 {
		filter = append(filter, bson.E{Key: "millis", Value: bson.D{{Key: "$gte", Value: o.MinMillis}}})
	}
	cur, err := db.Collection("system.profile").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(limit).
		SetProjection(bson.D{{Key: "op", Value: 1}, {Key: "ns", Value: 1}, {Key: "command", Value: 1}, {Key: "originatingCommand", Value: 1},
			{Key: "millis", Value: 1}, {Key: "planSummary", Value: 1}, {Key: "docsExamined", Value: 1}, {Key: "keysExamined", Value: 1},
			{Key: "nreturned", Value: 1}, {Key: "ts", Value: 1}}))
	if err != nil {
		return adv, apperr.From(err)
	}
	defer cur.Close(context.Background())
	shapes := map[string]*shapeAcc{}
	var order []string
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return adv, apperr.From(err)
		}
		adv.Analyzed++
		op, _ := lookupStr(d, "op")
		ns, _ := lookupStr(d, "ns")
		coll := strings.TrimPrefix(ns, db.Name()+".")
		if coll == ns || strings.HasPrefix(coll, "system.") || coll == "$cmd" {
			continue
		}
		cmd := docAt(d, "command")
		if op == "getmore" {
			cmd = docAt(d, "originatingCommand")
		}
		f, s, ok := extract(op, cmd)
		if !ok {
			continue
		}
		var p predicates
		classify(f, &p)
		sf := sortFields(s)
		key := coll + "|" + op + "|" + shapeText(p, sf) + "|" + p.blocked
		if op == "getmore" {
			key = coll + "|query|" + shapeText(p, sf) + "|" + p.blocked
		}
		acc := shapes[key]
		if acc == nil {
			displayOp := op
			if op == "getmore" {
				displayOp = "query"
			}
			acc = &shapeAcc{QueryShape: QueryShape{Coll: coll, Op: displayOp, Shape: shapeText(p, sf), Example: truncate(mql.Compact(cmd), 600)}, preds: p, sort: sf, plans: map[string]bool{}}
			shapes[key] = acc
			order = append(order, key)
		}
		ms := toInt64(valOf(d, "millis"))
		acc.Count++
		acc.TotalMillis += ms
		if ms > acc.MaxMillis {
			acc.MaxMillis = ms
		}
		acc.DocsExamined += toInt64(valOf(d, "docsExamined"))
		acc.KeysExamined += toInt64(valOf(d, "keysExamined"))
		acc.Returned += toInt64(valOf(d, "nreturned"))
		if plan, ok := lookupStr(d, "planSummary"); ok && plan != "" {
			acc.plans[plan] = true
		}
		if ts, ok := valOf(d, "ts").(bson.DateTime); ok {
			t := ts.Time().UTC()
			if t.After(acc.LastSeen) {
				acc.LastSeen = t
			}
			if adv.Since == nil || t.Before(*adv.Since) {
				adv.Since = &t
			}
		}
	}
	if err := cur.Err(); err != nil {
		return adv, apperr.From(err)
	}

	// Collections to inspect: the one asked for, those seen in the profile,
	// and (for the whole database) every collection, for unused indexes.
	colls := map[string]bool{}
	if o.Coll != "" {
		colls[o.Coll] = true
	} else {
		names, err := db.ListCollectionNames(ctx, bson.D{{Key: "type", Value: "collection"}})
		if err != nil {
			return adv, apperr.From(err)
		}
		if len(names) > 300 {
			adv.Notes = append(adv.Notes, fmt.Sprintf("The database has %d collections; only the first 300 were checked for unused and redundant indexes.", len(names)))
			sort.Strings(names)
			names = names[:300]
		}
		for _, n := range names {
			if !strings.HasPrefix(n, "system.") {
				colls[n] = true
			}
		}
	}
	for _, k := range order {
		colls[shapes[k].Coll] = true
	}
	indexes := map[string][]indexInfo{}
	for c := range colls {
		ix, err := loadIndexes(ctx, db, c)
		if err != nil {
			adv.Notes = append(adv.Notes, fmt.Sprintf("Indexes of %s could not be read: %s", c, apperr.From(err).Message))
			continue
		}
		indexes[c] = ix
	}

	for _, k := range order {
		a := shapes[k]
		for p := range a.plans {
			a.Plans = append(a.Plans, p)
		}
		sort.Strings(a.Plans)
		if a.Count > 0 {
			a.AvgMillis = a.TotalMillis / int64(a.Count)
		}
		decide(a, indexes[a.Coll])
		adv.Shapes = append(adv.Shapes, a.QueryShape)
	}
	sort.SliceStable(adv.Shapes, func(i, j int) bool { return adv.Shapes[i].TotalMillis > adv.Shapes[j].TotalMillis })

	names := make([]string, 0, len(indexes))
	for c := range indexes {
		names = append(names, c)
	}
	sort.Strings(names)
	for _, c := range names {
		ixs := indexes[c]
		for _, ix := range ixs {
			if ix.name == "_id_" {
				continue
			}
			if ix.hasStats && ix.ops == 0 && !ix.ttl {
				reason := "Not used since " + ix.since.UTC().Format("2006-01-02 15:04") + " UTC (usage counts restart with the server)."
				if ix.unique {
					reason += " It enforces uniqueness, so it may still be needed."
				}
				adv.Unused = append(adv.Unused, IndexFinding{Coll: c, Name: ix.name, Keys: KeysText(ix.keys), Reason: reason, Size: ix.size, Hidden: ix.hidden})
			}
			if by := redundantWith(ix, ixs); by != "" {
				adv.Redundant = append(adv.Redundant, IndexFinding{Coll: c, Name: ix.name, Keys: KeysText(ix.keys), Reason: "Its keys are the start of " + by + ", which serves the same queries.", Size: ix.size, Hidden: ix.hidden})
			}
		}
	}
	switch {
	case adv.ProfilingLevel == 0 && adv.Analyzed == 0:
		adv.Notes = append(adv.Notes, "The profiler is off, so there are no queries to analyze. Turn it on (slow operations) and let the application run for a while.")
	case adv.Analyzed == 0:
		adv.Notes = append(adv.Notes, "No profiled queries yet. Let the application run for a while, or lower the slow-operation threshold.")
	case int64(adv.Analyzed) >= limit:
		adv.Notes = append(adv.Notes, fmt.Sprintf("Only the newest %d profiled operations were analyzed.", limit))
	}
	return adv, nil
}

func decide(a *shapeAcc, ixs []indexInfo) {
	if a.preds.blocked != "" {
		a.Status, a.Reason = "none", "This query "+a.preds.blocked+"."
		return
	}
	sug := suggest(a.preds, a.sort)
	if len(sug) == 0 {
		a.Status, a.Reason = "none", "The query has no filter or sort an index could serve."
		return
	}
	if len(sug) == 1 && sug[0].Field == "_id" {
		a.Status, a.Reason = "covered", "Served by the _id index."
		a.ServedBy = "_id_"
		return
	}
	a.suggestion = sug
	a.Suggested = KeysText(sug)
	a.Name = defaultName(sug)
	best, bestScore := "", 0
	hidden := ""
	for _, ix := range ixs {
		sc := serves(ix, a.preds, a.sort)
		if sc == 2 && ix.hidden {
			hidden = ix.name
			continue
		}
		if sc > bestScore {
			best, bestScore = ix.name, sc
		}
	}
	collscan := false
	for _, p := range a.Plans {
		if strings.Contains(p, "COLLSCAN") {
			collscan = true
		}
	}
	switch {
	case bestScore == 2:
		a.Status, a.ServedBy = "covered", best
		a.Reason = "Index " + best + " serves this query."
		if collscan {
			a.Reason += " Earlier runs scanned the collection; it may have been created since."
		}
	case hidden != "":
		a.Status, a.ServedBy = "hidden", hidden
		a.Reason = "Hidden index " + hidden + " would serve this query. Unhide it instead of creating a new one."
	case bestScore == 1:
		a.Status, a.ServedBy = "partial", best
		a.Reason = "Index " + best + " serves only the first fields; the rest are filtered or sorted after reading. Equality fields first, then the sort, then ranges."
	default:
		a.Status = "missing"
		if collscan {
			a.Reason = "No index serves this query: it scans the whole collection."
		} else {
			a.Reason = "No index serves this query."
		}
	}
	if a.Returned > 0 && a.DocsExamined > 10*a.Returned && a.Status != "covered" {
		a.Reason += fmt.Sprintf(" It examined %d documents to return %d.", a.DocsExamined, a.Returned)
	}
}

// redundantWith returns the index that makes ix redundant: one whose keys
// start with ix's keys in the same directions, and that has no restriction
// ix lacks. Unique, TTL and special indexes are never reported.
func redundantWith(ix indexInfo, all []indexInfo) string {
	if ix.unique || ix.ttl || ix.special || ix.name == "_id_" {
		return ""
	}
	for _, o := range all {
		if o.name == ix.name || o.special || o.partial || o.sparse || o.hidden || len(o.keys) <= len(ix.keys) {
			continue
		}
		if ix.partial || ix.sparse {
			continue
		}
		match := true
		for i, k := range ix.keys {
			if o.keys[i].Field != k.Field || o.keys[i].Dir != k.Dir {
				match = false
				break
			}
		}
		if match {
			return o.name
		}
	}
	return ""
}

func lookupStr(d bson.D, key string) (string, bool) {
	v, ok := lookupD(d, key)
	s, _ := v.(string)
	return s, ok
}

func valOf(d bson.D, key string) any {
	v, _ := lookupD(d, key)
	return v
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
