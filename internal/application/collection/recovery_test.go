package collection

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// recoverySource supplies fixed successful reports; cutover and failure scenarios
// belong to the integration suite rather than this recovery fixture.
type recoverySource struct{}

func (recoverySource) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "dual-test", MarginMode: adapters.MarginOKXPM}, nil
}

func (recoverySource) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, types []adapters.AccountType) (adapters.Batch, error) {
	now := time.Now().UTC()
	batch := adapters.Batch{ObservedAt: now, MarginMode: adapters.MarginOKXPM}
	for _, kind := range types {
		report := adapters.AccountReport{AccountType: kind, Status: adapters.StatusOK, ObservedAt: now}
		if kind == "earn" {
			report.Balances = []adapters.Balance{{PlatformAssetSymbol: "USDT", Equity: "60"}}
		} else {
			report.Balances = []adapters.Balance{{PlatformAssetSymbol: "USDT", Equity: "40"}}
			expiry := now.Add(24 * time.Hour)
			report.DualPositions = []adapters.DualPosition{
				{PlatformPositionRef: "one", InvestSymbol: "USDT", Principal: "15", Status: "active"},
				{PlatformPositionRef: "two", InvestSymbol: "USDT", Principal: "25", Status: "active", ExpiryAt: &expiry},
			}
		}
		batch.Reports = append(batch.Reports, report)
	}
	return batch, nil
}

func enableRecoveryIdentity(h *harness, name string, platform adapters.Platform, structure adapters.AccountStructure, types ...adapters.AccountType) identity.Identity {
	h.t.Helper()
	item := h.createIdentity(name, platform, structure, types...)
	item = h.storeCredentials(item, "seed-"+name)
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK {
		h.t.Fatalf("enable: %+v %v", outcome, err)
	}
	return enabled
}

func claimRecoveryCycle(h *harness) jobs.ClaimedJob {
	h.t.Helper()
	job, _, err := h.jobs.EnqueueCycle(h.ctx, h.portfolioID, jobs.TriggerManual, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	claimed, ok, err := h.jobs.ClaimOne(h.ctx, "before-restart", time.Minute)
	if err != nil || !ok || claimed.ID != job.ID {
		h.t.Fatalf("claim: %+v %v %v", claimed, ok, err)
	}
	return claimed
}

func reclaimRecoveryCycle(h *harness, jobID uuid.UUID) jobs.ClaimedJob {
	h.t.Helper()
	if _, err := h.collection.db.ExecContext(h.ctx, "UPDATE jobs SET lease_expires_at=? WHERE id=?", time.Now().UTC().Add(-time.Minute), jobID[:]); err != nil {
		h.t.Fatal(err)
	}
	claimed, ok, err := h.jobs.ClaimOne(h.ctx, "after-restart", time.Minute)
	if err != nil || !ok || claimed.ID != jobID || claimed.AttemptCount != 2 {
		h.t.Fatalf("reclaim: %+v %v %v", claimed, ok, err)
	}
	return claimed
}

func TestCycleResumesCommittedIdentityAfterLeaseExpiry(t *testing.T) {
	h := newHarness(t)
	first := &testAdapter{Adapter: h.fakes[adapters.PlatformBinance]}
	second := &testAdapter{Adapter: h.fakes[adapters.PlatformBybit]}
	h.registry.Register(adapters.PlatformBinance, first)
	h.registry.Register(adapters.PlatformBybit, second)
	enableRecoveryIdentity(h, "First", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot", "um_futures", adapters.TypeDualInvestment)
	enableRecoveryIdentity(h, "Second", adapters.PlatformBybit, adapters.StructureBybitUTA2, "funding")
	claimed := claimRecoveryCycle(h)
	h.collection.stagger = time.Second
	h.collection.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected interruption after first Identity: %v", err)
	}
	if _, err := h.valuation.PointByJob(h.ctx, h.portfolioID, claimed.ID); !errors.Is(err, valuation.ErrNotFound) {
		t.Fatalf("interrupted cycle has a point: %v", err)
	}
	rows, err := h.collection.q.ListReportsByJob(h.ctx, sqlcgen.ListReportsByJobParams{PortfolioID: h.portfolioID[:], JobID: claimed.ID[:]})
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
		balances, err := h.collection.q.ListBalancesByReport(h.ctx, reportID)
		if err != nil {
			t.Fatal(err)
		}
		positions, err := h.collection.q.ListPositionsByReport(h.ctx, reportID)
		if err != nil {
			t.Fatal(err)
		}
		dual, err := h.collection.q.ListDualPositionsByReport(h.ctx, reportID)
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
		t.Fatalf("completed Identity was recollected: %d %d", len(first.calls), len(second.calls))
	}
	for _, r := range rows {
		after, err := h.collection.q.GetReport(h.ctx, sqlcgen.GetReportParams{PortfolioID: h.portfolioID[:], ID: r.ID})
		if err != nil || after.Status != r.Status || after.ObservedAt != r.ObservedAt || after.Equity != r.Equity || after.ScopeVersion != r.ScopeVersion || after.CreatedAt != r.CreatedAt {
			t.Fatalf("persisted observation changed: %+v %v", after, err)
		}
		if !reflect.DeepEqual(before[string(r.ID)], readHoldings(r.ID)) {
			t.Fatal("recovery changed persisted holdings")
		}
	}
	if err := h.jobs.Finish(h.ctx, &reclaimed, jobs.Result{OK: true, Detail: summary.String()}); err != nil {
		t.Fatal(err)
	}
}

