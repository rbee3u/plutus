package collection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/valuation"
)

type harness struct {
	ctx         context.Context
	t           *testing.T
	userID      uuid.UUID
	portfolioID uuid.UUID
	identities  *identity.Service
	catalog     *catalog.Service
	valuation   *valuation.Service
	jobs        *jobs.Service
	collection  *Service
	fakes       map[adapters.Platform]*fake.Adapter
	registry    *adapters.Registry
}

// newHarness creates a fresh User and Portfolio in the shared test database and wires the
// services around fake adapters with an instant retry policy.
func newHarness(t *testing.T) *harness {
	t.Helper()
	db := mysql.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	registry := adapters.NewRegistry()
	fakes := map[adapters.Platform]*fake.Adapter{}
	for _, platform := range adapters.OfferedPlatforms() {
		f := fake.New(platform)
		fakes[platform] = f
		registry.Register(platform, f)
	}
	masterKey := []byte("0123456789abcdef0123456789abcdef")
	identities, err := identity.NewService(db, identity.Options{MasterKey: masterKey, Adapters: registry})
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(db, time.Hour, 24*time.Hour)
	user, err := authSvc.CreateUser(ctx, "collect-"+uuid.NewString()[:8], "Collect=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios, err := portfolio.NewService(db, 15*time.Minute).List(ctx, user.ID)
	if err != nil || len(portfolios) != 1 {
		t.Fatalf("expected the birth portfolio, got %d (%v)", len(portfolios), err)
	}
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	h := &harness{
		ctx: ctx, t: t, userID: user.ID, portfolioID: portfolios[0].ID,
		identities: identities, catalog: catalogSvc, valuation: valuationSvc, jobs: jobs.NewService(db), fakes: fakes, registry: registry,
	}
	h.collection = NewService(db, Options{
		Adapters: registry, Identities: identities, Catalog: catalogSvc, Valuation: valuationSvc,
		Stagger: 0, RetryDelays: []time.Duration{time.Millisecond, time.Millisecond},
	})
	return h
}

func (h *harness) createIdentity(name string, platform adapters.Platform, structure adapters.AccountStructure, types ...adapters.AccountType) identity.Identity {
	h.t.Helper()
	item, err := h.identities.Create(h.ctx, h.portfolioID, &identity.CreateInput{
		ActorUserID: h.userID, RequestID: "test", Platform: platform, DisplayName: name, AccountStructure: structure, SelectedAccountTypes: types,
	})
	if err != nil {
		h.t.Fatalf("create %s: %v", name, err)
	}
	return item
}

func (h *harness) storeCredentials(item identity.Identity, apiKey string) identity.Identity {
	h.t.Helper()
	stored, err := h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID, RequestID: "test"}, adapters.Credentials{APIKey: apiKey, APISecret: "secret"})
	if err != nil {
		h.t.Fatalf("store credentials for %s: %v", item.DisplayName, err)
	}
	return stored
}

func (h *harness) runCycle() (Summary, uuid.UUID) {
	h.t.Helper()
	job, _, err := h.jobs.EnqueueCycle(h.ctx, h.portfolioID, jobs.TriggerManual, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, job.ID)
	if err != nil {
		h.t.Fatalf("run cycle: %v", err)
	}
	// Finish the job so the next enqueue creates a new cycle instead of returning this one.
	claimed, ok, err := h.jobs.ClaimOne(h.ctx, "test-worker", time.Minute)
	if err != nil || !ok || claimed.ID != job.ID {
		h.t.Fatalf("claim cycle job: ok=%v err=%v", ok, err)
	}
	if err := h.jobs.Finish(h.ctx, &claimed, jobs.Result{OK: true, Detail: summary.String()}); err != nil {
		h.t.Fatal(err)
	}
	return summary, job.ID
}

