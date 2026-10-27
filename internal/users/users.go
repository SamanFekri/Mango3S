// Package users manages MongoDB users and roles: listing them with their
// effective privileges, creating users, changing passwords and role grants,
// and custom roles. Passwords are only ever sent to the server; they are never
// logged or returned.
package users

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
	"mango3s/internal/mongox"
)

// Role names a role in a database.
type Role struct {
	Role string `json:"role" bson:"role"`
	DB   string `json:"db" bson:"db"`
}

func (r Role) String() string { return r.Role + "@" + r.DB }

// Resource is what a privilege applies to; see mongox.Privilege.
type Resource struct {
	DB          *string `json:"db,omitempty" bson:"db,omitempty"`
	Collection  *string `json:"collection,omitempty" bson:"collection,omitempty"`
	Cluster     bool    `json:"cluster,omitempty" bson:"cluster,omitempty"`
	AnyResource bool    `json:"anyResource,omitempty" bson:"anyResource,omitempty"`
}

// Privilege is a set of actions on a resource.
type Privilege struct {
	Resource Resource `json:"resource" bson:"resource"`
	Actions  []string `json:"actions" bson:"actions"`
}

// User is a database user.
type User struct {
	User       string   `json:"user" bson:"user"`
	DB         string   `json:"db" bson:"db"`
	Roles      []Role   `json:"roles" bson:"roles"`
	Mechanisms []string `json:"mechanisms,omitempty" bson:"mechanisms,omitempty"`
	// Self marks the user this connection is signed in as.
	Self bool `json:"self" bson:"-"`
}

// RoleInfo describes a built-in or custom role.
type RoleInfo struct {
	Role       string      `json:"role" bson:"role"`
	DB         string      `json:"db" bson:"db"`
	IsBuiltin  bool        `json:"isBuiltin" bson:"isBuiltin"`
	Roles      []Role      `json:"roles" bson:"roles"` // roles it inherits from
	Privileges []Privilege `json:"privileges" bson:"privileges"`
}

// Effective is everything a user can do: all roles it holds directly or
// through inheritance, and the privileges they add up to.
type Effective struct {
	Roles      []Role      `json:"roles"`
	Privileges []Privilege `json:"privileges"`
}

// Service is the users and roles API.
type Service struct{ src mongox.Source }

// New returns a Service backed by src.
func New(src mongox.Source) *Service { return &Service{src: src} }

func (s *Service) session(connID string) (*mongox.Session, error) { return s.src.Session(connID) }

func (s *Service) writable(connID string) (*mongox.Session, error) {
	sess, err := s.src.Session(connID)
	if err != nil {
		return nil, err
	}
	return sess, sess.RequireWritable()
}

func run(ctx context.Context, d *mongo.Database, cmd bson.D, out any) error {
	r := d.RunCommand(ctx, cmd)
	if out == nil {
		return apperr.Check(r.Err())
	}
	return apperr.Check(r.Decode(out))
}

// Users lists the users defined in db, or in every database when db is "".
func (s *Service) Users(ctx context.Context, connID, db string) ([]User, error) {
	sess, err := s.session(connID)
	if err != nil {
		return nil, err
	}
	var res struct {
		Users []User `bson:"users"`
	}
	target, cmd := db, bson.D{{Key: "usersInfo", Value: 1}}
	if db == "" {
		target, cmd = "admin", bson.D{{Key: "usersInfo", Value: bson.D{{Key: "forAllDBs", Value: true}}}}
	}
	if err := run(ctx, sess.Client.Database(target), cmd, &res); err != nil {
		return nil, err
	}
	me, _ := self(ctx, sess)
	for i := range res.Users {
		u := &res.Users[i]
		u.Self = slices.Contains(me, Role{Role: u.User, DB: u.DB})
		if u.Roles == nil {
			u.Roles = []Role{}
		}
		sortRoles(u.Roles)
	}
	sort.Slice(res.Users, func(i, j int) bool {
		if res.Users[i].DB != res.Users[j].DB {
			return res.Users[i].DB < res.Users[j].DB
		}
		return res.Users[i].User < res.Users[j].User
	})
	return res.Users, nil
}

