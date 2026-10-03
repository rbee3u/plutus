package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/offline"
	"github.com/rbee3u/plutus/internal/decimal"
)

var testCreds = adapters.Credentials{APIKey: "fixture-key", APISecret: "fixture-secret"}

var testNow = time.Date(2026, 9, 25, 19, 55, 34, 0, time.UTC)

func allTypes() []adapters.AccountType {
	return adapters.AccountTypeIDs(adapters.StructureBybitUTA2)
}

// replay serves explicitly selected flat interface samples and verifies native authentication.
type replay struct {
	t        *testing.T
	adapter  *Adapter
	files    map[string]fixture.File // answers by request
	failures map[string]fixture.File // synthetic refusals by scenario
	answers  map[string]string       // invented answers by path, or by path and query
	// serverMillis is Bybit's synthetic clock. The frozen test clock corrected by it matches it.
	serverMillis int64

	mu      sync.Mutex
	seen    []string
	fixed   map[string]fixture.File   // answers to every call, by path
	queue   map[string][]fixture.File // answers to the next calls, by path
	lenient bool
}

// newReplay starts a replay server. selection chooses the explicitly listed flat files; answers maps a path, or a path with
// its query, to an invented answer that takes precedence over both.
func newReplay(t *testing.T, scenario string, answers map[string]string) *replay {
	t.Helper()
	selected, err := fixture.Load("testdata", sampleNames(scenario)...)
	if err != nil {
		t.Fatal(err)
	}
	files, failures := map[string]fixture.File{}, map[string]fixture.File{}
	for _, f := range selected {
		files[f.Request.Target()] = f
	}
	for _, variant := range []string{scenarioInvalidKey, scenarioBadSignature, scenarioStaleTimestamp} {
		f, err := fixture.ReadSynthetic("testdata/v5_account_info." + variant + ".json")
		if err != nil {
			t.Fatal(err)
		}
		failures[variant] = f
	}
	var clock struct {
		Time int64 `json:"time"`
	}
	if err := json.Unmarshal(files[epTime.request(nil)].Body, &clock); err != nil || clock.Time <= 0 {
		t.Fatalf("the synthetic server time has no clock: %v", err)
	}
	r := &replay{
		t: t, files: files, failures: failures, answers: answers, serverMillis: clock.Time,
		fixed: map[string]fixture.File{}, queue: map[string][]fixture.File{},
	}
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		for request, steps := range r.queue {
			if len(steps) != 0 {
				t.Errorf("%s: %d expected response steps not consumed", request, len(steps))
			}
		}
	})
	r.adapter = New(&Options{Host: server.URL, HTTP: offline.Client(t, server), Now: func() time.Time { return testNow }})
	return r
}

func (r *replay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	r.mu.Lock()
	r.seen = append(r.seen, path)
	lenient := r.lenient
	var override *fixture.File
	if queued := r.queue[path]; len(queued) > 0 {
		override, r.queue[path] = &queued[0], queued[1:]
	} else if f, ok := r.fixed[path]; ok {
		override = &f
	}
	r.mu.Unlock()
	if req.Method != http.MethodGet {
		r.t.Errorf("%s %s: the adapter may only read", req.Method, path)
	}
	if !strings.HasPrefix(path, "/v5/market/") {
		if scenario, msg := r.checkSigned(req); msg != "" {
			if !lenient {
				r.t.Errorf("%s: %s", path, msg)
			}
			serve(w, r.failures[scenario])
			return
		}
	}
	if override != nil {
		serve(w, *override)
		return
	}
	for _, key := range []string{req.URL.RequestURI(), path} {
		if body, ok := r.answers[key]; ok {
			serve(w, fixture.File{Status: http.StatusOK, Body: json.RawMessage(body)})
			return
		}
	}
	if f, ok := r.files[req.URL.RequestURI()]; ok {
		serve(w, f)
		return
	}
	r.t.Errorf("no fixture for %s", req.URL.RequestURI())
	w.WriteHeader(http.StatusInternalServerError)
}

