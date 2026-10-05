package valuation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/portfolio"
)

type rebuildFixture struct {
	t               *testing.T
	ctx             context.Context
	db              *sql.DB
	service         *Service
	identities      *identity.Service
	user, portfolio uuid.UUID
	at              time.Time
}

func newRebuildFixture(t *testing.T) *rebuildFixture {
	t.Helper()
	db := mysql.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "rebuild-"+uuid.NewString()[:8], "Rebuild=123456")
	if err != nil {
		t.Fatal(err)
	}
	portfolios, err := portfolio.NewService(db).List(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewService(db, identity.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return &rebuildFixture{t: t, ctx: ctx, db: db, service: NewService(db), identities: identities, user: user.ID, portfolio: portfolios[0].ID, at: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
}

func (f *rebuildFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *rebuildFixture) accounts(types ...adapters.AccountType) map[string]uuid.UUID {
	f.t.Helper()
	item, err := f.identities.Create(f.ctx, f.portfolio, &identity.CreateInput{ActorUserID: f.user, Platform: adapters.PlatformBinance, DisplayName: uuid.NewString(), AccountStructure: adapters.StructureBinanceClassic, SelectedAccountTypes: types})
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]uuid.UUID{}
	for _, a := range item.Accounts {
		if a.Selected {
			out[string(a.AccountType)] = a.ID
		}
	}
	return out
}

func (f *rebuildFixture) job(at time.Time, status string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec("INSERT INTO jobs (id,portfolio_id,kind,status,`trigger`,dedupe_key,available_at,created_at,updated_at,finished_at) VALUES (?,?,'collection_cycle',?,'manual','cycle',?,?,?,?)", id[:], f.portfolio[:], status, at, at, at, at.Add(2*time.Second))
	return id
}

func (f *rebuildFixture) report(job, account uuid.UUID, at time.Time, status string, scope int, equity any) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var errorClass any
	if status != "ok" {
		errorClass = "temporary"
	}
	f.exec("INSERT INTO account_reports (id,portfolio_id,job_id,account_id,status,observed_at,error_class,equity,method,scope_version,created_at) VALUES (?,?,?,?,?,?,?,?,'missing',?,?)", id[:], f.portfolio[:], job[:], account[:], status, at, errorClass, equity, scope, at)
	return id
}

func TestRebuildPreservesScopeSourceTimeAndInterruptedReports(t *testing.T) {
	f := newRebuildFixture(t)
	account := f.accounts("spot")["spot"]
	sourceAt := f.at.Add(-2 * time.Minute)
	first := f.report(f.job(sourceAt, "failed"), account, sourceAt, "ok", 2, nil)
	second := f.report(f.job(sourceAt, "cancelled"), account, sourceAt, "ok", 2, nil)
	ids := []uuid.UUID{first, second}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	chosen := ids[1]
	listing := uuid.New()
	symbol := "REBUILD-" + uuid.NewString()[:8]
	f.exec("INSERT INTO asset_listings (id,platform,symbol,asset_symbol,first_seen_at,last_seen_at,created_at,updated_at) VALUES (?,'binance',?,?,?,?,?,?)", listing[:], symbol, symbol, sourceAt, sourceAt, sourceAt, sourceAt)
	for _, id := range ids {
		f.exec("INSERT INTO balances (report_id,portfolio_id,listing_id,equity) VALUES (?,?,?,7)", id[:], f.portfolio[:], listing[:])
	}
	f.exec("INSERT INTO prices (platform,asset_symbol,observed_at,price,quote_symbol) VALUES ('binance',?,?,3,'USD'),('binance',?,?,9,'USD')", symbol, sourceAt, symbol, f.at)
	// A newer incompatible scope and an equal-time source must both be ignored.
	f.report(f.job(f.at.Add(-time.Minute), "failed"), account, f.at.Add(-time.Minute), "ok", 1, "99")
	f.report(f.job(f.at, "failed"), account, f.at, "ok", 2, "999")
	job := f.job(f.at, "succeeded")
	f.report(job, account, f.at, "failed", 2, nil)
	count, err := f.service.Rebuild(f.ctx, f.portfolio)
	if err != nil || count != 1 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	rows, err := f.service.Accounts(f.ctx, f.portfolio, job)
	if err != nil || len(rows) != 1 {
		t.Fatalf("accounts: %v %v", rows, err)
	}
	a := rows[0]
	if a.SourceReportID == nil || *a.SourceReportID != chosen || a.ValueUSD == nil || *a.ValueUSD != "21" || a.ReportObservedAt == nil || !a.ReportObservedAt.Equal(sourceAt) || a.Freshness != FreshnessStale {
		t.Fatalf("incompatible source, wrong observation price or interrupted source omitted: %+v", a)
	}
	// A successful empty report really removes holdings.
	emptyJob := f.job(f.at.Add(time.Minute), "succeeded")
	f.report(emptyJob, account, f.at.Add(time.Minute), "ok", 2, nil)
	if _, err := f.service.Rebuild(f.ctx, f.portfolio); err != nil {
		t.Fatal(err)
	}
	point, err := f.service.PointByJob(f.ctx, f.portfolio, emptyJob)
	if err != nil || point.TotalUSD != "0" || point.IncompleteAccountCount != 0 {
		t.Fatalf("empty success: %+v %v", point, err)
	}
}

