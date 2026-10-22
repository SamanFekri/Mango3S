package jsengine

import (
	"github.com/dop251/goja"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
)

// Cursor is the result of find() or aggregate(). Modifiers (sort, limit…)
// apply until it's first read; reading is lazy and streams.
type Cursor struct {
	st       *run
	coll     *Collection
	filter   bson.D
	proj     bson.D
	sort     bson.D
	skip     int64
	limit    int64
	pipeline bson.A
	isAgg    bool

	open   *mongo.Cursor
	peeked bson.D
	done   bool
}

func (c *Cursor) ns() string { return c.coll.db + "." + c.coll.name }

func (c *Cursor) modify() {
	if c.open != nil {
		c.st.throw(apperr.Invalid("The cursor was already read; modifiers must come before reading it."))
	}
}

func (c *Cursor) Sort(v goja.Value) *Cursor {
	c.modify()
	d, err := docFromJS(c.st.vm, v, "The sort")
	c.st.must(err)
	c.sort = d
	return c
}

func (c *Cursor) Limit(n int64) *Cursor { c.modify(); c.limit = n; return c }
func (c *Cursor) Skip(n int64) *Cursor  { c.modify(); c.skip = n; return c }

func (c *Cursor) Projection(v goja.Value) *Cursor {
	c.modify()
	d, err := docFromJS(c.st.vm, v, "The projection")
	c.st.must(err)
	c.proj = d
	return c
}

// Pretty is accepted for mongosh compatibility; output is always pretty.
func (c *Cursor) Pretty() *Cursor { return c }

func (c *Cursor) start() {
	if c.open != nil || c.done {
		return
	}
	var cur *mongo.Cursor
	var err error
	if c.isAgg {
		cur, err = c.coll.c.Aggregate(c.st.ctx, c.pipeline)
	} else {
		o := options.Find()
		if len(c.proj) > 0 {
			o.SetProjection(c.proj)
		}
		if len(c.sort) > 0 {
			o.SetSort(c.sort)
		}
		if c.skip > 0 {
			o.SetSkip(c.skip)
		}
		if c.limit > 0 {
			o.SetLimit(c.limit)
		}
		cur, err = c.coll.c.Find(c.st.ctx, c.filter, o)
	}
	if err != nil {
		c.st.throw(apperr.From(err))
	}
	c.open = cur
}

// nextDoc returns the next document, or nil at the end.
func (c *Cursor) nextDoc() bson.D {
	if c.peeked != nil {
		d := c.peeked
		c.peeked = nil
		return d
	}
	c.start()
	if c.done {
		return nil
	}
	if !c.open.Next(c.st.ctx) {
		if err := c.open.Err(); err != nil {
			c.st.throw(apperr.From(err))
		}
		c.done = true
		_ = c.open.Close(c.st.ctx)
		return nil
	}
	var d bson.D
	if err := c.open.Decode(&d); err != nil {
		c.st.throw(apperr.From(err))
	}
	return d
}

func (c *Cursor) HasNext() bool {
	if c.peeked == nil {
		c.peeked = c.nextDoc()
	}
	return c.peeked != nil
}

func (c *Cursor) Next() goja.Value {
	d := c.nextDoc()
	if d == nil {
		c.st.throw(apperr.Invalid("The cursor has no more documents."))
	}
	return toJS(c.st.vm, d)
}

func (c *Cursor) ToArray() goja.Value {
	docs, err := c.all()
	c.st.must(err)
	arr := make(bson.A, len(docs))
	for i, d := range docs {
		arr[i] = d
	}
	return toJS(c.st.vm, arr)
}

func (c *Cursor) ForEach(fn goja.Callable) {
	for d := c.nextDoc(); d != nil; d = c.nextDoc() {
		if _, err := fn(goja.Undefined(), toJS(c.st.vm, d)); err != nil {
			c.st.rethrow(err)
		}
	}
}

func (c *Cursor) Map(fn goja.Callable) goja.Value {
	out := []any{}
	for d := c.nextDoc(); d != nil; d = c.nextDoc() {
		v, err := fn(goja.Undefined(), toJS(c.st.vm, d))
		if err != nil {
			c.st.rethrow(err)
		}
		out = append(out, v)
	}
	return c.st.vm.NewArray(out...)
}

// Count counts the matching documents (find cursors only; it ignores
// limit and skip like mongosh's legacy count()).
func (c *Cursor) Count() int64 {
	if c.isAgg {
		c.st.throw(apperr.Invalid("count() works on find cursors; add a $count stage to the pipeline instead."))
	}
	n, err := c.coll.c.CountDocuments(c.st.ctx, c.filter)
	c.st.must(errOrNil(err))
	return n
}

// all reads every remaining document (with a safety cap).
func (c *Cursor) all() (docs []bson.D, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recoveredErr(r)
		}
	}()
	const capDocs = 100000
	for d := c.nextDoc(); d != nil; d = c.nextDoc() {
		docs = append(docs, d)
		if len(docs) > capDocs {
			return nil, apperr.Invalid("More than %d documents: add a limit or use forEach.", capDocs)
		}
	}
	return docs, nil
}

// firstPage reads up to n documents and reports whether there are more.
func (c *Cursor) firstPage(n int) (docs []bson.D, more bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = recoveredErr(r)
		}
	}()
	for len(docs) < n {
		d := c.nextDoc()
		if d == nil {
			return docs, false, nil
		}
		docs = append(docs, d)
	}
	return docs, c.HasNext(), nil
}

func updateOpts(upsert bool) *options.UpdateOneOptionsBuilder {
	return options.UpdateOne().SetUpsert(upsert)
}

func updateManyOpts(upsert bool) *options.UpdateManyOptionsBuilder {
	return options.UpdateMany().SetUpsert(upsert)
}

func replaceOpts(upsert bool) *options.ReplaceOptionsBuilder {
	return options.Replace().SetUpsert(upsert)
}

// recoveredErr turns a value raised by throw (a JavaScript error object)
// back into a Go error, for code that runs outside the script.
func recoveredErr(r any) error {
	switch x := r.(type) {
	case error:
		return x
	case goja.Value:
		if obj, ok := x.(*goja.Object); ok {
			if m := obj.Get("message"); m != nil && !goja.IsUndefined(m) {
				return apperr.Invalid("%s", m.String())
			}
		}
		return apperr.Invalid("%s", x.String())
	}
	panic(r)
}

// rethrow passes an error from a script callback on to the script.
func (st *run) rethrow(err error) {
	if ex, ok := err.(*goja.Exception); ok {
		panic(ex.Value())
	}
	st.throw(err)
}
