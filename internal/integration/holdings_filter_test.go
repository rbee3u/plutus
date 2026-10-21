package integration

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/valuation"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestHoldingsSubstringFiltersBeforePagination(t *testing.T) {
	h := newHarness(t)
	// Exercise the production composition root through collection and reads.
	h.collection = h.services.Collection
	item := h.createIdentity("Search", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot", "um_futures", "cm_futures", "dual_investment")
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
	if err != nil || missing.Total != 0 || *missing.TotalUSD != "0" || missing.Completeness != full.Completeness {
		t.Fatalf("empty filter total: %+v %v", missing, err)
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
	item := h.createIdentity("Pinned", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot", "um_futures", "earn", "dual_investment")
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
	if unknown.TotalUSD != nil || unknown.TotalCompleteness != valuation.CompletenessIncomplete || len(unknown.Items) != 1 || !unknown.Items[0].NoPrice {
		t.Fatalf("unknown subtotal: %+v", unknown)
	}
	known := read("usd")
	if known.TotalUSD == nil || *known.TotalUSD != "100" || known.TotalCompleteness != valuation.CompletenessComplete || len(known.TotalIssues) != 0 {
		t.Fatalf("filtered known total: %+v", known)
	}
	a.failed = true
	h.runCycle()
	stale := read("")
	if !stale.Coverage.Complete || stale.TotalCompleteness != valuation.CompletenessIncomplete || stale.TotalFreshness != valuation.FreshnessStale || len(stale.Items) == 0 || stale.Items[0].Freshness != valuation.FreshnessStale {
		t.Fatalf("stale report coverage: %+v", stale)
	}
	missing := h.createIdentity("No report", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	h.runCycle()
	known = read("usd")
	if known.Completeness != valuation.CompletenessIncomplete || known.TotalCompleteness != valuation.CompletenessComplete {
		t.Fatalf("unrelated missing account changed filtered total: %+v", known)
	}
	missingRows, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IdentityID: &missing.ID, Symbol: "unmatched"})
	if err != nil || missingRows.Coverage.Complete || missingRows.TotalCompleteness != valuation.CompletenessIncomplete || missingRows.TotalUSD != nil || len(missingRows.Coverage.MissingAccountIDs) == 0 || len(missingRows.TotalIssues) == 0 {
		t.Fatalf("empty search lost missing reports: %+v %v", missingRows, err)
	}
}

// Product summaries value selected position amounts, rather than whole accounts
// or separately reported rewards, using the point's adopted price evidence.
func TestProductTotalsUseMatchingHoldingsAndAdoptedPrices(t *testing.T) {
	h := newHarness(t)
	a := &summaryScenario{price: "50000"}
	h.registry.Register(adapter.PlatformOKX, a)
	item := h.createIdentity("Product totals", adapter.PlatformOKX, adapter.StructureOKXUnified, "earn", "dual_investment")
	item = h.storeCredentials(item, "summary")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("enable: %+v %v", outcome, err)
	}
	_, first := h.runCycle()
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	check := func(symbol string, cycle *uuid.UUID, earnWant, dualWant *string, complete, freshness string) {
		t.Helper()
		filter := &financial.ListFilter{IdentityID: &item.ID, Symbol: symbol, Limit: 1, Offset: 1, CycleID: cycle}
		earn, err := h.financial.FlatEarnPositions(h.ctx, p, filter)
		if err != nil {
			t.Fatal(err)
		}
		assertHoldingTotal(t, earn, earnWant, complete, freshness)
		dual, err := h.financial.FlatDualPositions(h.ctx, p, filter)
		if err != nil {
			t.Fatal(err)
		}
		assertHoldingTotal(t, dual, dualWant, complete, freshness)
	}
	check("", nil, new("50010"), new("50010"), "incomplete", "fresh")
	check("BTC", nil, new("50010"), new("50000"), "complete", "fresh")
	check("MYSTERY", nil, nil, nil, "incomplete", "fresh")
	check("no-match", nil, new("0"), new("0"), "complete", "fresh")
	// A missing unrelated identity affects the portfolio, not these totals.
	missing := h.createIdentity("Missing products", adapter.PlatformBinance, adapter.StructureBinanceClassic, "earn", "dual_investment")
	a.failed, a.price = true, "90000"
	h.runCycle()
	check("BTC", nil, new("50010"), new("50000"), "complete", "stale")
	check("MYSTERY", nil, nil, nil, "incomplete", "stale")
	check("BTC", &first, new("50010"), new("50000"), "complete", "fresh")
	filter := &financial.ListFilter{IdentityID: &missing.ID, Symbol: "no-match"}
	earn, err := h.financial.FlatEarnPositions(h.ctx, p, filter)
	if err != nil {
		t.Fatal(err)
	}
	assertHoldingTotal(t, earn, nil, "incomplete", "fresh")
	dual, err := h.financial.FlatDualPositions(h.ctx, p, filter)
	if err != nil {
		t.Fatal(err)
	}
	assertHoldingTotal(t, dual, nil, "incomplete", "fresh")
	// Only a successful empty report establishes zero held value.
	a.failed, a.empty, a.price = false, true, "90000"
	h.runCycle()
	check("", nil, new("0"), new("0"), "complete", "fresh")
}

