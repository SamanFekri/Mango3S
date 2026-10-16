package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/aggregate"
	"mango3s/internal/apperr"
	"mango3s/internal/backup"
	"mango3s/internal/config"
	"mango3s/internal/masking"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
	"mango3s/internal/schema"
)

// Deps are the server's collaborators.
type Deps struct {
	Policy    Policy
	Profiles  func() ([]config.Profile, error)
	Connect   func(ctx context.Context, profileID string) (*mongox.Session, error) // connects lazily with stored credentials
	Masker    *masking.Masker                                                      // nil = no masking
	BackupDir string
	Log       *slog.Logger
	Version   string
}

// Server wraps the MCP server with Mango3S policy.
type Server struct {
	d   Deps
	mcp *mcp.Server
}

// New builds a server that registers only the allowed tools.
func New(d Deps) *Server {
	d.Policy = d.Policy.Normalize()
	s := &Server{d: d, mcp: mcp.NewServer(&mcp.Implementation{Name: "mango3s", Version: d.Version}, nil)}
	s.register()
	return s
}

// MCP returns the underlying server (tests connect over in-memory transports).
func (s *Server) MCP() *mcp.Server { return s.mcp }

// Run serves on stdio until the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	if !s.d.Policy.Enabled {
		return apperr.New(apperr.CodeReadOnly, "The MCP server is disabled. Enable it in Mango3S → Settings → MCP.")
	}
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

// Inputs. Field descriptions become the JSON schema the client sees.
type connIn struct {
	Connection string `json:"connection" jsonschema:"connection profile name or id (see list_connections)"`
}

type dbIn struct {
	Connection string `json:"connection" jsonschema:"connection profile name or id"`
	Database   string `json:"database" jsonschema:"database name"`
}

type collIn struct {
	Connection string `json:"connection" jsonschema:"connection profile name or id"`
	Database   string `json:"database" jsonschema:"database name"`
	Collection string `json:"collection" jsonschema:"collection name"`
}

type findIn struct {
	collIn
	Filter     string `json:"filter,omitempty" jsonschema:"filter document in MongoDB shell syntax, e.g. { status: 'A' }"`
	Projection string `json:"projection,omitempty" jsonschema:"projection document"`
	Sort       string `json:"sort,omitempty" jsonschema:"sort document, e.g. { createdAt: -1 }"`
	Limit      int64  `json:"limit,omitempty" jsonschema:"maximum documents to return (capped by policy)"`
}

type aggIn struct {
	collIn
	Pipeline string `json:"pipeline" jsonschema:"pipeline array in shell syntax, e.g. [{ $match: {} }, { $group: { _id: '$x', n: { $sum: 1 } } }]"`
}

type insertIn struct {
	collIn
	Document string `json:"document" jsonschema:"document in shell syntax"`
}

type updateIn struct {
	collIn
	Filter string `json:"filter" jsonschema:"filter document; required"`
	Update string `json:"update" jsonschema:"update document with operators such as $set, or an update pipeline"`
}

type deleteIn struct {
	collIn
	Filter string `json:"filter" jsonschema:"filter document; required and must not be empty"`
}

func textResult(v any) *mcp.CallToolResult {
	var text string
	switch t := v.(type) {
	case string:
		text = t
	default:
		b, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			text = fmt.Sprint(t)
		} else {
			text = string(b)
		}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(err error) *mcp.CallToolResult {
	r := textResult(apperr.From(err).Message)
	r.IsError = true
	return r
}

// add registers a tool only if the policy allows it, wrapping the handler
// with the policy re-check and the audit log.
func add[In any](s *Server, name string, h func(ctx context.Context, in In) (any, error)) {
	if s.d.Policy.CheckTool(name) != nil {
		return
	}
	info, _ := toolInfo(name)
	mcp.AddTool(s.mcp, &mcp.Tool{Name: name, Description: info.Description}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		start := time.Now()
		var out any
		err := s.d.Policy.CheckTool(name)
		if err == nil {
			out, err = h(ctx, in)
		}
		if s.d.Policy.AuditLog && s.d.Log != nil {
			s.d.Log.Info("mcp tool call", "tool", name, "ok", err == nil, "ms", time.Since(start).Milliseconds())
		}
		if err != nil {
			return errorResult(err), nil, nil
		}
		return textResult(out), nil, nil
	})
}

// resolve checks the profile and database and returns a connected session.
func (s *Server) resolve(ctx context.Context, conn, db string) (*mongox.Session, error) {
	profiles, err := s.d.Profiles()
	if err != nil {
		return nil, err
	}
	var id string
	for _, p := range profiles {
		if p.ID == conn || p.Name == conn {
			id = p.ID
		}
	}
	if id == "" {
		return nil, denied("unknown connection %q", conn)
	}
	if err := s.d.Policy.CheckProfile(id); err != nil {
		return nil, err
	}
	if db != "" {
		if err := s.d.Policy.CheckDatabase(db); err != nil {
			return nil, err
		}
	}
	return s.d.Connect(ctx, id)
}

