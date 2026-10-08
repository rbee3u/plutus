package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/jobs"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

type manualTestAdapter struct {
	usd       string
	btc       string
	failed    bool
	positions bool
	onPrices  func()
}

func (a *manualTestAdapter) Validate(context.Context, adapters.Credentials, adapters.AccountStructure) (adapters.ValidationResult, error) {
	return adapters.ValidationResult{ExternalRef: "manual-test", MarginMode: adapters.MarginOKXPM}, nil
}

func (a *manualTestAdapter) Collect(_ context.Context, _ adapters.Credentials, _ adapters.AccountStructure, types []adapters.AccountType) (adapters.Batch, error) {
	at := time.Now().UTC()
	if a.failed {
		return adapters.Batch{}, adapters.NewError(adapters.ClassTemporary, "Synthetic private failure.")
	}
	batch := adapters.Batch{ObservedAt: at, MarginMode: adapters.MarginOKXPM}
	for _, typ := range types {
		if typ == adapters.TypeManual {
			panic("Manual must not reach a private Adapter")
		}
		report := adapters.AccountReport{AccountType: typ, Status: adapters.StatusOK, ObservedAt: at}
		if a.usd != "0" {
			report.Balances = []adapters.Balance{{Symbol: "USD", Equity: a.usd}}
		}
		if a.positions {
			report.Positions = []adapters.Position{{PlatformPositionRef: "manual-test-position", Instrument: adapters.Instrument{Symbol: "BTC-USD", Kind: "linear_perpetual", BaseSymbol: "BTC", QuoteSymbol: "USD", SettlementSymbol: "USD"}, Side: "long", Quantity: "1", QuantityUnit: "base"}}
		}
		batch.Reports = append(batch.Reports, report)
	}
	return batch, nil
}

func (a *manualTestAdapter) CollectPrices(_ context.Context, symbols []string) ([]adapters.Price, error) {
	if a.onPrices != nil {
		a.onPrices()
	}
	return adapters.SymbolPrices(symbols, []string{"USD"}, func(symbol, _ string) (string, bool) { return a.btc, symbol == "BTC" && a.btc != "" }, time.Now().UTC()), nil
}

