// Package jsengine runs shell scripts as real JavaScript (variables, loops,
// functions) in an embedded engine, with a db object backed by the driver. It
// needs no mongosh, and results come back as the shell tab's usual outputs.
// Read-only connections refuse every write, as in the built-in shell.
package jsengine

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/model"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/shell"
)

// Runner runs scripts on open connections.
type Runner struct {
	src     mongox.Source
	maxDocs int
}

// New returns a runner; maxDocs bounds the documents shown for a result.
func New(src mongox.Source, maxDocs int) *Runner {
	if maxDocs <= 0 {
		maxDocs = 50
	}
	return &Runner{src: src, maxDocs: maxDocs}
}

// run holds one script's state.
type run struct {
	ctx     context.Context
	vm      *goja.Runtime
	sess    *mongox.Session
	db      string
	outputs []shell.Output
	maxDocs int
	start   time.Time
}

var (
	reUse  = regexp.MustCompile(`(?m)^[ \t]*use[ \t]+([A-Za-z0-9_$.-]+)[ \t]*;?[ \t]*$`)
	reShow = regexp.MustCompile(`(?m)^[ \t]*show[ \t]+(dbs|databases|collections|tables)[ \t]*;?[ \t]*$`)
)

// translate turns mongosh's non-JavaScript helpers ("use db", "show
// collections") into calls, line by line, so line numbers stay the same.
func translate(script string) string {
	script = reUse.ReplaceAllString(script, `__use("$1");`)
	return reShow.ReplaceAllString(script, `__show("$1");`)
}

// Run executes script against connID, starting in database db.
func (r *Runner) Run(ctx context.Context, connID, db, script string) (shell.Result, error) {
	sess, err := r.src.Session(connID)
	if err != nil {
		return shell.Result{}, err
	}
	if db == "" {
		db = "test"
	}
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
	st := &run{ctx: ctx, vm: vm, sess: sess, db: db, maxDocs: r.maxDocs, start: time.Now()}
	st.install()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("The script was stopped.")
		case <-done:
		}
	}()

	val, err := vm.RunScript("script", translate(script))
	if err != nil {
		if ctx.Err() != nil {
			return shell.Result{}, apperr.From(ctx.Err())
		}
		st.outputs = append(st.outputs, shell.Output{Statement: "script", Kind: "error", Text: errText(err)})
		return shell.Result{Outputs: st.outputs, CurrentDB: st.db}, nil
	}
	if out, ok := st.result(val); ok {
		st.outputs = append(st.outputs, out)
	}
	if st.outputs == nil {
		st.outputs = []shell.Output{}
	}
	return shell.Result{Outputs: st.outputs, CurrentDB: st.db}, nil
}

// throw raises err inside the script as a JavaScript error.
func (st *run) throw(err error) {
	panic(st.vm.NewGoError(errors.New(errText(err))))
}

func (st *run) must(err error) {
	if err != nil {
		st.throw(err)
	}
}

func (st *run) printText(text string) {
	st.outputs = append(st.outputs, shell.Output{Statement: "print", Kind: "text", Text: text})
}

// result turns the script's last value into an output.
func (st *run) result(v goja.Value) (shell.Output, bool) {
	if v == nil || goja.IsUndefined(v) {
		return shell.Output{}, false
	}
	ms := time.Since(st.start).Milliseconds()
	if c, ok := v.Export().(*Cursor); ok {
		docs, more, err := c.firstPage(st.maxDocs)
		if err != nil {
			return shell.Output{Statement: "result", Kind: "error", Text: errText(err)}, true
		}
		out := shell.Output{Statement: c.ns(), Kind: "documents", Documents: make([]model.Document, 0, len(docs)), HasMore: more, DurationMs: ms}
		for _, d := range docs {
			out.Documents = append(out.Documents, model.NewDocument(d))
		}
		if more {
			out.Text = fmt.Sprintf("The first %d documents. Use .skip(), .limit() or .toArray() to see others.", st.maxDocs)
		}
		return out, true
	}
	if obj, ok := v.(*goja.Object); ok && obj.ClassName() == "Array" {
		x, err := fromJS(st.vm, v)
		if err == nil {
			arr := x.(bson.A)
			allDocs := len(arr) > 0
			for _, el := range arr {
				if _, ok := el.(bson.D); !ok {
					allDocs = false
				}
			}
			if allDocs {
				out := shell.Output{Statement: "result", Kind: "documents", DurationMs: ms}
				for i, el := range arr {
					if i >= st.maxDocs {
						out.HasMore = true
						out.Text = fmt.Sprintf("The first %d of %d documents.", st.maxDocs, len(arr))
						break
					}
					out.Documents = append(out.Documents, model.NewDocument(el.(bson.D)))
				}
				return out, true
			}
		}
	}
	return shell.Output{Statement: "result", Kind: "text", Text: display(st.vm, v, false), DurationMs: ms}, true
}

