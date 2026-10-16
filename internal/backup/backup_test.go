package backup

import (
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	w, err := Create(dir, "sync", "test run", "shop", "orders/2024")
	if err != nil {
		t.Fatal(err)
	}
	dec, _ := bson.ParseDecimal128("9.99")
	docs := []bson.D{
		{{Key: "_id", Value: bson.NewObjectID()}, {Key: "n", Value: int64(5)}, {Key: "p", Value: dec}, {Key: "d", Value: bson.NewDateTimeFromTime(time.Now())}},
		{{Key: "_id", Value: "custom"}, {Key: "x", Value: 1.0}},
	}
	if err := w.Replaced(docs[0]); err != nil {
		t.Fatal(err)
	}
	if err := w.Deleted(docs[1]); err != nil {
		t.Fatal(err)
	}
	if err := w.Inserted(int32(42)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	info := w.Info()
	if info.Replaced != 1 || info.Deleted != 1 || info.Inserted != 1 || info.Bytes == 0 {
		t.Fatalf("info: %+v", info)
	}
	if strings.Contains(info.Path, "orders/2024") {
		t.Fatal("collection name must be made filename-safe")
	}
	st, _ := os.Stat(info.Path)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perms %v", st.Mode().Perm())
	}

	var got []any
	h, err := Read(info.Path, func(op string, doc bson.D, id any) error {
		if doc != nil {
			got = append(got, doc)
		} else {
			got = append(got, id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.Operation != "sync" || h.DB != "shop" || h.Coll != "orders/2024" {
		t.Fatalf("header: %+v", h)
	}
	if !mql.Equal(got[0], docs[0]) || !mql.Equal(got[1], docs[1]) || !mql.Equal(got[2], int32(42)) {
		t.Fatalf("round trip lost types: %v", mql.Compact(bson.A(got)))
	}

	list, err := List(dir)
	if err != nil || len(list) != 1 || list[0].ID != h.ID || list[0].Replaced != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
}

func TestReadRejectsForeignFiles(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/x.ndjson"
	_ = os.WriteFile(path, []byte(`{"hello":1}`+"\n"), 0o600)
	if _, err := Read(path, func(string, bson.D, any) error { return nil }); err == nil {
		t.Fatal("foreign file accepted")
	}
	if list, _ := List(dir); len(list) != 0 {
		t.Fatal("foreign file listed")
	}
	if list, err := List(dir + "/missing"); err != nil || len(list) != 0 {
		t.Fatal("missing dir should list empty")
	}
}