func assertHoldingTotal[T any](t *testing.T, list financial.FlatList[T], want *string, completeness, freshness string) {
	t.Helper()
	if !reflect.DeepEqual(list.TotalUSD, want) || list.TotalCompleteness != completeness || list.TotalFreshness != freshness {
		t.Fatalf("total: %+v; want %v / %s / %s", list, want, completeness, freshness)
	}
	if completeness == "incomplete" && len(list.TotalIssues) == 0 {
		t.Fatal("incomplete total has no named omissions")
	}
}

type summaryScenario struct {
	price         string
	failed, empty bool
}

func (a *summaryScenario) Validate(context.Context, adapter.Credentials, adapter.AccountStructure) (adapter.ValidationResult, error) {
	return adapter.ValidationResult{ExternalRef: "summary", MarginMode: adapter.MarginOKXPM}, nil
}

func (a *summaryScenario) Collect(_ context.Context, _ adapter.Credentials, _ adapter.AccountStructure, kinds []adapter.AccountType) (adapter.Batch, error) {
	at := time.Now().UTC()
	out := adapter.Batch{CollectedAt: at, MarginMode: adapter.MarginOKXPM, Prices: []adapter.Price{{CanonicalAssetSymbol: "BTC", QuoteCanonicalAssetSymbol: "USD", Price: a.price, SourceMarketRef: "BTCUSD", RetrievedAt: at}}}
	for _, kind := range kinds {
		if a.failed {
			out.Reports = append(out.Reports, adapter.FailedReport(kind, adapter.NewError(adapter.ClassTemporary, "Synthetic product failure.")))
			continue
		}
		r := adapter.AccountReport{AccountType: kind, Status: adapter.StatusSucceeded, CollectedAt: at}
		if !a.empty {
			r.Balances = []adapter.Balance{{AssetSymbol: "BTC", Equity: "1"}, {AssetSymbol: "USDT", Equity: "10"}, {AssetSymbol: "MYSTERY", Equity: "2"}}
			if kind == "earn" {
				p := adapter.NewEarnPosition("multi", "flexible", "position", "total")
				p.Assets = []adapter.EarnAmount{{AssetSymbol: "BTC", Amount: "1"}, {AssetSymbol: "USDT", Amount: "10"}}
				p.AddReward("USDT", "1000", "pending")
				q := adapter.NewEarnPosition("unknown", "fixed", "position", "principal")
				q.Assets = []adapter.EarnAmount{{AssetSymbol: "MYSTERY", Amount: "2"}}
				r.EarnPositions = []adapter.EarnPosition{p, q}
			} else {
				r.DualPositions = []adapter.DualPosition{
					{PositionKey: "btc", PrincipalAssetSymbol: "BTC", Principal: "1", Status: "active", BaseAssetSymbol: new("BTC"), QuoteAssetSymbol: new("USDT"), AnnualizedRate: new("0.99"), RateKind: new("apr"), TargetPrice: new("100000")},
					{PositionKey: "usdt", PrincipalAssetSymbol: "USDT", Principal: "10", Status: "active", BaseAssetSymbol: new("ETH"), QuoteAssetSymbol: new("USDT")},
					{PositionKey: "unknown", PrincipalAssetSymbol: "MYSTERY", Principal: "2", Status: "active"},
				}
			}
		}
		out.Reports = append(out.Reports, r)
	}
	return out, nil
}
