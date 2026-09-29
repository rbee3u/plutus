package binance

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/fs"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/decimal"
)

const (
	testKey    = "test-api-key"
	testSecret = "test-api-secret"
)

var testCreds = adapters.Credentials{APIKey: testKey, APISecret: testSecret}

var testEdCreds = adapters.Credentials{APIKey: testKey, APISecret: testEdPEM()}

func testEdPEM() string {
	key := ed25519.NewKeyFromSeed([]byte("plutus-binance-ed25519-test-seed"))
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))
}

var (
	pmTypes      = adapters.AccountTypeIDs(adapters.StructureBinancePM)
	classicTypes = adapters.AccountTypeIDs(adapters.StructureBinanceClassic)
)

// Documentation-derived scenarios under testdata/documented/ (ADR 0027).
const (
	docClassic        = "binance_classic"
	docDualInvestment = "dual_investment"
)

// endpoints lists every endpoint the adapter calls; a fixture for any other is a mistake.
var endpoints = []endpoint{
	epServerTime, epTickerPrice, epSpotAccount, epAccountInfo, epUserAsset, epFundingAsset, epWalletValue,
	epFlexibleEarn, epLockedEarn, epDualInvest, epPMAccount, epPMBalance, epPMUMPositionRisk, epPMCMPositionRisk,
	epCrossMargin, epIsolatedMargin, epUMAccount, epUMPositionRisk, epUMMultiAssets, epUMSymbolConfig,
	epCMAccount, epCMPositionRisk, epUMExchangeInfo, epCMExchangeInfo,
}

func knownEndpoint(name string) (endpoint, bool) {
	for _, ep := range endpoints {
		if ep.String() == name {
			return ep, true
		}
	}
	return endpoint{}, false
}

// replay serves the committed fixtures: the recorded Portfolio Margin account in testdata/, with
// the documentation-derived files of one testdata/documented/<scenario>/ directory laid over it.
// It verifies the API key header, the signature, and that timestamps follow the recorded
// server time, so signing and clock sync are exercised.
type replay struct {
	t          *testing.T
	answers    map[string]fixture.File // successful answers by request, as fixture headers name it
	failures   map[string]fixture.File // recorded error answers by scenario
	serverTime int64
	started    time.Time

	mu    sync.Mutex
	calls map[string]int                           // by request
	queue map[string][]func(w http.ResponseWriter) // by endpoint
	fixed map[string]func(w http.ResponseWriter)   // by request, or by endpoint for every query
	// lenient answers a request with a bad key, signature, or timestamp with the matching error
	// fixture instead of failing the test; the recorder provokes exactly those.
	lenient bool
	secret  string
}

func newReplay(t *testing.T, overlay string) (*replay, *Adapter) {
	t.Helper()
	r := &replay{
		t: t, answers: map[string]fixture.File{}, failures: map[string]fixture.File{}, started: time.Now(),
		calls: map[string]int{}, queue: map[string][]func(http.ResponseWriter){}, fixed: map[string]func(http.ResponseWriter){},
		secret: testSecret,
	}
	for _, path := range fixturePaths(t, "testdata") {
		f := readFixture(t, path)
		if f.Fixture.Scenario == fixture.ScenarioRecorded {
			r.answers[f.Fixture.Endpoint] = f
		} else {
			r.failures[f.Fixture.Scenario] = f
		}
	}
	if overlay != "" {
		for _, path := range fixturePaths(t, filepath.Join("testdata", "documented", overlay)) {
			f := readFixture(t, path)
			r.answers[f.Fixture.Endpoint] = f
		}
	}
	var st struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal(r.answer(epServerTime, nil).Body, &st); err != nil {
		t.Fatal(err)
	}
	r.serverTime = st.ServerTime
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	hosts := Hosts{API: server.URL, PAPI: server.URL, FAPI: server.URL, DAPI: server.URL}
	return r, New(&Options{Hosts: hosts, HTTP: server.Client()})
}

// earnUSD values coin amounts with the recorded tickers, as Binance values its Earn wallet.
func (r *replay) earnUSD(amounts adapters.Totals) string {
	r.t.Helper()
	var rows []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.Unmarshal(r.answer(epTickerPrice, nil).Body, &rows); err != nil {
		r.t.Fatal(err)
	}
	prices := lastPrices{}
	for _, row := range rows {
		prices[row.Symbol] = row.Price
	}
	sum := new(big.Rat)
	for symbol, amount := range amounts {
		unit, ok := prices.usd(symbol)
		if !ok {
			r.t.Fatalf("no recorded price for %s", symbol)
		}
		sum.Add(sum, new(big.Rat).Mul(amount, unit))
	}
	return sum.FloatString(8)
}

func fixturePaths(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures in %s: %v", dir, err)
	}
	return paths
}

func readFixture(t *testing.T, path string) fixture.File {
	t.Helper()
	f, err := fixture.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// requestOf names a call as fixture headers do: its method and target.
func requestOf(ep endpoint, params url.Values) string {
	return ep.method + " " + target(ep, params)
}

// answer returns the fixture that answers one call.
func (r *replay) answer(ep endpoint, params url.Values) fixture.File {
	r.t.Helper()
	f, ok := r.answers[requestOf(ep, params)]
	if !ok {
		r.t.Fatalf("no fixture for %s", requestOf(ep, params))
	}
	return f
}

func (r *replay) failure(scenario string) fixture.File {
	r.t.Helper()
	f, ok := r.failures[scenario]
	if !ok {
		r.t.Fatalf("no %s fixture", scenario)
	}
	return f
}

// tolerateErrors makes the replay answer badly signed requests with the recorded error fixtures.
func (r *replay) tolerateErrors() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lenient = true
}

// then queues one-shot answers for an endpoint, served before the default fixture.
func (r *replay) then(ep endpoint, answers ...func(w http.ResponseWriter)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue[ep.String()] = append(r.queue[ep.String()], answers...)
}

// always replaces an endpoint's default answer for every query.
func (r *replay) always(ep endpoint, answer func(w http.ResponseWriter)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fixed[ep.String()] = answer
}

// alwaysAt replaces the default answer to one query of an endpoint.
func (r *replay) alwaysAt(ep endpoint, params url.Values, answer func(w http.ResponseWriter)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fixed[requestOf(ep, params)] = answer
}

// count returns how often an endpoint was called, with any query.
func (r *replay) count(ep endpoint) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for request, calls := range r.calls {
		if path, _, _ := strings.Cut(request, "?"); path == ep.String() {
			n += calls
		}
	}
	return n
}

// called lists the requests made so far.
func (r *replay) called() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	for name, n := range r.calls {
		if n > 0 {
			out[name] = true
		}
	}
	return out
}

func serve(f fixture.File) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { write(w, f.Status, f.Body) }
}

func status(code int, body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { write(w, code, []byte(body)) }
}

