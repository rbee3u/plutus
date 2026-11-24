package financial

import (
	"math/big"
	"testing"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func exposureAccount(name, platform, identity, structure, accountType string) *valuation.AccountValuation {
	return &valuation.AccountValuation{
		AccountID:        uuid.New(),
		AccountName:      name,
		IdentityID:       uuid.New(),
		IdentityName:     identity,
		Platform:         platform,
		AccountStructure: structure,
		AccountType:      accountType,
		Freshness:        valuation.FreshnessFresh,
	}
}

func knownContribution(account *valuation.AccountValuation, canonical, quantity, price, source string) *exposureContribution {
	c := &exposureContribution{
		account:             account,
		canonical:           canonical,
		symbols:             map[string]bool{canonical: true},
		spot:                decimal.Zero(),
		contract:            decimal.Zero(),
		priceSource:         source,
		priceSourcePlatform: account.Platform,
	}
	if quantity != "" {
		c.quantity, _ = decimal.Parse(quantity)
	}
	if price != "" {
		c.price, _ = decimal.Parse(price)
	}
	return c
}

func TestExposureRowStatesAndReferenceValue(t *testing.T) {
	binance := exposureAccount("Spot", "binance", "Main", "binance_classic", "spot")
	group := &exposureGroup{
		canonical: "BTC",
		contributions: []*exposureContribution{{
			account:             binance,
			canonical:           "BTC",
			symbols:             map[string]bool{"BTC": true},
			spot:                big.NewRat(1, 2),
			contract:            big.NewRat(-1, 10),
			hasBalance:          true,
			hasPosition:         true,
			quantity:            big.NewRat(2, 5),
			price:               big.NewRat(60000, 1),
			priceSource:         "spot",
			priceSourcePlatform: "binance",
		}},
	}
	row := exposureRow(group)
	if row.State != ExposureComplete || row.NetQuantity == nil || *row.NetQuantity != "0.4" {
		t.Fatalf("complete row = %+v", row)
	}
	if row.AccountCount != 1 || row.UnknownAccountCount != 0 || row.Freshness != valuation.FreshnessFresh {
		t.Fatalf("complete counts = %+v", row)
	}
	if row.PriceUSD == nil || *row.PriceUSD != "60000" || row.ValueUSD == nil || *row.ValueUSD != "24000" {
		t.Fatalf("complete value = %+v", row)
	}
	if len(row.Issues) != 0 {
		t.Fatalf("complete issues = %+v", row.Issues)
	}

	known := knownContribution(exposureAccount("Spot", "binance", "Main", "binance_classic", "spot"), "DOGE", "12000", "0.1421", "spot")
	unknown := &exposureContribution{
		account:   exposureAccount("Unified", "bybit", "Main", "bybit_uta_2_0", "unified"),
		canonical: "DOGE",
		symbols:   map[string]bool{"DOGE": true},
		unknown:   true,
		issues: []valuation.Issue{{
			Code:   "exposure_unknown_position",
			Symbol: "DOGE",
			Detail: "position inputs unavailable",
		}},
	}
	partial := exposureRow(&exposureGroup{canonical: "DOGE", contributions: []*exposureContribution{known, unknown}})
	if partial.State != ExposurePartial || partial.NetQuantity == nil || *partial.NetQuantity != "12000" {
		t.Fatalf("partial row = %+v", partial)
	}
	if partial.UnknownAccountCount != 1 || len(partial.Issues) != 1 {
		t.Fatalf("partial counts/issues = %+v", partial)
	}
	if partial.ValueUSD == nil || *partial.ValueUSD != "1705.2" {
		t.Fatalf("partial value = %+v", partial)
	}

	onlyUnknown := exposureRow(&exposureGroup{canonical: "LINK", contributions: []*exposureContribution{{
		account:   unknown.account,
		canonical: "LINK",
		symbols:   map[string]bool{"LINK": true},
		unknown:   true,
		issues: []valuation.Issue{{
			Code:   "exposure_unknown_position",
			Symbol: "LINK",
			Detail: "position inputs unavailable",
		}},
	}}})
	if onlyUnknown.State != ExposureUnknown || onlyUnknown.NetQuantity != nil || onlyUnknown.PriceUSD != nil || onlyUnknown.ValueUSD != nil {
		t.Fatalf("unknown row = %+v", onlyUnknown)
	}

	flat := exposureRow(&exposureGroup{canonical: "AVAX", contributions: []*exposureContribution{{
		account:     exposureAccount("Unified", "bybit", "Main", "bybit_uta_2_0", "unified"),
		canonical:   "AVAX",
		symbols:     map[string]bool{"AVAX": true},
		hasPosition: true,
		quantity:    decimal.Zero(),
		price:       big.NewRat(342, 10),
		priceSource: "spot",
		priceWeight: decimal.Zero(),
	}}})
	if flat.State != ExposureComplete || flat.NetQuantity == nil || *flat.NetQuantity != "0" {
		t.Fatalf("flat row = %+v", flat)
	}
	if flat.ValueUSD == nil || *flat.ValueUSD != "0" {
		t.Fatalf("flat value = %+v", flat)
	}
}

func TestReferenceExposurePriceUsesLargestKnownQuantity(t *testing.T) {
	binance := exposureAccount("Spot", "binance", "Main", "binance_classic", "spot")
	okx := exposureAccount("Trading", "okx", "Main", "okx_unified", "trading")
	bybit := exposureAccount("Unified", "bybit", "Main", "bybit_uta_2_0", "unified")
	contributions := []*exposureContribution{
		knownContribution(binance, "BTC", "1", "100", "spot"),
		knownContribution(okx, "BTC", "2", "101", "spot"),
		knownContribution(bybit, "BTC", "1.5", "102", "contract"),
	}
	price, source, _, platform := referenceExposurePrice(contributions)
	if price == nil || price.Cmp(big.NewRat(101, 1)) != 0 || source != "spot" || platform != "okx" {
		t.Fatalf("reference = %v %q %q", price, source, platform)
	}

	tie := []*exposureContribution{
		knownContribution(bybit, "BTC", "1", "102", "spot"),
		knownContribution(binance, "BTC", "1", "100", "spot"),
		knownContribution(okx, "BTC", "1", "101", "spot"),
	}
	price, _, _, platform = referenceExposurePrice(tie)
	if price == nil || price.Cmp(big.NewRat(100, 1)) != 0 || platform != "binance" {
		t.Fatalf("tie reference = %v %q", price, platform)
	}

	zero := []*exposureContribution{
		knownContribution(bybit, "AVAX", "0", "102", "spot"),
		knownContribution(binance, "AVAX", "0", "100", "spot"),
	}
	price, _, _, platform = referenceExposurePrice(zero)
	if price == nil || price.Cmp(big.NewRat(100, 1)) != 0 || platform != "binance" {
		t.Fatalf("zero reference = %v %q", price, platform)
	}
}

func TestRiskPriceWeightUsesAbsoluteNotional(t *testing.T) {
	asset := &riskAsset{positions: []riskPosition{
		{mark: big.NewRat(2, 1), notional: big.NewRat(-10, 1), quote: "USDT"},
		{mark: big.NewRat(3, 1), notional: big.NewRat(5, 1), quote: "USDT"},
	}}
	weight := riskPriceWeight(asset)
	if weight == nil || weight.Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatalf("weight = %v", weight)
	}
}

