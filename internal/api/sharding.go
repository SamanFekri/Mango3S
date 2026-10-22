package api

import (
	"context"
	"strings"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/manage"
	"mango3s/internal/mongox"
)

// Sharded clusters: the shard list, and connecting straight to one shard
// (profiling and some statistics only exist per shard, not on the mongos).

// ListShards lists the shards of the cluster connID is connected to.
func (a *App) ListShards(connID string) ([]manage.Shard, error) {
	return call(a, "ListShards", "", func(ctx context.Context) ([]manage.Shard, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return nil, err
		}
		if s.Topology != "sharded" {
			return nil, apperr.Invalid("%s isn't a sharded cluster.", s.Name)
		}
		return manage.ListShards(ctx, s.Client)
	})
}

// ConnectToShard opens a connection to one shard of connID's cluster with the
// same saved credentials, TLS, SSH and proxy settings. The connection isn't
// saved; it appears in the explorer until it's closed.
func (a *App) ConnectToShard(connID, shardID string) (mongox.SessionInfo, error) {
	return call(a, "ConnectToShard", "connect-shard:"+connID+":"+shardID, func(ctx context.Context) (mongox.SessionInfo, error) {
		s, err := a.conns.Session(connID)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		shards, err := manage.ListShards(ctx, s.Client)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		var host string
		for _, sh := range shards {
			if sh.ID == shardID {
				host = sh.Host
			}
		}
		if host == "" {
			return mongox.SessionInfo{}, apperr.New(apperr.CodeNotFound, "The cluster has no shard %q.", shardID)
		}
		p, err := a.store.GetProfile(s.ProfileID)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		creds, err := a.credentials(p, nil)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		uri, err := config.ShardURI(p.URI, host)
		if err != nil {
			return mongox.SessionInfo{}, err
		}
		shard := p
		shard.ID = shardSessionID(s.ProfileID, shardID)
		shard.Name = p.Name + " · " + shardID
		shard.URI = uri
		shard.Server = config.ServerConfig{Mode: config.ServerModeURI}
		if _, err := a.conns.Connect(ctx, shard, creds); err != nil {
			return mongox.SessionInfo{}, err
		}
		for _, info := range a.conns.List() {
			if info.ID == shard.ID {
				return info, nil
			}
		}
		return mongox.SessionInfo{}, apperr.New(apperr.CodeInternal, "The shard connection closed right away.")
	})
}

func shardSessionID(profileID, shardID string) string {
	return profileID + "~shard~" + strings.ReplaceAll(shardID, "~", "-")
}
