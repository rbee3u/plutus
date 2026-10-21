package kraken

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fixture"
)

func commonSamples() []string {
	return []string{"0_public_Assets", "0_public_AssetPairs", "0_public_Ticker", "0_private_GetApiKeyInfo", "0_private_Earn_Strategies", "0_private_Earn_Allocations", "api_auth_v1_api_keys_v3_check", "derivatives_api_v3_openpositions", "derivatives_api_v3_instruments", "derivatives_api_v3_tickers"}
}

func sampleNames(selection string) []string {
	names := commonSamples()
	mode := "unified"
	switch selection {
	case "":
	case "kraken_separate":
		mode = "separate"
	default:
		panic("unknown Kraken test selection")
	}
	names = append(names, "0_private_BalanceEx."+mode, "0_private_TradeBalance."+mode, "ws_v1_balances_snapshot."+mode)
	for i := range names {
		names[i] += ".json"
	}
	return names
}

// These deliberately invalid credentials are used only against the local replay server.
type refusal struct {
	scenario string
	ep       endpoint
	creds    adapter.Credentials
	nonce    string
}

func refusals(creds adapter.Credentials) []refusal {
	return []refusal{
		{"invalid_key", epAPIKeyInfo, adapter.Credentials{APIKey: "invalid", APISecret: creds.APISecret}, ""},
		{"bad_signature", epAPIKeyInfo, adapter.Credentials{APIKey: creds.APIKey, APISecret: "d3Jvbmc="}, ""},
		{"invalid_nonce", epAPIKeyInfo, creds, "1"},
		{"invalid_key", epFuturesKey, adapter.Credentials{Futures: &adapter.KeyPair{APIKey: "invalid", APISecret: creds.FuturesSecret()}}, ""},
		{"bad_signature", epFuturesKey, adapter.Credentials{Futures: &adapter.KeyPair{APIKey: creds.FuturesKey(), APISecret: "d3Jvbmc="}}, ""},
	}
}

func TestFlatSamples(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no interface samples")
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			t.Fatalf("unexpected sample layout: %s", entry.Name())
		}
		f, err := fixture.ReadSynthetic(filepath.Join("testdata", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := RecordingRequests().Find(f.Request); !ok {
			t.Fatalf("sample request is not a native read-only interface: %s", entry.Name())
		}
		base := fixture.Name(f.Request.Path, "")
		if entry.Name() != base+".json" && !strings.HasPrefix(entry.Name(), base+".") {
			t.Fatalf("filename does not describe request: %s", entry.Name())
		}
	}
}

const (
	refusedKey    = "invalid"
	refusedSecret = "d3Jvbmc="
)
