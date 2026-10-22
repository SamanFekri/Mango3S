package monitor

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func status(insert, query, bytesIn int64, conns int32) bson.D {
	return bson.D{
		{Key: "opcounters", Value: bson.D{{Key: "insert", Value: insert}, {Key: "query", Value: query}, {Key: "update", Value: int64(0)}}},
		{Key: "network", Value: bson.D{{Key: "bytesIn", Value: bytesIn}, {Key: "bytesOut", Value: int64(0)}, {Key: "numRequests", Value: int64(0)}}},
		{Key: "connections", Value: bson.D{{Key: "current", Value: conns}, {Key: "available", Value: int32(100)}}},
		{Key: "wiredTiger", Value: bson.D{{Key: "cache", Value: bson.D{
			{Key: "bytes currently in the cache", Value: int64(512)}, {Key: "maximum bytes configured", Value: int64(1024)},
		}}}},
	}
}

func TestRates(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := CountersFrom(status(100, 10, 1000, 5), t0)
	first := Rates(nil, a)
	if !first.First || first.Insert != 0 || first.ConnsCur != 5 || first.CacheMax != 1024 {
		t.Fatalf("first sample: %+v", first)
	}
	b := CountersFrom(status(140, 30, 5000, 7), t0.Add(2*time.Second))
	s := Rates(&a, b)
	if s.First || s.Insert != 20 || s.Query != 10 || s.NetInBps != 2000 || s.ConnsCur != 7 || s.CacheBytes != 512 {
		t.Fatalf("rates: %+v", s)
	}
	// A restart resets the counters: no negative rates.
	c := CountersFrom(status(3, 1, 10, 1), t0.Add(4*time.Second))
	if r := Rates(&b, c); r.Insert != 0 || r.Query != 0 || r.NetInBps != 0 {
		t.Fatalf("after restart: %+v", r)
	}
}

func TestReplicationLag(t *testing.T) {
	primary := time.Date(2026, 1, 1, 12, 0, 10, 0, time.UTC)
	rs := bson.D{
		{Key: "set", Value: "rs0"},
		{Key: "members", Value: bson.A{
			bson.D{{Key: "name", Value: "a:27017"}, {Key: "stateStr", Value: "PRIMARY"}, {Key: "health", Value: 1.0}, {Key: "optimeDate", Value: bson.NewDateTimeFromTime(primary)}, {Key: "self", Value: true}},
			bson.D{{Key: "name", Value: "b:27017"}, {Key: "stateStr", Value: "SECONDARY"}, {Key: "health", Value: 1.0}, {Key: "optimeDate", Value: bson.NewDateTimeFromTime(primary.Add(-4 * time.Second))}, {Key: "pingMs", Value: int64(3)}},
			bson.D{{Key: "name", Value: "c:27017"}, {Key: "stateStr", Value: "ARBITER"}, {Key: "health", Value: 0.0}},
		}},
	}
	r := replicationFrom(rs)
	if r.Set != "rs0" || len(r.Members) != 3 {
		t.Fatalf("%+v", r)
	}
	if m := r.Members[1]; m.LagSecs == nil || *m.LagSecs != 4 || m.PingMs == nil || *m.PingMs != 3 {
		t.Fatalf("secondary: %+v", m)
	}
	if m := r.Members[0]; !m.Self || m.LagSecs == nil || *m.LagSecs != 0 {
		t.Fatalf("primary: %+v", m)
	}
	if m := r.Members[2]; m.Health || m.LagSecs != nil {
		t.Fatalf("arbiter: %+v", m)
	}
}
