package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
	testKey        = "test-api-key"
	testSecret     = "test-api-secret"
	testPassphrase = "test-passphrase"
)

var testCreds = adapters.Credentials{APIKey: testKey, APISecret: testSecret, Passphrase: testPassphrase}

var allTypes = []adapters.AccountType{TypeTrading, TypeFunding, TypeEarn, TypeDualInvestment}

// replay serves the committed fixtures: the recorded set, with the answers of the named
// documentation-derived scenarios (testdata/documented/<name>/) in place of the recorded ones. It
// verifies the key, passphrase, and signature headers and that timestamps follow the recorded
// server time, so signing and clock sync are exercised.
type replay struct {
	t          *testing.T
	files      map[string]fixture.File
	serverTime int64
	started    time.Time

	mu    sync.Mutex
	calls map[string]int
	queue map[string][]func(w http.ResponseWriter)
	fixed map[string]func(w http.ResponseWriter)
}

// signedPaths are the paths whose requests must carry valid OK-ACCESS-* headers.
var signedPaths = func() map[string]bool {
	out := map[string]bool{}
	for _, ep := range []endpoint{epConfig, epBalance, epPositions, epFunding, epSavings, epStaking, epDCD, epValuation} {
		out[ep.path] = true
	}
	return out
}()

func newReplay(t *testing.T, scenarios ...string) (*replay, *Adapter) {
	t.Helper()
	r := &replay{t: t, files: map[string]fixture.File{}, calls: map[string]int{}, queue: map[string][]func(http.ResponseWriter){}, fixed: map[string]func(http.ResponseWriter){}, started: time.Now()}
	r.load(filepath.Join("testdata", "*.json"), false)
	for _, scenario := range scenarios {
		r.load(filepath.Join("testdata", "documented", scenario, "*.json"), true)
	}
	times := rowsIn[struct {
		TS string `json:"ts"`
	}](t, r.file(epTime, ""))
	if len(times) != 1 {
		t.Fatalf("public/time fixture rows %d", len(times))
	}
	var err error
	r.serverTime, err = strconv.ParseInt(times[0].TS, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	a := New(&Options{Host: server.URL, HTTP: server.Client()})
	a.pace = 0
	return r, a
}

// load adds the fixtures matching pattern, replacing earlier answers of the same endpoint.
func (r *replay) load(pattern string, documented bool) {
	r.t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil || len(paths) == 0 {
		r.t.Fatalf("no fixtures match %s: %v", pattern, err)
	}
	for _, path := range paths {
		f, err := fixture.Read(path)
		if err != nil {
			r.t.Fatal(err)
		}
		if problem := headerProblem(&f.Fixture, documented); problem != "" {
			r.t.Fatalf("%s: %s: %+v", path, problem, f.Fixture)
		}
		r.files[key(f.Fixture.Endpoint, answerScenario(f.Fixture.Scenario))] = f
	}
}

// answerScenario is the replay key of a fixture's scenario: a recorded or documented success is
// the endpoint's default answer "", and an error answer keeps the name of its failure.
func answerScenario(scenario string) string {
	if scenario == fixture.ScenarioRecorded || scenario == fixture.ScenarioDocumented {
		return ""
	}
	return scenario
}

// headerProblem checks a fixture header against the provenance rules of ADR 0023 and ADR 0027: a
// recorded fixture carries its recording date, a documentation-derived one the documentation it
// follows and no date.
func headerProblem(h *fixture.Header, documented bool) string {
	switch {
	case h.Platform != "okx" || h.Endpoint == "":
		return "the header names no platform or endpoint"
	case h.AccountStructure != string(adapters.StructureOKXUnified) || h.MarginMode == "" ||
		!adapters.ValidMarginMode(adapters.StructureOKXUnified, adapters.MarginMode(h.MarginMode)):
		return "the header names no okx_unified Account Structure and Margin Mode"
	case documented && (!h.Documented() || h.RecordedAt != nil || !strings.HasPrefix(h.Documentation, "https://www.okx.com/docs-v5/")):
		return "a documentation-derived fixture needs scenario documented, the documentation it follows, and no recording date"
	case !documented && (h.Documented() || h.Scenario == "" || h.RecordedAt == nil || h.RecordedAt.IsZero() || h.Documentation != ""):
		return "a recorded fixture needs a scenario and its recording date, and no documentation"
	}
	return ""
}

func key(endpoint, scenario string) string { return endpoint + "#" + scenario }

func (r *replay) file(ep endpoint, scenario string) fixture.File {
	r.t.Helper()
	f, ok := r.files[key(ep.String(), scenario)]
	if !ok {
		r.t.Fatalf("no fixture for %s scenario %q", ep, scenario)
	}
	return f
}

// then queues one-shot answers for an endpoint, served before the default fixture.
func (r *replay) then(ep endpoint, answers ...func(w http.ResponseWriter)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue[ep.String()] = append(r.queue[ep.String()], answers...)
}

// always replaces an endpoint's default answer.
func (r *replay) always(ep endpoint, answer func(w http.ResponseWriter)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fixed[ep.String()] = answer
}

func (r *replay) count(ep endpoint) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[ep.String()]
}

func TestRecordSelectedDualFixture(t *testing.T) {
	r, a := newReplay(t)
	name := recordingName(epDCD, fixture.ScenarioRecorded)
	recordings, err := a.RecordSelected(context.Background(), testCreds, name)
	if err != nil {
		t.Fatal(err)
	}
	if len(recordings) != 1 || recordings[0].Name != name || recordings[0].Endpoint != epDCD.String() {
		t.Fatalf("selected recording: %+v", recordings)
	}
	if r.count(epConfig) != 1 || r.count(epDCD) != 1 || r.count(epBalance) != 0 || r.count(epSavings) != 0 || r.count(epTickers) != 0 {
		t.Fatal("selected recording must read only config, clock, and Dual Investment")
	}
}

// configAt answers account/config as recorded but at another account level.
func (r *replay) configAt(level string) func(w http.ResponseWriter) {
	r.t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(r.file(epConfig, "").Body, &top); err != nil {
		r.t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(top["data"], &rows); err != nil {
		r.t.Fatal(err)
	}
	rows[0]["acctLv"] = level
	var err error
	if top["data"], err = json.Marshal(rows); err != nil {
		r.t.Fatal(err)
	}
	body, err := json.Marshal(top)
	if err != nil {
		r.t.Fatal(err)
	}
	return status(http.StatusOK, string(body))
}

func serve(f fixture.File) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { write(w, f.Status, f.Body) }
}

func status(code int, body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { write(w, code, []byte(body)) }
}

// ok answers a successful envelope with the given data rows.
func ok(data string) func(w http.ResponseWriter) {
	return status(http.StatusOK, `{"code":"0","msg":"","data":`+data+`}`)
}

func write(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func (r *replay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	name := req.Method + " " + req.URL.RequestURI()
	r.mu.Lock()
	r.calls[name]++
	answer := r.fixed[name]
	if queue := r.queue[name]; len(queue) > 0 {
		answer, r.queue[name] = queue[0], queue[1:]
	}
	r.mu.Unlock()
	if signedPaths[req.URL.Path] {
		if msg := r.checkSigned(req); msg != "" {
			r.t.Errorf("%s: %s", name, msg)
			serve(r.files[key(epConfig.String(), "bad_signature")])(w)
			return
		}
	}
	if answer != nil {
		answer(w)
		return
	}
	f, ok := r.files[key(name, "")]
	if !ok {
		r.t.Errorf("unexpected request %s", name)
		write(w, http.StatusNotFound, []byte(`{"code":"50005","msg":"no fixture","data":[]}`))
		return
	}
	serve(f)(w)
}

func (r *replay) checkSigned(req *http.Request) string {
	if req.Header.Get("OK-ACCESS-KEY") != testKey {
		return "missing or wrong OK-ACCESS-KEY header"
	}
	if req.Header.Get("OK-ACCESS-PASSPHRASE") != testPassphrase {
		return "missing or wrong OK-ACCESS-PASSPHRASE header"
	}
	stamp := req.Header.Get("OK-ACCESS-TIMESTAMP")
	at, err := time.Parse(timestampLayout, stamp)
	if err != nil {
		return "OK-ACCESS-TIMESTAMP is not ISO 8601 UTC with milliseconds: " + stamp
	}
	mac := hmac.New(sha256.New, []byte(testSecret))
	_, _ = mac.Write([]byte(stamp + req.Method + req.URL.RequestURI()))
	if req.Header.Get("OK-ACCESS-SIGN") != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		return "signature does not match"
	}
	expected := r.serverTime + time.Since(r.started).Milliseconds()
	if diff := at.UnixMilli() - expected; diff > 5000 || diff < -5000 {
		return fmt.Sprintf("timestamp is not aligned with the recorded server time: off by %d ms", diff)
	}
	return ""
}

