//go:build integration

package integration

import (
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/dump"
	"mango3s/internal/manage"
	"mango3s/internal/mql"
)

type quietLog struct{ lines []string }

func (l *quietLog) Log(f string, a ...any) { l.lines = append(l.lines, fmt.Sprintf(f, a...)) }
func (l *quietLog) SetTotal(int64, string) {}
func (l *quietLog) Add(int64)              {}

// Needs the sharded cluster from `make sharded-up`:
// MONGO_SHARDED_URI=mongodb://localhost:27021
func TestShardingAwarenessBackupRestoreAndCopy(t *testing.T) {
	e := setupOptional(t, "MONGO_SHARDED_URI")
	s, _ := e.mgr.Session(e.conn)
	c := s.Client
	if !manage.IsMongos(e.ctx, c) {
		t.Fatal("MONGO_SHARDED_URI must point at a mongos")
	}
	if err := c.Database("admin").RunCommand(e.ctx, bson.D{{Key: "enableSharding", Value: e.db}}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := manage.ShardCollection(e.ctx, c, e.db, "events", bson.D{{Key: "userId", Value: 1}}, false); err != nil {
		t.Fatal(err)
	}
	docs := make([]any, 500)
	for i := range docs {
		docs[i] = bson.D{{Key: "userId", Value: i}, {Key: "n", Value: i * 2}}
	}
	if _, err := c.Database(e.db).Collection("events").InsertMany(e.ctx, docs); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Database(e.db).Collection("plain").InsertOne(e.ctx, bson.D{{Key: "x", Value: 1}}); err != nil {
		t.Fatal(err)
	}

	// Collection and database views.
	cs, err := manage.CollectionShardingInfo(e.ctx, c, e.db, "events")
	if err != nil || cs == nil || cs.Key != `{"userId": 1}` || cs.Chunks < 1 || len(cs.ByShard) < 1 || !cs.Balancing {
		t.Fatalf("collection sharding: %+v %v", cs, err)
	}
	if cs, err := manage.CollectionShardingInfo(e.ctx, c, e.db, "plain"); err != nil || cs != nil {
		t.Fatalf("unsharded collection: %+v %v", cs, err)
	}
	ds, err := manage.DatabaseShardingInfo(e.ctx, c, e.db)
	if err != nil || ds == nil || ds.Primary == "" || len(ds.Shards) != 2 || len(ds.Sharded) != 1 || ds.Sharded[0] != "events" || ds.Balancer.Mode == "" {
		t.Fatalf("database sharding: %+v %v", ds, err)
	}

	// The backup records the shard key …
	dir := t.TempDir() + "/backup"
	m, err := dump.Run(e.ctx, c, dump.Options{Dir: dir, Selection: dump.Selection{Databases: []string{e.db}}}, &quietLog{})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, ns := range m.Namespaces {
		keys[ns.Coll] = ns.ShardKey
	}
	if keys["events"] != `{"userId": 1}` || keys["plain"] != "" {
		t.Fatalf("manifest shard keys: %v", keys)
	}
	// … and a restore can shard the collection again before loading it.
	target := e.db + "_restored"
	t.Cleanup(func() { _ = c.Database(target).Drop(e.ctx) })
	if err := c.Database("admin").RunCommand(e.ctx, bson.D{{Key: "enableSharding", Value: target}}).Err(); err != nil {
		t.Fatal(err)
	}
	log := &quietLog{}
	if _, err := dump.Restore(e.ctx, c, dump.RestoreOptions{Dir: dir, Mode: dump.ModeInsert, ShardCollections: true,
		Renames: []dump.Rename{{From: e.db + ".*", To: target + ".*"}}}, log); err != nil {
		t.Fatalf("restore: %v\n%v", err, log.lines)
	}
	if key, _, _, _, err := manage.ShardKeyOf(e.ctx, c, target, "events"); err != nil || mql.Compact(key) != `{"userId": 1}` {
		t.Fatalf("restored shard key: %v %v\n%v", key, err, log.lines)
	}
	if n, _ := c.Database(target).Collection("events").CountDocuments(e.ctx, bson.D{}); n != 500 {
		t.Fatalf("restored %d documents", n)
	}
	if key, _, _, _, _ := manage.ShardKeyOf(e.ctx, c, target, "plain"); key != nil {
		t.Fatal("an unsharded collection must stay unsharded")
	}

	// Copy with the same option.
	copied := e.db + "_copy"
	t.Cleanup(func() { _ = c.Database(copied).Drop(e.ctx) })
	if err := c.Database("admin").RunCommand(e.ctx, bson.D{{Key: "enableSharding", Value: copied}}).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := dump.Copy(e.ctx, c, c, true, dump.CopyOptions{SrcDB: e.db, Collections: []string{"events"}, DstDB: copied, Mode: dump.ModeInsert, ShardCollections: true}, &quietLog{}); err != nil {
		t.Fatal(err)
	}
	if key, _, _, _, err := manage.ShardKeyOf(e.ctx, c, copied, "events"); err != nil || key == nil {
		t.Fatalf("copied shard key: %v %v", key, err)
	}
}