func write(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func (r *replay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := req.Method + " " + req.URL.Path
	request := name + unsignedQuery(req)
	r.mu.Lock()
	r.calls[request]++
	answer, lenient := r.fixed[request], r.lenient
	if answer == nil {
		answer = r.fixed[name]
	}
	if queue := r.queue[name]; len(queue) > 0 {
		answer, r.queue[name] = queue[0], queue[1:]
	}
	r.mu.Unlock()
	if ep, ok := knownEndpoint(name); ok && ep.signed {
		if scenario, msg := r.checkSigned(req); msg != "" {
			if !lenient {
				r.t.Errorf("%s: %s", name, msg)
			}
			serve(r.failures[scenario])(w)
			return
		}
	}
	if answer != nil {
		answer(w)
		return
	}
	f, ok := r.answers[request]
	if !ok {
		r.t.Errorf("unexpected request %s", request)
		write(w, http.StatusNotFound, []byte(`{"code":-1100,"msg":"no fixture"}`))
		return
	}
	serve(f)(w)
}

// unsignedQuery is a request's query without the signing parameters, with its "?", as the
// adapter encodes it before signing.
func unsignedQuery(req *http.Request) string {
	query := req.URL.Query()
	for _, signing := range []string{"timestamp", "recvWindow", "signature"} {
		query.Del(signing)
	}
	if len(query) == 0 {
		return ""
	}
	return "?" + query.Encode()
}

// checkSigned returns the error scenario a signed request falls into and why, or empty strings.
func (r *replay) checkSigned(req *http.Request) (scenario, msg string) {
	if req.Header.Get("X-Mbx-Apikey") != testKey {
		return "invalid_key", "missing or wrong X-MBX-APIKEY header"
	}
	raw := req.URL.RawQuery
	idx := strings.LastIndex(raw, "&signature=")
	if idx < 0 {
		return "bad_signature", "signature is not the last parameter"
	}
	key, ed, err := adapters.BinancePrivateKey(r.secret)
	if err != nil {
		return "bad_signature", "test signing key is invalid"
	}
	if ed {
		signature, err := base64.StdEncoding.DecodeString(req.URL.Query().Get("signature"))
		if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), []byte(raw[:idx]), signature) ||
			raw[idx+len("&signature="):] != url.QueryEscape(base64.StdEncoding.EncodeToString(signature)) {
			return "bad_signature", "Ed25519 signature does not match"
		}
	} else {
		mac := hmac.New(sha256.New, []byte(r.secret))
		_, _ = mac.Write([]byte(raw[:idx]))
		if raw[idx+len("&signature="):] != hex.EncodeToString(mac.Sum(nil)) {
			return "bad_signature", "HMAC signature does not match"
		}
	}
	if req.URL.Query().Get("recvWindow") != recvWindowMillis {
		return "bad_signature", "recvWindow is not " + recvWindowMillis
	}
	ts, err := strconv.ParseInt(req.URL.Query().Get("timestamp"), 10, 64)
	if err != nil {
		return "stale_timestamp", "timestamp is not a number"
	}
	expected := r.serverTime + time.Since(r.started).Milliseconds()
	if diff := ts - expected; diff > 5000 || diff < -5000 {
		return "stale_timestamp", "timestamp is not aligned with the recorded server time: off by " + strconv.FormatInt(diff, 10) + " ms"
	}
	return "", ""
}

func body[T any](t *testing.T, r *replay, ep endpoint, params url.Values) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(r.answer(ep, params).Body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func rat(t *testing.T, value string) *big.Rat {
	t.Helper()
	out, ok := decimal.Parse(value)
	if !ok {
		t.Fatalf("not a decimal: %q", value)
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func reportOf(t *testing.T, batch adapters.Batch, accountType adapters.AccountType) adapters.AccountReport {
	t.Helper()
	for _, report := range batch.Reports {
		if report.AccountType == accountType {
			return report
		}
	}
	t.Fatalf("no %s report in %+v", accountType, batch.Reports)
	return adapters.AccountReport{}
}

func balanceOf(t *testing.T, report adapters.AccountReport, symbol string) adapters.Balance {
	t.Helper()
	for _, b := range report.Balances {
		if b.Symbol == symbol {
			return b
		}
	}
	t.Fatalf("no %s balance in %s: %+v", symbol, report.AccountType, report.Balances)
	return adapters.Balance{}
}

func classOfErr(t *testing.T, err error) string {
	t.Helper()
	var adapterErr *adapters.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("error %v is not an adapter error", err)
	}
	return adapterErr.Class
}

func collect(t *testing.T, a *Adapter, structure adapters.AccountStructure, accounts ...adapters.AccountType) adapters.Batch {
	t.Helper()
	batch, err := a.Collect(context.Background(), testCreds, structure, accounts)
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestFixtureHeaders(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir("testdata", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(file) != ".json" {
			return err
		}
		f := readFixture(t, file)
		h := &f.Fixture
		if h.Platform != "binance" || h.Endpoint == "" {
			t.Errorf("%s: incomplete header %+v", file, h)
		}
		method, request, _ := strings.Cut(h.Endpoint, " ")
		path, _, _ := strings.Cut(request, "?")
		if _, ok := knownEndpoint(method + " " + path); !ok {
			t.Errorf("%s: the adapter never calls %s", file, h.Endpoint)
		}
		suffix := h.Scenario
		if suffix == fixture.ScenarioRecorded || h.Documented() {
			suffix = ""
		}
		if want := fixture.Name(request, suffix) + ".json"; filepath.Base(file) != want {
			t.Errorf("%s: the fixture of %s is named %s", file, h.Endpoint, want)
		}
		if h.MarginMode != "" {
			t.Errorf("%s: Binance has no Margin Mode", file)
		}
		documented := filepath.Base(filepath.Dir(filepath.Dir(file))) == "documented"
		switch {
		case documented:
			if !h.Documented() || h.RecordedAt != nil || !strings.HasPrefix(h.Documentation, "https://developers.binance.com/") ||
				!adapters.ValidStructure(adapters.PlatformBinance, adapters.AccountStructure(h.AccountStructure)) {
				t.Errorf("%s: a documentation-derived fixture names its documentation and no recording date: %+v", file, h)
			}
		default:
			if h.Documented() || h.RecordedAt == nil || h.Documentation != "" || h.AccountStructure != string(adapters.StructureBinancePM) {
				t.Errorf("%s: a recorded fixture names its recording date and the recorded structure: %+v", file, h)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFixturesHaveNoCredentialOrIdentityEcho(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path) //nolint:gosec // committed fixture
		if err != nil {
			return err
		}
		text := string(raw)
		for _, forbidden := range []string{"apiKey", "secret", "signature=", "X-MBX-APIKEY"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s contains %q", path, forbidden)
			}
		}
		if ipv4.MatchString(strings.ReplaceAll(text, RedactedIP, "")) {
			t.Errorf("%s contains an IPv4 address other than the redaction placeholder", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := newReplay(t, "")
	var spot struct {
		UID int64 `json:"uid"`
	}
	if err := json.Unmarshal(r.answer(epSpotAccount, spotAccountQuery()).Body, &spot); err != nil || spot.UID != RedactedUID {
		t.Fatalf("api/v3/account uid %d is not the redaction placeholder: %v", spot.UID, err)
	}
}

func TestValidateReturnsTheUIDInBothStructures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		structure adapters.AccountStructure
		overlay   string
	}{{adapters.StructureBinancePM, ""}, {adapters.StructureBinanceClassic, docClassic}} {
		t.Run(string(tc.structure), func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, tc.overlay)
			result, err := a.Validate(context.Background(), testCreds, tc.structure)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExternalRef != strconv.Itoa(RedactedUID) || result.MarginMode != "" {
				t.Fatalf("result %+v", result)
			}
			if r.count(epServerTime) != 1 || r.count(epAccountInfo) != 1 {
				t.Fatalf("clock synced %d times and structure read %d times, want 1 each", r.count(epServerTime), r.count(epAccountInfo))
			}
		})
	}
}

// Synthetic Ed25519 credentials reuse the existing response fixtures. This exercises signing for
// every signed endpoint in both structures; it does not establish live Ed25519 access.
func TestEd25519ValidateAndCollectBothStructures(t *testing.T) {
	for _, tc := range []struct {
		structure adapters.AccountStructure
		overlay   string
	}{{adapters.StructureBinancePM, ""}, {adapters.StructureBinanceClassic, docClassic}} {
		t.Run(string(tc.structure), func(t *testing.T) {
			r, a := newReplay(t, tc.overlay)
			r.secret = testEdCreds.APISecret
			if _, err := a.Validate(context.Background(), testEdCreds, tc.structure); err != nil {
				t.Fatal(err)
			}
			batch, err := a.Collect(context.Background(), testEdCreds, tc.structure, adapters.AccountTypeIDs(tc.structure))
			if err != nil {
				t.Fatal(err)
			}
			if len(batch.Reports) != len(adapters.AccountTypeIDs(tc.structure)) {
				t.Fatalf("got %d Account Reports", len(batch.Reports))
			}
			if r.count(epAccountInfo) != 2 || r.count(epSpotAccount) != 1 {
				t.Fatalf("account info %d times, spot account %d times", r.count(epAccountInfo), r.count(epSpotAccount))
			}
		})
	}
}

func TestStructureMismatchStopsBeforeAnyAccountRead(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		overlay    string
		configured adapters.AccountStructure
		reported   string
	}{
		{"", adapters.StructureBinanceClassic, "Portfolio Margin"},
		{docClassic, adapters.StructureBinancePM, "Classic Trading"},
	} {
		t.Run(string(tc.configured), func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, tc.overlay)
			_, validateErr := a.Validate(context.Background(), testCreds, tc.configured)
			_, collectErr := a.Collect(context.Background(), testCreds, tc.configured, adapters.AccountTypeIDs(tc.configured))
			for _, err := range []error{validateErr, collectErr} {
				if classOfErr(t, err) != adapters.ClassUnsupported || !strings.Contains(err.Error(), tc.reported) ||
					!strings.Contains(err.Error(), adapters.StructureName(tc.configured)) {
					t.Fatalf("mismatch error %v", err)
				}
			}
			for name := range r.called() {
				if name != epServerTime.String() && name != epAccountInfo.String() {
					t.Fatalf("%s was read after the structure mismatch", name)
				}
			}
		})
	}
}