// rowsIn decodes the data rows of a fixture.
func rowsIn[T any](t *testing.T, f fixture.File) []T {
	t.Helper()
	var envelope struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(f.Body, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func rat(t *testing.T, value string) *big.Rat {
	t.Helper()
	out, ok := decimal.Parse(value)
	if !ok {
		t.Fatalf("not a decimal: %q", value)
	}
	return out
}

// sameDecimal compares a mapped value with the reported one by value, not by spelling.
func sameDecimal(t *testing.T, got *string, want string) bool {
	t.Helper()
	return got != nil && rat(t, *got).Cmp(rat(t, want)) == 0
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

func classOfErr(t *testing.T, err error) string {
	t.Helper()
	var adapterErr *adapters.Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("error %v is not an adapter error", err)
	}
	return adapterErr.Class
}

var eighteenDigits = regexp.MustCompile(`\b[0-9]{18}\b`)

func TestFixturesHaveNoCredentialOrIdentityEcho(t *testing.T) {
	t.Parallel()

	var paths []string
	err := filepath.WalkDir("testdata", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".json") {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures in testdata: %v", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path) //nolint:gosec // committed fixture
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"apiKey", "secretKey", "OK-ACCESS-SIGN:", "OK-ACCESS-KEY:"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s contains %q", path, forbidden)
			}
		}
		if ipv4.MatchString(strings.ReplaceAll(text, RedactedIP, "")) {
			t.Errorf("%s contains an IPv4 address other than the redaction placeholder", path)
		}
		// OKX UIDs are 18 digits; position ids are 19 and times 13, so any 18-digit number
		// must be a placeholder.
		for _, number := range eighteenDigits.FindAllString(text, -1) {
			if number != RedactedUID && number != RedactedMainUID {
				t.Errorf("%s contains the 18-digit number %s, which may be an account UID", path, number)
			}
		}
	}
	configs := rowsIn[map[string]any](t, fixtureFile(t, epConfig, ""))
	if len(configs) != 1 {
		t.Fatalf("account/config rows %d", len(configs))
	}
	cfg := configs[0]
	if cfg["uid"] != RedactedUID || (cfg["mainUid"] != RedactedUID && cfg["mainUid"] != RedactedMainUID) || cfg["ip"] != "" || cfg["label"] != RedactedLabel {
		t.Fatalf("account/config is not redacted: uid %v, mainUid %v, ip %v, label %v", cfg["uid"], cfg["mainUid"], cfg["ip"], cfg["label"])
	}
}

// TestRecordedFixturesAreNamedAsTheRecorderNamesThem keeps the committed recording and the
// recorder in step: a successful answer is stored under its endpoint alone, an error answer under
// its endpoint and scenario.
func TestRecordedFixturesAreNamedAsTheRecorderNamesThem(t *testing.T) {
	t.Parallel()

	if got := recordingName(dcdPage("987654321"), fixture.ScenarioRecorded); got != "api_v5_finance_sfp_dcd_order_history.987654321.100" {
		t.Fatalf("recorded dual investment page named %q", got)
	}
	if got := recordingName(epConfig, "invalid_key"); got != "api_v5_account_config.invalid_key" {
		t.Fatalf("error answer named %q", got)
	}
	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no recorded fixtures: %v", err)
	}
	for _, path := range paths {
		f, err := fixture.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		_, request, _ := strings.Cut(f.Fixture.Endpoint, " ")
		scenario := f.Fixture.Scenario
		if scenario == fixture.ScenarioRecorded {
			scenario = ""
		}
		if want := fixture.Name(request, scenario) + ".json"; filepath.Base(path) != want {
			t.Errorf("%s holds %s (%s), which the recorder writes as %s", path, f.Fixture.Endpoint, f.Fixture.Scenario, want)
		}
	}
}

// TestEveryRecordedAnswerIsRequested replays one validation and one collection of every Account
// Type and checks that each recorded success answers a request the adapter sends, so a header
// whose endpoint differs from the request URI cannot go unnoticed. The FUTURES list is requested
// only for a futures position, which TestPositionSidesAndSkippedRows replays.
func TestEveryRecordedAnswerIsRequested(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	ctx := context.Background()
	if _, err := a.Validate(ctx, testCreds, adapters.StructureOKXUnified); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Collect(ctx, testCreds, adapters.StructureOKXUnified, allTypes); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.files {
		if answerScenario(f.Fixture.Scenario) != "" || f.Fixture.Endpoint == epFutures.String() {
			continue
		}
		if r.calls[f.Fixture.Endpoint] == 0 {
			t.Errorf("the recorded answer of %s was never requested", f.Fixture.Endpoint)
		}
	}
}

func fixtureFile(t *testing.T, ep endpoint, scenario string) fixture.File {
	t.Helper()
	f, err := fixture.Read(filepath.Join("testdata", fixture.Name(ep.request(), scenario)+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDocumentedScenariosDeclareTheirProvenance(t *testing.T) {
	t.Parallel()

	scenarios, err := os.ReadDir(filepath.Join("testdata", "documented"))
	if err != nil || len(scenarios) == 0 {
		t.Fatalf("no documented scenarios: %v", err)
	}
	for _, scenario := range scenarios {
		// newReplay fails on a header that breaks the provenance rules.
		newReplay(t, scenario.Name())
	}
}

func TestValidateReportsTheMarginMode(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	result, err := a.Validate(context.Background(), testCreds, adapters.StructureOKXUnified)
	if err != nil || result.ExternalRef != RedactedUID || result.MarginMode != adapters.MarginOKXPM {
		t.Fatalf("validate the recorded portfolio margin account: %v %+v", err, result)
	}
	if r.count(epTime) != 1 || r.count(epConfig) != 1 {
		t.Fatalf("server time %d, config %d calls; want one clock sync and one config read", r.count(epTime), r.count(epConfig))
	}
	for level, mode := range marginModes {
		t.Run("acctLv "+level, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epConfig, r.configAt(level))
			result, err := a.Validate(context.Background(), testCreds, adapters.StructureOKXUnified)
			if err != nil || result.ExternalRef != RedactedUID || result.MarginMode != mode {
				t.Fatalf("validate at acctLv %s: %v %+v, want %s", level, err, result, mode)
			}
		})
	}
}

func TestUnknownAccountLevelIsRefused(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		level, class, name string
	}{
		{"5", adapters.ClassUnsupported, "acctLv 5"},
		{"", adapters.ClassNormalization, "acctLv"},
	} {
		t.Run("acctLv "+tc.level, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epConfig, r.configAt(tc.level))
			_, err := a.Validate(context.Background(), testCreds, adapters.StructureOKXUnified)
			if classOfErr(t, err) != tc.class || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("validate: %v, want %s naming %q", err, tc.class, tc.name)
			}
			_, err = a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
			if classOfErr(t, err) != tc.class || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("collect: %v, want %s naming %q", err, tc.class, tc.name)
			}
			for _, ep := range []endpoint{epTickers, epBalance, epFunding, epSavings} {
				if r.count(ep) != 0 {
					t.Errorf("%s called %d times after the Margin Mode was refused", ep, r.count(ep))
				}
			}
		})
	}
	if adapters.StopsScheduling(adapters.ClassUnsupported) || adapters.Retryable(adapters.ClassUnsupported) {
		t.Fatal("an unknown Margin Mode must neither invalidate the Connection nor be retried")
	}
}