func TestRebuildRetainsNonmonotonicEvaluationTimes(t *testing.T) {
	f := newRebuildFixture(t)
	accounts := f.accounts("spot", "funding")
	cycles := []uuid.UUID{}
	at := []time.Time{f.at.Add(20 * time.Minute), f.at.Add(10 * time.Minute), f.at.Add(30 * time.Minute)}
	for i := range 3 {
		created := f.at.Add(time.Duration(i) * time.Minute)
		job := f.job(created, "succeeded")
		f.report(job, accounts["spot"], created, "ok", 2, "1")
		if i == 1 {
			f.report(job, accounts["funding"], created, "ok", 2, "3")
		}
		if _, err := f.service.WritePoint(f.ctx, f.portfolio, job, at[i]); err != nil {
			t.Fatal(err)
		}
		cycles = append(cycles, job)
	}
	for i, job := range cycles {
		if _, err := f.service.WritePoint(f.ctx, f.portfolio, job, at[i]); err != nil {
			t.Fatal(err)
		}
	}
	if count, err := f.service.Rebuild(f.ctx, f.portfolio); err != nil || count != 3 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	for i, job := range cycles {
		point, err := f.service.PointByJob(f.ctx, f.portfolio, job)
		wantWithdrawal := "0"
		if i == 0 {
			wantWithdrawal = "3"
		}
		if err != nil || !point.EvaluatedAt.Equal(at[i]) || point.BoundaryWithdrawalUSD != wantWithdrawal || point.BoundaryContributionUSD != "0" {
			t.Fatalf("point %d used creation order instead of evaluation time: %+v %v", i, point, err)
		}
	}
}

func (f *rebuildFixture) rejectTrigger(table, operation string, id uuid.UUID) func() {
	f.t.Helper()
	name := "rebuild_" + uuid.NewString()[:8]
	column := "id"
	if table == "valuation_points" {
		column = "job_id"
	}
	query := fmt.Sprintf("CREATE TRIGGER %#q BEFORE %s ON %s FOR EACH ROW BEGIN IF NEW.%s = UNHEX('%s') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test rollback'; END IF; END", name, operation, table, column, fmt.Sprintf("%x", id[:]))
	f.exec(query)
	drop := func() { f.exec("DROP TRIGGER IF EXISTS `" + name + "`") }
	f.t.Cleanup(drop)
	return drop
}

func TestRebuildCommitsOnlyCompletePointsAndCanRetry(t *testing.T) {
	f := newRebuildFixture(t)
	account := f.accounts("spot")["spot"]
	jobs := []uuid.UUID{f.job(f.at, "succeeded"), f.job(f.at.Add(time.Minute), "succeeded")}
	for i, job := range jobs {
		f.report(job, account, f.at.Add(time.Duration(i)*time.Minute), "ok", 2, "3")
	}
	drop := f.rejectTrigger("valuation_points", "INSERT", jobs[1])
	count, err := f.service.Rebuild(f.ctx, f.portfolio)
	if err == nil || count != 1 {
		t.Fatalf("partial count=%d error=%v", count, err)
	}
	first, err := f.service.PointByJob(f.ctx, f.portfolio, jobs[0])
	if err != nil || first.TotalUSD != "3" {
		t.Fatalf("committed predecessor lost: %+v %v", first, err)
	}
	if _, err := f.service.PointByJob(f.ctx, f.portfolio, jobs[1]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed point exists: %v", err)
	}
	rows, err := f.service.Accounts(f.ctx, f.portfolio, jobs[1])
	if err != nil || rows[0].ValueUSD != nil || rows[0].Method != MethodMissing {
		t.Fatalf("failed transaction leaked report values: %+v %v", rows, err)
	}
	drop()
	if count, err := f.service.Rebuild(f.ctx, f.portfolio); err != nil || count != 2 {
		t.Fatalf("retry count=%d error=%v", count, err)
	}
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()
	if count, err := f.service.Rebuild(ctx, f.portfolio); !errors.Is(err, context.Canceled) || count != 0 {
		t.Fatalf("canceled rebuild count=%d error=%v", count, err)
	}
}

