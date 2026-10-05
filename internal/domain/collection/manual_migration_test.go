package collection

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestManualMigrationBackfillsWithoutChangingHistory(t *testing.T) {
	db, err := mysql.CreateTestDatabase(context.Background(), mysql.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	h := newHarness(t, db.DB)
	a := &manualTestAdapter{usd: "30"}
	item := manualEnabled(t, h, a)
	initial, job := h.runCycle()
	// Remove only the empty application-owned report/account to represent the
	// populated schema immediately before Manual was introduced.
	if _, err := db.DB.ExecContext(h.ctx, "UPDATE account_reports r JOIN accounts a ON a.id=r.account_id SET r.source_report_id=NULL, r.method='missing', r.value_usd=NULL WHERE a.identity_id=? AND a.account_type='manual'", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(h.ctx, "DELETE r FROM account_reports r JOIN accounts a ON a.id=r.account_id WHERE a.identity_id=? AND a.account_type='manual'", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(db.DB, ".", 20261005010000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(h.ctx, "DELETE FROM accounts WHERE identity_id=? AND account_type='manual'", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(h.ctx, "UPDATE valuation_points SET account_count=1 WHERE job_id=?", job[:]); err != nil {
		t.Fatal(err)
	}
	var beforeReport, beforePoint string
	if err := db.DB.QueryRowContext(h.ctx, "SELECT CONCAT(CAST(value_usd AS CHAR), ':', CAST(observed_at AS CHAR)) FROM account_reports WHERE job_id=?", job[:]).Scan(&beforeReport); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(h.ctx, "SELECT CONCAT(CAST(total_usd AS CHAR), ':', CAST(evaluated_at AS CHAR), ':',account_count) FROM valuation_points WHERE job_id=?", job[:]).Scan(&beforePoint); err != nil {
		t.Fatal(err)
	}
	current, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := mysql.Up(db.DB); err != nil {
		t.Fatal(err)
	}
	upgraded, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || upgraded.Version != current.Version+1 {
		t.Fatalf("version %+v %v", upgraded, err)
	}
	var count int
	for _, a := range upgraded.Accounts {
		if a.AccountType == adapters.TypeManual && a.Selected {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Manual accounts=%d", count)
	}
	var afterReport, afterPoint string
	if err := db.DB.QueryRowContext(h.ctx, "SELECT CONCAT(CAST(value_usd AS CHAR), ':', CAST(observed_at AS CHAR)) FROM account_reports WHERE job_id=?", job[:]).Scan(&afterReport); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(h.ctx, "SELECT CONCAT(CAST(total_usd AS CHAR), ':', CAST(evaluated_at AS CHAR), ':',account_count) FROM valuation_points WHERE job_id=?", job[:]).Scan(&afterPoint); err != nil {
		t.Fatal(err)
	}
	if beforeReport != afterReport || beforePoint != afterPoint {
		t.Fatal("upgrade rewrote financial history")
	}
	manualSave(t, h, upgraded, "USD", "100")
	next, _ := h.runCycle()
	if next.Point.TotalUSD != "130" || next.Point.BoundaryContributionUSD != "0" || !hasManualAdjustment(next.Point) || !next.Point.EvaluatedAt.After(initial.Point.EvaluatedAt) {
		t.Fatalf("upgrade continuity %+v", next)
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	old, err := h.valuation.PointByJob(h.ctx, h.portfolioID, job)
	if err != nil || old.TotalUSD != "30" || old.AccountCount != 1 || !old.EvaluatedAt.Equal(initial.Point.EvaluatedAt.Truncate(time.Microsecond)) {
		t.Fatalf("old rebuild %+v %v", old, err)
	}
}
