package integration

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

// harness owns public cross-module scenarios, including direct database setup.
// Package-private Collection recovery tests retain their own local assembly.
type harness struct {
	ctx         context.Context
	t           *testing.T
	db          *sql.DB
	userID      uuid.UUID
	portfolioID uuid.UUID
	services    *app.Services
	identities  *identity.Service
	catalog     *catalog.Service
	valuation   *valuation.Service
	financial   *financial.Service
	manual      *manual.Service
	jobs        *jobs.Service
	collection  *collection.Service
	registry    *adapters.Registry
	fakes       map[adapters.Platform]*fake.Adapter
}

func newHarness(t *testing.T, databases ...*sql.DB) *harness {
	t.Helper()
	db := mysqltest.OpenTestDB(t)
	if len(databases) > 0 {
		db = databases[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	registry := adapters.NewRegistry()
	fakes := map[adapters.Platform]*fake.Adapter{}
	for _, platform := range adapters.OfferedPlatforms() {
		fakes[platform] = fake.New(platform)
		registry.Register(platform, fakes[platform])
	}
	cfg := &config.Config{
		Auth:   config.Auth{SessionIdleLifetime: time.Hour, SessionAbsoluteLifetime: 24 * time.Hour},
		Crypto: config.Crypto{MasterKey: []byte("0123456789abcdef0123456789abcdef")},
	}
	services, err := app.New(db, cfg, registry, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	user, err := services.Auth.CreateUser(ctx, "integration-"+uuid.NewString()[:8], "Integration=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios, err := services.Portfolios.List(ctx, user.ID)
	if err != nil || len(portfolios) != 1 {
		t.Fatalf("expected the birth portfolio, got %d (%v)", len(portfolios), err)
	}
	// Scenario retries should be fast. Share the actual assembled collaborators;
	// the assembly scenario below uses services.Collection without this override.
	runner := collection.NewService(db, collection.Options{
		Adapters: registry, Identities: services.Identities, Catalog: services.Catalog,
		Manual: services.Manual, Valuation: services.Valuation,
		RetryDelays: []time.Duration{time.Millisecond, time.Millisecond},
	})
	return &harness{
		ctx: ctx, t: t, db: db, userID: user.ID, portfolioID: portfolios[0].ID,
		services: services, identities: services.Identities, catalog: services.Catalog,
		valuation: services.Valuation, financial: services.Financial, manual: services.Manual,
		jobs: services.Jobs, collection: runner, registry: registry, fakes: fakes,
	}
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

func (h *harness) runCycle() (collection.Summary, uuid.UUID) {
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
