package collection

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/binance"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/offline"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

// TestBinanceFixturesStoreAndValue runs one cycle through the live Binance adapter replaying the
// committed synthetic responses, so every selected balance, position, instrument, and price must be
// accepted by the catalog and the database and valued without omissions.
func TestBinanceFixturesStoreAndValue(t *testing.T) {
	h := newHarness(t)
	registerBinanceReplay(t, h, "")

	item := h.createIdentity("Binance fixtures", adapters.PlatformBinance, adapters.StructureBinancePM,
		binance.TypePortfolioMargin, binance.TypeSpot, binance.TypeFunding, binance.TypeEarn, binance.TypeDualInvestment)
	item = h.storeCredentials(item, "fixture-key")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || enabled.ExternalRef == nil || *enabled.ExternalRef != "123456789" {
		t.Fatalf("enable: err=%v outcome=%+v ref=%v", err, outcome, enabled.ExternalRef)
	}

	summary, jobID := h.runCycle()
	if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("summary %+v", summary)
	}
	accounts := h.accountsByType(jobID, item.ID)
	if pm := accounts[string(binance.TypePortfolioMargin)]; pm.Method != valuation.MethodReported || pm.ValueUSD == nil || pm.Equity == nil || *pm.ValueUSD != *pm.Equity {
		t.Fatalf("portfolio margin valuation %+v", pm)
	}
	for _, accountType := range []adapters.AccountType{binance.TypeSpot, binance.TypeFunding, binance.TypeEarn, binance.TypeDualInvestment} {
		a := accounts[string(accountType)]
		if a.Method != valuation.MethodCalculated || a.ValueUSD == nil || len(a.Omitted) != 0 || a.Freshness != valuation.FreshnessFresh {
			t.Fatalf("%s valuation %+v", accountType, a)
		}
	}
	instruments, err := h.catalog.ListInstruments(h.ctx, catalog.ListFilter{Platform: string(adapters.PlatformBinance), Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(instruments.Items) == 0 {
		t.Fatal("positions stored no instruments")
	}
	assertInverseNotional(t, h, accounts[string(binance.TypePortfolioMargin)].AccountID)
	assertSyntheticDualPositions(t, h, accounts[string(binance.TypeDualInvestment)])
}

// TestBinanceClassicScenarioStoresAndValues runs one cycle of a Classic
// Trading Identity with all supported Account Types: synthetic answers
// selected explicitly from the flat library (ADR 0009). Every Classic Account
// is valued from its Balances, and the COIN-M position keeps its inverse
// notional.
func TestBinanceClassicScenarioStoresAndValues(t *testing.T) {
	h := newHarness(t)
	registerBinanceReplay(t, h, "binance_classic")

	types := make([]adapters.AccountType, 0, 7)
	for _, info := range adapters.AccountTypes(adapters.StructureBinanceClassic) {
		types = append(types, info.ID)
	}
	item := h.createIdentity("Binance classic fixtures", adapters.PlatformBinance, adapters.StructureBinanceClassic, types...)
	item = h.storeCredentials(item, "fixture-key")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
	}

	summary, jobID := h.runCycle()
	if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("summary %s", summary.String())
	}
	accounts := h.accountsByType(jobID, item.ID)
	if len(accounts) != len(types) {
		t.Fatalf("%d valued Accounts, want %d", len(accounts), len(types))
	}
	for _, accountType := range types {
		a := accounts[string(accountType)]
		if a.Method != valuation.MethodCalculated || a.ValueUSD == nil || len(a.Omitted) != 0 || a.Freshness != valuation.FreshnessFresh {
			t.Fatalf("%s valuation %+v", accountType, a)
		}
	}
	assertInverseNotional(t, h, accounts["cm_futures"].AccountID)
	assertSyntheticDualPositions(t, h, accounts[string(binance.TypeDualInvestment)])
}

