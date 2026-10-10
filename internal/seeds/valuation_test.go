package seeds

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Compare every edge-case report and point with the ordinary collection path.
// This includes several read chunks, stale sources outside the current chunk,
// scope cutover, missing prices, exclusion changes and boundary withdrawals.
func assertImmutableRetries(ctx context.Context, t *testing.T, db *sql.DB, v *valuation.Service) {
	t.Helper()
	p := PortfolioAliceEdge
	cycles, err := sqlcgen.New(db).ListSucceededCycles(ctx, p[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) <= 128 {
		t.Fatal("fixture no longer crosses multiple read batches")
	}
	before := map[uuid.UUID]string{}
	for _, cycle := range cycles {
		id := uuid.Must(uuid.FromBytes(cycle.ID))
		before[id] = valuationSnapshot(ctx, t, v, p, id)
		point, err := v.PointByCycle(ctx, p, id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.WritePoint(ctx, p, id, point.EvaluatedAt); err != nil {
			t.Fatal(err)
		}
		if got := valuationSnapshot(ctx, t, v, p, id); got != before[id] {
			t.Fatalf("sequential revaluation changed %s\nexpected %s\nactual %s", id, got, before[id])
		}
	}
}

func valuationSnapshot(ctx context.Context, t *testing.T, v *valuation.Service, p, cycleID uuid.UUID) string {
	t.Helper()
	point, err := v.PointByCycle(ctx, p, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	point.CycleID = uuid.Nil
	for i := range point.BoundaryEvents {
		point.BoundaryEvents[i].CycleID = uuid.Nil
	}
	accounts, err := v.Accounts(ctx, p, cycleID)
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
