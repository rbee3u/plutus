package catalog

import "testing"

func TestSeedUUIDsAreStable(t *testing.T) {
	t.Parallel()

	if AssetUSD.String() != "01a0bdfc-1fa4-7a54-9fa1-24f4721a71af" {
		t.Fatalf("usd id %s", AssetUSD)
	}
}

func TestAccountModeSupported(t *testing.T) {
	t.Parallel()

	if !AccountModeSupported(AccountModeBinancePM) {
		t.Fatal("binance_pm should be supported")
	}
	if AccountModeSupported("spot") {
		t.Fatal("spot must not be treated as supported")
	}
}
