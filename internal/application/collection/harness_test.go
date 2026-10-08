package collection

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

type harness struct {
	ctx         context.Context
	t           *testing.T
	userID      uuid.UUID
	portfolioID uuid.UUID
	identities  *identity.Service
	catalog     *catalog.Service
	valuation   *valuation.Service
	financial   *financial.Service
	jobs        *jobs.Service
	collection  *Service
	fakes       map[adapters.Platform]*fake.Adapter
	registry    *adapters.Registry
}

// newHarness creates a fresh User and Portfolio in the shared test database and wires the
// services around fake adapters with an instant retry policy.
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
	portfolios, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil || len(portfolios) != 1 {
		t.Fatalf("expected the birth portfolio, got %d (%v)", len(portfolios), err)
	}
	catalogSvc := catalog.NewService(db, jobs.NewService(db))
	valuationSvc := valuation.NewService(db)
	h := &harness{
		ctx: ctx, t: t, userID: user.ID, portfolioID: portfolios[0].ID,
		identities: identities, catalog: catalogSvc, valuation: valuationSvc, financial: financial.NewService(db, valuationSvc), jobs: jobs.NewService(db), fakes: fakes, registry: registry,
	}
	h.collection = NewService(db, Options{
		Adapters: registry, Identities: identities, Catalog: catalogSvc, Manual: manual.NewService(db, catalogSvc), Valuation: valuationSvc,
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

type testAdapter struct {
	adapters.Adapter
	calls   [][]adapters.AccountType
	collect func([]adapters.AccountType) (adapters.Batch, error)
}

func (a *testAdapter) Collect(ctx context.Context, credentials adapters.Credentials, structure adapters.AccountStructure, types []adapters.AccountType) (adapters.Batch, error) {
	a.calls = append(a.calls, append([]adapters.AccountType(nil), types...))
	if a.collect != nil {
		return a.collect(types)
	}
	return a.Adapter.Collect(ctx, credentials, structure, types)
}
