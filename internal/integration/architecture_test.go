package integration

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/capital"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

// Keep this inventory independent of the implementation. A new scoped table
// needs an explicit lifecycle decision and a populated deletion scenario.
var portfolioTables = []string{
	"account_reports", "accounts", "audit_events", "balances", "capital_flows",
	"dual_positions", "earn_positions", "idempotency_records", "identities", "jobs",
	"manual_balances", "manual_cycle_accounts", "manual_cycles", "manual_revisions",
	"portfolio_listing_exclusion_events", "portfolio_listing_exclusions", "positions", "valuation_points",
}

// These scenarios deliberately mutate shared Listings and Instruments. Give
// them their own migrated database so other fixture contracts stay independent.
func architectureHarness(t *testing.T) *harness {
	t.Helper()
	database, err := mysqltest.CreateTestDatabase(t.Context(), mysqltest.GotestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Drop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return newHarness(t, database.DB)
}

func populatedPortfolio(t *testing.T, base *harness, name string) (*harness, portfolio.Portfolio, uuid.UUID) {
	t.Helper()
	p, err := base.services.Portfolios.Create(base.ctx, base.userID, portfolio.CreateInput{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	copyHarness := *base
	h := &copyHarness
	h.portfolioID = p.ID
	item := h.createIdentity(name, adapters.PlatformBinance, adapters.StructureBinancePM, "spot", "portfolio_margin", "earn", "dual_investment")
	item = h.storeCredentials(item, name)
	item, _, err = h.identities.Enable(h.ctx, p.ID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil {
		t.Fatal(err)
	}
	manualSave(t, h, item, "USD", "12")
	_, firstJob := h.runCycle()
	// Replacing credentials disables private collection; previous reports carry
	// forward while the Manual Account continues to produce fresh reports.
	h.storeCredentials(item, name)
	summary, _ := h.runCycle()
	if summary.Point.StaleAccountCount == 0 {
		t.Fatal("fixture needs stale source references")
	}
	listings, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: p.ID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(listings.Items) != 1 {
		t.Fatalf("BTC listing: %+v %v", listings, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, listings.Items[0].ID, &catalog.ExclusionInput{PortfolioID: p.ID, ActorUserID: h.userID, Ignored: true, Reason: "Lifecycle fixture"})
	if err != nil {
		t.Fatal(err)
	}
	direction, symbol, quantity := capital.DirectionContribution, "USD", "10"
	at := time.Now().UTC()
	if _, err := h.services.Capital.Create(h.ctx, p.ID, &capital.Input{ActorUserID: h.userID, Direction: &direction, AssetSymbol: &symbol, Quantity: &quantity, EffectiveAt: &at}); err != nil {
		t.Fatal(err)
	}
	if err := h.services.Idempotency.Put(h.ctx, h.userID, &p.ID, "POST", "/lifecycle", name, idempotency.Hash(nil), idempotency.Record{Status: 201}); err != nil {
		t.Fatal(err)
	}
	return h, p, firstJob
}

func scopedCounts(t *testing.T, h *harness, id uuid.UUID) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, table := range portfolioTables {
		var count int
		// Table names are the fixed test inventory above, never request input.
		if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM "+table+" WHERE portfolio_id = ?", id[:]).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func TestPortfolioDeletionOwnsCompleteLifetime(t *testing.T) {
	h := architectureHarness(t)
	rows, err := h.db.QueryContext(h.ctx, "SELECT TABLE_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND COLUMN_NAME = 'portfolio_id' ORDER BY TABLE_NAME")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tables, portfolioTables) {
		t.Fatalf("decide lifecycle coverage for schema tables: got %v, covered %v", tables, portfolioTables)
	}
	target, p, _ := populatedPortfolio(t, h, "delete-target")
	_, other, _ := populatedPortfolio(t, h, "delete-survivor")
	beforeOther := scopedCounts(t, h, other.ID)
	for table, count := range scopedCounts(t, h, p.ID) {
		if count == 0 {
			t.Fatalf("fixture did not populate %s", table)
		}
	}
	// NULL-scoped records belong to the User, not to the deleted Portfolio.
	if err := h.services.Idempotency.Put(h.ctx, h.userID, nil, "POST", "/lifecycle", "global", idempotency.Hash(nil), idempotency.Record{Status: 201}); err != nil {
		t.Fatal(err)
	}
	resource := uuid.New()
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO audit_events (id, actor_user_id, action, resource_type, resource_id, request_id, occurred_at) VALUES (?, ?, 'test', 'user', ?, 'test', UTC_TIMESTAMP(6))", resource[:], h.userID[:], h.userID[:]); err != nil {
		t.Fatal(err)
	}
	sharedCounts := func() map[string]int {
		counts := map[string]int{}
		for _, table := range []string{"users", "sessions", "asset_listings", "instruments", "prices"} {
			var n int
			if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			counts[table] = n
		}
		return counts
	}
	beforeShared := sharedCounts()
	if err := target.services.Portfolios.Delete(h.ctx, h.userID, h.portfolioID, 1); !errors.Is(err, portfolio.ErrUndeletable) {
		t.Fatalf("default protection: %v", err)
	}
	if err := target.services.Portfolios.Delete(h.ctx, h.userID, p.ID, p.Version+1); !errors.Is(err, portfolio.ErrPrecondition) {
		t.Fatalf("version protection: %v", err)
	}
	if err := target.services.Portfolios.Delete(h.ctx, h.userID, p.ID, p.Version); err != nil {
		t.Fatal(err)
	}
	for table, count := range scopedCounts(t, h, p.ID) {
		if count != 0 {
			t.Fatalf("%s retained %d scoped rows", table, count)
		}
	}
	if _, err := h.services.Portfolios.Get(h.ctx, h.userID, p.ID); !errors.Is(err, portfolio.ErrNotFound) {
		t.Fatalf("deleted Portfolio: %v", err)
	}
	if !reflect.DeepEqual(beforeOther, scopedCounts(t, h, other.ID)) {
		t.Fatal("another Portfolio changed")
	}
	if !reflect.DeepEqual(beforeShared, sharedCounts()) {
		t.Fatal("shared data changed")
	}
	record, _, err := h.services.Idempotency.Get(h.ctx, h.userID, "POST", "/lifecycle", "global")
	if err != nil || record == nil {
		t.Fatalf("global idempotency record lost: %v", err)
	}
	var auditCount int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM audit_events WHERE id = ? AND portfolio_id IS NULL", resource[:]).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("global audit: count=%d err=%v", auditCount, err)
	}
}

func TestPortfolioDeletionRollsBackOnForeignOwnership(t *testing.T) {
	h := architectureHarness(t)
	target, p, jobID := populatedPortfolio(t, h, "rollback-target")
	survivor, other, _ := populatedPortfolio(t, h, "rollback-survivor")
	var accountID []byte
	if err := h.db.QueryRowContext(h.ctx, "SELECT id FROM accounts WHERE portfolio_id = ? AND account_type = 'manual'", other.ID[:]).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	// The current schema permits this mismatched ownership. Deletion must refuse
	// it with the FK still enabled instead of leaving another Portfolio orphaned.
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO manual_cycle_accounts (portfolio_id, job_id, account_id, version, balances_json) VALUES (?, ?, ?, 0, '[]')", other.ID[:], jobID[:], accountID); err != nil {
		t.Fatal(err)
	}
	before := scopedCounts(t, h, p.ID)
	beforeOther := scopedCounts(t, h, other.ID)
	reports := func() []string {
		rows, err := h.db.QueryContext(h.ctx, "SELECT CONCAT(HEX(id), ':', method, ':', COALESCE(value_usd, 'NULL'), ':', COALESCE(HEX(source_report_id), 'NULL')) FROM account_reports WHERE portfolio_id = ? ORDER BY id", p.ID[:])
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var values []string
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				t.Fatal(err)
			}
			values = append(values, value)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return values
	}
	beforeReports := reports()
	if err := target.services.Portfolios.Delete(h.ctx, h.userID, p.ID, p.Version); err == nil {
		t.Fatal("foreign ownership must block deletion")
	}
	if !reflect.DeepEqual(before, scopedCounts(t, h, p.ID)) || !reflect.DeepEqual(beforeReports, reports()) {
		t.Fatal("failed deletion did not restore rows and report values/sources")
	}
	if !reflect.DeepEqual(beforeOther, scopedCounts(t, survivor, other.ID)) {
		t.Fatal("failed deletion changed another Portfolio")
	}
	if _, err := h.services.Portfolios.Get(h.ctx, h.userID, p.ID); err != nil {
		t.Fatal(err)
	}
	var checks int
	if err := h.db.QueryRowContext(h.ctx, "SELECT @@FOREIGN_KEY_CHECKS").Scan(&checks); err != nil || checks != 1 {
		t.Fatalf("foreign key enforcement: %d %v", checks, err)
	}
}

