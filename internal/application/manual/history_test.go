package manual

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeHistoricalBalances(t *testing.T) {
	const id = "01900000-0000-7000-8000-000000000001"
	for _, fields := range []string{
		`"listingId":"` + id + `","symbol":"DEMO-BTC"`,
		`"platformAssetId":"` + id + `","platformAssetSymbol":"DEMO-BTC"`,
		`"listingId":"` + id + `","platformAssetId":"` + id + `","symbol":"DEMO-BTC","platformAssetSymbol":"DEMO-BTC"`,
	} {
		input := `[{` + fields + `,"assetSymbol":"BTC","quantity":"-2","modifiedAt":"2026-10-09T00:00:00Z"}]`
		rows, err := decodeBalances([]byte(input))
		if err != nil || len(rows) != 1 {
			t.Fatalf("decode: %+v %v", rows, err)
		}
		got := rows[0]
		if got.PlatformAssetID.String() != id || got.PlatformAssetSymbol != "DEMO-BTC" || got.AssetSymbol != "BTC" || got.Quantity != "-2" || got.ModifiedAt.IsZero() {
			t.Fatalf("lost history: %+v", got)
		}
		encoded, err := json.Marshal(rows)
		if err != nil || strings.Contains(string(encoded), `"listingId"`) || strings.Contains(string(encoded), `"symbol"`) {
			t.Fatalf("legacy write: %s %v", encoded, err)
		}
	}
}

func TestDecodeHistoricalBalancesRejectsAmbiguousIdentity(t *testing.T) {
	for _, input := range []string{
		`null`, `[{}]`,
		`[{"listingId":"01900000-0000-7000-8000-000000000001","platformAssetId":"01900000-0000-7000-8000-000000000002","symbol":"BTC","assetSymbol":"BTC"}]`,
		`[{"listingId":"01900000-0000-7000-8000-000000000001","symbol":"BTC","platformAssetSymbol":"ETH","assetSymbol":"BTC"}]`,
	} {
		if _, err := decodeBalances([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