func TestAnotherPlatformsStructureIsUnsupported(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	_, validateErr := a.Validate(context.Background(), testCreds, adapters.StructureOKXUnified)
	_, collectErr := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeSpot})
	for _, err := range []error{validateErr, collectErr} {
		if classOfErr(t, err) != adapters.ClassUnsupported {
			t.Fatalf("foreign structure error %v", err)
		}
	}
	if len(r.called()) != 0 {
		t.Fatalf("calls %v for a structure Binance does not have", r.called())
	}
}

func TestMissingStructureFlagIsNormalization(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epAccountInfo, status(http.StatusOK, `{"vipLevel":0,"isMarginEnabled":true,"isFutureEnabled":true}`))
	_, err := a.Validate(context.Background(), testCreds, adapters.StructureBinancePM)
	if classOfErr(t, err) != adapters.ClassNormalization || !strings.Contains(err.Error(), "isPortfolioMarginRetailEnabled") {
		t.Fatalf("error %v", err)
	}
}

func TestCollectMapsPortfolioMarginAccount(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin)
	if batch.MarginMode != "" {
		t.Fatalf("Binance has no Margin Mode, got %q", batch.MarginMode)
	}
	report := reportOf(t, batch, TypePortfolioMargin)
	if report.Status != adapters.StatusOK || report.ObservedAt.IsZero() {
		t.Fatalf("report %+v", report)
	}
	account := body[pmAccount](t, r, epPMAccount, nil)
	checks := map[string][2]string{
		"equity":             {deref(report.Equity), decimal.Compact(account.ActualEquity)},
		"adjustedEquity":     {deref(report.AdjustedEquity), decimal.Compact(account.AccountEquity)},
		"availableEquity":    {deref(report.AvailableEquity), decimal.Compact(account.TotalAvailable)},
		"unifiedMarginRatio": {deref(report.UnifiedMarginRatio), decimal.Compact(account.UniMMR)},
		"initialMarginRatio": {deref(report.InitialMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.AccountInitialMargin), rat(t, account.AccountEquity)).FloatString(8))},
		"maintMarginRatio":   {deref(report.MaintenanceMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.AccountMaintMargin), rat(t, account.AccountEquity)).FloatString(8))},
	}
	for name, pair := range checks {
		if pair[0] != pair[1] {
			t.Errorf("%s = %s, want %s", name, pair[0], pair[1])
		}
	}

	rows := body[[]pmBalance](t, r, epPMBalance, nil)
	nonzero := 0
	for i := range rows {
		if rat(t, rows[i].TotalWalletBalance).Sign() != 0 || rat(t, rows[i].UMUnrealizedPNL).Sign() != 0 || rat(t, rows[i].CMUnrealizedPNL).Sign() != 0 {
			nonzero++
		}
	}
	if len(report.Balances) != nonzero {
		t.Fatalf("%d balances, want the %d nonzero rows", len(report.Balances), nonzero)
	}
	sawNegative := false
	for _, b := range report.Balances {
		for i := range rows {
			if rows[i].Asset != b.Symbol {
				continue
			}
			row := &rows[i]
			want := new(big.Rat).Add(rat(t, row.TotalWalletBalance), rat(t, row.UMUnrealizedPNL))
			want.Add(want, rat(t, row.CMUnrealizedPNL)).Sub(want, rat(t, row.CrossMarginBorrowed)).Sub(want, rat(t, row.CrossMarginInterest))
			if rat(t, b.Equity).Cmp(want) != 0 {
				t.Errorf("%s equity %s, want %s", b.Symbol, b.Equity, decimal.Format(want))
			}
			sawNegative = sawNegative || want.Sign() < 0
		}
		if b.AvailableToTrade != nil || b.AvailableToWithdraw != nil || b.Name != "" {
			t.Errorf("%s: availability and name stay unmapped on the PM account: %+v", b.Symbol, b)
		}
	}
	if !sawNegative {
		t.Log("fixture has no negative-equity balance; the COIN-M loss case is not exercised")
	}

	// Σ balance equity × spot price must land within 0.5% of actualEquity, which ties the balance
	// formula to Binance's own account equity.
	prices := map[string]*big.Rat{"USDT": big.NewRat(1, 1), "USDC": big.NewRat(1, 1)}
	for _, p := range batch.Prices {
		prices[p.AssetSymbol] = rat(t, p.Price)
	}
	sum := new(big.Rat)
	for _, b := range report.Balances {
		price, ok := prices[b.Symbol]
		if !ok {
			t.Fatalf("no price for %s in %+v", b.Symbol, batch.Prices)
		}
		sum.Add(sum, new(big.Rat).Mul(rat(t, b.Equity), price))
	}
	actual := rat(t, account.ActualEquity)
	gap, _ := new(big.Rat).Quo(new(big.Rat).Sub(sum, actual), actual).Float64()
	if gap > 0.005 || gap < -0.005 {
		t.Fatalf("Σ balance equity × price differs from actualEquity by %.4f%%", gap*100)
	}
}

func TestCollectMapsPortfolioMarginPositionsAndInstruments(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin)
	report := reportOf(t, batch, TypePortfolioMargin)
	type source struct {
		m    market
		rows []positionRisk
	}
	expected := []source{{marketUM, body[[]positionRisk](t, r, epPMUMPositionRisk, nil)}, {marketCM, body[[]positionRisk](t, r, epPMCMPositionRisk, nil)}}
	want := 0
	for _, s := range expected {
		for _, row := range s.rows {
			if rat(t, row.PositionAmt).Sign() == 0 {
				continue
			}
			want++
			ref := string(s.m) + ":" + row.Symbol + ":" + row.PositionSide
			var got *adapters.Position
			for i := range report.Positions {
				if report.Positions[i].PlatformPositionRef == ref {
					got = &report.Positions[i]
				}
			}
			if got == nil {
				t.Fatalf("no position %s in %+v", ref, report.Positions)
			}
			side := adapters.SideLong
			if rat(t, row.PositionAmt).Sign() < 0 {
				side = adapters.SideShort
			}
			unit, kind := adapters.UnitBase, adapters.KindLinearPerp
			if s.m == marketCM {
				unit, kind = adapters.UnitContracts, adapters.KindInversePerp
			}
			if got.Side != side || got.QuantityUnit != unit || rat(t, got.Quantity).Cmp(new(big.Rat).Abs(rat(t, row.PositionAmt))) != 0 {
				t.Errorf("%s side/unit/quantity %s %s %s", ref, got.Side, got.QuantityUnit, got.Quantity)
			}
			if got.Instrument.Symbol != row.Symbol || got.Instrument.Kind != kind || got.Instrument.SettlementSymbol == "" || got.Instrument.BaseSymbol == "" {
				t.Errorf("%s instrument %+v", ref, got.Instrument)
			}
			if (s.m == marketCM) != (got.Instrument.ContractSize != nil) {
				t.Errorf("%s contract size %v: only COIN-M carries one", ref, got.Instrument.ContractSize)
			}
			if got.Margin != nil || got.MarkPrice == nil || got.EntryPrice == nil || got.Leverage == nil || got.UnrealizedPnl == nil {
				t.Errorf("%s measures %+v", ref, got)
			}
		}
	}
	if len(report.Positions) != want {
		t.Fatalf("%d positions, want %d", len(report.Positions), want)
	}
	if r.count(epUMExchangeInfo) != 1 || r.count(epCMExchangeInfo) != 1 {
		t.Fatalf("exchangeInfo fetched UM %d / CM %d times", r.count(epUMExchangeInfo), r.count(epCMExchangeInfo))
	}
	collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin)
	if r.count(epUMExchangeInfo) != 1 || r.count(epCMExchangeInfo) != 1 {
		t.Fatal("exchangeInfo must be served from the in-process cache within its TTL")
	}
}

