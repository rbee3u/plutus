package collection

import (
	"context"
	"errors"
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
	for _, platform := range []adapters.Platform{adapters.PlatformBinance, adapters.PlatformOKX, adapters.PlatformBybit} {
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
		identities: identities, catalog: catalogSvc, valuation: valuationSvc, jobs: jobs.NewService(db), fakes: fakes,
	}
	h.collection = NewService(db, Options{
		Adapters: registry, Identities: identities, Catalog: catalogSvc, Valuation: valuationSvc,
		Stagger: 0, RetryDelays: []time.Duration{time.Millisecond, time.Millisecond},
	})
	return h
}

func (h *harness) createIdentity(name string, platform adapters.Platform, mode adapters.AccountMode, types ...adapters.AccountType) identity.Identity {
	h.t.Helper()
	item, err := h.identities.Create(h.ctx, h.portfolioID, &identity.CreateInput{
		ActorUserID: h.userID, RequestID: "test", Platform: platform, DisplayName: name, AccountMode: mode, SelectedAccountTypes: types,
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
	item := h.createIdentity("OKX bad key", adapters.PlatformOKX, adapters.ModeOKXPM, "trading")
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
	good := h.createIdentity("Binance good", adapters.PlatformBinance, adapters.ModeBinancePM, "portfolio_margin", "spot")
	good = h.storeCredentials(good, "seed-good-key")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, good.ID, good.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || enabled.Connection.Status != identity.ConnectionEnabled {
		t.Fatalf("enable good key: err=%v outcome=%+v status=%q", err, outcome, enabled.Connection.Status)
	}
	flaky := h.createIdentity("Bybit flaky", adapters.PlatformBybit, adapters.ModeBybitUTA2, "unified")
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
	report := scenario.Report("spot", *spot.ReportObservedAt, false)
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

	// USDT on a second Platform maps to the same Asset symbol automatically (ADR 0018).
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

func TestRateLimitIsRetriedWithoutInvalidating(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("OKX rate limited", adapters.PlatformOKX, adapters.ModeOKXMCM, "trading", "funding")
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