func TestOtherStructuresAreRefused(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	for _, structure := range []adapters.AccountStructure{adapters.StructureBinancePM, adapters.StructureBybitUTA2, ""} {
		if _, err := a.Validate(context.Background(), testCreds, structure); classOfErr(t, err) != adapters.ClassUnsupported {
			t.Fatalf("validate %q: %v", structure, err)
		}
		if _, err := a.Collect(context.Background(), testCreds, structure, allTypes); classOfErr(t, err) != adapters.ClassUnsupported {
			t.Fatalf("collect %q: %v", structure, err)
		}
	}
	if r.count(epConfig) != 0 {
		t.Fatalf("another structure reached OKX %d times", r.count(epConfig))
	}
}

func TestLevelChangedInsideOKXIsObserved(t *testing.T) {
	t.Parallel()

	// The Margin Mode is observed, never declared: a level switched inside OKX is reported with
	// the batch instead of failing it.
	r, a := newReplay(t)
	r.always(epConfig, r.configAt("3"))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if err != nil || batch.MarginMode != adapters.MarginOKXMCM {
		t.Fatalf("collect after a switch to acctLv 3: %v, margin mode %q", err, batch.MarginMode)
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed: %+v", report.AccountType, report)
		}
	}
}

func TestUnknownAccountTypeFailsOnlyItsAccount(t *testing.T) {
	t.Parallel()

	_, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, "margin"})
	if err != nil {
		t.Fatal(err)
	}
	unknown := reportOf(t, batch, "margin")
	if unknown.Status != adapters.StatusFailed || unknown.ErrorClass != adapters.ClassUnsupported || !strings.Contains(unknown.ErrorDetail, "margin") {
		t.Fatalf("unknown Account Type report %+v", unknown)
	}
	if reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatal("funding must not fail with an unknown Account Type")
	}
}

func TestCollectMapsTradingAccount(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	report := reportOf(t, batch, TypeTrading)
	if report.Status != adapters.StatusOK || report.ObservedAt.IsZero() {
		t.Fatalf("report %+v", report)
	}
	accounts := rowsIn[tradingAccount](t, r.file(epBalance, ""))
	account := accounts[0]
	checks := map[string][2]string{
		"equity":             {deref(report.Equity), decimal.Compact(account.TotalEq)},
		"adjustedEquity":     {deref(report.AdjustedEquity), decimal.Compact(account.AdjEq)},
		"availableEquity":    {deref(report.AvailableEquity), decimal.Compact(account.AvailEq)},
		"unifiedMarginRatio": {deref(report.UnifiedMarginRatio), decimal.Compact(account.MgnRatio)},
		"initialMarginRatio": {deref(report.InitialMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.IMR), rat(t, account.AdjEq)).FloatString(8))},
		"maintMarginRatio":   {deref(report.MaintenanceMarginRatio), decimal.Compact(new(big.Rat).Quo(rat(t, account.MMR), rat(t, account.AdjEq)).FloatString(8))},
	}
	for name, pair := range checks {
		if pair[0] != pair[1] {
			t.Errorf("%s = %s, want %s", name, pair[0], pair[1])
		}
	}

	// OKX's own per-coin USD equities add up to totalEq exactly, which is why `eq` is taken as
	// reported instead of being rebuilt from cash, PnL, and liabilities.
	var withUSD []struct {
		Ccy   string `json:"ccy"`
		EqUSD string `json:"eqUsd"`
	}
	var details struct {
		Details json.RawMessage `json:"details"`
	}
	raw := rowsIn[json.RawMessage](t, r.file(epBalance, ""))
	if err := json.Unmarshal(raw[0], &details); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(details.Details, &withUSD); err != nil {
		t.Fatal(err)
	}
	sumUSD := new(big.Rat)
	for _, d := range withUSD {
		sumUSD.Add(sumUSD, rat(t, d.EqUSD))
	}
	if sumUSD.Cmp(rat(t, account.TotalEq)) != 0 {
		t.Errorf("Σ eqUsd %s differs from totalEq %s", decimal.Format(sumUSD), account.TotalEq)
	}

	nonzero := 0
	for i := range account.Details {
		if rat(t, account.Details[i].Eq).Sign() != 0 {
			nonzero++
		}
	}
	if len(report.Balances) != nonzero {
		t.Fatalf("%d balances, want the %d nonzero rows", len(report.Balances), nonzero)
	}
	for _, b := range report.Balances {
		for i := range account.Details {
			d := &account.Details[i]
			if d.Ccy != b.Symbol {
				continue
			}
			if rat(t, b.Equity).Cmp(rat(t, d.Eq)) != 0 {
				t.Errorf("%s equity %s, want eq %s", b.Symbol, b.Equity, d.Eq)
			}
			if !sameDecimal(t, b.AvailableToTrade, d.AvailEq) || !sameDecimal(t, b.AvailableToWithdraw, d.AvailBal) ||
				!sameDecimal(t, b.Liability, d.Liab) || !sameDecimal(t, b.Interest, d.Interest) || !sameDecimal(t, b.UnrealizedPnl, d.Upl) {
				t.Errorf("%s availability, liability, interest, or PnL differs from the row: %+v vs %+v", b.Symbol, b, *d)
			}
			if b.Name != "" {
				t.Errorf("%s: name stays unmapped", b.Symbol)
			}
		}
	}

	// Σ balance equity × spot price lands within 0.5% of totalEq, which ties the per-coin
	// mapping to OKX's own account equity (USDT and USDC count at 1 per ADR 0017).
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
	total := rat(t, account.TotalEq)
	gap, _ := new(big.Rat).Quo(new(big.Rat).Sub(sum, total), total).Float64()
	if gap > 0.005 || gap < -0.005 {
		t.Fatalf("Σ balance equity × price differs from totalEq by %.4f%%", gap*100)
	}
}

