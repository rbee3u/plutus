package seeds

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/valuation"
)

// The invalid key is confined to the regression fixture, not the development seed.
const keyAliceOKXInvalid = "invalid-alice-okx"

func buildEdgeCases(now time.Time) Seed {
	b := newBuilder()
	portfolios := edgeCases(b, now.UTC().Truncate(15*time.Minute))
	return Seed{SQL: b.SQL(), Live: b.live, Portfolios: portfolios}
}

func (b *builder) users(now time.Time) {
	b.usersWithInterval(now, 900)
}

// mapListing records a manual Asset override for one listing.
func (b *builder) mapListing(platform adapters.Platform, symbol, asset string) {
	key := string(platform) + "|" + symbol
	if row, ok := b.listings[key]; ok {
		row.asset = asset
		row.source = catalog.MappingManual
	}
}

// edgeCases: stale, never-collected, unpriced, boundary change, a day without points, and an unvalued flow.
func edgeCases(b *builder, now time.Time) []uuid.UUID {
	b.users(now)
	created := now.AddDate(0, 0, -10)
	b.identity(&identitySpec{ID: IdentityAliceBinance, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinancePM, Name: "Binance main", APIKey: keyAliceBinance, CreatedAt: created, CollectedAt: now.Add(-2 * time.Hour)})
	b.accounts(IdentityAliceBinance, aliceBinanceAccounts, selectedSet(typePortfolioMargin, typeFunding), created)
	b.identity(&identitySpec{ID: IdentityAliceOKX, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformOKX, Structure: adapters.StructureOKXUnified, MarginMode: modeAliceOKX, Name: "OKX revoked key", APIKey: keyAliceOKXInvalid, CreatedAt: created, CollectedAt: now.Add(-2 * time.Hour), LastError: adapters.ClassAuthentication, LastDetail: "The platform rejected the API key."})
	b.accounts(IdentityAliceOKX, aliceOKXAccounts, selectedSet(typeTrading), created)
	b.identity(&identitySpec{ID: IdentityAliceBybit, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBybit, Structure: adapters.StructureBybitUTA2, MarginMode: modeAliceBybit, Name: "Bybit never collected", APIKey: keyAliceBybit, CreatedAt: created, CollectedAt: now.Add(-2 * time.Hour), LastError: adapters.ClassTemporary, LastDetail: "The platform returned a temporary error."})
	b.accounts(IdentityAliceBybit, aliceBybitAccounts, selectedSet(typeUnified), created)
	b.identity(&identitySpec{ID: IdentityBobBinance, Portfolio: PortfolioBobMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinancePM, Name: "Binance", CreatedAt: created})
	b.accounts(IdentityBobBinance, bobBinanceAccounts, selectedSet(typePortfolioMargin), created)

	// Local-day phases: A = D-7..D-5, gap = D-4, B = D-3, C = D-2..now-2h.
	phaseA := localMidnight(now, 7, shanghai)
	gapStart := localMidnight(now, 4, shanghai)
	phaseB := localMidnight(now, 3, shanghai)
	phaseC := localMidnight(now, 2, shanghai)
	end := now.Add(-2 * time.Hour)
	for at := phaseA; !at.After(end); at = at.Add(2 * time.Hour) {
		if !at.Before(gapStart) && at.Before(phaseB) {
			continue
		}
		b.fakePrices(keyAliceBinance, adapters.PlatformBinance, at)
		b.fakePrices(keyAliceOKX, adapters.PlatformOKX, at)
		var reports []reportSpec
		switch {
		case at.Before(phaseB):
			reports = fakeReports(keyAliceBinance, adapters.PlatformBinance, "", at, aliceBinanceAccounts, typePortfolioMargin, typeSpot)
			// A wrapped coin the catalog maps manually to BTC, and TOKENX with no price.
			reports[1].Report.Balances = append(reports[1].Report.Balances, adapters.Balance{Symbol: "WBTC", Name: "Wrapped Bitcoin", Equity: "0.05"}, fake.UnpricedBalance(1))
		default:
			reports = fakeReports(keyAliceBinance, adapters.PlatformBinance, "", at, aliceBinanceAccounts, typePortfolioMargin, typeFunding)
		}
		if at.Before(phaseC) {
			reports = append(reports, fakeReports(keyAliceOKX, adapters.PlatformOKX, modeAliceOKX, at, aliceOKXAccounts, typeTrading)...)
		} else {
			reports = append(reports, failedReports(adapters.PlatformOKX, adapters.ClassAuthentication, "The platform rejected the API key.", aliceOKXAccounts, typeTrading)...)
		}
		reports = append(reports, failedReports(adapters.PlatformBybit, adapters.ClassTemporary, "The platform returned a temporary error.", aliceBybitAccounts, typeUnified)...)
		b.cycle(PortfolioAliceMain, at, reports)
	}
	b.mapListing(adapters.PlatformBinance, "WBTC", "BTC")

	btcAt := phaseA.AddDate(0, 0, 1).Add(12 * time.Hour)
	b.flow(&flowSpec{ID: FlowAliceBTC, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "BTC", Quantity: "0.1", ValueUSD: scale(fakePrice(keyAliceBinance, "BTC", btcAt), 0.1), At: btcAt, Note: "Deposit from cold wallet"})
	funding := AccountAliceBinanceFunding
	b.flow(&flowSpec{ID: FlowAliceUSDT, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "withdrawal", Asset: "USDT", Quantity: "500", ValueUSD: "500", At: phaseB.Add(12 * time.Hour), Account: &funding, Note: "Rent"})
	b.flow(&flowSpec{ID: FlowAliceTOKENX, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "TOKENX", Quantity: "1000", At: phaseC.Add(12 * time.Hour), Note: "Airdrop with no market price"})
	b.audit(&auditSpec{Portfolio: PortfolioAliceMain, Actor: UserAlice, Action: "identity.create", ResourceType: "identity", ResourceID: IdentityAliceBinance, After: map[string]any{"displayName": "Binance main", "platform": "binance"}, At: created})
	b.audit(&auditSpec{Portfolio: PortfolioAliceMain, Actor: UserAlice, Action: "identity.update", ResourceType: "identity", ResourceID: IdentityAliceBinance, After: map[string]any{"accounts": map[string]bool{"spot": false, "funding": true}}, At: phaseB})
	b.audit(&auditSpec{Portfolio: PortfolioAliceMain, Actor: UserAlice, Action: "catalog.listing.map", ResourceType: "asset_listing", ResourceID: uuid.Nil, After: map[string]any{"symbol": "WBTC", "assetSymbol": "BTC"}, At: phaseA})
	return []uuid.UUID{PortfolioAliceMain, PortfolioAliceTrading, PortfolioBobMain}
}