// checkSigned returns the refusal a signed request earns and why, or empty strings. Bybit accepts
// a timestamp from the receive window before its clock to one second after it.
func (r *replay) checkSigned(req *http.Request) (scenario, msg string) {
	key, timestamp, window := req.Header.Get("X-Bapi-Api-Key"), req.Header.Get("X-Bapi-Timestamp"), req.Header.Get("X-Bapi-Recv-Window")
	if key != testCreds.APIKey {
		return scenarioInvalidKey, "missing or wrong API key header"
	}
	mac := hmac.New(sha256.New, []byte(testCreds.APISecret))
	_, _ = mac.Write([]byte(timestamp + key + window + req.URL.RawQuery))
	if window != recvWindowMillis || req.Header.Get("X-Bapi-Sign") != hex.EncodeToString(mac.Sum(nil)) {
		return scenarioBadSignature, "signature does not match"
	}
	millis, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || millis < r.serverMillis-5000 || millis >= r.serverMillis+1000 {
		return scenarioStaleTimestamp, "timestamp is not aligned with the synthetic server time"
	}
	return "", ""
}

func serve(w http.ResponseWriter, f fixture.File) {
	body := f.Body
	if f.BodyFormat == "text" {
		var text string
		if err := json.Unmarshal(body, &text); err != nil {
			panic(err) // All file-backed inputs were validated before starting the replay server.
		}
		body = []byte(text)
		w.Header().Set("Content-Type", "text/plain")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(f.Status)
	_, _ = w.Write(body)
}

// failure is the synthetic refusal of a scenario.
func (r *replay) failure(scenario string) fixture.File {
	r.t.Helper()
	f, ok := r.failures[scenario]
	if !ok {
		r.t.Fatalf("no %s refusal", scenario)
	}
	return f
}

// always answers every call to path with f.
func (r *replay) always(path string, f fixture.File) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fixed[path] = f
}

// once answers the next call to path with f, before any other answer.
func (r *replay) once(path string, f fixture.File) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue[path] = append(r.queue[path], f)
}

func (r *replay) paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.seen...)
}

// count returns how often path was requested.
func (r *replay) count(path string) int {
	n := 0
	for _, seen := range r.paths() {
		if seen == path {
			n++
		}
	}
	return n
}

// retMsg reads the platform message from a synthetic response.
func retMsg(f fixture.File) string {
	var env struct {
		Msg string `json:"retMsg"`
	}
	_ = json.Unmarshal(f.Body, &env)
	return env.Msg
}

