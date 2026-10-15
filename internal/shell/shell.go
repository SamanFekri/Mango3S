package shell

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// Output is the result of one statement.
type Output struct {
	Statement  string           `json:"statement"`
	Kind       string           `json:"kind"` // documents | text | error
	Text       string           `json:"text,omitempty"`
	Documents  []model.Document `json:"documents,omitempty"`
	HasMore    bool             `json:"hasMore"`
	DurationMs int64            `json:"durationMs"`
}

// Result is the result of a script.
type Result struct {
	Outputs   []Output `json:"outputs"`
	CurrentDB string   `json:"currentDb"`
}

// Service executes shell scripts.
type Service interface {
	Execute(ctx context.Context, connID, db, script string, maxDocs int) (Result, error)
}

type service struct{ src mongox.Source }

// New returns a shell service backed by src.
func New(src mongox.Source) Service { return &service{src: src} }

// DefaultDB is used when no database is selected.
const DefaultDB = "test"

func (s *service) Execute(ctx context.Context, connID, db, script string, maxDocs int) (Result, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return Result{}, err
	}
	if db == "" {
		db = DefaultDB
	}
	if maxDocs <= 0 {
		maxDocs = 20
	}
	res := Result{Outputs: []Output{}, CurrentDB: db}
	stmts, err := Parse(script)
	if err != nil {
		res.Outputs = append(res.Outputs, Output{Statement: firstLine(script), Kind: "error", Text: err.Error()})
		return res, nil
	}
	if len(stmts) == 0 {
		return res, nil
	}
	ex := &executor{sess: sess, db: db, maxDocs: maxDocs}
	for _, st := range stmts {
		start := time.Now()
		out, err := ex.run(ctx, st)
		out.Statement = st.text
		out.DurationMs = time.Since(start).Milliseconds()
		if err != nil {
			ae := apperr.From(err)
			out.Kind, out.Text, out.Documents = "error", ae.Message, nil
			if ae.Detail != "" && ae.Detail != ae.Message {
				out.Text += "\n" + ae.Detail
			}
			res.Outputs = append(res.Outputs, out)
			break // stop at the first failing statement, like mongosh
		}
		res.Outputs = append(res.Outputs, out)
		if ctx.Err() != nil {
			break
		}
	}
	res.CurrentDB = ex.db
	return res, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " ..."
	}
	return s
}

type executor struct {
	sess    *mongox.Session
	db      string
	maxDocs int
}

func text(s string) Output { return Output{Kind: "text", Text: s} }

func value(v any) Output { return Output{Kind: "text", Text: mql.Pretty(v)} }

func (e *executor) run(ctx context.Context, st statement) (Output, error) {
	switch st.kind {
	case "help":
		return text(helpText), nil
	case "use":
		e.db = st.word
		return text("switched to db " + st.word), nil
	case "show":
		return e.show(ctx, st.word)
	}
	return e.evalDB(ctx, e.db, st.segs)
}

func (e *executor) show(ctx context.Context, what string) (Output, error) {
	c := e.sess.Client
	switch what {
	case "dbs", "databases":
		res, err := c.ListDatabases(ctx, bson.D{}, options.ListDatabases().SetAuthorizedDatabases(true))
		if err != nil {
			return Output{}, err
		}
		var b strings.Builder
		tw := tabwriter.NewWriter(&b, 0, 4, 3, ' ', 0)
		for _, d := range res.Databases {
			fmt.Fprintf(tw, "%s\t%s\n", d.Name, humanBytes(d.SizeOnDisk))
		}
		tw.Flush()
		return text(strings.TrimRight(b.String(), "\n")), nil
	case "collections", "tables":
		names, err := c.Database(e.db).ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return Output{}, err
		}
		sort.Strings(names)
		return text(strings.Join(names, "\n")), nil
	}
	return Output{}, apperr.Invalid("show %s is not supported (use show dbs or show collections).", what)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func firstCall(segs []segment) int {
	for i, s := range segs {
		if s.call {
			return i
		}
	}
	return -1
}