func manualEnabled(t *testing.T, h *harness, a *manualTestAdapter) identity.Identity {
	t.Helper()
	h.registry.Register(adapters.PlatformOKX, a)
	item := h.createIdentity("Manual tests", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	item = h.storeCredentials(item, "manual-key")
	enabled, _, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil {
		t.Fatal(err)
	}
	return enabled
}

func manualSave(t *testing.T, h *harness, item identity.Identity, symbol, quantity string) manual.State {
	t.Helper()
	s := h.manual
	current, err := s.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := manual.Input{Symbol: symbol, Quantity: quantity, Version: current.Version, ActorUserID: h.userID}
	exists := false
	for _, balance := range current.Items {
		if balance.Symbol == symbol {
			exists = true
		}
	}
	var next manual.State
	if exists {
		next, err = s.Update(h.ctx, h.portfolioID, item.ID, in)
	} else {
		next, err = s.Create(h.ctx, h.portfolioID, item.ID, in)
	}
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func hasManualAdjustment(point valuation.Point) bool {
	for _, issue := range point.Issues {
		if issue.Code == "manual_balance_changed" {
			return true
		}
	}
	return false
}

func TestManualCRUDVersionsAndFrozenInputs(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("No credentials", adapters.PlatformOKX, adapters.StructureOKXUnified, "trading")
	s := h.manual
	initial, err := s.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || initial.Pending || len(initial.Items) != 0 {
		t.Fatalf("initial %+v %v", initial, err)
	}
	next, err := s.Create(h.ctx, h.portfolioID, item.ID, manual.Input{Symbol: " usd ", Quantity: "100.000", Version: initial.Version, ActorUserID: h.userID})
	if err != nil || !next.Pending || next.Items[0].Quantity != "100" {
		t.Fatalf("create %+v %v", next, err)
	}
	in := manual.Input{Symbol: "USD", Quantity: "5", Version: next.Version, ActorUserID: h.userID}
	if _, err := s.Create(h.ctx, h.portfolioID, item.ID, in); !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("duplicate %v", err)
	}
	in.Version = initial.Version
	if _, err := s.Update(h.ctx, h.portfolioID, item.ID, in); !errors.Is(err, identity.ErrPrecondition) {
		t.Fatalf("conflict %v", err)
	}
	if _, err := s.Get(h.ctx, uuid.New(), item.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("ownership %v", err)
	}
	in.Version = next.Version
	in.Quantity = "100.0"
	noOp, err := s.Update(h.ctx, h.portfolioID, item.ID, in)
	if err != nil || noOp.Version != next.Version {
		t.Fatalf("no-op %+v %v", noOp, err)
	}
	for _, qty := range []string{"1e3", "0.0000000000000000000000000000001", "999999999999999999999999999999999999"} {
		in.Quantity = qty
		if _, err := s.Update(h.ctx, h.portfolioID, item.ID, in); err == nil {
			t.Fatalf("accepted %s", qty)
		}
	}
	job, _, err := h.jobs.EnqueueCycle(h.ctx, h.portfolioID, jobs.TriggerManual, nil)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := s.Freeze(h.ctx, h.portfolioID, job.ID)
	if err != nil || len(frozen) != 1 || frozen[0].Items[0].Quantity != "100" {
		t.Fatalf("freeze %v %v", frozen, err)
	}
	t.Cleanup(func() {
		_, _ = h.db.ExecContext(context.Background(), "UPDATE jobs SET status='cancelled', finished_at=UTC_TIMESTAMP(6) WHERE id=?", job.ID[:])
	})
	changed := manualSave(t, h, item, "USD", "-100")
	again, err := s.Freeze(h.ctx, h.portfolioID, job.ID)
	if err != nil || again[0].Version != frozen[0].Version || again[0].Items[0].Quantity != "100" {
		t.Fatalf("retry %v %v", again, err)
	}
	summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, job.ID)
	if err != nil || summary.Point.TotalUSD != "100" || summary.IdentityCount != 0 || summary.ReportFailed != 1 {
		t.Fatalf("unconfigured %+v %v", summary, err)
	}
	applied, err := s.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || !applied.Pending || applied.AppliedItems[0].Quantity != "100" || applied.Items[0].Quantity != "-100" {
		t.Fatalf("pending %+v %v", applied, err)
	}
	in.Version = changed.Version
	in.Quantity = "0"
	cleared, err := s.Update(h.ctx, h.portfolioID, item.ID, in)
	if err != nil || len(cleared.Items) != 0 || !cleared.Pending {
		t.Fatalf("zero %+v %v", cleared, err)
	}
	if _, err := s.Delete(h.ctx, h.portfolioID, item.ID, manual.Input{Symbol: "USD", Version: cleared.Version}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("delete missing %v", err)
	}
	var revisions, audits int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM manual_revisions WHERE account_id=?", initial.AccountID[:]).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM audit_events WHERE portfolio_id=? AND action LIKE 'manual.balance.%'", h.portfolioID[:]).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if revisions != 3 || audits != 3 {
		t.Fatalf("revisions=%d audits=%d", revisions, audits)
	}
}

