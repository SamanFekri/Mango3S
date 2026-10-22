// Package monitor samples a server's activity for the live Monitor tab:
// serverStatus counters turned into per-second rates, replica set member
// state and lag, the oplog window, and the operations running right now.
package monitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

// History is how much the sampler keeps in memory.
const History = 15 * time.Minute

// Sample is one point of the live charts. Rates are per second since the
// previous sample; the first sample of a run has none.
type Sample struct {
	Time       time.Time `json:"time"`
	Insert     float64   `json:"insert"`
	Query      float64   `json:"query"`
	Update     float64   `json:"update"`
	Delete     float64   `json:"delete"`
	GetMore    float64   `json:"getmore"`
	Command    float64   `json:"command"`
	NetInBps   float64   `json:"netIn"`
	NetOutBps  float64   `json:"netOut"`
	Requests   float64   `json:"requests"`
	ConnsCur   int64     `json:"connections"`
	ConnsAvail int64     `json:"connectionsAvailable"`
	QueueRead  int64     `json:"queueReaders"`
	QueueWrite int64     `json:"queueWriters"`
	ActiveRead int64     `json:"activeReaders"`
	ActiveWrt  int64     `json:"activeWriters"`
	CacheBytes int64     `json:"cacheBytes"`
	CacheMax   int64     `json:"cacheMax"`
	CacheDirty int64     `json:"cacheDirty"`
	ResidentMB int64     `json:"residentMb"`
	First      bool      `json:"first"` // no rates yet
}

// Counters are the raw cumulative values a rate is computed from.
type Counters struct {
	At                                           time.Time
	Insert, Query, Update, Delete, GetMore, Cmd  int64
	BytesIn, BytesOut, Requests                  int64
	ConnsCur, ConnsAvail                         int64
	QueueRead, QueueWrite, ActiveRead, ActiveWrt int64
	CacheBytes, CacheMax, CacheDirty, ResidentMB int64
}

// Rates turns two consecutive counters into a sample. Counters that went
// backwards (a server restart) give 0 rather than a negative rate.
func Rates(prev *Counters, cur Counters) Sample {
	s := Sample{
		Time: cur.At, ConnsCur: cur.ConnsCur, ConnsAvail: cur.ConnsAvail,
		QueueRead: cur.QueueRead, QueueWrite: cur.QueueWrite, ActiveRead: cur.ActiveRead, ActiveWrt: cur.ActiveWrt,
		CacheBytes: cur.CacheBytes, CacheMax: cur.CacheMax, CacheDirty: cur.CacheDirty, ResidentMB: cur.ResidentMB,
	}
	if prev == nil {
		s.First = true
		return s
	}
	dt := cur.At.Sub(prev.At).Seconds()
	if dt <= 0 {
		s.First = true
		return s
	}
	rate := func(a, b int64) float64 {
		if b < a {
			return 0
		}
		return float64(b-a) / dt
	}
	s.Insert, s.Query, s.Update = rate(prev.Insert, cur.Insert), rate(prev.Query, cur.Query), rate(prev.Update, cur.Update)
	s.Delete, s.GetMore, s.Command = rate(prev.Delete, cur.Delete), rate(prev.GetMore, cur.GetMore), rate(prev.Cmd, cur.Cmd)
	s.NetInBps, s.NetOutBps, s.Requests = rate(prev.BytesIn, cur.BytesIn), rate(prev.BytesOut, cur.BytesOut), rate(prev.Requests, cur.Requests)
	return s
}

