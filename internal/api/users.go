package api

import (
	"context"

	"mango3s/internal/users"
)

// Users and roles (Explorer › connection › Users and roles…).

// ListUsers lists the users of a database, or of every database when db is "".
func (a *App) ListUsers(connID, db string) ([]users.User, error) {
	return call(a, "ListUsers", "", func(ctx context.Context) ([]users.User, error) {
		return a.users.Users(ctx, connID, db)
	})
}

// ListRoles lists a database's custom roles, and its built-in ones when
// builtin is set, with their privileges.
func (a *App) ListRoles(connID, db string, builtin bool) ([]users.RoleInfo, error) {
	return call(a, "ListRoles", "", func(ctx context.Context) ([]users.RoleInfo, error) {
		return a.users.Roles(ctx, connID, db, builtin)
	})
}

// UserEffective reports every role and privilege a user has, inherited ones
// included.
func (a *App) UserEffective(connID, db, user string) (users.Effective, error) {
	return call(a, "UserEffective", "", func(ctx context.Context) (users.Effective, error) {
		return a.users.EffectiveOf(ctx, connID, db, user)
	})
}

// CreateUser creates a user. The password goes to the server only.
func (a *App) CreateUser(u users.NewUser) error {
	_, err := call(a, "CreateUser", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.CreateUser(ctx, u)
	})
	if err == nil {
		a.log.Info("user created", "db", u.DB, "user", u.User)
	}
	return err
}

// SetUserPassword changes a user's password.
func (a *App) SetUserPassword(connID, db, user, password string) error {
	_, err := call(a, "SetUserPassword", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.SetPassword(ctx, connID, db, user, password)
	})
	if err == nil {
		a.log.Info("user password changed", "db", db, "user", user)
	}
	return err
}

// SetUserRoles makes roles the user's direct roles (grants and revokes the
// difference). Call CheckUserChange first to show any warnings.
func (a *App) SetUserRoles(connID, db, user string, roles []users.Role) error {
	_, err := call(a, "SetUserRoles", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.SetRoles(ctx, connID, db, user, roles)
	})
	if err == nil {
		a.log.Info("user roles changed", "db", db, "user", user, "roles", len(roles))
	}
	return err
}

// DropUser deletes a user; confirm must repeat the user name.
func (a *App) DropUser(connID, db, user, confirm string) error {
	_, err := call(a, "DropUser", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.DropUser(ctx, connID, db, user, confirm)
	})
	if err == nil {
		a.log.Info("user dropped", "db", db, "user", user)
	}
	return err
}

// CheckUserChange warns when a change takes away your own right to manage
// users, or removes the last user that has it.
func (a *App) CheckUserChange(c users.Change) ([]string, error) {
	return call(a, "CheckUserChange", "", func(ctx context.Context) ([]string, error) {
		w, err := a.users.Check(ctx, c)
		if w == nil {
			w = []string{}
		}
		return w, err
	})
}

// SaveRole creates a custom role (create) or replaces an existing one's
// privileges and inherited roles.
func (a *App) SaveRole(r users.RoleSpec, create bool) error {
	_, err := call(a, "SaveRole", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.SaveRole(ctx, r, create)
	})
	if err == nil {
		a.log.Info("role saved", "db", r.DB, "role", r.Role, "created", create)
	}
	return err
}

// DropRole deletes a custom role; confirm must repeat the role name.
func (a *App) DropRole(connID, db, role, confirm string) error {
	_, err := call(a, "DropRole", "", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, a.users.DropRole(ctx, connID, db, role, confirm)
	})
	if err == nil {
		a.log.Info("role dropped", "db", db, "role", role)
	}
	return err
}

// UserCan reports which of actions the user may not perform on db.coll
// (empty: all allowed), counting inherited roles.
func (a *App) UserCan(connID, db, user, targetDB, coll string, actions []string) ([]string, error) {
	return call(a, "UserCan", "", func(ctx context.Context) ([]string, error) {
		e, err := a.users.EffectiveOf(ctx, connID, db, user)
		if err != nil {
			return nil, err
		}
		missing := users.Can(e.Privileges, targetDB, coll, actions...)
		if missing == nil {
			missing = []string{}
		}
		return missing, nil
	})
}