// Roles lists the roles of db: its custom roles, plus the built-in ones when
// builtin is set, each with its privileges.
func (s *Service) Roles(ctx context.Context, connID, db string, builtin bool) ([]RoleInfo, error) {
	sess, err := s.session(connID)
	if err != nil {
		return nil, err
	}
	if db == "" {
		return nil, apperr.Invalid("Choose a database.")
	}
	var res struct {
		Roles []RoleInfo `bson:"roles"`
	}
	cmd := bson.D{{Key: "rolesInfo", Value: 1}, {Key: "showPrivileges", Value: true}, {Key: "showBuiltinRoles", Value: builtin}}
	if err := run(ctx, sess.Client.Database(db), cmd, &res); err != nil {
		return nil, err
	}
	for i := range res.Roles {
		r := &res.Roles[i]
		if r.Roles == nil {
			r.Roles = []Role{}
		}
		if r.Privileges == nil {
			r.Privileges = []Privilege{}
		}
	}
	sort.Slice(res.Roles, func(i, j int) bool {
		if res.Roles[i].IsBuiltin != res.Roles[j].IsBuiltin {
			return !res.Roles[i].IsBuiltin // custom roles first
		}
		return res.Roles[i].Role < res.Roles[j].Role
	})
	return res.Roles, nil
}

// EffectiveOf reports what a user can do, as the server computes it from the
// user's roles and everything they inherit.
func (s *Service) EffectiveOf(ctx context.Context, connID, db, user string) (Effective, error) {
	sess, err := s.session(connID)
	if err != nil {
		return Effective{}, err
	}
	var res struct {
		Users []struct {
			InheritedRoles      []Role      `bson:"inheritedRoles"`
			InheritedPrivileges []Privilege `bson:"inheritedPrivileges"`
		} `bson:"users"`
	}
	cmd := bson.D{{Key: "usersInfo", Value: bson.D{{Key: "user", Value: user}, {Key: "db", Value: db}}}, {Key: "showPrivileges", Value: true}}
	if err := run(ctx, sess.Client.Database(db), cmd, &res); err != nil {
		return Effective{}, err
	}
	if len(res.Users) == 0 {
		return Effective{}, apperr.New(apperr.CodeNotFound, "User %s@%s does not exist.", user, db)
	}
	e := Effective{Roles: res.Users[0].InheritedRoles, Privileges: res.Users[0].InheritedPrivileges}
	if e.Roles == nil {
		e.Roles = []Role{}
	}
	if e.Privileges == nil {
		e.Privileges = []Privilege{}
	}
	sortRoles(e.Roles)
	return e, nil
}

// Can reports which of actions a set of privileges does not grant on db.coll
// (empty: all granted), with MongoDB's resource matching.
func Can(privs []Privilege, db, coll string, actions ...string) []string {
	mp := make([]mongox.Privilege, len(privs))
	for i, p := range privs {
		mp[i].Resource.DB = p.Resource.DB
		mp[i].Resource.Collection = p.Resource.Collection
		mp[i].Resource.Cluster = p.Resource.Cluster
		mp[i].Resource.AnyResource = p.Resource.AnyResource
		mp[i].Actions = p.Actions
	}
	return mongox.MissingActions(mp, db, coll, actions...)
}

// NewUser is a user to create.
type NewUser struct {
	ConnID   string `json:"connId"`
	DB       string `json:"db"`
	User     string `json:"user"`
	Password string `json:"password"`
	Roles    []Role `json:"roles"`
}

// CreateUser creates a user with a password and roles.
func (s *Service) CreateUser(ctx context.Context, u NewUser) error {
	sess, err := s.writable(u.ConnID)
	if err != nil {
		return err
	}
	u.User = strings.TrimSpace(u.User)
	switch {
	case u.DB == "":
		return apperr.Invalid("Choose the database the user belongs to (its authentication database).")
	case u.User == "":
		return apperr.Invalid("Enter a user name.")
	case u.Password == "":
		return apperr.Invalid("Enter a password.")
	}
	cmd := bson.D{{Key: "createUser", Value: u.User}, {Key: "pwd", Value: u.Password}, {Key: "roles", Value: rolesArg(u.Roles)}}
	return run(ctx, sess.Client.Database(u.DB), cmd, nil)
}

