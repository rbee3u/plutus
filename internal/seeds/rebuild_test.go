package seeds

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Compare every edge-case report and point with the ordinary collection path.
// This includes several read chunks, stale sources outside the current chunk,
// scope cutover, missing prices, exclusion changes and boundary withdrawals.
func assertRebuildEquivalent(ctx context.Context, t *testing.T, db *sql.DB, v *valuation.Service) {
	t.Helper()
	p := PortfolioAliceEdge
	cycles, err := sqlcgen.New(db).ListSucceededCycleJobs(ctx, p[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) <= 128 {
		t.Fatal("fixture no longer crosses multiple rebuild chunks")
	}
	before := map[uuid.UUID]string{}
	for _, cycle := range cycles {
		id := uuid.Must(uuid.FromBytes(cycle.ID))
		before[id] = valuationSnapshot(ctx, t, v, p, id)
		point, err := v.PointByJob(ctx, p, id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.WritePoint(ctx, p, id, point.EvaluatedAt); err != nil {
			t.Fatal(err)
		}
		if got := valuationSnapshot(ctx, t, v, p, id); got != before[id] {
			t.Fatalf("batch and sequential valuation differ for %s\nexpected %s\nactual %s", id, got, before[id])
		}
	}
	// Recompute from source facts as well as upserting existing points.
	if _, err := db.ExecContext(ctx, "DELETE FROM valuation_points WHERE portfolio_id=?", p[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE account_reports SET method='missing', value_usd=NULL, source_report_id=NULL, omitted_json=NULL WHERE portfolio_id=?", p[:]); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		count, err := v.Rebuild(ctx, p)
		if err != nil || count != len(cycles) {
			t.Fatalf("rebuild count=%d error=%v", count, err)
		}
		for id, expected := range before {
			if got := valuationSnapshot(ctx, t, v, p, id); got != expected {
				t.Fatalf("rebuild changed %s\nexpected %s\nactual %s", id, expected, got)
			}
		}
	}
}

func valuationSnapshot(ctx context.Context, t *testing.T, v *valuation.Service, p, job uuid.UUID) string {
	t.Helper()
	point, err := v.PointByJob(ctx, p, job)
	if err != nil {
		t.Fatal(err)
	}
	point.ID, point.CreatedAt, point.RecalculatedAt = uuid.Nil, time.Time{}, nil
	for i := range point.BoundaryEvents {
		point.BoundaryEvents[i].PointID = uuid.Nil
	}
	accounts, err := v.Accounts(ctx, p, job)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(struct { //nolint:musttag // Internal models are serialized only for complete test snapshots, not an API contract.
		Point    valuation.Point              `json:"point"`
		Accounts []valuation.AccountValuation `json:"accounts"`
	}{point, accounts})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
