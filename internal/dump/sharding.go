package dump

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/manage"
	"mango3s/internal/mql"
)

// recordShardKeys stores each sharded collection's shard key in its
// namespace (best effort: a backup never fails because of it).
func recordShardKeys(ctx context.Context, cl *mongo.Client, nss []Namespace, log Logger) {
	dbs := map[string]bool{}
	var list []string
	for _, ns := range nss {
		if !dbs[ns.DB] {
			dbs[ns.DB] = true
			list = append(list, ns.DB)
		}
	}
	keys, uniq, err := manage.ShardKeys(ctx, cl, list)
	if err != nil {
		log.Log("Shard keys could not be read (%v); the backup won't record them.", err)
		return
	}
	for i := range nss {
		ns := nss[i].DB + "." + nss[i].Coll
		if k, ok := keys[ns]; ok {
			nss[i].ShardKey, nss[i].ShardUnique = mql.Compact(k), uniq[ns]
		}
	}
	if len(keys) > 0 {
		log.Log("Recorded the shard keys of %d sharded collections.", len(keys))
	}
}

// shardTarget shards db.coll on key before data is loaded into it. Without a
// sharded cluster as the target it does nothing and says so.
func shardTarget(ctx context.Context, cl *mongo.Client, db, coll, key string, unique bool, log Logger) error {
	if !manage.IsMongos(ctx, cl) {
		log.Log("%s.%s was sharded on %s, but the target isn't a sharded cluster: restored unsharded.", db, coll, key)
		return nil
	}
	if existing, _, _, _, err := manage.ShardKeyOf(ctx, cl, db, coll); err == nil && existing != nil {
		log.Log("%s.%s is already sharded on %s.", db, coll, mql.Compact(existing))
		return nil
	}
	k, err := mql.ParseDocument(key)
	if err != nil {
		return fmt.Errorf("shard key %s can't be read: %w", key, err)
	}
	if err := manage.ShardCollection(ctx, cl, db, coll, k, unique); err != nil {
		return fmt.Errorf("shardCollection %s.%s: %w", db, coll, err)
	}
	log.Log("Sharded %s.%s on %s.", db, coll, key)
	return nil
}
