//go:build integration

package integration

import (
	"testing"

	"mango3s/internal/charts"
)

// Charts group on the server, over every matching document.
func TestChartsOnRealData(t *testing.T) {
	e := setup(t)
	seedDocs(t, e, "orders", representative(1200))
	svc := charts.New(e.mgr)
	run := func(filter string, s charts.Spec) charts.Data {
		t.Helper()
		d, err := svc.Run(e.ctx, charts.Request{ConnID: e.conn, DB: e.db, Coll: "orders", Filter: filter, Spec: s})
		if err != nil {
			t.Fatalf("%+v: %v", s, err)
		}
		return d
	}

	// Count per status: 4 statuses × 300.
	d := run("", charts.Spec{Kind: "pie", X: "status"})
	if d.XKind != "category" || len(d.Points) != 4 {
		t.Fatalf("pie: %+v", d)
	}
	total := 0.0
	for _, p := range d.Points {
		total += p.Y
	}
	if total != 1200 {
		t.Fatalf("counts add up to %v", total)
	}

	// A filter narrows it; sum of qty over nested-field groups.
	d = run(`{status: "shipped"}`, charts.Spec{Kind: "bar", X: "customer.city", Agg: "sum", Y: "qty"})
	if len(d.Points) != 3 {
		t.Fatalf("bar: %+v", d)
	}

	// Time series per day (orders are hourly from 2024-01-01) split by status.
	d = run("", charts.Spec{Kind: "timeseries", X: "createdAt", DateUnit: "day", Series: "status"})
	if d.XKind != "time" || len(d.Points) != 50*4 {
		t.Fatalf("time series: kind %s, %d points", d.XKind, len(d.Points))
	}
	if first := d.Points[0]; first.X.(float64) != 1704067200000 || first.Series == "" {
		t.Fatalf("first day: %+v", first)
	}

	// Averages of a decimal, and the group limit.
	d = run("", charts.Spec{Kind: "line", X: "qty", Agg: "avg", Y: "price", Limit: 3})
	if d.XKind != "number" || len(d.Points) != 3 || !d.Truncated || d.Points[0].Y <= 0 {
		t.Fatalf("avg with limit: %+v", d)
	}

	// Scatter: raw points.
	d = run("", charts.Spec{Kind: "scatter", X: "rating", Y: "qty", Limit: 100})
	if len(d.Points) != 100 || !d.Truncated {
		t.Fatalf("scatter: %d points, truncated %v", len(d.Points), d.Truncated)
	}
}
