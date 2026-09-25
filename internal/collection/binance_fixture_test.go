package collection

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/binance"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

// TestBinanceFixturesStoreAndValue runs one cycle through the live Binance adapter replaying the
// committed fixtures, so every recorded balance, position, instrument, and price must be
// accepted by the catalog and the database and valued without omissions.
func TestBinanceFixturesStoreAndValue(t *testing.T) {
	h := newHarness(t)
	registerBinanceReplay(t, h, "")

	item := h.createIdentity("Binance fixtures", adapters.PlatformBinance, adapters.StructureBinancePM,
		binance.TypePortfolioMargin, binance.TypeSpot, binance.TypeFunding, binance.TypeEarn)
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
	for _, accountType := range []adapters.AccountType{binance.TypeSpot, binance.TypeFunding, binance.TypeEarn} {
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
}

// TestBinanceClassicScenarioStoresAndValues runs one cycle of a Classic Trading Identity with all
// seven Account Types: the documented Classic answers laid over the recorded set (ADR 0027). Every
// Classic Account is valued from its Balances, and the COIN-M position keeps its inverse notional.
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
}

// registerBinanceReplay registers the live Binance adapter against a server that replays the
// committed recorded fixtures, overlaid with the documented scenario in testdata/documented/overlay
// when overlay is not empty.
func registerBinanceReplay(t *testing.T, h *harness, overlay string) {
	t.Helper()
	testdata := filepath.Join("..", "adapters", "binance", "testdata")
	paths, err := filepath.Glob(filepath.Join(testdata, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no binance fixtures: %v", err)
	}
	answers := map[string]fixture.File{}
	for _, path := range paths {
		f, err := fixture.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if f.Fixture.Scenario == fixture.ScenarioRecorded {
			answers[f.Fixture.Endpoint] = f
		}
	}
	if overlay != "" {
		documented, err := filepath.Glob(filepath.Join(testdata, "documented", overlay, "*.json"))
		if err != nil || len(documented) == 0 {
			t.Fatalf("no documented %s fixtures: %v", overlay, err)
		}
		for _, path := range documented {
			f, err := fixture.Read(path)
			if err != nil {
				t.Fatal(err)
			}
			answers[f.Fixture.Endpoint] = f
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Headers name a call by its method, path, and query without the signing parameters.
		query := r.URL.Query()
		for _, signing := range []string{"timestamp", "recvWindow", "signature"} {
			query.Del(signing)
		}
		target := r.URL.Path
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
		f, ok := answers[r.Method+" "+target]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, target)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(f.Status)
		_, _ = w.Write(f.Body)
	}))
	t.Cleanup(server.Close)
	live := binance.New(&binance.Options{Hosts: binance.Hosts{API: server.URL, PAPI: server.URL, FAPI: server.URL, DAPI: server.URL}, HTTP: server.Client()})
	// The harness registry is shared by the identity and collection services.
	h.registry.Register(adapters.PlatformBinance, live)
}

// assertInverseNotional checks the inverse positions of an Account, whose contract size is a USD
// face value: one DOGEUSD_PERP contract is 10 USD whatever the DOGE mark price is.
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