func TestContractKinds(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t, "")
	c := a.client(testCreds)
	um, err := fetchInstruments(context.Background(), c, marketUM)
	if err != nil {
		t.Fatal(err)
	}
	cm, err := fetchInstruments(context.Background(), c, marketCM)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, set := range []*instrumentSet{um, cm} {
		for symbol, in := range set.bySymbol {
			kinds[in.Kind]++
			delivery := in.Kind == adapters.KindLinearDeliv || in.Kind == adapters.KindInverseDlv
			if delivery != (in.ExpiryAt != nil) {
				t.Errorf("%s %s expiry %v", symbol, in.Kind, in.ExpiryAt)
			}
			if delivery && !strings.Contains(symbol, "_") {
				t.Errorf("%s is mapped as delivery", symbol)
			}
		}
	}
	for _, kind := range []string{adapters.KindLinearPerp, adapters.KindLinearDeliv, adapters.KindInversePerp, adapters.KindInverseDlv} {
		if kinds[kind] == 0 {
			t.Errorf("fixtures cover no %s contract; re-record with the coverage sample", kind)
		}
	}
	for symbol, in := range um.bySymbol {
		if strings.HasSuffix(symbol, "USDT") && in.Kind != adapters.KindLinearPerp {
			t.Errorf("%s (including TRADIFI_PERPETUAL) should be linear_perpetual, got %s", symbol, in.Kind)
		}
	}
}

func TestCollectMapsWalletsAndEarn(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypeSpot, TypeFunding, TypeEarn)
	for _, tc := range []struct {
		accountType adapters.AccountType
		ep          endpoint
	}{{TypeSpot, epUserAsset}, {TypeFunding, epFundingAsset}} {
		report := reportOf(t, batch, tc.accountType)
		if report.Status != adapters.StatusOK || report.Equity != nil || report.AdjustedEquity != nil || report.UnifiedMarginRatio != nil {
			t.Fatalf("%s report %+v: wallets carry no account-level figures", tc.accountType, report)
		}
		rows := body[[]walletAsset](t, r, tc.ep, nil)
		if len(report.Balances) != len(rows) {
			t.Fatalf("%s: %d balances from %d rows", tc.accountType, len(report.Balances), len(rows))
		}
		for i, b := range report.Balances {
			row := rows[i]
			want := new(big.Rat).Add(rat(t, row.Free), rat(t, row.Locked))
			want.Add(want, rat(t, row.Freeze)).Add(want, rat(t, row.Withdrawing))
			if rat(t, b.Equity).Cmp(want) != 0 || deref(b.AvailableToWithdraw) != decimal.Compact(row.Free) || deref(b.AvailableToTrade) != decimal.Compact(row.Free) {
				t.Errorf("%s %s mapped %+v", tc.accountType, b.Symbol, b)
			}
		}
	}
	// The spot account fixture shows the flexible-earn receipt (LD*) that getUserAsset omits.
	spotAccount := body[struct {
		Balances []struct {
			Asset string `json:"asset"`
		} `json:"balances"`
	}](t, r, epSpotAccount, spotAccountQuery())
	hasReceipt := false
	for _, b := range spotAccount.Balances {
		hasReceipt = hasReceipt || strings.HasPrefix(b.Asset, "LD")
	}
	for _, b := range reportOf(t, batch, TypeSpot).Balances {
		if strings.HasPrefix(b.Symbol, "LD") && b.Symbol != "LDO" {
			t.Errorf("spot balance %s is a Simple Earn receipt", b.Symbol)
		}
	}
	if !hasReceipt {
		t.Log("api/v3/account fixture holds no LD* receipt; the double-count guard is not exercised")
	}
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusOK {
		t.Fatalf("earn report %+v", earn)
	}
	want := adapters.Totals{}
	for _, row := range body[earnPage](t, r, epFlexibleEarn, simpleEarnQuery(1)).Rows {
		if err := want.Add("test", row.Asset, rat(t, row.TotalAmount)); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range body[earnPage](t, r, epLockedEarn, simpleEarnQuery(1)).Rows {
		if err := want.Add("test", row.Asset, rat(t, row.Amount)); err != nil {
			t.Fatal(err)
		}
	}
	wantBalances := want.Balances()
	if len(earn.Balances) != len(wantBalances) {
		t.Fatalf("earn balances %+v, want %+v", earn.Balances, wantBalances)
	}
	for i, b := range earn.Balances {
		if b.Symbol != wantBalances[i].Symbol || rat(t, b.Equity).Cmp(rat(t, wantBalances[i].Equity)) != 0 {
			t.Errorf("earn balance %+v, want %+v", b, wantBalances[i])
		}
	}
	if r.count(epWalletValue) != 1 {
		t.Fatalf("the Earn valuation was read %d times, want once", r.count(epWalletValue))
	}
}

