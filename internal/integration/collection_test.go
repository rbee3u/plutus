package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestExclusionAuditPreservesHistoricalReasons(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Audit", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	state := manualSave(t, h, item, "USD", "1")
	asset := state.Items[0].AssetID
	for i, reason := range []string{"First reviewed reason", "", "Later reviewed reason"} {
		_, err := h.catalog.PutExclusion(h.ctx, asset, &catalog.ExclusionInput{
			PortfolioID: h.portfolioID, ActorUserID: h.userID, Version: uint64(i),
			Excluded: reason != "", Reason: reason,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	s := audit.NewService(h.db)
	page, err := s.List(h.ctx, h.portfolioID, &audit.ListFilter{ResourceType: "asset", Limit: 2})
	if err != nil || len(page.Items) != 2 || !page.HasMore || page.NextCursor == nil {
		t.Fatalf("audit page: %+v %v", page, err)
	}
	older, err := s.List(h.ctx, h.portfolioID, &audit.ListFilter{ResourceType: "asset", Limit: 2, Cursor: *page.NextCursor})
	if err != nil || len(older.Items) != 1 || older.HasMore {
		t.Fatalf("older audit page: %+v %v", older, err)
	}
	events := page.Items
	events = append(events, older.Items...)
	for i, want := range []struct{ before, after string }{{"", "Later reviewed reason"}, {"First reviewed reason", ""}, {"", "First reviewed reason"}} {
		for j, state := range []*map[string]any{events[i].Before, events[i].After} {
			reason := []string{want.before, want.after}[j]
			if state == nil || (*state)["excluded"] != (reason != "") {
				t.Fatalf("exclusion state: %+v", state)
			}
			if reason == "" {
				if (*state)["reason"] != nil {
					t.Fatalf("restored reason: %v", (*state)["reason"])
				}
			} else if (*state)["reason"] != reason || (*state)["effectiveAt"] == nil {
				t.Fatalf("historical exclusion: %+v, want %s", *state, reason)
			}
		}
	}
	if (*older.Items[0].Before)["effectiveAt"] != nil {
		t.Fatal("invented an effective time for implicit version zero")
	}
	other := newHarness(t)
	otherPage, err := s.List(h.ctx, other.portfolioID, &audit.ListFilter{ResourceType: "asset"})
	if err != nil || len(otherPage.Items) != 0 {
		t.Fatalf("cross-portfolio audit: %+v %v", otherPage, err)
	}
}

func TestEnableRequiresSuccessfulValidation(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("OKX bad key", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	if item.Connection.Status != identity.ConnectionUnconfigured {
		t.Fatalf("new identity connection %q, want unconfigured", item.Connection.Status)
	}
	if _, _, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); !errors.Is(err, identity.ErrCredentialsMissing) {
		t.Fatalf("enable without credentials: %v, want ErrCredentialsMissing", err)
	}
	stored := h.storeCredentials(item, "invalid-key")
	if stored.Connection.Status != identity.ConnectionDisabled || !stored.Connection.CredentialsConfigured {
		t.Fatalf("after storing credentials: %+v", stored.Connection)
	}
	updated, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, stored.ID, stored.Version, identity.ActionInput{ActorUserID: h.userID})
	if !errors.Is(err, identity.ErrValidationFailed) {
		t.Fatalf("enable with an invalid key: err=%v", err)
	}
	if outcome.OK || outcome.ErrorClass != adapters.ClassAuthentication {
		t.Fatalf("outcome %+v, want authentication failure", outcome)
	}
	if updated.Connection.Status != identity.ConnectionInvalid {
		t.Fatalf("connection %q, want invalid", updated.Connection.Status)
	}
	// A later successful validation clears invalid back to disabled without enabling.
	fixed := h.storeCredentials(updated, "seed-fixed-key")
	validated, outcome, err := h.identities.Validate(h.ctx, h.portfolioID, fixed.ID, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || validated.Connection.Status != identity.ConnectionDisabled || validated.ExternalRef == nil {
		t.Fatalf("validate fixed key: err=%v outcome=%+v connection=%+v", err, outcome, validated.Connection)
	}
}

func TestCycleValuesStablecoinsAtOneAndCarriesFailuresAsStale(t *testing.T) {
	h := newHarness(t)
	at := time.Now().UTC().Truncate(time.Minute).Add(-2 * time.Minute)
	for _, adapter := range h.fakes {
		adapter.WithClock(func() time.Time { return at })
	}
	good := h.createIdentity("Binance good", adapters.PlatformBinance, adapters.StructureBinancePM, "portfolio_margin", "spot")
	good = h.storeCredentials(good, "seed-good-key")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, good.ID, good.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || enabled.Connection.Status != identity.ConnectionEnabled {
		t.Fatalf("enable good key: err=%v outcome=%+v status=%q", err, outcome, enabled.Connection.Status)
	}
	flaky := h.createIdentity("Bybit flaky", adapters.PlatformBybit, adapters.StructureBybitUTA2, "unified")
	flaky = h.storeCredentials(flaky, "flaky-key")
	// The flaky key fails once per minute with a temporary error and then succeeds; enabling may need one retry.
	enabledFlaky, _, err := h.identities.Enable(h.ctx, h.portfolioID, flaky.ID, flaky.Version, identity.ActionInput{ActorUserID: h.userID})
	if errors.Is(err, identity.ErrValidationFailed) {
		enabledFlaky, _, err = h.identities.Enable(h.ctx, h.portfolioID, flaky.ID, enabledFlaky.Version, identity.ActionInput{ActorUserID: h.userID})
	}
	if err != nil || enabledFlaky.Connection.Status != identity.ConnectionEnabled {
		t.Fatalf("enable flaky key: err=%v status=%q", err, enabledFlaky.Connection.Status)
	}
	// Move the fake clock to a fresh minute so the cycle's first Collect call fails once and is retried.
	at = at.Add(time.Minute)

	summary, cycleID := h.runCycle()
	if summary.IdentityCount != 2 || summary.ReportFailed != 0 || summary.Invalidated != 0 {
		t.Fatalf("first cycle summary: %+v", summary)
	}
	if summary.Point.Completeness() != valuation.CompletenessComplete || summary.Point.StaleAccountCount != 0 {
		t.Fatalf("first point should be complete and fresh: %+v", summary.Point)
	}

	// The spot account has no reported equity: it is calculated from USDC (1 USD, no price row) and SOL (priced).
	accounts := h.accountsByType(cycleID, good.ID)
	spot := accounts["spot"]
	if spot.Method != valuation.MethodCalculated || spot.Freshness != valuation.FreshnessFresh || len(spot.Omitted) != 0 || spot.ValueUSD == nil {
		t.Fatalf("spot valuation: %+v", spot)
	}
	scenario := fake.Scenario("seed-good-key", *spot.ReportObservedAt)
	report := scenario.Report("", "spot", *spot.ReportObservedAt, false)
	expected := decimal.Zero()
	for _, b := range report.Balances {
		qty, _ := decimal.Parse(b.Equity)
		switch b.AssetSymbol {
		case "USDC":
			expected = decimal.Add(expected, qty)
		default:
			price, ok := decimal.Parse(scenario.Price(b.AssetSymbol))
			if !ok {
				t.Fatalf("no fake price for %s", b.AssetSymbol)
			}
			expected = decimal.Add(expected, decimal.Mul(qty, price))
		}
	}
	if got, _ := decimal.Parse(*spot.ValueUSD); got.Cmp(expected) != 0 {
		t.Fatalf("spot value %s, want %s", *spot.ValueUSD, decimal.Format(expected))
	}
	pm := accounts["portfolio_margin"]
	if pm.Method != valuation.MethodReported || pm.Equity == nil || *pm.ValueUSD != *pm.Equity {
		t.Fatalf("portfolio margin should be valued by reported equity: %+v", pm)
	}
	total := decimal.Zero()
	all, err := h.valuation.Accounts(h.ctx, h.portfolioID, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		v, _ := decimal.Parse(*a.ValueUSD)
		total = decimal.Add(total, v)
	}
	if pointTotal, _ := decimal.Parse(summary.Point.TotalUSD); pointTotal.Cmp(total) != 0 {
		t.Fatalf("point total %s, want sum of accounts %s", summary.Point.TotalUSD, decimal.Format(total))
	}

	// USDT on a second platform maps to the same canonical asset symbol automatically
	// (ADR 0003).
	assets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Query: "USDT", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	platforms := map[string]bool{}
	for _, l := range assets.Items {
		if l.Symbol == "USDT" {
			if l.CanonicalAssetSymbol != "USDT" || l.MappingSource != catalog.MappingAuto {
				t.Fatalf("USDT asset on %s maps to %s (%s)", l.Platform, l.CanonicalAssetSymbol, l.MappingSource)
			}
			platforms[l.Platform] = true
		}
	}
	if !platforms["binance"] || !platforms["bybit"] {
		t.Fatalf("USDT should be listed on both platforms: %v", platforms)
	}

	// Break the good key in place: the next cycle must fail with authentication, set the connection
	// invalid, and carry the accounts forward as stale.
	broken := h.storeCredentials(enabled, "invalid-rotated-key")
	if _, err := h.db.ExecContext(h.ctx, "UPDATE identities SET connection_status = 'enabled', platform_login_ref = 'rate-limited-login' WHERE id = ?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	summary, cycleID = h.runCycle()
	if summary.Invalidated != 1 || summary.ReportFailed != 2 {
		t.Fatalf("second cycle summary: %+v", summary)
	}
	after, err := h.identities.Get(h.ctx, h.portfolioID, broken.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Connection.Status != identity.ConnectionInvalid || after.Connection.LastErrorClass == nil || *after.Connection.LastErrorClass != adapters.ClassAuthentication {
		t.Fatalf("connection after auth failure: %+v", after.Connection)
	}
	accounts = h.accountsByType(cycleID, good.ID)
	for _, kind := range []string{"portfolio_margin", "spot"} {
		a := accounts[kind]
		if a.Freshness != valuation.FreshnessStale || a.ValueUSD == nil || a.Method == valuation.MethodMissing {
			t.Fatalf("%s should be stale with its previous value: %+v", kind, a)
		}
	}
	if summary.Point.StaleAccountCount != 2 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("second point should be complete with two stale accounts: %+v", summary.Point)
	}
	prevTotal, _ := decimal.Parse(summary.Point.TotalUSD)
	if prevTotal.Sign() <= 0 {
		t.Fatalf("stale carry-forward must keep the total positive: %s", summary.Point.TotalUSD)
	}

	// The invalid identity is skipped by the scheduler-facing list and by the next cycle.
	collectable, err := h.identities.ListCollectable(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range collectable {
		if c.Identity.ID == broken.ID {
			t.Fatal("an invalid connection must not be collectable")
		}
	}
	summary, cycleID = h.runCycle()
	accounts = h.accountsByType(cycleID, good.ID)
	if got := accounts["spot"]; got.Status != "failed" || got.ErrorClass == nil || *got.ErrorClass != collection.ClassSkipped || got.Freshness != valuation.FreshnessStale {
		t.Fatalf("skipped account should carry a skipped failed report: %+v", got)
	}
	if summary.Invalidated != 0 {
		t.Fatalf("a skipped identity must not be invalidated again: %+v", summary)
	}
}

func TestAssetExclusionAppliesToFutureCalculatedAccountsOnly(t *testing.T) {
	h := newHarness(t)
	assetIdentity := h.createIdentity("Binance reviewed", adapters.PlatformBinance, adapters.StructureBinancePM, "portfolio_margin", "spot")
	assetIdentity = h.storeCredentials(assetIdentity, "unpriced-reviewed-key")
	assetIdentity, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, assetIdentity.ID, assetIdentity.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK {
		t.Fatalf("enable: %v %+v", err, outcome)
	}
	first, firstCycle := h.runCycle()
	firstAccounts := h.accountsByType(firstCycle, assetIdentity.ID)
	if first.Point.Completeness() != valuation.CompletenessIncomplete || firstAccounts["spot"].Method != valuation.MethodCalculated || len(firstAccounts["spot"].Omitted) != 1 {
		t.Fatalf("expected TOKENX omission in calculated spot: %+v %+v", first.Point, firstAccounts["spot"])
	}
	if firstAccounts["portfolio_margin"].Method != valuation.MethodReported {
		t.Fatalf("margin should use reported equity: %+v", firstAccounts["portfolio_margin"])
	}

	assets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Query: "TOKENX", Limit: 50})
	if err != nil || len(assets.Items) != 1 {
		t.Fatalf("TOKENX asset: %+v %v", assets, err)
	}
	token := assets.Items[0]
	input := catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, RequestID: "test", Excluded: true, Reason: "Reviewed dust balance", Version: 0}
	if _, err := h.catalog.PutExclusion(h.ctx, token.ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: true}); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("missing review reason: %v", err)
	}
	excluded, err := h.catalog.PutExclusion(h.ctx, token.ID, &input)
	if err != nil || !excluded.Excluded || excluded.ExclusionVersion != 1 {
		t.Fatalf("exclude TOKENX: %+v %v", excluded, err)
	}
	var auditCount int
	if err := h.db.QueryRowContext(h.ctx, `SELECT COUNT(*) FROM audit_events WHERE portfolio_id = ? AND action = 'catalog.asset.exclude' AND resource_id = ?`, h.portfolioID[:], token.ID[:]).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("review audit event count = %d: %v", auditCount, err)
	}
	statusExcludedPage, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Status: "excluded", Limit: 50})
	if err != nil || statusExcludedPage.Total != 1 || statusExcludedPage.Items[0].ID != token.ID {
		t.Fatalf("status=excluded catalog filter: %+v %v", statusExcludedPage, err)
	}
	activePage, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Status: "active", Query: "TOKENX", Limit: 50})
	if err != nil || activePage.Total != 0 {
		t.Fatalf("status=active catalog filter should not return excluded TOKENX: %+v %v", activePage, err)
	}
	if _, err := h.catalog.PutExclusion(h.ctx, token.ID, &input); !errors.Is(err, catalog.ErrPrecondition) {
		t.Fatalf("stale exclusion version: %v", err)
	}
	other, err := portfolio.NewService(h.db).Create(h.ctx, h.userID, portfolio.CreateInput{Name: "separate", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	otherAssets, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: other.ID, Query: "TOKENX", Limit: 50})
	if err != nil || len(otherAssets.Items) != 0 {
		t.Fatalf("exclusion leaked to another portfolio: %+v %v", otherAssets, err)
	}
	oldPoint, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, firstCycle)
	if err != nil || oldPoint.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("existing point changed before another cycle: %+v %v", oldPoint, err)
	}

	second, secondCycle := h.runCycle()
	spot := h.accountsByType(secondCycle, assetIdentity.ID)["spot"]
	if second.Point.Completeness() != valuation.CompletenessComplete || len(spot.Omitted) != 0 {
		t.Fatalf("reviewed TOKENX must not cause an omission: %+v %+v", second.Point, spot)
	}
	pctx := financial.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	flat, err := h.financial.FlatBalances(h.ctx, pctx, &financial.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range flat.Items {
		if row.AssetSymbol == "TOKENX" {
			t.Fatal("excluded TOKENX appeared in ordinary balances")
		}
	}
	view, err := h.financial.AccountBalances(h.ctx, pctx, spot.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range view.Balances {
		if row.AssetSymbol == "TOKENX" {
			t.Fatal("excluded TOKENX appeared in account balances")
		}
	}
	oldPoint, err = h.valuation.PointByCycle(h.ctx, h.portfolioID, firstCycle)
	if err != nil || oldPoint.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("historical rule lost during retry: %+v %v", oldPoint, err)
	}
	secondPoint, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, secondCycle)
	if err != nil || secondPoint.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("excluded cycle lost during retry: %+v %v", secondPoint, err)
	}

	priced, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Platform: "binance", Query: "USDC", Limit: 50})
	if err != nil || len(priced.Items) != 1 {
		t.Fatalf("USDC asset: %+v %v", priced, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, priced.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: true, Reason: "Reviewed priced balance", Version: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, thirdCycle := h.runCycle()
	thirdSpot := h.accountsByType(thirdCycle, assetIdentity.ID)["spot"]
	if thirdSpot.ValueUSD == nil || spot.ValueUSD == nil || *thirdSpot.ValueUSD == *spot.ValueUSD {
		t.Fatalf("priced exclusion did not change calculated account: before=%+v after=%+v", spot, thirdSpot)
	}
	thirdPoint, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, thirdCycle)
	if err != nil {
		t.Fatal(err)
	}
	change, err := h.financial.CustomValueChange(h.ctx, pctx, secondPoint.EvaluatedAt, thirdPoint.EvaluatedAt.Add(time.Microsecond))
	if err != nil || change.Completeness != valuation.CompletenessIncomplete || !strings.Contains(strings.Join(change.Reasons, " "), "asset_exclusion_changed") {
		t.Fatalf("treatment change must limit value change: %+v %v", change, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, priced.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: false, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, restoredPricedCycle := h.runCycle()
	restoredPricedPoint, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, restoredPricedCycle)
	if err != nil {
		t.Fatal(err)
	}
	change, err = h.financial.CustomValueChange(h.ctx, pctx, secondPoint.EvaluatedAt, restoredPricedPoint.EvaluatedAt.Add(time.Microsecond))
	if err != nil || change.Completeness != valuation.CompletenessComplete {
		t.Fatalf("matching endpoint rules should allow value change: %+v %v", change, err)
	}

	btc, err := h.catalog.ListAssets(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(btc.Items) != 1 {
		t.Fatalf("BTC asset: %+v %v", btc, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, btc.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: true, Reason: "Test reported equity", Version: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, fourthCycle := h.runCycle()
	margin := h.accountsByType(fourthCycle, assetIdentity.ID)["portfolio_margin"]
	if margin.Method != valuation.MethodReported || margin.Equity == nil || margin.ValueUSD == nil || *margin.Equity != *margin.ValueUSD {
		t.Fatalf("reported equity changed by exclusion: %+v", margin)
	}
	marginBalances, err := h.financial.AccountBalances(h.ctx, pctx, margin.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	foundBTC := false
	for _, row := range marginBalances.Balances {
		foundBTC = foundBTC || row.AssetSymbol == "BTC"
	}
	if !foundBTC {
		t.Fatal("reported-equity account hid BTC")
	}

	_, err = h.catalog.PutExclusion(h.ctx, token.ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: false, Version: excluded.ExclusionVersion})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRowContext(h.ctx, `SELECT COUNT(*) FROM audit_events WHERE portfolio_id = ? AND action = 'catalog.asset.restore' AND resource_id = ?`, h.portfolioID[:], token.ID[:]).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("restore audit event count = %d: %v", auditCount, err)
	}
	last, lastCycle := h.runCycle()
	if last.Point.Completeness() != valuation.CompletenessIncomplete || len(h.accountsByType(lastCycle, assetIdentity.ID)["spot"].Omitted) != 1 {
		t.Fatalf("restored TOKENX did not restore no_price: %+v", last.Point)
	}
}

func TestRateLimitPreservesConnectionAndMissingAccounts(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("OKX rate limited", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading", "funding")
	item = h.storeCredentials(item, "ratelimit-key")
	if _, err := h.db.ExecContext(h.ctx, "UPDATE identities SET connection_status = 'enabled', platform_login_ref = 'rate-limited-login' WHERE id = ?", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	summary, cycleID := h.runCycle()
	if summary.ReportFailed != 2 || summary.Invalidated != 0 || summary.IdentityFailed != 1 {
		t.Fatalf("rate-limited cycle summary: %+v", summary)
	}
	after, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Connection.Status != identity.ConnectionEnabled || after.Connection.LastErrorClass == nil || *after.Connection.LastErrorClass != adapters.ClassRateLimited {
		t.Fatalf("connection after rate limit: %+v", after.Connection)
	}
	accounts := h.accountsByType(cycleID, item.ID)
	for kind, a := range accounts {
		if kind == string(adapters.TypeManual) {
			if a.Method != valuation.MethodCalculated || a.ValueUSD == nil || *a.ValueUSD != "0" {
				t.Fatalf("Manual should remain independent: %+v", a)
			}
			continue
		}
		if a.Method != valuation.MethodMissing || a.ValueUSD != nil || a.ErrorClass == nil || *a.ErrorClass != adapters.ClassRateLimited {
			t.Fatalf("%s should be missing after a rate limit with no earlier report: %+v", kind, a)
		}
	}
	if summary.Point.Completeness() != valuation.CompletenessIncomplete || summary.Point.IncompleteAccountCount != 2 {
		t.Fatalf("point with never-collected accounts must be incomplete: %+v", summary.Point)
	}
	if summary.Point.TotalUSD != "0" {
		t.Fatalf("total of unknown values must be 0 with incomplete marking, got %s", summary.Point.TotalUSD)
	}
}

func TestOwnershipIsScopedByUser(t *testing.T) {
	h := newHarness(t)
	other, err := auth.NewService(h.db, time.Hour, 24*time.Hour).CreateUser(h.ctx, "other-"+uuid.NewString()[:8], "Other=123456")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := portfolio.NewService(h.db).Get(h.ctx, other.ID, h.portfolioID); !errors.Is(err, portfolio.ErrNotFound) {
		t.Fatalf("another user's portfolio must be not found, got %v", err)
	}
}

func TestMarginModeIsStoredAtValidationAndAuditedWhenItChanges(t *testing.T) {
	h := newHarness(t)
	start := time.Now().UTC()
	clock := start
	h.fakes[adapters.PlatformOKX].WithClock(func() time.Time { return clock })
	item := h.createIdentity("OKX switching", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	if item.MarginMode != "" {
		t.Fatalf("a new identity has no margin mode before validation, got %q", item.MarginMode)
	}
	item = h.storeCredentials(item, "switch-okx")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	first := fake.MarginMode(adapters.StructureOKXUnified, "switch-okx", start)
	if err != nil || !outcome.OK || enabled.MarginMode != first {
		t.Fatalf("enable: err=%v outcome=%+v mode=%q, want %q", err, outcome, enabled.MarginMode, first)
	}
	changes := func() int {
		t.Helper()
		var n int
		if err := h.db.QueryRowContext(h.ctx,
			"SELECT COUNT(*) FROM audit_events WHERE resource_id = ? AND action = 'identity.margin_mode.change' AND actor_user_id IS NULL", item.ID[:]).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if changes() != 0 {
		t.Fatal("validation observed the first margin mode; that is not a change")
	}

	// A cycle a minute later observes the next margin mode, as if the owner switched it on OKX.
	clock = start.Add(time.Minute)
	if summary, _ := h.runCycle(); summary.ReportFailed != 0 {
		t.Fatalf("cycle summary: %+v", summary)
	}
	after, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	second := fake.MarginMode(adapters.StructureOKXUnified, "switch-okx", clock)
	if second == first || after.MarginMode != second {
		t.Fatalf("stored margin mode %q after the switch from %q, want %q", after.MarginMode, first, second)
	}
	if changes() != 1 {
		t.Fatal("the switch must be audited once, with the system as the actor")
	}
	h.runCycle()
	if changes() != 1 {
		t.Fatal("an unchanged margin mode must not be audited again")
	}
}

func TestValidationRefusesAKeyOfAnotherPlatformAccount(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Binance bound", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures")
	item = h.storeCredentials(item, "key-first-account")
	validated, outcome, err := h.identities.Validate(h.ctx, h.portfolioID, item.ID, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || validated.ExternalRef == nil || validated.MarginMode != "" {
		t.Fatalf("validate: err=%v outcome=%+v identity=%+v", err, outcome, validated)
	}
	bound := *validated.ExternalRef

	replaced := h.storeCredentials(validated, "key-other-account")
	refused, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, replaced.ID, replaced.Version, identity.ActionInput{ActorUserID: h.userID})
	if !errors.Is(err, identity.ErrValidationFailed) || outcome.ErrorClass != adapters.ClassUnsupported || !strings.Contains(outcome.ErrorDetail, bound) {
		t.Fatalf("a key of another account must be refused: err=%v outcome=%+v", err, outcome)
	}
	if refused.Connection.Status != identity.ConnectionDisabled || refused.ExternalRef == nil || *refused.ExternalRef != bound {
		t.Fatalf("the refused key must leave the connection disabled and the binding unchanged: %+v", refused)
	}

	restored := h.storeCredentials(refused, "key-first-account")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, restored.ID, restored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("the original account's key must still validate: err=%v outcome=%+v", err, outcome)
	}
}

// foreignModeAdapter validates with a proper OKX margin mode but reports mode in a collection
// run, as a broken adapter would: another platform's margin mode, or none.
type foreignModeAdapter struct{ mode adapters.MarginMode }

func (foreignModeAdapter) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "foreign", MarginMode: adapters.MarginOKXPM}, nil
}

func (a foreignModeAdapter) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	batch := adapters.Batch{CollectedAt: time.Now().UTC(), MarginMode: a.mode}
	for _, accountType := range accounts {
		batch.Reports = append(batch.Reports, adapters.AccountReport{AccountType: accountType, Status: adapters.StatusSucceeded, CollectedAt: batch.CollectedAt})
	}
	return batch, nil
}

func TestAForeignOrMissingMarginModeFailsTheRun(t *testing.T) {
	for _, mode := range []adapters.MarginMode{adapters.MarginBybitCross, ""} {
		t.Run("mode "+string(mode), func(t *testing.T) {
			h := newHarness(t)
			h.registry.Register(adapters.PlatformOKX, foreignModeAdapter{mode: mode})
			item := h.createIdentity("OKX foreign mode", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
			item = h.storeCredentials(item, "k")
			if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
				t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
			}
			summary, cycleID := h.runCycle()
			if summary.ReportFailed != 1 || summary.Invalidated != 0 {
				t.Fatalf("summary %+v", summary)
			}
			if got := h.accountsByType(cycleID, item.ID)["trading"]; got.ErrorClass == nil || *got.ErrorClass != adapters.ClassUnsupported {
				t.Fatalf("trading report: %+v", got)
			}
			after, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
			if err != nil || after.MarginMode != adapters.MarginOKXPM || after.Connection.Status != identity.ConnectionEnabled {
				t.Fatalf("the stored margin mode and the connection must be kept: err=%v identity=%+v", err, after)
			}
		})
	}
}
