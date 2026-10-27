package users

import (
	"slices"
	"testing"

	"mango3s/internal/mql"
)

func TestDiffRoles(t *testing.T) {
	cur := []Role{{"read", "shop"}, {"readWrite", "logs"}}
	want := []Role{{"readWrite", "logs"}, {"dbAdmin", "shop"}, {"dbAdmin", "shop"}}
	grant, revoke := diffRoles(cur, want)
	if !slices.Equal(grant, []Role{{"dbAdmin", "shop"}}) || !slices.Equal(revoke, []Role{{"read", "shop"}}) {
		t.Fatalf("grant %v, revoke %v", grant, revoke)
	}
	if g, r := diffRoles(cur, cur); len(g) != 0 || len(r) != 0 {
		t.Fatalf("no change: grant %v, revoke %v", g, r)
	}
}

func TestIsAdmin(t *testing.T) {
	for _, c := range []struct {
		roles []Role
		want  bool
	}{
		{[]Role{{"root", "admin"}}, true},
		{[]Role{{"read", "shop"}, {"userAdminAnyDatabase", "admin"}}, true},
		{[]Role{{"userAdmin", "admin"}}, true},
		{[]Role{{"userAdmin", "shop"}}, false}, // only that database's users
		{[]Role{{"readWriteAnyDatabase", "admin"}}, false},
		{nil, false},
	} {
		if got := isAdmin(c.roles); got != c.want {
			t.Errorf("isAdmin(%v) = %v", c.roles, got)
		}
	}
}

func TestResourceArg(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, c := range []struct {
		r    Resource
		want string
	}{
		{Resource{DB: s("shop"), Collection: s("orders")}, `{"db": "shop", "collection": "orders"}`},
		{Resource{DB: s("shop")}, `{"db": "shop", "collection": ""}`},
		{Resource{}, `{"db": "", "collection": ""}`},
		{Resource{Cluster: true, DB: s("x")}, `{"cluster": true}`},
		{Resource{AnyResource: true}, `{"anyResource": true}`},
	} {
		if got := mql.Compact(resourceArg(c.r)); got != c.want {
			t.Errorf("resourceArg(%+v) = %s, want %s", c.r, got, c.want)
		}
	}
}

func TestCan(t *testing.T) {
	s := func(v string) *string { return &v }
	privs := []Privilege{
		{Resource: Resource{DB: s("shop"), Collection: s("")}, Actions: []string{"find", "insert"}},
		{Resource: Resource{DB: s(""), Collection: s("logs")}, Actions: []string{"remove"}},
	}
	if m := Can(privs, "shop", "orders", "find", "insert"); len(m) != 0 {
		t.Fatalf("shop.orders: missing %v", m)
	}
	if m := Can(privs, "shop", "orders", "update"); !slices.Equal(m, []string{"update"}) {
		t.Fatalf("update: %v", m)
	}
	if m := Can(privs, "other", "logs", "remove"); len(m) != 0 {
		t.Fatalf("logs in any db: %v", m)
	}
	if m := Can(privs, "shop", "system.users", "find"); len(m) == 0 {
		t.Fatal("system collections need their own grant")
	}
}
