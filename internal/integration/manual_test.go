package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/manual"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/apperr"
)

type manualTestAdapter struct {
	reported  *string
	unpriced  bool
	usd       string
	btc       string
	failed    bool
	positions bool
	onPrices  func()
}

func (a *manualTestAdapter) Validate(context.Context, adapter.Credentials, adapter.AccountStructure) (adapter.ValidationResult, error) {
	return adapter.ValidationResult{ExternalRef: "manual-test", MarginMode: adapter.MarginOKXPM}, nil
}

func (a *manualTestAdapter) Collect(_ context.Context, _ adapter.Credentials, _ adapter.AccountStructure, types []adapter.AccountType) (adapter.Batch, error) {
	at := time.Now().UTC()
	if a.failed {
		return adapter.Batch{}, adapter.NewError(adapter.ClassTemporary, "Synthetic private failure.")
	}
	batch := adapter.Batch{CollectedAt: at, MarginMode: adapter.MarginOKXPM}
	for _, typ := range types {
		if typ == adapter.TypeManual {
			panic("Manual must not reach a private Adapter")
		}
		report := adapter.AccountReport{AccountType: typ, Status: adapter.StatusSucceeded, CollectedAt: at, Equity: a.reported}
		if a.usd != "0" {
			report.Balances = []adapter.Balance{{AssetSymbol: "USD", Equity: a.usd}}
		}
		if a.unpriced {
			report.Balances = append(report.Balances, adapter.Balance{AssetSymbol: "MYSTERY", Equity: "2"})
		}
		if a.positions {
			report.Positions = []adapter.Position{{PositionKey: "manual-test-position", FuturesContract: adapter.FuturesContract{Symbol: "BTC-USD", Kind: "linear_perpetual", BaseAssetSymbol: "BTC", QuoteAssetSymbol: "USD", SettlementAssetSymbol: "USD"}, Side: "long", PositionMode: "one_way", SourceQuantity: "1", SourceQuantityUnit: "base"}}
		}
		batch.Reports = append(batch.Reports, report)
	}
	return batch, nil
}

func (a *manualTestAdapter) CollectPrices(_ context.Context, symbols []string) ([]adapter.Price, error) {
	if a.onPrices != nil {
		a.onPrices()
	}
	return adapter.SymbolPrices(symbols, []string{"USD"}, func(symbol, _ string) (adapter.Quote, bool) {
		return adapter.Quote{Price: a.btc, SourceMarketRef: "BTCUSD"}, symbol == "BTC" && a.btc != ""
	}, time.Now().UTC()), nil
}