func TestEmptyFieldsStayUnknown(t *testing.T) {
	t.Parallel()

	// Futures mode (acctLv 2) leaves the account-level margin figures empty; an empty string is
	// unknown, never zero.
	r, a := newReplay(t)
	r.always(epBalance, ok(`[{"totalEq":"100","adjEq":"","availEq":"","imr":"","mmr":"","mgnRatio":"","details":[
		{"ccy":"BTC","eq":"0.5","availEq":"","availBal":"0.5","liab":"","interest":"","upl":""},
		{"ccy":"ETH","eq":"-1","availEq":"0","availBal":"0","liab":"-1.2","interest":"0.01","upl":"0.2"},
		{"ccy":"DUST","eq":"0","availEq":"0","availBal":"0","liab":"0","interest":"0","upl":"0"}]}]`))
	r.always(epPositions, ok(`[]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	report := reportOf(t, batch, TypeTrading)
	if deref(report.Equity) != "100" || report.AdjustedEquity != nil || report.AvailableEquity != nil ||
		report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || report.UnifiedMarginRatio != nil {
		t.Fatalf("empty account figures must stay nil: %+v", report)
	}
	if len(report.Balances) != 2 {
		t.Fatalf("balances %+v; the all-zero DUST row must be dropped", report.Balances)
	}
	btc, eth := report.Balances[0], report.Balances[1]
	if btc.AvailableToTrade != nil || btc.Liability != nil || btc.Interest != nil || btc.UnrealizedPnl != nil || deref(btc.AvailableToWithdraw) != "0.5" {
		t.Fatalf("BTC balance %+v", btc)
	}
	if eth.Equity != "-1" || deref(eth.Liability) != "1.2" || deref(eth.Interest) != "0.01" {
		t.Fatalf("ETH balance %+v: a liability is stored as a positive amount", eth)
	}
}

func TestZeroAdjustedEquityLeavesMarginRatiosNull(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epBalance, ok(`[{"totalEq":"0","adjEq":"0","availEq":"0","imr":"0","mmr":"0","mgnRatio":"","details":[]}]`))
	r.always(epPositions, ok(`[]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	report := reportOf(t, batch, TypeTrading)
	if report.Status != adapters.StatusOK || report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || deref(report.Equity) != "0" {
		t.Fatalf("report %+v", report)
	}
}

func TestCollectMapsPositionsAndInstruments(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	report := reportOf(t, batch, TypeTrading)
	rows := rowsIn[openPosition](t, r.file(epPositions, ""))
	if len(report.Positions) != len(rows) || len(rows) == 0 {
		t.Fatalf("%d positions for %d recorded rows", len(report.Positions), len(rows))
	}
	for i := range rows {
		row := &rows[i]
		var p adapters.Position
		for _, candidate := range report.Positions {
			if candidate.Instrument.Symbol == row.InstID {
				p = candidate
			}
		}
		if p.PlatformPositionRef != row.InstID+":"+row.PosSide || p.QuantityUnit != adapters.UnitContracts {
			t.Fatalf("position %+v for row %+v", p, *row)
		}
		if rat(t, p.Quantity).Cmp(new(big.Rat).Abs(rat(t, row.Pos))) != 0 || p.Side != adapters.SideLong {
			t.Errorf("%s side %s quantity %s from pos %s", row.InstID, p.Side, p.Quantity, row.Pos)
		}
		if !sameDecimal(t, p.EntryPrice, row.AvgPx) || !sameDecimal(t, p.MarkPrice, row.MarkPx) || !sameDecimal(t, p.UnrealizedPnl, row.Upl) {
			t.Errorf("%s prices %+v", row.InstID, p)
		}
		// Recorded in portfolio margin cross: lever "0", liqPx and margin empty.
		if p.Leverage != nil || p.LiquidationPrice != nil || p.Margin != nil {
			t.Errorf("%s leverage %s, liquidation %s, margin %s must be nil", row.InstID, deref(p.Leverage), deref(p.LiquidationPrice), deref(p.Margin))
		}
	}
	doge := report.Positions[0].Instrument
	if doge.Symbol != "DOGE-USDT-SWAP" || doge.Kind != adapters.KindLinearPerp || doge.BaseSymbol != "DOGE" || doge.QuoteSymbol != "USDT" ||
		doge.SettlementSymbol != "USDT" || deref(doge.ContractSize) != "1000" || doge.ExpiryAt != nil {
		t.Fatalf("DOGE-USDT-SWAP instrument %+v", doge)
	}
	if r.count(epSwaps) != 1 || r.count(epFutures) != 0 {
		t.Fatalf("instrument lists fetched: SWAP %d, FUTURES %d; want only the list a position needs", r.count(epSwaps), r.count(epFutures))
	}
}

func TestInstrumentKinds(t *testing.T) {
	t.Parallel()

	listed := map[string]instrumentRow{}
	for _, ep := range []endpoint{epSwaps, epFutures} {
		for _, row := range rowsIn[instrumentRow](t, fixtureFile(t, ep, "")) {
			listed[row.InstID] = row
		}
	}
	for _, tc := range []struct {
		instID, kind, base, quote, settle, size string
		m                                       market
		expires                                 bool
	}{
		{"BTC-USDT-SWAP", adapters.KindLinearPerp, "BTC", "USDT", "USDT", "0.01", marketSwap, false},
		{"BTC-USD-SWAP", adapters.KindInversePerp, "BTC", "USD", "BTC", "100", marketSwap, false},
		{recordedFuture(t, "linear"), adapters.KindLinearDeliv, "BTC", "USD", "USD", "0.01", marketFutures, true},
		{recordedFuture(t, "inverse"), adapters.KindInverseDlv, "BTC", "USD", "BTC", "100", marketFutures, true},
	} {
		row, ok := listed[tc.instID]
		if !ok {
			t.Fatalf("%s is not in the recorded instruments", tc.instID)
		}
		got, ok := mapInstrument(tc.m, &row)
		if !ok || got.Kind != tc.kind || got.BaseSymbol != tc.base || got.QuoteSymbol != tc.quote || got.SettlementSymbol != tc.settle || deref(got.ContractSize) != tc.size {
			t.Errorf("%s mapped to %+v (size %s)", tc.instID, got, deref(got.ContractSize))
		}
		if tc.expires != (got.ExpiryAt != nil) {
			t.Errorf("%s expiry %v", tc.instID, got.ExpiryAt)
		}
		if tc.expires && got.ExpiryAt.UnixMilli() != mustMillis(t, row.ExpTime) {
			t.Errorf("%s expiry %v, want expTime %s", tc.instID, got.ExpiryAt, row.ExpTime)
		}
	}

	// Rows Plutus cannot store are skipped, so a position in them fails through lookup.
	for _, tc := range []struct {
		name string
		m    market
		row  instrumentRow
	}{
		{"pre-open listing without an id", marketFutures, instrumentRow{CtType: "linear", Uly: "AR-USD", CtVal: "1", ExpTime: "1948176000000"}},
		{"no contract type", marketSwap, instrumentRow{InstID: "X-USDT-SWAP", Uly: "X-USDT", CtVal: "1"}},
		{"future without a delivery time", marketFutures, instrumentRow{InstID: "X-USD-261225", CtType: "inverse", Uly: "X-USD", CtVal: "10"}},
	} {
		if got, ok := mapInstrument(tc.m, &tc.row); ok {
			t.Errorf("%s mapped to %+v", tc.name, got)
		}
	}
	multiplied, ok := mapInstrument(marketSwap, &instrumentRow{InstID: "X-USDT-SWAP", CtType: "linear", Uly: "X-USDT", CtVal: "0.1", CtMult: "10"})
	if !ok || deref(multiplied.ContractSize) != "1" {
		t.Fatalf("contract size is ctVal × ctMult: %+v", multiplied)
	}
}

// recordedFuture is the BTC delivery contract of a contract type that the recording's FUTURES
// sample holds. Futures expire, so every recording samples different ones.
func recordedFuture(t *testing.T, ctType string) string {
	t.Helper()
	for _, row := range rowsIn[instrumentRow](t, fixtureFile(t, epFutures, "")) {
		if row.CtType == ctType && strings.HasPrefix(row.InstID, "BTC-") {
			return row.InstID
		}
	}
	t.Fatalf("the recorded FUTURES sample has no %s BTC contract", ctType)
	return ""
}

func mustMillis(t *testing.T, value string) int64 {
	t.Helper()
	millis, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return millis
}

func TestPositionSidesAndSkippedRows(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	linear := recordedFuture(t, "linear")
	r.always(epPositions, ok(`[
		{"instType":"MARGIN","instId":"BTC-USDT","posSide":"net","pos":"0.5","mgnMode":"cross","posCcy":"BTC"},
		{"instType":"SWAP","instId":"BTC-USDT-SWAP","posSide":"net","pos":"0","mgnMode":"isolated"},
		{"instType":"SWAP","instId":"BTC-USD-SWAP","posSide":"net","pos":"-3","mgnMode":"cross","avgPx":"60000","markPx":"61000","liqPx":"","upl":"-0.00008","lever":"0","margin":""},
		{"instType":"FUTURES","instId":"`+linear+`","posSide":"short","pos":"2","mgnMode":"isolated","avgPx":"62000","markPx":"61000","liqPx":"65000","upl":"20","lever":"5","margin":"12.5"}]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	positions := reportOf(t, batch, TypeTrading).Positions
	if len(positions) != 2 {
		t.Fatalf("positions %+v; the MARGIN row and the zero-size row must be skipped", positions)
	}
	inverse, future := positions[0], positions[1]
	if inverse.Side != adapters.SideShort || inverse.Quantity != "3" || inverse.Instrument.Kind != adapters.KindInversePerp || inverse.PlatformPositionRef != "BTC-USD-SWAP:net" {
		t.Fatalf("net short position %+v", inverse)
	}
	if future.Side != adapters.SideShort || future.Quantity != "2" || deref(future.LiquidationPrice) != "65000" || deref(future.Leverage) != "5" ||
		deref(future.Margin) != "12.5" || future.Instrument.Kind != adapters.KindLinearDeliv || future.PlatformPositionRef != linear+":short" {
		t.Fatalf("long/short-mode short position %+v", future)
	}
}

func TestUnstorablePositionFailsTheAccount(t *testing.T) {
	t.Parallel()

	for _, instType := range []string{"OPTION", "EVENTS"} {
		t.Run(instType, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epPositions, ok(`[{"instType":"`+instType+`","instId":"BTC-USD-261225-100000-C","posSide":"net","pos":"1"}]`))
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading, TypeFunding})
			if err != nil {
				t.Fatal(err)
			}
			trading := reportOf(t, batch, TypeTrading)
			if trading.Status != adapters.StatusFailed || trading.ErrorClass != adapters.ClassNormalization ||
				!strings.Contains(trading.ErrorDetail, "BTC-USD-261225-100000-C") || !strings.Contains(trading.ErrorDetail, instType) {
				t.Fatalf("trading report %+v", trading)
			}
			if reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
				t.Fatal("funding must not fail with the trading account")
			}
		})
	}
}

func TestCollectMapsFundingAndEarn(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	funding := reportOf(t, batch, TypeFunding)
	rows := rowsIn[fundingBalance](t, r.file(epFunding, ""))
	if funding.Status != adapters.StatusOK || len(funding.Balances) != len(rows) || funding.Equity != nil {
		t.Fatalf("funding report %+v for %d rows", funding, len(rows))
	}
	for i, b := range funding.Balances {
		if b.Symbol != rows[i].Ccy || rat(t, b.Equity).Cmp(rat(t, rows[i].Bal)) != 0 || !sameDecimal(t, b.AvailableToWithdraw, rows[i].AvailBal) || b.AvailableToTrade != nil {
			t.Errorf("funding balance %+v for row %+v", b, rows[i])
		}
	}
	// A successful Earn report has also passed the Earn check against the recorded valuation.
	earn := reportOf(t, batch, TypeEarn)
	want := recordedOrdinaryEarn(t, r)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != len(want) || earn.Equity != nil {
		t.Fatalf("earn report %+v, want %v", earn, want)
	}
	for _, b := range earn.Balances {
		amount, ok := want[b.Symbol]
		if !ok || rat(t, b.Equity).Cmp(amount) != 0 || b.AvailableToTrade != nil || b.AvailableToWithdraw != nil || b.Liability != nil {
			t.Errorf("earn balance %+v, want %v", b, amount)
		}
	}
}

// recordedOrdinaryEarn sums Simple Earn and on-chain earn, excluding Dual principal.
func recordedOrdinaryEarn(t *testing.T, r *replay) map[string]*big.Rat {
	t.Helper()
	sums := map[string]*big.Rat{}
	add := func(ccy, amount string) {
		if sums[ccy] == nil {
			sums[ccy] = new(big.Rat)
		}
		sums[ccy].Add(sums[ccy], rat(t, amount))
	}
	for _, row := range rowsIn[savingsBalance](t, r.file(epSavings, "")) {
		add(row.Ccy, row.Amt)
	}
	for _, order := range rowsIn[stakingOrder](t, r.file(epStaking, "")) {
		for _, invested := range order.InvestData {
			add(invested.Ccy, invested.Amt)
		}
	}
	for ccy, amount := range sums {
		if amount.Sign() == 0 {
			delete(sums, ccy)
		}
	}
	return sums
}

// TestRecordedValuationCatchesUnreadEarn replays OKX's real valuation answer without the Earn
// Balances it values, so the check must read the recorded earn bucket to fail.
func TestRecordedValuationCatchesUnreadEarn(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	recorded := rat(t, rowsIn[struct {
		Details struct {
			Earn string `json:"earn"`
		} `json:"details"`
	}](t, r.file(epValuation, ""))[0].Details.Earn)
	if recorded.Cmp(big.NewRat(1, 1)) <= 0 {
		t.Skip("the recorded account holds no Earn value to leave out")
	}
	r.always(epSavings, ok(`[]`))
	r.always(epStaking, ok(`[]`))
	r.always(epDCD, ok(`[]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassUnsupported || !strings.Contains(earn.ErrorDetail, recorded.FloatString(2)+" USD") {
		t.Fatalf("earn report %+v; want unsupported naming the recorded %s USD", earn, recorded.FloatString(2))
	}
}

func TestEarnMergesSavingsAndOnChainOrders(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epSavings, ok(`[{"ccy":"USDT","amt":"100","loanAmt":"99","pendingAmt":"1"},{"ccy":"ETH","amt":"0"}]`))
	r.always(epStaking, ok(`[
		{"ordId":"1","ccy":"USDT","state":"1","investData":[{"ccy":"USDT","amt":"10.5"}]},
		{"ordId":"2","ccy":"SOL","state":"2","investData":[{"ccy":"SOL","amt":"2.5"}]}]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	balances := reportOf(t, batch, TypeEarn).Balances
	if len(balances) != 2 || balances[0].Symbol != "SOL" || balances[0].Equity != "2.5" || balances[1].Symbol != "USDT" || balances[1].Equity != "110.5" {
		t.Fatalf("earn balances %+v; want SOL 2.5 and USDT 100 + 10.5, and no zero ETH", balances)
	}
}

func TestCollectPrices(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if err != nil {
		t.Fatal(err)
	}
	last := map[string]string{}
	for _, row := range rowsIn[struct {
		InstID string `json:"instId"`
		Last   string `json:"last"`
	}](t, r.file(epTickers, "")) {
		last[row.InstID] = row.Last
	}
	held := map[string]bool{}
	for _, report := range batch.Reports {
		for _, b := range report.Balances {
			if !adapters.USDEquivalent(b.Symbol) {
				held[b.Symbol] = true
			}
		}
	}
	if len(batch.Prices) != len(held) || len(held) == 0 {
		t.Fatalf("prices %+v for held coins %v", batch.Prices, held)
	}
	for _, p := range batch.Prices {
		if !held[p.AssetSymbol] || p.QuoteSymbol != "USDT" || p.Price != last[p.AssetSymbol+"-USDT"] || p.ObservedAt.IsZero() || p.ObservedAt.After(batch.ObservedAt) {
			t.Errorf("price %+v", p)
		}
		for _, report := range batch.Reports {
			if p.ObservedAt.After(report.ObservedAt) {
				t.Errorf("price %s observed after the %s report", p.AssetSymbol, report.AccountType)
			}
		}
	}
}

func TestPricesFallBackToUSDC(t *testing.T) {
	t.Parallel()

	reports := []adapters.AccountReport{{Balances: []adapters.Balance{{Symbol: "AAA"}, {Symbol: "BBB"}, {Symbol: "CCC"}, {Symbol: "USDT"}, {Symbol: "USDC"}}}}
	tickers := map[string]string{"AAA-USDT": "2", "AAA-USDC": "2.1", "BBB-USDC": "3", "CCC-BTC": "4", "USDC-USDT": "0.9998"}
	prices := adapters.QuotePrices(reports, quotes, spotLookup(tickers), time.Unix(1, 0))
	if len(prices) != 2 || prices[0].AssetSymbol != "AAA" || prices[0].QuoteSymbol != "USDT" || prices[1].AssetSymbol != "BBB" || prices[1].QuoteSymbol != "USDC" {
		t.Fatalf("prices %+v", prices)
	}
}

func TestTickerFailureKeepsReports(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epTickers, status(http.StatusServiceUnavailable, ``))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Prices) != 0 {
		t.Fatalf("prices %+v after a ticker failure", batch.Prices)
	}
	for _, report := range batch.Reports {
		if report.Status != adapters.StatusOK {
			t.Fatalf("%s failed with the ticker: %+v", report.AccountType, report)
		}
	}
}

func TestErrorCodeInsideHTTP200IsAFailure(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epSavings, status(http.StatusOK, `{"code":"50011","msg":"Rate limit reached.","data":[]}`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if err != nil {
		t.Fatal(err)
	}
	if earn := reportOf(t, batch, TypeEarn); earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassRateLimited || !strings.Contains(earn.ErrorDetail, "50011") {
		t.Fatalf("earn report %+v", earn)
	}

	r, a = newReplay(t)
	r.always(epFunding, status(http.StatusOK, `{"code":"50120","msg":"This API key doesn't have permission to use this function","data":[]}`))
	_, err = a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if classOfErr(t, err) != adapters.ClassAuthorization || !adapters.StopsScheduling(adapters.ClassAuthorization) {
		t.Fatalf("permission error %v must fail the batch and stop scheduling", err)
	}
}

func TestAuthenticationFailsTheBatch(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"invalid_key", "bad_signature", "wrong_passphrase"} {
		for _, ep := range []endpoint{epConfig, epFunding} {
			t.Run(scenario+" on "+ep.path, func(t *testing.T) {
				t.Parallel()
				r, a := newReplay(t)
				r.always(ep, serve(r.file(epConfig, scenario)))
				_, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
				if classOfErr(t, err) != adapters.ClassAuthentication || !adapters.StopsScheduling(adapters.ClassAuthentication) {
					t.Fatalf("error %v", err)
				}
			})
		}
	}
}

func TestErrorDetailNamesOKXCodeOnly(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epConfig, serve(r.file(epConfig, "invalid_key")))
	_, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	_, detail := adapters.Classify(err)
	if !strings.Contains(detail, "GET /api/v5/account/config") || !strings.Contains(detail, "50119") {
		t.Fatalf("detail %q", detail)
	}
	if strings.Contains(detail, testKey) || strings.Contains(detail, testPassphrase) || strings.Contains(detail, "{") {
		t.Fatalf("detail %q echoes a credential or a body", detail)
	}
}

func TestStaleTimestampResyncsAndRetries(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.then(epBalance, serve(r.file(epConfig, "stale_timestamp")))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil {
		t.Fatal(err)
	}
	if reportOf(t, batch, TypeTrading).Status != adapters.StatusOK {
		t.Fatal("the retried call must succeed")
	}
	if r.count(epTime) != 2 || r.count(epBalance) != 2 {
		t.Fatalf("server time %d, balance %d calls; want one resync and one retry", r.count(epTime), r.count(epBalance))
	}
}

func TestErrorClasses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		body   string
		class  string
	}{
		{http.StatusUnauthorized, `{"code":"50111","msg":"Invalid OK-ACCESS-KEY."}`, adapters.ClassAuthentication},
		{http.StatusUnauthorized, `{"code":"50110","msg":"Your IP 203.0.113.1 is not included in your API key's IP whitelist."}`, adapters.ClassAuthentication},
		{http.StatusUnauthorized, `{"code":"50101","msg":"APIKey does not match current environment."}`, adapters.ClassAuthentication},
		{http.StatusForbidden, `{"code":"50035","msg":"This endpoint requires that APIKey must be bound to IP"}`, adapters.ClassAuthentication},
		{http.StatusOK, `{"code":"50030","msg":"You don't have permission to use this API endpoint"}`, adapters.ClassAuthorization},
		{http.StatusOK, `{"code":"59500","msg":"Only the API key of the main account has permission."}`, adapters.ClassAuthorization},
		{http.StatusTooManyRequests, `{"code":"50011","msg":"Too Many Requests"}`, adapters.ClassRateLimited},
		{http.StatusTooManyRequests, `{"code":"50013","msg":"Systems are busy. Please try again later."}`, adapters.ClassRateLimited},
		{http.StatusOK, `{"code":"50040","msg":"Too frequent operations, please try again later"}`, adapters.ClassRateLimited},
		{http.StatusTooManyRequests, ``, adapters.ClassRateLimited},
		{http.StatusServiceUnavailable, `{"code":"50001","msg":"Service temporarily unavailable. Please try again later."}`, adapters.ClassTemporary},
		{http.StatusBadRequest, `{"code":"50004","msg":"API endpoint request timeout."}`, adapters.ClassTemporary},
		{http.StatusOK, `{"code":"51767","msg":"System maintenance, please retry"}`, adapters.ClassTemporary},
		{http.StatusOK, `{"code":"51008_1000","msg":"Order failed."}`, adapters.ClassTemporary},
		{http.StatusInternalServerError, ``, adapters.ClassTemporary},
		{http.StatusForbidden, `<html>Forbidden</html>`, adapters.ClassTemporary},
		{http.StatusUnauthorized, ``, adapters.ClassAuthentication},
		// The dual investment endpoints send the code as a number.
		{http.StatusOK, `{"code":50011,"msg":"Too Many Requests","data":[]}`, adapters.ClassRateLimited},
		{http.StatusOK, `{"code":50120,"msg":"no permission","data":[]}`, adapters.ClassAuthorization},
	} {
		err := decode(epBalance, response{status: tc.status, body: []byte(tc.body)}, &json.RawMessage{})
		if got := classOfErr(t, err); got != tc.class {
			t.Errorf("HTTP %d %s: class %s, want %s", tc.status, tc.body, got, tc.class)
		}
	}
}

func TestNumericAnswerCodeIsDecoded(t *testing.T) {
	t.Parallel()

	var rows []dcdOrder
	if err := decode(epDCD, response{status: http.StatusOK, body: []byte(`{"code":0,"msg":"","data":[]}`)}, &rows); err != nil || len(rows) != 0 {
		t.Fatalf("a numeric success code: %v %v", err, rows)
	}
	err := decode(epDCD, response{status: http.StatusOK, body: []byte(`{"code":{},"msg":"","data":[]}`)}, &rows)
	if classOfErr(t, err) != adapters.ClassNormalization {
		t.Fatalf("a code that is neither a string nor a number: %v", err)
	}
}

func TestOneFailingAccountLeavesSiblingsOK(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epSavings, status(http.StatusInternalServerError, `{"code":"50026","msg":"System error. Try again later","data":[]}`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, allTypes)
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassTemporary || !strings.Contains(earn.ErrorDetail, "50026") {
		t.Fatalf("earn report %+v", earn)
	}
	for _, sibling := range []adapters.AccountType{TypeTrading, TypeFunding} {
		if reportOf(t, batch, sibling).Status != adapters.StatusOK {
			t.Fatalf("%s failed with earn", sibling)
		}
	}
}

func TestEveryAccountRetryableFailureFailsTheBatch(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	for _, ep := range []endpoint{epBalance, epFunding} {
		r.always(ep, status(http.StatusServiceUnavailable, ``))
	}
	_, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading, TypeFunding})
	if classOfErr(t, err) != adapters.ClassTemporary {
		t.Fatalf("error %v: the collection service retries only a batch error", err)
	}
}

func TestUnknownInstrumentFailsTheAccount(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epPositions, ok(`[{"instType":"SWAP","instId":"NOPE-USDT-SWAP","posSide":"net","pos":"2","avgPx":"1","markPx":"1"}]`))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading, TypeFunding})
	if err != nil {
		t.Fatal(err)
	}
	trading := reportOf(t, batch, TypeTrading)
	if trading.Status != adapters.StatusFailed || trading.ErrorClass != adapters.ClassNormalization || !strings.Contains(trading.ErrorDetail, "NOPE-USDT-SWAP") {
		t.Fatalf("trading report %+v", trading)
	}
	if reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatal("funding must not fail with an unknown contract")
	}
}

func TestNonDecimalFieldIsNormalizationFailure(t *testing.T) {
	t.Parallel()

	for name, answer := range map[string]string{
		"account total": `[{"totalEq":"n/a","adjEq":"1","availEq":"1","imr":"0","mmr":"0","mgnRatio":"","details":[]}]`,
		"missing total": `[{"totalEq":"","adjEq":"1","availEq":"1","imr":"0","mmr":"0","mgnRatio":"","details":[]}]`,
		"coin equity":   `[{"totalEq":"1","adjEq":"1","availEq":"1","imr":"0","mmr":"0","mgnRatio":"","details":[{"ccy":"BTC","eq":"1e-3"}]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epBalance, ok(answer))
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading, TypeFunding})
			if err != nil {
				t.Fatal(err)
			}
			if trading := reportOf(t, batch, TypeTrading); trading.Status != adapters.StatusFailed || trading.ErrorClass != adapters.ClassNormalization {
				t.Fatalf("trading report %+v", trading)
			}
		})
	}
}

func TestEmptyAmountFailsItsAccount(t *testing.T) {
	t.Parallel()

	// An empty amount is unknown. Read as zero it would drop the coin under the Complete Set rule,
	// so it fails its Account, naming the coin, and leaves the siblings alone.
	for name, tc := range map[string]struct {
		ep      endpoint
		answer  string
		account adapters.AccountType
		coin    string
	}{
		"trading eq":        {epBalance, `[{"totalEq":"1","adjEq":"1","availEq":"1","imr":"0","mmr":"0","mgnRatio":"","details":[{"ccy":"BTC","eq":"","availBal":"0.5"}]}]`, TypeTrading, "BTC"},
		"funding bal":       {epFunding, `[{"ccy":"ENS","bal":"","availBal":"0.5"}]`, TypeFunding, "ENS"},
		"savings amt":       {epSavings, `[{"ccy":"USDC","amt":""}]`, TypeEarn, "USDC"},
		"on-chain earn amt": {epStaking, `[{"ordId":"1","ccy":"SOL","investData":[{"ccy":"SOL","amt":""}]}]`, TypeEarn, "SOL"},
		"dual investment":   {epDCD, `[{"ordId":"1","state":"live","notionalSz":"","notionalCcy":"ETH"}]`, TypeEarn, "ETH"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(tc.ep, ok(tc.answer))
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading, TypeFunding, TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			for _, report := range batch.Reports {
				if report.AccountType != tc.account {
					if report.Status != adapters.StatusOK {
						t.Errorf("%s report %+v must not fail", report.AccountType, report)
					}
					continue
				}
				if report.Status != adapters.StatusFailed || report.ErrorClass != adapters.ClassNormalization || !strings.Contains(report.ErrorDetail, tc.coin) {
					t.Errorf("%s report %+v, want a normalization failure naming %s", report.AccountType, report, tc.coin)
				}
			}
		})
	}
}