// Inline pages exercise paging and every counted status; the Earn valuation matches them.
func TestEarnSeparatesSimpleEarnAndDualInvestment(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.then(epFlexibleEarn,
		status(http.StatusOK, `{"total":3,"rows":[{"asset":"USDT","totalAmount":"1.5"},{"asset":"SOL","totalAmount":"0.2"}]}`),
		status(http.StatusOK, `{"total":3,"rows":[{"asset":"ETH","totalAmount":"0.01"}]}`))
	r.then(epLockedEarn, status(http.StatusOK, `{"total":1,"rows":[{"asset":"USDT","amount":"10","positionId":1}]}`))
	// PURCHASE_SUCCESS spans two pages, and its ETH position starts settling between that read and
	// the SETTLING one, so both list it. REFUNDING keeps the recorded account's empty answer.
	r.alwaysAt(epDualInvest, dualInvestmentQuery("PENDING", 1), status(http.StatusOK,
		`{"total":1,"list":[{"id":4,"investCoin":"USDC","subscriptionAmount":"20","purchaseStatus":"PENDING"}]}`))
	r.alwaysAt(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1), status(http.StatusOK,
		`{"total":2,"list":[{"id":"1","investCoin":"USDT","subscriptionAmount":"100","purchaseStatus":"PURCHASE_SUCCESS"}]}`))
	r.alwaysAt(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 2), status(http.StatusOK,
		`{"total":2,"list":[{"id":"2","investCoin":"ETH","subscriptionAmount":"0.5","purchaseStatus":"PURCHASE_SUCCESS"}]}`))
	r.alwaysAt(epDualInvest, dualInvestmentQuery("SETTLING", 1), status(http.StatusOK,
		`{"total":1,"list":[{"id":"2","investCoin":"ETH","subscriptionAmount":"0.5","purchaseStatus":"SETTLING"}]}`))
	want := map[string]string{"USDT": "111.5", "SOL": "0.2", "ETH": "0.51", "USDC": "20"}
	value := adapters.Totals{}
	for symbol, amount := range want {
		if err := value.Add("test", symbol, rat(t, amount)); err != nil {
			t.Fatal(err)
		}
	}
	r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"0","walletName":"Spot"},{"activate":true,"balance":"`+r.earnUSD(value)+`","walletName":"Earn"}]`))
	batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeDualInvestment)
	earn := reportOf(t, batch, TypeEarn)
	dual := reportOf(t, batch, TypeDualInvestment)
	if dual.Status != adapters.StatusOK || len(dual.DualPositions) != 3 {
		t.Fatalf("dual report %+v", dual)
	}
	if balanceOf(t, earn, "USDT").Equity != "11.5" || balanceOf(t, dual, "USDT").Equity != "100" {
		t.Fatal("principal must be separated from savings")
	}
	if earn.Status != adapters.StatusOK {
		t.Fatalf("earn report %+v", earn)
	}
	got := map[string]string{}
	for _, b := range append(earn.Balances, dual.Balances...) {
		got[b.Symbol] = decimal.Format(decimal.Add(decimalOrZero(got[b.Symbol]), rat(t, b.Equity)))
	}
	if len(got) != len(want) {
		t.Fatalf("earn balances %v, want %v", got, want)
	}
	for symbol, equity := range want {
		if got[symbol] != equity {
			t.Fatalf("earn balances %v, want %v: a position listed under two states is counted once", got, want)
		}
	}
	if r.count(epFlexibleEarn) != 2 || r.count(epDualInvest) != 5 {
		t.Fatalf("flexible pages %d, dual investment pages %d; want 2 and 5", r.count(epFlexibleEarn), r.count(epDualInvest))
	}
}

func TestDualInvestmentQueriesOnlyRunningStates(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	collect(t, a, adapters.StructureBinancePM, TypeEarn)
	var queried []string
	for request := range r.called() {
		path, query, _ := strings.Cut(request, "?")
		if path != epDualInvest.String() {
			continue
		}
		values, err := url.ParseQuery(query)
		if err != nil || values.Get("pageIndex") != "1" || values.Get("pageSize") != strconv.Itoa(pageSize) {
			t.Fatalf("dual investment request %s", request)
		}
		queried = append(queried, values.Get("status"))
	}
	slices.Sort(queried)
	if want := []string{"PENDING", "PURCHASE_SUCCESS", "REFUNDING", "SETTLING"}; !slices.Equal(queried, want) {
		t.Fatalf("queried states %v, want %v: finished positions are never read", queried, want)
	}
}

// dualInvestmentAnswer is a dual investment page holding one 0.25 ETH position in state.
func dualInvestmentAnswer(state string) func(w http.ResponseWriter) {
	return status(http.StatusOK, `{"total":1,"list":[{"id":"7","investCoin":"ETH","subscriptionAmount":"0.25","purchaseStatus":"`+state+`"}]}`)
}

func TestDualInvestmentStatuses(t *testing.T) {
	t.Parallel()

	for _, state := range runningDualInvestmentStates() {
		t.Run(state+" is counted", func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, "")
			r.alwaysAt(epDualInvest, dualInvestmentQuery(state, 1), dualInvestmentAnswer(state))
			earn := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeDualInvestment), TypeDualInvestment)
			if earn.Status != adapters.StatusOK || balanceOf(t, earn, "ETH").Equity != "0.25" {
				t.Fatalf("earn report %+v", earn)
			}
		})
	}
	// A query answered with a finished, undocumented, or other running state no longer shows which
	// positions are running.
	unexpected := []string{"EXERCISING", "SETTLING"}
	for _, state := range adapters.SortedKeys(dualInvestmentStates) {
		if !dualInvestmentStates[state] {
			unexpected = append(unexpected, state)
		}
	}
	for _, state := range unexpected {
		t.Run("PURCHASE_SUCCESS answered with "+state, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, "")
			r.alwaysAt(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1), dualInvestmentAnswer(state))
			batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeSpot)
			earn := reportOf(t, batch, TypeEarn)
			if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassNormalization || !strings.Contains(earn.ErrorDetail, state) {
				t.Fatalf("earn report %+v", earn)
			}
			if reportOf(t, batch, TypeSpot).Status != adapters.StatusOK {
				t.Fatal("spot must not fail with Earn")
			}
		})
	}
}

func TestDocumentedDualInvestmentCountsThePrincipal(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t, docDualInvestment)
	earn := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeDualInvestment), TypeDualInvestment)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 1 || earn.Balances[0].Symbol != "USDT" || earn.Balances[0].Equity != "0.5" {
		t.Fatalf("earn report %+v: the example's running 0.5 USDT principal is counted", earn)
	}
}

func TestEarnCompletenessCheck(t *testing.T) {
	t.Parallel()

	t.Run("an unread product fails Earn", func(t *testing.T) {
		t.Parallel()
		r, a := newReplay(t, docDualInvestment)
		r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"5000","walletName":"Earn"}]`))
		batch := collect(t, a, adapters.StructureBinancePM, TypeEarn, TypeSpot)
		earn := reportOf(t, batch, TypeEarn)
		if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassUnsupported ||
			!strings.Contains(earn.ErrorDetail, "5000.00") || !strings.Contains(earn.ErrorDetail, "0.50") {
			t.Fatalf("earn report %+v", earn)
		}
		if reportOf(t, batch, TypeSpot).Status != adapters.StatusOK {
			t.Fatal("spot must not fail with Earn")
		}
	})
	t.Run("the tolerance absorbs price differences", func(t *testing.T) {
		t.Parallel()
		r, a := newReplay(t, docDualInvestment)
		r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"1.49","walletName":"Earn"}]`))
		if earn := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeEarn), TypeEarn); earn.Status != adapters.StatusOK {
			t.Fatalf("earn report %+v", earn)
		}
	})
	t.Run("no Earn wallet is a normalization failure", func(t *testing.T) {
		t.Parallel()
		r, a := newReplay(t, docDualInvestment)
		r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"0","walletName":"Spot"}]`))
		if earn := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeEarn), TypeEarn); earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassNormalization {
			t.Fatalf("earn report %+v", earn)
		}
	})
	t.Run("a coin without a price skips the check", func(t *testing.T) {
		t.Parallel()
		r, a := newReplay(t, docDualInvestment)
		r.always(epTickerPrice, status(http.StatusServiceUnavailable, ``))
		r.alwaysAt(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1), status(http.StatusOK,
			`{"total":1,"list":[{"id":"7","investCoin":"BNB","subscriptionAmount":"3","purchaseStatus":"PURCHASE_SUCCESS"}]}`))
		r.always(epWalletValue, status(http.StatusOK, `[{"activate":true,"balance":"5000","walletName":"Earn"}]`))
		if earn := reportOf(t, collect(t, a, adapters.StructureBinancePM, TypeEarn), TypeEarn); earn.Status != adapters.StatusOK {
			t.Fatalf("earn report %+v", earn)
		}
	})
}

func TestClassicCrossMargin(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeCrossMargin), TypeCrossMargin)
	if report.Status != adapters.StatusOK || report.Equity != nil || report.AdjustedEquity != nil || report.InitialMarginRatio != nil {
		t.Fatalf("report %+v: a Classic wallet is valued from its Balances", report)
	}
	account := body[crossMarginAccount](t, r, epCrossMargin, nil)
	if deref(report.UnifiedMarginRatio) != decimal.Compact(account.MarginLevel) {
		t.Fatalf("margin level %s, want %s", deref(report.UnifiedMarginRatio), account.MarginLevel)
	}
	if len(report.Balances) != 2 {
		t.Fatalf("balances %+v: the all-zero ETH row is omitted", report.Balances)
	}
	usdt := balanceOf(t, report, "USDT")
	if usdt.Equity != "-50.00416667" || deref(usdt.Liability) != "100" || deref(usdt.Interest) != "0.00416667" || usdt.AvailableToTrade != nil {
		t.Fatalf("USDT %+v", usdt)
	}
	if btc := balanceOf(t, report, "BTC"); btc.Equity != "0.004995" || deref(btc.Liability) != "0" {
		t.Fatalf("BTC %+v", btc)
	}
}

func TestClassicCrossMarginNotCreatedIsEmpty(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	r.always(epCrossMargin, status(http.StatusOK, `{"created":false,"marginLevel":"999","userAssets":[]}`))
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeCrossMargin), TypeCrossMargin)
	if report.Status != adapters.StatusOK || len(report.Balances) != 0 || report.UnifiedMarginRatio != nil {
		t.Fatalf("report %+v", report)
	}
}

func TestClassicIsolatedMarginSumsCoinsAcrossPairs(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t, docClassic)
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeIsolatedMargin), TypeIsolatedMargin)
	if report.Status != adapters.StatusOK || report.Equity != nil || report.UnifiedMarginRatio != nil || len(report.Balances) != 3 {
		t.Fatalf("report %+v", report)
	}
	usdt := balanceOf(t, report, "USDT")
	if usdt.Equity != "699.97916667" || deref(usdt.Liability) != "500" || deref(usdt.Interest) != "0.02083333" {
		t.Fatalf("USDT across BTCUSDT and ETHUSDT %+v", usdt)
	}
	if balanceOf(t, report, "BTC").Equity != "0.1" || balanceOf(t, report, "ETH").Equity != "1" {
		t.Fatalf("base coins %+v", report.Balances)
	}
}

