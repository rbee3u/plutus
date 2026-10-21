package kraken

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/recording"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
)

// testCreds carry the base64 secret "secret" for both key groups, which the replay verifies.
var testCreds = adapter.Credentials{APIKey: "spot-test", APISecret: "c2VjcmV0", Futures: &adapter.KeyPair{APIKey: "futures-test", APISecret: "c2VjcmV0"}}

// separateScenario selects the explicit separate-wallet interface samples (ADR
// 0009).
const separateScenario = string(adapter.StructureKrakenSeparate)

// allEndpoints are every endpoint the adapter and the recorder call.
var allEndpoints = []endpoint{
	epAssets, epAssetPairs, epSpotTicker, epAPIKeyInfo, epBalanceEx, epTradeBalance, epEarnStrategies,
	epEarnAllocations, epFuturesKey, epOpenPositions, epInstruments, epFuturesTickers, epBalancesSnapshot,
}

// accountEndpoints are the private endpoints that read an account's holdings.
var accountEndpoints = []string{
	epBalanceEx.String(), epTradeBalance.String(), epEarnStrategies.String(),
	epEarnAllocations.String(), epOpenPositions.String(),
}

// errorClasses is the class of each error answer, by the scenario its file name ends with: the
// synthetic refusals and the synthetic errors no recording can provoke.
var errorClasses = map[string]string{
	"invalid_key":          adapter.ClassAuthentication,
	"bad_signature":        adapter.ClassAuthentication,
	"authentication_error": adapter.ClassAuthentication,
	"permission_denied":    adapter.ClassAuthorization,
	"rate_limited":         adapter.ClassRateLimited,
	"throttled":            adapter.ClassRateLimited,
	"too_many_requests":    adapter.ClassRateLimited,
	"api_limit_exceeded":   adapter.ClassRateLimited,
	"invalid_nonce":        adapter.ClassTemporary,
	"unavailable":          adapter.ClassTemporary,
}

// fixtures are the committed answers: successful ones by endpoint, error answers by endpoint and
// scenario.
type fixtures struct {
	answers  map[string]fixture.File
	failures map[string]fixture.File
}

func failureKey(endpoint, scenario string) string { return endpoint + " " + scenario }

// loadFixtures reads the explicitly selected flat samples and error responses.
func loadFixtures(t *testing.T, scenario string) fixtures {
	t.Helper()
	out := fixtures{answers: map[string]fixture.File{}, failures: map[string]fixture.File{}}
	files, err := fixture.Load("testdata", sampleNames(scenario)...)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := f.Request.Method + " " + f.Request.Path
		if _, exists := out.answers[name]; exists {
			t.Fatalf("conflicting selected request: %s", name)
		}
		out.answers[name] = f
	}
	for variant := range errorClasses {
		ep := epEarnAllocations
		if variant == "invalid_key" || variant == "bad_signature" || variant == "invalid_nonce" {
			ep = epAPIKeyInfo
		}
		if variant == "authentication_error" || variant == "api_limit_exceeded" {
			ep = epOpenPositions
		}
		f, err := fixture.ReadSynthetic("testdata/" + fixture.Name(ep.path, variant) + ".json")
		if err != nil {
			t.Fatal(err)
		}
		out.failures[failureKey(ep.String(), variant)] = f
	}
	for _, variant := range []string{"invalid_key", "bad_signature"} {
		f, err := fixture.ReadSynthetic("testdata/" + fixture.Name(epFuturesKey.path, variant) + ".json")
		if err != nil {
			t.Fatal(err)
		}
		out.failures[failureKey(epFuturesKey.String(), variant)] = f
	}
	return out
}

// readDocumented reads a documentation-derived fixture (ADR 0009): it names
// its documentation and structure and has no recording date.
type replay struct {
	t         *testing.T
	adapter   *Adapter
	files     fixtures
	overrides map[string]string
	snapshot  []byte

	mu        sync.Mutex
	seen      []string
	lastNonce int64
	refusing  bool
}

func newReplay(t *testing.T, scenario string, overrides map[string]string) *replay {
	t.Helper()
	r := &replay{t: t, files: loadFixtures(t, scenario), overrides: overrides}
	r.snapshot, _, _ = r.answer(epBalancesSnapshot.String())
	var snap struct {
		Timestamp int64 `json:"timestamp"`
	}
	_ = json.Unmarshal(r.snapshot, &snap)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	now := time.UnixMilli(snap.Timestamp).Add(-time.Second)
	r.adapter = New(&Options{
		SpotHost: server.URL, FuturesHost: server.URL, HTTP: offline.Client(t, server),
		WebSocketURL: strings.Replace(server.URL, "http:", "ws:", 1) + "/ws/v1",
		Now:          func() time.Time { return now },
	})
	return r
}

