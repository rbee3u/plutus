package okx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func commonSamples() []string {
	return []string{"api_v5_public_time", "api_v5_asset_balances", "api_v5_finance_savings_balance", "api_v5_finance_staking_defi_orders_active", "api_v5_market_tickers", "api_v5_public_instruments.swap", "api_v5_public_instruments.futures", "api_v5_asset_asset_valuation"}
}

const (
	RedactedUID     = "100000000000000001"
	RedactedMainUID = "100000000000000002"
	RedactedLabel   = "plutus-fixture"
	RedactedIP      = "203.0.113.1"
)

var ipv4 = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

func sampleNames(selection string) []string {
	names := commonSamples()
	mode := "pm"
	switch selection {
	case "", "dual_investment":
	case "okx_spot":
		mode = "spot"
	case "okx_futures":
		mode = "futures"
	case "okx_mcm":
		mode = "mcm"
	default:
		panic("unknown OKX test selection")
	}
	for _, base := range []string{"api_v5_account_config", "api_v5_account_balance", "api_v5_account_positions"} {
		names = append(names, base+"."+mode)
	}
	dual := "api_v5_finance_sfp_dcd_order_history"
	if selection == "dual_investment" {
		dual += ".put"
	}
	names = append(names, dual)
	for i := range names {
		names[i] += ".json"
	}
	return names
}

func rowsOf[T any](ep endpoint, body []byte) ([]T, error) {
	var env struct {
		Data []T `json:"data"`
	}
	err := json.Unmarshal(body, &env)
	return env.Data, err
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