func TestClassicUSDMFuturesInMultiAssetsMode(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeUMFutures), TypeUMFutures)
	if report.Status != adapters.StatusOK || report.Equity != nil || report.UnifiedMarginRatio != nil {
		t.Fatalf("report %+v", report)
	}
	account := body[futuresAccount](t, r, epUMAccount, nil)
	checks := map[string][2]string{
		"adjustedEquity":     {deref(report.AdjustedEquity), decimal.Compact(account.TotalMarginBalance)},
		"availableEquity":    {deref(report.AvailableEquity), decimal.Compact(account.AvailableBalance)},
		"initialMarginRatio": {deref(report.InitialMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.TotalInitialMargin), rat(t, account.TotalMarginBalance)).FloatString(8))},
		"maintMarginRatio":   {deref(report.MaintenanceMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.TotalMaintMargin), rat(t, account.TotalMarginBalance)).FloatString(8))},
	}
	for name, pair := range checks {
		if pair[0] != pair[1] || pair[0] == "0" {
			t.Errorf("%s = %s, want %s", name, pair[0], pair[1])
		}
	}
	if len(report.Balances) != 1 {
		t.Fatalf("balances %+v: the all-zero BNB asset is omitted", report.Balances)
	}
	usdt := balanceOf(t, report, "USDT")
	if usdt.Equity != "127.48896906" || deref(usdt.UnrealizedPnl) != "0.764277" || deref(usdt.AvailableToTrade) != "126.87325521" || deref(usdt.AvailableToWithdraw) != "126.72469206" {
		t.Fatalf("USDT %+v", usdt)
	}
	if len(report.Positions) != 1 {
		t.Fatalf("positions %+v", report.Positions)
	}
	p := report.Positions[0]
	if p.PlatformPositionRef != "um:BTCUSDT:BOTH" || p.QuantityUnit != adapters.UnitBase || p.Quantity != "30" || p.Side != adapters.SideLong ||
		deref(p.Leverage) != "21" || p.LiquidationPrice != nil || p.Instrument.Kind != adapters.KindLinearPerp {
		t.Fatalf("position %+v", p)
	}
	if r.count(epUMSymbolConfig) != 1 {
		t.Fatalf("symbol configuration read %d times, want once", r.count(epUMSymbolConfig))
	}
}

func TestClassicUSDMFuturesInSingleAssetModeHasNoAccountFigures(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	r.always(epUMMultiAssets, status(http.StatusOK, `{"multiAssetsMargin":false}`))
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeUMFutures), TypeUMFutures)
	if report.Status != adapters.StatusOK || report.AdjustedEquity != nil || report.AvailableEquity != nil ||
		report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || len(report.Balances) != 1 {
		t.Fatalf("report %+v: Single-Asset totals cover USDT only and are not stored", report)
	}
}

func TestClassicUSDMWithoutPositionSkipsSymbolConfig(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	r.always(epUMPositionRisk, status(http.StatusOK, `[]`))
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeUMFutures), TypeUMFutures)
	if report.Status != adapters.StatusOK || len(report.Positions) != 0 || r.count(epUMSymbolConfig) != 0 {
		t.Fatalf("report %+v, symbol configuration read %d times", report, r.count(epUMSymbolConfig))
	}
}

func TestClassicCOINMFutures(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t, docClassic)
	report := reportOf(t, collect(t, a, adapters.StructureBinanceClassic, TypeCMFutures), TypeCMFutures)
	if report.Status != adapters.StatusOK || report.Equity != nil || report.AdjustedEquity != nil || len(report.Balances) != 1 {
		t.Fatalf("report %+v", report)
	}
	if btc := balanceOf(t, report, "BTC"); btc.Equity != "0.00241969" || deref(btc.AvailableToWithdraw) != "0.00241969" {
		t.Fatalf("BTC %+v", btc)
	}
	if len(report.Positions) != 1 {
		t.Fatalf("positions %+v: the zero BTCUSD_201225 row is omitted", report.Positions)
	}
	p := report.Positions[0]
	if p.PlatformPositionRef != "cm:BTCUSD_PERP:BOTH" || p.Side != adapters.SideShort || p.Quantity != "3" || p.QuantityUnit != adapters.UnitContracts ||
		deref(p.Leverage) != "20" || deref(p.LiquidationPrice) != "95000.12345678" || p.Instrument.ContractSize == nil {
		t.Fatalf("position %+v", p)
	}
}

func TestClassicWalletsBinanceSaysAreNotOpenedReportEmpty(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	r.always(epAccountInfo, status(http.StatusOK, `{"vipLevel":0,"isMarginEnabled":false,"isFutureEnabled":false,"isOptionsEnabled":false,"isPortfolioMarginRetailEnabled":false}`))
	batch := collect(t, a, adapters.StructureBinanceClassic, TypeCrossMargin, TypeIsolatedMargin, TypeUMFutures, TypeCMFutures)
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK || len(report.Balances) != 0 || len(report.Positions) != 0 || report.ObservedAt.IsZero() {
			t.Fatalf("%s report %+v", report.AccountType, report)
		}
	}
	for _, ep := range []endpoint{epCrossMargin, epIsolatedMargin, epUMMultiAssets, epUMAccount, epUMPositionRisk, epCMAccount, epCMPositionRisk} {
		if r.count(ep) != 0 {
			t.Fatalf("%s was read for a wallet that is not opened", ep)
		}
	}
}

func TestClassicBatchValuesEveryAccount(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	batch := collect(t, a, adapters.StructureBinanceClassic, classicTypes...)
	if len(batch.Reports) != len(classicTypes) {
		t.Fatalf("%d reports for %d Account Types", len(batch.Reports), len(classicTypes))
	}
	priced := map[string]bool{}
	for _, p := range batch.Prices {
		priced[p.AssetSymbol] = true
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
		}
		for _, b := range report.Balances {
			if !adapters.USDEquivalent(b.Symbol) && !priced[b.Symbol] {
				t.Errorf("%s balance %s has no price", report.AccountType, b.Symbol)
			}
		}
	}
	for _, ep := range []endpoint{epPMAccount, epPMBalance, epPMUMPositionRisk, epPMCMPositionRisk} {
		if r.count(ep) != 0 {
			t.Fatalf("Classic Trading read the Portfolio Margin endpoint %s", ep)
		}
	}
}

func TestAccountTypeOfTheOtherStructureFailsOnlyItself(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, TypeUMFutures, TypeSpot)
	if um := reportOf(t, batch, TypeUMFutures); um.Status != adapters.StatusFailed || um.ErrorClass != adapters.ClassUnsupported {
		t.Fatalf("um_futures under Portfolio Margin %+v", um)
	}
	if reportOf(t, batch, TypeSpot).Status != adapters.StatusOK || r.count(epUMAccount) != 0 {
		t.Fatal("spot must still be collected and no Classic endpoint read")
	}
}

func TestCollectPrices(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t, "")
	batch := collect(t, a, adapters.StructureBinancePM, pmTypes...)
	priced := map[string]adapters.Price{}
	for _, p := range batch.Prices {
		priced[p.AssetSymbol] = p
		if p.QuoteSymbol != "USDT" && p.QuoteSymbol != "USDC" {
			t.Errorf("%s quoted in %s", p.AssetSymbol, p.QuoteSymbol)
		}
		if p.ObservedAt.IsZero() {
			t.Errorf("%s has no observation time", p.AssetSymbol)
		}
		for _, report := range batch.Reports {
			if p.ObservedAt.After(report.ObservedAt) {
				t.Errorf("price %s observed after the %s report; valuation would not pick it", p.AssetSymbol, report.AccountType)
			}
		}
	}
	for _, stable := range []string{"USDT", "USDC", "USD"} {
		if _, ok := priced[stable]; ok {
			t.Errorf("%s must not get a price row", stable)
		}
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed: %s %s", report.AccountType, report.ErrorClass, report.ErrorDetail)
		}
		for _, b := range report.Balances {
			if !adapters.USDEquivalent(b.Symbol) {
				if _, ok := priced[b.Symbol]; !ok {
					t.Errorf("%s balance %s has no price", report.AccountType, b.Symbol)
				}
			}
		}
	}
}

