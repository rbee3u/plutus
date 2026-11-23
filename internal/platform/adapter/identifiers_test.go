package adapter

import "testing"

func TestSupportedUnifiedAccountScopes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		platform  Platform
		structure AccountStructure
		account   AccountType
		want      bool
	}{
		{PlatformBinance, StructureBinancePM, "portfolio_margin", true},
		{PlatformBinance, StructureBinancePM, "spot", false},
		{PlatformBinance, StructureBinanceClassic, "portfolio_margin", false},
		{PlatformOKX, StructureOKXUnified, "trading", true},
		{PlatformOKX, StructureOKXUnified, "main", false},
		{PlatformBybit, StructureBybitUTA2, "unified", true},
		{PlatformBybit, StructureBybitUTA2, "funding", false},
		{PlatformKraken, StructureKrakenUnified, "main", true},
		{PlatformKraken, StructureKrakenUnified, "coin_m", false},
		{PlatformKraken, StructureKrakenSeparate, "main", false},
	}
	for _, tc := range cases {
		if got := SupportedUnifiedAccount(tc.platform, tc.structure, tc.account); got != tc.want {
			t.Errorf("%s/%s/%s = %v, want %v", tc.platform, tc.structure, tc.account, got, tc.want)
		}
	}
}
