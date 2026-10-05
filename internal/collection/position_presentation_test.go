package collection

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

func TestPositionNotionalCompletenessAndHistoricalSource(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Positions", adapters.PlatformBinance, adapters.StructureBinanceClassic, "um_futures", "cm_futures")
	stored := h.storeCredentials(item, "position-display")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, stored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	full, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Limit: 1})
	if err != nil || full.Total != 2 || full.Completeness != "complete" {
		t.Fatalf("full %+v %v", full, err)
	}
	filtered, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Symbol: "btcUsd_p", Limit: 1})
	if err != nil || filtered.Total != 1 || *filtered.TotalUSD != *full.TotalUSD {
		t.Fatalf("row filters changed boundary total: %+v %v", filtered, err)
	}
	for _, account := range h.accountsByType(first.Point.JobID, item.ID) {
		if account.AccountType == string(adapters.TypeManual) {
			continue
		}
		historical, err := h.valuation.AccountPositionsAt(h.ctx, pctx, account.AccountID, &first.Point.ID)
		if err != nil || len(historical.Positions) != 1 || historical.Positions[0].PositionMode == nil || *historical.Positions[0].PositionMode != "one_way" {
			t.Fatalf("historical %+v %v", historical, err)
		}
	}
	// A missing linear Mark must not hide the known inverse face notional.
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE positions p JOIN instruments n ON n.id=p.instrument_id JOIN account_reports r ON r.id=p.report_id SET p.mark_price=NULL WHERE r.portfolio_id=? AND n.kind='linear_perpetual'", h.portfolioID[:]); err != nil {
		t.Fatal(err)
	}
	incomplete, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Limit: 1})
	if err != nil || incomplete.Completeness != "incomplete" || incomplete.TotalUSD == nil || *incomplete.TotalUSD == "0" {
		t.Fatalf("notional completeness %+v %v", incomplete, err)
	}
	found := false
	for _, issue := range incomplete.Issues {
		if issue.Code == "position_notional_unknown" {
			found = true
		}
	}
	if !found {
		t.Fatal("unknown notional omitted without explanation")
	}
	historicalAccount := h.accountsByType(first.Point.JobID, item.ID)["cm_futures"]
	before, err := h.valuation.AccountPositionsAt(h.ctx, pctx, historicalAccount.AccountID, &first.Point.ID)
	if err != nil {
		t.Fatal(err)
	}
	h.collection.now = func() time.Time { return first.Point.EvaluatedAt.Add(time.Minute) }
	h.fakes[adapters.PlatformBinance].WithClock(h.collection.now)
	h.runCycle()
	after, err := h.valuation.AccountPositionsAt(h.ctx, pctx, historicalAccount.AccountID, &first.Point.ID)
	if err != nil || *before.Positions[0].MarkPrice != *after.Positions[0].MarkPrice || !before.Positions[0].ReportObservedAt.Equal(*after.Positions[0].ReportObservedAt) {
		t.Fatal("historical Positions changed after new collection")
	}
	foreign := valuation.PortfolioContext{ID: uuid.New(), Timezone: "UTC"}
	if _, err := h.valuation.AccountPositionsAt(h.ctx, foreign, historicalAccount.AccountID, &first.Point.ID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("foreign point access %v", err)
	}
}

func TestStalePositionsKeepSourceModeAndObservedTime(t *testing.T) {
	h := newHarness(t)
	clock := time.Now().UTC().Truncate(time.Second)
	key := "switch-position-source"
	for fake.MarginMode(adapters.StructureOKXUnified, key, clock) != adapters.MarginOKXPM {
		clock = clock.Add(time.Minute)
	}
	h.collection.now = func() time.Time { return clock }
	h.fakes[adapters.PlatformOKX].WithClock(h.collection.now)
	item := h.createIdentity("Source mode", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	item = h.storeCredentials(item, key)
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	before, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Limit: 50})
	if err != nil || len(before.Items) != 2 {
		t.Fatalf("before %+v %v", before, err)
	}
	item, err = h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	broken := h.storeCredentials(item, "invalid-position-source")
	// The Identity's latest setting cannot establish an older Report's regime.
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='enabled',margin_mode='okx_futures' WHERE id=?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	h.runCycle()
	after, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Limit: 50})
	if err != nil || len(after.Items) != len(before.Items) {
		t.Fatalf("after %+v %v", after, err)
	}
	for i, row := range after.Items {
		if row.Freshness != valuation.FreshnessStale || row.PortfolioMargin == nil || !*row.PortfolioMargin || !row.ReportObservedAt.Equal(*before.Items[i].ReportObservedAt) || !row.ReportObservedAt.Equal(first.Point.EvaluatedAt) {
			t.Fatalf("source mode or time changed: %+v", row)
		}
		if row.SettleMargin != nil {
			t.Fatal("Portfolio risk allocated to a stale Position")
		}
	}
}

