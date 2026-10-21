package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/collection"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/manual"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

type leaseAdapter struct {
	started   chan struct{}
	duplicate bool
}

func (a *leaseAdapter) Validate(context.Context, adapter.Credentials, adapter.AccountStructure) (adapter.ValidationResult, error) {
	return adapter.ValidationResult{ExternalRef: "worker-test"}, nil
}

func (a *leaseAdapter) Collect(ctx context.Context, _ adapter.Credentials, _ adapter.AccountStructure, _ []adapter.AccountType) (adapter.Batch, error) {
	if a.duplicate {
		return adapter.Batch{CollectedAt: time.Now().UTC(), Reports: []adapter.AccountReport{{
			AccountType: "spot", Status: adapter.StatusSucceeded,
			Balances: []adapter.Balance{{AssetSymbol: "USDT", Equity: "1"}, {AssetSymbol: "USDT", Equity: "2"}},
		}}}, nil
	}
	close(a.started)
	<-ctx.Done()
	return adapter.Batch{}, ctx.Err()
}

func TestWorkerStopsAfterLosingLeaseAndDoesNotRetryDuplicates(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "lost_lease"
		if duplicate {
			name = "duplicate_persistence"
		}
		t.Run(name, func(t *testing.T) {
			db := mysqltest.OpenTestDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			user, err := auth.NewService(db, time.Hour, time.Hour).CreateUser(ctx, "worker-"+uuid.NewString()[:8], "Worker=123456")
			if err != nil {
				t.Fatal(err)
			}
			portfolios, err := portfolio.NewService(db).List(ctx, user.ID)
			if err != nil || len(portfolios) != 1 {
				t.Fatalf("portfolios: %v %v", portfolios, err)
			}
			portfolioID := portfolios[0].ID
			platformAdapter := &leaseAdapter{started: make(chan struct{}), duplicate: duplicate}
			registry := adapter.NewRegistry()
			registry.Register(adapter.PlatformBinance, platformAdapter)
			identities, err := identity.NewService(db, identity.Options{MasterKey: []byte("0123456789abcdef0123456789abcdef"), Adapters: registry})
			if err != nil {
				t.Fatal(err)
			}
			item, err := identities.Create(ctx, portfolioID, &identity.CreateInput{ActorUserID: user.ID, Platform: adapter.PlatformBinance, AccountStructure: adapter.StructureBinanceClassic, DisplayName: "Worker", SelectedAccountTypes: []adapter.AccountType{"spot"}})
			if err != nil {
				t.Fatal(err)
			}
			item, err = identities.PutCredentials(ctx, portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: user.ID}, adapter.Credentials{APIKey: "synthetic", APISecret: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			if _, outcome, err := identities.Enable(ctx, portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: user.ID}); err != nil || !outcome.OK {
				t.Fatalf("enable: %+v %v", outcome, err)
			}
			cycleService := cycle.NewService(db)
			collectionCycle, _, err := cycleService.Enqueue(ctx, portfolioID, cycle.TriggerUser)
			if err != nil {
				t.Fatal(err)
			}
			value := valuation.NewService(db)
			assets := catalog.NewService(db)
			collect := collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: assets, Manual: manual.NewService(db, assets), Valuation: value, RetryDelays: []time.Duration{}})
			w := New(&Options{Cycles: cycleService, Collection: collect, Lease: 3 * time.Second})
			if duplicate {
				if ok, err := w.ProcessOnce(ctx); err != nil || !ok {
					t.Fatalf("process: %v %v", ok, err)
				}
				current, err := cycleService.Get(ctx, portfolioID, collectionCycle.ID)
				if err != nil || current.Status != cycle.StatusFailed || current.AttemptCount != 1 || current.ErrorClass == nil || *current.ErrorClass != "persistence" {
					t.Fatalf("duplicate was retried: %+v %v", current, err)
				}
				return
			}
			done := make(chan error, 1)
			go func() { _, err := w.ProcessOnce(ctx); done <- err }()
			select {
			case <-platformAdapter.started:
			case <-ctx.Done():
				t.Fatal("collection did not start")
			}
			if _, err := db.ExecContext(ctx, "UPDATE collection_cycles SET lease_owner='replacement' WHERE portfolio_id=? AND id=?", portfolioID[:], collectionCycle.ID[:]); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, cycle.ErrLostLease) {
					t.Fatalf("lost lease was ignored: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("lease renewal failure did not cancel collection")
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports r JOIN accounts a ON a.id=r.account_id WHERE r.cycle_id=? AND a.account_type <> 'manual'", collectionCycle.ID[:]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("lost owner wrote reports: %d %v", count, err)
			}
		})
	}
}

func TestSafeDetailKeepsValidUTF8(t *testing.T) {
	t.Parallel()

	detail := safeDetail(errors.New(strings.Repeat("错", 299) + "\xff" + strings.Repeat("误", 10)))
	if !utf8.ValidString(detail) || utf8.RuneCountInString(detail) != 300 || !strings.HasSuffix(detail, "\uFFFD") {
		t.Fatalf("detail %q", detail)
	}
}