// CountersFrom reads the values Rates needs from a serverStatus reply.
func CountersFrom(status bson.D, at time.Time) Counters {
	n := func(path ...string) int64 { return number(lookup(status, path...)) }
	return Counters{
		At:     at,
		Insert: n("opcounters", "insert"), Query: n("opcounters", "query"), Update: n("opcounters", "update"),
		Delete: n("opcounters", "delete"), GetMore: n("opcounters", "getmore"), Cmd: n("opcounters", "command"),
		BytesIn: n("network", "bytesIn"), BytesOut: n("network", "bytesOut"), Requests: n("network", "numRequests"),
		ConnsCur: n("connections", "current"), ConnsAvail: n("connections", "available"),
		QueueRead: n("globalLock", "currentQueue", "readers"), QueueWrite: n("globalLock", "currentQueue", "writers"),
		ActiveRead: n("globalLock", "activeClients", "readers"), ActiveWrt: n("globalLock", "activeClients", "writers"),
		CacheBytes: n("wiredTiger", "cache", "bytes currently in the cache"),
		CacheMax:   n("wiredTiger", "cache", "maximum bytes configured"),
		CacheDirty: n("wiredTiger", "cache", "tracked dirty bytes in the cache"),
		ResidentMB: n("mem", "resident"),
	}
}

func lookup(d bson.D, path ...string) any {
	var cur any = d
	for _, p := range path {
		doc, ok := cur.(bson.D)
		if !ok {
			return nil
		}
		v, ok := mql.Lookup(doc, p)
		if !ok {
			return nil
		}
		cur = v
	}
	return cur
}

func number(v any) int64 {
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case bson.Decimal128:
		f, err := mql.ParseValue(x.String())
		if err == nil {
			return number(f)
		}
	}
	return 0
}

// ---- server info and replication -----------------------------------------

// Member is one replica set member.
type Member struct {
	Name     string     `json:"name"`
	State    string     `json:"state"` // PRIMARY, SECONDARY, ARBITER…
	Health   bool       `json:"health"`
	Self     bool       `json:"self"`
	Optime   *time.Time `json:"optime,omitempty"`
	LagSecs  *float64   `json:"lagSeconds,omitempty"` // behind the primary
	PingMs   *int64     `json:"pingMs,omitempty"`
	Uptime   int64      `json:"uptime"`
	SyncFrom string     `json:"syncSource,omitempty"`
}

// Replication describes the replica set, when there is one.
type Replication struct {
	Set         string   `json:"set"`
	Members     []Member `json:"members"`
	OplogHours  *float64 `json:"oplogHours,omitempty"` // time between the first and last oplog entry
	OplogSizeMB *float64 `json:"oplogSizeMb,omitempty"`
	Problem     string   `json:"problem,omitempty"` // why some of it couldn't be read
}

// Overview is the slow-changing part of the Monitor tab.
type Overview struct {
	Host        string       `json:"host"`
	Version     string       `json:"version"`
	Process     string       `json:"process"` // mongod | mongos
	UptimeSecs  int64        `json:"uptime"`
	StorageEng  string       `json:"storageEngine"`
	Replication *Replication `json:"replication,omitempty"`
}

// ReadOverview reads the server description and, on a replica set, member
// state, lag and the oplog window.
func ReadOverview(ctx context.Context, c *mongo.Client) (Overview, error) {
	status, err := serverStatus(ctx, c)
	if err != nil {
		return Overview{}, err
	}
	ov := Overview{
		Host: str(lookup(status, "host")), Version: str(lookup(status, "version")), Process: str(lookup(status, "process")),
		UptimeSecs: number(lookup(status, "uptime")), StorageEng: str(lookup(status, "storageEngine", "name")),
	}
	if strings.Contains(ov.Process, "mongos") {
		return ov, nil
	}
	var rs bson.D
	err = c.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&rs)
	if err != nil {
		if isNotReplSet(err) {
			return ov, nil
		}
		ov.Replication = &Replication{Problem: privilegeHint(err, "replSetGetStatus")}
		return ov, nil
	}
	ov.Replication = replicationFrom(rs)
	if h, size, err := oplogWindow(ctx, c); err == nil {
		ov.Replication.OplogHours, ov.Replication.OplogSizeMB = h, size
	} else if ov.Replication.Problem == "" {
		ov.Replication.Problem = privilegeHint(err, "reading local.oplog.rs")
	}
	return ov, nil
}

