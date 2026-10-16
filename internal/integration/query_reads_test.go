package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

func TestCapitalAndCatalogQueryBudgets(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	h := newHarness(t, db)
	item := h.createIdentity("Original", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	var account uuid.UUID
	for _, a := range item.Accounts {
		if a.AccountType == "spot" {
			account = a.ID
		}
	}
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for range 50 {
		flow := uuid.New()
		if _, err := db.ExecContext(t.Context(), "INSERT INTO capital_flows(portfolio_id,id,direction,canonical_asset_symbol,quantity,value_usd,pricing_method,pricing_attempted_at,calculation_version,effective_at,account_id,version,created_at,updated_at) VALUES(?,?,'contribution','USD',1,1,'explicit',?,1,?,?,1,?,?)", h.portfolioID[:], flow[:], at, at, account[:], at, at); err != nil {
			t.Fatal(err)
		}
	}
	name := "Renamed"
	if _, err := h.identities.Patch(t.Context(), h.portfolioID, item.ID, item.Version, identity.PatchInput{ActorUserID: h.userID, DisplayName: &name}); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{1, 50} {
		stats.Queries.Store(0)
		page, err := h.services.Capital.ListPage(t.Context(), h.portfolioID, at.Add(-time.Hour), at.Add(time.Hour), 0, limit)
		if err != nil || page.Total != 50 || len(page.Items) != limit {
			t.Fatalf("capital page: %+v %v", page, err)
		}
		for _, flow := range page.Items {
			if flow.AccountName == nil || *flow.AccountName != "Renamed / Spot" {
				t.Fatalf("account label lost its current name: %+v", flow)
			}
		}
		if queries := stats.Queries.Load(); queries != 2 {
			t.Fatalf("capital labels amplified reads: %d queries for %d rows", queries, limit)
		}
	}
	asset := uuid.New()
	if _, err := db.ExecContext(t.Context(), "INSERT INTO assets(portfolio_id,id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", h.portfolioID[:], asset[:], at, at); err != nil {
		t.Fatal(err)
	}
	for _, version := range []int{1, 2, 3} {
		excluded := version != 2
		var reason any
		if excluded {
			reason = "Reviewed"
		}
		if _, err := db.ExecContext(t.Context(), "INSERT INTO asset_exclusion_versions(portfolio_id,asset_id,version,excluded,reason,effective_at) VALUES(?,?,?,?,?,?)", h.portfolioID[:], asset[:], version, excluded, reason, at); err != nil {
			t.Fatal(err)
		}
	}
	stats.Queries.Store(0)
	page, err := h.catalog.ListAssets(t.Context(), catalog.ListFilter{PortfolioID: h.portfolioID, Status: "excluded", Limit: 50})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ExclusionVersion != 3 || page.Items[0].ExclusionReason == nil {
		t.Fatalf("latest exclusion state: %+v %v", page, err)
	}
	if queries := stats.Queries.Load(); queries != 2 {
		t.Fatalf("catalog loaded unrelated rules: %d queries", queries)
	}
}

func TestManualAppliedRevisionReusesBalances(t *testing.T) {
	db, stats := mysqltest.ObserveQueries(t)
	h := newHarness(t, db)
	item := h.createIdentity("Manual query budget", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	state := manualSave(t, h, item, "USD", "10")
	// The normal collection workflow establishes an applied manual revision.
	h.runCycle()
	stats.Queries.Store(0)
	current, err := h.manual.Get(t.Context(), h.portfolioID, item.ID)
	if err != nil || current.Pending || current.AppliedVersion == nil || *current.AppliedVersion != state.Version || len(current.Items) != 1 || len(current.AppliedItems) != 1 {
		t.Fatalf("applied ledger: %+v %v", current, err)
	}
	if stats.Queries.Load() != 4 {
		t.Fatalf("same revision read twice: %d reads", stats.Queries.Load())
	}
	current.AppliedItems[0].Quantity = "99"
	if current.Items[0].Quantity != "10" {
		t.Fatal("editing applied presentation mutates the current ledger")
	}
	manualSave(t, h, item, "USD", "20")
	stats.Queries.Store(0)
	pending, err := h.manual.Get(t.Context(), h.portfolioID, item.ID)
	if err != nil || !pending.Pending || pending.Items[0].Quantity != "20" || pending.AppliedItems[0].Quantity != "10" || stats.Queries.Load() != 5 {
		t.Fatalf("pending ledger: %+v reads=%d error=%v", pending, stats.Queries.Load(), err)
	}
}
