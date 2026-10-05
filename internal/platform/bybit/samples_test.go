package bybit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/fixture"
)

func commonSamples() []string {
	return []string{"v5_market_time", "v5_user_query_api", "v5_asset_transfer_query_account_coins_balance", "v5_earn_position", "v5_earn_product", "v5_market_tickers", "v5_market_instruments_info.linear", "v5_market_instruments_info.inverse", "v5_position_list.option"}
}

const (
	scenarioInvalidKey     = "invalid_key"
	scenarioBadSignature   = "bad_signature"
	scenarioStaleTimestamp = "stale_timestamp"
	RedactedIP             = "203.0.113.1"
)

func sampleNames(selection string) []string {
	names := commonSamples()
	mode := "pm"
	switch selection {
	case "", "dual_investment", "fixed_saving", "region_restricted", "uta_1_0_pro", "rate_limited", "ip_rate_limited":
	case "bybit_cross":
		mode = "cross"
	case "bybit_isolated":
		mode = "isolated"
	default:
		panic("unknown Bybit test selection")
	}
	account := "v5_account_info." + mode
	wallet := "v5_account_wallet_balance." + mode
	if selection == "region_restricted" || selection == "uta_1_0_pro" || selection == "ip_rate_limited" {
		account = "v5_account_info." + selection
	}
	if selection == "rate_limited" {
		wallet = "v5_account_wallet_balance.rate_limited"
		for i, name := range names {
			if name == "v5_asset_transfer_query_account_coins_balance" {
				names[i] += ".rate_limited"
			}
		}
	}
	names = append(names, account, wallet)
	for _, kind := range []string{"linear_usdt", "linear_usdc", "inverse"} {
		names = append(names, "v5_position_list."+kind+"_"+mode)
	}
	fixed := "v5_earn_fixed_term_position.empty"
	dual := "v5_earn_advance_position.sell_high"
	overview := "v5_asset_asset_overview"
	if selection == "dual_investment" {
		dual = "v5_earn_advance_position.buy_low"
		overview += ".dual_only"
	}
	if selection == "fixed_saving" {
		fixed = "v5_earn_fixed_term_position"
		dual = "v5_earn_advance_position.empty"
		overview += ".fixed_saving"
	}
	names = append(names, fixed, dual, overview)
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