func TestCycleReusesPointBeforeJobFinish(t *testing.T) {
	h := newHarness(t)
	adapter := &testAdapter{Adapter: h.fakes[adapters.PlatformBinance]}
	h.registry.Register(adapters.PlatformBinance, adapter)
	enableRecoveryIdentity(h, "Completed", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	claimed := claimRecoveryCycle(h)
	before, err := h.collection.RunCycle(h.ctx, h.portfolioID, claimed.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Boundary changes after the point was committed must not reopen its Cycle.
	enableRecoveryIdentity(h, "Later", adapters.PlatformBinance, adapters.StructureBinanceClassic, "spot")
	reclaimed := reclaimRecoveryCycle(h, claimed.ID)
	after, err := h.collection.RunCycle(h.ctx, h.portfolioID, reclaimed.ID)
	if err != nil || !reflect.DeepEqual(before, after) || len(adapter.calls) != 1 {
		t.Fatalf("completed point changed: before=%+v after=%+v calls=%d err=%v", before, after, len(adapter.calls), err)
	}
	if err := h.jobs.Finish(h.ctx, &reclaimed, jobs.Result{OK: true, Detail: after.String()}); err != nil {
		t.Fatal(err)
	}
	job, err := h.jobs.Get(h.ctx, h.portfolioID, claimed.ID)
	if err != nil || job.Status != jobs.StatusSucceeded {
		t.Fatalf("job: %+v %v", job, err)
	}
}

func TestCycleResumesLegacyPartialAccountReports(t *testing.T) {
	for _, status := range []string{adapters.StatusOK, adapters.StatusFailed, ClassSkipped} {
		t.Run(status, func(t *testing.T) {
			h := newHarness(t)
			adapter := &testAdapter{Adapter: recoverySource{}}
			h.registry.Register(adapters.PlatformOKX, adapter)
			item := enableRecoveryIdentity(h, "Legacy", adapters.PlatformOKX, adapters.StructureOKXUnified, "earn", adapters.TypeDualInvestment)
			_, previousJob := h.runCycle()
			previous := h.accountsByType(previousJob, item.ID)
			claimed := claimRecoveryCycle(h)
			accountID := previous["earn"].AccountID
			now := time.Now().UTC().Truncate(time.Microsecond)
			if status == adapters.StatusOK {
				tx, err := h.collection.db.BeginTx(h.ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = tx.Rollback() }()
				report := adapters.AccountReport{AccountType: "earn", Status: adapters.StatusOK, Balances: []adapters.Balance{{PlatformAssetSymbol: "USDT", Equity: "60"}}}
				if err := h.collection.insertSuccessfulReport(h.ctx, h.collection.q.WithTx(tx), h.portfolioID, claimed.ID, adapters.PlatformOKX, accountID, &report, adapters.MarginOKXPM, now, now); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			} else {
				class := adapters.ClassTemporary
				if status == ClassSkipped {
					class = ClassSkipped
				}
				if err := insertFailedReport(h.ctx, h.collection.q, h.portfolioID, claimed.ID, accountID, class, "Original failure", now); err != nil {
					t.Fatal(err)
				}
			}
			original, err := h.collection.q.GetReportForAccountInJob(h.ctx, sqlcgen.GetReportForAccountInJobParams{PortfolioID: h.portfolioID[:], JobID: claimed.ID[:], AccountID: accountID[:]})
			if err != nil {
				t.Fatal(err)
			}
			adapter.calls = nil
			if status == ClassSkipped {
				if _, err := h.identities.Disable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil {
					t.Fatal(err)
				}
			}
			// Model an ordinary infrastructure retry as well as restart recovery.
			if err := h.jobs.Finish(h.ctx, &claimed, jobs.Result{Retry: true, ErrorClass: "temporary", Detail: "Interrupted"}); err != nil {
				t.Fatal(err)
			}
			reclaimed, ok, err := h.jobs.ClaimOne(h.ctx, "retry", time.Minute)
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
			if status == adapters.StatusOK {
				if summary.ReportFailed != 0 || earn.Freshness != valuation.FreshnessFresh || !earn.ReportObservedAt.Equal(now) {
					t.Fatalf("fresh report changed: %+v", earn)
				}
			} else {
				if earn.Freshness != valuation.FreshnessStale || earn.SourceReportID == nil || *earn.SourceReportID != previous["earn"].ReportID || earn.ErrorDetail == nil || *earn.ErrorDetail != "Original failure" {
					t.Fatalf("failure overwritten: %+v", earn)
				}
			}
			if status == ClassSkipped {
				if len(adapter.calls) != 0 || summary.ReportFailed != 2 || summary.IdentityCount != 0 {
					t.Fatalf("skipped recovery: %+v calls=%v", summary, adapter.calls)
				}
			} else if !reflect.DeepEqual(adapter.calls, [][]adapters.AccountType{{adapters.TypeDualInvestment}}) {
				t.Fatalf("recollected a committed Account: %v", adapter.calls)
			}
			if err := h.jobs.Finish(h.ctx, &reclaimed, jobs.Result{OK: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIdentityReportsAndOutcomeRollbackTogether(t *testing.T) {
	for _, result := range []string{"success", "missing_report", "authentication"} {
		t.Run(result, func(t *testing.T) {
			h := newHarness(t)
			adapter := &testAdapter{Adapter: recoverySource{}}
			adapter.collect = func(types []adapters.AccountType) (adapters.Batch, error) {
				if result == "authentication" {
					return adapters.Batch{}, adapters.NewError(adapters.ClassAuthentication, "Invalid key")
				}
				batch := adapters.Batch{ObservedAt: time.Now().UTC(), MarginMode: adapters.MarginOKXMCM}
				for i, kind := range types {
					if result == "missing_report" && i > 0 {
						continue
					}
					batch.Reports = append(batch.Reports, adapters.AccountReport{AccountType: kind, Status: adapters.StatusOK, Balances: []adapters.Balance{{PlatformAssetSymbol: "USDT", Equity: "10"}}})
				}
				return batch, nil
			}
			h.registry.Register(adapters.PlatformOKX, adapter)
			item := enableRecoveryIdentity(h, "Atomic", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading", "funding")
			claimed := claimRecoveryCycle(h)
			trigger := "collection_recovery_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			// #nosec G201 -- The identifier and hex literal contain only test-generated UUIDs; CREATE TRIGGER cannot bind them.
			statement := fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON identities FOR EACH ROW BEGIN IF NEW.id = X'%x' AND NOT (NEW.last_collected_at <=> OLD.last_collected_at) THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Injected collection outcome failure'; END IF; END", trigger, item.ID[:])
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
			if err := h.collection.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM account_reports r JOIN accounts a ON a.id=r.account_id WHERE r.job_id=? AND a.account_type <> 'manual'", claimed.ID[:]).Scan(&count); err != nil || count != 0 {
				t.Fatalf("reports escaped rollback: %d %v", count, err)
			}
			current, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
			if err != nil || current.Connection.Status != identity.ConnectionEnabled || current.MarginMode != item.MarginMode || current.Connection.LastCollectedAt != nil {
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
			if err := h.jobs.Finish(h.ctx, &claimed, jobs.Result{OK: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFinishRejectsLostLease(t *testing.T) {
	for _, result := range []jobs.Result{{OK: true}, {Retry: true, ErrorClass: "temporary"}, {ErrorClass: "persistence"}} {
		h := newHarness(t)
		claimed := claimRecoveryCycle(h)
		reclaimed := reclaimRecoveryCycle(h, claimed.ID)
		if err := h.jobs.Finish(h.ctx, &claimed, result); !errors.Is(err, jobs.ErrLostLease) {
			t.Fatalf("stale owner finish: %v", err)
		}
		job, err := h.jobs.Get(h.ctx, h.portfolioID, claimed.ID)
		if err != nil || job.Status != jobs.StatusRunning || job.AttemptCount != 2 || job.LastDetail != nil {
			t.Fatalf("new owner's job changed: %+v %v", job, err)
		}
		if err := h.jobs.Finish(h.ctx, &reclaimed, jobs.Result{OK: true}); err != nil {
			t.Fatal(err)
		}
	}
}
