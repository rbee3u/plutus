package seed

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

// Compare initialized edge-case history with sequential WritePoint on the same
// source facts. It crosses several history batches with stale sources outside
// the current batch, scope cutover, missing prices, exclusion and treatment
// changes, and boundary withdrawals.
func assertSequentialHistory(ctx context.Context, t *testing.T, initialized *sql.DB, seed Seed) {
	t.Helper()
	database, err := mysqltest.CreateTestDatabase(ctx, mysqltest.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Drop(context.WithoutCancel(ctx)); err != nil {
			t.Error(err)
		}
	})
	tx, err := database.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := mysql.ExecSQLStatements(ctx, tx, seed.SQL); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	p := PortfolioAliceEdge
	cycles, err := sqlcgen.New(database.DB).ListSucceededCycles(ctx, p[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) <= 128 {
		t.Fatal("fixture no longer crosses multiple history batches")
	}
	points := initializedTimeline(ctx, t, initialized, p)
	if len(points) != len(cycles) {
		t.Fatalf("initialized %d points for %d succeeded cycles", len(points), len(cycles))
	}
	sequential := valuation.NewService(database.DB)
	for _, point := range points {
		if _, err := sequential.WritePoint(ctx, p, point.cycleID, point.evaluatedAt); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"valuation_points", "account_valuations", "balance_valuations", "boundary_events", "manual_balance_changes", "calculated_asset_changes"} {
		if want, got := tableRows(ctx, t, database.DB, table, p), tableRows(ctx, t, initialized, table, p); !slices.Equal(got, want) {
			t.Fatalf("%s differs from sequential WritePoint: %d initialized rows, %d sequential rows", table, len(got), len(want))
		}
	}
}

type timelinePoint struct {
	cycleID     uuid.UUID
	evaluatedAt time.Time
}

func initializedTimeline(ctx context.Context, t *testing.T, db *sql.DB, p uuid.UUID) []timelinePoint {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT cycle_id, evaluated_at FROM valuation_points WHERE portfolio_id = ? ORDER BY evaluated_at, cycle_id", p[:])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	out := []timelinePoint{}
	for rows.Next() {
		var id []byte
		var point timelinePoint
		if err := rows.Scan(&id, &point.evaluatedAt); err != nil {
			t.Fatal(err)
		}
		point.cycleID = uuid.Must(uuid.FromBytes(id))
		out = append(out, point)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func tableRows(ctx context.Context, t *testing.T, db *sql.DB, table string, p uuid.UUID) []string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+table+" WHERE portfolio_id = ?", p[:]) // #nosec G202 -- Callers pass fixed table names.
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		fields := make([]string, len(values))
		for i, value := range values {
			fields[i] = "NULL"
			if value != nil {
				fields[i] = hex.EncodeToString(value)
			}
		}
		out = append(out, strings.Join(fields, ","))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

// Retrying an existing point returns it unchanged.
func assertImmutableRetries(ctx context.Context, t *testing.T, db *sql.DB, v *valuation.Service) {
	t.Helper()
	p := PortfolioAliceEdge
	cycles, err := sqlcgen.New(db).ListSucceededCycles(ctx, p[:])
	if err != nil {
		t.Fatal(err)
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
