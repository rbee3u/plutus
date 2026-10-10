package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/okx"
	"github.com/rbee3u/plutus/internal/shared/decimal"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
)

// TestOKXFixturesStoreAndValue runs one cycle through the live OKX adapter replaying the committed
// fixtures, so every synthetic balance, position, futures contract, and price must be accepted by the
// catalog and the database and valued without omissions.
func TestOKXFixturesStoreAndValue(t *testing.T) {
	h := newHarness(t)
	files := loadAdapterSamples(t, adapters.PlatformOKX, "")
	answers := map[string]fixture.File{}
	for _, f := range files {
		answers[f.Request.Endpoint()] = f
	}
	var source struct {
		Data []struct {
			UID string `json:"uid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(answers["GET /api/v5/account/config"].Body, &source); err != nil || len(source.Data) == 0 || source.Data[0].UID == "" {
		t.Fatalf("fixture lacks its customer reference: %v", err)
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
	h.registry.Register(adapters.PlatformOKX, okx.New(&okx.Options{Host: server.URL, HTTP: offline.Client(t, server)}))

	item := h.createIdentity("OKX fixtures", adapters.PlatformOKX, adapters.StructureOKXUnified, okx.TypeTrading, okx.TypeFunding, okx.TypeEarn)
	item = h.storeCredentials(item, "fixture-key")
	enabled, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil || !outcome.OK || enabled.ExternalRef == nil || *enabled.ExternalRef != source.Data[0].UID {
		t.Fatalf("enable: err=%v outcome=%+v ref=%v", err, outcome, enabled.ExternalRef)
	}

	summary, cycleID := h.runCycle()
	if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
		t.Fatalf("summary %+v", summary)
	}
	accounts := h.accountsByType(cycleID, item.ID)
	// Earn = 20 USDT in savings + 0.1 ETH at 2000 USD in staking.
	assertFixtureAccountValues(t, accounts, map[string]fixtureAccountValue{
		"trading": {valuation.MethodReported, "10000"},
		"funding": {valuation.MethodCalculated, "100"},
		"earn":    {valuation.MethodCalculated, "220"},
		"manual":  {valuation.MethodCalculated, "0"},
	})
	if summary.Point.TotalUSD != "10320" {
		t.Fatalf("Portfolio total = %s, want 10320", summary.Point.TotalUSD)
	}
	futuresContracts, err := h.catalog.ListFuturesContracts(h.ctx, catalog.ListFilter{PortfolioID: h.portfolioID, Platform: string(adapters.PlatformOKX), Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(futuresContracts.Items) == 0 {
		t.Fatal("positions stored no futures_contracts")
	}
	assertOKXTradingPosition(t, h, accounts[string(okx.TypeTrading)].AccountID)
}

// The synthetic swap is one 0.01 BTC contract at 50000 USD, worth 500 USD.
func assertOKXTradingPosition(t *testing.T, h *harness, accountID uuid.UUID) {
	t.Helper()
	view, err := h.financial.AccountPositions(h.ctx, financial.PortfolioContext{ID: h.portfolioID, Timezone: "UTC"}, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Positions) != 1 {
		t.Fatalf("stored positions %+v, want one synthetic swap", view.Positions)
	}
	position := view.Positions[0]
	if position.PositionKey != "BTC-USDT-SWAP:net" || position.Kind != adapters.KindLinearPerpetual || position.Side != adapters.SideLong ||
		position.SourceQuantityUnit != adapters.UnitContracts || decimal.Compact(position.SourceQuantity) != "1" ||
		position.ContractSize == nil || decimal.Compact(*position.ContractSize) != "0.01" ||
		position.MarkPrice == nil || decimal.Compact(*position.MarkPrice) != "50000" ||
		position.NotionalUSD == nil || decimal.Compact(*position.NotionalUSD) != "500" {
		t.Fatalf("stored synthetic swap %+v", position)
	}
}
