//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/shell"
)

func TestShellResultsArePaged(t *testing.T) {
	e := setup(t)
	s, _ := e.mgr.Session(e.conn)
	docs := make([]any, 45)
	for i := range docs {
		docs[i] = bson.D{{Key: "_id", Value: int32(i)}, {Key: "n", Value: int32(i)}}
	}
	if _, err := s.Client.Database(e.db).Collection("paged").InsertMany(e.ctx, docs); err != nil {
		t.Fatal(err)
	}
	sh := shell.New(e.mgr)
	ids := func(o shell.Output) string {
		var b []string
		for _, d := range o.Documents {
			b = append(b, d.ID)
		}
		return strings.Join(b, ",")
	}
	stmt := `db.getCollection("paged").find({}).sort({ _id: 1 })`
	res, err := sh.Execute(e.ctx, e.conn, e.db, stmt, 20)
	if err != nil {
		t.Fatal(err)
	}
	first := res.Outputs[0]
	if !first.Pageable || !first.HasMore || len(first.Documents) != 20 || first.Page != 0 {
		t.Fatalf("first page: %+v", first)
	}
	for page, want := range map[int]string{1: "20", 2: "40"} {
		out, err := sh.Page(e.ctx, e.conn, e.db, stmt, page, 20)
		if err != nil {
			t.Fatal(err)
		}
		if out.Documents[0].ID != want || out.Page != page {
			t.Fatalf("page %d starts at %s", page, out.Documents[0].ID)
		}
		if page == 2 && (out.HasMore || len(out.Documents) != 5) {
			t.Fatalf("last page: %d docs, hasMore=%v", len(out.Documents), out.HasMore)
		}
	}
	// The statement's own skip/limit still apply: 12 documents from _id 3, in pages of 5.
	lim := `db.paged.find().sort({ _id: 1 }).skip(3).limit(12)`
	var got []string
	for p := 0; ; p++ {
		out, err := sh.Page(e.ctx, e.conn, e.db, lim, p, 5)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ids(out))
		if !out.HasMore {
			break
		}
	}
	if strings.Join(got, "|") != "3,4,5,6,7|8,9,10,11,12|13,14" {
		t.Fatalf("skip/limit paging: %v", got)
	}
	// Aggregations are paged too.
	agg := `db.paged.aggregate([{ $match: { n: { $gte: 10 } } }, { $sort: { n: 1 } }])`
	out, err := sh.Page(e.ctx, e.conn, e.db, agg, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Pageable || ids(out) != fmt.Sprint("20,21,22,23,24,25,26,27,28,29") || !out.HasMore {
		t.Fatalf("aggregate page: %s %+v", ids(out), out.HasMore)
	}
	if _, err := sh.Page(e.ctx, e.conn, e.db, `db.paged.countDocuments({})`, 1, 10); err == nil {
		t.Fatal("non-cursor statements can't be paged")
	}
}