func TestManualConservationHistoryAndSettlement(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "130"}
	item := manualEnabled(t, h, a)
	first, firstJob := h.runCycle()
	if first.Point.TotalUSD != "130" {
		t.Fatal(first)
	}
	a.usd = "30"
	manualSave(t, h, item, "USD", "100")
	second, secondJob := h.runCycle()
	if second.Point.TotalUSD != "130" || second.Point.BoundaryContributionUSD != "0" || !hasManualAdjustment(second.Point) {
		t.Fatalf("conservation %+v", second)
	}
	current, err := h.manual.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || current.Pending {
		t.Fatalf("applied %+v %v", current, err)
	}
	accounts := h.accountsByType(secondJob, item.ID)
	account := accounts["manual"]
	if account.Method != valuation.MethodCalculated || account.Equity != nil || account.ManualModifiedAt == nil || account.ReportObservedAt == nil {
		t.Fatalf("manual provenance %+v", account)
	}
	change, err := h.financial.CustomValueChange(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, first.Point.EvaluatedAt, second.Point.EvaluatedAt)
	if err != nil || change.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("adjustment %+v %v", change, err)
	}
	manualSave(t, h, item, "USD", "101")
	manualSave(t, h, item, "USD", "100")
	third, _ := h.runCycle()
	if hasManualAdjustment(third.Point) {
		t.Fatal("edit then undo marked adjustment")
	}
	a.failed = true
	failed, failedJob := h.runCycle()
	if failed.IdentityFailed != 1 || failed.ReportFailed != 1 {
		t.Fatalf("masked private failure %+v", failed)
	}
	accounts = h.accountsByType(failedJob, item.ID)
	if accounts["manual"].Freshness != valuation.FreshnessFresh || accounts["trading"].Freshness != valuation.FreshnessStale {
		t.Fatal(accounts)
	}
	a.failed = false
	a.usd = "130"
	manualSave(t, h, item, "USD", "0")
	settled, _ := h.runCycle()
	if settled.Point.TotalUSD != "130" {
		t.Fatalf("settled %+v", settled)
	}
	old, err := h.valuation.PointByJob(h.ctx, h.portfolioID, firstJob)
	if err != nil || old.TotalUSD != "130" {
		t.Fatalf("history changed %+v %v", old, err)
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := h.valuation.PointByJob(h.ctx, h.portfolioID, secondJob)
	if err != nil || rebuilt.TotalUSD != "130" || !hasManualAdjustment(rebuilt) || rebuilt.BoundaryContributionUSD != "0" {
		t.Fatalf("rebuilt %+v %v", rebuilt, err)
	}
}

