package integration

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/db/migrations"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestEarnMigrationPreservesLegacyReports(t *testing.T) {
	db, err := mysqltest.CreateTestDatabase(context.Background(), mysqltest.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	h := newHarness(t, db.DB)
	h.registry.Register(adapters.PlatformOKX, &earnScenario{legacy: true})
	id := h.createIdentity("Legacy Earn", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn")
	id = h.storeCredentials(id, "test")
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, id.ID, id.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
		t.Fatal(err)
	}
	_, job := h.runCycle()
	account := h.accountsByType(job, id.ID)["earn"]
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	before, err := h.financial.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Account.ValueUSD == nil || *before.Account.ValueUSD != "50" || len(before.EarnBalanceSummaries) != 1 {
		t.Fatalf("expected populated legacy Earn: %+v", before)
	}
	// Apply the appended migration to populated tables, preserving old report facts.
	if _, err := h.db.ExecContext(h.ctx, "ALTER TABLE account_reports DROP COLUMN earn_details_collected"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "DROP TABLE earn_positions"); err != nil {
		t.Fatal(err)
	}
	source, err := migrations.FS.ReadFile("20261005010000_earn_positions.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(source), "-- +goose Down")
	if err := mysql.ExecSQLStatements(h.ctx, h.db, up); err != nil {
		t.Fatal(err)
	}
	after, err := h.financial.AccountEarnPositions(h.ctx, pctx, account.AccountID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("migration changed legacy report: before=%+v after=%+v err=%v", before, after, err)
	}
}