// sampleBody decodes the result of one synthetic fixture, so tests compare mappings with the
// synthetic values instead of copies of them.
func sampleBody(t *testing.T, name string, out any) {
	t.Helper()
	f, err := fixture.ReadSynthetic(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(f.Body, &env); err != nil || json.Unmarshal(env.Result, out) != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func reportOf(t *testing.T, batch *adapters.Batch, accountType adapters.AccountType) adapters.AccountReport {
	t.Helper()
	for _, report := range batch.Reports {
		if report.AccountType == accountType {
			return report
		}
	}
	t.Fatalf("no %s report", accountType)
	return adapters.AccountReport{}
}

// nonzero reports whether any of the decimal fields is nonzero; an empty field counts as zero.
func nonzero(values ...string) bool {
	for _, value := range values {
		if rat, ok := decimal.Parse(value); ok && rat.Sign() != 0 {
			return true
		}
	}
	return false
}

func text(value *string) string {
	if value == nil {
		return "<nil>"
	}
	return *value
}

func TestDocumentedPortfolioMarginAccount(t *testing.T) {
	r := newReplay(t, "", nil)
	ctx := context.Background()
	result, err := r.adapter.Validate(ctx, testCreds, adapters.StructureBybitUTA2)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExternalRef != "100001" || result.MarginMode != adapters.MarginBybitPM {
		t.Fatalf("validation = %+v", result)
	}
	batch, err := r.adapter.Collect(ctx, testCreds, adapters.StructureBybitUTA2, allTypes())
	if err != nil {
		t.Fatal(err)
	}
	if batch.MarginMode != adapters.MarginBybitPM || len(batch.Reports) != len(allTypes()) {
		t.Fatalf("batch Margin Mode %q with %d reports", batch.MarginMode, len(batch.Reports))
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
		}
		for _, balance := range report.Balances {
			if balance.Name != "" {
				t.Fatalf("%s %s: Bybit reports no full name", report.AccountType, balance.Symbol)
			}
		}
		for _, price := range batch.Prices {
			if price.ObservedAt.After(report.ObservedAt) {
				t.Fatal("a price must be observed before the reports it values")
			}
		}
	}

	var wallet struct {
		List []walletAccount `json:"list"`
	}
	sampleBody(t, "v5_account_wallet_balance.pm", &wallet)
	synthetic := wallet.List[0]
	unified := reportOf(t, &batch, TypeUnified)
	if text(unified.Equity) != decimal.Compact(synthetic.TotalEquity) || text(unified.AvailableEquity) != decimal.Compact(synthetic.TotalAvailableBalance) ||
		text(unified.InitialMarginRatio) != decimal.Compact(synthetic.AccountIMRate) || text(unified.MaintenanceMarginRatio) != decimal.Compact(synthetic.AccountMMRate) ||
		unified.AdjustedEquity != nil || unified.UnifiedMarginRatio != nil {
		t.Fatalf("Portfolio Margin account figures = %+v", unified)
	}
	for _, coin := range synthetic.Coins {
		found := false
		for _, balance := range unified.Balances {
			if balance.Symbol == coin.Coin {
				found = true
				if balance.Equity != decimal.Format(decimal.MustParse(coin.Equity)) || text(balance.UnrealizedPnl) != decimal.Compact(coin.UnrealisedPnl) ||
					text(balance.Liability) != decimal.Compact(coin.BorrowAmount) || text(balance.Interest) != decimal.Compact(coin.AccruedInterest) {
					t.Fatalf("%s balance = %+v", coin.Coin, balance)
				}
			}
		}
		if found != nonzero(coin.Equity, coin.BorrowAmount, coin.AccruedInterest, coin.UnrealisedPnl) {
			t.Fatalf("%s: exactly the coins with a nonzero figure are kept", coin.Coin)
		}
	}
	if len(unified.Positions) == 0 {
		t.Fatal("the synthetic linear position is missing")
	}
	for i := range unified.Positions {
		position := &unified.Positions[i]
		if position.Leverage != nil || position.LiquidationPrice != nil || position.Margin != nil {
			t.Fatalf("%s: a Portfolio Margin position has no leverage, liquidation price, or margin of its own", position.PlatformPositionRef)
		}
		if position.EntryPrice == nil || position.MarkPrice == nil ||
			(position.Instrument.Kind == adapters.KindLinearPerp && position.QuantityUnit != adapters.UnitBase) ||
			(position.Instrument.Kind == adapters.KindInversePerp && position.QuantityUnit != adapters.UnitQuote) {
			t.Fatalf("linear position = %+v", position)
		}
	}

	var funding struct {
		Balance []fundingCoin `json:"balance"`
	}
	sampleBody(t, "v5_asset_transfer_query_account_coins_balance", &funding)
	fundingReport := reportOf(t, &batch, TypeFunding)
	held := 0
	for _, coin := range funding.Balance {
		if decimal.MustParse(coin.WalletBalance).Sign() != 0 {
			held++
		}
	}
	if len(fundingReport.Balances) != held || fundingReport.Equity != nil {
		t.Fatalf("Funding keeps every nonzero coin and is calculated from its Balances: %+v", fundingReport)
	}

	principal := adapters.Totals{}
	for _, name := range []string{"v5_earn_position", "v5_earn_fixed_term_position.empty"} {
		var savings struct {
			List []savingsPosition `json:"list"`
		}
		sampleBody(t, name, &savings)
		for _, position := range savings.List {
			if err := principal.Add("test", position.Coin, decimal.MustParse(position.Amount)); err != nil {
				t.Fatal(err)
			}
		}
	}
	earn := reportOf(t, &batch, TypeEarn)
	want := principal.Balances()
	if len(earn.Balances) != len(want) {
		t.Fatalf("Earn = %+v, want the summed principal %+v without claimable yield", earn.Balances, want)
	}
	for i := range want {
		if earn.Balances[i].Symbol != want[i].Symbol || earn.Balances[i].Equity != want[i].Equity {
			t.Fatalf("Earn = %+v, want the summed principal %+v without claimable yield", earn.Balances, want)
		}
	}
}

func TestIsolatedMarginIgnoresAccountWideFigures(t *testing.T) {
	r := newReplay(t, "bybit_isolated", nil)
	ctx := context.Background()
	result, err := r.adapter.Validate(ctx, testCreds, adapters.StructureBybitUTA2)
	if err != nil || result.MarginMode != adapters.MarginBybitIsolated {
		t.Fatalf("validation = %+v, %v", result, err)
	}
	batch, err := r.adapter.Collect(ctx, testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeUnified})
	if err != nil {
		t.Fatal(err)
	}
	unified := reportOf(t, &batch, TypeUnified)
	if batch.MarginMode != adapters.MarginBybitIsolated || unified.Status != adapters.StatusOK {
		t.Fatalf("batch = %+v", batch)
	}
	if unified.Equity != nil || unified.AvailableEquity != nil || unified.InitialMarginRatio != nil || unified.MaintenanceMarginRatio != nil {
		t.Fatal("Bybit documents the account-wide figures as not applicable to Isolated Margin")
	}
	if len(unified.Balances) != 1 || unified.Balances[0].Symbol != "BTC" || unified.Balances[0].Equity != "0.00990587" ||
		text(unified.Balances[0].UnrealizedPnl) != "-0.00029413" || text(unified.Balances[0].Liability) != "0" {
		t.Fatalf("balances = %+v", unified.Balances)
	}
	if len(unified.Positions) != 1 {
		t.Fatalf("positions = %+v", unified.Positions)
	}
	position := unified.Positions[0]
	if position.PlatformPositionRef != "BTCUSD:0" || position.Side != adapters.SideShort || position.Quantity != "300" ||
		position.QuantityUnit != adapters.UnitQuote || position.Instrument.Kind != adapters.KindInversePerp ||
		position.Instrument.SettlementSymbol != "BTC" || text(position.EntryPrice) != "27464.50441675" || text(position.MarkPrice) != "28224.5" ||
		text(position.LiquidationPrice) != "30191.5" || text(position.Leverage) != "10" || text(position.Margin) != "0.00010923" ||
		text(position.UnrealizedPnl) != "-0.00029413" {
		t.Fatalf("isolated position = %+v", position)
	}
}