func TestBinancePMMarginUsesHistoricalFactsAndPreservesStaleTime(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Binance margin", adapters.PlatformBinance, adapters.StructureBinancePM, "portfolio_margin")
	item = h.storeCredentials(item, "binance-margin-history")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	accountID := h.accountsByType(first.Point.JobID, item.ID)["portfolio_margin"].AccountID
	// Old PM rows retained size/Mark/leverage without margin provenance. Reads
	// can establish Cross from the immutable supported PM structure.
	if _, err := h.collection.db.ExecContext(h.ctx, `UPDATE positions p JOIN account_reports r ON r.id=p.report_id
        SET p.margin_mode=NULL,p.margin_basis=NULL,p.margin_currency=NULL WHERE r.portfolio_id=?`, h.portfolioID[:]); err != nil {
		t.Fatal(err)
	}
	before, err := h.valuation.AccountPositionsAt(h.ctx, pctx, accountID, &first.Point.ID)
	if err != nil || len(before.Positions) != 2 {
		t.Fatalf("historical %+v %v", before, err)
	}
	for _, row := range before.Positions {
		if row.SettleMargin == nil || row.MarginSource == nil || *row.MarginSource != "calculated" || row.Margin != nil || row.MarginBasis != nil || row.MarginMode == nil || *row.MarginMode != "cross" {
			t.Fatalf("historical PM margin %+v", row)
		}
		want := decimal.Format(new(big.Rat).Quo(decimal.MustParse(*row.NotionalUSD), decimal.MustParse(*row.Leverage)))
		if *row.SettleMargin != want {
			t.Fatalf("margin %s, want %s", *row.SettleMargin, want)
		}
	}
	item, err = h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	broken := h.storeCredentials(item, "invalid-binance-margin-history")
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='enabled' WHERE id=?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	h.collection.now = func() time.Time { return first.Point.EvaluatedAt.Add(time.Minute) }
	h.fakes[adapters.PlatformBinance].WithClock(h.collection.now)
	h.runCycle()
	stale, err := h.valuation.AccountPositions(h.ctx, pctx, accountID)
	if err != nil || len(stale.Positions) != len(before.Positions) {
		t.Fatalf("stale %+v %v", stale, err)
	}
	for i, row := range stale.Positions {
		if row.Freshness != valuation.FreshnessStale || row.SettleMargin == nil || *row.SettleMargin != *before.Positions[i].SettleMargin || !row.ReportObservedAt.Equal(*before.Positions[i].ReportObservedAt) {
			t.Fatal("stale margin used newer facts or changed source time")
		}
	}
	var amountCount, modeCount int
	if err := h.collection.db.QueryRowContext(h.ctx, `SELECT SUM(p.margin IS NOT NULL),SUM(p.margin_mode IS NOT NULL)
        FROM positions p JOIN account_reports r ON r.id=p.report_id WHERE r.portfolio_id=?`, h.portfolioID[:]).Scan(&amountCount, &modeCount); err != nil || amountCount != 0 || modeCount != 0 {
		t.Fatalf("source facts rewritten by reads: %d %d %v", amountCount, modeCount, err)
	}
}

func TestPositionCompletenessSeparatesBalanceOmissionsAndMissingSources(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Unpriced balance", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures")
	item = h.storeCredentials(item, "unpriced-position-completeness")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	positions, err := h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Limit: 50})
	if err != nil || first.Point.Completeness() != valuation.CompletenessIncomplete || positions.Completeness != valuation.CompletenessComplete || len(positions.Issues) != 0 {
		t.Fatalf("balance omission contaminated notional: %+v %v", positions, err)
	}
	missing := h.createIdentity("Missing positions", adapters.PlatformBybit, adapters.StructureBybitUTA2, "unified")
	missing = h.storeCredentials(missing, "position-missing-source")
	enabled, result, err := h.identities.Enable(h.ctx, h.portfolioID, missing.ID, missing.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	broken := h.storeCredentials(enabled, "invalid-position-missing-source")
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='enabled' WHERE id=?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	second, _ := h.runCycle()
	accountID := h.accountsByType(second.Point.JobID, missing.ID)["unified"].AccountID
	positions, err = h.valuation.FlatPositions(h.ctx, pctx, &valuation.ListFilter{Platform: "binance", Limit: 50})
	if err != nil || positions.Completeness != valuation.CompletenessIncomplete || positions.TotalUSD == nil || *positions.TotalUSD == "0" {
		t.Fatalf("missing position source omitted: %+v %v", positions, err)
	}
	found := false
	for _, issue := range positions.Issues {
		if issue.Code == "missing" && issue.AccountID != nil && *issue.AccountID == accountID {
			found = true
		}
	}
	if !found {
		t.Fatal("missing Account without position rows was not named")
	}
}