// refuse makes the replay answer refused requests with their error answers.
func (r *replay) refuse() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refusing = true
}

func (r *replay) note(endpoint string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, endpoint)
}

func (r *replay) reached(endpoint string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, seen := range r.seen {
		if seen == endpoint {
			return true
		}
	}
	return false
}

func (r *replay) answer(endpoint string) ([]byte, int, bool) {
	if body, ok := r.overrides[endpoint]; ok {
		return []byte(body), http.StatusOK, true
	}
	f, ok := r.files.answers[endpoint]
	return f.Body, f.Status, ok
}

func (r *replay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/ws/v1" {
		r.note(epBalancesSnapshot.String())
		r.serveSnapshot(w, req)
		return
	}
	endpoint := req.Method + " " + req.URL.Path
	r.note(endpoint)

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		r.t.Error(err)
		w.WriteHeader(500)
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(raw))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
	identity, err := recording.HTTPRequest(adapter.PlatformKraken, req)
	expected, selected := r.files.answers[endpoint]
	if err != nil || !selected || identity.Key() != expected.Request.Key() {
		r.t.Errorf("unexpected request %s; expected %s (decode: %v)", identity.Key(), expected.Request.Key(), err)
		w.WriteHeader(500)
		return
	}
	body, status, ok := r.answer(endpoint)
	if scenario := r.refusal(req); scenario != "" {
		r.mu.Lock()
		refusing := r.refusing
		r.mu.Unlock()
		if !refusing {
			r.t.Errorf("%s: Kraken would refuse the request as %s", endpoint, scenario)
		}
		f, found := r.files.failures[failureKey(endpoint, scenario)]
		body, status, ok = f.Body, f.Status, found
	}
	if !ok {
		r.t.Errorf("no answer for %s", endpoint)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// refusal returns the scenario Kraken refuses a signed request with: a key it does not know, a
// wrong signature, or a Spot nonce that is not above the key's last one.
func (r *replay) refusal(req *http.Request) string {
	switch {
	case strings.HasPrefix(req.URL.Path, "/0/private/"):
		body, _ := io.ReadAll(req.Body)
		values, _ := url.ParseQuery(string(body))
		if req.URL.Path == epEarnAllocations.path && values.Get("hide_zero_allocations") != "true" {
			r.t.Error("Earn allocations must request only current holdings")
		}
		if req.Header.Get("Api-Key") != testCreds.APIKey {
			return "invalid_key"
		}
		if req.Header.Get("Api-Sign") != spotDigest(req.URL.Path, values.Get("nonce"), string(body)) {
			return "bad_signature"
		}
		nonce, err := strconv.ParseInt(values.Get("nonce"), 10, 64)
		r.mu.Lock()
		defer r.mu.Unlock()
		if err != nil || nonce <= r.lastNonce {
			return "invalid_nonce"
		}
		r.lastNonce = nonce
	case req.Header.Get("Apikey") != "":
		if req.Header.Get("Apikey") != testCreds.FuturesKey() {
			return "invalid_key"
		}
		if req.Header.Get("Authent") != digest(req.URL.RawQuery+strings.TrimPrefix(req.URL.Path, "/derivatives")) {
			return "bad_signature"
		}
	}
	return ""
}

// serveSnapshot plays Kraken's side of the balances feed: challenge, signed subscription, snapshot.
func (r *replay) serveSnapshot(w http.ResponseWriter, req *http.Request) {
	conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		r.t.Error(err)
		return
	}
	defer func() { _ = conn.CloseNow() }()
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err != nil {
		return
	}
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"event":"challenge","message":"test-challenge"}`))
	_, body, err := conn.Read(ctx)
	if err != nil {
		return
	}
	var subscription map[string]string
	if json.Unmarshal(body, &subscription) != nil || subscription["signed_challenge"] != digest("test-challenge") {
		r.t.Error("incorrect balances subscription signature")
	}
	_ = conn.Write(ctx, websocket.MessageText, r.snapshot)
}

// spotDigest is the Spot signature with the test secret.
func spotDigest(path, nonce, body string) string {
	hash := sha256.Sum256([]byte(nonce + body))
	mac := hmac.New(sha512.New, []byte("secret"))
	_, _ = mac.Write([]byte(path))
	_, _ = mac.Write(hash[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// digest is the futures and balances-feed signature with the test secret.
func digest(message string) string {
	hash := sha256.Sum256([]byte(message))
	mac := hmac.New(sha512.New, []byte("secret"))
	_, _ = mac.Write(hash[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func endpointNamed(t *testing.T, name string) endpoint {
	t.Helper()
	for _, ep := range allEndpoints {
		if ep.String() == name {
			return ep
		}
	}
	t.Fatalf("unknown endpoint %s", name)
	return endpoint{}
}

func reportsByType(t *testing.T, batch *adapter.Batch) map[adapter.AccountType]adapter.AccountReport {
	t.Helper()
	out := map[adapter.AccountType]adapter.AccountReport{}
	for i := range batch.Reports {
		report := batch.Reports[i]
		if report.Status != adapter.StatusSucceeded {
			t.Fatalf("%s: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
		}
		out[report.AccountType] = report
	}
	return out
}

func balances(report *adapter.AccountReport) map[string]string {
	out := map[string]string{}
	for _, balance := range report.Balances {
		out[balance.AssetSymbol] = balance.Equity
	}
	return out
}

func value(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func classOf(err error) string {
	class, _ := adapter.Classify(err)
	return class
}

func TestReplayUnifiedWallet(t *testing.T) {
	r := newReplay(t, "", nil)
	ctx := context.Background()
	result, err := r.adapter.Validate(ctx, testCreds, adapter.StructureKrakenUnified)
	if err != nil || result.ExternalRef == "" || result.MarginMode != "" {
		t.Fatalf("validate: %+v %v", result, err)
	}
	batch, err := r.adapter.Collect(ctx, testCreds, adapter.StructureKrakenUnified, adapter.AccountTypeIDs(adapter.StructureKrakenUnified))
	if err != nil {
		t.Fatal(err)
	}
	if batch.MarginMode != "" || len(batch.Reports) != 4 {
		t.Fatalf("batch: margin mode %q, %d reports", batch.MarginMode, len(batch.Reports))
	}
	reports := reportsByType(t, &batch)

	mainReport := reports[TypeMain]
	if value(mainReport.Equity) != "1000" || value(mainReport.AdjustedEquity) != "950" || value(mainReport.AvailableEquity) != "800" ||
		mainReport.ReportedInitialMarginRatio != nil || mainReport.ReportedMaintenanceMarginRatio != nil || value(mainReport.InitialMarginRequirementUSD) != "100" || value(mainReport.MaintenanceMarginRequirementUSD) != "10" || mainReport.ReportedMarginRatio != nil {
		t.Fatalf("Unified main figures: %s %s %s %s %s", value(mainReport.Equity), value(mainReport.AdjustedEquity), value(mainReport.AvailableEquity), value(mainReport.ReportedInitialMarginRatio), value(mainReport.ReportedMaintenanceMarginRatio))
	}
	if got := balances(&mainReport); len(got) != 2 || got["USD"] != "500" || got["BTC"] != "0.01" {
		t.Fatalf("Unified main balances: %v", got)
	}
	for _, balance := range mainReport.Balances {
		if balance.AvailableToTrade == nil {
			t.Fatalf("Unified main balance %s: available %v", balance.AssetSymbol, balance.AvailableToTrade)
		}
	}
	if len(mainReport.Positions) != 1 {
		t.Fatalf("Unified main positions: %+v", mainReport.Positions)
	}
	position := mainReport.Positions[0]
	if position.PositionKey != "PF_XBTUSD:long" || position.Side != adapter.SideLong || position.SourceQuantity != "1" ||
		position.SourceQuantityUnit != adapter.UnitContracts || position.FuturesContract.Kind != adapter.KindLinearPerpetual ||
		position.FuturesContract.BaseAssetSymbol != "BTC" || position.FuturesContract.SettlementAssetSymbol != "USD" || value(position.FuturesContract.ContractSize) != "1" ||
		value(position.EntryPrice) != "570" || value(position.MarkPrice) != "30209.9" || position.UnrealizedPnl != nil {
		t.Fatalf("Unified main position: %+v", position)
	}

	if earn := reports[TypeEarn]; len(earn.Balances) != 1 || balances(&earn)["USDT"] != "20" {
		t.Fatalf("Earn: %v", balances(&earn))
	}
	if holding := reports[TypeDerivativesHolding]; len(holding.Balances) != 1 || balances(&holding)["BTC"] != "0.01" {
		t.Fatalf("Derivatives holding: %v", balances(&holding))
	}
	if coin := reports[TypeCoinM]; value(coin.Equity) != "10" || balances(&coin)["BTC"] != "0.02" || len(coin.Positions) != 2 {
		t.Fatalf("Coin-M: %+v", coin)
	}
	if len(batch.Prices) != 1 || batch.Prices[0].CanonicalAssetSymbol != "BTC" || batch.Prices[0].QuoteCanonicalAssetSymbol != "USD" || batch.Prices[0].Price != "30000" {
		t.Fatalf("prices: %+v", batch.Prices)
	}
	for i := range batch.Reports {
		for _, price := range batch.Prices {
			if price.RetrievedAt.After(batch.Reports[i].CollectedAt) {
				t.Fatalf("the price of %s is later than the %s report it values", price.CanonicalAssetSymbol, batch.Reports[i].AccountType)
			}
		}
	}
	if r.reached(epBalanceEx.String()) || r.reached(epTradeBalance.String()) {
		t.Fatal("Unified main read the separate-wallet main endpoints")
	}
}

// The separate-wallet responses are documentation-derived (ADR 0009): they
// prove the field mapping of the synthetic shapes, not live access.
func TestReplaySeparateWallets(t *testing.T) {
	r := newReplay(t, separateScenario, nil)
	ctx := context.Background()
	if _, err := r.adapter.Validate(ctx, testCreds, adapter.StructureKrakenSeparate); err != nil {
		t.Fatalf("validate: %v", err)
	}
	batch, err := r.adapter.Collect(ctx, testCreds, adapter.StructureKrakenSeparate, adapter.AccountTypeIDs(adapter.StructureKrakenSeparate))
	if err != nil {
		t.Fatal(err)
	}
	reports := reportsByType(t, &batch)
	if len(reports) != 5 {
		t.Fatalf("%d reports", len(reports))
	}

	// Main: ZUSD and its Kraken Rewards row ZUSD.F merge; the USDT.B allocation belongs to earn;
	// the equity is the priced coins plus the open Spot margin result n = -10.0232.
	mainReport := reports[TypeMain]
	if got := balances(&mainReport); len(got) != 2 || got["USD"] != "25445.21" || got["BTC"] != "1.2435" {
		t.Fatalf("Separate main balances: %v", got)
	}
	if mainReport.Equity != nil || value(mainReport.SpotMarginUnrealizedPnlUSD) != "-10.0232" || mainReport.AdjustedEquity != nil {
		t.Fatalf("Separate main equity: %s", value(mainReport.Equity))
	}

	multi := reports[TypeMultiM]
	if value(multi.Equity) != "5000" || value(multi.AdjustedEquity) != "5000" || multi.ReportedInitialMarginRatio != nil || value(multi.InitialMarginRequirementUSD) != "0" || balances(&multi)["USD"] != "5000" {
		t.Fatalf("Multi-M: %+v", multi)
	}
	if len(multi.Positions) != 1 || multi.Positions[0].PositionKey != "PF_XBTUSD:long" || multi.Positions[0].FuturesContract.BaseAssetSymbol != "BTC" ||
		multi.Positions[0].UnrealizedPnl != nil || value(multi.Positions[0].MarkPrice) != "30209.9" {
		t.Fatalf("Multi-M positions (a BTC-denominated PnL is not the USD settlement): %+v", multi.Positions)
	}

	coin := reports[TypeCoinM]
	if value(coin.Equity) != "3683.7011" || len(coin.Balances) != 1 || balances(&coin)["BTC"] != "0.1219368845" {
		t.Fatalf("Coin-M: equity %s, balances %v", value(coin.Equity), balances(&coin))
	}
	positions := map[string]adapter.Position{}
	for _, p := range coin.Positions {
		positions[p.PositionKey] = p
	}
	perpetual, delivery := positions["PI_XBTUSD:short"], positions["FI_XBTUSD_220930:long"]
	if len(positions) != 2 || perpetual.Side != adapter.SideShort || perpetual.SourceQuantity != "10000" || perpetual.FuturesContract.Kind != adapter.KindInversePerpetual ||
		perpetual.FuturesContract.SettlementAssetSymbol != "BTC" || value(perpetual.FuturesContract.ContractSize) != "1" || perpetual.UnrealizedPnl != nil {
		t.Fatalf("inverse perpetual: %+v", perpetual)
	}
	if delivery.FuturesContract.Kind != adapter.KindInverseDelivery || delivery.FuturesContract.ExpiryAt == nil ||
		!delivery.FuturesContract.ExpiryAt.Equal(time.Date(2022, 9, 30, 15, 0, 0, 0, time.UTC)) || value(delivery.MarkPrice) != "20478.5" {
		t.Fatalf("inverse delivery: %+v", delivery)
	}

	holding := reports[TypeDerivativesHolding]
	if got := balances(&holding); len(got) != 10 || got["BTC"] != "0.1285407184" || got["USD"] != "5000" {
		t.Fatalf("Derivatives holding: %v", got)
	}
	if earn := reports[TypeEarn]; balances(&earn)["USDT"] != "20" {
		t.Fatal("Separate earn lost its allocations")
	}
}

func TestStructureMismatchStopsCollection(t *testing.T) {
	cases := []struct {
		name       string
		scenario   string
		configured adapter.AccountStructure
		snapshot   string
		class      string
		detail     string
	}{
		{"Unified account, separate identity", "", adapter.StructureKrakenSeparate, "", adapter.ClassUnsupported, "unified_balances = true"},
		{"Separate account, unified identity", separateScenario, adapter.StructureKrakenUnified, "", adapter.ClassUnsupported, "unified_balances = false"},
		{"missing flag", "", adapter.StructureKrakenUnified, `{"feed":"balances_snapshot","flex_futures":{}}`, adapter.ClassNormalization, "unified_balances"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			overrides := map[string]string{}
			if tc.snapshot != "" {
				overrides[epBalancesSnapshot.String()] = tc.snapshot
			}
			r := newReplay(t, tc.scenario, overrides)
			_, err := r.adapter.Collect(context.Background(), testCreds, tc.configured, []adapter.AccountType{TypeMain, TypeEarn})
			class, detail := adapter.Classify(err)
			if class != tc.class || !strings.Contains(detail, tc.detail) {
				t.Fatalf("collect: %v", err)
			}
			for _, endpoint := range accountEndpoints {
				if r.reached(endpoint) {
					t.Fatalf("read %s after the structure check failed", endpoint)
				}
			}
			if _, err := r.adapter.Validate(context.Background(), testCreds, tc.configured); classOf(err) != tc.class {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

func TestUnsupportedStructureAndAccountType(t *testing.T) {
	r := newReplay(t, "", nil)
	ctx := context.Background()
	if _, err := r.adapter.Validate(ctx, testCreds, adapter.StructureOKXUnified); classOf(err) != adapter.ClassUnsupported {
		t.Fatalf("another platform's structure: %v", err)
	}
	if _, err := r.adapter.Collect(ctx, testCreds, adapter.StructureBinancePM, []adapter.AccountType{TypeMain}); classOf(err) != adapter.ClassUnsupported {
		t.Fatalf("another platform's structure: %v", err)
	}
	batch, err := r.adapter.Collect(ctx, testCreds, adapter.StructureKrakenUnified, []adapter.AccountType{TypeMultiM, TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Reports[0].Status != adapter.StatusFailed || batch.Reports[0].ErrorClass != adapter.ClassUnsupported || batch.Reports[1].Status != adapter.StatusSucceeded {
		t.Fatalf("Multi-M under unified wallet must fail alone: %+v", batch.Reports)
	}
}

func TestEarnFailsOnUnclassifiedAllocations(t *testing.T) {
	spotOnly := adapter.Credentials{APIKey: testCreds.APIKey, APISecret: testCreds.APISecret}
	cases := map[string]map[string]string{
		"unknown strategy": {
			epEarnAllocations.String(): `{"error":[],"result":{"items":[{"strategy_id":"unknown","native_asset":"USD","amount_allocated":{"total":{"native":"10"}}}],"next_cursor":null}}`,
		},
		"undocumented lock type": {
			epEarnStrategies.String():  `{"error":[],"result":{"items":[{"id":"S1","asset":"DOT","lock_type":{"type":"mystery"}}],"next_cursor":null}}`,
			epEarnAllocations.String(): `{"error":[],"result":{"items":[{"strategy_id":"S1","native_asset":"DOT","amount_allocated":{"total":{"native":"1"}}}],"next_cursor":null}}`,
		},
		"paged allocations": {
			epEarnAllocations.String(): `{"error":[],"result":{"items":[],"next_cursor":"2"}}`,
		},
		"missing amount in historical allocation": {
			epEarnAllocations.String(): `{"error":[],"result":{"items":[{"strategy_id":"unknown","native_asset":"USD","amount_allocated":{"total":{}}}],"next_cursor":null}}`,
		},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, "", overrides)
			batch, err := r.adapter.Collect(context.Background(), spotOnly, adapter.StructureKrakenUnified, []adapter.AccountType{TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			if batch.Reports[0].Status != adapter.StatusFailed || batch.Reports[0].ErrorClass != adapter.ClassNormalization {
				t.Fatalf("unclassified earn holdings were accepted: %+v", batch.Reports[0])
			}
			if r.reached(epBalancesSnapshot.String()) {
				t.Fatal("Earn alone read the balances feed")
			}
		})
	}
}

func TestEarnIgnoresZeroHistoricalAllocations(t *testing.T) {
	for _, structure := range []adapter.AccountStructure{adapter.StructureKrakenSeparate, adapter.StructureKrakenUnified} {
		t.Run(string(structure), func(t *testing.T) {
			r := newReplay(t, "", map[string]string{
				epEarnStrategies.String():  `{"error":[],"result":{"items":[{"id":"S1","asset":"DOT","lock_type":{"type":"bonded"}}],"next_cursor":null}}`,
				epEarnAllocations.String(): `{"error":[],"result":{"items":[{"strategy_id":"retired","native_asset":"USD","amount_allocated":{"total":{"native":"0"}}},{"strategy_id":"S1","native_asset":"DOT","amount_allocated":{"total":{"native":"1"}}}],"next_cursor":null}}`,
			})
			spotOnly := adapter.Credentials{APIKey: testCreds.APIKey, APISecret: testCreds.APISecret}
			batch, err := r.adapter.Collect(context.Background(), spotOnly, structure, []adapter.AccountType{TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			report := batch.Reports[0]
			if report.Status != adapter.StatusSucceeded || len(report.Balances) != 1 || report.Balances[0].AssetSymbol != "DOT" || report.Balances[0].Equity != "1" {
				t.Fatalf("historical allocation changed current earn holdings: %+v", report)
			}
			if r.reached(epBalancesSnapshot.String()) {
				t.Fatal("Earn alone read the balances feed")
			}
		})
	}
}

func TestSeparateMainRules(t *testing.T) {
	ctx := context.Background()
	collectMain := func(t *testing.T, overrides map[string]string) adapter.AccountReport {
		t.Helper()
		r := newReplay(t, separateScenario, overrides)
		batch, err := r.adapter.Collect(ctx, testCreds, adapter.StructureKrakenSeparate, []adapter.AccountType{TypeMain, TypeEarn})
		if err != nil {
			t.Fatal(err)
		}
		if batch.Reports[1].Status != adapter.StatusSucceeded {
			t.Fatal("a main failure spread to earn")
		}
		return batch.Reports[0]
	}
	unpriced := `{"error":[],"result":{"ZUSD":{"balance":"100.0000"},"XTZ":{"balance":"3.00000000"}}}`

	used := collectMain(t, map[string]string{epBalanceEx.String(): `{"error":[],"result":{"ZUSD":{"balance":"100","credit":"50","credit_used":"5"}}}`})
	if used.Status != adapter.StatusFailed || used.ErrorClass != adapter.ClassUnsupported {
		t.Fatalf("used credit: %+v", used)
	}
	margin := collectMain(t, map[string]string{epBalanceEx.String(): unpriced})
	if margin.Status != adapter.StatusSucceeded || margin.Equity != nil || margin.SpotMarginUnrealizedPnlUSD == nil || balances(&margin)["XTZ"] != "3" {
		t.Fatalf("a margin result with an unpriced coin: %+v", margin)
	}
	flat := collectMain(t, map[string]string{epBalanceEx.String(): unpriced, epTradeBalance.String(): `{"error":[],"result":{"n":"0.0000"}}`})
	if flat.Status != adapter.StatusSucceeded || flat.Equity != nil || balances(&flat)["XTZ"] != "3" {
		t.Fatalf("without a margin result the value is calculated from balances: %+v", flat)
	}
}

func TestKeyChecks(t *testing.T) {
	ctx := context.Background()
	r := newReplay(t, "", map[string]string{
		epAPIKeyInfo.String(): `{"error":[],"result":{"iban":"AA11 BBBB CCCC DDDD"}}`,
		epFuturesKey.String(): `{"iiban":"ZZ99BBBBCCCCDDDD","accountUid":"u-1"}`,
	})
	if _, err := r.adapter.Validate(ctx, testCreds, adapter.StructureKrakenUnified); classOf(err) != adapter.ClassAuthentication {
		t.Fatalf("keys of different customers: %v", err)
	}
	r = newReplay(t, "", map[string]string{
		epAPIKeyInfo.String(): `{"error":[],"result":{"iban":"AA11 BBBB CCCC DDDD"}}`,
		epFuturesKey.String(): `{"iiban":"aa11bbbbccccdddd"}`,
	})
	result, err := r.adapter.Validate(ctx, testCreds, adapter.StructureKrakenUnified)
	if err != nil || result.ExternalRef != "AA11BBBBCCCCDDDD" {
		t.Fatalf("one customer printed two ways: %+v %v", result, err)
	}
	spotOnly := adapter.Credentials{APIKey: testCreds.APIKey, APISecret: testCreds.APISecret}
	if _, err := r.adapter.Collect(ctx, spotOnly, adapter.StructureKrakenUnified, []adapter.AccountType{TypeMain}); classOf(err) != adapter.ClassAuthentication {
		t.Fatalf("Main without the futures key that checks the structure: %v", err)
	}

	// A missing canonical customer reference cannot establish a login binding.
	futuresOnly := adapter.Credentials{Futures: testCreds.Futures}
	for check, want := range map[string]string{
		`{"iiban":"aa11 bbbb cccc dddd","accountUid":"u-1"}`: "AA11BBBBCCCCDDDD",
		`{"iiban":null,"accountUid":"u-1"}`:                  "",
		`{"iiban":null,"accountUid":null}`:                   "",
	} {
		r := newReplay(t, "", map[string]string{epFuturesKey.String(): check})
		result, err := r.adapter.Validate(ctx, futuresOnly, adapter.StructureKrakenUnified)
		if want == "" && classOf(err) != adapter.ClassNormalization || want != "" && (err != nil || result.ExternalRef != want) {
			t.Fatalf("Futures-only key check %s: %+v %v", check, result, err)
		}
	}
}

func TestOptionPositionFailsTheMultiCollateralWallet(t *testing.T) {
	r := newReplay(t, "", map[string]string{
		epOpenPositions.String(): `{"result":"success","openPositions":[{"symbol":"OF_XBTUSD_260925_50000_P","side":"long","size":1,"price":10}]}`,
		epInstruments.String():   `{"result":"success","instruments":[{"symbol":"OF_XBTUSD_260925_50000_P","type":"options","pair":"BTC:USD","base":"BTC","quote":"USD","contractSize":1}]}`,
	})
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapter.StructureKrakenUnified, []adapter.AccountType{TypeMain, TypeCoinM})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Reports[0].Status != adapter.StatusFailed || batch.Reports[0].ErrorClass != adapter.ClassNormalization || batch.Reports[1].Status != adapter.StatusSucceeded {
		t.Fatalf("an option must fail main and leave Coin-M: %+v", batch.Reports)
	}
}

func TestMapPositionKeepsPnlOnlyInTheSettlementCurrency(t *testing.T) {
	cases := []struct {
		name     string
		position openPosition
		contract nativeInstrument
		pnl      string
	}{
		{"flexible defaults to USD", openPosition{Symbol: "PF_XBTUSD", Side: "long", Size: "2", UnrealizedPnl: "5"}, nativeInstrument{Symbol: "PF_XBTUSD", Type: typeFlexible, Pair: "XBT:USD", ContractSize: "1"}, "5"},
		{"flexible in USDT", openPosition{Symbol: "PF_XBTUSD", Side: "long", Size: "2", UnrealizedPnl: "5", PnlCurrency: "USDT"}, nativeInstrument{Symbol: "PF_XBTUSD", Type: typeFlexible, Pair: "XBT:USD", ContractSize: "1"}, "5"},
		{"flexible in BTC", openPosition{Symbol: "PF_XBTUSD", Side: "long", Size: "2", UnrealizedPnl: "5", PnlCurrency: "BTC"}, nativeInstrument{Symbol: "PF_XBTUSD", Type: typeFlexible, Pair: "XBT:USD", ContractSize: "1"}, "<nil>"},
		{"inverse in XBT", openPosition{Symbol: "PI_XBTUSD", Side: "short", Size: "100", UnrealizedPnl: "0.01", PnlCurrency: "XBT"}, nativeInstrument{Symbol: "PI_XBTUSD", Type: typeInverse, Base: "XBT", Quote: "USD", ContractSize: "1"}, "0.01"},
		{"inverse without currency", openPosition{Symbol: "PI_XBTUSD", Side: "short", Size: "100", UnrealizedPnl: "0.01"}, nativeInstrument{Symbol: "PI_XBTUSD", Type: typeInverse, Base: "XBT", Quote: "USD", ContractSize: "1"}, "<nil>"},
	}
	for _, tc := range cases {
		p, err := mapPosition(&tc.position, &tc.contract, &futuresTicker{Tag: "perpetual"}, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if p.FuturesContract.BaseAssetSymbol != "BTC" || value(p.UnrealizedPnl) != tc.pnl {
			t.Fatalf("%s: base %s, pnl %s", tc.name, p.FuturesContract.BaseAssetSymbol, value(p.UnrealizedPnl))
		}
	}
	dated := nativeInstrument{Symbol: "FI_XBTUSD_X", Type: typeInverse, Base: "XBT", Quote: "USD", ContractSize: "1"}
	if _, err := mapPosition(&openPosition{Symbol: "FI_XBTUSD_X", Side: "long", Size: "1"}, &dated, &futuresTicker{Tag: "quarter"}, nil); classOf(err) != adapter.ClassNormalization {
		t.Fatalf("a dated contract without an expiry: %v", err)
	}
}

func TestNumberReadsEveryKrakenAmountForm(t *testing.T) {
	var got struct {
		Float    number `json:"float"`
		String   number `json:"string"`
		Exponent number `json:"exponent"`
		Null     number `json:"null"`
		Garbage  number `json:"garbage"`
	}
	raw := `{"float":134.87264532567443,"string":"0.00000","exponent":-4.680761641037005e-06,"null":null,"garbage":"n/a"}`
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Float != "134.87264532567443" || got.String != "0" || got.Exponent != "-0.000004680761641037005" || got.Null != "" || got.Garbage != "n/a" {
		t.Fatalf("decoded %+v", got)
	}
	if _, err := adapter.Decimal("test", "garbage", string(got.Garbage)); err == nil {
		t.Fatal("a malformed amount must fail where it is read")
	}
}

// Every synthetic error answer maps to its class, and the detail names only the
// endpoint, the status, and the matched error name, never Kraken's text. Every one of them is a
// synthetic name, so none falls back to the generic detail.
func TestErrorAnswersClassify(t *testing.T) {
	files := loadFixtures(t, "")
	for _, refused := range refusals(testCreds) {
		if _, ok := files.failures[failureKey(refused.ep.String(), refused.scenario)]; !ok {
			t.Errorf("no %s answer for %s", refused.scenario, refused.ep)
		}
	}
	for key, f := range files.failures {
		endpoint, scenario := key[:strings.LastIndex(key, " ")], key[strings.LastIndex(key, " ")+1:]
		var out json.RawMessage
		class, detail := adapter.Classify(decode(endpointNamed(t, endpoint), response{status: f.Status, body: f.Body}, &out))
		if class != errorClasses[scenario] || !strings.HasPrefix(detail, "Kraken "+endpoint+": HTTP ") || strings.ContainsAny(detail, "{[\"") ||
			strings.HasSuffix(detail, "Platform request failed.") {
			t.Errorf("%s: %s %q", key, class, detail)
		}
	}
}

// Without a synthetic error name, the HTTP status decides, and an unreadable success fails as
// normalization.
func TestHTTPStatusDecidesWithoutAnErrorName(t *testing.T) {
	cases := []struct {
		ep     endpoint
		status int
		body   string
		class  string
	}{
		{epBalanceEx, http.StatusTooManyRequests, `{}`, adapter.ClassRateLimited},
		{epBalanceEx, http.StatusForbidden, `{"error":[]}`, adapter.ClassAuthorization},
		{epBalanceEx, http.StatusOK, `{"error":["EGeneral:Internal error"]}`, adapter.ClassTemporary},
		{epOpenPositions, http.StatusUnauthorized, `{"result":"success"}`, adapter.ClassAuthentication},
		{epOpenPositions, http.StatusServiceUnavailable, `<html>`, adapter.ClassTemporary},
		{epOpenPositions, http.StatusOK, `not json`, adapter.ClassNormalization},
	}
	for _, tc := range cases {
		var out json.RawMessage
		if class := classOf(decode(tc.ep, response{status: tc.status, body: []byte(tc.body)}, &out)); class != tc.class {
			t.Errorf("%s %d %s: %s", tc.ep, tc.status, tc.body, class)
		}
	}
}

// A key Kraken refuses fails validation with authentication, which stops scheduling.
func TestRefusedCredentialsStopScheduling(t *testing.T) {
	cases := map[string]adapter.Credentials{
		"unknown Spot key":     {APIKey: refusedKey, APISecret: testCreds.APISecret, Futures: testCreds.Futures},
		"wrong Spot secret":    {APIKey: testCreds.APIKey, APISecret: refusedSecret, Futures: testCreds.Futures},
		"unknown Futures key":  {APIKey: testCreds.APIKey, APISecret: testCreds.APISecret, Futures: &adapter.KeyPair{APIKey: refusedKey, APISecret: testCreds.FuturesSecret()}},
		"wrong Futures secret": {APIKey: testCreds.APIKey, APISecret: testCreds.APISecret, Futures: &adapter.KeyPair{APIKey: testCreds.FuturesKey(), APISecret: refusedSecret}},
	}
	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, "", nil)
			r.refuse()
			_, err := r.adapter.Validate(context.Background(), creds, adapter.StructureKrakenUnified)
			if class := classOf(err); class != adapter.ClassAuthentication || !adapter.StopsScheduling(class) {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

// A rate limit fails only its account; a refused permission or authentication fails the batch.
func TestDocumentedErrorScope(t *testing.T) {
	cases := []struct {
		ep       endpoint
		scenario string
		failed   adapter.AccountType
		batch    string
	}{
		{epEarnAllocations, "rate_limited", TypeEarn, ""},
		{epEarnAllocations, "permission_denied", "", adapter.ClassAuthorization},
		{epOpenPositions, "api_limit_exceeded", TypeMain, ""},
		{epOpenPositions, "authentication_error", "", adapter.ClassAuthentication},
	}
	files := loadFixtures(t, "")
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			f := files.failures[failureKey(tc.ep.String(), tc.scenario)]
			if f.Status != http.StatusOK {
				t.Fatalf("the synthetic %s answer has status %d", tc.scenario, f.Status)
			}
			r := newReplay(t, "", map[string]string{tc.ep.String(): string(f.Body)})
			batch, err := r.adapter.Collect(context.Background(), testCreds, adapter.StructureKrakenUnified, []adapter.AccountType{TypeMain, TypeEarn})
			if tc.batch != "" {
				if classOf(err) != tc.batch {
					t.Fatalf("collect: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for i := range batch.Reports {
				report := &batch.Reports[i]
				if failed := report.AccountType == tc.failed; failed != (report.Status == adapter.StatusFailed) || failed && report.ErrorClass != errorClasses[tc.scenario] {
					t.Fatalf("%s: %s %s", report.AccountType, report.Status, report.ErrorClass)
				}
			}
		})
	}
}
