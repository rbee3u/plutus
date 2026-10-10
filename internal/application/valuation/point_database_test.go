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

	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/testsupport/mysqltest"
)

type pointFixture struct {
	t               *testing.T
	ctx             context.Context
	db              *sql.DB
	service         *Service
	identities      *identity.Service
	user, portfolio uuid.UUID
	at              time.Time
}

func newPointFixture(t *testing.T) *pointFixture {
	t.Helper()
	db := mysqltest.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	user, err := auth.NewService(db, time.Hour, 24*time.Hour).CreateUser(ctx, "retry-"+uuid.NewString()[:8], "Retry=123456")
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
	return &pointFixture{t: t, ctx: ctx, db: db, service: NewService(db), identities: identities, user: user.ID, portfolio: portfolios[0].ID, at: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
}

func (f *pointFixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatal(err)
	}
}

func (f *pointFixture) accounts(types ...adapters.AccountType) map[string]uuid.UUID {
	f.t.Helper()
	item, err := f.identities.Create(f.ctx, f.portfolio, &identity.CreateInput{ActorUserID: f.user, Platform: adapters.PlatformBinance, DisplayName: uuid.NewString(), AccountStructure: adapters.StructureBinanceClassic, SelectedAccountTypes: types})
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]uuid.UUID{}
	for _, a := range item.Accounts {
		if a.Selected && a.AccountType != adapters.TypeManual {
			out[string(a.AccountType)] = a.ID
		}
	}
	return out
}

func (f *pointFixture) cycle(at time.Time, status string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec("INSERT INTO collection_cycles (portfolio_id,id,status,trigger_kind,available_at,created_at,collection_finished_at,finished_at) VALUES (?,?,?,'user',?,?,?,?)", f.portfolio[:], id[:], status, at, at, at, at.Add(2*time.Second))
	return id
}

func (f *pointFixture) report(cycle, account uuid.UUID, at time.Time, status string, _ int, equity any) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	var errorClass any
	var collectedAt any = at
	if status != "succeeded" {
		errorClass = "temporary"
		collectedAt = nil
	}
	f.exec("INSERT INTO account_reports (id,portfolio_id,cycle_id,account_id,status,collected_at,error_class,reported_equity_usd,recorded_at) VALUES (?,?,?,?,?,?,?,?,?)", id[:], f.portfolio[:], cycle[:], account[:], status, collectedAt, errorClass, equity, at)
	return id
}

func (f *pointFixture) manualReport(cycle, account uuid.UUID, at time.Time, revision int) uuid.UUID {
	f.t.Helper()
	id := f.report(cycle, account, at, "succeeded", 2, nil)
	f.exec("UPDATE account_reports SET manual_revision_number=? WHERE id=?", revision, id[:])
	return id
}

func TestPointPreservesSourceTimeAndInterruptedReports(t *testing.T) {
	f := newPointFixture(t)
	account := f.accounts("spot")["spot"]
	sourceAt := f.at.Add(-2 * time.Minute)
	first := f.report(f.cycle(sourceAt, "failed"), account, sourceAt, "succeeded", 2, nil)
	second := f.report(f.cycle(sourceAt, "failed"), account, sourceAt, "succeeded", 2, nil)
	ids := []uuid.UUID{first, second}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	chosen := ids[1]
	asset := uuid.New()
	symbol := "RETRY-" + uuid.NewString()[:8]
	f.exec("INSERT INTO assets (id,portfolio_id,platform,symbol,canonical_asset_symbol,first_observed_at,last_observed_at,mapping_version) VALUES (?,?,'binance',?,?,?,?,1)", asset[:], f.portfolio[:], symbol, symbol, sourceAt, sourceAt)
	for _, id := range ids {
		f.exec("INSERT INTO balances (report_id,portfolio_id,asset_id,equity) VALUES (?,?,?,7)", id[:], f.portfolio[:], asset[:])
	}
	f.exec("INSERT INTO price_observations (id,portfolio_id,source_market_ref,platform,requested_canonical_asset_symbol,retrieved_at,price,quote_canonical_asset_symbol) VALUES (UUID_TO_BIN(UUID()),?,'fixture','binance',?,?,3,'USD'),(UUID_TO_BIN(UUID()),?,'fixture','binance',?,?,9,'USD')", f.portfolio[:], symbol, sourceAt, f.portfolio[:], symbol, f.at)
	// A newer incompatible scope and an equal-time source must both be ignored.
	f.report(f.cycle(f.at.Add(-time.Minute), "failed"), account, f.at.Add(-time.Minute), "failed", 2, nil)
	f.report(f.cycle(f.at, "failed"), account, f.at, "succeeded", 2, "999")
	cycleID := f.cycle(f.at, "succeeded")
	f.report(cycleID, account, f.at, "failed", 2, nil)
	f.point(cycleID, f.at)
	rows, err := f.service.Accounts(f.ctx, f.portfolio, cycleID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("accounts: %v %v", rows, err)
	}
	a := rows[0]
	if a.SourceReportID == nil || *a.SourceReportID != chosen || a.ValueUSD == nil || *a.ValueUSD != "21" || a.ReportObservedAt == nil || !a.ReportObservedAt.Equal(sourceAt) || a.Freshness != FreshnessStale {
		t.Fatalf("incompatible source, wrong observation price or interrupted source omitted: %+v", a)
	}
	// A successful empty report really removes holdings.
	emptyCycle := f.cycle(f.at.Add(time.Minute), "succeeded")
	f.report(emptyCycle, account, f.at.Add(time.Minute), "succeeded", 2, nil)
	f.point(emptyCycle, f.at.Add(time.Minute))
	point, err := f.service.PointByCycle(f.ctx, f.portfolio, emptyCycle)
	if err != nil || point.TotalUSD != "0" || point.IncompleteAccountCount != 0 {
		t.Fatalf("empty success: %+v %v", point, err)
	}
}