func TestCrossMarginMapsAccountFigures(t *testing.T) {
	r := newReplay(t, "bybit_cross", nil)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeUnified})
	if err != nil {
		t.Fatal(err)
	}
	unified := reportOf(t, &batch, TypeUnified)
	if batch.MarginMode != adapters.MarginBybitCross || unified.Status != adapters.StatusOK {
		t.Fatalf("batch = %+v", batch)
	}
	if text(unified.Equity) != "3.31216591" || text(unified.AvailableEquity) != "3.00326056" ||
		text(unified.InitialMarginRatio) != "0" || text(unified.MaintenanceMarginRatio) != "0" || unified.AdjustedEquity != nil {
		t.Fatalf("Cross Margin account figures = %+v", unified)
	}
	if len(unified.Balances) != 0 {
		t.Fatalf("a coin whose figures are all zero is dropped: %+v", unified.Balances)
	}
	if len(unified.Positions) != 1 {
		t.Fatalf("positions = %+v", unified.Positions)
	}
	position := unified.Positions[0]
	if position.LiquidationPrice != nil || text(position.Leverage) != "10" || text(position.Margin) != "0.00010923" {
		t.Fatalf("an empty liqPrice stays unknown; leverage and position IM map: %+v", position)
	}
}

func TestOtherAccountStructuresStopEveryRead(t *testing.T) {
	r := newReplay(t, "uta_1_0_pro", nil)
	ctx := context.Background()
	_, err := r.adapter.Validate(ctx, testCreds, adapters.StructureBybitUTA2)
	if class, detail := adapters.Classify(err); class != adapters.ClassUnsupported || !strings.Contains(detail, "Unified Trading Account 1.0 Pro (unifiedMarginStatus 4)") {
		t.Fatalf("validation error = %v", err)
	}
	_, err = r.adapter.Collect(ctx, testCreds, adapters.StructureBybitUTA2, allTypes())
	if class, _ := adapters.Classify(err); class != adapters.ClassUnsupported {
		t.Fatalf("collection error = %v", err)
	}
	for _, path := range r.paths() {
		if path != epTime.path && path != epAccountInfo.path {
			t.Fatalf("read %s after the structure check failed", path)
		}
	}

	other := newReplay(t, "", nil)
	if _, err := other.adapter.Validate(ctx, testCreds, adapters.StructureOKXUnified); err == nil {
		t.Fatal("another Platform's structure must be refused")
	}
	if _, err := other.adapter.Collect(ctx, testCreds, adapters.StructureKrakenUnified, allTypes()); err == nil {
		t.Fatal("another Platform's structure must be refused")
	}
	if len(other.paths()) != 0 {
		t.Fatalf("another Platform's structure must be refused before any request: %v", other.paths())
	}
}