// install defines the globals: db, print, the BSON helpers, use and show.
func (st *run) install() {
	vm := st.vm
	set := func(name string, fn func(goja.FunctionCall) goja.Value) { _ = vm.Set(name, fn) }

	set("print", func(c goja.FunctionCall) goja.Value {
		parts := make([]string, len(c.Arguments))
		for i, a := range c.Arguments {
			parts[i] = display(vm, a, true)
		}
		st.printText(strings.Join(parts, " "))
		return goja.Undefined()
	})
	set("printjson", func(c goja.FunctionCall) goja.Value {
		st.printText(display(vm, c.Argument(0), false))
		return goja.Undefined()
	})
	_ = vm.Set("console", map[string]any{"log": vm.Get("print")})
	set("sleep", func(c goja.FunctionCall) goja.Value {
		d := time.Duration(c.Argument(0).ToInteger()) * time.Millisecond
		select {
		case <-time.After(d):
		case <-st.ctx.Done():
			st.throw(st.ctx.Err())
		}
		return goja.Undefined()
	})

	// BSON helpers, as in mongosh.
	set("ObjectId", func(c goja.FunctionCall) goja.Value {
		if len(c.Arguments) == 0 || goja.IsUndefined(c.Argument(0)) {
			return vm.ToValue(&ObjectIDValue{ID: bson.NewObjectID()})
		}
		id, err := bson.ObjectIDFromHex(c.Argument(0).String())
		if err != nil {
			st.throw(apperr.Invalid("ObjectId needs 24 hex characters, not %q.", c.Argument(0).String()))
		}
		return vm.ToValue(&ObjectIDValue{ID: id})
	})
	_ = vm.Set("ObjectID", vm.Get("ObjectId"))
	set("ISODate", func(c goja.FunctionCall) goja.Value {
		if len(c.Arguments) == 0 {
			return newDate(vm, time.Now())
		}
		t, err := mql.ParseDate(c.Argument(0).String())
		if err != nil {
			st.throw(apperr.Invalid("ISODate: %v", err))
		}
		return newDate(vm, t)
	})
	set("NumberLong", func(c goja.FunctionCall) goja.Value {
		x, err := mql.ParseValue("NumberLong(" + quoteArg(c.Argument(0)) + ")")
		st.must(err)
		return vm.ToValue(&LongValue{N: x.(int64)})
	})
	_ = vm.Set("Long", vm.Get("NumberLong"))
	set("NumberInt", func(c goja.FunctionCall) goja.Value {
		return vm.ToValue(&IntValue{N: int32(c.Argument(0).ToInteger())})
	})
	_ = vm.Set("Int32", vm.Get("NumberInt"))
	set("NumberDecimal", func(c goja.FunctionCall) goja.Value {
		d, err := bson.ParseDecimal128(c.Argument(0).String())
		if err != nil {
			st.throw(apperr.Invalid("NumberDecimal: %v", err))
		}
		return vm.ToValue(&DecimalValue{D: d})
	})
	_ = vm.Set("Decimal128", vm.Get("NumberDecimal"))
	set("UUID", func(c goja.FunctionCall) goja.Value {
		src := "UUID()"
		if len(c.Arguments) > 0 {
			src = "UUID(" + quoteArg(c.Argument(0)) + ")"
		}
		x, err := mql.ParseValue(src)
		st.must(err)
		return vm.ToValue(&BinaryValue{B: x.(bson.Binary)})
	})
	set("Timestamp", func(c goja.FunctionCall) goja.Value {
		return vm.ToValue(&TimestampValue{T: bson.Timestamp{T: uint32(c.Argument(0).ToInteger()), I: uint32(c.Argument(1).ToInteger())}})
	})

	set("__use", func(c goja.FunctionCall) goja.Value {
		st.db = c.Argument(0).String()
		st.printText("switched to db " + st.db)
		_ = vm.Set("db", st.dbObject(st.db))
		return goja.Undefined()
	})
	set("__show", func(c goja.FunctionCall) goja.Value {
		switch c.Argument(0).String() {
		case "dbs", "databases":
			res, err := st.sess.Client.ListDatabases(st.ctx, bson.D{})
			st.must(err)
			lines := make([]string, 0, len(res.Databases))
			for _, d := range res.Databases {
				lines = append(lines, fmt.Sprintf("%-24s %d bytes", d.Name, d.SizeOnDisk))
			}
			st.printText(strings.Join(lines, "\n"))
		default:
			names, err := st.sess.Client.Database(st.db).ListCollectionNames(st.ctx, bson.D{})
			st.must(err)
			st.printText(strings.Join(names, "\n"))
		}
		return goja.Undefined()
	})
	_ = vm.Set("db", st.dbObject(st.db))
}

