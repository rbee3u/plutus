package collection

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/manual"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/valuation"
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
	cycles      *cycle.Service
	collection  *Service
	fakes       map[adapter.Platform]*fake.Adapter
	registry    *adapter.Registry
}

// newHarness creates a fresh user and portfolio in the shared test database and wires the
// services around fake adapters with an instant retry policy.
func newHarness(t *testing.T, databases ...*sql.DB) *harness {
	t.Helper()
	db := mysqltest.OpenTestDB(t)
	if len(databases) > 0 {
		db = databases[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	registry := adapter.NewRegistry()
	fakes := map[adapter.Platform]*fake.Adapter{}
	for _, platform := range adapter.OfferedPlatforms() {
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
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	h := &harness{
		ctx: ctx, t: t, userID: user.ID, portfolioID: portfolios[0].ID,
		identities: identities, catalog: catalogSvc, valuation: valuationSvc, financial: financial.NewService(db, valuationSvc), cycles: cycle.NewService(db), fakes: fakes, registry: registry,
	}
	h.collection = NewService(db, Options{
		Adapters: registry, Identities: identities, Catalog: catalogSvc, Manual: manual.NewService(db, catalogSvc), Valuation: valuationSvc,
		Cycles: h.cycles, Stagger: 0, RetryDelays: []time.Duration{time.Millisecond, time.Millisecond},
	})
	return h
}

func (h *harness) createIdentity(name string, platform adapter.Platform, structure adapter.AccountStructure, types ...adapter.AccountType) identity.Identity {
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
	stored, err := h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID, RequestID: "test"}, adapter.Credentials{APIKey: apiKey, APISecret: "secret"})
	if err != nil {
		h.t.Fatalf("store credentials for %s: %v", item.DisplayName, err)
	}
	return stored
}

func (h *harness) runCycle() (Summary, uuid.UUID) {
	h.t.Helper()
	collectionCycle, _, err := h.cycles.Enqueue(h.ctx, h.portfolioID, cycle.TriggerUser)
	if err != nil {
		h.t.Fatal(err)
	}
	// Install a lease on this fixture's cycle without claiming unrelated test work.
	now := time.Now().UTC()
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE collection_cycles SET status='running',lease_owner='test-worker',lease_expires_at=?,attempt_count=1,started_at=? WHERE portfolio_id=? AND id=?", now.Add(time.Minute), now, h.portfolioID[:], collectionCycle.ID[:]); err != nil {
		h.t.Fatal(err)
	}
	collectionCycle.Status, collectionCycle.AttemptCount = cycle.StatusRunning, 1
	claimed := cycle.Claim{Cycle: collectionCycle, WorkerID: "test-worker"}
	summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, collectionCycle.ID, &claimed)
	if err != nil {
		h.t.Fatalf("run cycle: %v", err)
	}
	return summary, collectionCycle.ID
}

func (h *harness) accountsByType(cycleID uuid.UUID, identityID uuid.UUID) map[string]valuation.AccountValuation {
	h.t.Helper()
	accounts, err := h.valuation.Accounts(h.ctx, h.portfolioID, cycleID)
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
	adapter.Adapter
	calls   [][]adapter.AccountType
	collect func([]adapter.AccountType) (adapter.Batch, error)
}

func (a *testAdapter) Collect(ctx context.Context, credentials adapter.Credentials, structure adapter.AccountStructure, types []adapter.AccountType) (adapter.Batch, error) {
	a.calls = append(a.calls, append([]adapter.AccountType(nil), types...))
	if a.collect != nil {
		return a.collect(types)
	}
	return a.Adapter.Collect(ctx, credentials, structure, types)
}