func TestMarginModeMustBeKnown(t *testing.T) {
	for _, tc := range []struct {
		name, answer, class, detail string
	}{
		{"unknown mode", `{"retCode":0,"result":{"unifiedMarginStatus":6,"marginMode":"FUTURE_MARGIN"}}`, adapters.ClassUnsupported, "FUTURE_MARGIN"},
		{"empty mode", `{"retCode":0,"result":{"unifiedMarginStatus":6,"marginMode":""}}`, adapters.ClassNormalization, "marginMode"},
		{"missing status", `{"retCode":0,"result":{"marginMode":"PORTFOLIO_MARGIN"}}`, adapters.ClassNormalization, "unifiedMarginStatus"},
		{"classic account", `{"retCode":0,"result":{"unifiedMarginStatus":1,"marginMode":"REGULAR_MARGIN"}}`, adapters.ClassUnsupported, "a classic account (unifiedMarginStatus 1)"},
		{"undocumented status", `{"retCode":0,"result":{"unifiedMarginStatus":9,"marginMode":"PORTFOLIO_MARGIN"}}`, adapters.ClassUnsupported, "undocumented"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReplay(t, "", map[string]string{epAccountInfo.path: tc.answer})
			_, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, allTypes())
			if class, detail := adapters.Classify(err); class != tc.class || !strings.Contains(detail, tc.detail) {
				t.Fatalf("error = %v, want %s naming %s", err, tc.class, tc.detail)
			}
		})
	}
}

// earnAnswers are invented Earn answers, so a test controls every product.
func earnAnswers(flexible, overviewUSD string) map[string]string {
	return map[string]string{
		epFlexible.path:      `{"retCode":0,"result":{"list":[` + flexible + `]}}`,
		epFixedTerm.path:     `{"retCode":0,"result":{"list":[]}}`,
		epDualAssets.path:    `{"retCode":0,"result":{"list":[]}}`,
		epAssetOverview.path: `{"retCode":0,"result":{"list":[{"accountType":"Earn","totalEquity":"` + overviewUSD + `","valuationCurrency":"USD"}]}}`,
	}
}

func TestDualAssetPrincipalCountsInTheInvestedCoin(t *testing.T) {
	r := newReplay(t, "dual_investment", nil)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 1 || earn.Balances[0].Symbol != "USDT" || earn.Balances[0].Equity != "20" {
		t.Fatalf("Earn = %+v: only the synthetic 20 USDT Dual Asset principal", earn)
	}
}

