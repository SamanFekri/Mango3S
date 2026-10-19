package manage

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

// Profiling is a database's profiler setting.
type Profiling struct {
	Level      int     `json:"level"`  // 0 off, 1 slow operations, 2 everything
	SlowMS     int     `json:"slowms"` // threshold for level 1 (server-wide)
	SampleRate float64 `json:"sampleRate"`
}

// GetProfiling reads the profiler level of a database.
func GetProfiling(ctx context.Context, db *mongo.Database) (Profiling, error) {
	var r struct {
		Was        int32   `bson:"was"`
		SlowMS     int32   `bson:"slowms"`
		SampleRate float64 `bson:"sampleRate"`
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "profile", Value: -1}}).Decode(&r); err != nil {
		return Profiling{}, profilerErr(err)
	}
	return Profiling{Level: int(r.Was), SlowMS: int(r.SlowMS), SampleRate: r.SampleRate}, nil
}

// SetProfiling changes the profiler level. slowms applies to the whole
// server (and to its log), as in mongosh's db.setProfilingLevel.
func SetProfiling(ctx context.Context, db *mongo.Database, p Profiling) error {
	if p.Level < 0 || p.Level > 2 {
		return apperr.Invalid("The profiling level must be 0, 1 or 2.")
	}
	if p.SlowMS < 0 {
		return apperr.Invalid("The slow operation threshold must be 0 ms or more.")
	}
	if p.SampleRate < 0 || p.SampleRate > 1 {
		return apperr.Invalid("The sample rate must be between 0 and 1.")
	}
	cmd := bson.D{{Key: "profile", Value: int32(p.Level)}}
	if p.SlowMS > 0 {
		cmd = append(cmd, bson.E{Key: "slowms", Value: int32(p.SlowMS)})
	}
	if p.SampleRate > 0 {
		cmd = append(cmd, bson.E{Key: "sampleRate", Value: p.SampleRate})
	}
	return profilerErr(db.RunCommand(ctx, cmd).Err())
}

func profilerErr(err error) error {
	if err == nil {
		return nil
	}
	e := apperr.From(err)
	if strings.Contains(strings.ToLower(e.Message), "mongos") || strings.Contains(e.Message, "CommandNotSupported") {
		return apperr.New(apperr.CodeUnsupported, "The profiler runs on each shard's mongod, not through mongos. Connect to a shard directly to profile it.")
	}
	return err
}

// ProfileQuery filters system.profile.
type ProfileQuery struct {
	Coll      string `json:"coll"`      // empty: every collection
	Op        string `json:"op"`        // query | update | remove | insert | command | getmore; empty: all
	MinMillis int    `json:"minMillis"` // only operations at least this slow
	Limit     int    `json:"limit"`     // newest first; default 100, at most 1000
}

// ProfileEntry is one profiled operation.
type ProfileEntry struct {
	TS           time.Time `json:"ts"`
	Op           string    `json:"op"`
	NS           string    `json:"ns"`
	Millis       int64     `json:"millis"`
	PlanSummary  string    `json:"planSummary"`
	KeysExamined int64     `json:"keysExamined"`
	DocsExamined int64     `json:"docsExamined"`
	NReturned    int64     `json:"nreturned"`
	Command      string    `json:"command"` // mongosh syntax, shortened
	Client       string    `json:"client"`
	User         string    `json:"user"`
	AppName      string    `json:"appName"`
}

const maxCommandText = 2000

// ListProfile reads profiled operations, newest first.
func ListProfile(ctx context.Context, db *mongo.Database, q ProfileQuery) ([]ProfileEntry, error) {
	filter := bson.D{}
	if q.Coll != "" {
		filter = append(filter, bson.E{Key: "ns", Value: db.Name() + "." + q.Coll})
	}
	if q.Op != "" {
		filter = append(filter, bson.E{Key: "op", Value: q.Op})
	}
	if q.MinMillis > 0 {
		filter = append(filter, bson.E{Key: "millis", Value: bson.D{{Key: "$gte", Value: q.MinMillis}}})
	}
	limit := int64(q.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	cur, err := db.Collection("system.profile").Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(limit))
	if err != nil {
		return nil, apperr.From(err)
	}
	defer cur.Close(context.Background())
	out := []ProfileEntry{}
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return nil, apperr.From(err)
		}
		e := ProfileEntry{}
		for _, f := range d {
			switch f.Key {
			case "ts":
				if t, ok := f.Value.(bson.DateTime); ok {
					e.TS = t.Time().UTC()
				}
			case "op":
				e.Op, _ = f.Value.(string)
			case "ns":
				e.NS, _ = f.Value.(string)
			case "millis":
				e.Millis = toInt64(f.Value)
			case "planSummary":
				e.PlanSummary, _ = f.Value.(string)
			case "keysExamined":
				e.KeysExamined = toInt64(f.Value)
			case "docsExamined":
				e.DocsExamined = toInt64(f.Value)
			case "nreturned":
				e.NReturned = toInt64(f.Value)
			case "command":
				e.Command = mql.Compact(f.Value)
				if len(e.Command) > maxCommandText {
					e.Command = e.Command[:maxCommandText] + "…"
				}
			case "client":
				e.Client, _ = f.Value.(string)
			case "user":
				e.User, _ = f.Value.(string)
			case "appName":
				e.AppName, _ = f.Value.(string)
			}
		}
		out = append(out, e)
	}
	return out, apperr.Check(cur.Err())
}

// ClearProfile empties system.profile, which requires the profiler to be off
// for the moment of the drop; the previous level is restored afterwards.
func ClearProfile(ctx context.Context, db *mongo.Database) error {
	prev, err := GetProfiling(ctx, db)
	if err != nil {
		return err
	}
	if prev.Level != 0 {
		if err := SetProfiling(ctx, db, Profiling{Level: 0}); err != nil {
			return err
		}
	}
	dropErr := apperr.Check(db.Collection("system.profile").Drop(ctx))
	if prev.Level != 0 {
		if err := SetProfiling(ctx, db, Profiling{Level: prev.Level, SlowMS: prev.SlowMS, SampleRate: prev.SampleRate}); err != nil {
			return err
		}
	}
	return dropErr
}
