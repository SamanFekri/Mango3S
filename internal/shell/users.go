package shell

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"mango3s/internal/apperr"
)

// userMethods are the mongosh user and role helpers, mapped onto their
// commands. Those that change something are rejected on read-only
// connections.
var userMethods = map[string]bool{
	"getUsers": false, "getUser": false, "getRoles": false, "getRole": false,
	"createUser": true, "updateUser": true, "changeUserPassword": true, "dropUser": true,
	"grantRolesToUser": true, "revokeRolesFromUser": true,
	"createRole": true, "updateRole": true, "dropRole": true,
	"grantRolesToRole": true, "revokeRolesFromRole": true,
	"grantPrivilegesToRole": true, "revokePrivilegesFromRole": true,
}

func (e *executor) userMethod(ctx context.Context, d *mongo.Database, m segment) (Output, error) {
	if userMethods[m.name] {
		if err := e.sess.RequireWritable(); err != nil {
			return Output{}, err
		}
	}
	switch m.name {
	case "getUsers":
		opts, err := docArg(m, 0, "options")
		if err != nil {
			return Output{}, err
		}
		return e.command(ctx, d, append(bson.D{{Key: "usersInfo", Value: 1}}, opts...))
	case "getRoles":
		opts, err := docArg(m, 0, "options")
		if err != nil {
			return Output{}, err
		}
		return e.command(ctx, d, append(bson.D{{Key: "rolesInfo", Value: 1}}, opts...))
	case "getUser", "getRole":
		name, err := argString(m, 0, "name")
		if err != nil {
			return Output{}, err
		}
		opts, err := docArg(m, 1, "options")
		if err != nil {
			return Output{}, err
		}
		cmd, list := "usersInfo", "users"
		if m.name == "getRole" {
			cmd, list = "rolesInfo", "roles"
		}
		var res bson.M
		if err := d.RunCommand(ctx, append(bson.D{{Key: cmd, Value: name}}, opts...)).Decode(&res); err != nil {
			return Output{}, err
		}
		if a, ok := res[list].(bson.A); ok && len(a) > 0 {
			return value(a[0]), nil
		}
		return text("null"), nil
	case "createUser", "createRole":
		key := "user"
		if m.name == "createRole" {
			key = "role"
		}
		doc, err := requiredDoc(m, 0, key+" document")
		if err != nil {
			return Output{}, err
		}
		name, rest, err := lift(doc, key)
		if err != nil {
			return Output{}, apperr.Invalid("%s(): %v", m.name, err)
		}
		if _, err := e.command(ctx, d, append(bson.D{{Key: m.name, Value: name}}, rest...)); err != nil {
			return Output{}, err
		}
		return value(bson.D{{Key: "ok", Value: 1}, {Key: key, Value: name}}), nil
	case "updateUser", "updateRole":
		name, err := argString(m, 0, "name")
		if err != nil {
			return Output{}, err
		}
		doc, err := requiredDoc(m, 1, "update document")
		if err != nil {
			return Output{}, err
		}
		return e.command(ctx, d, append(bson.D{{Key: m.name, Value: name}}, doc...))
	case "changeUserPassword":
		name, err := argString(m, 0, "user name")
		if err != nil {
			return Output{}, err
		}
		pwd, err := argString(m, 1, "password")
		if err != nil {
			return Output{}, err
		}
		return e.command(ctx, d, bson.D{{Key: "updateUser", Value: name}, {Key: "pwd", Value: pwd}})
	case "dropUser", "dropRole":
		name, err := argString(m, 0, "name")
		if err != nil {
			return Output{}, err
		}
		return e.command(ctx, d, bson.D{{Key: m.name, Value: name}})
	case "grantRolesToUser", "revokeRolesFromUser", "grantRolesToRole", "revokeRolesFromRole":
		return e.nameAndList(ctx, d, m, "roles")
	case "grantPrivilegesToRole", "revokePrivilegesFromRole":
		return e.nameAndList(ctx, d, m, "privileges")
	}
	return Output{}, apperr.New(apperr.CodeUnsupported, "db.%s() is not supported (try help).", m.name)
}

// nameAndList runs {<method>: name, <field>: [...]}, e.g. grantRolesToUser.
func (e *executor) nameAndList(ctx context.Context, d *mongo.Database, m segment, field string) (Output, error) {
	name, err := argString(m, 0, "name")
	if err != nil {
		return Output{}, err
	}
	list, ok := argArray(m, 1)
	if !ok {
		return Output{}, apperr.Invalid("%s() expects an array of %s as argument 2.", m.name, field)
	}
	return e.command(ctx, d, bson.D{{Key: m.name, Value: name}, {Key: field, Value: list}})
}

// lift takes the string field key out of doc: {user: "a", pwd: …} becomes
// "a" and {pwd: …}.
func lift(doc bson.D, key string) (string, bson.D, error) {
	var name string
	rest := bson.D{}
	for _, e := range doc {
		if e.Key == key {
			s, ok := e.Value.(string)
			if !ok || s == "" {
				return "", nil, apperr.Invalid("%q must be a non-empty string", key)
			}
			name = s
			continue
		}
		rest = append(rest, e)
	}
	if name == "" {
		return "", nil, apperr.Invalid("the document needs a %q field", key)
	}
	return name, rest, nil
}