func TestManualDebtPricesMissingValuesAndTree(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "100", btc: "20000"}
	item := manualEnabled(t, h, a)
	manualSave(t, h, item, "USD", "-100")
	point, _ := h.runCycle()
	if point.Point.TotalUSD != "0" {
		t.Fatalf("debt %+v", point)
	}
	tree, err := h.financial.Tree(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 2 {
		t.Fatalf("net zero holdings %v %v", tree, err)
	}
	manualSave(t, h, item, "USD", "0")
	manualSave(t, h, item, "BTC", "2")
	priced, job := h.runCycle()
	if priced.Point.TotalUSD != "40100" {
		t.Fatalf("manual-only price %+v", priced)
	}
	before := h.accountsByType(job, item.ID)["manual"].ManualModifiedAt
	a.btc = "25000"
	moved, job := h.runCycle()
	if moved.Point.TotalUSD != "50100" || hasManualAdjustment(moved.Point) || !h.accountsByType(job, item.ID)["manual"].ManualModifiedAt.Equal(*before) {
		t.Fatalf("price-only %+v", moved)
	}
	manualSave(t, h, item, "UNKNOWN", "-1")
	missing, job := h.runCycle()
	unknown := h.accountsByType(job, item.ID)["manual"]
	if missing.Point.Completeness() != valuation.CompletenessIncomplete || len(unknown.Omitted) != 1 || unknown.Omitted[0].Equity != "-1" {
		t.Fatalf("negative missing %+v %+v", missing, unknown)
	}
	manualSave(t, h, item, "UNKNOWN", "0")
	manualSave(t, h, item, "BTC", "0")
	a.usd = "0"
	empty, _ := h.runCycle()
	tree, err = h.financial.Tree(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || empty.Point.AccountCount != 2 || len(tree.Platforms[0].Identities[0].Accounts) != 0 {
		t.Fatalf("empty %v %+v %v", tree, empty, err)
	}
	a.positions = true
	h.runCycle()
	tree, err = h.financial.Tree(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 1 {
		t.Fatalf("position retained %v %v", tree, err)
	}
}

func TestManualNewIdentityArchiveAndDuringCollectionEdits(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "0", btc: "20000"}
	first := manualEnabled(t, h, a)
	h.runCycle()
	item := h.createIdentity("Manual only", adapters.PlatformOKX, adapters.StructureOKXUnified)
	manualSave(t, h, item, "USD", "-100")
	entered, _ := h.runCycle()
	if entered.Point.TotalUSD != "-100" || entered.Point.BoundaryContributionUSD != "-100" || hasManualAdjustment(entered.Point) {
		t.Fatalf("signed entry %+v", entered)
	}
	state := manualSave(t, h, first, "BTC", "2")
	a.onPrices = func() { manualSave(t, h, first, "BTC", "3") }
	during, job := h.runCycle()
	a.onPrices = nil
	account := h.accountsByType(job, first.ID)["manual"]
	if account.ValueUSD == nil || *account.ValueUSD != "40000" {
		t.Fatalf("frozen during collection %+v", account)
	}
	ledger, err := h.manual.Get(h.ctx, h.portfolioID, first.ID)
	if err != nil || !ledger.Pending || ledger.AppliedVersion == nil || *ledger.AppliedVersion != state.Version || ledger.Items[0].Quantity != "3" {
		t.Fatalf("new edit %+v %v", ledger, err)
	}
	next, _ := h.runCycle()
	if next.Point.TotalUSD != "59900" || !hasManualAdjustment(next.Point) || during.Point.TotalUSD != "39900" {
		t.Fatalf("next %+v", next)
	}
	archived, err := h.identities.Archive(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil {
		t.Fatal(err)
	}
	exited, _ := h.runCycle()
	if exited.Point.TotalUSD != "60000" || exited.Point.BoundaryWithdrawalUSD != "-100" {
		t.Fatalf("signed exit %+v", exited)
	}
	retained, err := h.manual.Get(h.ctx, h.portfolioID, archived.ID)
	if err != nil || len(retained.Items) != 1 {
		t.Fatalf("archive storage %+v %v", retained, err)
	}
	if _, err := h.manual.Update(h.ctx, h.portfolioID, archived.ID, manual.Input{Symbol: "USD", Quantity: "1", Version: retained.Version}); !errors.Is(err, identity.ErrArchived) {
		t.Fatalf("archived write %v", err)
	}
}

func TestManualStrictPlatformPricingAndConcurrentUpdates(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "0"}
	item := manualEnabled(t, h, a)
	symbol := "MANUAL-" + uuid.NewString()[:8]
	state := manualSave(t, h, item, symbol, "-2")
	now := time.Now().UTC()
	for _, row := range []struct {
		platform string
		at       time.Time
	}{{"binance", now}, {"okx", now.Add(-25 * time.Hour)}, {"okx", now.Add(time.Hour)}} {
		if err := h.catalog.RecordPrice(h.ctx, sqlcgen.New(h.db), adapters.Platform(row.platform), adapters.Price{AssetSymbol: symbol, Price: "20000", QuoteSymbol: "USD", ObservedAt: row.at}); err != nil {
			t.Fatal(err)
		}
	}
	unpriced, job := h.runCycle()
	account := h.accountsByType(job, item.ID)["manual"]
	if unpriced.Point.TotalUSD != "0" || len(account.Omitted) != 1 || account.Omitted[0].Equity != "-2" {
		t.Fatalf("fallback violated %+v %+v", unpriced, account)
	}
	if _, err := h.valuation.Rebuild(h.ctx, h.portfolioID); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := h.valuation.PointByJob(h.ctx, h.portfolioID, job)
	if err != nil || rebuilt.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("rebuild used fallback %+v %v", rebuilt, err)
	}
	results := make(chan error, 2)
	for _, qty := range []string{"-3", "-4"} {
		go func() {
			_, err := h.manual.Update(h.ctx, h.portfolioID, item.ID, manual.Input{Symbol: symbol, Quantity: qty, Version: state.Version, ActorUserID: h.userID})
			results <- err
		}()
	}
	success, conflict := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, identity.ErrPrecondition) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("writers success=%d conflict=%d", success, conflict)
	}
}

