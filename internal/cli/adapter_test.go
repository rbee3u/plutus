package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/recording"
)

func runRecord(args ...string) (string, error) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"adapter", "record"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

// Each recording test owns a unique output directory and removes only its files.
func recordingTestDirectory(t *testing.T) string {
	t.Helper()
	if err := checkRecordingDirectory(recordingDirectory); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(recordingDirectory, "record-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return dir
}

func TestRecordingDefaultDirectory(t *testing.T) {
	cmd := newAdapterRecordCommand()
	if got := cmd.Flags().Lookup("out").DefValue; got != "/tmp/plutus/evidence" {
		t.Fatalf("default output: %s", got)
	}
}

func TestListAndDryRunNeedNeitherCredentialsNorNetwork(t *testing.T) {
	out, err := runRecord("--platform", "binance", "--list-requests")
	if err != nil || !strings.Contains(out, "sapi_v1_simple_earn_locked_position:") {
		t.Fatalf("list: %s %v", out, err)
	}
	out, err = runRecord("--platform", "binance", "--only", "sapi_v1_simple_earn_locked_position", "--variant", "holding", "--query", "current=2", "--dry-run")
	if err != nil || !strings.Contains(out, "current=2") || !strings.Contains(out, "sapi_v1_simple_earn_locked_position.holding.json") {
		t.Fatalf("plan: %s %v", out, err)
	}
}

func TestRecorderUsesRecordingNamespaceAndSelectedKeyGroup(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config", 0o750); err != nil {
		t.Fatal(err)
	}
	// A broken test configuration must not affect private recording credential loading.
	if err := os.WriteFile("config/gotest.env", []byte("not-an-assignment\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		platform adapters.Platform
		request  string
		fields   []string
	}{
		{adapters.PlatformBinance, "papi_v1_balance", []string{"BINANCE_APIKEY", "BINANCE_SECRET"}},
		{adapters.PlatformOKX, "api_v5_account_balance", []string{"OKX_APIKEY", "OKX_SECRET", "OKX_PASSPHRASE"}},
		{adapters.PlatformBybit, "v5_account_info", []string{"BYBIT_APIKEY", "BYBIT_SECRET"}},
		{adapters.PlatformKraken, "0_private_GetApiKeyInfo", []string{"KRAKEN_SPOT_APIKEY", "KRAKEN_SPOT_SECRET"}},
		{adapters.PlatformKraken, "ws_v1_balances_snapshot", []string{"KRAKEN_FUTURES_APIKEY", "KRAKEN_FUTURES_SECRET"}},
	} {
		t.Run(tc.request, func(t *testing.T) {
			for _, field := range tc.fields {
				t.Setenv("PLUTUS_RECORD_"+field, "synthetic-recording-value")
			}
			// Supplying only one Kraken group must suffice; the other group is empty.
			if tc.platform == adapters.PlatformKraken {
				other := "SPOT"
				if tc.request == "0_private_GetApiKeyInfo" {
					other = "FUTURES"
				}
				for _, suffix := range []string{"APIKEY", "SECRET"} {
					t.Setenv("PLUTUS_RECORD_KRAKEN_"+other+"_"+suffix, "")
				}
			}
			spec := recorders[tc.platform]
			catalogue := spec.requests()
			request, err := recording.SelectRequest(catalogue, tc.request, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			creds, err := recorderCredentials(tc.platform, spec.needPassphrase, catalogue, request)
			if err != nil {
				t.Fatal(err)
			}
			if tc.request == "ws_v1_balances_snapshot" {
				if creds.Futures == nil || creds.Futures.APIKey != "synthetic-recording-value" || creds.Futures.APISecret != "synthetic-recording-value" || creds.APIKey != "" {
					t.Fatal("wrong Futures credential group")
				}
			} else if creds.APIKey != "synthetic-recording-value" || creds.APISecret != "synthetic-recording-value" || creds.Futures != nil {
				t.Fatal("wrong Spot credential group")
			}
			if spec.needPassphrase && creds.Passphrase != "synthetic-recording-value" {
				t.Fatal("missing recording passphrase")
			}
		})
	}
}

func TestRecorderDoesNotFallBackToOldTestKeys(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config/gotest.env", []byte("PLUTUS_GOTEST_BINANCE_APIKEY=legacy-key\nPLUTUS_GOTEST_BINANCE_SECRET=legacy-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLUTUS_GOTEST_BINANCE_APIKEY", "legacy-process-key")
	t.Setenv("PLUTUS_GOTEST_BINANCE_SECRET", "legacy-process-secret")
	t.Setenv("PLUTUS_RECORD_BINANCE_APIKEY", "")
	t.Setenv("PLUTUS_RECORD_BINANCE_SECRET", "")
	catalogue := recorders[adapters.PlatformBinance].requests()
	request, err := recording.SelectRequest(catalogue, "papi_v1_balance", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = recorderCredentials(adapters.PlatformBinance, false, catalogue, request)
	if err == nil || !strings.Contains(err.Error(), "config/record.env") || strings.Contains(err.Error(), "legacy-") {
		t.Fatal("legacy test keys were accepted or disclosed")
	}
}

func TestPublicRecordingIgnoresInvalidCredentialFiles(t *testing.T) {
	outDir := recordingTestDirectory(t)
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config", 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gotest.env", "record.env"} {
		if err := os.WriteFile(filepath.Join("config", name), []byte("not-an-assignment\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	original := recorders[adapters.PlatformBinance]
	stub := &recordingStub{}
	changed := original
	changed.adapter = func(_ func(time.Duration) *http.Client) recorder { return stub }
	recorders[adapters.PlatformBinance] = changed
	t.Cleanup(func() { recorders[adapters.PlatformBinance] = original })
	if _, err := runRecord("--platform", "binance", "--only", "api_v3_time", "--out", outDir); err != nil || stub.calls != 1 {
		t.Fatalf("public recording accessed credential files: %v", err)
	}
	if _, err := runRecord("--platform", "binance", "--only", "papi_v1_balance", "--dry-run"); err != nil || stub.calls != 1 {
		t.Fatalf("dry run accessed credentials or the recorder: %v", err)
	}
}

func TestInvalidSelectionFailsBeforeKeys(t *testing.T) {
	cases := [][]string{
		{},
		{"--only", "unknown"},
		{"--only", "api_v3_time,api_v3_account"},
		{"--only", "api_v3_time", "--only", "api_v3_account"},
		{"--only", "api_v3_time", "--variant", "../pm"},
		{"--only", "api_v3_time", "--query", "signature=secret"},
		{"--only", "sapi_v1_dci_product_positions"},
		{"--scenario", "capture_pm"},
		{"--full"},
		{"--include-refusals"},
	}
	for _, args := range cases {
		_, err := runRecord(append([]string{"--platform", "binance"}, args...)...)
		if err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	_, err := runRecord("--platform", "bybit", "--only", "v5_position_list", "--query", "category=linear", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "settleCoin") {
		t.Fatalf("missing required query: %v", err)
	}
	_, err = runRecord("--platform", "okx", "--only", "api_v5_public_futures_contracts", "--dry-run")
	if err == nil {
		t.Fatal("missing futures contract type accepted")
	}
}

type recordingStub struct {
	calls       int
	failure     error
	keepSymbols []string
}

func (s *recordingStub) RecordRequest(_ context.Context, _ adapters.Credentials, request fixture.Request, options ...recording.Options) (*fixture.File, error) {
	s.calls++
	if len(options) > 0 {
		s.keepSymbols = options[0].KeepSymbols
	}
	now := time.Now().UTC()
	return &fixture.File{Source: "recorded", RecordedAt: &now, Request: request, Status: 200, Body: []byte(`{"serverTime":1700000000000}`)}, s.failure
}

func TestRecordingWritesOnlyOneReviewableFile(t *testing.T) {
	outDir := recordingTestDirectory(t)
	t.Chdir(t.TempDir())
	spec := recorders[adapters.PlatformBinance]
	stub := &recordingStub{}
	changed := spec
	changed.adapter = func(_ func(time.Duration) *http.Client) recorder { return stub }
	recorders[adapters.PlatformBinance] = changed
	t.Cleanup(func() { recorders[adapters.PlatformBinance] = spec })
	for _, failure := range []error{nil, errors.New("platform failure")} {
		stub.failure = failure
		out, err := runRecord("--platform", "binance", "--only", "api_v3_time", "--keep-symbol", "DOGEUSDT", "--out", outDir)
		if (err != nil) != (failure != nil) || !strings.Contains(out, "business mapping has not been accepted") {
			t.Fatalf("status: %s %v", out, err)
		}
	}
	if len(stub.keepSymbols) != 1 || stub.keepSymbols[0] != "DOGEUSDT" {
		t.Fatal("local selection was not passed to the recorder")
	}
	if stub.calls != 2 {
		t.Fatal("unexpected recorder calls")
	}
	paths, err := filepath.Glob(filepath.Join(outDir, "binance", "*", "*"))
	if err != nil || len(paths) != 2 {
		t.Fatalf("output: %v %v", paths, err)
	}
	for _, path := range paths {
		if filepath.Base(path) != "api_v3_time.json" {
			t.Fatalf("extra output %s", path)
		}
		if _, err := fixture.Read(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordingOutputCannotEscapeTmp(t *testing.T) {
	root := recordingTestDirectory(t)
	t.Chdir(t.TempDir())
	for _, path := range []string{"tmp/evidence", "elsewhere", "/tmp/plutus-other/evidence", filepath.Join(recordingDirectory, "..", "elsewhere")} {
		if err := checkRecordingDirectory(path); err == nil {
			t.Fatalf("outside /tmp/plutus accepted: %s", path)
		}
	}
	if err := checkRecordingDirectory(filepath.Join(root, "nested", "evidence")); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := checkRecordingDirectory(filepath.Join(link, "new")); err == nil {
		t.Fatal("symlink escape accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatalf("symlink escape created an outside directory: %v", err)
	}
}

func TestRecordingPlatformDirectoryCannotEscapeTmp(t *testing.T) {
	outDir := recordingTestDirectory(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(outDir, "binance")); err != nil {
		t.Fatal(err)
	}
	original := recorders[adapters.PlatformBinance]
	changed := original
	changed.adapter = func(_ func(time.Duration) *http.Client) recorder { return &recordingStub{} }
	recorders[adapters.PlatformBinance] = changed
	t.Cleanup(func() { recorders[adapters.PlatformBinance] = original })
	if _, err := runRecord("--platform", "binance", "--only", "api_v3_time", "--out", outDir); err == nil {
		t.Fatal("platform symlink escape accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("recording escaped output directory: %v %v", entries, err)
	}
}

type recordingFailureStub struct {
	unsafe, disk bool
	outDir       string
}

func (s recordingFailureStub) RecordRequest(_ context.Context, _ adapters.Credentials, request fixture.Request, _ ...recording.Options) (*fixture.File, error) {
	if s.unsafe {
		return nil, errors.New("redaction failed")
	}
	if s.disk {
		if err := os.WriteFile(filepath.Join(s.outDir, "binance"), []byte("blocked output parent"), 0o600); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	return &fixture.File{Source: "recorded", RecordedAt: &now, Request: request, Status: 200, Body: []byte(`[]`)}, nil
}

func TestFailuresAndEmptyResponsesCannotPromoteSamples(t *testing.T) {
	for _, mode := range []string{"empty", "unsafe", "disk"} {
		t.Run(mode, func(t *testing.T) {
			outDir := recordingTestDirectory(t)
			t.Chdir(t.TempDir())
			if err := os.Mkdir("testdata", 0o750); err != nil {
				t.Fatal(err)
			}
			const sentinel = `{"existing":"reviewed sample"}`
			if err := os.WriteFile("testdata/api_v3_time.json", []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}
			original := recorders[adapters.PlatformBinance]
			changed := original
			changed.adapter = func(_ func(time.Duration) *http.Client) recorder {
				return recordingFailureStub{unsafe: mode == "unsafe", disk: mode == "disk", outDir: outDir}
			}
			recorders[adapters.PlatformBinance] = changed
			t.Cleanup(func() { recorders[adapters.PlatformBinance] = original })
			out, err := runRecord("--platform", "binance", "--only", "api_v3_time", "--out", outDir)
			if (err == nil) != (mode == "empty") {
				t.Fatalf("unexpected result: %s %v", out, err)
			}
			if mode == "empty" && !strings.Contains(out, "empty: true") {
				t.Fatal("empty state not reported")
			}
			files, e := filepath.Glob(filepath.Join(outDir, "binance", "*", "*.json"))
			if e != nil {
				t.Fatal(e)
			}
			if mode != "empty" && len(files) != 0 {
				t.Fatal("failure exposed a sample")
			}
			raw, e := os.ReadFile("testdata/api_v3_time.json")
			if e != nil || string(raw) != sentinel {
				t.Fatal("sample promoted without review")
			}
		})
	}
}

func TestRecordingIgnoresRuntimeProxyConfiguration(t *testing.T) {
	// Both inherited runtime config flags must remain irrelevant to recording's offline commands.
	t.Setenv("PLUTUS_COLLECTION_PROXY_MODE", "invalid")
	t.Setenv("PLUTUS_COLLECTION_PROXY_URL", "ss://unused.invalid:1")
	for _, flags := range [][]string{{"--list-requests"}, {"--only", "api_v3_account", "--dry-run"}} {
		cmd := NewRootCommand("test")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		args := []string{"--config", "/missing/runtime.toml", "--env-file", "/missing/runtime.env", "adapter", "record", "--platform", "binance"}
		cmd.SetArgs(append(args, flags...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("recording loaded runtime configuration: %v", err)
		}
	}
}

func TestRecordingRejectsConfigBeforeCredentials(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config", 0o750); err != nil {
		t.Fatal(err)
	}
	// Reading credentials would fail with an unrelated dotenv syntax error.
	if err := os.WriteFile("config/record.env", []byte("invalid-secret-file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config/record.toml", []byte("[proxy]\nmode = 'environment'"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, want string }{
		{"config/record.toml", "proxy.mode no longer supports environment"},
		{"missing.toml", "read config file missing.toml"},
	} {
		_, err := runRecord("--config", tc.path, "--platform", "binance", "--only", "api_v3_account")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected config failure before credentials, got %v", err)
		}
	}
}

func TestRecordingAdaptersUseConfiguredProxy(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config", 0o750); err != nil {
		t.Fatal(err)
	}
	// Actual public recording must ignore both runtime files and all credential files.
	for _, name := range []string{"local.toml", "local.env", "record.env", "gotest.env"} {
		if err := os.WriteFile(filepath.Join("config", name), []byte("invalid-file"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NO_PROXY", "*")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	var requests atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			t.Errorf("method = %s, want CONNECT", r.Method)
		}
		requests.Add(1)
		http.Error(w, "synthetic proxy unavailable", http.StatusBadGateway)
	}))
	defer proxy.Close()
	contents := "[proxy]\nmode = 'proxy'\nurl = '" + proxy.URL + "'\n"
	for _, name := range []string{"config/record.toml", "chosen.toml"} {
		if err := os.WriteFile(name, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ platform, request string }{
		{"binance", "api_v3_time"},
		{"okx", "api_v5_public_time"},
		{"bybit", "v5_market_time"},
		{"kraken", "0_public_Assets"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			for _, path := range []string{"", "chosen.toml"} {
				args := []string{"--env-file", "missing.env", "--platform", tc.platform, "--only", tc.request, "--out", recordingTestDirectory(t)}
				if path != "" {
					args = append(args, "--config", path)
				}
				before := requests.Load()
				_, err := runRecord(args...)
				class, _ := adapters.Classify(err)
				if class != adapters.ClassTemporary || requests.Load() != before+1 {
					t.Fatalf("recording error=%v requests=%d", err, requests.Load()-before)
				}
			}
		})
	}
}