func TestSortExposureRowsKeepsUnknownLast(t *testing.T) {
	rows := []ExposureRow{
		{CanonicalAssetSymbol: "AVAX", valueRat: decimal.Zero()},
		{CanonicalAssetSymbol: "BTC", valueRat: big.NewRat(100, 1)},
		{CanonicalAssetSymbol: "LINK", valueRat: nil},
		{CanonicalAssetSymbol: "ETH", valueRat: big.NewRat(-50, 1)},
	}
	sortExposureRows(rows, ExposureSortValue)
	got := []string{rows[0].CanonicalAssetSymbol, rows[1].CanonicalAssetSymbol, rows[2].CanonicalAssetSymbol, rows[3].CanonicalAssetSymbol}
	want := []string{"BTC", "ETH", "AVAX", "LINK"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("value order = %v, want %v", got, want)
		}
	}
	sortExposureRows(rows, ExposureSortSymbol)
	got = []string{rows[0].CanonicalAssetSymbol, rows[1].CanonicalAssetSymbol, rows[2].CanonicalAssetSymbol, rows[3].CanonicalAssetSymbol}
	want = []string{"AVAX", "BTC", "ETH", "LINK"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("symbol order = %v, want %v", got, want)
		}
	}
}

func TestSummarizeExposureCountsAndCompleteness(t *testing.T) {
	withValue := func(row ExposureRow, value string) ExposureRow {
		t.Helper()
		rat, ok := decimal.Parse(value)
		if !ok {
			t.Fatalf("parse %q", value)
		}
		row.valueRat = rat
		row.ValueUSD = decimal.FormatPtr(rat)
		return row
	}
	long := withValue(ExposureRow{CanonicalAssetSymbol: "BTC", State: ExposureComplete, NetQuantity: new("1"), Freshness: valuation.FreshnessFresh}, "24000")
	short := withValue(ExposureRow{CanonicalAssetSymbol: "ETH", State: ExposureComplete, NetQuantity: new("-1"), Freshness: valuation.FreshnessStale}, "-3300")
	flat := withValue(ExposureRow{CanonicalAssetSymbol: "AVAX", State: ExposureComplete, NetQuantity: new("0"), Freshness: valuation.FreshnessFresh}, "0")
	partial := withValue(ExposureRow{CanonicalAssetSymbol: "DOGE", State: ExposurePartial, NetQuantity: new("10"), Freshness: valuation.FreshnessFresh}, "1705.2")
	unknown := ExposureRow{CanonicalAssetSymbol: "LINK", State: ExposureUnknown, Freshness: valuation.FreshnessFresh}
	summary := summarizeExposure([]ExposureRow{long, short, flat, partial, unknown}, true, 0, true)
	if summary.AssetCount != 5 || summary.LongCount != 1 || summary.ShortCount != 1 || summary.FlatCount != 1 || summary.PartialCount != 1 || summary.UnknownCount != 1 {
		t.Fatalf("counts = %+v", summary)
	}
	if summary.StablecoinValueUSD == nil || *summary.StablecoinValueUSD != "0" {
		t.Fatalf("stablecoin value = %+v", summary.StablecoinValueUSD)
	}
	if summary.NonStablecoinValueUSD == nil || *summary.NonStablecoinValueUSD != "22405.2" {
		t.Fatalf("non-stablecoin value = %+v", summary.NonStablecoinValueUSD)
	}
	if summary.TotalValueUSD == nil || *summary.TotalValueUSD != "22405.2" {
		t.Fatalf("total value = %+v", summary.TotalValueUSD)
	}
	if summary.UnpricedAssetCount != 1 || len(summary.UnpricedAssetSymbols) != 1 || summary.UnpricedAssetSymbols[0] != "LINK" {
		t.Fatalf("unpriced = %+v", summary)
	}
	if len(summary.StablecoinSymbols) != 0 {
		t.Fatalf("stablecoin symbols = %+v", summary.StablecoinSymbols)
	}
	if summary.Completeness != valuation.CompletenessIncomplete || summary.Freshness != valuation.FreshnessStale {
		t.Fatalf("state = %+v", summary)
	}
	if summary.AssetCount != summary.LongCount+summary.ShortCount+summary.FlatCount+summary.PartialCount+summary.UnknownCount {
		t.Fatalf("count invariant broken: %+v", summary)
	}
	complete := summarizeExposure([]ExposureRow{long, flat}, true, 0, true)
	if complete.Completeness != valuation.CompletenessComplete || complete.Freshness != valuation.FreshnessFresh {
		t.Fatalf("complete state = %+v", complete)
	}
	if complete.StablecoinValueUSD == nil || *complete.StablecoinValueUSD != "0" ||
		complete.NonStablecoinValueUSD == nil || *complete.NonStablecoinValueUSD != "24000" ||
		complete.TotalValueUSD == nil || *complete.TotalValueUSD != "24000" {
		t.Fatalf("complete values = %+v", complete)
	}
}