func TestManualExcludedRecordsStayEditableAndInvalidConnectionsDoNotBlock(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "0", btc: "10000"}
	item := manualEnabled(t, h, a)
	state := manualSave(t, h, item, "BTC", "1")
	_, firstJob := h.runCycle()
	listingID := state.Items[0].ListingID
	listing, err := h.catalog.GetListing(h.ctx, listingID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutExclusion(h.ctx, listingID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Ignored: true, Reason: "Explicitly excluded synthetic holding.", Version: listing.ExclusionVersion}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='invalid' WHERE id=?", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	excluded, job := h.runCycle()
	account := h.accountsByType(job, item.ID)["manual"]
	if excluded.Point.TotalUSD != "0" || account.Freshness != valuation.FreshnessFresh || account.Completeness() != valuation.CompletenessComplete || excluded.IdentityCount != 0 {
		t.Fatalf("excluded or invalid %+v %+v", excluded, account)
	}
	ledger, err := h.manual.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || len(ledger.Items) != 1 {
		t.Fatalf("excluded editor %+v %v", ledger, err)
	}
	manualSave(t, h, item, "BTC", "2")
	if view, err := h.financial.AccountBalances(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, account.AccountID); err != nil || len(view.Balances) != 1 || !view.Balances[0].Excluded || view.Balances[0].Equity != "1" || view.Balances[0].ManualChange != "updated" || view.Balances[0].EquityUSD != nil {
		t.Fatalf("excluded read %+v %v", view, err)
	}
	tree, err := h.financial.Tree(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Platforms[0].Identities[0].Accounts) != 2 {
		t.Fatal("excluded nonzero holdings or failed Account was hidden")
	}
	old, err := h.valuation.PointByJob(h.ctx, h.portfolioID, firstJob)
	if err != nil || old.TotalUSD != "10000" {
		t.Fatalf("exclusion rewrote old total %+v %v", old, err)
	}
}

func TestManualBalancePresentationBeforeAndBetweenCollections(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0"})
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	state := manualSave(t, h, item, "USD", "100")
	list, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{Limit: 50})
	if err != nil || len(list.Items) != 1 || list.Items[0].ManualChange != "added" || list.Items[0].PriceUSD != nil || list.TotalUSD != nil {
		t.Fatalf("before first collection: %+v %v", list, err)
	}
	view, err := h.financial.AccountBalances(h.ctx, p, state.AccountID)
	if err != nil || len(view.Balances) != 1 || view.Balances[0].Equity != "100" {
		t.Fatalf("account before collection: %+v %v", view, err)
	}
	tree, err := h.financial.Tree(h.ctx, p)
	if err != nil || len(tree.Platforms) != 1 || len(tree.Platforms[0].Identities[0].Accounts) != 1 || tree.TotalUSD != nil {
		t.Fatalf("tree before collection: %+v %v", tree, err)
	}
	first, _ := h.runCycle()
	manualSave(t, h, item, "USD", "150")
	manualSave(t, h, item, "BTC", "-0.000000000000000000000000000001")
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{Limit: 1, Offset: 1})
	if err != nil || list.Total != 2 || len(list.Items) != 1 || list.Items[0].Symbol != "USD" || list.Items[0].Equity != "100" || list.Items[0].ManualChange != "updated" || *list.Items[0].ManualQuantity != "150" || *list.TotalUSD != "100" {
		t.Fatalf("updated and paginated: %+v %v", list, err)
	}
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{Symbol: " btc ", Limit: 50})
	if err != nil || list.Total != 1 || list.Items[0].Symbol != "BTC" || list.Items[0].ManualChange != "added" || *list.TotalUSD != "0" {
		t.Fatalf("pending search: %+v %v", list, err)
	}
	manualSave(t, h, item, "USD", "100")
	view, err = h.financial.AccountBalances(h.ctx, p, state.AccountID)
	if err != nil || view.Balances[1].ManualChange != "" {
		t.Fatalf("reverted: %+v %v", view, err)
	}
	manualSave(t, h, item, "USD", "0")
	manualSave(t, h, item, "BTC", "0")
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{Limit: 50})
	if err != nil || list.Total != 1 || list.Items[0].ManualChange != "deleted" || list.Items[0].Equity != "100" || *list.TotalUSD != "100" {
		t.Fatalf("pending deletion: %+v %v", list, err)
	}
	h.runCycle()
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{Limit: 50})
	if err != nil || list.Total != 0 || *list.TotalUSD != "0" {
		t.Fatalf("applied deletion: %+v %v", list, err)
	}
	manualSave(t, h, item, "USD", "200")
	tree, err = h.financial.Tree(h.ctx, p)
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 1 || *tree.TotalUSD != "0" {
		t.Fatalf("pending account hidden: %+v %v", tree, err)
	}
	old, err := h.valuation.Get(h.ctx, h.portfolioID, first.Point.ID)
	if err != nil || old.TotalUSD != "100" {
		t.Fatalf("history changed: %+v %v", old, err)
	}
}
