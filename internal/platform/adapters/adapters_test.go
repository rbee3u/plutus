package adapters

import (
	"context"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func TestCatalogOffersEachPlatformsStructures(t *testing.T) {
	t.Parallel()

	offered := OfferedPlatforms()
	want := []Platform{PlatformBinance, PlatformOKX, PlatformBybit, PlatformKraken}
	if len(offered) != len(want) {
		t.Fatalf("offered %v, want %v", offered, want)
	}
	for i := range want {
		if offered[i] != want[i] {
			t.Fatalf("offered %v, want %v", offered, want)
		}
	}
	structures := map[Platform][]AccountStructure{}
	modes := map[AccountStructure]int{}
	for _, info := range NewRegistry().Platforms() {
		if info.AdapterAvailable {
			t.Fatalf("%s: an empty registry has no adapter", info.ID)
		}
		for _, structure := range info.Structures {
			structures[info.ID] = append(structures[info.ID], structure.ID)
			modes[structure.ID] = len(structure.MarginModes)
			if len(structure.AccountTypes) == 0 {
				t.Fatalf("%s offers no Account Types", structure.ID)
			}
		}
	}
	wantStructures := map[Platform][]AccountStructure{
		PlatformBinance: {StructureBinanceClassic, StructureBinancePM},
		PlatformOKX:     {StructureOKXUnified},
		PlatformBybit:   {StructureBybitUTA2},
		PlatformKraken:  {StructureKrakenSeparate, StructureKrakenUnified},
	}
	for platform, want := range wantStructures {
		if strings.Join(structureStrings(structures[platform]), ",") != strings.Join(structureStrings(want), ",") {
			t.Fatalf("%s structures %v, want %v", platform, structures[platform], want)
		}
	}
	wantModes := map[AccountStructure]int{
		StructureBinanceClassic: 0, StructureBinancePM: 0, StructureOKXUnified: 4,
		StructureBybitUTA2: 3, StructureKrakenSeparate: 0, StructureKrakenUnified: 0,
	}
	for structure, want := range wantModes {
		if modes[structure] != want {
			t.Fatalf("%s has %d Margin Modes, want %d", structure, modes[structure], want)
		}
	}
	if got := AccountTypeIDs(StructureBinanceClassic); len(got) != 8 || got[0] != "spot" || got[6] != "earn" || got[7] != TypeDualInvestment {
		t.Fatalf("Binance Classic Account Types = %v", got)
	}
}

func structureStrings(in []AccountStructure) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func TestStructureAndMarginModeValidation(t *testing.T) {
	t.Parallel()

	if !ValidStructure(PlatformBinance, StructureBinanceClassic) || ValidStructure(PlatformBinance, StructureOKXUnified) || ValidStructure(PlatformKraken, "") {
		t.Fatal("a structure is valid only for its own Platform")
	}
	if !ValidAccountType(StructureBinanceClassic, "um_futures") || ValidAccountType(StructureBinancePM, "um_futures") {
		t.Fatal("Account Types belong to one structure")
	}
	if ValidAccountType(StructureKrakenUnified, "multi_m") || !ValidAccountType(StructureKrakenSeparate, "multi_m") {
		t.Fatal("Multi-M exists only with Separate wallets")
	}
	cases := []struct {
		structure AccountStructure
		mode      MarginMode
		valid     bool
	}{
		{StructureBinanceClassic, "", true},
		{StructureBinanceClassic, MarginOKXPM, false},
		{StructureOKXUnified, "", false},
		{StructureOKXUnified, MarginOKXFutures, true},
		{StructureOKXUnified, MarginBybitCross, false},
		{StructureBybitUTA2, MarginBybitIsolated, true},
		{StructureKrakenUnified, "", true},
	}
	for _, tc := range cases {
		if got := ValidMarginMode(tc.structure, tc.mode); got != tc.valid {
			t.Fatalf("ValidMarginMode(%s, %q) = %v, want %v", tc.structure, tc.mode, got, tc.valid)
		}
	}
	if MarginModeName(MarginBybitCross) != "Cross Margin" || StructureName(StructureKrakenSeparate) != "Separate wallets" {
		t.Fatal("display names")
	}
}

func TestCheckCredentialsKrakenGroups(t *testing.T) {
	t.Parallel()

	spot := Credentials{APIKey: "k", APISecret: "s"}
	both := Credentials{APIKey: "k", APISecret: "s", Futures: &KeyPair{APIKey: "fk", APISecret: "fs"}}
	if err := CheckCredentials(PlatformKraken, StructureKrakenUnified, []AccountType{"earn"}, spot); err != nil {
		t.Fatalf("Earn needs only the Spot group: %v", err)
	}
	if err := CheckCredentials(PlatformKraken, StructureKrakenUnified, []AccountType{"main"}, spot); err == nil {
		t.Fatal("Main also needs the Futures group to verify the structure")
	}
	if err := CheckCredentials(PlatformKraken, StructureOKXUnified, []AccountType{"main"}, both); err == nil {
		t.Fatal("another Platform's structure must be refused")
	}
	if err := CheckCredentials(PlatformBinance, StructureBinanceClassic, nil, both); err == nil {
		t.Fatal("Binance takes one key pair")
	}
}

func TestCollectAccountsFailsOnlyTheFailingAccount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	accounts := []AccountType{"spot", "earn"}
	reports, err := CollectAccounts(ctx, accounts, func(_ context.Context, accountType AccountType) (AccountReport, error) {
		if accountType == "earn" {
			return AccountReport{}, NewError(ClassUnsupported, "no")
		}
		return AccountReport{AccountType: accountType, Status: StatusOK}, nil
	})
	if err != nil || len(reports) != 2 || reports[0].Status != StatusOK || reports[1].Status != StatusFailed || reports[1].ErrorClass != ClassUnsupported {
		t.Fatalf("reports = %+v, err = %v", reports, err)
	}

	_, err = CollectAccounts(ctx, accounts, func(context.Context, AccountType) (AccountReport, error) {
		return AccountReport{}, NewError(ClassTemporary, "down")
	})
	if class, _ := Classify(err); class != ClassTemporary {
		t.Fatalf("every Account failing retryably must fail the batch, got %v", err)
	}

	calls := 0
	_, err = CollectAccounts(ctx, accounts, func(context.Context, AccountType) (AccountReport, error) {
		calls++
		return AccountReport{}, NewError(ClassAuthentication, "bad key")
	})
	if class, _ := Classify(err); class != ClassAuthentication || calls != 1 {
		t.Fatalf("an authentication failure must stop the batch at once, got %v after %d calls", err, calls)
	}
}

