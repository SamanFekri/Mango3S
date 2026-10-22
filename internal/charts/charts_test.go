package charts

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"mango3s/internal/mql"
)

func pipeText(t *testing.T, filter bson.D, s Spec) string {
	t.Helper()
	p, err := Build(filter, s)
	if err != nil {
		t.Fatal(err)
	}
	a := bson.A{}
	for _, st := range p {
		a = append(a, st)
	}
	return mql.Compact(a)
}

func TestBuildGroupsOnTheServer(t *testing.T) {
	got := pipeText(t, bson.D{{Key: "status", Value: "A"}}, Spec{Kind: "bar", X: "city", Agg: "sum", Y: "total"})
	want := `[{"$match": {"status": "A"}}, {"$group": {"_id": {"x": "$city"}, "v": {"$sum": "$total"}}}, {"$sort": {"v": -1}}, {"$limit": 201}]`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestBuildTimeSeriesBySeries(t *testing.T) {
	got := pipeText(t, nil, Spec{Kind: "timeseries", X: "created", DateUnit: "week", Series: "status"})
	for _, want := range []string{
		`{"$match": {"created": {"$type": "date"}}}`,
		`{"$dateFromParts": {"isoWeekYear": {"$isoWeekYear": "$created"}, "isoWeek": {"$isoWeek": "$created"}}}`,
		`"s": "$status"`, `"v": {"$sum": 1}`, `{"$sort": {"_id.x": 1}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%s lacks %s", got, want)
		}
	}
}

func TestBuildScatterAndValidation(t *testing.T) {
	got := pipeText(t, nil, Spec{Kind: "scatter", X: "age", Y: "income", Limit: 10})
	if !strings.Contains(got, `{"$project": {"_id": 0, "x": "$age", "y": "$income"}}`) || !strings.Contains(got, `{"$limit": 11}`) {
		t.Fatal(got)
	}
	for _, bad := range []Spec{
		{Kind: "bar", X: ""},
		{Kind: "bar", X: "$where"},
		{Kind: "bar", X: "a", Agg: "avg"},
		{Kind: "bar", X: "a", Agg: "median", Y: "b"},
		{Kind: "line", X: "a", DateUnit: "fortnight"},
		{Kind: "scatter", X: "a"},
	} {
		if _, err := Build(nil, bad); err == nil {
			t.Errorf("%+v should be refused", bad)
		}
	}
}