// registerBinanceReplay registers the live Binance adapter against a server that replays the
// committed samples, chosen explicitly by the test selection.
func registerBinanceReplay(t *testing.T, h *harness, selection string) {
	t.Helper()
	files := loadAdapterSamples(t, adapters.PlatformBinance, selection)
	answers := map[string]fixture.File{}
	for _, f := range files {
		answers[f.Request.Key()] = f
	}
	var mu sync.Mutex
	consumed := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Headers name a call by its method, path, and query without the signing parameters.
		query := r.URL.Query()
		for _, signing := range []string{"timestamp", "recvWindow", "signature"} {
			query.Del(signing)
		}
		service := "spot"
		switch {
		case strings.HasPrefix(r.URL.Path, "/fapi/"):
			service = "um"
		case strings.HasPrefix(r.URL.Path, "/dapi/"):
			service = "cm"
		case strings.HasPrefix(r.URL.Path, "/papi/"):
			service = "portfolio"
		}
		key := (fixture.Request{Service: service, Method: r.Method, Path: r.URL.Path, Query: query}).Key()
		f, ok := answers[key]
		if !ok {
			t.Errorf("unexpected request %s", key)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mu.Lock()
		consumed[key]++
		mu.Unlock()
		w.WriteHeader(f.Status)
		_, _ = w.Write(f.Body)
	}))
	if selection == "binance_classic_single_asset" || selection == "binance_classic_multi_assets" || selection == "binance_classic_one_way_isolated" || selection == "binance_classic_coin_m" {
		t.Cleanup(func() {
			mu.Lock()
			defer mu.Unlock()
			for key, f := range answers {
				want := 1
				if f.Request.Path == "/sapi/v1/account/info" {
					want = 2
				} // Validation and collection.
				if consumed[key] != want {
					t.Errorf("%s consumed %d times, want %d", key, consumed[key], want)
				}
			}
		})
	}
	t.Cleanup(server.Close)
	live := binance.New(&binance.Options{Hosts: binance.Hosts{API: server.URL, PAPI: server.URL, FAPI: server.URL, DAPI: server.URL}, HTTP: offline.Client(t, server)})
	// The harness registry is shared by the identity and collection services.
	h.registry.Register(adapters.PlatformBinance, live)
}