func TestDualAssetPagesAndStates(t *testing.T) {
	first := epDualAssets.request(nil)
	second := epDualAssets.request(map[string][]string{"cursor": {"page2"}})
	page := func(position, cursor string) string {
		return `{"retCode":0,"result":{"category":"DualAssets","list":[` + position + `],"nextPageCursor":"` + cursor + `"}}`
	}
	buyLow := `{"positionId":"1","category":"DualAssets","investCoin":"USDT","amount":"100","direction":"BuyLow","status":"Active"}`
	redeeming := `{"positionId":"2","category":"DualAssets","investCoin":"USDC","amount":"30","direction":"BuyLow","status":"Redeeming"}`

	answers := earnAnswers("", "130")
	answers[first] = page(redeeming, "page2")
	answers[second] = page(buyLow, "")
	r := newReplay(t, "", answers)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 2 || earn.Balances[0].Symbol != "USDC" || earn.Balances[0].Equity != "30" ||
		earn.Balances[1].Symbol != "USDT" || earn.Balances[1].Equity != "100" {
		t.Fatalf("Earn = %+v: both pages, Active and Redeeming", earn)
	}

	for name, pages := range map[string][2]string{
		"repeated cursor": {page(buyLow, "page2"), page(buyLow, "page2")},
		"unknown state":   {page(strings.Replace(buyLow, "Active", "Settled", 1), ""), page("", "")},
		"missing coin":    {page(strings.Replace(buyLow, `"USDT"`, `""`, 1), ""), page("", "")},
	} {
		t.Run(name, func(t *testing.T) {
			answers := earnAnswers("", "0")
			answers[first], answers[second] = pages[0], pages[1]
			r := newReplay(t, "", answers)
			batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeFunding, TypeDualInvestment})
			if err != nil {
				t.Fatal(err)
			}
			if reportOf(t, &batch, TypeDualInvestment).ErrorClass != adapters.ClassNormalization || reportOf(t, &batch, TypeFunding).Status != adapters.StatusOK {
				t.Fatalf("only Earn fails with normalization: %+v", batch.Reports)
			}
		})
	}
}

func TestFixedTermPrincipalIncludesEarlyRedemption(t *testing.T) {
	r := newReplay(t, "fixed_saving", nil)
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeEarn)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 1 || earn.Balances[0].Equity != "301" {
		t.Fatalf("Earn = %+v: the synthetic 20 USDT of savings plus 201 active and 80 redeeming", earn)
	}
	for _, path := range r.paths() {
		if path == epWallet.path || path == epPositions.path {
			t.Fatal("an unselected Account was collected")
		}
	}
}

func TestEarnCheckKeepsUnreadProductsStale(t *testing.T) {
	savings := `{"coin":"USDT","amount":"20","claimableYield":"0.5"}`
	r := newReplay(t, "", earnAnswers(savings, "500"))
	batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, allTypes())
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, &batch, TypeEarn)
	if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassUnsupported ||
		!strings.Contains(earn.ErrorDetail, "500.00") || !strings.Contains(earn.ErrorDetail, "20.00") {
		t.Fatalf("Earn = %+v: Bybit's valuation exceeds the itemized principal", earn)
	}
	if reportOf(t, &batch, TypeUnified).Status != adapters.StatusOK || reportOf(t, &batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatal("the Earn check fails only Earn")
	}

	for name, answers := range map[string]map[string]string{
		"within tolerance": earnAnswers(savings, "20.9"),
		"unpriced coin":    earnAnswers(`{"coin":"NOPRICE","amount":"5"}`, "500"),
		"no Earn bucket":   {epFlexible.path: `{"retCode":0,"result":{"list":[` + savings + `]}}`, epFixedTerm.path: `{"retCode":0,"result":{"list":[]}}`, epAssetOverview.path: `{"retCode":0,"result":{"list":[]}}`},
	} {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, "", answers)
			batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			if earn := reportOf(t, &batch, TypeEarn); earn.Status != adapters.StatusOK {
				t.Fatalf("Earn = %+v", earn)
			}
		})
	}
}

