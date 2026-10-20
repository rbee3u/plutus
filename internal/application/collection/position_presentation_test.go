package collection

import (
	"database/sql"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestPositionPersistenceRequiresPositiveMagnitude(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Position quantities", adapters.PlatformBinance, adapters.StructureBinanceClassic, "um_futures")
	var accountID uuid.UUID
	for _, account := range item.Accounts {
		if account.AccountType == "um_futures" {
			accountID = account.ID
		}
	}
	claim := claimRecoveryCycle(h)
	t.Cleanup(func() {
		if err := h.cycles.Finish(h.ctx, &claim, cycles.Result{ErrorClass: adapters.ClassNormalization, ErrorDetail: "SourceQuantity validation fixture."}); err != nil {
			t.Error(err)
		}
	})
	for _, quantity := range []string{"0", "-1", "invalid", "1"} {
		t.Run(quantity, func(t *testing.T) {
			tx, err := h.collection.db.BeginTx(h.ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			report := adapters.AccountReport{
				AccountType: "um_futures", Status: adapters.StatusSucceeded,
				Positions: []adapters.Position{{
					PositionKey: "BTCUSDT:short", Side: adapters.SideShort, PositionMode: "one_way",
					SourceQuantity: quantity, SourceQuantityUnit: adapters.UnitBase,
					FuturesContract: adapters.FuturesContract{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerpetual},
				}},
			}
			now := time.Now().UTC()
			err = h.collection.insertSuccessfulReport(h.ctx, h.collection.q.WithTx(tx), h.portfolioID, claim.ID, adapters.PlatformBinance, accountID, &report, "", now, now, sql.NullInt64{})
			if quantity == "1" {
				if err != nil {
					t.Fatalf("positive short quantity rejected: %v", err)
				}
			} else if !errors.Is(err, catalog.ErrInvalid) {
				t.Fatalf("invalid quantity must fail normalization before SQL insertion: %v", err)
			}
		})
	}
}

func TestPositionNotionalCompletenessAndHistoricalSource(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Positions", adapters.PlatformBinance, adapters.StructureBinanceClassic, "um_futures", "cm_futures")
	stored := h.storeCredentials(item, "position-display")
	if _, result, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, stored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !result.OK {
		t.Fatalf("enable %+v %v", result, err)
	}
	first, _ := h.runCycle()
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	full, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 1})
	if err != nil || full.Total != 2 || full.Completeness != "complete" {
		t.Fatalf("full %+v %v", full, err)
	}
	filtered, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Symbol: "btcUsd_p", Limit: 1})
	if err != nil || filtered.Total != 1 || filtered.TotalUSD == nil || *filtered.TotalUSD != *filtered.Items[0].NotionalUSD {
		t.Fatalf("filtered notional did not follow rows: %+v %v", filtered, err)
	}
	for _, account := range h.accountsByType(first.Point.CycleID, item.ID) {
		if account.AccountType == string(adapters.TypeManual) {
			continue
		}
		historical, err := h.financial.AccountPositionsAt(h.ctx, pctx, account.AccountID, &first.Point.CycleID)
		if err != nil || len(historical.Positions) != 1 || historical.Positions[0].PositionMode == nil || *historical.Positions[0].PositionMode != "one_way" {
			t.Fatalf("historical %+v %v", historical, err)
		}
	}
	// A missing linear mark must not hide the known inverse face notional.
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE futures_positions p JOIN futures_contracts n ON n.id=p.futures_contract_id JOIN account_reports r ON r.id=p.report_id SET p.mark_price=NULL WHERE r.portfolio_id=? AND n.kind='linear_perpetual'", h.portfolioID[:]); err != nil {
		t.Fatal(err)
	}
	incomplete, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 1})
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
	historicalAccount := h.accountsByType(first.Point.CycleID, item.ID)["cm_futures"]
	before, err := h.financial.AccountPositionsAt(h.ctx, pctx, historicalAccount.AccountID, &first.Point.CycleID)
	if err != nil {
		t.Fatal(err)
	}
	h.collection.now = func() time.Time { return first.Point.EvaluatedAt.Add(time.Minute) }
	h.fakes[adapters.PlatformBinance].WithClock(h.collection.now)
	h.runCycle()
	after, err := h.financial.AccountPositionsAt(h.ctx, pctx, historicalAccount.AccountID, &first.Point.CycleID)
	if err != nil || *before.Positions[0].MarkPrice != *after.Positions[0].MarkPrice || !before.Positions[0].ReportObservedAt.Equal(*after.Positions[0].ReportObservedAt) {
		t.Fatal("historical positions changed after new collection")
	}
	foreign := financial.PortfolioContext{ID: uuid.New(), Timezone: "UTC"}
	if _, err := h.financial.AccountPositionsAt(h.ctx, foreign, historicalAccount.AccountID, &first.Point.CycleID); !errors.Is(err, valuation.ErrNotFound) {
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
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	before, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || len(before.Items) != 2 {
		t.Fatalf("before %+v %v", before, err)
	}
	item, err = h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	broken := h.storeCredentials(item, "invalid-position-source")
	// The identity's latest setting cannot establish an older report's regime.
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='enabled',last_observed_margin_mode='okx_futures' WHERE id=?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	h.runCycle()
	after, err := h.financial.FlatPositions(h.ctx, pctx, &financial.ListFilter{Limit: 50})
	if err != nil || len(after.Items) != len(before.Items) {
		t.Fatalf("after %+v %v", after, err)
	}
	for i, row := range after.Items {
		if row.Freshness != valuation.FreshnessStale || row.PortfolioMargin == nil || !*row.PortfolioMargin || !row.ReportObservedAt.Equal(*before.Items[i].ReportObservedAt) || !row.ReportObservedAt.Equal(first.Point.EvaluatedAt) {
			t.Fatalf("source mode or time changed: %+v", row)
		}
		if row.SettleMargin != nil {
			t.Fatal("Portfolio risk allocated to a stale position")
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
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	accountID := h.accountsByType(first.Point.CycleID, item.ID)["portfolio_margin"].AccountID
	// Old PM rows retained size/mark/leverage without margin provenance. Reads
	// can establish cross from the immutable supported PM structure.
	if _, err := h.collection.db.ExecContext(h.ctx, `UPDATE futures_positions p JOIN account_reports r ON r.id=p.report_id
        SET p.margin_setting=NULL,p.margin_basis=NULL,p.margin_asset_symbol=NULL WHERE r.portfolio_id=?`, h.portfolioID[:]); err != nil {
		t.Fatal(err)
	}
	before, err := h.financial.AccountPositionsAt(h.ctx, pctx, accountID, &first.Point.CycleID)
	if err != nil || len(before.Positions) != 2 {
		t.Fatalf("historical %+v %v", before, err)
	}
	for _, row := range before.Positions {
		if row.SettleMargin == nil || row.MarginSource == nil || *row.MarginSource != "calculated" || row.Margin != nil || row.MarginBasis != nil || row.MarginSetting == nil || *row.MarginSetting != "cross" {
			t.Fatalf("historical PM margin %+v", row)
		}
		want := decimal.Format(new(big.Rat).Quo(decimal.ParseOrZero(*row.NotionalUSD), decimal.ParseOrZero(*row.Leverage)))
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
	stale, err := h.financial.AccountPositions(h.ctx, pctx, accountID)
	if err != nil || len(stale.Positions) != len(before.Positions) {
		t.Fatalf("stale %+v %v", stale, err)
	}
	for i, row := range stale.Positions {
		if row.Freshness != valuation.FreshnessStale || row.SettleMargin == nil || *row.SettleMargin != *before.Positions[i].SettleMargin || !row.ReportObservedAt.Equal(*before.Positions[i].ReportObservedAt) {
			t.Fatal("stale margin used newer facts or changed source time")
		}
	}
	var amountCount, modeCount int
	if err := h.collection.db.QueryRowContext(h.ctx, `SELECT SUM(p.margin IS NOT NULL),SUM(p.margin_setting IS NOT NULL)
        FROM futures_positions p JOIN account_reports r ON r.id=p.report_id WHERE r.portfolio_id=?`, h.portfolioID[:]).Scan(&amountCount, &modeCount); err != nil || amountCount != 0 || modeCount != 0 {
		t.Fatalf("source facts rewritten by reads: %d %d %v", amountCount, modeCount, err)
	}
}
