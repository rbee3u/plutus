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
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

// Keep this inventory independent of the implementation. A new scoped table
// needs an explicit lifecycle decision and a populated deletion scenario.
var portfolioTables = []string{
	"account_reports", "account_valuations", "accounts", "asset_exclusion_versions", "assets", "audit_events", "balance_valuations", "balances", "boundary_events", "calculated_asset_changes", "capital_flows", "collection_cycles", "dual_positions", "earn_positions", "futures_contracts", "futures_positions", "identities", "manual_adjustments", "manual_revision_balances", "manual_revisions", "price_observations", "valuation_points",
}

// Exercise the complete owned graph in a dedicated temporary database.
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
	_, firstCycle := h.runCycle()
	manualSave(t, h, item, "USD", "13")
	newcomer := h.createIdentity(name+" manual", adapters.PlatformBinance, adapters.StructureBinancePM)
	manualSave(t, h, newcomer, "USD", "1")
	// Replacing credentials disables private collection; previous reports carry
	// forward while the Manual Account continues to produce fresh reports.
	h.storeCredentials(item, name)
	summary, _ := h.runCycle()
	if summary.Point.StaleAccountCount == 0 {
		t.Fatal("fixture needs stale source references")
	}
	assets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: p.ID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(assets.Items) != 1 {
		t.Fatalf("BTC asset: %+v %v", assets, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, assets.Items[0].ID, &catalog.ExclusionInput{PortfolioID: p.ID, ActorUserID: h.userID, Excluded: true, Reason: "Lifecycle fixture"})
	if err != nil {
		t.Fatal(err)
	}
	direction, symbol, quantity := capital.DirectionContribution, "USD", "10"
	at := time.Now().UTC()
	if _, err := h.services.Capital.Create(h.ctx, p.ID, &capital.Input{ActorUserID: h.userID, Direction: &direction, CanonicalAssetSymbol: &symbol, Quantity: &quantity, EffectiveAt: &at}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.services.Idempotency.Run(h.ctx, h.userID, "POST", "/lifecycle", name, nil, func(context.Context) (idempotency.Record, error) {
		return idempotency.Record{Status: 201, Body: []byte(`{}`)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	return h, p, firstCycle
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
	// Idempotency records belong to the User and retain their retry lifetime.
	if _, err := h.services.Idempotency.Run(h.ctx, h.userID, "POST", "/lifecycle", "global", nil, func(context.Context) (idempotency.Record, error) {
		return idempotency.Record{Status: 201, Body: []byte(`{}`)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	sharedCounts := func() map[string]int {
		counts := map[string]int{}
		for _, table := range []string{"users", "sessions", "idempotency_records"} {
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
	drainPortfolioDeletion(t, h, p.ID)
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
	_, err = h.services.Idempotency.Run(h.ctx, h.userID, "POST", "/lifecycle", "global", nil, func(context.Context) (idempotency.Record, error) {
		t.Fatal("replay invoked creation")
		return idempotency.Record{}, nil
	})
	if err != nil {
		t.Fatalf("global idempotency record lost: %v", err)
	}
}

func TestSchemaRejectsCrossPortfolioPointEvidence(t *testing.T) {
	h := architectureHarness(t)
	_, _, cycleID := populatedPortfolio(t, h, "constraint-target")
	_, other, _ := populatedPortfolio(t, h, "constraint-other")
	var accountID []byte
	if err := h.db.QueryRowContext(h.ctx, "SELECT id FROM accounts WHERE portfolio_id=? AND account_type='manual'", other.ID[:]).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "INSERT INTO manual_adjustments (portfolio_id,cycle_id,account_id) VALUES (?,?,?)", other.ID[:], cycleID[:], accountID); err == nil {
		t.Fatal("cross-Portfolio point reference accepted")
	}
}

func TestCatalogMappingRemainsPortfolioScoped(t *testing.T) {
	h := architectureHarness(t)
	_, a, _ := populatedPortfolio(t, h, "mapping-a")
	_, b, _ := populatedPortfolio(t, h, "mapping-b")
	assets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: a.ID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(assets.Items) != 1 {
		t.Fatalf("asset: %v", err)
	}
	asset := assets.Items[0]
	updated, err := h.catalog.PutMapping(h.ctx, asset.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: a.ID, CanonicalAssetSymbol: "XBT", MappingVersion: asset.MappingVersion})
	if err != nil || updated.CanonicalAssetSymbol != "XBT" || !updated.Excluded {
		t.Fatalf("mapping and exclusion view: %+v %v", updated, err)
	}

	otherAssets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: b.ID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(otherAssets.Items) != 1 || otherAssets.Items[0].ID == asset.ID || otherAssets.Items[0].CanonicalAssetSymbol != "BTC" {
		t.Fatalf("cross-Portfolio mapping: %+v %v", otherAssets, err)
	}
}

func drainPortfolioDeletion(t *testing.T, h *harness, id uuid.UUID) {
	t.Helper()
	for range 150 {
		if _, err := h.services.Portfolios.Get(h.ctx, h.userID, id); errors.Is(err, portfolio.ErrNotFound) {
			return
		}
		worked, err := h.services.Portfolios.DeleteBatch(h.ctx)
		if err != nil || !worked {
			t.Fatalf("deletion batch: %v %v", worked, err)
		}
	}
	t.Fatal("deletion did not finish within bounded batches")
}