func TestPointRollsBackEveryReportBatch(t *testing.T) {
	f := newRebuildFixture(t)
	job := f.job(f.at, "succeeded")
	// Six supported Accounts on each of 45 Identities cross the write boundary.
	for range 45 {
		for _, account := range f.accounts("spot", "funding", "um_futures", "cm_futures", "earn", "dual_investment") {
			f.report(job, account, f.at, "ok", 2, "2")
		}
	}
	// Reports are sorted by Identity and Account name. Select the last one so
	// the failure follows at least one successful UPDATE batch.
	accounts, err := f.service.valueJob(f.ctx, f.portfolio, job)
	if err != nil {
		t.Fatal(err)
	}
	last := accounts[len(accounts)-1].ReportID
	drop := f.rejectTrigger("account_reports", "UPDATE", last)
	if _, err := f.service.WritePoint(f.ctx, f.portfolio, job, f.at); err == nil {
		t.Fatal("injected report failure was ignored")
	}
	var valued int
	if err := f.db.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM account_reports WHERE portfolio_id=? AND value_usd IS NOT NULL", f.portfolio[:]).Scan(&valued); err != nil || valued != 0 {
		t.Fatalf("partial batch leaked %d values: %v", valued, err)
	}
	if _, err := f.service.PointByJob(f.ctx, f.portfolio, job); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed aggregate exists: %v", err)
	}
	drop()
	point, err := f.service.WritePoint(f.ctx, f.portfolio, job, f.at)
	if err != nil || point.AccountCount != 270 || point.TotalUSD != "540" {
		t.Fatalf("batch retry: %+v %v", point, err)
	}
}

func TestHistoricalPriceFallbackTiesMatchOrdinaryLookup(t *testing.T) {
	f := newRebuildFixture(t)
	symbol := "PRICE-" + uuid.NewString()[:8]
	for i, platform := range []string{"binance", "okx"} {
		f.exec("INSERT INTO prices (platform,asset_symbol,observed_at,price,quote_symbol) VALUES (?,?,?,?, 'USD')", platform, symbol, f.at, i+10)
	}
	rows, err := f.service.q.ListRebuildPrices(f.ctx, sqlcgen.ListRebuildPricesParams{AssetSymbols: []string{symbol}, FromAt: f.at.Add(-maxPriceAge), ToAt: f.at})
	if err != nil {
		t.Fatal(err)
	}
	bulk := newHistoricalPrices(f.service.q, rows)
	ordinary := &priceBook{s: f.service, cache: map[priceKey]map[string]pricePoint{}}
	want, found, err := ordinary.lookup(f.ctx, "kraken", symbol, f.at)
	if err != nil || !found {
		t.Fatalf("ordinary lookup: %v %v", found, err)
	}
	for range 2 {
		got, found, err := bulk.lookup(f.ctx, "kraken", symbol, f.at)
		if err != nil || !found || got.Cmp(want) != 0 {
			t.Fatalf("fallback tie changed: %v %v %v", got, found, err)
		}
	}
}

func TestRebuildRetainsDuplicateEvaluationTimeError(t *testing.T) {
	f := newRebuildFixture(t)
	account := f.accounts("spot")["spot"]
	for range 2 {
		f.report(f.job(f.at, "succeeded"), account, f.at, "ok", 2, "3")
	}
	count, err := f.service.Rebuild(f.ctx, f.portfolio)
	if count != 1 || !errors.Is(err, ErrNotFound) {
		t.Fatalf("duplicate point time count=%d error=%v", count, err)
	}
}

func TestRebuildCancellationRetainsCommittedPredecessor(t *testing.T) {
	f := newRebuildFixture(t)
	account := f.accounts("spot")["spot"]
	first := f.job(f.at, "succeeded")
	second := f.job(f.at.Add(time.Minute), "succeeded")
	f.report(first, account, f.at, "ok", 2, "3")
	blocked := f.report(second, account, f.at.Add(time.Minute), "ok", 2, "3")
	lock, err := f.db.BeginTx(f.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback() }()
	if _, err := lock.ExecContext(f.ctx, "UPDATE account_reports SET equity=equity WHERE id=?", blocked[:]); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	type result struct {
		count int
		err   error
	}
	done := make(chan result, 1)
	go func() { count, err := f.service.Rebuild(ctx, f.portfolio); done <- result{count, err} }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, err := f.service.PointByJob(f.ctx, f.portfolio, first)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("first point did not commit while the second report was locked")
		}
	}
	cancel()
	select {
	case result := <-done:
		if result.count != 1 || !errors.Is(result.err, context.Canceled) {
			t.Fatalf("canceled rebuild: count=%d error=%v", result.count, result.err)
		}
	case <-deadline.C:
		t.Fatal("blocked rebuild did not honor cancellation")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.PointByJob(f.ctx, f.portfolio, second); !errors.Is(err, ErrNotFound) {
		t.Fatalf("canceled point exists: %v", err)
	}
	rows, err := f.service.Accounts(f.ctx, f.portfolio, second)
	if err != nil || rows[0].ValueUSD != nil {
		t.Fatalf("canceled report values leaked: %+v %v", rows, err)
	}
	if count, err := f.service.Rebuild(f.ctx, f.portfolio); err != nil || count != 2 {
		t.Fatalf("retry count=%d error=%v", count, err)
	}
}