func manualEnabled(t *testing.T, h *harness, a *manualTestAdapter) identity.Identity {
	t.Helper()
	h.registry.Register(adapter.PlatformOKX, a)
	item := h.createIdentity("Manual tests", adapter.PlatformOKX, adapter.StructureOKXUnified, "trading")
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
	in := manual.Input{AssetSymbol: symbol, Quantity: quantity, Version: current.Version, ActorUserID: h.userID}
	exists := false
	for _, balance := range current.Items {
		if balance.AssetSymbol == symbol {
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

func hasManualBalanceChange(point valuation.Point) bool {
	for _, issue := range point.Issues {
		if issue.Code == "manual_balance_changed" {
			return true
		}
	}
	return false
}

func TestManualCRUDVersionsAndCycleInputs(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("No credentials", adapter.PlatformOKX, adapter.StructureOKXUnified, "trading")
	s := h.manual
	initial, err := s.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || initial.Pending || len(initial.Items) != 0 {
		t.Fatalf("initial %+v %v", initial, err)
	}
	next, err := s.Create(h.ctx, h.portfolioID, item.ID, manual.Input{AssetSymbol: " usd ", Quantity: "100.000", Version: initial.Version, ActorUserID: h.userID})
	if err != nil || !next.Pending || next.Items[0].Quantity != "100" {
		t.Fatalf("create %+v %v", next, err)
	}
	in := manual.Input{AssetSymbol: "USD", Quantity: "5", Version: next.Version, ActorUserID: h.userID}
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
	for _, tc := range []struct{ quantity, message string }{
		{"abc", "Enter a valid quantity."},
		{"", "Enter a valid quantity."},
		{"1e3", "Use a plain decimal quantity with up to 35 integer digits and 30 decimal places."},
		{"1/2", "Use a plain decimal quantity with up to 35 integer digits and 30 decimal places."},
		{"0.0000000000000000000000000000001", "Use a plain decimal quantity with up to 35 integer digits and 30 decimal places."},
		{"999999999999999999999999999999999999", "Use a plain decimal quantity with up to 35 integer digits and 30 decimal places."},
	} {
		in.Quantity = tc.quantity
		for _, mutate := range []func(context.Context, uuid.UUID, uuid.UUID, manual.Input) (manual.State, error){s.Create, s.Update} {
			if _, err := mutate(h.ctx, h.portfolioID, item.ID, in); apperr.KindOf(err) != apperr.KindInvalid || err.Error() != tc.message {
				t.Fatalf("quantity %q: got %v, want validation error %q", tc.quantity, err, tc.message)
			}
		}
	}
	collectionCycle, _, err := h.cycles.Enqueue(h.ctx, h.portfolioID, cycle.TriggerUser)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := s.CycleInputs(h.ctx, h.portfolioID)
	if err != nil || len(inputs) != 1 || inputs[0].RevisionNumber != 2 || inputs[0].Items[0].Quantity != "100" {
		t.Fatalf("cycle inputs %v %v", inputs, err)
	}
	t.Cleanup(func() {
		_, _ = h.db.ExecContext(context.Background(), "UPDATE collection_cycles SET status='failed', finished_at=UTC_TIMESTAMP(6) WHERE portfolio_id=? AND id=? AND status='queued'", h.portfolioID[:], collectionCycle.ID[:])
	})
	summary, err := h.collection.RunCycle(h.ctx, h.portfolioID, collectionCycle.ID)
	if err != nil || summary.Point.TotalUSD != "100" || summary.IdentityCount != 0 || summary.ReportFailed != 1 {
		t.Fatalf("unconfigured %+v %v", summary, err)
	}
	changed := manualSave(t, h, item, "USD", "-100")
	applied, err := s.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || applied.AppliedVersion == nil || *applied.AppliedVersion != next.Version || !applied.Pending || applied.AppliedItems[0].Quantity != "100" || applied.Items[0].Quantity != "-100" {
		t.Fatalf("pending %+v %v", applied, err)
	}
	in.Version = changed.Version
	in.Quantity = "0"
	cleared, err := s.Update(h.ctx, h.portfolioID, item.ID, in)
	if err != nil || len(cleared.Items) != 0 || !cleared.Pending {
		t.Fatalf("zero %+v %v", cleared, err)
	}
	if _, err := s.Delete(h.ctx, h.portfolioID, item.ID, manual.Input{AssetSymbol: "USD", Version: cleared.Version}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("delete missing %v", err)
	}
	var revisions, audits int
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM manual_revisions WHERE account_id=?", initial.AccountID[:]).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := h.db.QueryRowContext(h.ctx, "SELECT COUNT(*) FROM audit_events WHERE portfolio_id=? AND action LIKE 'manual.balance.%'", h.portfolioID[:]).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if revisions != 4 || audits != 3 {
		t.Fatalf("revisions=%d audits=%d", revisions, audits)
	}
	// Revision 3 was never collected and can expire. Its audit must still name
	// the changed asset and both quantities without consulting that revision.
	q := sqlcgen.New(h.db)
	if _, err := q.PruneManualRevisionBalances(h.ctx, sqlcgen.PruneManualRevisionBalancesParams{PortfolioID: h.portfolioID[:], AccountID: initial.AccountID[:], RevisionNumber: 3}); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteManualRevision(h.ctx, sqlcgen.DeleteManualRevisionParams{PortfolioID: h.portfolioID[:], AccountID: initial.AccountID[:], RevisionNumber: 3}); err != nil {
		t.Fatal(err)
	}
	page, err := audit.NewService(h.db).List(h.ctx, h.portfolioID, &audit.ListFilter{ResourceType: "account"})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("manual audit: %+v %v", page, err)
	}
	for i, want := range []struct{ before, after string }{{"-100", ""}, {"100", "-100"}, {"", "100"}} {
		event := page.Items[i]
		if event.ActorUserID == nil || *event.ActorUserID != h.userID || event.ResourceID != initial.AccountID || event.OccurredAt.IsZero() {
			t.Fatalf("manual audit identity: %+v", event)
		}
		for j, state := range []*map[string]any{event.Before, event.After} {
			quantity := []string{want.before, want.after}[j]
			if state == nil {
				t.Fatal("missing manual audit state")
			}
			value, exists := (*state)["balance"]
			if !exists {
				t.Fatal("missing changed balance")
			}
			if quantity == "" {
				if value != nil {
					t.Fatalf("absent balance: %v", value)
				}
				continue
			}
			balance, ok := value.(map[string]any)
			if !ok || balance["quantity"] != quantity || balance["assetSymbol"] != "USD" || balance["assetId"] != next.Items[0].AssetID.String() {
				t.Fatalf("changed balance: %v, want %s USD", value, quantity)
			}
		}
	}
}

func TestManualConservationHistoryAndSettlement(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "130"}
	item := manualEnabled(t, h, a)
	first, firstCycle := h.runCycle()
	if first.Point.TotalUSD != "130" {
		t.Fatal(first)
	}
	a.usd = "30"
	manualSave(t, h, item, "USD", "100")
	second, secondCycle := h.runCycle()
	if second.Point.TotalUSD != "130" || second.Point.BoundaryContributionUSD != "0" || !hasManualBalanceChange(second.Point) {
		t.Fatalf("conservation %+v", second)
	}
	current, err := h.manual.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil || current.Pending {
		t.Fatalf("applied %+v %v", current, err)
	}
	accounts := h.accountsByType(secondCycle, item.ID)
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
	if hasManualBalanceChange(third.Point) {
		t.Fatal("edit then undo marked adjustment")
	}
	a.failed = true
	failed, failedCycle := h.runCycle()
	if failed.IdentityFailed != 1 || failed.ReportFailed != 1 {
		t.Fatalf("masked private failure %+v", failed)
	}
	accounts = h.accountsByType(failedCycle, item.ID)
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
	old, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, firstCycle)
	if err != nil || old.TotalUSD != "130" {
		t.Fatalf("history changed %+v %v", old, err)
	}
	rebuilt, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, secondCycle)
	if err != nil || rebuilt.TotalUSD != "130" || !hasManualBalanceChange(rebuilt) || rebuilt.BoundaryContributionUSD != "0" {
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
	tree, err := h.financial.Overview(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 2 {
		t.Fatalf("net zero holdings %v %v", tree, err)
	}
	manualSave(t, h, item, "USD", "0")
	manualSave(t, h, item, "BTC", "2")
	priced, cycleID := h.runCycle()
	if priced.Point.TotalUSD != "40100" {
		t.Fatalf("manual-only price %+v", priced)
	}
	before := h.accountsByType(cycleID, item.ID)["manual"].ManualModifiedAt
	a.btc = "25000"
	moved, cycleID := h.runCycle()
	if moved.Point.TotalUSD != "50100" || hasManualBalanceChange(moved.Point) || !h.accountsByType(cycleID, item.ID)["manual"].ManualModifiedAt.Equal(*before) {
		t.Fatalf("price-only %+v", moved)
	}
	manualSave(t, h, item, "UNKNOWN", "-1")
	missing, cycleID := h.runCycle()
	unknown := h.accountsByType(cycleID, item.ID)["manual"]
	if missing.Point.Completeness() != valuation.CompletenessIncomplete || len(unknown.Omitted) != 1 || unknown.Omitted[0].Equity != "-1" {
		t.Fatalf("negative missing %+v %+v", missing, unknown)
	}
	manualSave(t, h, item, "UNKNOWN", "0")
	manualSave(t, h, item, "BTC", "0")
	a.usd = "0"
	empty, _ := h.runCycle()
	tree, err = h.financial.Overview(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || empty.Point.AccountCount != 2 || len(tree.Platforms[0].Identities[0].Accounts) != 0 {
		t.Fatalf("empty %v %+v %v", tree, empty, err)
	}
	a.positions = true
	h.runCycle()
	tree, err = h.financial.Overview(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 1 {
		t.Fatalf("position retained %v %v", tree, err)
	}
}

func TestManualNewIdentityArchiveAndDuringCollectionEdits(t *testing.T) {
	h := newHarness(t)
	a := &manualTestAdapter{usd: "0", btc: "20000"}
	first := manualEnabled(t, h, a)
	h.runCycle()
	item := h.createIdentity("Manual only", adapter.PlatformOKX, adapter.StructureOKXUnified)
	manualSave(t, h, item, "USD", "-100")
	entered, _ := h.runCycle()
	if entered.Point.TotalUSD != "-100" || entered.Point.BoundaryContributionUSD != "-100" || hasManualBalanceChange(entered.Point) {
		t.Fatalf("signed entry %+v", entered)
	}
	state := manualSave(t, h, first, "BTC", "2")
	a.onPrices = func() { manualSave(t, h, first, "BTC", "3") }
	during, cycleID := h.runCycle()
	a.onPrices = nil
	account := h.accountsByType(cycleID, first.ID)["manual"]
	if account.ValueUSD == nil || *account.ValueUSD != "40000" {
		t.Fatalf("frozen during collection %+v", account)
	}
	ledger, err := h.manual.Get(h.ctx, h.portfolioID, first.ID)
	if err != nil || !ledger.Pending || ledger.AppliedVersion == nil || *ledger.AppliedVersion != state.Version || ledger.Items[0].Quantity != "3" {
		t.Fatalf("new edit %+v %v", ledger, err)
	}
	next, _ := h.runCycle()
	if next.Point.TotalUSD != "59900" || !hasManualBalanceChange(next.Point) || during.Point.TotalUSD != "39900" {
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
	if err != nil || len(retained.Items) != 1 || retained.Pending || retained.AppliedVersion == nil || *retained.AppliedVersion != retained.Version {
		t.Fatalf("archive storage %+v %v", retained, err)
	}
	if _, err := h.manual.Update(h.ctx, h.portfolioID, archived.ID, manual.Input{AssetSymbol: "USD", Quantity: "1", Version: retained.Version}); !errors.Is(err, identity.ErrArchived) {
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
		if err := h.catalog.RecordPrice(h.ctx, sqlcgen.New(h.db), h.portfolioID, adapter.Platform(row.platform), adapter.Price{CanonicalAssetSymbol: symbol, Price: "20000", QuoteCanonicalAssetSymbol: "USD", SourceMarketRef: "BTCUSD", RetrievedAt: row.at}); err != nil {
			t.Fatal(err)
		}
	}
	unpriced, cycleID := h.runCycle()
	account := h.accountsByType(cycleID, item.ID)["manual"]
	if unpriced.Point.TotalUSD != "0" || len(account.Omitted) != 1 || account.Omitted[0].Equity != "-2" {
		t.Fatalf("fallback violated %+v %+v", unpriced, account)
	}
	rebuilt, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, cycleID)
	if err != nil || rebuilt.Completeness() != valuation.CompletenessIncomplete {
		t.Fatalf("retry used fallback %+v %v", rebuilt, err)
	}
	results := make(chan error, 2)
	for _, qty := range []string{"-3", "-4"} {
		go func() {
			_, err := h.manual.Update(h.ctx, h.portfolioID, item.ID, manual.Input{AssetSymbol: symbol, Quantity: qty, Version: state.Version, ActorUserID: h.userID})
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
	_, firstCycle := h.runCycle()
	assetID := state.Items[0].AssetID
	asset, err := h.catalog.GetAsset(h.ctx, h.portfolioID, assetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutExclusion(h.ctx, assetID, &catalog.ExclusionInput{PortfolioID: h.portfolioID, ActorUserID: h.userID, Excluded: true, Reason: "Explicitly excluded synthetic holding.", Version: asset.ExclusionVersion}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.ExecContext(h.ctx, "UPDATE identities SET connection_status='invalid' WHERE id=?", item.ID[:]); err != nil {
		t.Fatal(err)
	}
	excluded, cycleID := h.runCycle()
	account := h.accountsByType(cycleID, item.ID)["manual"]
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
	tree, err := h.financial.Overview(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Platforms[0].Identities[0].Accounts) != 2 {
		t.Fatal("excluded nonzero holdings or failed account was hidden")
	}
	old, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, firstCycle)
	if err != nil || old.TotalUSD != "10000" {
		t.Fatalf("exclusion rewrote old total %+v %v", old, err)
	}
}

func TestManualBalancePresentationBeforeAndBetweenCollections(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0"})
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	state := manualSave(t, h, item, "USD", "100")
	list, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IncludePendingManual: true, Limit: 50})
	if err != nil || len(list.Items) != 1 || list.Items[0].ManualChange != "added" || list.Items[0].PriceUSD != nil || list.TotalUSD != nil {
		t.Fatalf("before first collection: %+v %v", list, err)
	}
	view, err := h.financial.AccountBalances(h.ctx, p, state.AccountID)
	if err != nil || len(view.Balances) != 1 || view.Balances[0].Equity != "100" {
		t.Fatalf("account before collection: %+v %v", view, err)
	}
	tree, err := h.financial.Overview(h.ctx, p)
	if err != nil || len(tree.Platforms) != 1 || len(tree.Platforms[0].Identities[0].Accounts) != 1 || tree.Point != nil {
		t.Fatalf("tree before collection: %+v %v", tree, err)
	}
	first, firstID := h.runCycle()
	manualSave(t, h, item, "USD", "150")
	manualSave(t, h, item, "BTC", "-0.000000000000000000000000000001")
	collected, err := h.financial.AccountReport(h.ctx, p, state.AccountID, &firstID, false)
	if err != nil || len(collected.Balances) != 1 || collected.Balances[0].Equity != "100" || collected.Balances[0].ManualChange != "" {
		t.Fatalf("historical report admitted pending edits: %+v %v", collected, err)
	}
	pending, err := h.financial.AccountReport(h.ctx, p, state.AccountID, &firstID, true)
	if err != nil || len(pending.Balances) != 2 || *pending.Account.SourceReportID != *collected.Account.SourceReportID || pending.Balances[0].ManualChange != "added" || pending.Balances[0].Freshness != "" {
		t.Fatalf("explicit pending overlay: %+v %v", pending, err)
	}

	if collected.ManualOverlay != nil || pending.ManualOverlay == nil || len(pending.ManualOverlay.Versions) != 1 || pending.ManualOverlay.Versions[0].AccountID != state.AccountID || pending.ManualOverlay.Versions[0].Version <= state.Version {
		t.Fatalf("overlay versions: %+v", pending.ManualOverlay)
	}
	frozen, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{CycleID: &firstID})
	if err != nil || frozen.ManualOverlay != nil || len(frozen.Items) != 1 || frozen.Items[0].Equity != "100" || frozen.Items[0].ManualChange != "" {
		t.Fatalf("flat collected-only read: %+v %v", frozen, err)
	}

	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IncludePendingManual: true, Limit: 1, Offset: 1})
	if err != nil || list.Total != 2 || len(list.Items) != 1 || list.Items[0].AssetSymbol != "USD" || list.Items[0].Equity != "100" || list.Items[0].ManualChange != "updated" || *list.Items[0].ManualQuantity != "150" || *list.TotalUSD != "100" {
		t.Fatalf("updated and paginated: %+v %v", list, err)
	}
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IncludePendingManual: true, Symbol: " btc ", Limit: 50})
	if err != nil || list.Total != 1 || list.Items[0].AssetSymbol != "BTC" || list.Items[0].ManualChange != "added" || *list.TotalUSD != "0" {
		t.Fatalf("pending search: %+v %v", list, err)
	}
	manualSave(t, h, item, "USD", "100")
	view, err = h.financial.AccountBalances(h.ctx, p, state.AccountID)
	if err != nil || view.Balances[1].ManualChange != "" {
		t.Fatalf("reverted: %+v %v", view, err)
	}
	manualSave(t, h, item, "USD", "0")
	manualSave(t, h, item, "BTC", "0")
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IncludePendingManual: true, Limit: 50})
	if err != nil || list.Total != 1 || list.Items[0].ManualChange != "deleted" || list.Items[0].Equity != "100" || *list.TotalUSD != "100" {
		t.Fatalf("pending deletion: %+v %v", list, err)
	}
	h.runCycle()
	list, err = h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{IncludePendingManual: true, Limit: 50})
	if err != nil || list.Total != 0 || *list.TotalUSD != "0" {
		t.Fatalf("applied deletion: %+v %v", list, err)
	}
	manualSave(t, h, item, "USD", "200")
	tree, err = h.financial.Overview(h.ctx, p)
	if err != nil || len(tree.Platforms[0].Identities[0].Accounts) != 1 || tree.Point.TotalUSD != "0" {
		t.Fatalf("pending account hidden: %+v %v", tree, err)
	}
	old, err := h.valuation.Get(h.ctx, h.portfolioID, first.Point.CycleID)
	if err != nil || old.TotalUSD != "100" {
		t.Fatalf("history changed: %+v %v", old, err)
	}
}