// assertInverseNotional checks the inverse positions of an Account, whose contract size is a USD
// face value, whatever the mark price. Contract terms retain their selected sample provenance.
func assertInverseNotional(t *testing.T, h *harness, accountID uuid.UUID) {
	t.Helper()
	view, err := h.valuation.AccountPositions(h.ctx, valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, accountID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, position := range view.Positions {
		if position.Kind != adapters.KindInversePerp && position.Kind != adapters.KindInverseDlv {
			continue
		}
		found = true
		if position.QuantityUnit != adapters.UnitContracts || position.ContractSize == nil || position.NotionalUSD == nil {
			t.Fatalf("inverse position %+v", position)
		}
		want := decimal.Format(decimal.Abs(decimal.Mul(decimal.MustParse(position.Quantity), decimal.MustParse(*position.ContractSize))))
		if *position.NotionalUSD != want {
			t.Fatalf("%s notional %s, want %s (quantity %s x contract size %s, never multiplied by the mark price)",
				position.Symbol, *position.NotionalUSD, want, position.Quantity, *position.ContractSize)
		}
	}
	if !found {
		t.Fatal("no inverse position in the COIN-M fixture")
	}
}

// assertSyntheticDualPositions checks stored principal and terms from the constructed response.
// Its USD value uses the independently constructed SOL price.
func assertSyntheticDualPositions(t *testing.T, h *harness, account valuation.AccountValuation) {
	t.Helper()
	if account.ValueUSD == nil || *account.ValueUSD != "12" {
		t.Fatalf("Dual principal valuation %+v, want 10 USDT + 0.1 SOL at 20 USD", account)
	}
	view, err := h.valuation.AccountDualPositions(h.ctx, valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, account.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.DualPositions) != 2 || len(view.Issues) != 0 {
		t.Fatalf("stored Dual view %+v", view)
	}
	want := map[string]struct{ symbol, principal, direction string }{
		"1001": {"USDT", "10", "buy_low"},
		"1002": {"SOL", "0.1", "sell_high"},
	}
	for _, position := range view.DualPositions {
		expected, ok := want[position.PlatformPositionRef]
		if !ok || position.InvestSymbol != expected.symbol || decimal.Format(decimal.MustParse(position.Principal)) != expected.principal ||
			position.Direction == nil || *position.Direction != expected.direction || len(position.MissingFields()) != 0 {
			t.Fatalf("stored Dual principal or terms %+v", position)
		}
		delete(want, position.PlatformPositionRef)
	}
	if len(want) != 0 {
		t.Fatalf("missing stored subscriptions %+v", want)
	}
}

// TestSyntheticBinanceClassicPositionsStoreAndValueBalances checks references and avoids double valuation.
func TestSyntheticBinanceClassicPositionsStoreAndValueBalances(t *testing.T) {
	for _, tc := range []struct{ selection, valueUSD string }{
		{"binance_classic_single_asset", "101.5"},
		// 101.5 USDT + 0.01 ETH at the synthetic 2000 USD price.
		{"binance_classic_multi_assets", "121.5"},
		// The isolated scene has its own coherent balance and PnL.
		{"binance_classic_one_way_isolated", "101"},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			h := newHarness(t)
			registerBinanceReplay(t, h, tc.selection)
			item := h.createIdentity("Synthetic Classic UM", adapters.PlatformBinance, adapters.StructureBinanceClassic, binance.TypeUMFutures)
			item = h.storeCredentials(item, "fixture-key")
			if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
				t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
			}
			summary, jobID := h.runCycle()
			if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
				t.Fatalf("summary %+v", summary)
			}
			accounts := h.accountsByType(jobID, item.ID)
			account := accounts[string(binance.TypeUMFutures)]
			if len(accounts) != 1 || account.Method != valuation.MethodCalculated || account.ValueUSD == nil || *account.ValueUSD != tc.valueUSD || account.Equity != nil || len(account.Omitted) != 0 {
				t.Fatalf("Balances must be valued once, independently of adjusted equity: %+v", account)
			}

			measures := map[string]*string{"adjusted": account.AdjustedEquity, "available": account.AvailableEquity, "initialRatio": account.InitialMarginRatio, "maintenanceRatio": account.MaintenanceMarginRatio}
			wantMeasures := map[string]string{"adjusted": "121.5", "available": "100", "initialRatio": "0.1", "maintenanceRatio": "0.02"}
			for name, value := range measures {
				if tc.selection != "binance_classic_multi_assets" {
					if value != nil {
						t.Fatalf("Single-Asset %s must remain unknown, got %s", name, *value)
					}
				} else if value == nil || decimal.Compact(*value) != wantMeasures[name] {
					t.Fatalf("stored Multi-Assets %s = %v, want %s", name, value, wantMeasures[name])
				}
			}
			view, err := h.valuation.AccountPositions(h.ctx, valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, account.AccountID)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]struct{ side, quantity, entry, pnl string }{
				"um:DOGEUSDT:LONG":  {adapters.SideLong, "100", "0.09", "1"},
				"um:DOGEUSDT:SHORT": {adapters.SideShort, "50", "0.11", "0.5"},
			}
			liquidation := "0.05"
			if tc.selection == "binance_classic_one_way_isolated" {
				want = map[string]struct{ side, quantity, entry, pnl string }{
					"um:DOGEUSDT:BOTH": {adapters.SideShort, "100", "0.11", "1"},
				}
				liquidation = "0.12"
			}
			if len(view.Positions) != len(want) || len(view.Issues) != 0 {
				t.Fatalf("stored positions %+v", view)
			}
			for _, position := range view.Positions {
				expected, ok := want[position.PlatformPositionRef]
				if position.Side == adapters.SideShort && tc.selection != "binance_classic_one_way_isolated" {
					liquidation = "0.2"
				}
				if !ok || position.Symbol != "DOGEUSDT" || position.Kind != adapters.KindLinearPerp || position.Side != expected.side || decimal.Compact(position.Quantity) != expected.quantity || position.QuantityUnit != adapters.UnitBase ||
					position.EntryPrice == nil || decimal.Compact(*position.EntryPrice) != expected.entry ||
					position.UnrealizedPnl == nil || decimal.Compact(*position.UnrealizedPnl) != expected.pnl ||
					position.Leverage == nil || decimal.Compact(*position.Leverage) != "5" ||
					position.LiquidationPrice == nil || decimal.Compact(*position.LiquidationPrice) != liquidation {
					t.Fatalf("stored position %+v", position)
				}
				delete(want, position.PlatformPositionRef)
			}
			if len(want) != 0 {
				t.Fatalf("missing stored positions %+v", want)
			}
		})
	}
}

