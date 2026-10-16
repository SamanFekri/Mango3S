package mongox

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/apperr"
)

// Actions used by Mango3S write operations.
const (
	ActionFind             = "find"
	ActionInsert           = "insert"
	ActionUpdate           = "update"
	ActionRemove           = "remove"
	ActionCreateCollection = "createCollection"
	ActionDropCollection   = "dropCollection"
)

// Privilege is one entry of connectionStatus.authInfo.authenticatedUserPrivileges.
type Privilege struct {
	Resource struct {
		DB          *string `bson:"db"`
		Collection  *string `bson:"collection"`
		Cluster     bool    `bson:"cluster"`
		AnyResource bool    `bson:"anyResource"`
	} `bson:"resource"`
	Actions []string `bson:"actions"`
}

type connStatus struct {
	AuthInfo struct {
		Users      []bson.D    `bson:"authenticatedUsers"`
		Privileges []Privilege `bson:"authenticatedUserPrivileges"`
	} `bson:"authInfo"`
}

// MissingActions returns the actions from want that privs do not grant on
// db.coll. It follows MongoDB's resource matching: {db: "", collection: ""}
// covers every non-system collection, {db: "x", collection: ""} every
// collection in x, {db: "", collection: "c"} collection c in any database.
func MissingActions(privs []Privilege, db, coll string, want ...string) []string {
	var missing []string
	for _, action := range want {
		if !granted(privs, db, coll, action) {
			missing = append(missing, action)
		}
	}
	return missing
}

func granted(privs []Privilege, db, coll, action string) bool {
	system := strings.HasPrefix(coll, "system.")
	for _, p := range privs {
		if !contains(p.Actions, action) {
			continue
		}
		r := p.Resource
		if r.AnyResource {
			return true
		}
		if r.DB == nil || r.Collection == nil {
			continue
		}
		dbOK := *r.DB == "" || *r.DB == db
		collOK := *r.Collection == coll || (*r.Collection == "" && !system)
		if dbOK && collOK {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// RequirePrivileges checks with connectionStatus that the connected user holds
// the given actions on db.coll, so a write fails before it starts rather than
// half way through. When the server runs without authentication (no
// authenticated users) the check passes: there is nothing to restrict.
func (s *Session) RequirePrivileges(ctx context.Context, db, coll string, actions ...string) error {
	var st connStatus
	cmd := bson.D{{Key: "connectionStatus", Value: 1}, {Key: "showPrivileges", Value: true}}
	if err := s.Client.Database("admin").RunCommand(ctx, cmd).Decode(&st); err != nil {
		return apperr.From(err)
	}
	if len(st.AuthInfo.Users) == 0 {
		return nil
	}
	if missing := MissingActions(st.AuthInfo.Privileges, db, coll, actions...); len(missing) > 0 {
		return apperr.New(apperr.CodeMongo, "The connected user lacks the %s privilege(s) on %s.%s.", strings.Join(missing, ", "), db, coll)
	}
	return nil
}

// RequireWrite combines the read-only profile check with the server-side privilege check.
func (s *Session) RequireWrite(ctx context.Context, db, coll string, actions ...string) error {
	if err := s.RequireWritable(); err != nil {
		return err
	}
	return s.RequirePrivileges(ctx, db, coll, actions...)
}
