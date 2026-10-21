package collection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// recoverySource supplies fixed successful reports; cutover and failure scenarios
// belong to the integration suite rather than this recovery fixture.
type recoverySource struct{}

func (recoverySource) Validate(context.Context, adapter.Credentials, adapter.AccountStructure) (adapter.ValidationResult, error) {
	return adapter.ValidationResult{ExternalRef: "dual-test", MarginMode: adapter.MarginOKXPM}, nil
}

func (recoverySource) Collect(_ context.Context, _ adapter.Credentials, _ adapter.AccountStructure, types []adapter.AccountType) (adapter.Batch, error) {
	now := time.Now().UTC()
	batch := adapter.Batch{CollectedAt: now, MarginMode: adapter.MarginOKXPM}
	for _, kind := range types {
		report := adapter.AccountReport{AccountType: kind, Status: adapter.StatusSucceeded, CollectedAt: now}
		if kind == "earn" {
			report.Balances = []adapter.Balance{{AssetSymbol: "USDT", Equity: "60"}}
			p := adapter.NewEarnPosition("earn", "flexible", "position", "principal")
			p.Assets = []adapter.EarnAmount{{AssetSymbol: "USDT", Amount: report.Balances[0].Equity}}
			report.EarnPositions = []adapter.EarnPosition{p}
		} else {
			report.Balances = []adapter.Balance{{AssetSymbol: "USDT", Equity: "40"}}
			expiry := now.Add(24 * time.Hour)
			report.DualPositions = []adapter.DualPosition{
				{PositionKey: "one", PrincipalAssetSymbol: "USDT", Principal: "15", Status: "active"},
				{PositionKey: "two", PrincipalAssetSymbol: "USDT", Principal: "25", Status: "active", ExpiryAt: &expiry},
			}
		}
		batch.Reports = append(batch.Reports, report)
	}
	return batch, nil
}

func enableRecoveryIdentity(h *harness, name string, platform adapter.Platform, structure adapter.AccountStructure, types ...adapter.AccountType) identity.Identity {
	h.t.Helper()
	item := h.createIdentity(name, platform, structure, types...)
	item = h.storeCredentials(item, "seed-"+name)
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK {
		h.t.Fatalf("enable: %+v %v", outcome, err)
	}
	return enabled
}

func claimRecoveryCycle(h *harness) cycle.Claim {
	h.t.Helper()
	collectionCycle, _, err := h.cycles.Enqueue(h.ctx, h.portfolioID, cycle.TriggerUser)
	if err != nil {
		h.t.Fatal(err)
	}
	claimed, ok, err := h.cycles.ClaimOne(h.ctx, "before-restart", time.Minute)
	if err != nil || !ok || claimed.ID != collectionCycle.ID {
		h.t.Fatalf("claim: %+v %v %v", claimed, ok, err)
	}
	return claimed
}

func reclaimRecoveryCycle(h *harness, cycleID uuid.UUID) cycle.Claim {
	h.t.Helper()
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE collection_cycles SET lease_expires_at=? WHERE id=?", time.Now().UTC().Add(-time.Minute), cycleID[:]); err != nil {
		h.t.Fatal(err)
	}
	claimed, ok, err := h.cycles.ClaimOne(h.ctx, "after-restart", time.Minute)
	if err != nil || !ok || claimed.ID != cycleID || claimed.AttemptCount != 2 {
		h.t.Fatalf("reclaim: %+v %v %v", claimed, ok, err)
	}
	return claimed
}

