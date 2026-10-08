package ordering

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestIdentityOrder(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	want := []Identity{
		{Platform: "binance", Name: "Alpha", ID: id1},
		{Platform: "binance", Name: "alpha", ID: id2},
		{Platform: "binance", Name: "Zulu", ID: id1},
		{Platform: "okx", Name: "Zulu", ID: id1},
		{Platform: "bybit", Name: "Alpha", ID: id1},
		{Platform: "kraken", Name: "Alpha", ID: id1},
	}
	got := slices.Clone(want)
	slices.Reverse(got)
	slices.SortFunc(got, CompareIdentities)
	if !slices.Equal(got, want) {
		t.Fatalf("Identities = %v, want %v", got, want)
	}
	if got := PlatformsCSV(); got != "binance,okx,bybit,kraken" {
		t.Fatalf("SQL Platform order = %q", got)
	}
}

func TestAccountBusinessOrder(t *testing.T) {
	cases := []struct {
		structure adapters.AccountStructure
		types     []adapters.AccountType
	}{
		{adapters.StructureBinanceClassic, []adapters.AccountType{"spot", "cross_margin", "isolated_margin", "um_futures", "cm_futures", "funding", "earn", "dual_investment"}},
		{adapters.StructureBinancePM, []adapters.AccountType{"spot", "portfolio_margin", "funding", "earn", "dual_investment"}},
		{adapters.StructureOKXUnified, []adapters.AccountType{"trading", "funding", "earn", "dual_investment"}},
		{adapters.StructureBybitUTA2, []adapters.AccountType{"unified", "funding", "earn", "dual_investment"}},
		{adapters.StructureKrakenSeparate, []adapters.AccountType{"main", "multi_m", "coin_m", "derivatives_holding", "earn"}},
		{adapters.StructureKrakenUnified, []adapters.AccountType{"main", "coin_m", "derivatives_holding", "earn"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.structure), func(t *testing.T) {
			accounts := make([]Account, len(tc.types))
			for i, kind := range tc.types {
				accounts[i] = Account{Structure: tc.structure, Type: kind}
			}
			slices.Reverse(accounts)
			slices.SortFunc(accounts, CompareAccounts)
			for i, account := range accounts {
				if account.Type != tc.types[i] {
					t.Fatalf("Account %d = %s, want %s", i, account.Type, tc.types[i])
				}
			}
		})
	}
}