func TestSpotModeMapsBorrowingFigures(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "okx_spot")
	result, err := a.Validate(context.Background(), testCreds, adapters.StructureOKXUnified)
	if err != nil || result.MarginMode != adapters.MarginOKXSpot {
		t.Fatalf("validate: %v %+v", err, result)
	}
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil || batch.MarginMode != adapters.MarginOKXSpot {
		t.Fatalf("collect: %v, margin mode %q", err, batch.MarginMode)
	}
	report := reportOf(t, batch, TypeTrading)
	account := rowsIn[tradingAccount](t, r.file(epBalance, ""))[0]
	// Spot mode reports the adjusted equity, the margin requirements of its borrowings, and the
	// margin ratio, but no account-level available equity.
	if !sameDecimal(t, report.Equity, account.TotalEq) || !sameDecimal(t, report.AdjustedEquity, account.AdjEq) ||
		!sameDecimal(t, report.UnifiedMarginRatio, account.MgnRatio) || report.AvailableEquity != nil {
		t.Fatalf("account figures %+v", report)
	}
	if deref(report.InitialMarginRatio) != decimal.Compact(new(big.Rat).Quo(rat(t, account.IMR), rat(t, account.AdjEq)).FloatString(8)) ||
		deref(report.MaintenanceMarginRatio) != decimal.Compact(new(big.Rat).Quo(rat(t, account.MMR), rat(t, account.AdjEq)).FloatString(8)) {
		t.Fatalf("margin ratios %s, %s", deref(report.InitialMarginRatio), deref(report.MaintenanceMarginRatio))
	}
	if len(report.Balances) != len(account.Details) || len(report.Positions) != 0 {
		t.Fatalf("balances %+v, positions %+v", report.Balances, report.Positions)
	}
	for _, b := range report.Balances {
		if b.AvailableToTrade != nil || b.UnrealizedPnl != nil || b.AvailableToWithdraw == nil {
			t.Errorf("%s: Spot mode reports no per-coin available equity or PnL: %+v", b.Symbol, b)
		}
		if b.Symbol == "USDT" && (deref(b.Liability) != "100.0125" || deref(b.Interest) != "0.0125") {
			t.Errorf("the USDT borrowing is %s with interest %s", deref(b.Liability), deref(b.Interest))
		}
	}
}

