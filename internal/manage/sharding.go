package manage

import (
	"context"
	"errors"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// Sharding awareness for a sharded cluster (a mongos connection): a
// collection's shard key and chunk distribution, a database's primary shard,
// the shards and the balancer state.

// ShardChunks is how many chunks of a collection one shard holds.
type ShardChunks struct {
	Shard  string `json:"shard"`
	Chunks int64  `json:"chunks"`
	Jumbo  int64  `json:"jumbo"`
}

// CollectionSharding describes a sharded collection.
type CollectionSharding struct {
	Key       string        `json:"key"` // mongosh syntax
	Unique    bool          `json:"unique"`
	Balancing bool          `json:"balancing"` // false when the balancer is off for this collection
	Chunks    int64         `json:"chunks"`
	Jumbo     int64         `json:"jumbo"`
	ByShard   []ShardChunks `json:"byShard"`
}

// Shard is one shard of the cluster.
type Shard struct {
	ID       string `json:"id"`
	Host     string `json:"host"` // e.g. "shard1/a:27017,b:27017"
	State    int64  `json:"state"`
	Draining bool   `json:"draining"`
}

// Balancer is the balancer's state.
type Balancer struct {
	Mode    string `json:"mode"` // full | off
	InRound bool   `json:"inBalancerRound"`
	Rounds  int64  `json:"numBalancerRounds"`
	Problem string `json:"problem,omitempty"`
}

// DatabaseSharding describes a database on a sharded cluster.
type DatabaseSharding struct {
	Primary  string   `json:"primary"`
	Sharded  []string `json:"shardedCollections"`
	Shards   []Shard  `json:"shards"`
	Balancer Balancer `json:"balancer"`
}

// IsMongos reports whether the client is connected to a mongos.
func IsMongos(ctx context.Context, c *mongo.Client) bool {
	var hello bson.D
	if c.Database("admin").RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello) != nil {
		return false
	}
	msg, _ := mql.Lookup(hello, "msg")
	return msg == "isdbgrid"
}

// ShardKeyOf returns the shard key of db.coll, or nil when it isn't sharded.
func ShardKeyOf(ctx context.Context, c *mongo.Client, db, coll string) (key bson.D, unique bool, uuid any, noBalance bool, err error) {
	var doc bson.D
	err = c.Database("config").Collection("collections").FindOne(ctx, bson.D{{Key: "_id", Value: db + "." + coll}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil, false, nil
	}
	if err != nil {
		return nil, false, nil, false, apperr.From(err)
	}
	if dropped, _ := mql.Lookup(doc, "dropped"); dropped == true {
		return nil, false, nil, false, nil
	}
	k, _ := mql.Lookup(doc, "key")
	key, _ = k.(bson.D)
	if u, ok := mql.Lookup(doc, "unique"); ok {
		unique, _ = u.(bool)
	}
	uuid, _ = mql.Lookup(doc, "uuid")
	if nb, ok := mql.Lookup(doc, "noBalance"); ok {
		noBalance, _ = nb.(bool)
	}
	return key, unique, uuid, noBalance, nil
}

// ShardKeys returns the shard key of every sharded collection in the given
// databases (all when dbs is empty), keyed by "db.coll". It is empty when the
// client isn't connected to a mongos.
func ShardKeys(ctx context.Context, c *mongo.Client, dbs []string) (map[string]bson.D, map[string]bool, error) {
	keys, uniq := map[string]bson.D{}, map[string]bool{}
	if !IsMongos(ctx, c) {
		return keys, uniq, nil
	}
	cur, err := c.Database("config").Collection("collections").Find(ctx, bson.D{{Key: "dropped", Value: bson.D{{Key: "$ne", Value: true}}}})
	if err != nil {
		return keys, uniq, apperr.From(err)
	}
	defer cur.Close(ctx)
	want := map[string]bool{}
	for _, d := range dbs {
		want[d] = true
	}
	for cur.Next(ctx) {
		var doc bson.D
		if err := cur.Decode(&doc); err != nil {
			return keys, uniq, apperr.From(err)
		}
		idv, _ := mql.Lookup(doc, "_id")
		ns, _ := idv.(string)
		db, _, ok := strings.Cut(ns, ".")
		if !ok || db == "config" || (len(want) > 0 && !want[db]) {
			continue
		}
		k, _ := mql.Lookup(doc, "key")
		if kd, ok := k.(bson.D); ok {
			keys[ns] = kd
			if u, ok := mql.Lookup(doc, "unique"); ok {
				uniq[ns], _ = u.(bool)
			}
		}
	}
	return keys, uniq, errOrNil(cur.Err())
}

// CollectionShardingInfo reads the shard key and chunk distribution of
// db.coll; nil when the collection isn't sharded.
func CollectionShardingInfo(ctx context.Context, c *mongo.Client, db, coll string) (*CollectionSharding, error) {
	key, unique, uuid, noBalance, err := ShardKeyOf(ctx, c, db, coll)
	if err != nil || key == nil {
		return nil, err
	}
	out := &CollectionSharding{Key: mql.Compact(key), Unique: unique, Balancing: !noBalance, ByShard: []ShardChunks{}}
	// config.chunks refers to the collection by UUID since MongoDB 5.0, by namespace before.
	match := bson.D{{Key: "ns", Value: db + "." + coll}}
	if uuid != nil {
		match = bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "uuid", Value: uuid}}, match}}}
	}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$shard"},
			{Key: "chunks", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "jumbo", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$jumbo", true}}}, 1, 0}}}}}},
		}}},
	}
	cur, err := c.Database("config").Collection("chunks").Aggregate(ctx, pipe)
	if err != nil {
		return out, apperr.From(err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var row struct {
			Shard  string `bson:"_id"`
			Chunks int64  `bson:"chunks"`
			Jumbo  int64  `bson:"jumbo"`
		}
		if err := cur.Decode(&row); err != nil {
			return out, apperr.From(err)
		}
		out.ByShard = append(out.ByShard, ShardChunks{Shard: row.Shard, Chunks: row.Chunks, Jumbo: row.Jumbo})
		out.Chunks += row.Chunks
		out.Jumbo += row.Jumbo
	}
	sort.Slice(out.ByShard, func(i, j int) bool { return out.ByShard[i].Shard < out.ByShard[j].Shard })
	return out, errOrNil(cur.Err())
}

