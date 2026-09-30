package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func runRecord(args ...string) (string, error) {
	cmd := newAdapterRecordCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
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
	_, err = runRecord("--platform", "okx", "--only", "api_v5_public_instruments", "--dry-run")
	if err == nil {
		t.Fatal("missing instrument type accepted")
	}
}

type recordingStub struct {
	calls   int
	failure error
}

func (s *recordingStub) RecordRequest(_ context.Context, _ adapters.Credentials, request fixture.Request) (*fixture.File, error) {
	s.calls++
	now := time.Now().UTC()
	return &fixture.File{Source: "recorded", RecordedAt: &now, Request: request, Status: 200, Body: []byte(`{"serverTime":1700000000000}`)}, s.failure
}

func TestRecordingWritesOnlyOneReviewableFile(t *testing.T) {
	t.Chdir(t.TempDir())
	spec := recorders[adapters.PlatformBinance]
	stub := &recordingStub{}
	changed := spec
	changed.adapter = func() recorder { return stub }
	recorders[adapters.PlatformBinance] = changed
	t.Cleanup(func() { recorders[adapters.PlatformBinance] = spec })
	for _, failure := range []error{nil, errors.New("platform failure")} {
		stub.failure = failure
		out, err := runRecord("--platform", "binance", "--only", "api_v3_time")
		if (err != nil) != (failure != nil) || !strings.Contains(out, "business mapping has not been accepted") {
			t.Fatalf("status: %s %v", out, err)
		}
	}
	if stub.calls != 2 {
		t.Fatal("unexpected recorder calls")
	}
	paths, err := filepath.Glob("tmp/fixtures/binance/*/*")
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
	root := t.TempDir()
	t.Chdir(root)
	if err := checkRecordingDirectory("elsewhere"); err == nil {
		t.Fatal("outside tmp accepted")
	}
	if err := os.Mkdir("tmp", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), "tmp/link"); err != nil {
		t.Fatal(err)
	}
	if err := checkRecordingDirectory("tmp/link"); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

type recordingFailureStub struct{ unsafe, disk bool }

func (s recordingFailureStub) RecordRequest(_ context.Context, _ adapters.Credentials, request fixture.Request) (*fixture.File, error) {
	if s.unsafe {
		return nil, errors.New("redaction failed")
	}
	if s.disk {
		if err := os.WriteFile("tmp/fixtures/binance", []byte("blocked output parent"), 0o600); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	return &fixture.File{Source: "recorded", RecordedAt: &now, Request: request, Status: 200, Body: []byte(`[]`)}, nil
}

func TestFailuresAndEmptyResponsesCannotPromoteSamples(t *testing.T) {
	for _, mode := range []string{"empty", "unsafe", "disk"} {
		t.Run(mode, func(t *testing.T) {
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
			changed.adapter = func() recorder {
				return recordingFailureStub{unsafe: mode == "unsafe", disk: mode == "disk"}
			}
			recorders[adapters.PlatformBinance] = changed
			t.Cleanup(func() { recorders[adapters.PlatformBinance] = original })
			out, err := runRecord("--platform", "binance", "--only", "api_v3_time")
			if (err == nil) != (mode == "empty") {
				t.Fatalf("unexpected result: %s %v", out, err)
			}
			if mode == "empty" && !strings.Contains(out, "empty: true") {
				t.Fatal("empty state not reported")
			}
			files, e := filepath.Glob("tmp/fixtures/binance/*/*.json")
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