func TestFuturesModeLeavesAccountMarginFiguresUnknown(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "okx_futures")
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeTrading})
	if err != nil || batch.MarginMode != adapters.MarginOKXFutures {
		t.Fatalf("collect: %v, margin mode %q", err, batch.MarginMode)
	}
	report := reportOf(t, batch, TypeTrading)
	account := rowsIn[tradingAccount](t, r.file(epBalance, ""))[0]
	// Futures mode reports only totalEq at account level; margin is assessed per coin.
	if !sameDecimal(t, report.Equity, account.TotalEq) || report.AdjustedEquity != nil || report.AvailableEquity != nil ||
		report.InitialMarginRatio != nil || report.MaintenanceMarginRatio != nil || report.UnifiedMarginRatio != nil {
		t.Fatalf("account figures %+v", report)
	}
	if len(report.Balances) != len(account.Details) {
		t.Fatalf("balances %+v", report.Balances)
	}
	for _, b := range report.Balances {
		// Borrowings live on the skipped MARGIN position rows: no liability or interest per coin.
		if b.AvailableToTrade == nil || b.UnrealizedPnl == nil || b.Liability != nil || b.Interest != nil {
			t.Errorf("%s: %+v", b.Symbol, b)
		}
	}
	if len(report.Positions) != 1 {
		t.Fatalf("positions %+v; the MARGIN row must be skipped", report.Positions)
	}
	swap := report.Positions[0]
	if swap.Instrument.Symbol != "BTC-USDT-SWAP" || swap.PlatformPositionRef != "BTC-USDT-SWAP:long" || swap.Side != adapters.SideLong ||
		swap.Quantity != "2" || deref(swap.Leverage) != "10" || deref(swap.LiquidationPrice) != "31012.5" || swap.Margin != nil {
		t.Fatalf("cross swap position %+v", swap)
	}
}