func TestMalformedAccountsFailOnlyThemselves(t *testing.T) {
	for name, answers := range map[string]map[string]string{
		"funding without balances":  {epFunding.path: `{"retCode":0,"result":{"accountType":"FUND"}}`},
		"funding without a balance": {epFunding.path: `{"retCode":0,"result":{"accountType":"FUND","balance":[{"coin":"USDT","walletBalance":""}]}}`},
		"repeated position cursor":  {epPositions.path: `{"retCode":0,"result":{"list":[],"nextPageCursor":"repeat"}}`},
		"open option position":      {epPositions.request(map[string][]string{"category": {"option"}}): `{"retCode":0,"result":{"list":[{"symbol":"BTC-26DEC26-100000-C","side":"Buy","size":"1"}],"nextPageCursor":""}}`},
		"unlisted contract":         {epPositions.request(map[string][]string{"category": {"inverse"}}): `{"retCode":0,"result":{"list":[{"symbol":"NOPEUSD","side":"Buy","size":"1"}],"nextPageCursor":""}}`},
		"savings with a cursor":     {epFlexible.path: `{"retCode":0,"result":{"list":[],"nextPageCursor":"more"}}`},
	} {
		t.Run(name, func(t *testing.T) {
			r := newReplay(t, "", answers)
			batch, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, allTypes())
			if err != nil {
				t.Fatal(err)
			}
			failed := 0
			for _, report := range batch.Reports {
				if report.Status == adapters.StatusFailed {
					failed++
					if report.ErrorClass != adapters.ClassNormalization || len(report.Balances) != 0 {
						t.Fatalf("%s: %+v", report.AccountType, report)
					}
				}
			}
			wantFailed := 1
			if name == "savings with a cursor" {
				wantFailed = 2
			}
			if failed != wantFailed {
				t.Fatalf("the malformed Account and any dependent Earn/Dual report fail: %+v", batch.Reports)
			}
		})
	}
}

func TestAuthenticationFailureStopsTheBatch(t *testing.T) {
	r := newReplay(t, "", nil)
	r.always(epWallet.path, r.failure(scenarioInvalidKey))
	_, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, allTypes())
	if class, _ := adapters.Classify(err); class != adapters.ClassAuthentication {
		t.Fatalf("error = %v: an authentication failure of one Account fails the whole batch", err)
	}
}

func TestEveryRetryableFailureFailsTheBatch(t *testing.T) {
	r := newReplay(t, "rate_limited", nil)
	_, err := r.adapter.Collect(context.Background(), testCreds, adapters.StructureBybitUTA2, []adapters.AccountType{TypeUnified, TypeFunding})
	if class, _ := adapters.Classify(err); class != adapters.ClassRateLimited {
		t.Fatalf("error = %v: the collection service retries a run whose every Account was rate limited", err)
	}
}

// TestErrorClasses replays, on the call every run starts with, the refusals Bybit gave the
// synthetic key and the synthetic ones no recording can provoke.
func TestErrorClasses(t *testing.T) {
	for _, tc := range []struct {
		name, failure, scenario, want string
	}{
		{"unknown key", scenarioInvalidKey, "", adapters.ClassAuthentication},
		{"wrong signature", scenarioBadSignature, "", adapters.ClassAuthentication},
		{"timestamp refused again after one resync", scenarioStaleTimestamp, "", adapters.ClassTemporary},
		{"region not served", "", "region_restricted", adapters.ClassUnsupported},
		{"IP over the request budget", "", "ip_rate_limited", adapters.ClassTemporary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newReplay(t, tc.scenario, nil)
			answer := r.files[epAccountInfo.request(nil)]
			if tc.failure != "" {
				answer = r.failure(tc.failure)
				r.always(epAccountInfo.path, answer)
			}
			ctx := context.Background()
			_, err := r.adapter.Validate(ctx, testCreds, adapters.StructureBybitUTA2)
			class, detail := adapters.Classify(err)
			if class != tc.want || !strings.HasPrefix(detail, epAccountInfo.String()+": HTTP ") {
				t.Fatalf("validation error = %v, want %s naming the endpoint and status", err, tc.want)
			}
			if msg := retMsg(answer); msg != "" && strings.Contains(detail, msg) {
				t.Fatalf("detail %q repeats Bybit's message, which can echo the request", detail)
			}
			_, err = r.adapter.Collect(ctx, testCreds, adapters.StructureBybitUTA2, allTypes())
			if class, _ := adapters.Classify(err); class != tc.want {
				t.Fatalf("collection error = %v, want %s for the whole batch", err, tc.want)
			}
			for _, path := range r.paths() {
				if path != epTime.path && path != epAccountInfo.path {
					t.Fatalf("read %s after Bybit refused the first call", path)
				}
			}
		})
	}
}