func (e *executor) evalDB(ctx context.Context, db string, segs []segment) (Output, error) {
	if len(segs) == 0 {
		return text(db), nil
	}
	k := firstCall(segs)
	if k < 0 {
		return Output{}, apperr.Invalid("%s is incomplete: call a method, e.g. %s.find().", describe(segs), describe(segs))
	}
	if k > 0 {
		names := make([]string, k)
		for i := range names {
			names[i] = segs[i].name
		}
		return e.evalColl(ctx, db, strings.Join(names, "."), segs[k:])
	}
	m := segs[0]
	switch m.name {
	case "getCollection":
		name, err := argString(m, 0, "collection name")
		if err != nil {
			return Output{}, err
		}
		if len(segs) < 2 || !segs[1].call {
			return text(db + "." + name), nil
		}
		return e.evalColl(ctx, db, name, segs[1:])
	case "getSiblingDB":
		name, err := argString(m, 0, "database name")
		if err != nil {
			return Output{}, err
		}
		return e.evalDB(ctx, name, segs[1:])
	}
	if len(segs) > 1 {
		return Output{}, apperr.Invalid("Chaining after db.%s() is not supported.", m.name)
	}
	return e.dbMethod(ctx, db, m)
}

// readOnlyCommands may run via runCommand on read-only connections.
var readOnlyCommands = map[string]bool{
	"ping": true, "buildinfo": true, "serverstatus": true, "dbstats": true, "collstats": true,
	"listcollections": true, "listdatabases": true, "listindexes": true, "hello": true, "ismaster": true,
	"count": true, "distinct": true, "find": true, "explain": true, "connectionstatus": true,
	"hostinfo": true, "getcmdlineopts": true, "replsetgetstatus": true, "currentop": true,
	"top": true, "validate": true, "datasize": true, "getlog": true,
}

func (e *executor) dbMethod(ctx context.Context, db string, m segment) (Output, error) {
	d := e.sess.Client.Database(db)
	switch m.name {
	case "getName":
		return text(db), nil
	case "getCollectionNames":
		names, err := d.ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return Output{}, err
		}
		sort.Strings(names)
		a := bson.A{}
		for _, n := range names {
			a = append(a, n)
		}
		return value(a), nil
	case "stats":
		return e.command(ctx, d, bson.D{{Key: "dbStats", Value: 1}})
	case "serverStatus":
		return e.command(ctx, e.sess.Client.Database("admin"), bson.D{{Key: "serverStatus", Value: 1}})
	case "version":
		var bi struct {
			Version string `bson:"version"`
		}
		if err := e.sess.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&bi); err != nil {
			return Output{}, err
		}
		return text(bi.Version), nil
	case "runCommand", "adminCommand":
		cmd, err := commandArg(m)
		if err != nil {
			return Output{}, err
		}
		if e.sess.ReadOnly && (len(cmd) == 0 || !readOnlyCommands[strings.ToLower(cmd[0].Key)]) {
			return Output{}, e.sess.RequireWritable()
		}
		target := d
		if m.name == "adminCommand" {
			target = e.sess.Client.Database("admin")
		}
		return e.command(ctx, target, cmd)
	case "createCollection":
		if err := e.sess.RequireWritable(); err != nil {
			return Output{}, err
		}
		name, err := argString(m, 0, "collection name")
		if err != nil {
			return Output{}, err
		}
		cmd := bson.D{{Key: "create", Value: name}}
		if opts, ok := argDoc(m, 1); ok {
			cmd = append(cmd, opts...)
		}
		return e.command(ctx, d, cmd)
	case "dropDatabase":
		if err := e.sess.RequireWritable(); err != nil {
			return Output{}, err
		}
		if err := d.Drop(ctx); err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "ok", Value: 1}, {Key: "dropped", Value: db}}), nil
	}
	return Output{}, apperr.New(apperr.CodeUnsupported, "db.%s() is not supported (try help).", m.name)
}

func commandArg(m segment) (bson.D, error) {
	if len(m.args) == 0 {
		return nil, apperr.Invalid("%s expects a command document.", m.name)
	}
	switch v := m.args[0].(type) {
	case string:
		return bson.D{{Key: v, Value: 1}}, nil
	case bson.D:
		if len(v) == 0 {
			return nil, apperr.Invalid("%s: the command document is empty.", m.name)
		}
		return v, nil
	}
	return nil, apperr.Invalid("%s expects a command document.", m.name)
}

func (e *executor) command(ctx context.Context, d *mongo.Database, cmd bson.D) (Output, error) {
	var out bson.D
	if err := d.RunCommand(ctx, cmd).Decode(&out); err != nil {
		return Output{}, err
	}
	return value(out), nil
}

