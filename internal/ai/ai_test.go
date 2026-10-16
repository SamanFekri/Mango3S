package ai

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
	"mango3s/internal/schema"
)

func answerJSON(t *testing.T, a map[string]any) string {
	t.Helper()
	base := map[string]any{"kind": "find", "filter": "", "projection": "", "sort": "", "limit": 0, "pipeline": []any{}, "update": "", "explanation": "x", "warnings": []string{}}
	for k, v := range a {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return string(b)
}

func checkNames(p *Proposal, ok bool) []string {
	var out []string
	for _, c := range p.Validation {
		if c.OK == ok {
			out = append(out, c.Name)
		}
	}
	return out
}

func TestValidateRead(t *testing.T) {
	req := Request{DB: "shop", Coll: "orders"}
	p, err := Validate(answerJSON(t, map[string]any{"filter": `{ status: "shipped", total: { $gt: 100 } }`, "sort": "{ createdAt: -1 }", "limit": 10}), req)
	if err != nil || !p.Valid || p.IsWrite {
		t.Fatalf("valid find: %+v %v", p, err)
	}
	if p.Shell != `db.orders.find({ status: "shipped", total: { $gt: 100 } }).sort({ createdAt: -1 }).limit(10)` {
		t.Fatalf("shell: %s", p.Shell)
	}
	p, _ = Validate(answerJSON(t, map[string]any{"kind": "aggregate", "pipeline": []map[string]string{
		{"operator": "$match", "body": `{ status: "A" }`}, {"operator": "$group", "body": `{ _id: "$city", n: { $sum: 1 } }`}}}), req)
	if !p.Valid || len(p.Pipeline) != 2 || !strings.Contains(p.Shell, "$group") {
		t.Fatalf("aggregate: %+v", p)
	}
}

func TestValidateRejectsBadSyntaxAndShapes(t *testing.T) {
	req := Request{DB: "d", Coll: "c"}
	cases := []map[string]any{
		{"filter": `{ status: "A" `},
		{"sort": `{ a: }`},
		{"kind": "aggregate", "pipeline": []map[string]string{}},
		{"kind": "aggregate", "pipeline": []map[string]string{{"operator": "match", "body": "{}"}}},
		{"kind": "aggregate", "pipeline": []map[string]string{{"operator": "$out", "body": `"x"`}, {"operator": "$match", "body": "{}"}}},
		{"kind": "updateMany", "filter": "{}", "update": `{ name: "x" }`}, // replacement, not operators
	}
	for i, c := range cases {
		p, err := Validate(answerJSON(t, c), Request{DB: req.DB, Coll: req.Coll, AllowWrites: true})
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if p.Valid {
			t.Errorf("case %d accepted: %+v", i, p.Validation)
		}
	}
	if _, err := Validate("not json", req); err == nil {
		t.Fatal("non-JSON accepted")
	}
	if _, err := Validate(answerJSON(t, map[string]any{"kind": "dropDatabase"}), req); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestWritesNeedPermissionAndConfirmation(t *testing.T) {
	upd := answerJSON(t, map[string]any{"kind": "updateMany", "filter": `{ status: "old" }`, "update": `{ $set: { status: "archived" } }`})
	p, _ := Validate(upd, Request{DB: "d", Coll: "c"})
	if p.Valid || !p.IsWrite || !contains(checkNames(p, false), "Read-only policy") {
		t.Fatalf("write without permission must fail validation: %+v", p.Validation)
	}
	p, _ = Validate(upd, Request{DB: "d", Coll: "c", AllowWrites: true})
	if !p.Valid || !p.RequiresConfirmation {
		t.Fatalf("permitted write: %+v", p)
	}
	del, _ := Validate(answerJSON(t, map[string]any{"kind": "deleteMany", "filter": "{}"}), Request{DB: "d", Coll: "c", AllowWrites: true})
	if !del.RequiresConfirmation || len(del.Warnings) == 0 {
		t.Fatalf("empty-filter delete must warn: %+v", del)
	}
	out, _ := Validate(answerJSON(t, map[string]any{"kind": "aggregate", "pipeline": []map[string]string{{"operator": "$match", "body": "{}"}, {"operator": "$merge", "body": `{ into: "x" }`}}}), Request{DB: "d", Coll: "c"})
	if out.Valid || !out.IsWrite {
		t.Fatal("$merge pipeline must count as a write")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestContextNeverLeaksValues(t *testing.T) {
	a := schema.NewAnalyzer()
	secret := bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "email", Value: "ada.lovelace@example.com"}, {Key: "salary", Value: int64(185000)}}
	a.Add(secret)
	c := BuildContext("people", a.Report(), nil)
	msg := UserMessage(Request{Prompt: "high earners"}, c)
	if strings.Contains(msg, "ada.lovelace") || strings.Contains(msg, "185000") {
		t.Fatalf("schema summary leaked values:\n%s", msg)
	}
	if !strings.Contains(msg, "email: string") || !strings.Contains(msg, "Only read queries") {
		t.Fatalf("summary missing fields or read-only instruction:\n%s", msg)
	}
	// With samples: values are masked but structure and types survive.
	rng := rand.New(rand.NewPCG(1, 2))
	masked := MaskShape(secret, rng).(bson.D)
	text := mql.Compact(masked)
	if strings.Contains(text, "ada.lovelace") || strings.Contains(text, "185000") {
		t.Fatalf("sample leaked values: %s", text)
	}
	email, _ := mql.Lookup(masked, "email")
	if s := email.(string); len(s) != len("ada.lovelace@example.com") || s[12] != '@' {
		t.Fatalf("shape lost: %q", s)
	}
	if v, _ := mql.Lookup(masked, "salary"); mql.TypeName(v) != "int64" {
		t.Fatal("type lost")
	}
}

func TestSystemPromptHasNoConnectionDetails(t *testing.T) {
	for _, bad := range []string{"mongodb://", "password", "localhost"} {
		if strings.Contains(strings.ToLower(SystemPrompt), bad) {
			t.Fatalf("system prompt mentions %q", bad)
		}
	}
	if s := AnswerSchema(); s["additionalProperties"] != false {
		t.Fatal("structured output schema must be closed")
	}
}