// Synthetic inverse contracts retain their face value independently of market price.
func TestSyntheticBinanceCoinMStoresAndValues(t *testing.T) {
	h := newHarness(t)
	registerBinanceReplay(t, h, "binance_classic_coin_m")
	item := h.createIdentity("Synthetic Classic COIN-M", adapters.PlatformBinance, adapters.StructureBinanceClassic, binance.TypeCMFutures)
	item = h.storeCredentials(item, "fixture-key")
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
	}
	summary, jobID := h.runCycle()
	if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("summary %+v", summary)
	}
	accounts := h.accountsByType(jobID, item.ID)
	account := accounts[string(binance.TypeCMFutures)]
	if len(accounts) != 1 || account.Method != valuation.MethodCalculated || account.ValueUSD == nil || *account.ValueUSD != "22" || account.Equity != nil || account.AdjustedEquity != nil || account.AvailableEquity != nil || len(account.Omitted) != 0 {
		t.Fatalf("ETH balance must be valued once at synthetic 2000 USD, independently of position notional: %+v", account)
	}
	view, err := h.valuation.AccountPositions(h.ctx, valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, account.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ kind, side, entry, mark, pnl, liquidation, expiry string }{
		"cm:ETHUSD_PERP:BOTH":   {adapters.KindInversePerp, adapters.SideLong, "2000", "2500", "0.002", "1500", ""},
		"cm:ETHUSD_261225:BOTH": {adapters.KindInverseDlv, adapters.SideShort, "2000", "2500", "-0.001", "3500", "2026-12-25T08:00:00Z"},
	}
	if len(view.Positions) != len(want) || len(view.Issues) != 0 {
		t.Fatalf("stored positions %+v", view)
	}
	for _, position := range view.Positions {
		expected, ok := want[position.PlatformPositionRef]
		expiry := ""
		if position.ExpiryAt != nil {
			expiry = position.ExpiryAt.UTC().Format(time.RFC3339)
		}
		if !ok || position.Kind != expected.kind || position.Side != expected.side || decimal.Compact(position.Quantity) != map[string]string{adapters.KindInversePerp: "2", adapters.KindInverseDlv: "1"}[expected.kind] || position.QuantityUnit != adapters.UnitContracts || position.BaseSymbol == nil || *position.BaseSymbol != "ETH" || position.QuoteSymbol == nil || *position.QuoteSymbol != "USD" || position.SettlementSymbol == nil || *position.SettlementSymbol != "ETH" || position.ContractSize == nil || decimal.Compact(*position.ContractSize) != "10" || position.NotionalUSD == nil || decimal.Compact(*position.NotionalUSD) != map[string]string{adapters.KindInversePerp: "20", adapters.KindInverseDlv: "10"}[expected.kind] || expiry != expected.expiry {
			t.Fatalf("stored inverse contract %+v; expiry %s", position, expiry)
		}
		for name, pair := range map[string]struct {
			got  *string
			want string
		}{
			"entry": {position.EntryPrice, expected.entry}, "mark": {position.MarkPrice, expected.mark}, "pnl": {position.UnrealizedPnl, expected.pnl}, "liquidation": {position.LiquidationPrice, expected.liquidation}, "leverage": {position.Leverage, "5"},
		} {
			if pair.got == nil || decimal.Compact(*pair.got) != pair.want {
				t.Fatalf("stored %s = %v, want %s", name, pair.got, pair.want)
			}
		}
		delete(want, position.PlatformPositionRef)
	}
	if len(want) != 0 {
		t.Fatalf("missing stored positions %+v", want)
	}
}