func TestCycleResumesCommittedIdentityAfterLeaseExpiry(t *testing.T) {
	h := newHarness(t)
	first := &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}
	second := &testAdapter{Adapter: h.fakes[adapter.PlatformBybit]}
	h.registry.Register(adapter.PlatformBinance, first)
	h.registry.Register(adapter.PlatformBybit, second)
	enableRecoveryIdentity(h, "First", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot", "um_futures", adapter.TypeDualInvestment)
	enableRecoveryIdentity(h, "Second", adapter.PlatformBybit, adapter.StructureBybitUTA2, "funding")
	claimed := claimRecoveryCycle(h)
	h.collection.stagger = time.Second
	h.collection.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected interruption after first identity: %v", err)
	}
	if _, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, claimed.ID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("interrupted cycle has a point: %v", err)
	}
	rows, err := h.collection.q.ListReportsByCycle(h.ctx, sqlcgen.ListReportsByCycleParams{PortfolioID: h.portfolioID[:], CycleID: claimed.ID[:]})
	if err != nil || len(rows) != 5 {
		t.Fatalf("committed reports: %d %v", len(rows), err)
	}
	// Capture the original facts and all three kinds of holdings before recovery.
	type holdings struct {
		balances  []sqlcgen.ListBalancesByReportRow
		positions []sqlcgen.ListPositionsByReportRow
		dual      []sqlcgen.DualPosition
	}
	readHoldings := func(reportID []byte) holdings {
		t.Helper()
		balances, err := h.collection.q.ListBalancesByReport(h.ctx, sqlcgen.ListBalancesByReportParams{PortfolioID: h.portfolioID[:], ReportID: reportID})
		if err != nil {
			t.Fatal(err)
		}
		positions, err := h.collection.q.ListPositionsByReport(h.ctx, sqlcgen.ListPositionsByReportParams{PortfolioID: h.portfolioID[:], ReportID: reportID})
		if err != nil {
			t.Fatal(err)
		}
		dual, err := h.collection.q.ListDualPositionsByReport(h.ctx, sqlcgen.ListDualPositionsByReportParams{PortfolioID: h.portfolioID[:], ReportID: reportID})
		if err != nil {
			t.Fatal(err)
		}
		return holdings{balances, positions, dual}
	}
	before := map[string]holdings{}
	var balances, positions, dual int
	for _, r := range rows {
		facts := readHoldings(r.ID)
		before[string(r.ID)] = facts
		balances += len(facts.balances)
		positions += len(facts.positions)
		dual += len(facts.dual)
	}
	if balances == 0 || positions == 0 || dual == 0 {
		t.Fatalf("fixture must include every kind of holding: %d %d %d", balances, positions, dual)
	}
	reclaimed := reclaimRecoveryCycle(h, claimed.ID)
	h.collection.stagger = 0
	summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, reclaimed.ID)
	if err != nil || summary.ReportCount != 6 || summary.ReportFailed != 0 || summary.IdentityCount != 2 || summary.Point.AccountCount != 6 {
		t.Fatalf("recovered cycle: %+v %v", summary, err)
	}
	if len(first.calls) != 1 || len(second.calls) != 1 {
		t.Fatalf("completed identity was recollected: %d %d", len(first.calls), len(second.calls))
	}
	for _, r := range rows {
		after, err := h.collection.q.GetReport(h.ctx, sqlcgen.GetReportParams{PortfolioID: h.portfolioID[:], ID: r.ID})
		if err != nil || after.Status != r.Status || after.CollectedAt != r.CollectedAt || after.ReportedEquityUsd != r.ReportedEquityUsd || after.RecordedAt != r.RecordedAt {
			t.Fatalf("persisted observation changed: %+v %v", after, err)
		}
		if !reflect.DeepEqual(before[string(r.ID)], readHoldings(r.ID)) {
			t.Fatal("recovery changed persisted holdings")
		}
	}
	if err := h.cycles.Finish(h.ctx, &reclaimed, cycle.Result{OK: true}); err != nil {
		t.Fatal(err)
	}
}