func TestPricesFallBackToUSDC(t *testing.T) {
	t.Parallel()

	reports := []adapters.AccountReport{{Balances: []adapters.Balance{{Symbol: "AAA"}, {Symbol: "BBB"}, {Symbol: "CCC"}, {Symbol: "USDT"}}}}
	table := lastPrices{"AAAUSDT": "2", "AAAUSDC": "2.1", "BBBUSDC": "3", "CCCFDUSD": "4"}
	prices := adapters.QuotePrices(reports, quotes, table.quote, time.Unix(1, 0))
	if len(prices) != 2 || prices[0].AssetSymbol != "AAA" || prices[0].QuoteSymbol != "USDT" || prices[1].AssetSymbol != "BBB" || prices[1].QuoteSymbol != "USDC" {
		t.Fatalf("prices %+v", prices)
	}
}

// Binance keeps delisted pairs in the ticker at price 0, such as the recorded BCPTUSDC; such a
// pair must not value a coin at zero.
func TestDelistedPairIsNotAPrice(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	recorded, _ := a.lastPrices(context.Background(), a.client(testCreds))
	if _, ok := recorded.quote("BCPT", "USDC"); ok {
		t.Fatal("the recorded BCPTUSDC pair is listed at price 0 and must not price BCPT")
	}
	r.always(epTickerPrice, status(http.StatusOK, `[{"symbol":"AAAUSDT","price":"0.00000000"},{"symbol":"AAAUSDC","price":"2.5"}]`))
	prices, _ := a.lastPrices(context.Background(), a.client(testCreds))
	reports := []adapters.AccountReport{{Balances: []adapters.Balance{{Symbol: "AAA"}}}}
	got := adapters.QuotePrices(reports, quotes, prices.quote, time.Unix(1, 0))
	if len(got) != 1 || got[0].QuoteSymbol != "USDC" || got[0].Price != "2.5" {
		t.Fatalf("prices %+v: the delisted USDT pair must give way to the USDC one", got)
	}
}

func TestTickerFailureKeepsReports(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epTickerPrice, status(http.StatusServiceUnavailable, ``))
	batch := collect(t, a, adapters.StructureBinancePM, pmTypes...)
	if len(batch.Prices) != 0 {
		t.Fatalf("prices %+v after a ticker failure", batch.Prices)
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed with the ticker: %+v", report.AccountType, report)
		}
	}
}

func TestAuthenticationFailsTheBatch(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"invalid_key", "bad_signature"} {
		for _, ep := range []endpoint{epAccountInfo, epPMAccount} {
			t.Run(scenario+" "+ep.path, func(t *testing.T) {
				t.Parallel()
				r, a := newReplay(t, "")
				r.always(ep, serve(r.failure(scenario)))
				_, err := a.Collect(context.Background(), testCreds, adapters.StructureBinancePM, pmTypes)
				if classOfErr(t, err) != adapters.ClassAuthentication {
					t.Fatalf("%s on %s: error %v", scenario, ep, err)
				}
				if !adapters.StopsScheduling(classOfErr(t, err)) {
					t.Fatal("authentication must stop scheduling")
				}
			})
		}
	}
}

func TestErrorDetailNamesBinanceCodeOnly(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epAccountInfo, serve(r.failure("invalid_key")))
	_, err := a.Collect(context.Background(), testCreds, adapters.StructureBinancePM, []adapters.AccountType{TypePortfolioMargin})
	_, detail := adapters.Classify(err)
	if !strings.Contains(detail, "GET /sapi/v1/account/info") || !strings.Contains(detail, "-2008") {
		t.Fatalf("detail %q", detail)
	}
	if strings.Contains(detail, testKey) || strings.Contains(detail, "{") {
		t.Fatalf("detail %q echoes a key or a body", detail)
	}
}

func TestStaleTimestampResyncsAndRetries(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.then(epAccountInfo, serve(r.failure("stale_timestamp")))
	batch := collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin)
	if reportOf(t, batch, TypePortfolioMargin).Status != adapters.StatusOK {
		t.Fatal("the retried call must succeed")
	}
	if r.count(epServerTime) != 2 || r.count(epAccountInfo) != 2 {
		t.Fatalf("server time %d, account info %d calls; want one resync and one retry", r.count(epServerTime), r.count(epAccountInfo))
	}
}

func TestErrorClasses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		body   string
		class  string
	}{
		{http.StatusTooManyRequests, `{"code":-1003,"msg":"Too many requests."}`, adapters.ClassRateLimited},
		{http.StatusTeapot, `{"code":-1003,"msg":"Way too many requests; IP banned."}`, adapters.ClassRateLimited},
		{http.StatusForbidden, ``, adapters.ClassRateLimited},
		{http.StatusInternalServerError, ``, adapters.ClassTemporary},
		{http.StatusServiceUnavailable, `{"code":-1001,"msg":"Internal error; unable to process your request."}`, adapters.ClassTemporary},
		{http.StatusUnauthorized, ``, adapters.ClassAuthentication},
		{http.StatusBadRequest, `{"code":-1102,"msg":"Mandatory parameter was not sent."}`, adapters.ClassTemporary},
		{http.StatusBadRequest, `{"code":-2014,"msg":"API-key format invalid."}`, adapters.ClassAuthentication},
		{http.StatusBadRequest, `{"code":-2008,"msg":"Invalid Api-Key ID."}`, adapters.ClassAuthentication},
	} {
		err := classify(epPMAccount, response{status: tc.status, body: []byte(tc.body)})
		if got := classOfErr(t, err); got != tc.class {
			t.Errorf("HTTP %d %s: class %s, want %s", tc.status, tc.body, got, tc.class)
		}
	}
}

func TestOneFailingAccountLeavesSiblingsOK(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epFlexibleEarn, status(http.StatusBadRequest, `{"code":-6001,"msg":"Simple Earn is not available for this account."}`))
	batch := collect(t, a, adapters.StructureBinancePM, pmTypes...)
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusFailed || !strings.Contains(earn.ErrorDetail, "-6001") {
		t.Fatalf("earn report %+v", earn)
	}
	for _, sibling := range []adapters.AccountType{TypePortfolioMargin, TypeSpot, TypeFunding} {
		if reportOf(t, batch, sibling).Status != adapters.StatusOK {
			t.Fatalf("%s failed with earn", sibling)
		}
	}
}

func TestEveryAccountRetryableFailureFailsTheBatch(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	for _, ep := range []endpoint{epPMAccount, epUserAsset} {
		r.always(ep, status(http.StatusServiceUnavailable, ``))
	}
	_, err := a.Collect(context.Background(), testCreds, adapters.StructureBinancePM, []adapters.AccountType{TypePortfolioMargin, TypeSpot})
	if classOfErr(t, err) != adapters.ClassTemporary {
		t.Fatalf("error %v: the collection service retries only a batch error", err)
	}
}

func TestUnknownInstrumentFailsTheAccount(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epPMCMPositionRisk, status(http.StatusOK, `[{"symbol":"NOPEUSD_PERP","positionAmt":"2","positionSide":"BOTH","entryPrice":"1","markPrice":"1","liquidationPrice":"0","unRealizedProfit":"0","leverage":"2"}]`))
	batch := collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin, TypeSpot)
	pm := reportOf(t, batch, TypePortfolioMargin)
	if pm.Status != adapters.StatusFailed || pm.ErrorClass != adapters.ClassNormalization || !strings.Contains(pm.ErrorDetail, "NOPEUSD_PERP") {
		t.Fatalf("pm report %+v", pm)
	}
	if reportOf(t, batch, TypeSpot).Status != adapters.StatusOK {
		t.Fatal("spot must not fail with an unknown contract")
	}
}

