package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

type leaseAdapter struct {
	started   chan struct{}
	duplicate bool
}

func (a *leaseAdapter) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "worker-test"}, nil
}

func (a *leaseAdapter) Collect(ctx context.Context, _ adapters.Credentials, _ adapters.AccountStructure, _ []adapters.AccountType) (adapters.Batch, error) {
	if a.duplicate {
		return adapters.Batch{ObservedAt: time.Now().UTC(), Reports: []adapters.AccountReport{{
			AccountType: "spot", Status: adapters.StatusOK,
			Balances: []adapters.Balance{{Symbol: "USDT", Equity: "1"}, {Symbol: "USDT", Equity: "2"}},
		}}}, nil
	}
	close(a.started)
	<-ctx.Done()
	return adapters.Batch{}, ctx.Err()
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
			adapter := &leaseAdapter{started: make(chan struct{}), duplicate: duplicate}
			registry := adapters.NewRegistry()
			registry.Register(adapters.PlatformBinance, adapter)
			identities, err := identity.NewService(db, identity.Options{MasterKey: []byte("0123456789abcdef0123456789abcdef"), Adapters: registry})
			if err != nil {
				t.Fatal(err)
			}
			item, err := identities.Create(ctx, portfolioID, &identity.CreateInput{ActorUserID: user.ID, Platform: adapters.PlatformBinance, AccountStructure: adapters.StructureBinanceClassic, DisplayName: "Worker", SelectedAccountTypes: []adapters.AccountType{"spot"}})
			if err != nil {
				t.Fatal(err)
			}
			item, err = identities.PutCredentials(ctx, portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: user.ID}, adapters.Credentials{APIKey: "synthetic", APISecret: "synthetic"})
			if err != nil {
				t.Fatal(err)
			}
			if _, outcome, err := identities.Enable(ctx, portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: user.ID}); err != nil || !outcome.OK {
				t.Fatalf("enable: %+v %v", outcome, err)
			}
			jobService := jobs.NewService(db)
			job, _, err := jobService.EnqueueCycle(ctx, portfolioID, jobs.TriggerManual, nil)
			if err != nil {
				t.Fatal(err)
			}
			value := valuation.NewService(db)
			listings := catalog.NewService(db, jobs.NewService(db))
			collect := collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: listings, Manual: manual.NewService(db, listings), Valuation: value, RetryDelays: []time.Duration{}})
			w := New(&Options{Jobs: jobService, Collection: collect, Valuation: value, Lease: 3 * time.Second})
			if duplicate {
				if ok, err := w.ProcessOnce(ctx); err != nil || !ok {
					t.Fatalf("process: %v %v", ok, err)
				}
				current, err := jobService.Get(ctx, portfolioID, job.ID)
				if err != nil || current.Status != jobs.StatusFailed || current.AttemptCount != 1 || current.LastErrorClass == nil || *current.LastErrorClass != "persistence" {
					t.Fatalf("duplicate was retried: %+v %v", current, err)
				}
				return
			}
			done := make(chan error, 1)
			go func() { _, err := w.ProcessOnce(ctx); done <- err }()
			select {
			case <-adapter.started:
			case <-ctx.Done():
				t.Fatal("collection did not start")
			}
			if _, err := db.ExecContext(ctx, "UPDATE jobs SET lease_owner='replacement' WHERE id=?", job.ID[:]); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, jobs.ErrLostLease) {
					t.Fatalf("lost lease was ignored: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("lease renewal failure did not cancel collection")
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_reports r JOIN accounts a ON a.id=r.account_id WHERE r.job_id=? AND a.account_type <> 'manual'", job.ID[:]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("lost owner wrote reports: %d %v", count, err)
			}
		})
	}
}
