package collection

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/okx"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

// TestOKXFixturesStoreAndValue runs one cycle through the live OKX adapter replaying the committed
// fixtures, so every recorded balance, position, instrument, and price must be accepted by the
// catalog and the database and valued without omissions.
func TestOKXFixturesStoreAndValue(t *testing.T) {
	h := newHarness(t)
	paths, err := filepath.Glob(filepath.Join("..", "adapters", "okx", "testdata", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no okx fixtures: %v", err)
	}
	// OKX tells some endpoints apart by query alone, so answers are keyed by the full request.
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := answers[r.Method+" "+r.URL.RequestURI()]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(f.Status)
		_, _ = w.Write(f.Body)
	}))
	t.Cleanup(server.Close)
	// The harness registry is shared by the identity and collection services.
	h.registry.Register(adapters.PlatformOKX, okx.New(&okx.Options{Host: server.URL, HTTP: server.Client()}))

	item := h.createIdentity("OKX fixtures", adapters.PlatformOKX, adapters.StructureOKXUnified, okx.TypeTrading, okx.TypeFunding, okx.TypeEarn)
	item = h.storeCredentials(item, "fixture-key")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || enabled.ExternalRef == nil || *enabled.ExternalRef != okx.RedactedUID {
		t.Fatalf("enable: err=%v outcome=%+v ref=%v", err, outcome, enabled.ExternalRef)
	}

	summary, jobID := h.runCycle()
	if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("summary %+v", summary)
	}
	accounts := h.accountsByType(jobID, item.ID)
	if trading := accounts[string(okx.TypeTrading)]; trading.Method != valuation.MethodReported || trading.ValueUSD == nil || trading.Equity == nil || *trading.ValueUSD != *trading.Equity {
		t.Fatalf("trading valuation %+v", trading)
	}
	for _, accountType := range []adapters.AccountType{okx.TypeFunding, okx.TypeEarn} {
		a := accounts[string(accountType)]
		if a.Method != valuation.MethodCalculated || a.ValueUSD == nil || len(a.Omitted) != 0 || a.Freshness != valuation.FreshnessFresh {
			t.Fatalf("%s valuation %+v", accountType, a)
		}
	}
	instruments, err := h.catalog.ListInstruments(h.ctx, catalog.ListFilter{Platform: string(adapters.PlatformOKX), Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(instruments.Items) == 0 {
		t.Fatal("positions stored no instruments")
	}
	assertLinearContractNotional(t, h, accounts[string(okx.TypeTrading)].AccountID)
}

// assertLinearContractNotional checks the recorded swap position, which OKX counts in contracts of
// ctVal base coins each: its USD notional is contracts × contract size × mark price.
func assertLinearContractNotional(t *testing.T, h *harness, accountID uuid.UUID) {
	t.Helper()
	view, err := h.valuation.AccountPositions(h.ctx, valuation.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Positions) == 0 {
		t.Fatal("no position in the recorded trading account")
	}
	for _, position := range view.Positions {
		if position.Kind != adapters.KindLinearPerp || position.QuantityUnit != adapters.UnitContracts ||
			position.ContractSize == nil || position.MarkPrice == nil || position.NotionalUSD == nil {
			t.Fatalf("recorded position %+v", position)
		}
		want := decimal.Format(decimal.Abs(decimal.Mul(decimal.Mul(decimal.MustParse(position.Quantity),
			decimal.MustParse(*position.ContractSize)), decimal.MustParse(*position.MarkPrice))))
		if *position.NotionalUSD != want {
			t.Fatalf("%s notional %s, want %s (%s contracts x %s x mark %s)",
				position.Symbol, *position.NotionalUSD, want, position.Quantity, *position.ContractSize, *position.MarkPrice)
		}
	}
}