func replicationFrom(rs bson.D) *Replication {
	r := &Replication{Set: str(lookup(rs, "set"))}
	var primary *time.Time
	arr, _ := lookup(rs, "members").(bson.A)
	for _, el := range arr {
		m, _ := el.(bson.D)
		mem := Member{
			Name: str(lookup(m, "name")), State: str(lookup(m, "stateStr")), Health: number(lookup(m, "health")) == 1,
			Uptime: number(lookup(m, "uptime")), SyncFrom: str(lookup(m, "syncSourceHost")),
		}
		if self, ok := lookup(m, "self").(bool); ok {
			mem.Self = self
		}
		if t, ok := lookup(m, "optimeDate").(bson.DateTime); ok {
			tt := t.Time().UTC()
			mem.Optime = &tt
			if mem.State == "PRIMARY" {
				primary = &tt
			}
		}
		if _, ok := lookup(m, "pingMs").(int64); ok {
			p := number(lookup(m, "pingMs"))
			mem.PingMs = &p
		}
		r.Members = append(r.Members, mem)
	}
	if primary != nil {
		for i := range r.Members {
			m := &r.Members[i]
			if m.Optime != nil && m.State == "SECONDARY" {
				lag := primary.Sub(*m.Optime).Seconds()
				if lag < 0 {
					lag = 0
				}
				m.LagSecs = &lag
			} else if m.State == "PRIMARY" {
				zero := 0.0
				m.LagSecs = &zero
			}
		}
	}
	return r
}

func oplogWindow(ctx context.Context, c *mongo.Client) (*float64, *float64, error) {
	coll := c.Database("local").Collection("oplog.rs")
	first, last := bson.D{}, bson.D{}
	if err := coll.FindOne(ctx, bson.D{}, optsSortNatural(1)).Decode(&first); err != nil {
		return nil, nil, err
	}
	if err := coll.FindOne(ctx, bson.D{}, optsSortNatural(-1)).Decode(&last); err != nil {
		return nil, nil, err
	}
	t1, ok1 := lookup(first, "ts").(bson.Timestamp)
	t2, ok2 := lookup(last, "ts").(bson.Timestamp)
	if !ok1 || !ok2 {
		return nil, nil, errors.New("the oplog has no timestamps")
	}
	h := float64(t2.T-t1.T) / 3600
	var stats bson.D
	var size *float64
	if err := c.Database("local").RunCommand(ctx, bson.D{{Key: "collStats", Value: "oplog.rs"}}).Decode(&stats); err == nil {
		mb := float64(number(lookup(stats, "maxSize"))) / (1 << 20)
		size = &mb
	}
	return &h, size, nil
}

func serverStatus(ctx context.Context, c *mongo.Client) (bson.D, error) {
	var status bson.D
	err := c.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&status)
	if err != nil {
		return nil, apperr.New(apperr.CodeMongo, "%s", privilegeHint(err, "serverStatus"))
	}
	return status, nil
}

func isNotReplSet(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return ce.Code == 76 || ce.Name == "NoReplicationEnabled" || strings.Contains(ce.Message, "not running with --replSet")
	}
	return strings.Contains(err.Error(), "not running with --replSet")
}

