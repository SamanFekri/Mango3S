package mongox

import (
	"reflect"
	"testing"
)

func priv(db, coll string, any bool, actions ...string) Privilege {
	var p Privilege
	if any {
		p.Resource.AnyResource = true
	} else {
		p.Resource.DB, p.Resource.Collection = &db, &coll
	}
	p.Actions = actions
	return p
}

func TestMissingActions(t *testing.T) {
	privs := []Privilege{
		priv("shop", "orders", false, "find", "insert"),
		priv("shop", "", false, "find"),
		priv("", "audit", false, "insert"),
	}
	cases := []struct {
		db, coll string
		want     []string
		missing  []string
	}{
		{"shop", "orders", []string{"find", "insert", "update"}, []string{"update"}},
		{"shop", "customers", []string{"find"}, nil},
		{"shop", "customers", []string{"insert"}, []string{"insert"}},
		{"other", "audit", []string{"insert"}, nil},
		{"shop", "system.users", []string{"find"}, []string{"find"}}, // db-wide grants exclude system collections
	}
	for _, c := range cases {
		got := MissingActions(privs, c.db, c.coll, c.want...)
		if !reflect.DeepEqual(got, c.missing) {
			t.Errorf("%s.%s %v: got %v want %v", c.db, c.coll, c.want, got, c.missing)
		}
	}
	if m := MissingActions([]Privilege{priv("", "", true, "remove")}, "a", "b", "remove"); m != nil {
		t.Fatalf("anyResource: %v", m)
	}
	if m := MissingActions([]Privilege{priv("", "", false, "remove")}, "a", "b", "remove"); m != nil {
		t.Fatalf("all-databases grant: %v", m)
	}
}
