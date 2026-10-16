// Package mcpserver exposes approved database tools to MCP clients (Claude
// Desktop, Claude Code, IDE agents) over stdio via `mango3s mcp`.
//
// Access control is deny-by-default and enforced in two places: only tools
// on the allowlist are registered, and every call re-checks the policy
// (tool, connection profile, database pattern, limits). Write tools need the
// write switch, a writable profile and server privileges, are capped in
// size and are backed up. Server-side JavaScript ($where, $function,
// $accumulator) and pipeline writes ($out, $merge) are always refused.
package mcpserver

import (
	"path"
	"path/filepath"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
)

// Tool names.
const (
	ToolListConnections = "list_connections"
	ToolListDatabases   = "list_databases"
	ToolListCollections = "list_collections"
	ToolSchema          = "collection_schema"
	ToolFind            = "find"
	ToolCount           = "count"
	ToolAggregate       = "aggregate"
	ToolExplain         = "explain"
	ToolInsertOne       = "insert_one"
	ToolUpdateMany      = "update_many"
	ToolDeleteMany      = "delete_many"
)

// ToolInfo describes a tool for the settings UI.
type ToolInfo struct {
	Name        string `json:"name"`
	Write       bool   `json:"write"`
	Description string `json:"description"`
}

// Catalog lists every tool Mango3S can expose.
var Catalog = []ToolInfo{
	{ToolListConnections, false, "List the connection profiles the client may use."},
	{ToolListDatabases, false, "List databases on a connection (filtered by the database allowlist)."},
	{ToolListCollections, false, "List collections in a database."},
	{ToolSchema, false, "Summarize a collection's fields and types from a sample."},
	{ToolFind, false, "Run a find query (results capped and masked)."},
	{ToolCount, false, "Count documents matching a filter."},
	{ToolAggregate, false, "Run a read-only aggregation pipeline ($out/$merge refused)."},
	{ToolExplain, false, "Explain a find query (queryPlanner; not executed)."},
	{ToolInsertOne, true, "Insert one document."},
	{ToolUpdateMany, true, "Update matching documents (capped, backed up)."},
	{ToolDeleteMany, true, "Delete matching documents (capped, backed up)."},
}

func toolInfo(name string) (ToolInfo, bool) {
	for _, t := range Catalog {
		if t.Name == name {
			return t, true
		}
	}
	return ToolInfo{}, false
}

// Policy is the persisted MCP configuration (mcp.json).
type Policy struct {
	Enabled       bool     `json:"enabled"`
	Profiles      []string `json:"profiles"`  // allowed connection profile IDs
	Databases     []string `json:"databases"` // glob patterns, e.g. "shop", "analytics_*"; "*" = all non-system
	Tools         []string `json:"tools"`     // allowed tool names
	AllowWrites   bool     `json:"allowWrites"`
	MaxDocs       int      `json:"maxDocs"`       // results per call (default 50, max 500)
	MaxWriteDocs  int      `json:"maxWriteDocs"`  // documents a write may touch (default 100, max 10000)
	MaskRuleSetID string   `json:"maskRuleSetId"` // masking applied to returned documents
	AuditLog      bool     `json:"auditLog"`
}

// Normalize applies defaults and bounds.
func (p Policy) Normalize() Policy {
	if p.MaxDocs <= 0 {
		p.MaxDocs = 50
	}
	if p.MaxDocs > 500 {
		p.MaxDocs = 500
	}
	if p.MaxWriteDocs <= 0 {
		p.MaxWriteDocs = 100
	}
	if p.MaxWriteDocs > 10000 {
		p.MaxWriteDocs = 10000
	}
	var tools []string
	for _, t := range p.Tools {
		if _, ok := toolInfo(t); ok {
			tools = append(tools, t)
		}
	}
	p.Tools = tools
	if p.Profiles == nil {
		p.Profiles = []string{}
	}
	if p.Databases == nil {
		p.Databases = []string{}
	}
	if p.Tools == nil {
		p.Tools = []string{}
	}
	return p
}

// Denied is the error for policy violations.
func denied(format string, args ...any) error {
	return apperr.New(apperr.CodeReadOnly, "Not allowed by the Mango3S MCP policy: "+format, args...)
}

// CheckTool reports whether a tool may be used.
func (p Policy) CheckTool(name string) error {
	info, ok := toolInfo(name)
	if !ok {
		return denied("unknown tool %q", name)
	}
	if !contains(p.Tools, name) {
		return denied("tool %s is not enabled", name)
	}
	if info.Write && !p.AllowWrites {
		return denied("write tools are disabled")
	}
	return nil
}

// CheckProfile reports whether a profile may be used.
func (p Policy) CheckProfile(id string) error {
	if !contains(p.Profiles, id) {
		return denied("this connection is not shared with MCP clients")
	}
	return nil
}

// CheckDatabase matches db against the allowlist. System databases are
// never reachable.
func (p Policy) CheckDatabase(db string) error {
	if db == "" {
		return apperr.Invalid("A database name is required.")
	}
	if db == "admin" || db == "local" || db == "config" {
		return denied("system database %s", db)
	}
	for _, pat := range p.Databases {
		if ok, _ := path.Match(pat, db); ok {
			return nil
		}
	}
	return denied("database %s is not in the allowlist", db)
}

// CheckCollection refuses system collections.
func (p Policy) CheckCollection(coll string) error {
	if coll == "" {
		return apperr.Invalid("A collection name is required.")
	}
	if strings.HasPrefix(coll, "system.") {
		return denied("system collection %s", coll)
	}
	return nil
}

var forbiddenOperators = map[string]string{
	"$where":       "server-side JavaScript",
	"$function":    "server-side JavaScript",
	"$accumulator": "server-side JavaScript",
	"$out":         "writing from a pipeline",
	"$merge":       "writing from a pipeline",
}

// CheckValue walks a filter, update or pipeline for forbidden operators.
func CheckValue(v any) error {
	switch t := v.(type) {
	case bson.D:
		for _, e := range t {
			if why, bad := forbiddenOperators[e.Key]; bad {
				return denied("%s is refused (%s)", e.Key, why)
			}
			if err := CheckValue(e.Value); err != nil {
				return err
			}
		}
	case bson.A:
		for _, el := range t {
			if err := CheckValue(el); err != nil {
				return err
			}
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Store persists the policy in mcp.json.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore keeps the policy in dir/mcp.json.
func NewStore(dir string) *Store { return &Store{path: filepath.Join(dir, "mcp.json")} }

// Load returns the policy (disabled when the file does not exist).
func (s *Store) Load() (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p Policy
	if err := config.ReadJSON(s.path, &p); err != nil {
		return Policy{}, err
	}
	return p.Normalize(), nil
}

// Save validates and stores the policy.
func (s *Store) Save(p Policy) (Policy, error) {
	p = p.Normalize()
	for _, pat := range p.Databases {
		if _, err := path.Match(pat, "x"); err != nil {
			return Policy{}, apperr.Invalid("Invalid database pattern %q.", pat)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return p, config.WriteJSON(s.path, p)
}