// earnWithout empties Simple Earn and on-chain earn so a test sees only dual investment, and
// answers an empty Earn valuation, which the Earn check passes whatever the orders hold.
func (r *replay) earnWithout() {
	r.always(epSavings, ok(`[]`))
	r.always(epStaking, ok(`[]`))
	r.always(epValuation, valuation("0"))
}

// dcdOrders is a page of dual investment orders, one per id, all in state with 1 of ccy.
func dcdOrders(state, ccy string, ids ...int) string {
	rows := make([]string, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, fmt.Sprintf(`{"ordId":"%d","state":"%s","notionalSz":"1","notionalCcy":"%s"}`, id, state, ccy))
	}
	return "[" + strings.Join(rows, ",") + "]"
}

// descending lists count ids from newest down.
func descending(newest, count int) []int {
	out := make([]int, count)
	for i := range out {
		out[i] = newest - i
	}
	return out
}

func TestDualInvestmentCountsInvestedPrincipal(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t, "dual_investment")
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK {
		t.Fatalf("earn report %+v", earn)
	}
	want := map[string]*big.Rat{}
	add := func(ccy, amount string) { want[ccy] = decimal.Add(want[ccy], rat(t, amount)) }
	settled := 0
	for _, order := range rowsIn[dcdOrder](t, r.file(epDCD, "")) {
		if dcdInvested[order.State] {
			add(order.NotionalCcy, order.NotionalSz)
		} else {
			settled++
		}
	}
	if settled == 0 {
		t.Fatal("the scenario must include an order that has paid its principal back")
	}
	got := map[string]string{}
	for _, b := range earn.Balances {
		got[b.Symbol] = b.Equity
	}
	for ccy, amount := range want {
		if amount.Sign() != 0 && (got[ccy] == "" || rat(t, got[ccy]).Cmp(amount) != 0) {
			t.Errorf("%s: earn %s, want %s", ccy, got[ccy], decimal.Format(amount))
		}
	}
	if len(got) != len(want) {
		t.Errorf("earn balances %v, want %v", got, want)
	}
}

