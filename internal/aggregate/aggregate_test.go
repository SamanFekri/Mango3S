package aggregate

import (
	"strings"
	"testing"

	"mango3s/internal/apperr"
	"mango3s/internal/mql"
)

func TestBuild(t *testing.T) {
	stages := []Stage{
		{Operator: "$match", Body: `{status: "A"}`, Enabled: true},
		{Operator: "$sort", Body: `{n: -1}`, Enabled: false},
		{Operator: "$limit", Body: `5`, Enabled: true},
		{Operator: "$project", Body: `{_id: 0}`, Enabled: true},
	}
	p, err := Build(stages, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 3 || p[0][0].Key != "$match" || p[1][0].Key != "$limit" || p[2][0].Key != "$project" {
		t.Fatalf("pipeline: %s", mql.Compact(p))
	}
	p, err = Build(stages, 1)
	if err != nil || len(p) != 1 {
		t.Fatalf("preview up to stage 2: %v %v", p, err)
	}
}

func TestBuildErrors(t *testing.T) {
	cases := []struct {
		stages []Stage
		want   string
	}{
		{[]Stage{{Operator: "match", Body: "{}", Enabled: true}}, "must start with '$'"},
		{[]Stage{{Operator: "$match", Body: " ", Enabled: true}}, "body is empty"},
		{[]Stage{{Operator: "$match", Body: "{a:", Enabled: true}}, "Stage 1 ($match)"},
		{[]Stage{{Operator: "$out", Body: `"x"`, Enabled: true}, {Operator: "$match", Body: "{}", Enabled: true}}, "must be the last stage"},
	}
	for _, c := range cases {
		_, err := Build(c.stages, -1)
		if !apperr.Is(err, apperr.CodeInvalidInput) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want %q, got %v", c.want, err)
		}
	}
	// $out followed only by a disabled stage is fine.
	p, err := Build([]Stage{{Operator: "$out", Body: `"x"`, Enabled: true}, {Operator: "$match", Body: "{}", Enabled: false}}, -1)
	if err != nil || !IsWrite(p) {
		t.Fatalf("trailing disabled stage: %v", err)
	}
}

func TestTextRoundTrip(t *testing.T) {
	stages := []Stage{
		{Operator: "$match", Body: `{n: {$gt: 1}}`, Enabled: true},
		{Operator: "$limit", Body: `10`, Enabled: true},
	}
	text, err := ToText(stages)
	if err != nil {
		t.Fatal(err)
	}
	back, err := FromText(text)
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	if len(back) != 2 || back[0].Operator != "$match" || back[1].Body != "10" || !back[0].Enabled {
		t.Fatalf("round trip: %+v", back)
	}

	// Disabled stages are emitted as comments and therefore dropped on re-import.
	stages[1].Enabled = false
	text, _ = ToText(stages)
	if !strings.Contains(text, "// ") {
		t.Fatalf("disabled stage not commented:\n%s", text)
	}
	back, err = FromText(text)
	if err != nil || len(back) != 1 {
		t.Fatalf("commented stage: %+v %v\n%s", back, err, text)
	}

	if _, err := FromText(`[{a: 1}]`); err == nil {
		t.Fatal("non-operator stage accepted")
	}
	if _, err := FromText(`[{$match: {}, $limit: 1}]`); err == nil {
		t.Fatal("two-operator stage accepted")
	}
	if one, err := FromText(`{$count: "n"}`); err != nil || len(one) != 1 {
		t.Fatalf("single stage: %v", err)
	}
}
