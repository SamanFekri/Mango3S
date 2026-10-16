// Package docpath reads and rewrites values at dotted paths inside bson.D
// documents. Intermediate arrays of documents are traversed element by
// element ("items.sku" reaches the sku of every item), matching how MongoDB
// queries interpret paths. All writers return a modified copy.
package docpath

import (
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Split turns "a.b.c" into its segments.
func Split(path string) []string { return strings.Split(path, ".") }

// Clone deep-copies documents and arrays (scalars are immutable values).
func Clone(v any) any {
	switch t := v.(type) {
	case bson.D:
		out := make(bson.D, len(t))
		for i, e := range t {
			out[i] = bson.E{Key: e.Key, Value: Clone(e.Value)}
		}
		return out
	case bson.A:
		out := make(bson.A, len(t))
		for i, e := range t {
			out[i] = Clone(e)
		}
		return out
	case []byte:
		return append([]byte(nil), t...)
	case bson.Binary:
		return bson.Binary{Subtype: t.Subtype, Data: append([]byte(nil), t.Data...)}
	}
	return v
}

// CloneDoc deep-copies a document.
func CloneDoc(d bson.D) bson.D { return Clone(d).(bson.D) }

func index(d bson.D, key string) int {
	for i, e := range d {
		if e.Key == key {
			return i
		}
	}
	return -1
}

// Get returns the value at path without traversing arrays.
func Get(d bson.D, path string) (any, bool) {
	var cur any = d
	for _, seg := range Split(path) {
		doc, ok := cur.(bson.D)
		if !ok {
			return nil, false
		}
		i := index(doc, seg)
		if i < 0 {
			return nil, false
		}
		cur = doc[i].Value
	}
	return cur, true
}

// Update calls f for every value found at path (traversing arrays of
// documents on the way) and stores what f returns. f returns keep=false to
// remove the field. It returns how many values were visited. d is modified in
// place; pass a clone to keep the original.
func Update(d bson.D, path string, f func(v any) (any, bool, error)) (bson.D, int, error) {
	return update(d, Split(path), f)
}

func update(d bson.D, segs []string, f func(any) (any, bool, error)) (bson.D, int, error) {
	i := index(d, segs[0])
	if i < 0 {
		return d, 0, nil
	}
	if len(segs) == 1 {
		nv, keep, err := f(d[i].Value)
		if err != nil {
			return d, 1, err
		}
		if !keep {
			return append(d[:i:i], d[i+1:]...), 1, nil
		}
		d[i].Value = nv
		return d, 1, nil
	}
	switch child := d[i].Value.(type) {
	case bson.D:
		nd, n, err := update(child, segs[1:], f)
		d[i].Value = nd
		return d, n, err
	case bson.A:
		total := 0
		for j, el := range child {
			if sub, ok := el.(bson.D); ok {
				nd, n, err := update(sub, segs[1:], f)
				child[j] = nd
				total += n
				if err != nil {
					return d, total, err
				}
			}
		}
		return d, total, nil
	}
	return d, 0, nil
}

// Set stores v at path, creating intermediate documents. It reports false
// when an intermediate segment exists but is not a document.
func Set(d bson.D, path string, v any) (bson.D, bool) {
	return set(d, Split(path), v)
}

func set(d bson.D, segs []string, v any) (bson.D, bool) {
	i := index(d, segs[0])
	if len(segs) == 1 {
		if i >= 0 {
			d[i].Value = v
			return d, true
		}
		return append(d, bson.E{Key: segs[0], Value: v}), true
	}
	if i < 0 {
		nd, _ := set(bson.D{}, segs[1:], v)
		return append(d, bson.E{Key: segs[0], Value: nd}), true
	}
	child, ok := d[i].Value.(bson.D)
	if !ok {
		return d, false
	}
	nd, ok := set(child, segs[1:], v)
	d[i].Value = nd
	return d, ok
}

// Delete removes the value at path (no array traversal) and returns it.
func Delete(d bson.D, path string) (bson.D, any, bool) {
	segs := Split(path)
	var removed any
	var found bool
	var del func(bson.D, []string) bson.D
	del = func(doc bson.D, segs []string) bson.D {
		i := index(doc, segs[0])
		if i < 0 {
			return doc
		}
		if len(segs) == 1 {
			removed, found = doc[i].Value, true
			return append(doc[:i:i], doc[i+1:]...)
		}
		if child, ok := doc[i].Value.(bson.D); ok {
			doc[i].Value = del(child, segs[1:])
		}
		return doc
	}
	return del(d, segs), removed, found
}

// Exists reports whether path is present (no array traversal).
func Exists(d bson.D, path string) bool {
	_, ok := Get(d, path)
	return ok
}

// Parent splits "a.b.c" into ("a.b", "c").
func Parent(path string) (string, string) {
	i := strings.LastIndexByte(path, '.')
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