func TestCycleReusesPointBeforeFinish(t *testing.T) {
	h := newHarness(t)
	platformAdapter := &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}
	h.registry.Register(adapter.PlatformBinance, platformAdapter)
	enableRecoveryIdentity(h, "Completed", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	claimed := claimRecoveryCycle(h)
	before, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Boundary changes after the point was committed must not reopen its cycle.
	enableRecoveryIdentity(h, "Later", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	reclaimed := reclaimRecoveryCycle(h, claimed.ID)
	after, err := h.collection.RunCycle(h.ctx, h.portfolioID, reclaimed.ID)
	if err != nil || !reflect.DeepEqual(before, after) || len(platformAdapter.calls) != 1 {
		t.Fatalf("completed point changed: before=%+v after=%+v calls=%d err=%v", before, after, len(platformAdapter.calls), err)
	}
	if err := h.cycles.Finish(h.ctx, &reclaimed, cycle.Result{OK: true}); err != nil {
		t.Fatal(err)
	}
	collectionCycle, err := h.cycles.Get(h.ctx, h.portfolioID, claimed.ID)
	if err != nil || collectionCycle.Status != cycle.StatusSucceeded {
		t.Fatalf("cycle: %+v %v", collectionCycle, err)
	}
}

func TestCycleResumesPartialAccountReports(t *testing.T) {
	for _, status := range []string{adapter.StatusSucceeded, adapter.StatusFailed, ClassSkipped} {
		t.Run(status, func(t *testing.T) {
			h := newHarness(t)
			platformAdapter := &testAdapter{Adapter: recoverySource{}}
			h.registry.Register(adapter.PlatformOKX, platformAdapter)
			item := enableRecoveryIdentity(h, "Partial", adapter.PlatformOKX, adapter.StructureOKXUnified, "earn", adapter.TypeDualInvestment)
			_, previousCycle := h.runCycle()
			previous := h.accountsByType(previousCycle, item.ID)
			claimed := claimRecoveryCycle(h)
			accountID := previous["earn"].AccountID
			now := time.Now().UTC().Truncate(time.Microsecond)
			if status == adapter.StatusSucceeded {
				tx, err := h.collection.db.BeginTx(h.ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback() }()
				report := adapter.AccountReport{AccountType: "earn", Status: adapter.StatusSucceeded, Balances: []adapter.Balance{{AssetSymbol: "USDT", Equity: "60"}}}
				p := adapter.NewEarnPosition("earn", "flexible", "position", "principal")
				p.Assets = []adapter.EarnAmount{{AssetSymbol: "USDT", Amount: "60"}}
				report.EarnPositions = []adapter.EarnPosition{p}
				if err := h.collection.insertSuccessfulReport(h.ctx, h.collection.q.WithTx(tx), h.portfolioID, claimed.ID, adapter.PlatformOKX, accountID, &report, adapter.MarginOKXPM, now, now, sql.NullInt64{}); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			} else {
				class := adapter.ClassTemporary
				if status == ClassSkipped {
					class = ClassSkipped
				}
				if err := insertFailedReport(h.ctx, h.collection.q, h.portfolioID, claimed.ID, accountID, class, "Original failure", now); err != nil {
					t.Fatal(err)
				}
			}
			var original []byte
			if err := h.collection.db.QueryRowContext(h.ctx, "SELECT id FROM account_reports WHERE portfolio_id=? AND cycle_id=? AND account_id=?", h.portfolioID[:], claimed.ID[:], accountID[:]).Scan(&original); err != nil {
				t.Fatal(err)
			}
			platformAdapter.calls = nil
			if status == ClassSkipped {
				if _, err := h.identities.Disable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
					t.Fatal(err)
				}
			}
			// Model an ordinary infrastructure retry as well as restart recovery.
			if err := h.cycles.Finish(h.ctx, &claimed, cycle.Result{Retry: true, ErrorClass: "temporary", ErrorDetail: "Interrupted"}); err != nil {
				t.Fatal(err)
			}
			reclaimed, ok, err := h.cycles.ClaimOne(h.ctx, "retry", time.Minute)
			if err != nil || !ok || reclaimed.ID != claimed.ID {
				t.Fatalf("retry claim: %v %v", ok, err)
			}
			summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID)
			if err != nil || summary.ReportCount != 3 || summary.Point.TotalUSD != "100" {
				t.Fatalf("resume: %+v %v", summary, err)
			}
			accounts := h.accountsByType(claimed.ID, item.ID)
			earn := accounts["earn"]
			if string(original) != string(earn.ReportID[:]) {
				t.Fatal("persisted report was replaced")
			}
			if status == adapter.StatusSucceeded {
				if summary.ReportFailed != 0 || earn.Freshness != valuation.FreshnessFresh || !earn.ReportObservedAt.Equal(now) {
					t.Fatalf("fresh report changed: %+v", earn)
				}
			} else {
				if earn.Freshness != valuation.FreshnessStale || earn.SourceReportID == nil || *earn.SourceReportID != previous["earn"].ReportID || earn.ErrorDetail == nil || *earn.ErrorDetail != "Original failure" {
					t.Fatalf("failure overwritten: %+v", earn)
				}
			}
			if status == ClassSkipped {
				if len(platformAdapter.calls) != 0 || summary.ReportFailed != 2 || summary.IdentityCount != 0 {
					t.Fatalf("skipped recovery: %+v calls=%v", summary, platformAdapter.calls)
				}
			} else if !reflect.DeepEqual(platformAdapter.calls, [][]adapter.AccountType{{adapter.TypeDualInvestment}}) {
				t.Fatalf("recollected a committed account: %v", platformAdapter.calls)
			}
			if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE collection_cycles SET collection_finished_at=UTC_TIMESTAMP(6) WHERE id=?", reclaimed.ID[:]); err != nil {
				t.Fatal(err)
			}
			if err := h.cycles.Finish(h.ctx, &reclaimed, cycle.Result{OK: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIdentityReportsAndOutcomeRollbackTogether(t *testing.T) {
	for _, result := range []string{"success", "missing_report", "authentication"} {
		t.Run(result, func(t *testing.T) {
			h := newHarness(t)
			platformAdapter := &testAdapter{Adapter: recoverySource{}}
			platformAdapter.collect = func(types []adapter.AccountType) (adapter.Batch, error) {
				if result == "authentication" {
					return adapter.Batch{}, adapter.NewError(adapter.ClassAuthentication, "Invalid key")
				}
				batch := adapter.Batch{CollectedAt: time.Now().UTC(), MarginMode: adapter.MarginOKXMCM}
				for i, kind := range types {
					if result == "missing_report" && i > 0 {
						continue
					}
					batch.Reports = append(batch.Reports, adapter.AccountReport{AccountType: kind, Status: adapter.StatusSucceeded, Balances: []adapter.Balance{{AssetSymbol: "USDT", Equity: "10"}}})
				}
				return batch, nil
			}
			h.registry.Register(adapter.PlatformOKX, platformAdapter)
			item := enableRecoveryIdentity(h, "Atomic", adapter.PlatformOKX, adapter.StructureOKXUnified, "trading", "funding")
			claimed := claimRecoveryCycle(h)
			trigger := "collection_recovery_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			statement := fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON identities FOR EACH ROW BEGIN IF NEW.id = X'%x' AND NOT (NEW.last_collection_finished_at <=> OLD.last_collection_finished_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Injected collection outcome failure'; END IF; END", trigger, item.ID[:])
			if _, err := h.collection.db.ExecContext(h.ctx, statement); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := h.collection.db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+trigger); err != nil {
					t.Error(err)
				}
			})
			if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID); err == nil {
				t.Fatal("expected outcome write failure")
			}
			var count int
			if err := h.collection.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM account_reports r JOIN accounts a ON a.id=r.account_id WHERE r.cycle_id=? AND a.account_type <> 'manual'", claimed.ID[:]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("reports escaped rollback: %d %v", count, err)
			}
			current, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
			if err != nil || current.Connection.Status != identity.ConnectionEnabled || current.MarginMode != item.MarginMode || current.Connection.LastCollectionFinishedAt != nil {
				t.Fatalf("outcome escaped rollback: %+v %v", current, err)
			}
			if err := h.collection.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM audit_events WHERE resource_id=? AND action='identity.margin_mode.change'", item.ID[:]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("audit escaped rollback: %d %v", count, err)
			}
			if _, err := h.collection.db.ExecContext(h.ctx, "DROP TRIGGER "+trigger); err != nil {
				t.Fatal(err)
			}
			summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID)
			if err != nil || summary.ReportCount != 3 {
				t.Fatalf("retry after rollback: %+v %v", summary, err)
			}
			if result == "authentication" && summary.Invalidated != 1 {
				t.Fatalf("authentication outcome missing: %+v", summary)
			}
			if result == "missing_report" && summary.ReportFailed != 1 {
				t.Fatalf("missing report not recorded: %+v", summary)
			}
			if err := h.cycles.Finish(h.ctx, &claimed, cycle.Result{OK: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFinishRejectsLostLease(t *testing.T) {
	for _, result := range []cycle.Result{{OK: true}, {Retry: true, ErrorClass: "temporary"}, {ErrorClass: "persistence"}} {
		h := newHarness(t)
		claimed := claimRecoveryCycle(h)
		reclaimed := reclaimRecoveryCycle(h, claimed.ID)
		if err := h.cycles.Finish(h.ctx, &claimed, result); !errors.Is(err, cycle.ErrLostLease) {
			t.Fatalf("stale owner finish: %v", err)
		}
		collectionCycle, err := h.cycles.Get(h.ctx, h.portfolioID, claimed.ID)
		if err != nil || collectionCycle.Status != cycle.StatusRunning || collectionCycle.AttemptCount != 2 || collectionCycle.ErrorDetail != nil {
			t.Fatalf("new owner's cycle changed: %+v %v", collectionCycle, err)
		}
		if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE collection_cycles SET collection_finished_at=UTC_TIMESTAMP(6) WHERE id=?", reclaimed.ID[:]); err != nil {
			t.Fatal(err)
		}
		if err := h.cycles.Finish(h.ctx, &reclaimed, cycle.Result{OK: true}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrozenCycleRetryDoesNotCollectNewBoundary(t *testing.T) {
	h := newHarness(t)
	platformAdapter := &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}
	h.registry.Register(adapter.PlatformBinance, platformAdapter)
	enableRecoveryIdentity(h, "Original", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	claimed := claimRecoveryCycle(h)
	const trigger = "reject_frozen_point"
	if _, err := h.collection.db.ExecContext(h.ctx, "CREATE TRIGGER "+trigger+" BEFORE INSERT ON valuation_points FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='point rejected'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = h.collection.db.ExecContext(context.Background(), "DROP TRIGGER IF EXISTS "+trigger) })
	if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID, &claimed); err == nil {
		t.Fatal("point failure ignored")
	}
	collectionCycle, err := h.collection.q.GetCollectionCycle(h.ctx, sqlcgen.GetCollectionCycleParams{PortfolioID: h.portfolioID[:], ID: claimed.ID[:]})
	if err != nil || !collectionCycle.CollectionFinishedAt.Valid {
		t.Fatalf("reports not frozen: %+v %v", collectionCycle, err)
	}
	enableRecoveryIdentity(h, "New boundary", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	if _, err := h.collection.db.ExecContext(h.ctx, "DROP TRIGGER "+trigger); err != nil {
		t.Fatal(err)
	}
	reclaimed := reclaimRecoveryCycle(h, claimed.ID)
	result, err := h.collection.RunCycle(h.ctx, h.portfolioID, reclaimed.ID, &reclaimed)
	if err != nil || result.Point.AccountCount != 2 || len(platformAdapter.calls) != 1 || !result.Point.EvaluatedAt.Equal(collectionCycle.CollectionFinishedAt.Time) {
		t.Fatalf("retry reopened frozen reports: %+v calls=%d %v", result, len(platformAdapter.calls), err)
	}
	finished, err := h.cycles.Get(h.ctx, h.portfolioID, claimed.ID)
	if err != nil || finished.Status != cycle.StatusSucceeded {
		t.Fatalf("point and cycle did not commit together: %+v %v", finished, err)
	}
}

func TestOldCredentialFailureCannotInvalidateEditedIdentity(t *testing.T) {
	h := newHarness(t)
	item := enableRecoveryIdentity(h, "credentials-race", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	platformAdapter := &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}
	platformAdapter.collect = func([]adapter.AccountType) (adapter.Batch, error) {
		item = h.storeCredentials(item, "replacement")
		return adapter.Batch{}, adapter.NewError(adapter.ClassAuthentication, "Old credentials rejected.")
	}
	h.registry.Register(adapter.PlatformBinance, platformAdapter)
	_, collectionCycle := h.runCycle()
	latest, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || latest.Connection.Status != "disabled" || latest.Connection.LastErrorClass != nil {
		t.Fatalf("obsolete response overwrote edited identity: %+v %v", latest, err)
	}
	accounts := h.accountsByType(collectionCycle, item.ID)
	if accounts["spot"].Status != "failed" {
		t.Fatal("historical failure was lost")
	}
}

type mappedPriceSource struct {
	*testAdapter
	prices func([]string) ([]adapter.Price, error)
}

func (s *mappedPriceSource) CollectPrices(_ context.Context, symbols []string) ([]adapter.Price, error) {
	return s.prices(symbols)
}

func TestMappedPricesAreCollectedBeforeReportCutoff(t *testing.T) {
	h := newHarness(t)
	item := enableRecoveryIdentity(h, "mapped-prices", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	id, _, err := h.catalog.EnsureAsset(h.ctx, h.collection.q, h.portfolioID, adapter.PlatformBinance, "SOURCE", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutMapping(h.ctx, id, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: h.portfolioID, MappingVersion: 1, CanonicalAssetSymbol: "TARGET"}); err != nil {
		t.Fatal(err)
	}
	collected := false
	at := time.Now().UTC().Add(-time.Minute)
	platformAdapter := &mappedPriceSource{testAdapter: &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}}
	platformAdapter.collect = func([]adapter.AccountType) (adapter.Batch, error) {
		collected = true
		return adapter.Batch{CollectedAt: at, Reports: []adapter.AccountReport{{AccountType: "spot", Status: adapter.StatusSucceeded, CollectedAt: at, Balances: []adapter.Balance{{AssetSymbol: "SOURCE", Equity: "2"}}}}}, nil
	}
	platformAdapter.prices = func(symbols []string) ([]adapter.Price, error) {
		if !collected || !reflect.DeepEqual(symbols, []string{"TARGET"}) {
			t.Fatalf("public prices preceded private collection or used wrong symbols: %v", symbols)
		}
		return []adapter.Price{{CanonicalAssetSymbol: "TARGET", Price: "3", QuoteCanonicalAssetSymbol: "USD", SourceMarketRef: "TARGETUSD", RetrievedAt: time.Now().UTC()}}, nil
	}
	h.registry.Register(adapter.PlatformBinance, platformAdapter)
	summary, collectionCycle := h.runCycle()
	if summary.Point.TotalUSD != "6" || summary.Point.IncompleteAccountCount != 0 {
		t.Fatalf("supplemental price excluded by cutoff: %+v", summary)
	}
	account := h.accountsByType(collectionCycle, item.ID)["spot"]
	if account.ReportObservedAt == nil || account.ReportObservedAt.Sub(at) > time.Microsecond {
		t.Fatalf("private observation time changed: %+v", account)
	}
}

func TestDeletionRejectsInFlightCollectionCommit(t *testing.T) {
	h := newHarness(t)
	portfolios := portfolio.NewService(h.collection.db)
	p, err := portfolios.Create(h.ctx, h.userID, portfolio.CreateInput{Name: "in-flight-delete", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	h.portfolioID = p.ID
	enableRecoveryIdentity(h, "delete-race", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	platformAdapter := &testAdapter{Adapter: h.fakes[adapter.PlatformBinance]}
	before := map[string]int{}
	platformAdapter.collect = func(kinds []adapter.AccountType) (adapter.Batch, error) {
		for _, table := range []string{"account_reports", "valuation_points", "price_observations"} {
			var n int
			if err := h.collection.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM "+table+" WHERE portfolio_id=?", p.ID[:]).Scan(&n); err != nil {
				t.Fatal(err)
			}
			before[table] = n
		}
		if err := portfolios.Delete(h.ctx, h.userID, p.ID, p.Version); err != nil {
			t.Fatal(err)
		}
		return adapter.Batch{CollectedAt: time.Now().UTC(), Reports: []adapter.AccountReport{{AccountType: "spot", Status: adapter.StatusSucceeded, Balances: []adapter.Balance{{AssetSymbol: "USD", Equity: "10"}}}}}, nil
	}
	h.registry.Register(adapter.PlatformBinance, platformAdapter)
	collectionCycle, _, err := h.cycles.Enqueue(h.ctx, p.ID, cycle.TriggerUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.collection.RunCycle(h.ctx, p.ID, collectionCycle.ID, nil); err == nil {
		t.Fatal("collection committed after deletion request")
	}
	for _, table := range []string{"account_reports", "valuation_points", "price_observations"} {
		var n int
		if err := h.collection.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM "+table+" WHERE portfolio_id=?", p.ID[:]).Scan(&n); err != nil || n != before[table] {
			t.Fatalf("late write to %s: %d %v", table, n, err)
		}
	}
	if _, _, err := h.cycles.Enqueue(h.ctx, p.ID, cycle.TriggerUser); err == nil {
		t.Fatal("deleting portfolio accepted new collection")
	}
}