func (s *Server) collection(ctx context.Context, in collIn) (*mongox.Session, error) {
	if err := s.d.Policy.CheckCollection(in.Collection); err != nil {
		return nil, err
	}
	return s.resolve(ctx, in.Connection, in.Database)
}

func parseDoc(label, text string) (bson.D, error) {
	d, err := mql.ParseDocument(text)
	if err != nil {
		return nil, apperr.Invalid("%s: %v", label, err)
	}
	return d, CheckValue(d)
}

// render masks (if configured) and renders documents as relaxed Extended JSON.
func (s *Server) render(docs []bson.D) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(docs))
	for _, d := range docs {
		if s.d.Masker != nil {
			masked, _, err := s.d.Masker.Apply(d)
			if err != nil {
				return nil, apperr.New(apperr.CodeReadOnly, "A document could not be masked, so it was withheld: %v", err)
			}
			d = masked
		}
		out = append(out, json.RawMessage(mql.RelaxedJSON(d)))
	}
	return out, nil
}

func (s *Server) register() {
	add(s, ToolListConnections, func(ctx context.Context, _ struct{}) (any, error) {
		profiles, err := s.d.Profiles()
		if err != nil {
			return nil, err
		}
		type row struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			ReadOnly bool   `json:"readOnly"`
		}
		out := []row{}
		for _, p := range profiles {
			if s.d.Policy.CheckProfile(p.ID) == nil {
				out = append(out, row{p.ID, p.Name, p.ReadOnly})
			}
		}
		return out, nil
	})
	add(s, ToolListDatabases, func(ctx context.Context, in connIn) (any, error) {
		sess, err := s.resolve(ctx, in.Connection, "")
		if err != nil {
			return nil, err
		}
		names, err := sess.Client.ListDatabaseNames(ctx, bson.D{}, options.ListDatabases().SetAuthorizedDatabases(true))
		if err != nil {
			return nil, apperr.From(err)
		}
		out := []string{}
		for _, n := range names {
			if s.d.Policy.CheckDatabase(n) == nil {
				out = append(out, n)
			}
		}
		return out, nil
	})
	add(s, ToolListCollections, func(ctx context.Context, in dbIn) (any, error) {
		sess, err := s.resolve(ctx, in.Connection, in.Database)
		if err != nil {
			return nil, err
		}
		names, err := sess.Client.Database(in.Database).ListCollectionNames(ctx, bson.D{{Key: "name", Value: bson.D{{Key: "$not", Value: bson.Regex{Pattern: "^system\\."}}}}})
		if err != nil {
			return nil, apperr.From(err)
		}
		return names, nil
	})
	add(s, ToolSchema, func(ctx context.Context, in collIn) (any, error) {
		sess, err := s.collection(ctx, in)
		if err != nil {
			return nil, err
		}
		cur, err := sess.Client.Database(in.Database).Collection(in.Collection).Aggregate(ctx,
			bson.A{bson.D{{Key: "$sample", Value: bson.D{{Key: "size", Value: 200}}}}})
		if err != nil {
			return nil, apperr.From(err)
		}
		defer cur.Close(context.Background())
		a := schema.NewAnalyzer()
		for cur.Next(ctx) {
			var d bson.D
			if cur.Decode(&d) == nil {
				a.Add(d)
			}
		}
		rep := a.Report()
		type field struct {
			Path     string   `json:"path"`
			Types    []string `json:"types"`
			Presence float64  `json:"presence"`
		}
		fields := []field{}
		for _, f := range rep.Fields {
			fl := field{Path: f.Path, Presence: f.Presence}
			for _, t := range f.Types {
				fl.Types = append(fl.Types, t.Type)
			}
			fields = append(fields, fl) // no sample values: they may be sensitive
		}
		return map[string]any{"sampled": rep.Sampled, "fields": fields}, nil
	})
	add(s, ToolFind, func(ctx context.Context, in findIn) (any, error) {
		sess, err := s.collection(ctx, in.collIn)
		if err != nil {
			return nil, err
		}
		filter, err := parseDoc("filter", in.Filter)
		if err != nil {
			return nil, err
		}
		limit := int64(s.d.Policy.MaxDocs)
		if in.Limit > 0 && in.Limit < limit {
			limit = in.Limit
		}
		opts := options.Find().SetLimit(limit)
		if in.Projection != "" {
			proj, err := parseDoc("projection", in.Projection)
			if err != nil {
				return nil, err
			}
			opts.SetProjection(proj)
		}
		if in.Sort != "" {
			sort, err := parseDoc("sort", in.Sort)
			if err != nil {
				return nil, err
			}
			opts.SetSort(sort)
		}
		cur, err := sess.Client.Database(in.Database).Collection(in.Collection).Find(ctx, filter, opts)
		if err != nil {
			return nil, apperr.From(err)
		}
		var docs []bson.D
		if err := cur.All(ctx, &docs); err != nil {
			return nil, apperr.From(err)
		}
		return s.render(docs)
	})
	add(s, ToolCount, func(ctx context.Context, in findIn) (any, error) {
		sess, err := s.collection(ctx, in.collIn)
		if err != nil {
			return nil, err
		}
		filter, err := parseDoc("filter", in.Filter)
		if err != nil {
			return nil, err
		}
		n, err := sess.Client.Database(in.Database).Collection(in.Collection).CountDocuments(ctx, filter)
		if err != nil {
			return nil, apperr.From(err)
		}
		return map[string]int64{"count": n}, nil
	})
	add(s, ToolAggregate, func(ctx context.Context, in aggIn) (any, error) {
		sess, err := s.collection(ctx, in.collIn)
		if err != nil {
			return nil, err
		}
		stages, err := aggregate.FromText(in.Pipeline)
		if err != nil {
			return nil, err
		}
		pipe, err := aggregate.Build(stages, -1)
		if err != nil {
			return nil, err
		}
		if err := CheckValue(bson.A(toA(pipe))); err != nil {
			return nil, err
		}
		pipe = append(pipe, bson.D{{Key: "$limit", Value: s.d.Policy.MaxDocs}})
		cur, err := sess.Client.Database(in.Database).Collection(in.Collection).Aggregate(ctx, pipe)
		if err != nil {
			return nil, apperr.From(err)
		}
		var docs []bson.D
		if err := cur.All(ctx, &docs); err != nil {
			return nil, apperr.From(err)
		}
		return s.render(docs)
	})
	add(s, ToolExplain, func(ctx context.Context, in findIn) (any, error) {
		sess, err := s.collection(ctx, in.collIn)
		if err != nil {
			return nil, err
		}
		filter, err := parseDoc("filter", in.Filter)
		if err != nil {
			return nil, err
		}
		var out bson.D
		cmd := bson.D{{Key: "explain", Value: bson.D{{Key: "find", Value: in.Collection}, {Key: "filter", Value: filter}}}, {Key: "verbosity", Value: "queryPlanner"}}
		if err := sess.Client.Database(in.Database).RunCommand(ctx, cmd).Decode(&out); err != nil {
			return nil, apperr.From(err)
		}
		qp, _ := mql.Lookup(out, "queryPlanner")
		return json.RawMessage(mql.RelaxedJSON(bson.D{{Key: "queryPlanner", Value: qp}})), nil
	})
	add(s, ToolInsertOne, func(ctx context.Context, in insertIn) (any, error) {
		sess, err := s.collection(ctx, in.collIn)
		if err != nil {
			return nil, err
		}
		if err := sess.RequireWrite(ctx, in.Database, in.Collection, mongox.ActionInsert); err != nil {
			return nil, err
		}
		doc, err := parseDoc("document", in.Document)
		if err != nil {
			return nil, err
		}
		res, err := sess.Client.Database(in.Database).Collection(in.Collection).InsertOne(ctx, doc)
		if err != nil {
			return nil, apperr.From(err)
		}
		return map[string]string{"insertedId": mql.Compact(res.InsertedID)}, nil
	})
	write := func(ctx context.Context, in collIn, filterText string, update any, action string) (any, error) {
		sess, err := s.collection(ctx, in)
		if err != nil {
			return nil, err
		}
		if err := sess.RequireWrite(ctx, in.Database, in.Collection, action, mongox.ActionFind); err != nil {
			return nil, err
		}
		filter, err := parseDoc("filter", filterText)
		if err != nil {
			return nil, err
		}
		if len(filter) == 0 {
			return nil, denied("an empty filter would affect the whole collection")
		}
		bk, err := backup.Create(s.d.BackupDir, "mcp-"+action, "MCP "+action+" on "+in.Database+"."+in.Collection, in.Database, in.Collection)
		if err != nil {
			return nil, apperr.Wrap(apperr.CodeInternal, err, "Could not create the backup; nothing was changed.")
		}
		defer bk.Close()
		res, err := backup.WriteManyWithBackup(ctx, sess.Client.Database(in.Database).Collection(in.Collection), filter, update, bk, int64(s.d.Policy.MaxWriteDocs))
		if err != nil {
			return nil, err
		}
		if err := bk.Close(); err != nil {
			return nil, err
		}
		return res, nil
	}
	add(s, ToolUpdateMany, func(ctx context.Context, in updateIn) (any, error) {
		upd, err := mql.ParseValue(in.Update)
		if err != nil {
			return nil, apperr.Invalid("update: %v", err)
		}
		if err := CheckValue(upd); err != nil {
			return nil, err
		}
		if d, ok := upd.(bson.D); ok && (len(d) == 0 || d[0].Key[0] != '$') {
			return nil, apperr.Invalid("update must use operators such as $set")
		}
		return write(ctx, in.collIn, in.Filter, upd, mongox.ActionUpdate)
	})
	add(s, ToolDeleteMany, func(ctx context.Context, in deleteIn) (any, error) {
		return write(ctx, in.collIn, in.Filter, nil, mongox.ActionRemove)
	})
}

func toA(p []bson.D) []any {
	out := make([]any, len(p))
	for i, d := range p {
		out[i] = d
	}
	return out
}