// ListShards lists the cluster's shards.
func ListShards(ctx context.Context, c *mongo.Client) ([]Shard, error) {
	var res struct {
		Shards []struct {
			ID       string `bson:"_id"`
			Host     string `bson:"host"`
			State    int64  `bson:"state"`
			Draining bool   `bson:"draining"`
		} `bson:"shards"`
	}
	if err := c.Database("admin").RunCommand(ctx, bson.D{{Key: "listShards", Value: 1}}).Decode(&res); err != nil {
		return nil, apperr.From(err)
	}
	out := make([]Shard, 0, len(res.Shards))
	for _, s := range res.Shards {
		out = append(out, Shard{ID: s.ID, Host: s.Host, State: s.State, Draining: s.Draining})
	}
	return out, nil
}

// DatabaseShardingInfo reads a database's primary shard, its sharded
// collections, the shards and the balancer; nil when not on a mongos.
func DatabaseShardingInfo(ctx context.Context, c *mongo.Client, db string) (*DatabaseSharding, error) {
	if !IsMongos(ctx, c) {
		return nil, nil
	}
	out := &DatabaseSharding{Sharded: []string{}}
	var dbDoc bson.D
	if err := c.Database("config").Collection("databases").FindOne(ctx, bson.D{{Key: "_id", Value: db}}).Decode(&dbDoc); err == nil {
		if p, ok := mql.Lookup(dbDoc, "primary"); ok {
			out.Primary, _ = p.(string)
		}
	}
	keys, _, err := ShardKeys(ctx, c, []string{db})
	if err != nil {
		return nil, err
	}
	for ns := range keys {
		out.Sharded = append(out.Sharded, strings.TrimPrefix(ns, db+"."))
	}
	sort.Strings(out.Sharded)
	if out.Shards, err = ListShards(ctx, c); err != nil {
		return nil, err
	}
	var bs bson.D
	if err := c.Database("admin").RunCommand(ctx, bson.D{{Key: "balancerStatus", Value: 1}}).Decode(&bs); err != nil {
		out.Balancer.Problem = apperr.From(err).Message
	} else {
		if m, ok := mql.Lookup(bs, "mode"); ok {
			out.Balancer.Mode, _ = m.(string)
		}
		if r, ok := mql.Lookup(bs, "inBalancerRound"); ok {
			out.Balancer.InRound, _ = r.(bool)
		}
		if n, ok := mql.Lookup(bs, "numBalancerRounds"); ok {
			out.Balancer.Rounds = toInt64(n)
		}
	}
	return out, nil
}

// ShardCollection shards db.coll on key (the collection should be empty or
// already have an index on the key).
func ShardCollection(ctx context.Context, c *mongo.Client, db, coll string, key bson.D, unique bool) error {
	cmd := bson.D{{Key: "shardCollection", Value: db + "." + coll}, {Key: "key", Value: key}}
	if unique {
		cmd = append(cmd, bson.E{Key: "unique", Value: true})
	}
	return errOrNil(c.Database("admin").RunCommand(ctx, cmd).Err())
}

// errOrNil converts a driver error, keeping nil as a plain nil (apperr.From
// returns a typed nil pointer, which isn't == nil as an error).
func errOrNil(err error) error {
	if err == nil {
		return nil
	}
	return apperr.From(err)
}