func (f *pointFixture) rejectTrigger(table, operation string, id uuid.UUID) func() {
	f.t.Helper()
	name := "retry_" + uuid.NewString()[:8]
	column := "account_id"
	if table == "valuation_points" {
		column = "cycle_id"
	}
	query := fmt.Sprintf("CREATE TRIGGER %#q BEFORE %s ON %s FOR EACH ROW BEGIN IF NEW.%s = UNHEX('%s') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='test rollback'; END IF; END", name, operation, table, column, fmt.Sprintf("%x", id[:]))
	f.exec(query)
	drop := func() { f.exec("DROP TRIGGER IF EXISTS `" + name + "`") }
	f.t.Cleanup(drop)
	return drop
}

func TestPointRollsBackEveryReportBatch(t *testing.T) {
	f := newPointFixture(t)
	cycleID := f.cycle(f.at, "succeeded")
	// Six supported Accounts on each of 45 Identities cross the write boundary.
	for range 45 {
		for _, account := range f.accounts("spot", "funding", "um_futures", "cm_futures", "earn", "dual_investment") {
			f.report(cycleID, account, f.at, "succeeded", 2, "2")
		}
	}
	// Reports are sorted by Identity and Account name. Select the last one so
	// the failure follows at least one successful UPDATE batch.
	accounts, err := f.service.valueCycle(f.ctx, f.portfolio, cycleID)
	if err != nil {
		t.Fatal(err)
	}
	last := accounts[len(accounts)-1].AccountID
	drop := f.rejectTrigger("account_valuations", "INSERT", last)
	if _, err := f.service.WritePoint(f.ctx, f.portfolio, cycleID, f.at); err == nil {
		t.Fatal("injected report failure was ignored")
	}
	var valued int
	if err := f.db.QueryRowContext(f.ctx, "SELECT COUNT(*) FROM account_valuations WHERE portfolio_id=? AND value_usd IS NOT NULL", f.portfolio[:]).Scan(&valued); err != nil || valued != 0 {
		t.Fatalf("partial batch leaked %d values: %v", valued, err)
	}
	if _, err := f.service.PointByCycle(f.ctx, f.portfolio, cycleID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("failed aggregate exists: %v", err)
	}
	drop()
	point, err := f.service.WritePoint(f.ctx, f.portfolio, cycleID, f.at)
	if err != nil || point.AccountCount != 270 || point.TotalUSD != "540" {
		t.Fatalf("batch retry: %+v %v", point, err)
	}
}