// writeMethods are rejected on read-only connections.
var writeMethods = map[string]bool{
	"insertOne": true, "insertMany": true, "updateOne": true, "updateMany": true,
	"replaceOne": true, "deleteOne": true, "deleteMany": true, "drop": true,
	"createIndex": true, "dropIndex": true, "findOneAndUpdate": true,
	"findOneAndReplace": true, "findOneAndDelete": true,
}

func (e *executor) evalColl(ctx context.Context, db, name string, segs []segment) (Output, error) {
	c := e.sess.Client.Database(db).Collection(name)
	m := segs[0]
	chain := segs[1:]
	for _, s := range chain {
		if !s.call {
			return Output{}, apperr.Invalid("Property access .%s is not supported after %s().", s.name, m.name)
		}
	}
	if writeMethods[m.name] {
		if err := e.sess.RequireWritable(); err != nil {
			return Output{}, err
		}
	}
	if m.name != "find" && m.name != "aggregate" && len(chain) > 0 {
		if !(len(chain) == 1 && (chain[0].name == "pretty" || chain[0].name == "toArray")) {
			return Output{}, apperr.Invalid("%s() does not support .%s().", m.name, chain[0].name)
		}
	}

	switch m.name {
	case "find":
		return e.find(ctx, c, m, chain)
	case "findOne":
		filter, err := docArg(m, 0, "filter")
		if err != nil {
			return Output{}, err
		}
		opts := options.FindOne()
		if proj, ok := argDoc(m, 1); ok {
			opts.SetProjection(proj)
		}
		var d bson.D
		err = c.FindOne(ctx, filter, opts).Decode(&d)
		if err == mongo.ErrNoDocuments {
			return text("null"), nil
		}
		if err != nil {
			return Output{}, err
		}
		return Output{Kind: "documents", Documents: []model.Document{model.NewDocument(d)}}, nil
	case "countDocuments", "count":
		filter, err := docArg(m, 0, "filter")
		if err != nil {
			return Output{}, err
		}
		n, err := c.CountDocuments(ctx, filter)
		if err != nil {
			return Output{}, err
		}
		return text(fmt.Sprint(n)), nil
	case "estimatedDocumentCount":
		n, err := c.EstimatedDocumentCount(ctx)
		if err != nil {
			return Output{}, err
		}
		return text(fmt.Sprint(n)), nil
	case "distinct":
		field, err := argString(m, 0, "field name")
		if err != nil {
			return Output{}, err
		}
		filter, err := docArg(m, 1, "filter")
		if err != nil {
			return Output{}, err
		}
		res := c.Distinct(ctx, field, filter)
		if err := res.Err(); err != nil {
			return Output{}, err
		}
		var vals bson.A
		if err := res.Decode(&vals); err != nil {
			return Output{}, err
		}
		return value(vals), nil
	case "aggregate":
		return e.aggregate(ctx, c, m)
	case "insertOne":
		doc, err := requiredDoc(m, 0, "document")
		if err != nil {
			return Output{}, err
		}
		r, err := c.InsertOne(ctx, doc)
		if err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "acknowledged", Value: true}, {Key: "insertedId", Value: r.InsertedID}}), nil
	case "insertMany":
		arr, ok := argArray(m, 0)
		if !ok || len(arr) == 0 {
			return Output{}, apperr.Invalid("insertMany expects a non-empty array of documents.")
		}
		docs := make([]any, len(arr))
		for i, d := range arr {
			if _, ok := d.(bson.D); !ok {
				return Output{}, apperr.Invalid("insertMany: element %d is not a document.", i)
			}
			docs[i] = d
		}
		r, err := c.InsertMany(ctx, docs)
		if err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "acknowledged", Value: true}, {Key: "insertedIds", Value: bson.A(r.InsertedIDs)}}), nil
	case "updateOne", "updateMany":
		return e.update(ctx, c, m)
	case "replaceOne":
		filter, err := requiredDoc(m, 0, "filter")
		if err != nil {
			return Output{}, err
		}
		repl, err := requiredDoc(m, 1, "replacement document")
		if err != nil {
			return Output{}, err
		}
		opts := options.Replace()
		if o, ok := argDoc(m, 2); ok {
			if up, ok := boolOpt(o, "upsert"); ok {
				opts.SetUpsert(up)
			}
		}
		r, err := c.ReplaceOne(ctx, filter, repl, opts)
		if err != nil {
			return Output{}, err
		}
		return value(updateResult(r)), nil
	case "deleteOne", "deleteMany":
		filter, err := requiredDoc(m, 0, "filter")
		if err != nil {
			return Output{}, err
		}
		var r *mongo.DeleteResult
		if m.name == "deleteOne" {
			r, err = c.DeleteOne(ctx, filter)
		} else {
			r, err = c.DeleteMany(ctx, filter)
		}
		if err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "acknowledged", Value: true}, {Key: "deletedCount", Value: r.DeletedCount}}), nil
	case "drop":
		if err := c.Drop(ctx); err != nil {
			return Output{}, err
		}
		return text("true"), nil
	case "getIndexes":
		cur, err := c.Indexes().List(ctx)
		if err != nil {
			return Output{}, err
		}
		defer cur.Close(context.Background())
		var specs []bson.D
		if err := cur.All(ctx, &specs); err != nil {
			return Output{}, err
		}
		a := bson.A{}
		for _, s := range specs {
			a = append(a, s)
		}
		return value(a), nil
	case "createIndex":
		keys, err := requiredDoc(m, 0, "index keys")
		if err != nil {
			return Output{}, err
		}
		opts := options.Index()
		if o, ok := argDoc(m, 1); ok {
			for _, e := range o {
				switch e.Key {
				case "name":
					if s, ok := e.Value.(string); ok {
						opts.SetName(s)
					}
				case "unique":
					if b, ok := e.Value.(bool); ok {
						opts.SetUnique(b)
					}
				case "sparse":
					if b, ok := e.Value.(bool); ok {
						opts.SetSparse(b)
					}
				case "expireAfterSeconds":
					if n, ok := toInt(e.Value); ok {
						opts.SetExpireAfterSeconds(int32(n))
					}
				default:
					return Output{}, apperr.Invalid("createIndex option %q is not supported.", e.Key)
				}
			}
		}
		name, err := c.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys, Options: opts})
		if err != nil {
			return Output{}, err
		}
		return text(mql.Quote(name)), nil
	case "dropIndex":
		if len(m.args) != 1 {
			return Output{}, apperr.Invalid("dropIndex expects an index name or key document.")
		}
		var err error
		switch v := m.args[0].(type) {
		case string:
			err = c.Indexes().DropOne(ctx, v)
		case bson.D:
			err = c.Indexes().DropWithKey(ctx, v)
		default:
			return Output{}, apperr.Invalid("dropIndex expects an index name or key document.")
		}
		if err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "ok", Value: 1}}), nil
	}
	return Output{}, apperr.New(apperr.CodeUnsupported, "%s() is not supported (try help).", m.name)
}