func quoteArg(v goja.Value) string {
	if s, ok := v.Export().(string); ok {
		return mql.Quote(s)
	}
	return v.String()
}

// ---- db ---------------------------------------------------------------------

// dbObject is the db global: its methods, and any other property is a
// collection (db.orders), like mongosh.
func (st *run) dbObject(name string) goja.Value {
	return st.vm.NewDynamicObject(&dbObj{st: st, name: name})
}

type dbObj struct {
	st   *run
	name string
}

func (d *dbObj) Has(key string) bool { return true }
func (d *dbObj) Set(string, goja.Value) bool {
	return false
}
func (d *dbObj) Delete(string) bool { return false }
func (d *dbObj) Keys() []string     { return nil }

func (d *dbObj) Get(key string) goja.Value {
	st, vm := d.st, d.st.vm
	db := st.sess.Client.Database(d.name)
	fn := func(f func(goja.FunctionCall) goja.Value) goja.Value { return vm.ToValue(f) }
	switch key {
	case "getName", "toString":
		return fn(func(goja.FunctionCall) goja.Value { return vm.ToValue(d.name) })
	case "getCollection":
		return fn(func(c goja.FunctionCall) goja.Value { return st.collection(d.name, c.Argument(0).String()) })
	case "getSiblingDB", "getDB":
		return fn(func(c goja.FunctionCall) goja.Value { return st.dbObject(c.Argument(0).String()) })
	case "getCollectionNames":
		return fn(func(goja.FunctionCall) goja.Value {
			names, err := db.ListCollectionNames(st.ctx, bson.D{})
			st.must(err)
			out := make([]any, len(names))
			for i, n := range names {
				out[i] = n
			}
			return vm.NewArray(out...)
		})
	case "runCommand", "adminCommand":
		target := db
		if key == "adminCommand" {
			target = st.sess.Client.Database("admin")
		}
		return fn(func(c goja.FunctionCall) goja.Value {
			var cmd bson.D
			if s, ok := c.Argument(0).Export().(string); ok {
				cmd = bson.D{{Key: s, Value: int32(1)}}
			} else {
				var err error
				cmd, err = docFromJS(vm, c.Argument(0), "The command")
				st.must(err)
			}
			if len(cmd) == 0 {
				st.throw(apperr.Invalid("The command is empty."))
			}
			if st.sess.ReadOnly && !shell.ReadOnlyCommand(cmd[0].Key) {
				st.must(st.sess.RequireWritable())
			}
			var out bson.D
			st.must(errOrNil(target.RunCommand(st.ctx, cmd).Decode(&out)))
			return toJS(vm, out)
		})
	case "dropDatabase":
		return fn(func(goja.FunctionCall) goja.Value {
			st.must(st.sess.RequireWritable())
			st.must(errOrNil(db.Drop(st.ctx)))
			return toJS(vm, bson.D{{Key: "ok", Value: int32(1)}, {Key: "dropped", Value: d.name}})
		})
	case "createCollection":
		return fn(func(c goja.FunctionCall) goja.Value {
			st.must(st.sess.RequireWritable())
			st.must(errOrNil(db.CreateCollection(st.ctx, c.Argument(0).String())))
			return toJS(vm, bson.D{{Key: "ok", Value: int32(1)}})
		})
	case "stats":
		return fn(func(goja.FunctionCall) goja.Value {
			var out bson.D
			st.must(errOrNil(db.RunCommand(st.ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&out)))
			return toJS(vm, out)
		})
	case "valueOf", "constructor", "then", "Symbol.toPrimitive", "toJSON":
		return goja.Undefined()
	}
	return st.collection(d.name, key)
}

func errOrNil(err error) error {
	if err == nil {
		return nil
	}
	return apperr.From(err)
}

// ---- collections ------------------------------------------------------------

