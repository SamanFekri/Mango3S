package masking

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

var key = []byte("0123456789abcdef0123456789abcdef")

func doc(t *testing.T, s string) bson.D {
	t.Helper()
	d, err := mql.ParseDocument(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

const person = `{
  _id: 1,
  name: "Ada Lovelace",
  email: "ada.lovelace@example.com",
  ssn: "123-45-6789",
  card: "4111 1111 1111 1234",
  phones: ["+44 20 7946 0958", "+44 20 7946 0001"],
  salary: NumberLong(185000),
  bonus: 1234.5,
  born: ISODate("1815-12-10T00:00:00Z"),
  address: { street: "12 St James's Square", city: "London" },
  contacts: [ { name: "Charles Babbage", email: "charles@example.org" } ],
  notes: "VIP"
}`

// sensitive are the raw values that must not survive masking.
var sensitive = []string{"Ada Lovelace", "ada.lovelace", "123-45-6789", "4111 1111 1111", "7946 0958", "7946 0001",
	"12 St James", "Charles Babbage", "charles@", "VIP"}

func rules() []Rule {
	return []Rule{
		{Field: "name", Method: MethodRandomize},
		{Field: "email", Method: MethodEmail},
		{Field: "ssn", Method: MethodRedact},
		{Field: "card", Method: MethodPartial, KeepLast: 4},
		{Field: "phones", Method: MethodRandomize},
		{Field: "salary", Method: MethodNoise, Percent: 10},
		{Field: "bonus", Method: MethodNull},
		{Field: "born", Method: MethodDateShift, MaxDays: 30},
		{Field: "address.street", Method: MethodHash},
		{Field: "contacts.name", Method: MethodConstant, Value: `"Contact"`},
		{Field: "contacts.email", Method: MethodEmail},
		{Field: "notes", Method: MethodRemove},
	}
}

func TestSensitiveValuesAreProtected(t *testing.T) {
	m, err := Compile(rules(), key)
	if err != nil {
		t.Fatal(err)
	}
	in := doc(t, person)
	out, changed, err := m.Apply(in)
	if err != nil || !changed {
		t.Fatalf("apply: %v %v", changed, err)
	}
	raw, _ := bson.Marshal(out)
	text := mql.Compact(out)
	for _, s := range sensitive {
		if bytes.Contains(raw, []byte(s)) || strings.Contains(text, s) {
			t.Errorf("sensitive value %q survived masking:\n%s", s, text)
		}
	}
	// The input is untouched.
	if !strings.Contains(mql.Compact(in), "Ada Lovelace") {
		t.Fatal("input mutated")
	}
	get := func(k string) any { v, _ := mql.Lookup(out, k); return v }
	if v := get("card").(string); !strings.HasSuffix(v, "1234") || len([]rune(v)) != 19 {
		t.Errorf("partial: %q", v)
	}
	if v := get("email").(string); !strings.HasSuffix(v, "@example.com") || !strings.HasPrefix(v, "u_") {
		t.Errorf("email: %q", v)
	}
	if v := get("name").(string); len(v) != len("Ada Lovelace") || v[3] != ' ' || v[0] < 'A' || v[0] > 'Z' {
		t.Errorf("randomize keeps shape: %q", v)
	}
	phones := get("phones").(bson.A)
	if p := phones[0].(string); len(p) != len("+44 20 7946 0958") || p[0] != '+' || p[3] != ' ' {
		t.Errorf("phone shape: %q", p)
	}
	if s := get("salary").(int64); s < 166500 || s > 203500 || s == 185000 {
		t.Errorf("noise out of range or unchanged: %d", s)
	}
	if get("bonus") != nil {
		t.Error("null")
	}
	shift := get("born").(bson.DateTime).Time().Sub(time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC))
	if shift == 0 || shift > 30*24*time.Hour || shift < -30*24*time.Hour {
		t.Errorf("date shift %v", shift)
	}
	if _, ok := mql.Lookup(out, "notes"); ok {
		t.Error("remove")
	}
	if city, _ := mql.Lookup(get("address").(bson.D), "city"); city != "London" {
		t.Error("unmasked sibling field changed")
	}
}

func TestDeterministicAndKeyed(t *testing.T) {
	m1, _ := Compile(rules(), key)
	m2, _ := Compile(rules(), key)
	other, _ := Compile(rules(), []byte("ffffffffffffffffffffffffffffffff"))
	a, _, _ := m1.Apply(doc(t, person))
	b, _, _ := m2.Apply(doc(t, person))
	c, _, _ := other.Apply(doc(t, person))
	if !mql.Equal(a, b) {
		t.Fatal("same key must give same output")
	}
	if mql.Equal(a, c) {
		t.Fatal("different key must give different output")
	}
	// Hash is consistent across fields/collections so joins survive.
	h1, _ := Compile([]Rule{{Field: "a", Method: MethodHash}}, key)
	h2, _ := Compile([]Rule{{Field: "b", Method: MethodHash}}, key)
	x, _, _ := h1.Apply(doc(t, `{a: "same"}`))
	y, _, _ := h2.Apply(doc(t, `{b: "same"}`))
	if xv, _ := mql.Lookup(x, "a"); xv != func() any { v, _ := mql.Lookup(y, "b"); return v }() {
		t.Fatal("hash differs across fields")
	}
}

func TestUnsupportedTypes(t *testing.T) {
	m, _ := Compile([]Rule{{Field: "n", Method: MethodPartial}}, key)
	if _, _, err := m.Apply(doc(t, `{n: 5}`)); err == nil || !strings.Contains(err.Error(), "does not support int32") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
	m, _ = Compile([]Rule{{Field: "n", Method: MethodPartial, OnUnsupported: "null"}}, key)
	if out, _, err := m.Apply(doc(t, `{n: 5}`)); err != nil || mql.Compact(out) != `{"n": null}` {
		t.Fatalf("null policy: %s %v", mql.Compact(out), err)
	}
	m, _ = Compile([]Rule{{Field: "n", Method: MethodDateShift, OnUnsupported: "skip"}}, key)
	if out, changed, _ := m.Apply(doc(t, `{n: "text"}`)); changed || mql.Compact(out) != `{"n": "text"}` {
		t.Fatalf("skip policy: %s", mql.Compact(out))
	}
	m, _ = Compile([]Rule{{Field: "e", Method: MethodEmail}}, key)
	if _, _, err := m.Apply(doc(t, `{e: "not-an-email"}`)); err == nil {
		t.Fatal("bad email accepted")
	}
	// Missing fields and nulls are fine.
	if out, changed, err := m.Apply(doc(t, `{x: 1, e: null}`)); err != nil || changed || mql.Compact(out) != `{"x": 1, "e": null}` {
		t.Fatalf("missing/null: %s %v", mql.Compact(out), err)
	}
}

func TestCompileValidation(t *testing.T) {
	bad := [][]Rule{
		{},
		{{Field: "", Method: MethodNull}},
		{{Field: "a", Method: "shred"}},
		{{Field: "_id", Method: MethodNull}},
		{{Field: "a", Method: MethodNull}, {Field: "a", Method: MethodHash}},
		{{Field: "a", Method: MethodConstant, Value: "{"}},
		{{Field: "a", Method: MethodPartial, KeepLast: 20}},
		{{Field: "a", Method: MethodNull, OnUnsupported: "ignore"}},
	}
	for _, r := range bad {
		if _, err := Compile(r, key); err == nil {
			t.Errorf("accepted %+v", r)
		}
	}
	if _, err := Compile([]Rule{{Field: "a", Method: MethodNull}}, []byte("short")); err == nil {
		t.Error("short key accepted")
	}
	if _, err := Compile([]Rule{{Field: "_id", Method: MethodHash}}, key); err != nil {
		t.Errorf("_id hash rejected: %v", err)
	}
}

func TestPartialNeverRevealsMoreThanHalf(t *testing.T) {
	m, _ := Compile([]Rule{{Field: "s", Method: MethodPartial, KeepLast: 4}}, key)
	out, _, _ := m.Apply(doc(t, `{s: "abc"}`))
	if v, _ := mql.Lookup(out, "s"); v != "**c" {
		t.Fatalf("got %v", v)
	}
}

func TestRandomizeIntKeepsDigits(t *testing.T) {
	m, _ := Compile([]Rule{{Field: "n", Method: MethodRandomize}}, key)
	for _, src := range []string{`{n: 4711}`, `{n: -52}`, `{n: 7}`, `{n: NumberLong("123456789012")}`} {
		in := doc(t, src)
		out, _, err := m.Apply(in)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := mql.Lookup(in, "n")
		b, _ := mql.Lookup(out, "n")
		if mql.TypeName(a) != mql.TypeName(b) || len(mql.Compact(a)) != len(mql.Compact(b)) || mql.Equal(a, b) {
			t.Errorf("%s -> %s", mql.Compact(a), mql.Compact(b))
		}
	}
}

func TestRuleSetStore(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs, err := s.Save(RuleSet{Name: "PII", Rules: []Rule{{Field: "email", Method: MethodEmail}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(RuleSet{Name: "bad", Rules: []Rule{{Field: "x", Method: "nope"}}}); err == nil {
		t.Fatal("invalid rules saved")
	}
	got, err := NewStore(dir).Get(rs.ID)
	if err != nil || got.Rules[0].Method != MethodEmail {
		t.Fatalf("reload: %+v %v", got, err)
	}
	if err := s.Delete(rs.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatal("delete")
	}
}

func TestCompileDoesNotMutateInput(t *testing.T) {
	in := []Rule{{Field: "d", Method: MethodDateShift}}
	if _, err := Compile(in, key); err != nil {
		t.Fatal(err)
	}
	if in[0].MaxDays != 0 {
		t.Fatal("Compile changed the caller's rules")
	}
}