func TestNonDecimalFieldIsNormalizationFailure(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "")
	r.always(epPMAccount, status(http.StatusOK, `{"actualEquity":"n/a","accountEquity":"1","accountInitialMargin":"0","accountMaintMargin":"0","totalAvailableBalance":"1","uniMMR":"0"}`))
	batch := collect(t, a, adapters.StructureBinancePM, TypePortfolioMargin, TypeSpot)
	if pm := reportOf(t, batch, TypePortfolioMargin); pm.Status != adapters.StatusFailed || pm.ErrorClass != adapters.ClassNormalization {
		t.Fatalf("pm report %+v", pm)
	}
}

func TestAmountWithoutACoinIsNormalizationFailure(t *testing.T) {
	t.Parallel()

	if _, err := mapWallet(epUserAsset, []walletAsset{{Asset: " ", Free: "1"}}); classOfErr(t, err) != adapters.ClassNormalization {
		t.Fatalf("error %v: an amount without a coin must not disappear", err)
	}
	if balances, err := mapWallet(epUserAsset, []walletAsset{{Asset: "", Free: "0"}}); err != nil || len(balances) != 0 {
		t.Fatalf("an empty row is skipped: %+v %v", balances, err)
	}
}

func TestZeroEquityLeavesMarginRatiosNull(t *testing.T) {
	t.Parallel()

	report, err := mapPMAccount(&pmAccount{ActualEquity: "0", AccountEquity: "0", AccountInitialMargin: "0", AccountMaintMargin: "0", TotalAvailable: "0", UniMMR: "0"})
	if err != nil {
		t.Fatal(err)
	}
	if report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || deref(report.Equity) != "0" {
		t.Fatalf("report %+v", report)
	}
}

func TestShortPositionAndZeroLiquidation(t *testing.T) {
	t.Parallel()

	in := adapters.Instrument{Symbol: "BTCUSDT", Kind: adapters.KindLinearPerp}
	row := &positionRisk{Symbol: "BTCUSDT", PositionAmt: "-0.5", PositionSide: "BOTH", EntryPrice: "100", MarkPrice: "90", LiquidationPrice: "0", UnRealizedProfit: "5", Leverage: "3"}
	p, err := mapPosition(marketUM, epPMUMPositionRisk, row, rat(t, row.PositionAmt), &in)
	if err != nil {
		t.Fatal(err)
	}
	if p.Side != adapters.SideShort || p.Quantity != "0.5" || p.LiquidationPrice != nil || p.PlatformPositionRef != "um:BTCUSDT:BOTH" {
		t.Fatalf("position %+v", p)
	}
	hedged := &positionRisk{Symbol: "BTCUSDT", PositionAmt: "0.5", PositionSide: "SHORT", EntryPrice: "1", MarkPrice: "1", LiquidationPrice: "1", UnRealizedProfit: "0", Leverage: "1"}
	if p, _ := mapPosition(marketUM, epPMUMPositionRisk, hedged, rat(t, hedged.PositionAmt), &in); p.Side != adapters.SideShort {
		t.Fatalf("hedge-mode SHORT mapped to %s", p.Side)
	}
}

// The recorder must capture every endpoint a Collection Run of the recorded structure reads, so the
// replayed fixtures stay complete.
func TestRecordCapturesEveryEndpointTheStructureReads(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		structure adapters.AccountStructure
		overlay   string
	}{{adapters.StructureBinancePM, ""}, {adapters.StructureBinanceClassic, docClassic}} {
		t.Run(string(tc.structure), func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t, tc.overlay)
			if _, err := a.Validate(context.Background(), testCreds, tc.structure); err != nil {
				t.Fatal(err)
			}
			collect(t, a, tc.structure, adapters.AccountTypeIDs(tc.structure)...)
			read := r.called()

			r, a = newReplay(t, tc.overlay)
			r.tolerateErrors()
			recordings, err := a.Record(context.Background(), testCreds)
			if err != nil {
				t.Fatal(err)
			}
			recorded := map[string]bool{}
			failures := map[string]bool{}
			for _, rec := range recordings {
				if rec.AccountStructure != string(tc.structure) || rec.MarginMode != "" {
					t.Fatalf("%s recorded as %q/%q", rec.Endpoint, rec.AccountStructure, rec.MarginMode)
				}
				if rec.Scenario == fixture.ScenarioRecorded {
					if rec.Status != http.StatusOK || rec.Name != fixture.Name(strings.SplitN(rec.Endpoint, " ", 2)[1], "") {
						t.Fatalf("recording %s: status %d name %s", rec.Endpoint, rec.Status, rec.Name)
					}
					recorded[rec.Endpoint] = true
					continue
				}
				if rec.Endpoint != epAccountInfo.String() || rec.Status == http.StatusOK {
					t.Fatalf("error scenario %s on %s with status %d", rec.Scenario, rec.Endpoint, rec.Status)
				}
				failures[rec.Scenario] = true
			}
			for name := range read {
				if !recorded[name] {
					t.Errorf("a Collection Run reads %s, which the recorder does not capture", name)
				}
			}
			for _, scenario := range []string{"invalid_key", "bad_signature", "stale_timestamp"} {
				if !failures[scenario] {
					t.Errorf("no %s recording", scenario)
				}
			}
		})
	}
}

func TestRecordSelectedDualFixture(t *testing.T) {
	name := fixture.Name(target(epDualInvest, dualInvestmentQuery("PURCHASE_SUCCESS", 1)), "")
	for _, overlay := range []string{"", docClassic} {
		r, a := newReplay(t, overlay)
		recordings, err := a.RecordSelected(context.Background(), testCreds, name)
		if err != nil {
			t.Fatal(err)
		}
		if len(recordings) != 1 || recordings[0].Name != name || recordings[0].Scenario != fixture.ScenarioRecorded ||
			recordings[0].Endpoint != epDualInvest.String()+"?"+dualInvestmentQuery("PURCHASE_SUCCESS", 1).Encode() {
			t.Fatalf("selected recording: %+v", recordings)
		}
		for request := range r.called() {
			path, _, _ := strings.Cut(request, "?")
			if path != epServerTime.String() && path != epAccountInfo.String() && path != epDualInvest.String() {
				t.Fatalf("unrelated endpoint called: %s", request)
			}
		}
	}
}

func TestEd25519RecorderErrorProbes(t *testing.T) {
	r, a := newReplay(t, "")
	r.secret = testEdCreds.APISecret
	r.tolerateErrors()
	recordings, err := a.Record(context.Background(), testEdCreds)
	if err != nil {
		t.Fatal(err)
	}
	failures := map[string]bool{}
	for _, recording := range recordings {
		if recording.Scenario != fixture.ScenarioRecorded {
			failures[recording.Scenario] = true
		}
	}
	for _, scenario := range []string{"invalid_key", "bad_signature", "stale_timestamp"} {
		if !failures[scenario] {
			t.Errorf("missing %s probe", scenario)
		}
	}
}

func TestRecorderRedactsTheUIDAndTrimsMarketLists(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, docClassic)
	r.tolerateErrors()
	r.always(epUMSymbolConfig, status(http.StatusOK, `[{"symbol":"BTCUSDT","leverage":21},{"symbol":"ETHUSDT","leverage":5},{"symbol":"XRPUSDT","leverage":2}]`))
	r.always(epSpotAccount, status(http.StatusOK, `{"uid":987654321,"balances":[]}`))
	recordings, err := a.Record(context.Background(), testCreds)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range recordings {
		switch rec.Endpoint {
		case epSpotAccount.String():
			if strings.Contains(string(rec.Body), "987654321") || !strings.Contains(string(rec.Body), strconv.Itoa(RedactedUID)) {
				t.Fatalf("uid not redacted: %s", rec.Body)
			}
		case epUMSymbolConfig.String():
			if strings.Contains(string(rec.Body), "ETHUSDT") || !strings.Contains(string(rec.Body), "BTCUSDT") || rec.Trimmed == nil {
				t.Fatalf("symbol configuration not trimmed to held contracts: %s", rec.Body)
			}
		}
	}
}

func decimalOrZero(value string) *big.Rat {
	if value == "" {
		return new(big.Rat)
	}
	return decimal.MustParse(value)
}