func TestHistoricalPriceMatchesOrdinaryOwnPlatformLookup(t *testing.T) {
	f := newPointFixture(t)
	symbol := "PRICE-" + uuid.NewString()[:8]
	for i, platform := range []string{"binance", "okx"} {
		f.exec("INSERT INTO price_observations (id,portfolio_id,source_market_ref,platform,requested_canonical_asset_symbol,retrieved_at,price,quote_canonical_asset_symbol) VALUES (UUID_TO_BIN(UUID()),?,'fixture',?,?,?,?, 'USD')", f.portfolio[:], platform, symbol, f.at, i+10)
	}
	ordinary := NewPriceBook(sqlcgen.New(f.db), f.portfolio)
	want, found, err := ordinary.Lookup(f.ctx, "binance", symbol, f.at)
	if err != nil || !found {
		t.Fatalf("ordinary lookup: %v %v", found, err)
	}
	for range 2 {
		got, found, err := ordinary.Lookup(f.ctx, "binance", symbol, f.at)
		if err != nil || !found || got.Cmp(want) != 0 {
			t.Fatalf("fallback tie changed: %v %v %v", got, found, err)
		}
	}
}

func TestPointRetainsEqualEvaluationTimes(t *testing.T) {
	f := newPointFixture(t)
	account := f.accounts("spot")["spot"]
	cycles := []uuid.UUID{f.cycle(f.at, "succeeded"), f.cycle(f.at, "succeeded")}
	slices.SortFunc(cycles, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	for _, cycle := range cycles {
		f.report(cycle, account, f.at, "succeeded", 2, "3")
		f.point(cycle, f.at)
	}
	older := f.cycle(f.at.Add(-time.Minute), "succeeded")
	if _, err := f.service.WritePoint(f.ctx, f.portfolio, older, f.at.Add(-time.Minute)); err == nil {
		t.Fatal("out-of-order insertion invalidated committed comparison evidence")
	}
}

func TestCommittedPointIgnoresLaterMappingAndRetryTime(t *testing.T) {
	f := newPointFixture(t)
	account := f.accounts("spot")["spot"]
	cycle := f.cycle(f.at, "succeeded")
	report := f.report(cycle, account, f.at, "succeeded", 2, nil)
	asset := uuid.New()
	f.exec("INSERT INTO assets(id,portfolio_id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", asset[:], f.portfolio[:], f.at, f.at)
	f.exec("INSERT INTO balances(report_id,portfolio_id,asset_id,equity) VALUES(?,?,?,12)", report[:], f.portfolio[:], asset[:])
	first := f.point(cycle, f.at)
	f.exec("UPDATE assets SET canonical_asset_symbol='UNPRICED', mapping_version=mapping_version+1 WHERE id=?", asset[:])
	retried, err := f.service.WritePoint(f.ctx, f.portfolio, cycle, f.at.Add(time.Hour))
	if err != nil || retried.TotalUSD != "12" || retried.Revision() != first.Revision() || !retried.EvaluatedAt.Equal(first.EvaluatedAt) {
		t.Fatalf("retry changed immutable point: %+v %v", retried, err)
	}
	accounts, err := f.service.Accounts(f.ctx, f.portfolio, cycle)
	if err != nil || len(accounts) != 1 || accounts[0].BalanceValuations[0].CanonicalAssetSymbol != "USD" {
		t.Fatalf("retry changed evidence: %+v %v", accounts, err)
	}
	next := f.cycle(f.at.Add(time.Minute), "succeeded")
	f.report(next, account, f.at.Add(time.Minute), "failed", 2, nil)
	changed := f.point(next, f.at.Add(time.Minute))
	if changed.TotalUSD != "0" || changed.IncompleteAccountCount != 1 {
		t.Fatalf("new stale point did not adopt current mapping: %+v", changed)
	}
	found := false
	for _, issue := range changed.Issues {
		if issue.Code == "asset_mapping_changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("adopted mapping change has no durable marker")
	}
}

func (f *pointFixture) point(cycle uuid.UUID, at time.Time) Point {
	f.t.Helper()
	point, err := f.service.WritePoint(f.ctx, f.portfolio, cycle, at)
	if err != nil {
		f.t.Fatal(err)
	}
	return point
}

func TestManualAdjustmentsCompareReportedRevisions(t *testing.T) {
	f := newPointFixture(t)
	spot := f.accounts("spot")["spot"]
	var rawManual []byte
	if err := f.db.QueryRowContext(f.ctx, "SELECT m.id FROM accounts m JOIN accounts s ON s.identity_id=m.identity_id WHERE s.id=? AND m.account_type='manual'", spot[:]).Scan(&rawManual); err != nil {
		t.Fatal(err)
	}
	manualID := uuid.Must(uuid.FromBytes(rawManual))
	asset := uuid.New()
	f.exec("INSERT INTO assets(id,portfolio_id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", asset[:], f.portfolio[:], f.at, f.at)
	for revision, quantity := range map[int]string{2: "10", 3: "11", 4: "10.00", 5: ""} {
		f.exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES(?,?,?,?)", f.portfolio[:], manualID[:], revision, f.at)
		if quantity != "" {
			f.exec("INSERT INTO manual_revision_balances(portfolio_id,account_id,revision_number,asset_id,quantity,changed_at) VALUES(?,?,?,?,?,?)", f.portfolio[:], manualID[:], revision, asset[:], quantity, f.at)
		}
	}
	baseline := f.cycle(f.at, "succeeded")
	f.report(baseline, spot, f.at, "succeeded", 2, "100")
	f.point(baseline, f.at)
	// Equal evaluation times still have distinct predecessors. The baseline
	// deliberately stays outside the batch read below.
	at := f.at.Add(time.Minute)
	cycles := []uuid.UUID{f.cycle(at, "succeeded"), f.cycle(at, "succeeded")}
	slices.SortFunc(cycles, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	cycles = append(cycles, f.cycle(at.Add(time.Minute), "succeeded"))
	for i, revision := range []int{2, 4, 5} {
		cycle := cycles[i]
		recorded := at.Add(time.Duration(i) * time.Second)
		report := f.manualReport(cycle, manualID, recorded, revision)
		if revision != 5 {
			f.exec("INSERT INTO balances(portfolio_id,report_id,asset_id,equity) VALUES(?,?,?,10)", f.portfolio[:], report[:], asset[:])
		}
		evaluated := at
		if i == 2 {
			evaluated = at.Add(time.Minute)
		}
		point := f.point(cycle, evaluated)
		want := i != 1
		if (len(point.ManualAdjustments) == 1) != want {
			t.Fatalf("cycle %d adjustment=%+v, want %v", i, point.ManualAdjustments, want)
		}
	}
	// Neither current mappings nor a future editable revision can change the
	// already adopted quantities or omissions.
	f.exec("UPDATE assets SET canonical_asset_symbol='UNPRICED', mapping_version=mapping_version+1 WHERE id=?", asset[:])
	rows, err := sqlcgen.New(f.db).ListValuationPointsBetween(f.ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: f.portfolio[:], FromAt: at, ToAt: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	points, err := f.service.PointsWithEvents(f.ctx, rows)
	if err != nil || len(points) != 3 {
		t.Fatalf("batch details: %+v %v", points, err)
	}
	for i, point := range points {
		if (len(point.ManualAdjustments) == 1) != (i != 1) || len(point.Issues) != map[bool]int{true: 1, false: 0}[i != 1] {
			t.Fatalf("light batch %d changed Manual decisions or loaded issues: %+v", i, point)
		}
	}
	first, err := f.service.Get(f.ctx, f.portfolio, cycles[0])
	if err != nil || len(first.Issues) != 1 || first.Issues[0].Code != "manual_balance_changed" || first.TotalUSD != "10" {
		t.Fatalf("derived issues used mutable mappings: %+v %v", first, err)
	}
}

func TestManualAdjustmentStartsAtIdentityEntryAndUsesAdoptedSource(t *testing.T) {
	f := newPointFixture(t)
	spot := f.accounts("spot")["spot"]
	var raw []byte
	if err := f.db.QueryRowContext(f.ctx, "SELECT m.id FROM accounts m JOIN accounts s ON s.identity_id=m.identity_id WHERE s.id=? AND m.account_type='manual'", spot[:]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	account := uuid.Must(uuid.FromBytes(raw))
	asset := uuid.New()
	f.exec("INSERT INTO assets(id,portfolio_id,platform,symbol,canonical_asset_symbol,mapping_version,first_observed_at,last_observed_at) VALUES(?,?,'binance','USD','USD',1,?,?)", asset[:], f.portfolio[:], f.at, f.at)
	for revision, quantity := range map[int]int{2: 10, 3: 20} {
		f.exec("INSERT INTO manual_revisions(portfolio_id,account_id,revision_number,changed_at) VALUES(?,?,?,?)", f.portfolio[:], account[:], revision, f.at)
		f.exec("INSERT INTO manual_revision_balances(portfolio_id,account_id,revision_number,asset_id,quantity,changed_at) VALUES(?,?,?,?,?,?)", f.portfolio[:], account[:], revision, asset[:], quantity, f.at)
	}
	// An existing Portfolio point with no representation of this Identity
	// establishes an actual boundary entry, not a Manual adjustment.
	empty := f.cycle(f.at, "succeeded")
	f.point(empty, f.at)
	first := f.cycle(f.at.Add(time.Minute), "succeeded")
	report := f.manualReport(first, account, f.at.Add(time.Minute), 2)
	f.exec("INSERT INTO balances(portfolio_id,report_id,asset_id,equity) VALUES(?,?,?,10)", f.portfolio[:], report[:], asset[:])
	point := f.point(first, f.at.Add(time.Minute))
	if len(point.ManualAdjustments) != 0 || point.BoundaryContributionUSD != "10" {
		t.Fatalf("Identity entry became a Manual adjustment: %+v", point)
	}
	// Collection does not currently emit failed Manual Reports. This fixture
	// still proves that reads use the adopted source rather than the current
	// revision.
	failed := f.cycle(f.at.Add(2*time.Minute), "succeeded")
	f.report(failed, account, f.at.Add(2*time.Minute), "failed", 2, nil)
	point = f.point(failed, f.at.Add(2*time.Minute))
	if len(point.ManualAdjustments) != 0 || point.TotalUSD != "10" || len(point.Issues) != 1 || point.Issues[0].Code != "stale" {
		t.Fatalf("unadopted revision changed the point: %+v", point)
	}
}

func TestManualAdjustmentsCrossReadBatchesWithBoundedDatabaseWork(t *testing.T) {
	f := newPointFixture(t)
	spot := f.accounts("spot")["spot"]
	var raw []byte
	if err := f.db.QueryRowContext(f.ctx, "SELECT m.id FROM accounts m JOIN accounts s ON s.identity_id=m.identity_id WHERE s.id=? AND m.account_type='manual'", spot[:]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	account := uuid.Must(uuid.FromBytes(raw))
	const count = valuationReadBatch + 2
	for i := 0; i <= count; i++ {
		at := f.at.Add(time.Duration(i) * time.Minute)
		cycle := f.cycle(at, "succeeded")
		quantity := "10"
		if i > valuationReadBatch {
			quantity = "-20"
		}
		report := f.report(cycle, account, at, "succeeded", 2, nil)
		f.exec("INSERT INTO valuation_points(portfolio_id,cycle_id,evaluated_at,total_usd,account_count,stale_account_count,incomplete_account_count,calculation_version) VALUES(?,?,?,?,1,0,0,1)", f.portfolio[:], cycle[:], at, quantity)
		f.exec("INSERT INTO account_valuations(portfolio_id,cycle_id,account_id,source_report_id,valuation_method,value_usd) VALUES(?,?,?,?,'calculated',?)", f.portfolio[:], cycle[:], account[:], report[:], quantity)
		if i == valuationReadBatch+1 {
			f.exec("INSERT INTO manual_adjustments(portfolio_id,cycle_id,account_id) VALUES(?,?,?)", f.portfolio[:], cycle[:], account[:])
		}
	}
	rows, err := sqlcgen.New(f.db).ListValuationPointsBetween(f.ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: f.portfolio[:], FromAt: f.at.Add(time.Minute), ToAt: f.at.Add(count * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.BeginTx(f.ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	reads := func() uint64 {
		t.Helper()
		stats, err := tx.QueryContext(f.ctx, "SHOW SESSION STATUS WHERE Variable_name IN ('Handler_read_key','Handler_read_next','Handler_read_prev','Handler_read_rnd_next')")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = stats.Close() }()
		var total uint64
		for stats.Next() {
			var name string
			var value uint64
			if err := stats.Scan(&name, &value); err != nil {
				t.Fatal(err)
			}
			total += value
		}
		if err := stats.Err(); err != nil {
			t.Fatal(err)
		}
		return total
	}
	before := reads()
	points, err := f.service.PointsWithEvents(mysql.TransactionContext(f.ctx, f.db, tx), rows)
	if err != nil || len(points) != count {
		t.Fatalf("batch details: count=%d error=%v", len(points), err)
	}
	for i, point := range points {
		want := i == valuationReadBatch
		if (len(point.ManualAdjustments) == 1) != want {
			t.Fatalf("point %d adjustments=%+v, want changed=%v", i, point.ManualAdjustments, want)
		}
	}
	// Count storage-engine row reads instead of elapsed time. The generous
	// bound permits different index plans but rejects repeated history scans
	// and the former nested semijoin explosion for this small fixture.
	if work := reads() - before; work > 100*count {
		t.Fatalf("%d points required %d database row reads", count, work)
	}
}
