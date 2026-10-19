package shell

import (
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// QuerySpec is a find or aggregate statement from the collection tab's
// query editor, e.g. db.getCollection("orders").find({ status: "A" }).sort({ _id: -1 }).
// It is run through the paged query and aggregation services, never as a
// one-shot shell command, so results always come back page by page.
type QuerySpec struct {
	Kind       string            `json:"kind"` // find | aggregate
	DB         string            `json:"db"`
	Coll       string            `json:"coll"`
	Filter     string            `json:"filter"`
	Projection string            `json:"projection"`
	Sort       string            `json:"sort"`
	Skip       int64             `json:"skip"`
	Limit      int64             `json:"limit"`
	Stages     []aggregate.Stage `json:"stages"`
}

// ParseQuery reads one db.<coll>.find(…) / db.getCollection(…).find(…) or
// .aggregate([…]) statement. db is the database the statement runs in unless
// it uses db.getSiblingDB(…).
func ParseQuery(script, db string) (QuerySpec, error) {
	stmts, err := Parse(script)
	if err != nil {
		return QuerySpec{}, apperr.Wrap(apperr.CodeInvalidInput, err, err.Error())
	}
	if len(stmts) == 0 {
		return QuerySpec{}, apperr.Invalid(`Enter a query, e.g. db.getCollection("orders").find({}).`)
	}
	if len(stmts) > 1 {
		return QuerySpec{}, apperr.Invalid("Enter one query at a time here; use the Shell tab to run scripts.")
	}
	st := stmts[0]
	if st.kind != "db" {
		return QuerySpec{}, apperr.Invalid("Only find() and aggregate() queries run here; use the Shell tab for %q.", st.text)
	}
	segs := st.segs
	for len(segs) > 0 && segs[0].call && segs[0].name == "getSiblingDB" {
		name, err := argString(segs[0], 0, "database name")
		if err != nil {
			return QuerySpec{}, err
		}
		db, segs = name, segs[1:]
	}
	var coll string
	switch {
	case len(segs) > 0 && segs[0].call && segs[0].name == "getCollection":
		if coll, err = argString(segs[0], 0, "collection name"); err != nil {
			return QuerySpec{}, err
		}
		segs = segs[1:]
	default:
		k := firstCall(segs)
		if k <= 0 {
			return QuerySpec{}, apperr.Invalid(`Start the query with db.getCollection("name") or db.name, then .find(…) or .aggregate([…]).`)
		}
		names := make([]string, k)
		for i := range names {
			names[i] = segs[i].name
		}
		coll, segs = strings.Join(names, "."), segs[k:]
	}
	if len(segs) == 0 || !segs[0].call {
		return QuerySpec{}, apperr.Invalid("Add .find(…) or .aggregate([…]) after the collection.")
	}
	m, chain := segs[0], segs[1:]
	spec := QuerySpec{DB: db, Coll: coll}
	switch m.name {
	case "find":
		spec.Kind = "find"
		filter, err := docArg(m, 0, "filter")
		if err != nil {
			return QuerySpec{}, err
		}
		spec.Filter = mql.Pretty(filter)
		if p, ok := argDoc(m, 1); ok && len(p) > 0 {
			spec.Projection = mql.Pretty(p)
		}
		for _, s := range chain {
			switch s.name {
			case "sort":
				d, err := requiredDoc(s, 0, "sort document")
				if err != nil {
					return QuerySpec{}, err
				}
				spec.Sort = mql.Pretty(d)
			case "projection", "project":
				d, err := requiredDoc(s, 0, "projection")
				if err != nil {
					return QuerySpec{}, err
				}
				spec.Projection = mql.Pretty(d)
			case "skip", "limit":
				n, ok := intArg(s, 0)
				if !ok || n < 0 {
					return QuerySpec{}, apperr.Invalid("%s() expects a non-negative number.", s.name)
				}
				if s.name == "skip" {
					spec.Skip = n
				} else {
					spec.Limit = n
				}
			case "pretty", "toArray", "batchSize", "maxTimeMS", "allowDiskUse", "noCursorTimeout", "comment", "hint":
				// Accepted for copy-paste compatibility; results are always paged.
			case "count", "size", "itcount", "explain":
				return QuerySpec{}, apperr.Invalid(".%s() isn't a query to page through: use Count or Explain in this tab, or the Shell tab.", s.name)
			default:
				return QuerySpec{}, apperr.New(apperr.CodeUnsupported, "Cursor method .%s() is not supported.", s.name)
			}
		}
	case "aggregate":
		spec.Kind = "aggregate"
		var pipe bson.A
		if len(m.args) > 0 {
			a, ok := m.args[0].(bson.A)
			if !ok {
				return QuerySpec{}, apperr.Invalid("aggregate() expects an array of stages.")
			}
			pipe = a
		}
		for i, st := range pipe {
			d, ok := st.(bson.D)
			if !ok || len(d) != 1 || !strings.HasPrefix(d[0].Key, "$") {
				return QuerySpec{}, apperr.Invalid("Pipeline stage %d must be a document with one $operator.", i+1)
			}
			spec.Stages = append(spec.Stages, aggregate.Stage{Operator: d[0].Key, Body: mql.Pretty(d[0].Value), Enabled: true})
		}
		if len(chain) > 0 {
			return QuerySpec{}, apperr.Invalid("Add stages such as $sort and $limit inside the pipeline instead of .%s().", chain[0].name)
		}
	default:
		return QuerySpec{}, apperr.Invalid("Only find() and aggregate() run in the collection tab; use the Shell tab for %s().", m.name)
	}
	return spec, nil
}