// TestErrorCodesWithoutAnAnswer classifies the synthetic codes that neither a recording nor a
// synthetic answer shows.
func TestErrorCodesWithoutAnAnswer(t *testing.T) {
	for _, tc := range []struct {
		status, code int
		want         string
	}{
		{http.StatusOK, 10007, adapters.ClassAuthentication},
		{http.StatusOK, 33004, adapters.ClassAuthentication},
		{http.StatusOK, -2015, adapters.ClassAuthentication},
		{http.StatusUnauthorized, 0, adapters.ClassAuthentication},
		{http.StatusOK, 10005, adapters.ClassAuthorization},
		{http.StatusOK, 10010, adapters.ClassAuthorization},
		{http.StatusOK, 10018, adapters.ClassRateLimited},
		{http.StatusTooManyRequests, 0, adapters.ClassRateLimited},
		{http.StatusOK, 10028, adapters.ClassUnsupported},
		{http.StatusOK, 100028, adapters.ClassUnsupported},
		{http.StatusOK, 180004, adapters.ClassUnsupported},
		{http.StatusOK, 180024, adapters.ClassUnsupported},
		{http.StatusOK, 10016, adapters.ClassTemporary},
		{http.StatusBadGateway, 0, adapters.ClassTemporary},
	} {
		if got := classOf(tc.status, tc.code); got != tc.want {
			t.Errorf("HTTP %d, code %d: class %s, want %s", tc.status, tc.code, got, tc.want)
		}
	}
}

func TestClockResynchronizesOnceAfterATimestampRefusal(t *testing.T) {
	r := newReplay(t, "", nil)
	r.once(epAccountInfo.path, r.failure(scenarioStaleTimestamp))
	result, err := r.adapter.Validate(context.Background(), testCreds, adapters.StructureBybitUTA2)
	if err != nil || result.MarginMode != adapters.MarginBybitPM {
		t.Fatalf("validation = %+v, %v", result, err)
	}
	if r.count(epTime.path) != 2 || r.count(epAccountInfo.path) != 2 {
		t.Fatalf("server time %d, account info %d calls: sync, refusal, resync, retry", r.count(epTime.path), r.count(epAccountInfo.path))
	}
}

func TestPositionMapping(t *testing.T) {
	delivery := instrument{Symbol: "BTCUSDZ26", ContractType: "InverseFutures", BaseCoin: "BTC", QuoteCoin: "USD", SettleCoin: "BTC", DeliveryTime: "1798761600000"}
	index := 2
	row := openPosition{Symbol: delivery.Symbol, PositionIdx: &index, Side: "Sell", Size: "300", AvgPrice: "30000", MarkPrice: "29000", Leverage: "5", PositionIM: "0.002", LiqPrice: "0"}
	position, err := mapPosition(&row, big.NewRat(300, 1), &delivery, adapters.MarginBybitCross)
	if err != nil {
		t.Fatal(err)
	}
	if position.QuantityUnit != adapters.UnitQuote || position.Instrument.Kind != adapters.KindInverseDlv || position.Instrument.ExpiryAt == nil ||
		position.Side != adapters.SideShort || position.PlatformPositionRef != "BTCUSDZ26:2" || position.LiquidationPrice != nil || text(position.Leverage) != "5" {
		t.Fatalf("inverse delivery hedge position = %+v", position)
	}

	pm, err := mapPosition(&row, big.NewRat(300, 1), &delivery, adapters.MarginBybitPM)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Leverage != nil || pm.Margin != nil || pm.LiquidationPrice != nil {
		t.Fatal("a Portfolio Margin position has no leverage, margin, or liquidation price of its own")
	}

	for name, broken := range map[string]func(*openPosition, *instrument){
		"side":          func(p *openPosition, _ *instrument) { p.Side = "" },
		"price":         func(p *openPosition, _ *instrument) { p.MarkPrice = "n/a" },
		"contract type": func(_ *openPosition, i *instrument) { i.ContractType = "InverseOption" },
		"delivery time": func(_ *openPosition, i *instrument) { i.DeliveryTime = "0" },
	} {
		p, i := row, delivery
		broken(&p, &i)
		if _, err := mapPosition(&p, big.NewRat(1, 1), &i, adapters.MarginBybitCross); err == nil {
			t.Fatalf("a malformed %s must fail the position", name)
		}
	}
}