// SetPassword changes a user's password.
func (s *Service) SetPassword(ctx context.Context, connID, db, user, password string) error {
	sess, err := s.writable(connID)
	if err != nil {
		return err
	}
	if password == "" {
		return apperr.Invalid("Enter a password.")
	}
	return run(ctx, sess.Client.Database(db), bson.D{{Key: "updateUser", Value: user}, {Key: "pwd", Value: password}}, nil)
}

// SetRoles makes roles the user's direct roles: missing ones are granted,
// extra ones revoked. Call Check first to warn about locking yourself out.
func (s *Service) SetRoles(ctx context.Context, connID, db, user string, roles []Role) error {
	sess, err := s.writable(connID)
	if err != nil {
		return err
	}
	cur, err := s.userRoles(ctx, sess, db, user)
	if err != nil {
		return err
	}
	grant, revoke := diffRoles(cur, roles)
	d := sess.Client.Database(db)
	// Grant before revoking, so a swap never leaves the user with neither.
	if len(grant) > 0 {
		if err := run(ctx, d, bson.D{{Key: "grantRolesToUser", Value: user}, {Key: "roles", Value: rolesArg(grant)}}, nil); err != nil {
			return err
		}
	}
	if len(revoke) > 0 {
		if err := run(ctx, d, bson.D{{Key: "revokeRolesFromUser", Value: user}, {Key: "roles", Value: rolesArg(revoke)}}, nil); err != nil {
			return err
		}
	}
	return nil
}

// DropUser deletes a user. confirm must repeat the user name.
func (s *Service) DropUser(ctx context.Context, connID, db, user, confirm string) error {
	if confirm != user {
		return apperr.Invalid("Type the user name %q to confirm.", user)
	}
	sess, err := s.writable(connID)
	if err != nil {
		return err
	}
	return run(ctx, sess.Client.Database(db), bson.D{{Key: "dropUser", Value: user}}, nil)
}

// RoleSpec is a custom role to create or update: its privileges and the roles
// it inherits.
type RoleSpec struct {
	ConnID     string      `json:"connId"`
	DB         string      `json:"db"`
	Role       string      `json:"role"`
	Privileges []Privilege `json:"privileges"`
	Roles      []Role      `json:"roles"`
}

// SaveRole creates the custom role, or replaces its privileges and inherited
// roles when it exists (create = false).
func (s *Service) SaveRole(ctx context.Context, r RoleSpec, create bool) error {
	sess, err := s.writable(r.ConnID)
	if err != nil {
		return err
	}
	r.Role = strings.TrimSpace(r.Role)
	if r.DB == "" || r.Role == "" {
		return apperr.Invalid("Enter a role name and its database.")
	}
	privs := bson.A{}
	for _, p := range r.Privileges {
		if len(p.Actions) == 0 {
			continue
		}
		privs = append(privs, bson.D{{Key: "resource", Value: resourceArg(p.Resource)}, {Key: "actions", Value: p.Actions}})
	}
	verb := "updateRole"
	if create {
		verb = "createRole"
	}
	cmd := bson.D{{Key: verb, Value: r.Role}, {Key: "privileges", Value: privs}, {Key: "roles", Value: rolesArg(r.Roles)}}
	return run(ctx, sess.Client.Database(r.DB), cmd, nil)
}

// DropRole deletes a custom role. confirm must repeat the role name.
func (s *Service) DropRole(ctx context.Context, connID, db, role, confirm string) error {
	if confirm != role {
		return apperr.Invalid("Type the role name %q to confirm.", role)
	}
	sess, err := s.writable(connID)
	if err != nil {
		return err
	}
	return run(ctx, sess.Client.Database(db), bson.D{{Key: "dropRole", Value: role}}, nil)
}

// ---- safety checks ---------------------------------------------------------

// Change is a planned change to a user, for Check.
type Change struct {
	ConnID string `json:"connId"`
	DB     string `json:"db"`
	User   string `json:"user"`
	Drop   bool   `json:"drop"`  // drop the user
	Roles  []Role `json:"roles"` // or give it exactly these roles
}

// adminRoles let a user manage users and roles on the whole deployment.
var adminRoles = []Role{{"root", "admin"}, {"userAdminAnyDatabase", "admin"}, {"userAdmin", "admin"}, {"__system", "admin"}}

func isAdmin(roles []Role) bool {
	for _, r := range roles {
		if slices.Contains(adminRoles, r) {
			return true
		}
	}
	return false
}

