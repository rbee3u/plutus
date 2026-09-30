package binance

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

func commonSamples() []string {
	return []string{"api_v3_time", "api_v3_account", "api_v3_ticker_price", "sapi_v3_asset_getUserAsset", "sapi_v1_asset_get_funding_asset", "fapi_v1_exchangeInfo", "dapi_v1_exchangeInfo", "sapi_v1_dci_product_positions.pending", "sapi_v1_dci_product_positions.settling", "sapi_v1_dci_product_positions.refunding"}
}

const (
	RedactedUID = 123456789
	RedactedIP  = "203.0.113.1"
)

var ipv4 = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

func target(ep endpoint, params url.Values) string {
	if len(params) == 0 {
		return ep.path
	}
	return ep.path + "?" + params.Encode()
}

func sampleNames(selection string) []string {
	names := commonSamples()
	switch selection {
	case "", "dual_investment":
		names = append(names, "sapi_v1_account_info.pm", "papi_v1_account", "papi_v1_balance", "papi_v1_um_positionRisk", "papi_v1_cm_positionRisk")
	case "binance_classic":
		names = append(names, "sapi_v1_account_info.classic", "sapi_v1_margin_account", "sapi_v1_margin_isolated_account", "fapi_v3_account", "fapi_v3_positionRisk", "fapi_v1_multiAssetsMargin", "fapi_v1_symbolConfig", "dapi_v1_account", "dapi_v1_positionRisk")
	default:
		panic("unknown Binance test selection")
	}
	if selection == "dual_investment" {
		names = append(names, "sapi_v1_simple_earn_flexible_position.empty", "sapi_v1_simple_earn_locked_position.empty", "sapi_v1_asset_wallet_balance.dual_only", "sapi_v1_dci_product_positions.purchase_success_put")
	} else {
		names = append(names, "sapi_v1_simple_earn_flexible_position", "sapi_v1_simple_earn_locked_position", "sapi_v1_asset_wallet_balance", "sapi_v1_dci_product_positions.purchase_success_call")
	}
	for i := range names {
		names[i] += ".json"
	}
	return names
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
		f, err := fixture.Read(filepath.Join("testdata", entry.Name()))
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