func TestDualInvestmentStates(t *testing.T) {
	t.Parallel()

	var rows []string
	for state := range dcdInvested {
		for _, form := range []string{state, strings.ToUpper(state), strings.ToUpper(state[:1]) + state[1:]} {
			rows = append(rows, fmt.Sprintf(`{"ordId":"%d","state":"%s","notionalSz":"1","notionalCcy":"%s"}`,
				1000-len(rows), form, strings.ToUpper(strings.ReplaceAll(state, "_", ""))))
		}
	}
	r, a := newReplay(t)
	r.earnWithout()
	r.always(epDCD, ok("["+strings.Join(rows, ",")+"]"))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK {
		t.Fatalf("earn report %+v", earn)
	}
	got := map[string]string{}
	for _, b := range earn.Balances {
		got[b.Symbol] = b.Equity
	}
	for state, invested := range dcdInvested {
		coin := strings.ToUpper(strings.ReplaceAll(state, "_", ""))
		if invested && got[coin] != "3" || !invested && got[coin] != "" {
			t.Errorf("state %s counted %q %s, invested %v", state, got[coin], coin, invested)
		}
	}

	r, a = newReplay(t)
	r.always(epDCD, ok(dcdOrders("ExPiReD", "BTC", 7)))
	batch, err = a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	if earn := reportOf(t, batch, TypeDualInvestment); earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassNormalization || !strings.Contains(earn.ErrorDetail, `"ExPiReD"`) {
		t.Fatalf("an undocumented state must fail Earn by name: %+v", earn)
	}
	if reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
		t.Fatal("funding must not fail with Earn")
	}
}

func TestDualInvestmentReadsEveryPage(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.earnWithout()
	r.always(epDCD, ok(dcdOrders("live", "USDT", descending(1000, dcdPageSize)...)))
	r.always(dcdPage("901"), ok(dcdOrders("live", "USDT", descending(900, 3)...)))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeDualInvestment})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeDualInvestment)
	if earn.Status != adapters.StatusOK || len(earn.Balances) != 1 || earn.Balances[0].Equity != "103" {
		t.Fatalf("earn report %+v; want every order of both pages", earn)
	}
	if r.count(epDCD) != 1 || r.count(dcdPage("901")) != 1 {
		t.Fatalf("pages read: first %d, second %d", r.count(epDCD), r.count(dcdPage("901")))
	}
}

func TestDualInvestmentHistoryMustBeNewestFirst(t *testing.T) {
	t.Parallel()

	for name, page := range map[string]string{
		"ascending":       dcdOrders("live", "USDT", 1, 2),
		"repeated":        dcdOrders("live", "USDT", 5, 5),
		"missing id":      `[{"ordId":"","state":"live","notionalSz":"1","notionalCcy":"USDT"}]`,
		"not above page1": "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			if page == "" {
				// The second page repeats an order at or above the first page's oldest.
				r.always(epDCD, ok(dcdOrders("live", "USDT", descending(1000, dcdPageSize)...)))
				r.always(dcdPage("901"), ok(dcdOrders("live", "USDT", 901)))
			} else {
				r.always(epDCD, ok(page))
			}
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			if earn := reportOf(t, batch, TypeEarn); earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassNormalization {
				t.Fatalf("earn report %+v", earn)
			}
		})
	}
}

func TestDualInvestmentHistoryBeyondTheLimitFailsEarn(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	before, newest := "", 10000
	for page := 0; page < dcdMaxPages; page++ {
		r.always(dcdPage(before), ok(dcdOrders("settled", "USDT", descending(newest, dcdPageSize)...)))
		newest -= dcdPageSize
		before = strconv.Itoa(newest + 1)
	}
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	earn := reportOf(t, batch, TypeEarn)
	if earn.Status != adapters.StatusFailed || earn.ErrorClass != adapters.ClassUnsupported || !strings.Contains(earn.ErrorDetail, "1000 orders") {
		t.Fatalf("earn report %+v", earn)
	}
	if r.count(dcdPage(before)) != 0 {
		t.Fatal("the walk read past its limit")
	}
}

func TestDualInvestmentPagesWaitForTheRateLimit(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.earnWithout()
	r.always(epDCD, ok(dcdOrders("live", "USDT", descending(1000, dcdPageSize)...)))
	r.always(dcdPage("901"), ok(`[]`))
	a.pace = 50 * time.Millisecond
	started := time.Now()
	if _, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < a.pace {
		t.Fatalf("two pages %s apart, want at least %s", elapsed, a.pace)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled run must stop waiting: %v", err)
	}
}

// valuation answers asset-valuation with an Earn bucket of earn USD.
func valuation(earn string) func(w http.ResponseWriter) {
	return ok(`[{"details":{"classic":"0","earn":"` + earn + `","funding":"0","trading":"0"},"totalBal":"` + earn + `","ts":"1"}]`)
}

func TestEarnCheckComparesWithOKXValuation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		earn string
		ok   bool
	}{
		{"100", true},
		{"101.01", true}, // within max(1% of OKX's figure, 1 USD)
		{"101.5", false},
		{"99", true}, // Plutus may itemize more than OKX values, such as a dual investment order OKX books elsewhere
		{"150", false},
	} {
		t.Run(tc.earn, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epSavings, ok(`[{"ccy":"USDT","amt":"60"},{"ccy":"USDC","amt":"40"}]`))
			r.always(epStaking, ok(`[]`))
			r.always(epDCD, ok(`[]`))
			r.always(epValuation, valuation(tc.earn))
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			earn := reportOf(t, batch, TypeEarn)
			if tc.ok != (earn.Status == adapters.StatusOK) {
				t.Fatalf("OKX values Earn at %s against 100 itemized: %+v", tc.earn, earn)
			}
			if !tc.ok && (earn.ErrorClass != adapters.ClassUnsupported || !strings.Contains(earn.ErrorDetail, "100.00 USD")) {
				t.Fatalf("the failure must name both amounts: %+v", earn)
			}
			if reportOf(t, batch, TypeFunding).Status != adapters.StatusOK {
				t.Fatal("funding must not fail with the Earn check")
			}
		})
	}
}

func TestEarnCheckNeedsTheValuation(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		answer func(w http.ResponseWriter)
		class  string
	}{
		"no earn bucket": {ok(`[{"details":{"trading":"1"},"totalBal":"1","ts":"1"}]`), adapters.ClassNormalization},
		"no rows":        {ok(`[]`), adapters.ClassNormalization},
		"not a decimal":  {valuation("n/a"), adapters.ClassNormalization},
		"rate limited":   {status(http.StatusTooManyRequests, `{"code":"50011","msg":"Too Many Requests","data":[]}`), adapters.ClassRateLimited},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, a := newReplay(t)
			r.always(epValuation, tc.answer)
			batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeFunding, TypeEarn})
			if err != nil {
				t.Fatal(err)
			}
			if earn := reportOf(t, batch, TypeEarn); earn.Status != adapters.StatusFailed || earn.ErrorClass != tc.class {
				t.Fatalf("earn report %+v, want %s", earn, tc.class)
			}
		})
	}
}

func TestEarnCheckIsSkippedWithoutPrices(t *testing.T) {
	t.Parallel()

	r, a := newReplay(t)
	r.always(epSavings, ok(`[{"ccy":"BTC","amt":"1"}]`))
	r.always(epStaking, ok(`[]`))
	r.always(epTickers, status(http.StatusServiceUnavailable, ``))
	r.always(epValuation, valuation("1000000"))
	batch, err := a.Collect(context.Background(), testCreds, adapters.StructureOKXUnified, []adapters.AccountType{TypeEarn})
	if err != nil {
		t.Fatal(err)
	}
	if earn := reportOf(t, batch, TypeEarn); earn.Status != adapters.StatusOK {
		t.Fatalf("an unpriced Balance cannot be compared, so the check must pass: %+v", earn)
	}
}