func (st *run) collection(db, name string) goja.Value {
	return st.vm.ToValue(&Collection{st: st, c: st.sess.Client.Database(db).Collection(name), db: db, name: name})
}

// Collection is db.<name>: the usual mongosh collection methods.
type Collection struct {
	st       *run
	c        *mongo.Collection
	db, name string
}

func (c *Collection) ToString() string    { return c.db + "." + c.name }
func (c *Collection) GetName() string     { return c.name }
func (c *Collection) GetFullName() string { return c.db + "." + c.name }

func (c *Collection) writable() { c.st.must(c.st.sess.RequireWritable()) }

func (c *Collection) Find(call goja.FunctionCall) goja.Value {
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	proj, err := docFromJS(c.st.vm, call.Argument(1), "The projection")
	c.st.must(err)
	return c.st.vm.ToValue(&Cursor{st: c.st, coll: c, filter: filter, proj: proj})
}

func (c *Collection) FindOne(call goja.FunctionCall) goja.Value {
	cur := &Cursor{st: c.st, coll: c, limit: 1}
	var err error
	cur.filter, err = docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	cur.proj, err = docFromJS(c.st.vm, call.Argument(1), "The projection")
	c.st.must(err)
	docs, err := cur.all()
	c.st.must(err)
	if len(docs) == 0 {
		return goja.Null()
	}
	return toJS(c.st.vm, docs[0])
}

func (c *Collection) Aggregate(call goja.FunctionCall) goja.Value {
	x, err := fromJS(c.st.vm, call.Argument(0))
	c.st.must(err)
	pipe, ok := x.(bson.A)
	if !ok {
		pipe = bson.A{}
		for _, a := range call.Arguments {
			st, err := fromJS(c.st.vm, a)
			c.st.must(err)
			pipe = append(pipe, st)
		}
	}
	for _, s := range pipe {
		if d, ok := s.(bson.D); ok && len(d) > 0 && (d[0].Key == "$out" || d[0].Key == "$merge") {
			c.writable()
		}
	}
	return c.st.vm.ToValue(&Cursor{st: c.st, coll: c, pipeline: pipe, isAgg: true})
}

// Methods that read their arguments take a goja.FunctionCall and must
// return a goja.Value: only that signature receives the raw arguments.
func (c *Collection) CountDocuments(call goja.FunctionCall) goja.Value {
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	n, err := c.c.CountDocuments(c.st.ctx, filter)
	c.st.must(errOrNil(err))
	return c.st.vm.ToValue(n)
}

func (c *Collection) EstimatedDocumentCount() int64 {
	n, err := c.c.EstimatedDocumentCount(c.st.ctx)
	c.st.must(errOrNil(err))
	return n
}

func (c *Collection) Distinct(call goja.FunctionCall) goja.Value {
	filter, err := docFromJS(c.st.vm, call.Argument(1), "The filter")
	c.st.must(err)
	res := c.c.Distinct(c.st.ctx, call.Argument(0).String(), filter)
	c.st.must(errOrNil(res.Err()))
	var vals bson.A
	c.st.must(errOrNil(res.Decode(&vals)))
	return toJS(c.st.vm, vals)
}

func (c *Collection) InsertOne(call goja.FunctionCall) goja.Value {
	c.writable()
	doc, err := docFromJS(c.st.vm, call.Argument(0), "The document")
	c.st.must(err)
	res, err := c.c.InsertOne(c.st.ctx, doc)
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, bson.D{{Key: "acknowledged", Value: true}, {Key: "insertedId", Value: res.InsertedID}})
}

func (c *Collection) InsertMany(call goja.FunctionCall) goja.Value {
	c.writable()
	x, err := fromJS(c.st.vm, call.Argument(0))
	c.st.must(err)
	arr, ok := x.(bson.A)
	if !ok {
		c.st.throw(apperr.Invalid("insertMany needs an array of documents."))
	}
	docs := make([]any, len(arr))
	copy(docs, arr)
	res, err := c.c.InsertMany(c.st.ctx, docs)
	c.st.must(errOrNil(err))
	ids := bson.D{}
	for i, id := range res.InsertedIDs {
		ids = append(ids, bson.E{Key: fmt.Sprint(i), Value: id})
	}
	return toJS(c.st.vm, bson.D{{Key: "acknowledged", Value: true}, {Key: "insertedIds", Value: ids}})
}