func TestSummarizeExposureValuesByStablecoinClass(t *testing.T) {
	newRow := func(symbol, value, state string) ExposureRow {
		t.Helper()
		row := ExposureRow{CanonicalAssetSymbol: symbol, State: state, NetQuantity: new("1"), Freshness: valuation.FreshnessFresh}
		if value == "" {
			return row
		}
		rat, ok := decimal.Parse(value)
		if !ok {
			t.Fatalf("parse %q", value)
		}
		row.valueRat = rat
		row.ValueUSD = decimal.FormatPtr(rat)
		return row
	}

	complete := summarizeExposure([]ExposureRow{
		newRow("USDT", "1000", ExposureComplete),
		newRow("USD", "-200", ExposureComplete),
		newRow("BTC", "34000", ExposureComplete),
		newRow("ETH", "-3300", ExposureComplete),
		newRow("AVAX", "0", ExposureComplete),
	}, true, 0, true)
	if complete.Completeness != valuation.CompletenessComplete {
		t.Fatalf("complete = %+v", complete)
	}
	if complete.StablecoinValueUSD == nil || *complete.StablecoinValueUSD != "800" {
		t.Fatalf("stablecoin value = %+v", complete.StablecoinValueUSD)
	}
	if complete.NonStablecoinValueUSD == nil || *complete.NonStablecoinValueUSD != "30700" {
		t.Fatalf("non-stablecoin value = %+v", complete.NonStablecoinValueUSD)
	}
	if complete.TotalValueUSD == nil || *complete.TotalValueUSD != "31500" {
		t.Fatalf("total value = %+v", complete.TotalValueUSD)
	}
	if complete.UnpricedAssetCount != 0 || len(complete.UnpricedAssetSymbols) != 0 {
		t.Fatalf("unpriced = %+v", complete)
	}
	if len(complete.StablecoinSymbols) != 2 || complete.StablecoinSymbols[0] != "USD" || complete.StablecoinSymbols[1] != "USDT" {
		t.Fatalf("stablecoin symbols = %+v", complete.StablecoinSymbols)
	}

	unknownClass := summarizeExposure([]ExposureRow{
		newRow("USDT", "", ExposureComplete),
		newRow("USD", "", ExposureComplete),
		newRow("BTC", "34000", ExposureComplete),
		newRow("ETH", "-3300", ExposureComplete),
	}, true, 0, true)
	if unknownClass.StablecoinValueUSD != nil {
		t.Fatalf("unknown stablecoin value = %+v", unknownClass.StablecoinValueUSD)
	}
	if unknownClass.NonStablecoinValueUSD == nil || *unknownClass.NonStablecoinValueUSD != "30700" {
		t.Fatalf("non-stablecoin value = %+v", unknownClass.NonStablecoinValueUSD)
	}
	if unknownClass.TotalValueUSD == nil || *unknownClass.TotalValueUSD != "30700" {
		t.Fatalf("known partial total = %+v", unknownClass.TotalValueUSD)
	}
	if unknownClass.UnpricedAssetCount != 2 || unknownClass.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("unknown class state = %+v", unknownClass)
	}

	knownZeroWithUnpriced := summarizeExposure([]ExposureRow{
		newRow("AVAX", "0", ExposureComplete),
		newRow("USDT", "", ExposureComplete),
	}, true, 0, true)
	if knownZeroWithUnpriced.StablecoinValueUSD != nil {
		t.Fatalf("unpriced stablecoin value = %+v", knownZeroWithUnpriced.StablecoinValueUSD)
	}
	if knownZeroWithUnpriced.NonStablecoinValueUSD == nil || *knownZeroWithUnpriced.NonStablecoinValueUSD != "0" {
		t.Fatalf("known zero = %+v", knownZeroWithUnpriced.NonStablecoinValueUSD)
	}
	if knownZeroWithUnpriced.TotalValueUSD == nil || *knownZeroWithUnpriced.TotalValueUSD != "0" {
		t.Fatalf("known partial total = %+v", knownZeroWithUnpriced.TotalValueUSD)
	}

	empty := summarizeExposure(nil, true, 0, true)
	if empty.Completeness != valuation.CompletenessComplete {
		t.Fatalf("empty completeness = %+v", empty)
	}
	for name, value := range map[string]*string{
		"stablecoin": empty.StablecoinValueUSD,
		"non-stable": empty.NonStablecoinValueUSD,
		"total":      empty.TotalValueUSD,
	} {
		if value == nil || *value != "0" {
			t.Fatalf("empty %s value = %+v", name, value)
		}
	}

	noCoverage := summarizeExposure(nil, false, 0, true)
	if noCoverage.StablecoinValueUSD != nil || noCoverage.NonStablecoinValueUSD != nil || noCoverage.TotalValueUSD != nil {
		t.Fatalf("incomplete empty values = %+v", noCoverage)
	}
	if noCoverage.Completeness != valuation.CompletenessIncomplete {
		t.Fatalf("incomplete empty state = %+v", noCoverage)
	}
}

func TestExposureIssueDedupeKeepsAccountAndSymbol(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	issues := []valuation.Issue{
		{Code: "missing", AccountID: &first, AccountName: "Earn"},
		{Code: "missing", AccountID: &first, AccountName: "Earn"},
		{Code: "stale", AccountID: &second, Symbol: "USDT"},
	}
	out := dedupeIssues(issues)
	if len(out) != 2 {
		t.Fatalf("dedupe = %+v", out)
	}
}

func TestExposurePlatformSymbolsAreSorted(t *testing.T) {
	row := exposureRow(&exposureGroup{canonical: "BTC", contributions: []*exposureContribution{{
		account:   exposureAccount("Main", "kraken", "Main", "kraken_unified", "main"),
		canonical: "BTC",
		symbols:   map[string]bool{"XBT": true, "BTC": true},
		quantity:  big.NewRat(1, 1),
	}}})
	if len(row.PlatformSymbols) != 2 || row.PlatformSymbols[0] != "BTC" || row.PlatformSymbols[1] != "XBT" {
		t.Fatalf("symbols = %v", row.PlatformSymbols)
	}
	if row.State != ExposureComplete {
		t.Fatalf("state = %q", row.State)
	}
}
