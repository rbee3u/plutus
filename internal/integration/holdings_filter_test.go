package integration

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"

	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestHoldingsSubstringFiltersBeforePagination(t *testing.T) {
	h := newHarness(t)
	// Exercise the production composition root through collection and reads.
	h.collection = h.services.Collection
	item := h.createIdentity("Search", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures", "cm_futures", "dual_investment")
	item = h.storeCredentials(item, "search-filters")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable: %+v %v", result, err)
	}
	_, cycleID := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	full, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || full.Total != 2 {
		t.Fatalf("positions: %+v %v", full, err)
	}
	page, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "uSd", Offset: 1, Limit: 1})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].FuturesSymbol != "BTCUSD_PERP" || *page.TotalUSD != *full.TotalUSD {
		t.Fatalf("substring pagination or total: %+v %v", page, err)
	}
	missing, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "no-match"})
	if err != nil || missing.Total != 0 || *missing.TotalUSD != *full.TotalUSD || missing.Completeness != full.Completeness {
		t.Fatalf("empty filter changed boundary totals: %+v %v", missing, err)
	}
	balances, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Limit: 500})
	if err != nil || balances.Total == 0 {
		t.Fatalf("balances: %+v %v", balances, err)
	}
	sum := decimal.Zero()
	for _, row := range balances.Items {
		if row.AssetSymbol != "USDT" || row.EquityUSD == nil {
			t.Fatalf("unexpected filtered balance: %+v", row)
		}
		sum = decimal.Add(sum, decimal.ParseOrZero(*row.EquityUSD))
	}
	if balances.TotalUSD == nil || decimal.ParseOrZero(*balances.TotalUSD).Cmp(sum) != 0 {
		t.Fatal("Balance total did not follow the filtered rows")
	}
	pageBalances, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Offset: 1, Limit: 1})
	if err != nil || len(pageBalances.Items) != 1 || pageBalances.Total != balances.Total || *pageBalances.TotalUSD != *balances.TotalUSD {
		t.Fatalf("Balance search pagination: %+v %v", pageBalances, err)
	}
	duals, err := h.financial.FlatDualPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "sDt", Limit: 1})
	if err != nil || duals.Total == 0 || len(duals.Items) != 1 {
		t.Fatalf("Dual quote substring: %+v %v", duals, err)
	}
	for _, account := range h.accountsByType(cycleID, item.ID) {
		if _, err := h.financial.AccountBalances(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.financial.AccountPositions(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.financial.AccountDualPositions(h.ctx, pctx, account.AccountID); err != nil {
			t.Fatal(err)
		}
	}
	if overview, err := h.financial.Overview(h.ctx, pctx); err != nil || len(overview.Platforms) != 1 || len(overview.Platforms[0].Identities) != 1 {
		t.Fatalf("Overview groups: %+v %v", overview, err)
	}
}

func TestAccountHoldingsRemainPinnedAfterNewCollection(t *testing.T) {
	h := newHarness(t)
	h.collection = h.services.Collection
	item := h.createIdentity("Pinned", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures", "earn", "dual_investment")
	item = h.storeCredentials(item, "pinned-holdings")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("enable: %+v %v", outcome, err)
	}
	_, first := h.runCycle()
	accounts := h.accountsByType(first, item.ID)
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	report := func(ctx context.Context, p financial.PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (financial.AccountView, error) {
		return h.financial.AccountReport(ctx, p, accountID, cycleID, false)
	}
	readers := []struct {
		name    string
		account uuid.UUID
		read    func(context.Context, financial.PortfolioContext, uuid.UUID, *uuid.UUID) (financial.AccountView, error)
	}{
		{"balances", accounts["spot"].AccountID, report},
		{"positions", accounts["um_futures"].AccountID, report},
		{"earn", accounts["earn"].AccountID, report},
		{"dual", accounts["dual_investment"].AccountID, report},
	}
	original := make([]financial.AccountView, len(readers))
	for i, r := range readers {
		view, err := r.read(h.ctx, p, r.account, &first)
		if err != nil {
			t.Fatal(err)
		}
		original[i] = view
	}
	flatReaders := []func(*uuid.UUID) (any, error){
		func(id *uuid.UUID) (any, error) {
			return h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{CycleID: id, Limit: 1, Offset: 1})
		},
		func(id *uuid.UUID) (any, error) {
			return h.financial.FlatPositions(h.ctx, p, &financial.ListFilter{CycleID: id, Limit: 1})
		},
		func(id *uuid.UUID) (any, error) {
			return h.financial.FlatEarnPositions(h.ctx, p, &financial.ListFilter{CycleID: id, Limit: 1})
		},
		func(id *uuid.UUID) (any, error) {
			return h.financial.FlatDualPositions(h.ctx, p, &financial.ListFilter{CycleID: id, Limit: 1})
		},
	}
	originals := make([]any, len(flatReaders))
	for i, read := range flatReaders {
		var err error
		originals[i], err = read(&first)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, second := h.runCycle()
	for i, read := range flatReaders {
		pinned, err := read(&first)
		if err != nil || !reflect.DeepEqual(pinned, originals[i]) {
			t.Fatalf("flat read %d drifted after collection: %v", i, err)
		}
	}
	for i, r := range readers {
		t.Run(r.name, func(t *testing.T) {
			view, err := r.read(h.ctx, p, r.account, &first)
			if err != nil || view.Context.CycleID == nil || *view.Context.CycleID != first || !reflect.DeepEqual(view, original[i]) {
				t.Fatalf("pinned read changed: %+v %v", view, err)
			}
			latest, err := r.read(h.ctx, p, r.account, nil)
			if err != nil || latest.Context.CycleID == nil || *latest.Context.CycleID != second {
				t.Fatalf("latest: %+v %v", latest, err)
			}
			missing := uuid.New()
			if _, err := r.read(h.ctx, p, r.account, &missing); !errors.Is(err, valuation.ErrNotFound) {
				t.Fatalf("unknown cycle: %v", err)
			}
			foreign := p
			foreign.ID = uuid.New()
			if _, err := r.read(h.ctx, foreign, r.account, &first); !errors.Is(err, valuation.ErrNotFound) {
				t.Fatalf("foreign portfolio: %v", err)
			}
		})
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE valuation_points SET detail_state='expired' WHERE portfolio_id=? AND cycle_id=?", h.portfolioID[:], first[:]); err != nil {
		t.Fatal(err)
	}
	for i, read := range flatReaders {
		if _, err := read(&first); !errors.Is(err, valuation.ErrDetailExpired) {
			t.Fatalf("flat read %d expired detail: %v", i, err)
		}
	}
	for _, r := range readers {
		if _, err := r.read(h.ctx, p, r.account, &first); !errors.Is(err, valuation.ErrDetailExpired) {
			t.Fatalf("%s expired detail: %v", r.name, err)
		}
	}
}

func TestReportedEquityDoesNotQualifyFilteredBalanceTotals(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "100", reported: new("1000"), unpriced: true}
	item := manualEnabled(t, h, a)
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	h.runCycle()
	read := func(symbol string) financial.FlatList[financial.BalanceRow] {
		t.Helper()
		out, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IdentityID: &item.ID, Symbol: symbol, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	all := read("")
	if all.Completeness != valuation.CompletenessComplete || all.TotalCompleteness != valuation.CompletenessIncomplete || !all.Coverage.Complete || all.TotalUSD == nil || *all.TotalUSD != "100" || len(all.TotalIssues) != 1 || all.TotalIssues[0].Symbol != "MYSTERY" || all.TotalIssues[0].Code != "no_price" || all.TotalIssues[0].OmittedBalance == nil || all.TotalIssues[0].OmittedBalance.Equity != "2" {
		t.Fatalf("reported equity hid unknown balance values: %+v", all)
	}
	unknown := read("mystery")
	if unknown.TotalUSD == nil || *unknown.TotalUSD != "0" || unknown.TotalCompleteness != valuation.CompletenessIncomplete || len(unknown.Items) != 1 || !unknown.Items[0].NoPrice {
		t.Fatalf("unknown subtotal: %+v", unknown)
	}
	known := read("usd")
	if known.TotalUSD == nil || *known.TotalUSD != "100" || known.TotalCompleteness != valuation.CompletenessComplete || len(known.TotalIssues) != 0 {
		t.Fatalf("filtered known total: %+v", known)
	}
	a.failed = true
	h.runCycle()
	stale := read("")
	if !stale.Coverage.Complete || stale.TotalCompleteness != valuation.CompletenessIncomplete || len(stale.Items) == 0 || stale.Items[0].Freshness != valuation.FreshnessStale {
		t.Fatalf("stale report coverage: %+v", stale)
	}
	missing := h.createIdentity("No report", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	h.runCycle()
	known = read("usd")
	if known.Completeness != valuation.CompletenessIncomplete || known.TotalCompleteness != valuation.CompletenessComplete {
		t.Fatalf("unrelated missing account changed filtered total: %+v", known)
	}
	missingRows, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IdentityID: &missing.ID, Symbol: "unmatched"})
	if err != nil || missingRows.Coverage.Complete || missingRows.TotalCompleteness != valuation.CompletenessIncomplete || len(missingRows.Coverage.MissingAccountIDs) == 0 || len(missingRows.TotalIssues) == 0 {
		t.Fatalf("empty search lost missing reports: %+v %v", missingRows, err)
	}
}