func (h *harness) accountsByType(jobID uuid.UUID, identityID uuid.UUID) map[string]valuation.AccountValuation {
	h.t.Helper()
	accounts, err := h.valuation.Accounts(h.ctx, h.portfolioID, jobID)
	if err != nil {
		h.t.Fatal(err)
	}
	out := map[string]valuation.AccountValuation{}
	for _, a := range accounts {
		if a.IdentityID == identityID {
			out[a.AccountType] = a
		}
	}
	return out
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
	h.fakes[adapters.PlatformBybit].WithClock(func() time.Time { return time.Now().Add(2 * time.Minute) })

	summary, jobID := h.runCycle()
	if summary.IdentityCount != 2 || summary.ReportFailed != 0 || summary.Invalidated != 0 {
		t.Fatalf("first cycle summary: %+v", summary)
	}
	if summary.Point.Completeness() != valuation.CompletenessComplete || summary.Point.StaleAccountCount != 0 {
		t.Fatalf("first point should be complete and fresh: %+v", summary.Point)
	}

	// The spot Account has no reported equity: it is calculated from USDC (1 USD, no price row) and SOL (priced).
	accounts := h.accountsByType(jobID, good.ID)
	spot := accounts["spot"]
	if spot.Method != valuation.MethodCalculated || spot.Freshness != valuation.FreshnessFresh || len(spot.Omitted) != 0 || spot.ValueUSD == nil {
		t.Fatalf("spot valuation: %+v", spot)
	}
	scenario := fake.Scenario("seed-good-key", *spot.ReportObservedAt)
	report := scenario.Report("", "spot", *spot.ReportObservedAt, false)
	expected := decimal.Zero()
	for _, b := range report.Balances {
		qty, _ := decimal.Parse(b.Equity)
		switch b.Symbol {
		case "USDC":
			expected = decimal.Add(expected, qty)
		default:
			price, ok := decimal.Parse(scenario.Price(b.Symbol))
			if !ok {
				t.Fatalf("no fake price for %s", b.Symbol)
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
	all, err := h.valuation.Accounts(h.ctx, h.portfolioID, jobID)
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

	// USDT on a second Platform maps to the same Asset symbol automatically
	// (ADR 0003).
	listings, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{Query: "USDT", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	platforms := map[string]bool{}
	for _, l := range listings.Items {
		if l.Symbol == "USDT" {
			if l.AssetSymbol != "USDT" || l.MappingSource != catalog.MappingAuto {
				t.Fatalf("USDT listing on %s maps to %s (%s)", l.Platform, l.AssetSymbol, l.MappingSource)
			}
			platforms[l.Platform] = true
		}
	}
	if !platforms["binance"] || !platforms["bybit"] {
		t.Fatalf("USDT should be listed on both platforms: %v", platforms)
	}

	// Break the good key in place: the next cycle must fail with authentication, set the Connection
	// invalid, and carry the Accounts forward as stale.
	broken := h.storeCredentials(enabled, "invalid-rotated-key")
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status = 'enabled' WHERE id = ?", broken.ID[:]); err != nil {
		t.Fatal(err)
	}
	summary, jobID = h.runCycle()
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
	accounts = h.accountsByType(jobID, good.ID)
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

	// The invalid Identity is skipped by the scheduler-facing list and by the next cycle.
	collectable, err := h.identities.ListCollectable(h.ctx, h.portfolioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range collectable {
		if c.Identity.ID == broken.ID {
			t.Fatal("an invalid connection must not be collectable")
		}
	}
	summary, jobID = h.runCycle()
	accounts = h.accountsByType(jobID, good.ID)
	if got := accounts["spot"]; got.Status != "failed" || got.ErrorClass == nil || *got.ErrorClass != ClassSkipped || got.Freshness != valuation.FreshnessStale {
		t.Fatalf("skipped account should carry a skipped failed report: %+v", got)
	}
	if summary.Invalidated != 0 {
		t.Fatalf("a skipped identity must not be invalidated again: %+v", summary)
	}
}

func TestListingExclusionAppliesToFutureCalculatedAccountsOnly(t *testing.T) {
	h := newHarness(t)
	assetIdentity := h.createIdentity("Binance reviewed", adapters.PlatformBinance, adapters.StructureBinancePM, "portfolio_margin", "spot")
	assetIdentity = h.storeCredentials(assetIdentity, "unpriced-reviewed-key")
	assetIdentity, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, assetIdentity.ID, assetIdentity.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK {
		t.Fatalf("enable: %v %+v", err, outcome)
	}
	first, firstJob := h.runCycle()
	firstAccounts := h.accountsByType(firstJob, assetIdentity.ID)
	if first.Point.Completeness() != valuation.CompletenessIncomplete || firstAccounts["spot"].Method != valuation.MethodCalculated || len(firstAccounts["spot"].Omitted) != 1 {
		t.Fatalf("expected TOKENX omission in calculated spot: %+v %+v", first.Point, firstAccounts["spot"])
	}
	if firstAccounts["portfolio_margin"].Method != valuation.MethodReported {
		t.Fatalf("margin should use reported equity: %+v", firstAccounts["portfolio_margin"])
	}

	listings, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Query: "TOKENX", Limit: 50})
	if err != nil || len(listings.Items) != 1 {
		t.Fatalf("TOKENX listing: %+v %v", listings, err)
	}
	token := listings.Items[0]
	input := catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, RequestID: "test", Ignored: true, Reason: "Reviewed dust balance", Version: 0}
	if _, err := h.catalog.PutExclusion(h.ctx, token.ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: true}); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("missing review reason: %v", err)
	}
	ignored, err := h.catalog.PutExclusion(h.ctx, token.ID, &input)
	if err != nil || !ignored.Ignored || ignored.ExclusionVersion != 1 {
		t.Fatalf("ignore TOKENX: %+v %v", ignored, err)
	}
	var auditCount int
	if err := h.collection.db.QueryRowContext(h.ctx, `SELECT COUNT(*) FROM audit_events WHERE portfolio_id = ? AND action = 'catalog.listing.exclude' AND resource_id = ?`, h.portfolioID[:], token.ID[:]).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("review audit event count = %d: %v", auditCount, err)
	}
	ignoredPage, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, IgnoredOnly: true, Limit: 50})
	if err != nil || ignoredPage.Total != 1 || ignoredPage.Items[0].ID != token.ID {
		t.Fatalf("ignored-only catalog filter: %+v %v", ignoredPage, err)
	}
	if _, err := h.catalog.PutExclusion(h.ctx, token.ID, &input); !errors.Is(err, catalog.ErrPrecondition) {
		t.Fatalf("stale exclusion version: %v", err)
	}
	other, err := portfolio.NewService(h.collection.db, 15*time.Minute).Create(h.ctx, h.userID, portfolio.CreateInput{Name: "separate", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	otherListings, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: other.ID, Query: "TOKENX", Limit: 50})
	if err != nil || len(otherListings.Items) != 1 || otherListings.Items[0].Ignored {
		t.Fatalf("exclusion leaked to another Portfolio: %+v %v", otherListings, err)
	}
	oldPoint, err := h.valuation.PointByJob(h.ctx, h.portfolioID, firstJob)
	if err != nil || oldPoint.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("existing point changed before another cycle: %+v %v", oldPoint, err)
	}

	second, secondJob := h.runCycle()
	spot := h.accountsByType(secondJob, assetIdentity.ID)["spot"]
	if second.Point.Completeness() != valuation.CompletenessComplete || len(spot.Omitted) != 0 {
		t.Fatalf("reviewed TOKENX must not cause an omission: %+v %+v", second.Point, spot)
	}
	pctx := valuation.PortfolioContext{ID: h.portfolioID, Timezone: "Asia/Shanghai"}
	flat, err := h.valuation.FlatBalances(h.ctx, pctx, &valuation.ListFilter{GroupBy: "asset"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range flat.Items {
		if row.Symbol == "TOKENX" {
			t.Fatal("ignored TOKENX appeared in ordinary Balances")
		}
	}
	view, err := h.valuation.AccountBalances(h.ctx, pctx, spot.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range view.Balances {
		if row.Symbol == "TOKENX" {
			t.Fatal("ignored TOKENX appeared in Account Balances")
		}
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	oldPoint, err = h.valuation.PointByJob(h.ctx, h.portfolioID, firstJob)
	if err != nil || oldPoint.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("historical rule lost during rebuild: %+v %v", oldPoint, err)
	}
	secondPoint, err := h.valuation.PointByJob(h.ctx, h.portfolioID, secondJob)
	if err != nil || secondPoint.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("ignored cycle lost during rebuild: %+v %v", secondPoint, err)
	}

	priced, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Platform: "binance", Query: "USDC", Limit: 50})
	if err != nil || len(priced.Items) != 1 {
		t.Fatalf("USDC listing: %+v %v", priced, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, priced.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: true, Reason: "Reviewed priced balance", Version: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, thirdJob := h.runCycle()
	thirdSpot := h.accountsByType(thirdJob, assetIdentity.ID)["spot"]
	if thirdSpot.ValueUSD == nil || spot.ValueUSD == nil || *thirdSpot.ValueUSD == *spot.ValueUSD {
		t.Fatalf("priced exclusion did not change calculated account: before=%+v after=%+v", spot, thirdSpot)
	}
	thirdPoint, err := h.valuation.PointByJob(h.ctx, h.portfolioID, thirdJob)
	if err != nil {
		t.Fatal(err)
	}
	change, err := h.valuation.CustomValueChange(h.ctx, pctx, secondPoint.EvaluatedAt, thirdPoint.EvaluatedAt.Add(time.Microsecond))
	if err != nil || change.Completeness != valuation.CompletenessIncomplete || !strings.Contains(strings.Join(change.Reasons, " "), "listing_exclusion_changed") {
		t.Fatalf("treatment change must limit Value Change: %+v %v", change, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, priced.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: false, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, restoredPricedJob := h.runCycle()
	restoredPricedPoint, err := h.valuation.PointByJob(h.ctx, h.portfolioID, restoredPricedJob)
	if err != nil {
		t.Fatal(err)
	}
	change, err = h.valuation.CustomValueChange(h.ctx, pctx, secondPoint.EvaluatedAt, restoredPricedPoint.EvaluatedAt.Add(time.Microsecond))
	if err != nil || change.Completeness != valuation.CompletenessComplete {
		t.Fatalf("matching endpoint rules should allow Value Change: %+v %v", change, err)
	}

	btc, err := h.catalog.ListListings(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Platform: "binance", Query: "BTC", Limit: 50})
	if err != nil || len(btc.Items) != 1 {
		t.Fatalf("BTC listing: %+v %v", btc, err)
	}
	_, err = h.catalog.PutExclusion(h.ctx, btc.Items[0].ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: true, Reason: "Test reported equity", Version: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, fourthJob := h.runCycle()
	margin := h.accountsByType(fourthJob, assetIdentity.ID)["portfolio_margin"]
	if margin.Method != valuation.MethodReported || margin.Equity == nil || margin.ValueUSD == nil || *margin.Equity != *margin.ValueUSD {
		t.Fatalf("reported equity changed by exclusion: %+v", margin)
	}
	marginBalances, err := h.valuation.AccountBalances(h.ctx, pctx, margin.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	foundBTC := false
	for _, row := range marginBalances.Balances {
		foundBTC = foundBTC || row.Symbol == "BTC"
	}
	if !foundBTC {
		t.Fatal("reported-equity Account hid BTC")
	}

	_, err = h.catalog.PutExclusion(h.ctx, token.ID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: false, Version: ignored.ExclusionVersion})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.collection.db.QueryRowContext(h.ctx, `SELECT COUNT(*) FROM audit_events WHERE portfolio_id = ? AND action = 'catalog.listing.restore' AND resource_id = ?`, h.portfolioID[:], token.ID[:]).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("restore audit event count = %d: %v", auditCount, err)
	}
	last, lastJob := h.runCycle()
	if last.Point.Completeness() != valuation.CompletenessIncomplete || len(h.accountsByType(lastJob, assetIdentity.ID)["spot"].Omitted) != 1 {
		t.Fatalf("restored TOKENX did not restore no_price: %+v", last.Point)
	}
}

func TestRateLimitIsRetriedWithoutInvalidating(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("OKX rate limited", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading", "funding")
	item = h.storeCredentials(item, "ratelimit-key")
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE identities SET connection_status = 'enabled' WHERE id = ?", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	summary, jobID := h.runCycle()
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
	accounts := h.accountsByType(jobID, item.ID)
	for kind, a := range accounts {
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
	other, err := auth.NewService(h.collection.db, time.Hour, 24*time.Hour).CreateUser(h.ctx, "other-"+uuid.NewString()[:8], "Other=123456")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := portfolio.NewService(h.collection.db, 15*time.Minute).Get(h.ctx, other.ID, h.portfolioID); !errors.Is(err, portfolio.ErrNotFound) {
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
		t.Fatalf("a new Identity has no Margin Mode before validation, got %q", item.MarginMode)
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
		if err := h.collection.db.QueryRowContext(h.ctx,
			"SELECT COUNT(*) FROM audit_events WHERE resource_id = ? AND action = 'identity.margin_mode.change' AND actor_user_id IS NULL", item.ID[:]).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if changes() != 0 {
		t.Fatal("validation observed the first Margin Mode; that is not a change")
	}

	// A cycle a minute later observes the next Margin Mode, as if the owner switched it on OKX.
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
		t.Fatalf("stored Margin Mode %q after the switch from %q, want %q", after.MarginMode, first, second)
	}
	if changes() != 1 {
		t.Fatal("the switch must be audited once, with the system as the actor")
	}
	h.runCycle()
	if changes() != 1 {
		t.Fatal("an unchanged Margin Mode must not be audited again")
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
		t.Fatalf("the refused key must leave the Connection disabled and the binding unchanged: %+v", refused)
	}

	restored := h.storeCredentials(refused, "key-first-account")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, restored.ID, restored.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("the original account's key must still validate: err=%v outcome=%+v", err, outcome)
	}
}

// foreignModeAdapter validates with a proper OKX Margin Mode but reports mode in a Collection
// Run, as a broken adapter would: another Platform's Margin Mode, or none.
type foreignModeAdapter struct{ mode adapters.MarginMode }

func (foreignModeAdapter) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "foreign", MarginMode: adapters.MarginOKXPM}, nil
}

func (a foreignModeAdapter) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, accounts []adapters.AccountType) (adapters.Batch, error) {
	batch := adapters.Batch{ObservedAt: time.Now().UTC(), MarginMode: a.mode}
	for _, accountType := range accounts {
		batch.Reports = append(batch.Reports, adapters.AccountReport{AccountType: accountType, Status: adapters.StatusOK, ObservedAt: batch.ObservedAt})
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
			summary, jobID := h.runCycle()
			if summary.ReportFailed != 1 || summary.Invalidated != 0 {
				t.Fatalf("summary %+v", summary)
			}
			if got := h.accountsByType(jobID, item.ID)["trading"]; got.ErrorClass == nil || *got.ErrorClass != adapters.ClassUnsupported {
				t.Fatalf("trading report: %+v", got)
			}
			after, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
			if err != nil || after.MarginMode != adapters.MarginOKXPM || after.Connection.Status != identity.ConnectionEnabled {
				t.Fatalf("the stored Margin Mode and the Connection must be kept: err=%v identity=%+v", err, after)
			}
		})
	}
}