func TestCatalogMappingSchedulesRebuildsWithoutHTTP(t *testing.T) {
	h := architectureHarness(t)
	_, a, _ := populatedPortfolio(t, h, "mapping-a")
	_, b, _ := populatedPortfolio(t, h, "mapping-b")
	listings, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: a.ID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(listings.Items) != 1 {
		t.Fatalf("listing: %v", err)
	}
	listing := listings.Items[0]
	updated, err := h.catalog.PutMapping(h.ctx, listing.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: a.ID, AssetSymbol: "XBT", Version: listing.Version})
	if err != nil || updated.AssetSymbol != "XBT" || !updated.Ignored {
		t.Fatalf("mapping and exclusion view: %+v %v", updated, err)
	}
	var jobIDs []uuid.UUID
	for _, id := range []uuid.UUID{a.ID, b.ID} {
		page, err := h.jobs.List(h.ctx, id, jobs.KindValuationRecalc, 50, "")
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("rebuilds for %s: %+v %v", id, page, err)
		}
		jobIDs = append(jobIDs, page.Items[0].ID)
	}
	// Another update keeps the queue's existing active-job deduplication.
	if _, err := h.catalog.PutMapping(h.ctx, listing.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: a.ID, AssetSymbol: "BTC", Version: updated.Version}); err != nil {
		t.Fatal(err)
	}
	for i, id := range []uuid.UUID{a.ID, b.ID} {
		page, err := h.jobs.List(h.ctx, id, jobs.KindValuationRecalc, 50, "")
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != jobIDs[i] {
			t.Fatalf("deduplication: %+v %v", page, err)
		}
	}
}
