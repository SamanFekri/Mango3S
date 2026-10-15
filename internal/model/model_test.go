package model

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestComputePaging(t *testing.T) {
	cases := []struct {
		name              string
		skip, limit       int64
		page, size        int
		wantSkip, wantLim int64
		fetched           int
		wantMore          bool
	}{
		{"first page unlimited", 0, 0, 0, 50, 0, 50, 51, true},
		{"last page unlimited", 0, 0, 2, 50, 100, 50, 10, false},
		{"user skip offsets pages", 10, 0, 1, 20, 30, 20, 21, true},
		{"limit smaller than page", 0, 30, 0, 50, 0, 30, 31, false},
		{"limit spans pages", 0, 120, 1, 50, 50, 50, 51, true},
		{"limit tail page", 0, 120, 2, 50, 100, 20, 21, false},
		{"beyond limit", 0, 100, 2, 50, 100, 0, 0, false},
		{"defaults", -5, 0, -1, 0, 0, 50, 0, false},
	}
	for _, c := range cases {
		p := ComputePaging(c.skip, c.limit, c.page, c.size)
		if p.Skip != c.wantSkip || p.Limit != c.wantLim {
			t.Errorf("%s: got skip=%d limit=%d, want skip=%d limit=%d", c.name, p.Skip, p.Limit, c.wantSkip, c.wantLim)
		}
		skip := c.skip
		if skip < 0 {
			skip = 0
		}
		if got := p.HasMore(c.fetched, skip, c.limit); got != c.wantMore {
			t.Errorf("%s: HasMore = %v, want %v", c.name, got, c.wantMore)
		}
	}
}

func TestNewDocument(t *testing.T) {
	oid := bson.NewObjectID()
	d := NewDocument(bson.D{{Key: "_id", Value: oid}, {Key: "n", Value: int64(2)}})
	if d.ID != `ObjectId("`+oid.Hex()+`")` {
		t.Fatalf("id: %s", d.ID)
	}
	if d.JSON != `{"_id":{"$oid":"`+oid.Hex()+`"},"n":2}` {
		t.Fatalf("json: %s", d.JSON)
	}
	if NewDocument(bson.D{{Key: "x", Value: 1}}).ID != "" {
		t.Fatal("document without _id must have empty ID")
	}
}