func (e *executor) find(ctx context.Context, c *mongo.Collection, m segment, chain []segment) (Output, error) {
	filter, err := docArg(m, 0, "filter")
	if err != nil {
		return Output{}, err
	}
	var proj, sortDoc bson.D
	if p, ok := argDoc(m, 1); ok {
		proj = p
	}
	var skip, limit int64
	countOnly, explain := false, false
	for _, s := range chain {
		switch s.name {
		case "sort":
			if sortDoc, err = requiredDoc(s, 0, "sort document"); err != nil {
				return Output{}, err
			}
		case "projection", "project":
			if proj, err = requiredDoc(s, 0, "projection"); err != nil {
				return Output{}, err
			}
		case "skip", "limit":
			n, ok := intArg(s, 0)
			if !ok || n < 0 {
				return Output{}, apperr.Invalid("%s() expects a non-negative number.", s.name)
			}
			if s.name == "skip" {
				skip = n
			} else {
				limit = n
			}
		case "count", "size", "itcount":
			countOnly = true
		case "explain":
			explain = true
		case "pretty", "toArray", "batchSize", "maxTimeMS", "allowDiskUse", "noCursorTimeout", "comment", "hint":
			// No effect in this shell (hint and comment are accepted for copy-paste compatibility).
		default:
			return Output{}, apperr.New(apperr.CodeUnsupported, "Cursor method .%s() is not supported.", s.name)
		}
	}

	if countOnly {
		opts := options.Count()
		if skip > 0 {
			opts.SetSkip(skip)
		}
		if limit > 0 {
			opts.SetLimit(limit)
		}
		n, err := c.CountDocuments(ctx, filter, opts)
		if err != nil {
			return Output{}, err
		}
		return text(fmt.Sprint(n)), nil
	}

	if explain {
		findCmd := bson.D{{Key: "find", Value: c.Name()}, {Key: "filter", Value: filter}}
		if proj != nil {
			findCmd = append(findCmd, bson.E{Key: "projection", Value: proj})
		}
		if sortDoc != nil {
			findCmd = append(findCmd, bson.E{Key: "sort", Value: sortDoc})
		}
		if skip > 0 {
			findCmd = append(findCmd, bson.E{Key: "skip", Value: skip})
		}
		if limit > 0 {
			findCmd = append(findCmd, bson.E{Key: "limit", Value: limit})
		}
		return e.command(ctx, c.Database(), bson.D{{Key: "explain", Value: findCmd}, {Key: "verbosity", Value: "queryPlanner"}})
	}

	maxDocs := int64(e.maxDocs)
	fetch := maxDocs + 1
	if limit > 0 && limit <= maxDocs {
		fetch = limit
	}
	opts := options.Find().SetLimit(fetch).SetBatchSize(int32(fetch))
	if skip > 0 {
		opts.SetSkip(skip)
	}
	if proj != nil {
		opts.SetProjection(proj)
	}
	if sortDoc != nil {
		opts.SetSort(sortDoc)
	}
	cur, err := c.Find(ctx, filter, opts)
	if err != nil {
		return Output{}, err
	}
	defer cur.Close(context.Background())
	return e.drain(ctx, cur)
}

