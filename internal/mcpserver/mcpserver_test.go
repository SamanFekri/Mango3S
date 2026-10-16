package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mango3s/internal/apperr"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/mql"
)

func TestPolicyChecks(t *testing.T) {
	p := Policy{Enabled: true, Profiles: []string{"p1"}, Databases: []string{"shop", "analytics_*"}, Tools: []string{ToolFind, ToolDeleteMany, "bogus"}}.Normalize()
	if len(p.Tools) != 2 || p.MaxDocs != 50 || p.MaxWriteDocs != 100 {
		t.Fatalf("normalize: %+v", p)
	}
	if p.CheckTool(ToolFind) != nil || p.CheckTool(ToolCount) == nil || p.CheckTool("nope") == nil {
		t.Fatal("tool allowlist")
	}
	if p.CheckTool(ToolDeleteMany) == nil {
		t.Fatal("write tool allowed without the write switch")
	}
	p.AllowWrites = true
	if p.CheckTool(ToolDeleteMany) != nil {
		t.Fatal("write tool refused with the switch on")
	}
	if p.CheckProfile("p1") != nil || p.CheckProfile("p2") == nil {
		t.Fatal("profile allowlist")
	}
	for db, ok := range map[string]bool{"shop": true, "analytics_2024": true, "shop2": false, "admin": false, "local": false, "": false} {
		if (p.CheckDatabase(db) == nil) != ok {
			t.Errorf("database %q: allowed=%v", db, !ok)
		}
	}
	star := Policy{Databases: []string{"*"}}
	if star.CheckDatabase("config") == nil {
		t.Fatal("system database reachable through *")
	}
	if p.CheckCollection("system.users") == nil || p.CheckCollection("orders") != nil {
		t.Fatal("collection check")
	}
}

func TestCheckValueRefusesJavaScriptAndWrites(t *testing.T) {
	bad := []string{
		`{ $where: "this.a > 1" }`,
		`{ a: { $elemMatch: { $where: "x" } } }`,
		`[{ $match: {} }, { $out: "x" }]`,
		`[{ $group: { _id: null, x: { $accumulator: {} } } }]`,
		`{ $expr: { $function: { body: "f", args: [], lang: "js" } } }`,
		`[{ $merge: { into: "x" } }]`,
	}
	for _, s := range bad {
		v, err := mql.ParseValue(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckValue(v); !apperr.Is(err, apperr.CodeReadOnly) {
			t.Errorf("%s: %v", s, err)
		}
	}
	ok, _ := mql.ParseValue(`[{ $match: { a: { $gt: 1 } } }, { $group: { _id: "$b", n: { $sum: 1 } } }]`)
	if err := CheckValue(ok); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	p, err := s.Load()
	if err != nil || p.Enabled || len(p.Tools) != 0 {
		t.Fatalf("default policy must be disabled and empty: %+v %v", p, err)
	}
	if _, err := s.Save(Policy{Databases: []string{"[bad"}}); err == nil {
		t.Fatal("bad pattern accepted")
	}
	if _, err := s.Save(Policy{Enabled: true, Tools: []string{ToolFind}, Databases: []string{"shop"}, Profiles: []string{"p"}}); err != nil {
		t.Fatal(err)
	}
	p, _ = s.Load()
	if !p.Enabled || p.Tools[0] != ToolFind {
		t.Fatalf("reload: %+v", p)
	}
}

// connectClient wires an in-memory MCP client to a server built from policy.
func connectClient(t *testing.T, policy Policy) (*mcp.ClientSession, *int) {
	t.Helper()
	connects := 0
	srv := New(Deps{
		Policy:  policy,
		Version: "test",
		Profiles: func() ([]config.Profile, error) {
			return []config.Profile{{ID: "p1", Name: "Shared"}, {ID: "p2", Name: "Private"}}, nil
		},
		Connect: func(ctx context.Context, id string) (*mongox.Session, error) {
			connects++
			return nil, errors.New("no database in unit tests")
		},
	})
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, &connects
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return err.Error(), true
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func TestOnlyAllowedToolsAreExposed(t *testing.T) {
	cs, _ := connectClient(t, Policy{Enabled: true, Profiles: []string{"p1"}, Databases: []string{"shop"},
		Tools: []string{ToolListConnections, ToolFind, ToolDeleteMany}})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "find,list_connections" {
		t.Fatalf("exposed tools: %v (delete_many must be hidden without allowWrites)", names)
	}
	if _, isErr := call(t, cs, ToolCount, map[string]any{}); !isErr {
		t.Fatal("unregistered tool callable")
	}
}

func TestBoundaryEnforcementBeforeConnecting(t *testing.T) {
	cs, connects := connectClient(t, Policy{Enabled: true, Profiles: []string{"p1"}, Databases: []string{"shop"},
		Tools: []string{ToolListConnections, ToolFind}})

	text, isErr := call(t, cs, ToolListConnections, map[string]any{})
	if isErr || !strings.Contains(text, "Shared") || strings.Contains(text, "Private") {
		t.Fatalf("list_connections must only show shared profiles: %s", text)
	}
	cases := []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"connection": "Private", "database": "shop", "collection": "orders"}, "not shared"},
		{map[string]any{"connection": "Ghost", "database": "shop", "collection": "orders"}, "unknown connection"},
		{map[string]any{"connection": "Shared", "database": "admin", "collection": "orders"}, "system database"},
		{map[string]any{"connection": "Shared", "database": "hr", "collection": "salaries"}, "not in the allowlist"},
		{map[string]any{"connection": "Shared", "database": "shop", "collection": "system.users"}, "system collection"},
	}
	for _, c := range cases {
		text, isErr := call(t, cs, ToolFind, c.args)
		if !isErr || !strings.Contains(text, c.want) {
			t.Errorf("%v: got %q", c.args, text)
		}
	}
	if *connects != 0 {
		t.Fatalf("policy violations must be refused before connecting (connected %d times)", *connects)
	}
}

func TestDisabledServerRefusesToRun(t *testing.T) {
	srv := New(Deps{Policy: Policy{Enabled: false, Tools: []string{ToolFind}}})
	if err := srv.Run(context.Background()); !apperr.Is(err, apperr.CodeReadOnly) {
		t.Fatalf("disabled server ran: %v", err)
	}
}

func TestAuditLogOmitsArguments(t *testing.T) {
	var buf strings.Builder
	srv := New(Deps{
		Policy:   Policy{Enabled: true, Profiles: []string{"p1"}, Databases: []string{"shop"}, Tools: []string{ToolFind}, AuditLog: true},
		Profiles: func() ([]config.Profile, error) { return []config.Profile{{ID: "p1", Name: "Shared"}}, nil },
		Connect: func(ctx context.Context, id string) (*mongox.Session, error) {
			return nil, errors.New("no database in unit tests")
		},
		Log: slog.New(slog.NewJSONHandler(&buf, nil)),
	})
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	call(t, cs, ToolFind, map[string]any{"connection": "Shared", "database": "hr", "collection": "salaries", "filter": `{ ssn: "123-45-6789" }`})
	out := buf.String()
	if !strings.Contains(out, `"tool":"find"`) || !strings.Contains(out, `"ok":false`) {
		t.Fatalf("audit entry missing: %s", out)
	}
	for _, secret := range []string{"123-45-6789", "salaries", "hr"} {
		if strings.Contains(out, secret) {
			t.Fatalf("audit log leaked argument %q: %s", secret, out)
		}
	}
}