func TestManualMappingAffectsNextPointAndPreservesHistory(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0", btc: "30000"})
	state := manualSave(t, h, item, "BTC", "2")
	before, collectionCycle := h.runCycle()
	asset, err := h.catalog.GetAsset(h.ctx, h.portfolioID, state.Items[0].AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.catalog.PutMapping(h.ctx, asset.ID, catalog.MappingInput{ActorUserID: h.userID, PortfolioID: h.portfolioID, MappingVersion: asset.MappingVersion, CanonicalAssetSymbol: "ETH"}); err != nil {
		t.Fatal(err)
	}
	after, err := h.valuation.PointByCycle(h.ctx, h.portfolioID, collectionCycle)
	if err != nil || after.TotalUSD != before.Point.TotalUSD || after.TotalUSD != "60000" {
		t.Fatalf("historical point changed: %+v %v", after, err)
	}
	next, _ := h.runCycle()
	if next.Point.TotalUSD != "0" || next.Point.IncompleteAccountCount != 1 {
		t.Fatalf("new mapping not adopted: %+v", next.Point)
	}
	found := false
	for _, issue := range next.Point.Issues {
		if issue.Code == "asset_mapping_changed" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing durable mapping marker")
	}
}

func TestPendingNewManualAccountDoesNotQualifyCollectedTotal(t *testing.T) {
	h := newHarness(t)
	item := manualEnabled(t, h, &manualTestAdapter{usd: "0"})
	for _, account := range item.Accounts {
		if account.AccountType == "funding" {
			var err error
			item, err = h.identities.Patch(h.ctx, h.portfolioID, item.ID, item.Version, identity.PatchInput{ActorUserID: h.userID, Accounts: []identity.AccountPatch{{ID: account.ID, Selected: new(true)}}})
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	manualSave(t, h, item, "USD", "100")
	_, point := h.runCycle()
	pending := h.createIdentity("Pending identity", adapter.PlatformBinance, adapter.StructureBinanceClassic, "spot")
	state := manualSave(t, h, pending, "USD", "200")
	p := financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}
	out, err := h.financial.FlatBalances(h.ctx, p, &financial.ListFilter{CycleID: &point, IncludePendingManual: true, IdentityID: &pending.ID})
	if err != nil || out.TotalUSD == nil || *out.TotalUSD != "0" || out.TotalCompleteness != valuation.CompletenessComplete || !out.Coverage.Complete || len(out.TotalIssues) != 0 || len(out.Items) != 1 || out.Items[0].ManualChange != "added" || out.ManualOverlay == nil || len(out.ManualOverlay.Versions) != 1 || out.ManualOverlay.Versions[0].Version != state.Version {
		t.Fatalf("pending account contaminated collected coverage or total: %+v %v", out, err)
	}
}