func (e *executor) drain(ctx context.Context, cur *mongo.Cursor) (Output, error) {
	out := Output{Kind: "documents", Documents: []model.Document{}}
	for cur.Next(ctx) {
		if len(out.Documents) >= e.maxDocs {
			out.HasMore = true
			break
		}
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return Output{}, err
		}
		out.Documents = append(out.Documents, model.NewDocument(d))
	}
	if err := cur.Err(); err != nil {
		return Output{}, err
	}
	if out.HasMore {
		out.Text = fmt.Sprintf("Showing the first %d documents. Use .skip(n) and .limit(n) to see more.", e.maxDocs)
	}
	return out, nil
}

func (e *executor) aggregate(ctx context.Context, c *mongo.Collection, m segment) (Output, error) {
	var pipe bson.A
	switch {
	case len(m.args) == 0:
		pipe = bson.A{}
	default:
		if a, ok := m.args[0].(bson.A); ok {
			pipe = a
		} else {
			// Legacy form: aggregate(stage1, stage2, ...)
			pipe = bson.A(m.args)
		}
	}
	isWrite := false
	for i, st := range pipe {
		d, ok := st.(bson.D)
		if !ok || len(d) != 1 {
			return Output{}, apperr.Invalid("Pipeline stage %d must be a document with one $operator.", i+1)
		}
		if d[0].Key == "$out" || d[0].Key == "$merge" {
			isWrite = true
		}
	}
	if isWrite {
		if err := e.sess.RequireWritable(); err != nil {
			return Output{}, err
		}
	}
	opts := options.Aggregate().SetBatchSize(int32(e.maxDocs + 1))
	if len(m.args) == 2 {
		if o, ok := m.args[1].(bson.D); ok {
			if b, ok := boolOpt(o, "allowDiskUse"); ok {
				opts.SetAllowDiskUse(b)
			}
		}
	}
	cur, err := c.Aggregate(ctx, pipe, opts)
	if err != nil {
		return Output{}, err
	}
	defer cur.Close(context.Background())
	return e.drain(ctx, cur)
}

