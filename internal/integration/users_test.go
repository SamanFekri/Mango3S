//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"mango3s/internal/api"
	"mango3s/internal/config"
	"mango3s/internal/mongox"
	"mango3s/internal/secrets"
	"mango3s/internal/users"
)

// Roadmap 2.2: users and roles on a server with authentication
// (MONGO_AUTH_URI, e.g. mongodb://admin:change-me@localhost:27018/?authSource=admin).
func TestUsersAndRoles(t *testing.T) {
	uri := os.Getenv("MONGO_AUTH_URI")
	if uri == "" {
		t.Skip("MONGO_AUTH_URI not set")
	}
	app := api.New(api.Deps{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Store: config.NewStore(t.TempDir()), Secrets: secrets.NewMemoryStore(),
		Conns: mongox.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))})
	p, err := app.SaveProfile(api.ProfileInput{Name: "auth", URI: uri, ConnectTimeoutSeconds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Connect(p.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Disconnect(p.ID) })
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	db := "m3s_users_" + hex.EncodeToString(b)
	t.Cleanup(func() {
		_ = app.DropUser(p.ID, db, "alice", "alice")
		_ = app.DropUser(p.ID, db, "bob", "bob")
		_ = app.DropRole(p.ID, db, "reporter", "reporter")
		c := mustClient(t, uri)
		_ = c.Database(db).Drop(context.Background())
	})

	// Create a reader and check what the server says it can do.
	if err := app.CreateUser(users.NewUser{ConnID: p.ID, DB: db, User: "alice", Password: "first pass, 1", Roles: []users.Role{{Role: "read", DB: db}}}); err != nil {
		t.Fatal(err)
	}
	list, err := app.ListUsers(p.ID, db)
	if err != nil || len(list) != 1 || list[0].User != "alice" || !slices.Equal(list[0].Roles, []users.Role{{Role: "read", DB: db}}) || list[0].Self {
		t.Fatalf("users of %s: %+v %v", db, list, err)
	}
	all, err := app.ListUsers(p.ID, "")
	if err != nil || !slices.ContainsFunc(all, func(u users.User) bool { return u.Self && u.DB == "admin" }) {
		t.Fatalf("all users should include the signed-in admin: %+v %v", all, err)
	}
	eff, err := app.UserEffective(p.ID, db, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if m := users.Can(eff.Privileges, db, "orders", "find"); len(m) != 0 {
		t.Fatalf("read should allow find: missing %v", m)
	}
	if m := users.Can(eff.Privileges, db, "orders", "insert"); len(m) != 1 {
		t.Fatalf("read should not allow insert: %v", m)
	}

	// The user can sign in with exactly those rights.
	alice := func(pwd string) (*mongo.Client, error) { return connectAs(t, uri, "alice", pwd, db) }
	c, err := alice("first pass, 1")
	if err != nil {
		t.Fatalf("alice signs in: %v", err)
	}
	if _, err := c.Database(db).Collection("orders").InsertOne(context.Background(), bson.D{{Key: "x", Value: 1}}); err == nil {
		t.Fatal("a read-only user could insert")
	}
	_ = c.Disconnect(context.Background())

	// Grant readWrite (and drop read); then insert works.
	if err := app.SetUserRoles(p.ID, db, "alice", []users.Role{{Role: "readWrite", DB: db}}); err != nil {
		t.Fatal(err)
	}
	c, err = alice("first pass, 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Database(db).Collection("orders").InsertOne(context.Background(), bson.D{{Key: "x", Value: 1}}); err != nil {
		t.Fatalf("readWrite insert: %v", err)
	}
	_ = c.Disconnect(context.Background())
	if list, _ := app.ListUsers(p.ID, db); !slices.Equal(list[0].Roles, []users.Role{{Role: "readWrite", DB: db}}) {
		t.Fatalf("roles after change: %+v", list[0].Roles)
	}

	// New password: the old one stops working.
	if err := app.SetUserPassword(p.ID, db, "alice", "second-pass-2"); err != nil {
		t.Fatal(err)
	}
	if c, err := alice("first pass, 1"); err == nil {
		_ = c.Disconnect(context.Background())
		t.Fatal("old password still works")
	}
	if c, err := alice("second-pass-2"); err != nil {
		t.Fatalf("new password: %v", err)
	} else {
		_ = c.Disconnect(context.Background())
	}

	// A custom role with a privilege matrix, then inheriting a built-in role.
	s := func(v string) *string { return &v }
	spec := users.RoleSpec{ConnID: p.ID, DB: db, Role: "reporter",
		Privileges: []users.Privilege{{Resource: users.Resource{DB: s(db), Collection: s("reports")}, Actions: []string{"find", "insert"}}, {Resource: users.Resource{DB: s(db), Collection: s("x")}}}}
	if err := app.SaveRole(spec, true); err != nil {
		t.Fatal(err)
	}
	spec.Roles = []users.Role{{Role: "read", DB: db}}
	if err := app.SaveRole(spec, false); err != nil {
		t.Fatal(err)
	}
	roles, err := app.ListRoles(p.ID, db, true)
	if err != nil {
		t.Fatal(err)
	}
	ri := slices.IndexFunc(roles, func(r users.RoleInfo) bool { return r.Role == "reporter" })
	if ri != 0 || roles[ri].IsBuiltin || len(roles[ri].Privileges) != 1 || !slices.Equal(roles[ri].Roles, []users.Role{{Role: "read", DB: db}}) {
		t.Fatalf("custom role first, with one privilege (the empty one is skipped) and an inherited role: %+v", roles)
	}
	if !slices.ContainsFunc(roles, func(r users.RoleInfo) bool { return r.IsBuiltin && r.Role == "readWrite" }) {
		t.Fatal("built-in roles missing")
	}

	// Warnings: taking away the signed-in admin's rights.
	w, err := app.CheckUserChange(users.Change{ConnID: p.ID, DB: "admin", User: adminUser(t, uri), Roles: []users.Role{{Role: "read", DB: "admin"}}})
	if err != nil || len(w) == 0 || !strings.Contains(w[0], "your own") {
		t.Fatalf("self-demotion should warn: %v %v", w, err)
	}
	if w, err := app.CheckUserChange(users.Change{ConnID: p.ID, DB: db, User: "alice", Drop: true}); err != nil || len(w) != 0 {
		t.Fatalf("dropping a non-admin needs no warning: %v %v", w, err)
	}

	// Dropping needs the name typed.
	if err := app.DropUser(p.ID, db, "alice", "alic"); err == nil {
		t.Fatal("drop with a wrong confirmation")
	}
	if err := app.DropUser(p.ID, db, "alice", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := app.DropRole(p.ID, db, "reporter", "reporter"); err != nil {
		t.Fatal(err)
	}
	if list, _ := app.ListUsers(p.ID, db); len(list) != 0 {
		t.Fatalf("users left: %+v", list)
	}

	// The built-in shell has the user helpers, and history keeps no password.
	res, err := app.RunShell("users-shell", p.ID, db, `db.createUser({user: "bob", pwd: "shell secret, 3", roles: [{role: "read", db: "`+db+`"}]})
db.getUser("bob")
db.changeUserPassword("bob", "another secret")
db.dropUser("bob")`)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range res.Outputs {
		if o.Kind == "error" {
			t.Fatalf("shell: %s", o.Text)
		}
	}
	if !strings.Contains(res.Outputs[1].Text, `"bob"`) || strings.Contains(res.Outputs[1].Text, "secret") {
		t.Fatalf("getUser: %s", res.Outputs[1].Text)
	}
	h, err := app.ListHistory("", "createUser", 5)
	if err != nil || len(h) == 0 {
		t.Fatalf("history: %v %v", h, err)
	}
	if strings.Contains(h[0].Text, "secret") || !strings.Contains(h[0].Text, `pwd: "****"`) {
		t.Fatalf("password kept in history: %s", h[0].Text)
	}
}

func mustClient(t *testing.T, uri string) *mongo.Client {
	c, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c
}

// connectAs signs in as user on the server of uri and pings it.
func connectAs(t *testing.T, uri, user, pwd, authDB string) (*mongo.Client, error) {
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	u.User = nil
	u.RawQuery = ""
	opts := options.Client().ApplyURI(u.String()).SetServerSelectionTimeout(5 * time.Second).
		SetAuth(options.Credential{Username: user, Password: pwd, AuthSource: authDB})
	c, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	if err := c.Ping(context.Background(), nil); err != nil {
		_ = c.Disconnect(context.Background())
		return nil, err
	}
	return c, nil
}

func adminUser(t *testing.T, uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.User == nil {
		t.Fatalf("MONGO_AUTH_URI needs a user: %v", err)
	}
	return u.User.Username()
}