func TestFieldHelpersNeverTurnUnknownIntoZero(t *testing.T) {
	t.Parallel()

	if _, err := Decimal("GET /x", "equity", ""); err == nil {
		t.Fatal("a blank required field is a normalization failure")
	}
	if v, err := OptionalDecimal("GET /x", "interest", " "); err != nil || v.Sign() != 0 {
		t.Fatalf("a blank optional field reads as zero: %v %v", v, err)
	}
	if _, err := OptionalDecimal("GET /x", "interest", "abc"); err == nil {
		t.Fatal("a malformed optional field is a normalization failure")
	}
	if Plain("") != nil || Plain("x") != nil || *Plain("1.500") != "1.5" {
		t.Fatal("Plain keeps only valid decimals")
	}
	if NonZero("0") != nil || NonZero("0.000") != nil || *NonZero("12.30") != "12.3" {
		t.Fatal("NonZero drops zero placeholders")
	}
	if Reported("", big.NewRat(0, 1)) != nil || *Reported("0", big.NewRat(0, 1)) != "0" {
		t.Fatal("Reported distinguishes an absent field from a reported zero")
	}
	if Ratio(big.NewRat(1, 1), big.NewRat(0, 1)) != nil || *Ratio(big.NewRat(1, 1), big.NewRat(3, 1)) != "0.33333333" {
		t.Fatal("Ratio")
	}
}

func TestQuotePricesPicksTheFirstQuotedPair(t *testing.T) {
	t.Parallel()

	quotes := map[string]string{"BTC/USDT": "60000", "ETH/USDC": "3000"}
	lookup := func(symbol, quote string) (string, bool) {
		price, ok := quotes[symbol+"/"+quote]
		return price, ok
	}
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	reports := []AccountReport{{Balances: []Balance{{Symbol: "ETH"}, {Symbol: "BTC"}, {Symbol: "USDT"}, {Symbol: "DOGE"}}}}
	prices := QuotePrices(reports, []string{"USDT", "USDC"}, lookup, at)
	if len(prices) != 2 || prices[0].AssetSymbol != "BTC" || prices[0].QuoteSymbol != "USDT" || prices[1].QuoteSymbol != "USDC" {
		t.Fatalf("prices = %+v", prices)
	}
	if p, ok := USDPrice("usdc", nil, nil); !ok || p.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("USDC values at 1 USD")
	}
	if _, ok := USDPrice("DOGE", []string{"USDT"}, lookup); ok {
		t.Fatal("an unquoted symbol has no USD price")
	}
}

func TestCheckEarnComplete(t *testing.T) {
	t.Parallel()
	priced := []Balance{{Symbol: "BTC", Equity: "0.1"}, {Symbol: "USDT", Equity: "1000"}}
	for _, tc := range []struct {
		name, platformUSD string
		balances          []Balance
		class             string
		amounts           []string
	}{
		{"priced holdings within tolerance", "7050", priced, "", nil},
		{"missing product", "9000", priced, ClassUnsupported, []string{"9000.00", "7000.00"}},
		{"exact floor", "1", nil, "", nil},
		{"above floor", "1.0001", nil, ClassUnsupported, nil},
		{"exact percentage", "200", []Balance{{Symbol: "USDT", Equity: "198"}}, "", nil},
		{"above percentage", "200", []Balance{{Symbol: "USDT", Equity: "197.999"}}, ClassUnsupported, nil},
		{"itemized excess", "100", []Balance{{Symbol: "USDT", Equity: "150"}}, "", nil},
		{"missing price", "9000", append(slices.Clone(priced), Balance{Symbol: "XYZ", Equity: "5"}), "", nil},
		{"malformed amount", "10", []Balance{{Symbol: "USDT", Equity: "invalid"}}, ClassNormalization, nil},
		{"missing valuation", "", priced, ClassNormalization, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reported *big.Rat
			if tc.platformUSD != "" {
				reported = decimal.ParseOrZero(tc.platformUSD)
			}
			err := CheckEarnComplete(PlatformOKX, reported, tc.balances, func(symbol string) (*big.Rat, bool) {
				prices := map[string]*big.Rat{"BTC": big.NewRat(60000, 1), "USDT": big.NewRat(1, 1)}
				price, ok := prices[symbol]
				return price, ok
			})
			class, detail := Classify(err)
			if (err == nil) != (tc.class == "") || err != nil && class != tc.class {
				t.Fatalf("error = %v, want class %q", err, tc.class)
			}
			for _, amount := range tc.amounts {
				if !strings.Contains(detail, amount) {
					t.Fatalf("error must name amount %s: %v", amount, err)
				}
			}
		})
	}
}