// Check returns warnings for a change that removes your own ability to manage
// users, or the last user that can. They are warnings: the change is allowed.
func (s *Service) Check(ctx context.Context, c Change) ([]string, error) {
	sess, err := s.session(c.ConnID)
	if err != nil {
		return nil, err
	}
	cur, err := s.userRoles(ctx, sess, c.DB, c.User)
	if err != nil {
		return nil, err
	}
	after := c.Roles
	if c.Drop {
		after = nil
	}
	if !isAdmin(cur) || isAdmin(after) {
		return nil, nil // this change takes no user-admin rights away
	}
	var warnings []string
	me, _ := self(ctx, sess)
	if slices.Contains(me, Role{Role: c.User, DB: c.DB}) {
		what := "removes your own right"
		if c.Drop {
			what = "deletes the user you are signed in as, and with it your right"
		}
		warnings = append(warnings, fmt.Sprintf("This %s to manage users and roles. You may not be able to undo it from this connection.", what))
	}
	all, err := s.Users(ctx, c.ConnID, "")
	if err != nil {
		warnings = append(warnings, "Mango3S could not check whether other users can still manage users (listing all users needs more rights).")
		return warnings, nil
	}
	others := 0
	for _, u := range all {
		if (u.User != c.User || u.DB != c.DB) && isAdmin(u.Roles) {
			others++
		}
	}
	if others == 0 {
		warnings = append(warnings, fmt.Sprintf("%s@%s is the last user with root, userAdminAnyDatabase or userAdmin on admin. Afterwards nobody can create users or change roles.", c.User, c.DB))
	}
	return warnings, nil
}

// ---- helpers ---------------------------------------------------------------

func (s *Service) userRoles(ctx context.Context, sess *mongox.Session, db, user string) ([]Role, error) {
	var res struct {
		Users []User `bson:"users"`
	}
	cmd := bson.D{{Key: "usersInfo", Value: bson.D{{Key: "user", Value: user}, {Key: "db", Value: db}}}}
	if err := run(ctx, sess.Client.Database(db), cmd, &res); err != nil {
		return nil, err
	}
	if len(res.Users) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "User %s@%s does not exist.", user, db)
	}
	return res.Users[0].Roles, nil
}

// self returns the users this connection is authenticated as (none when the
// server runs without authentication).
func self(ctx context.Context, sess *mongox.Session) ([]Role, error) {
	var st struct {
		AuthInfo struct {
			Users []struct {
				User string `bson:"user"`
				DB   string `bson:"db"`
			} `bson:"authenticatedUsers"`
		} `bson:"authInfo"`
	}
	if err := run(ctx, sess.Client.Database("admin"), bson.D{{Key: "connectionStatus", Value: 1}}, &st); err != nil {
		return nil, err
	}
	out := make([]Role, len(st.AuthInfo.Users))
	for i, u := range st.AuthInfo.Users {
		out[i] = Role{Role: u.User, DB: u.DB}
	}
	return out, nil
}

func diffRoles(cur, want []Role) (grant, revoke []Role) {
	for _, r := range want {
		if !slices.Contains(cur, r) && !slices.Contains(grant, r) {
			grant = append(grant, r)
		}
	}
	for _, r := range cur {
		if !slices.Contains(want, r) {
			revoke = append(revoke, r)
		}
	}
	return grant, revoke
}

func rolesArg(roles []Role) bson.A {
	a := bson.A{}
	for _, r := range roles {
		a = append(a, bson.D{{Key: "role", Value: r.Role}, {Key: "db", Value: r.DB}})
	}
	return a
}

func resourceArg(r Resource) bson.D {
	if r.Cluster {
		return bson.D{{Key: "cluster", Value: true}}
	}
	if r.AnyResource {
		return bson.D{{Key: "anyResource", Value: true}}
	}
	d, c := "", ""
	if r.DB != nil {
		d = *r.DB
	}
	if r.Collection != nil {
		c = *r.Collection
	}
	return bson.D{{Key: "db", Value: d}, {Key: "collection", Value: c}}
}

func sortRoles(r []Role) {
	sort.Slice(r, func(i, j int) bool {
		if r[i].DB != r[j].DB {
			return r[i].DB < r[j].DB
		}
		return r[i].Role < r[j].Role
	})
}