// privilegeHint explains a refusal instead of passing the raw error on.
func privilegeHint(err error, what string) string {
	var ce mongo.CommandError
	if errors.As(err, &ce) && (ce.Code == 13 || ce.Name == "Unauthorized") {
		return fmt.Sprintf("This user isn't allowed to run %s. The clusterMonitor role (or a role with the serverStatus, replSetGetStatus and inprog privileges) gives access.", what)
	}
	return fmt.Sprintf("%s failed: %v", what, apperr.From(err).Message)
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---- the sampler ------------------------------------------------------------

// Emit receives each new sample.
type Emit func(connID string, s Sample)

// Sampler runs serverStatus on one connection at a fixed interval.
type sampler struct {
	cancel  context.CancelFunc
	samples []Sample
	err     string
	every   time.Duration
}

// Service keeps one sampler per connection while its Monitor tab is open.
type Service struct {
	src  mongox.Source
	emit Emit
	mu   sync.Mutex
	runs map[string]*sampler
}

// New returns a monitor service; emit may be nil.
func New(src mongox.Source, emit Emit) *Service {
	return &Service{src: src, emit: emit, runs: map[string]*sampler{}}
}

// SetEmit installs the event emitter (the desktop app does at startup).
func (s *Service) SetEmit(e Emit) {
	s.mu.Lock()
	s.emit = e
	s.mu.Unlock()
}

// Snapshot is what a newly opened Monitor tab starts from.
type Snapshot struct {
	IntervalSecs int      `json:"intervalSeconds"`
	Samples      []Sample `json:"samples"`
	Error        string   `json:"error,omitempty"`
}

// Start begins sampling connID every interval (1–5 s) and returns the
// history so far. Starting a running sampler only changes its interval.
func (s *Service) Start(connID string, interval time.Duration) (Snapshot, error) {
	if interval < time.Second {
		interval = time.Second
	}
	if interval > 5*time.Second {
		interval = 5 * time.Second
	}
	sess, err := s.src.Session(connID)
	if err != nil {
		return Snapshot{}, err
	}
	// Fail fast when serverStatus isn't allowed, with the explanation.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = serverStatus(ctx, sess.Client)
	cancel()
	if err != nil {
		return Snapshot{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.runs[connID]; ok {
		if r.every != interval {
			r.cancel()
			r.every = interval
			s.startLocked(connID, sess.Client, r)
		}
		return Snapshot{IntervalSecs: int(interval / time.Second), Samples: append([]Sample{}, r.samples...), Error: r.err}, nil
	}
	r := &sampler{every: interval}
	s.runs[connID] = r
	s.startLocked(connID, sess.Client, r)
	return Snapshot{IntervalSecs: int(interval / time.Second), Samples: []Sample{}}, nil
}

func (s *Service) startLocked(connID string, client *mongo.Client, r *sampler) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	every := r.every
	go func() {
		var prev *Counters
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			cctx, ccancel := context.WithTimeout(ctx, every+5*time.Second)
			status, err := serverStatus(cctx, client)
			ccancel()
			if ctx.Err() != nil {
				return
			}
			s.mu.Lock()
			if err != nil {
				r.err = apperr.From(err).Message
				prev = nil
			} else {
				r.err = ""
				cur := CountersFrom(status, time.Now())
				smp := Rates(prev, cur)
				prev = &cur
				r.samples = append(r.samples, smp)
				keep := int(History / every)
				if len(r.samples) > keep {
					r.samples = append([]Sample{}, r.samples[len(r.samples)-keep:]...)
				}
				if s.emit != nil {
					emit := s.emit
					s.mu.Unlock()
					emit(connID, smp)
					s.mu.Lock()
				}
			}
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// Stop ends sampling for connID and forgets its history.
func (s *Service) Stop(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.runs[connID]; ok {
		r.cancel()
		delete(s.runs, connID)
	}
}

// StopAll ends every sampler (shutdown, disconnect).
func (s *Service) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, r := range s.runs {
		r.cancel()
		delete(s.runs, id)
	}
}

// ---- running operations --------------------------------------------------

// Op is one running operation from $currentOp.
type Op struct {
	OpID        string  `json:"opid"` // a number on mongod, "shard:number" on mongos
	Op          string  `json:"op"`
	NS          string  `json:"ns"`
	Secs        float64 `json:"seconds"`
	Client      string  `json:"client"`
	AppName     string  `json:"appName"`
	User        string  `json:"user"`
	Desc        string  `json:"desc"`
	Command     string  `json:"command"`
	PlanSummary string  `json:"planSummary"`
	WaitingLock bool    `json:"waitingForLock"`
	Active      bool    `json:"active"`
	Shard       string  `json:"shard,omitempty"`
	killable    any
}

// OpsFilter narrows the list.
type OpsFilter struct {
	MinSeconds float64 `json:"minSeconds"`
	NS         string  `json:"ns"`     // substring
	Client     string  `json:"client"` // substring of client address or appName
	All        bool    `json:"all"`    // include idle connections and system operations
}

// CurrentOps lists running operations, longest first.
func (s *Service) CurrentOps(ctx context.Context, connID string, f OpsFilter) ([]Op, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return nil, err
	}
	stage := bson.D{{Key: "allUsers", Value: true}, {Key: "idleConnections", Value: f.All}}
	pipe := mongo.Pipeline{{{Key: "$currentOp", Value: stage}}}
	cur, err := sess.Client.Database("admin").Aggregate(ctx, pipe)
	if err != nil {
		return nil, apperr.New(apperr.CodeMongo, "%s", privilegeHint(err, "$currentOp (inprog)"))
	}
	defer cur.Close(ctx)
	var out []Op
	for cur.Next(ctx) {
		var d bson.D
		if err := cur.Decode(&d); err != nil {
			return nil, apperr.From(err)
		}
		op := opFrom(d)
		if !f.All && (op.Desc == "conn" || strings.HasPrefix(op.NS, "admin.$cmd") && op.Op == "none") {
			continue
		}
		if op.Secs < f.MinSeconds {
			continue
		}
		if f.NS != "" && !strings.Contains(strings.ToLower(op.NS), strings.ToLower(f.NS)) {
			continue
		}
		if f.Client != "" {
			c := strings.ToLower(f.Client)
			if !strings.Contains(strings.ToLower(op.Client), c) && !strings.Contains(strings.ToLower(op.AppName), c) {
				continue
			}
		}
		out = append(out, op)
		if len(out) >= 1000 {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Secs > out[j].Secs })
	if out == nil {
		out = []Op{}
	}
	if err := cur.Err(); err != nil {
		return nil, apperr.From(err)
	}
	return out, nil
}

func opFrom(d bson.D) Op {
	op := Op{
		Op: str(lookup(d, "op")), NS: str(lookup(d, "ns")), Client: str(lookup(d, "client")),
		AppName: str(lookup(d, "appName")), Desc: str(lookup(d, "desc")), PlanSummary: str(lookup(d, "planSummary")),
		Shard: str(lookup(d, "shard")),
	}
	if op.Client == "" {
		op.Client = str(lookup(d, "client_s"))
	}
	if us := number(lookup(d, "microsecs_running")); us > 0 {
		op.Secs = float64(us) / 1e6
	} else {
		op.Secs = float64(number(lookup(d, "secs_running")))
	}
	if b, ok := lookup(d, "waitingForLock").(bool); ok {
		op.WaitingLock = b
	}
	if b, ok := lookup(d, "active").(bool); ok {
		op.Active = b
	}
	if users, ok := lookup(d, "effectiveUsers").(bson.A); ok && len(users) > 0 {
		if u, ok := users[0].(bson.D); ok {
			op.User = str(lookup(u, "user")) + "@" + str(lookup(u, "db"))
		}
	}
	if cmd, ok := lookup(d, "command").(bson.D); ok {
		op.Command = clip(mql.Compact(cmd), 2000)
	}
	switch v := lookup(d, "opid").(type) {
	case string:
		op.OpID = v
	default:
		op.OpID = fmt.Sprint(number(v))
	}
	return op
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// KillOp stops operation opid. It refuses read-only connections, and confirm
// must repeat the operation ID.
func (s *Service) KillOp(ctx context.Context, connID, opid, confirm string) error {
	sess, err := s.src.Session(connID)
	if err != nil {
		return err
	}
	if err := sess.RequireWritable(); err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != opid {
		return apperr.Invalid("Type the operation ID (%s) to confirm.", opid)
	}
	var id any = opid
	if !strings.Contains(opid, ":") { // mongod: a number
		var n int64
		if _, err := fmt.Sscan(opid, &n); err != nil {
			return apperr.Invalid("%q is not an operation ID.", opid)
		}
		if n >= -(1<<31) && n < 1<<31 {
			id = int32(n)
		} else {
			id = n
		}
	}
	err = sess.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "killOp", Value: 1}, {Key: "op", Value: id}}).Err()
	if err != nil {
		return apperr.New(apperr.CodeMongo, "%s", privilegeHint(err, "killOp"))
	}
	return nil
}

func optsSortNatural(dir int) *options.FindOneOptionsBuilder {
	return options.FindOne().SetSort(bson.D{{Key: "$natural", Value: dir}})
}