// assertEdgeCases checks valuation rules against a test-only anomaly fixture.
func assertEdgeCases(ctx context.Context, t *testing.T, db *sql.DB, now time.Time) {
	t.Helper()
	svc := valuation.NewService(db)
	p := valuation.PortfolioContext{ID: PortfolioAliceMain, Timezone: "Asia/Shanghai"}

	overview, err := svc.Overview(ctx, p)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.Point == nil || overview.TotalUSD == nil {
		t.Fatal("overview: expected a latest point")
	}
	if overview.StaleAccountCount == 0 {
		t.Fatal("overview: the revoked OKX trading account must be carried forward as stale")
	}
	if overview.IncompleteAccountCount == 0 || overview.Completeness != valuation.CompletenessIncomplete {
		t.Fatal("overview: the never-collected Bybit account must make the point incomplete")
	}
	codes := map[string]bool{}
	for _, issue := range overview.Issues {
		codes[issue.Code] = true
	}
	if !codes["missing"] || !codes["stale"] {
		t.Fatalf("overview: expected missing and stale issues, got %v", codes)
	}

	tree, err := svc.Tree(ctx, p)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	methods := map[uuid.UUID]string{}
	for _, platform := range tree.Platforms {
		for _, ident := range platform.Identities {
			for _, account := range ident.Accounts {
				methods[account.AccountID] = account.Method
			}
		}
	}
	if methods[AccountAliceBybitUnified] != valuation.MethodMissing {
		t.Fatalf("tree: Bybit unified should be missing, got %q", methods[AccountAliceBybitUnified])
	}
	if methods[AccountAliceOKXTrading] == valuation.MethodMissing || methods[AccountAliceOKXTrading] == "" {
		t.Fatalf("tree: OKX trading should keep its last successful report, got %q", methods[AccountAliceOKXTrading])
	}
	if _, present := methods[AccountAliceBinanceSpot]; present {
		t.Fatal("tree: the deselected Binance spot account must have left the boundary")
	}

	from := now.AddDate(0, 0, -9)
	events, err := svc.BoundaryEvents(ctx, p, from, now)
	if err != nil {
		t.Fatalf("boundary events: %v", err)
	}
	directions := map[uuid.UUID]string{}
	for _, event := range events {
		directions[event.AccountID] = event.Direction
		if event.ValueUSD == nil {
			t.Fatalf("boundary events: %s %s is unvalued", event.AccountName, event.Direction)
		}
		if event.IdentityName == "" || event.Platform == "" || event.IdentityID == uuid.Nil || event.Method == valuation.MethodMissing {
			t.Fatalf("boundary events: %s %s lacks identity, platform, or method: %+v", event.AccountName, event.Direction, event)
		}
	}
	if directions[AccountAliceBinanceFunding] != valuation.DirectionContribution || directions[AccountAliceBinanceSpot] != valuation.DirectionWithdrawal {
		t.Fatalf("boundary events: expected funding contribution and spot withdrawal, got %v", directions)
	}

	closes, err := svc.PeriodCloses(ctx, p, valuation.PeriodDay, from, now)
	if err != nil {
		t.Fatalf("period closes: %v", err)
	}
	gapDay := localMidnight(now, 4, shanghai).Format("2006-01-02")
	collectedDay := localMidnight(now, 6, shanghai).Format("2006-01-02")
	closed := map[string]bool{}
	for _, c := range closes {
		closed[c.Label] = c.Point != nil
	}
	if closed[gapDay] {
		t.Fatalf("period closes: the gap day %s must have no close", gapDay)
	}
	if !closed[collectedDay] {
		t.Fatalf("period closes: %s must have a close", collectedDay)
	}

	weeks, err := svc.ValueChanges(ctx, p, valuation.PeriodWeek, from, now)
	if err != nil {
		t.Fatalf("value changes: %v", err)
	}
	gapReported := false
	for _, change := range weeks {
		for _, reason := range change.Reasons {
			if strings.HasPrefix(reason, "days_without_points") {
				gapReported = true
			}
		}
	}
	if !gapReported {
		t.Fatal("value changes: the week containing the gap day must be incomplete")
	}

	custom, err := svc.CustomValueChange(ctx, p, localMidnight(now, 6, shanghai), now)
	if err != nil {
		t.Fatalf("custom value change: %v", err)
	}
	if custom.AmountUSD == nil || custom.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("custom value change: expected an incomplete amount, got %+v", custom)
	}
	unvalued := false
	for _, reason := range custom.Reasons {
		if strings.HasPrefix(reason, "unvalued_flow: TOKENX") {
			unvalued = true
		}
	}
	if !unvalued {
		t.Fatalf("custom value change: the TOKENX flow must be reported as unvalued, got %v", custom.Reasons)
	}
	if custom.BoundaryContributionsUSD == "0" || custom.BoundaryWithdrawalsUSD == "0" {
		t.Fatalf("custom value change: boundary events must be subtracted, got %+v", custom)
	}

	var btcValued, tokenValued int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capital_flows WHERE id = ? AND value_usd IS NOT NULL", FlowAliceBTC[:]).Scan(&btcValued); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM capital_flows WHERE id = ? AND value_usd IS NOT NULL", FlowAliceTOKENX[:]).Scan(&tokenValued); err != nil {
		t.Fatal(err)
	}
	if btcValued != 1 || tokenValued != 0 {
		t.Fatalf("capital flows: BTC must be valued and TOKENX unvalued (got %d, %d)", btcValued, tokenValued)
	}
	var omitted, wbtcOmitted int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE JSON_SEARCH(omitted_json, 'one', 'TOKENX') IS NOT NULL").Scan(&omitted); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports WHERE JSON_SEARCH(omitted_json, 'one', 'WBTC') IS NOT NULL").Scan(&wbtcOmitted); err != nil {
		t.Fatal(err)
	}
	if omitted == 0 || wbtcOmitted != 0 {
		t.Fatalf("omitted balances: TOKENX must be omitted (%d) and manually mapped WBTC valued (%d omitted)", omitted, wbtcOmitted)
	}
}