func (e *executor) update(ctx context.Context, c *mongo.Collection, m segment) (Output, error) {
	filter, err := requiredDoc(m, 0, "filter")
	if err != nil {
		return Output{}, err
	}
	if len(m.args) < 2 {
		return Output{}, apperr.Invalid("%s expects (filter, update).", m.name)
	}
	upd := m.args[1]
	switch u := upd.(type) {
	case bson.D:
		if len(u) == 0 || !strings.HasPrefix(u[0].Key, "$") {
			return Output{}, apperr.Invalid("%s requires update operators such as $set (use replaceOne to replace a document).", m.name)
		}
	case bson.A:
		// aggregation pipeline update
	default:
		return Output{}, apperr.Invalid("%s: the update must be a document or pipeline.", m.name)
	}
	var upsert *bool
	if o, ok := argDoc(m, 2); ok {
		if b, ok := boolOpt(o, "upsert"); ok {
			upsert = &b
		}
	}
	var r *mongo.UpdateResult
	if m.name == "updateOne" {
		opts := options.UpdateOne()
		if upsert != nil {
			opts.SetUpsert(*upsert)
		}
		r, err = c.UpdateOne(ctx, filter, upd, opts)
	} else {
		opts := options.UpdateMany()
		if upsert != nil {
			opts.SetUpsert(*upsert)
		}
		r, err = c.UpdateMany(ctx, filter, upd, opts)
	}
	if err != nil {
		return Output{}, err
	}
	return value(updateResult(r)), nil
}

func updateResult(r *mongo.UpdateResult) bson.D {
	d := bson.D{
		{Key: "acknowledged", Value: true},
		{Key: "matchedCount", Value: r.MatchedCount},
		{Key: "modifiedCount", Value: r.ModifiedCount},
		{Key: "upsertedCount", Value: r.UpsertedCount},
	}
	if r.UpsertedID != nil {
		d = append(d, bson.E{Key: "upsertedId", Value: r.UpsertedID})
	}
	return d
}

func argString(m segment, i int, what string) (string, error) {
	if i < len(m.args) {
		if s, ok := m.args[i].(string); ok && s != "" {
			return s, nil
		}
	}
	return "", apperr.Invalid("%s() expects a %s string as argument %d.", m.name, what, i+1)
}

func argDoc(m segment, i int) (bson.D, bool) {
	if i < len(m.args) {
		d, ok := m.args[i].(bson.D)
		return d, ok
	}
	return nil, false
}

func argArray(m segment, i int) (bson.A, bool) {
	if i < len(m.args) {
		a, ok := m.args[i].(bson.A)
		return a, ok
	}
	return nil, false
}

// docArg returns an optional document argument (empty when omitted).
func docArg(m segment, i int, what string) (bson.D, error) {
	if i >= len(m.args) || m.args[i] == nil {
		return bson.D{}, nil
	}
	d, ok := m.args[i].(bson.D)
	if !ok {
		return nil, apperr.Invalid("%s(): the %s must be a document.", m.name, what)
	}
	return d, nil
}

// requiredDoc returns a mandatory document argument.
func requiredDoc(m segment, i int, what string) (bson.D, error) {
	if i >= len(m.args) {
		return nil, apperr.Invalid("%s() requires a %s.", m.name, what)
	}
	return docArg(m, i, what)
}

func intArg(m segment, i int) (int64, bool) {
	if i < len(m.args) {
		return toInt(m.args[i])
	}
	return 0, false
}

func toInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if n == float64(int64(n)) {
			return int64(n), true
		}
	}
	return 0, false
}

func boolOpt(d bson.D, key string) (bool, bool) {
	if v, ok := mql.Lookup(d, key); ok {
		b, ok := v.(bool)
		return b, ok
	}
	return false, false
}

const helpText = `Supported commands (this is a MongoDB command shell, not a JavaScript engine):

  show dbs | show collections        use <database>        db
  db.getCollectionNames()            db.getName()          db.stats()
  db.version()                       db.serverStatus()     db.dropDatabase()
  db.createCollection(name, opts)    db.runCommand(cmd)    db.adminCommand(cmd)
  db.getSiblingDB(name).<...>        db.getCollection(name).<method>(...)

Collection methods: db.<coll>.<method>(...)
  find(filter, projection) .sort() .skip() .limit() .projection() .count() .explain()
  findOne  countDocuments  estimatedDocumentCount  distinct  aggregate(pipeline, opts)
  insertOne  insertMany  updateOne  updateMany  replaceOne  deleteOne  deleteMany
  createIndex(keys, {name, unique, sparse, expireAfterSeconds})  getIndexes  dropIndex  drop

Values: ObjectId() ISODate() NumberLong() NumberInt() NumberDecimal() UUID()
        BinData() Timestamp() /regex/flags MinKey MaxKey, unquoted keys, 'single quotes'.
Run all with Cmd/Ctrl+Enter, or select text to run only the selection.`