// update parses the filter and update (a document or a pipeline).
func (c *Collection) update(call goja.FunctionCall) (bson.D, any) {
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	upd, err := fromJS(c.st.vm, call.Argument(1))
	c.st.must(err)
	switch upd.(type) {
	case bson.D, bson.A:
	default:
		c.st.throw(apperr.Invalid("The update must be a document or a pipeline."))
	}
	return filter, upd
}

func upsertOpt(vm *goja.Runtime, v goja.Value) bool {
	if obj, ok := v.(*goja.Object); ok {
		return obj.Get("upsert") != nil && obj.Get("upsert").ToBoolean()
	}
	return false
}

func updateResult(r *mongo.UpdateResult) bson.D {
	out := bson.D{{Key: "acknowledged", Value: true}, {Key: "matchedCount", Value: r.MatchedCount}, {Key: "modifiedCount", Value: r.ModifiedCount}, {Key: "upsertedCount", Value: r.UpsertedCount}}
	if r.UpsertedID != nil {
		out = append(out, bson.E{Key: "upsertedId", Value: r.UpsertedID})
	}
	return out
}

func (c *Collection) UpdateOne(call goja.FunctionCall) goja.Value {
	c.writable()
	filter, upd := c.update(call)
	opts := updateOpts(upsertOpt(c.st.vm, call.Argument(2)))
	r, err := c.c.UpdateOne(c.st.ctx, filter, upd, opts)
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, updateResult(r))
}

func (c *Collection) UpdateMany(call goja.FunctionCall) goja.Value {
	c.writable()
	filter, upd := c.update(call)
	r, err := c.c.UpdateMany(c.st.ctx, filter, upd, updateManyOpts(upsertOpt(c.st.vm, call.Argument(2))))
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, updateResult(r))
}

func (c *Collection) ReplaceOne(call goja.FunctionCall) goja.Value {
	c.writable()
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	doc, err := docFromJS(c.st.vm, call.Argument(1), "The replacement")
	c.st.must(err)
	r, err := c.c.ReplaceOne(c.st.ctx, filter, doc, replaceOpts(upsertOpt(c.st.vm, call.Argument(2))))
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, updateResult(r))
}

func (c *Collection) DeleteOne(call goja.FunctionCall) goja.Value {
	c.writable()
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	r, err := c.c.DeleteOne(c.st.ctx, filter)
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, bson.D{{Key: "acknowledged", Value: true}, {Key: "deletedCount", Value: r.DeletedCount}})
}

func (c *Collection) DeleteMany(call goja.FunctionCall) goja.Value {
	c.writable()
	filter, err := docFromJS(c.st.vm, call.Argument(0), "The filter")
	c.st.must(err)
	r, err := c.c.DeleteMany(c.st.ctx, filter)
	c.st.must(errOrNil(err))
	return toJS(c.st.vm, bson.D{{Key: "acknowledged", Value: true}, {Key: "deletedCount", Value: r.DeletedCount}})
}

func (c *Collection) CreateIndex(call goja.FunctionCall) goja.Value {
	c.writable()
	keys, err := docFromJS(c.st.vm, call.Argument(0), "The index keys")
	c.st.must(err)
	opts, err := docFromJS(c.st.vm, call.Argument(1), "The index options")
	c.st.must(err)
	spec := bson.D{{Key: "key", Value: keys}}
	name := ""
	for _, e := range opts {
		if e.Key == "name" {
			name, _ = e.Value.(string)
		}
		spec = append(spec, e)
	}
	if name == "" {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s_%v", k.Key, k.Value))
		}
		spec = append(spec, bson.E{Key: "name", Value: strings.Join(parts, "_")})
		name = strings.Join(parts, "_")
	}
	cmd := bson.D{{Key: "createIndexes", Value: c.name}, {Key: "indexes", Value: bson.A{spec}}}
	c.st.must(errOrNil(c.c.Database().RunCommand(c.st.ctx, cmd).Err()))
	return c.st.vm.ToValue(name)
}

func (c *Collection) GetIndexes() goja.Value {
	cur, err := c.c.Indexes().List(c.st.ctx)
	c.st.must(errOrNil(err))
	var specs []bson.D
	c.st.must(errOrNil(cur.All(c.st.ctx, &specs)))
	arr := bson.A{}
	for _, s := range specs {
		arr = append(arr, s)
	}
	return toJS(c.st.vm, arr)
}

func (c *Collection) Drop() bool {
	c.writable()
	c.st.must(errOrNil(c.c.Drop(c.st.ctx)))
	return true
}
