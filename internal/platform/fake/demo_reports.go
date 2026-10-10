package fake

import (
	"fmt"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type demoAsset struct{ Symbol, Name, Price string }

// These are offline price scales, not recorded quotes or an asset assertion.
var demoAssets = []demoAsset{
	{"BTC", "Bitcoin", "65000"},
	{"ETH", "Ethereum", "3200"},
	{"SOL", "Solana", "150"},
	{"BNB", "BNB", "600"},
	{"USDT", "Tether", "1"},
	{"USDC", "USD Coin", "1"},
	{"USD", "US Dollar", "1"},
	{"DAI", "Dai", "1.001"},
	{"XRP", "XRP", "0.55"},
	{"ADA", "Cardano", "0.4"},
	{"DOGE", "Dogecoin", "0.12"},
	{"AVAX", "Avalanche", "28"},
	{"DOT", "Polkadot", "5"},
	{"LINK", "Chainlink", "14"},
	{"LTC", "Litecoin", "70"},
	{"BCH", "Bitcoin Cash", "350"},
	{"TRX", "TRON", "0.15"},
	{"ATOM", "Cosmos", "6"},
	{"UNI", "Uniswap", "7"},
	{"NEAR", "NEAR", "4.5"},
	{"APT", "Aptos", "7"},
	{"ARB", "Arbitrum", "0.6"},
	{"OP", "Optimism", "1.6"},
	{"SUI", "Sui", "1.5"},
	{"FIL", "Filecoin", "4"},
	{"ETC", "Ethereum Classic", "20"},
	{"AAVE", "Aave", "150"},
	{"MKR", "Maker", "1800"},
	{"ALGO", "Algorand", "0.13"},
	{"XLM", "Stellar", "0.1"},
	{"ICP", "Internet Computer", "8"},
	{"HBAR", "Hedera", "0.06"},
	{"VET", "VeChain", "0.025"},
	{"THETA", "Theta", "1.3"},
	{"FTM", "Fantom", "0.5"},
	{"RUNE", "THORChain", "4"},
	{"INJ", "Injective", "22"},
	{"GRT", "The Graph", "0.16"},
	{"IMX", "Immutable", "1.5"},
	{"STX", "Stacks", "1.8"},
	{"FET", "Fetch.ai", "1.2"},
	{"RENDER", "Render", "6"},
	{"LDO", "Lido DAO", "1.4"},
	{"CRV", "Curve", "0.3"},
	{"SNX", "Synthetix", "1.8"},
	{"COMP", "Compound", "45"},
	{"SUSHI", "Sushi", "0.7"},
	{"YFI", "yearn.finance", "5000"},
	{"PEPE", "Pepe", "0.000009"},
	{"SHIB", "Shiba Inu", "0.000018"},
	{"BONK", "Bonk", "0.000023"},
	{"FLOKI", "Floki", "0.00014"},
	{"WIF", "dogwifhat", "2"},
	{"JUP", "Jupiter", "0.8"},
	{"PYTH", "Pyth Network", "0.35"},
	{"SEI", "Sei", "0.4"},
	{"TIA", "Celestia", "5"},
	{"DYDX", "dYdX", "1.2"},
	{"EGLD", "MultiversX", "28"},
	{"KAVA", "Kava", "0.4"},
	{"ZEC", "Zcash", "35"},
	{"DASH", "Dash", "25"},
	{"XTZ", "Tezos", "0.7"},
	{"EOS", "EOS", "0.5"},
	{"QNT", "Quant", "75"},
	{"ENS", "Ethereum Name Service", "18"},
	{"BAT", "Basic Attention Token", "0.2"},
	{"CHZ", "Chiliz", "0.06"},
}

// DemoPrice is coherent across logins and anchors. Integer triangular waves
// produce exact, bounded gains and losses without floating-point money math.
func DemoPrice(platform adapters.Platform, symbol string, at time.Time) string {
	for i, asset := range demoAssets {
		if asset.Symbol != symbol {
			continue
		}
		if symbol == "USD" || symbol == "USDT" || symbol == "USDC" {
			return "1"
		}
		phase := (at.UTC().Unix()/3600 + int64(i*31)) % 336
		if phase < 0 {
			phase += 336
		}
		if phase > 168 {
			phase = 336 - phase
		}
		premium := int64(0)
		if platform == adapters.PlatformOKX {
			premium = 2
		}
		return decimal.Format(decimal.Mul(decimal.ParseOrZero(asset.Price), big.NewRat(9500+phase*6+premium, 10000)))
	}
	return ""
}

func demoFailure(d DemoIdentity, anchor, at time.Time) error {
	switch d.Profile {
	case "missing":
		return adapters.NewError(adapters.ClassTemporary, "The first complete Account Report is not available.")
	case "unconfigured":
		return adapters.NewError(adapters.ClassAuthentication, "The Connection has no credentials.")
	case "normalization":
		return adapters.NewError(adapters.ClassNormalization, "Required Balance equity is unavailable; no partial Complete Set was stored.")
	case "stale", "stale_incomplete":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassRateLimited, "The Platform is withholding this Account's report.")
		}
	case "invalid":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassAuthentication, "The Platform rejected the revoked read-only key.")
		}
	case "disabled":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassTemporary, "Collection is disabled; compatible earlier holdings remain stale.")
		}
	case "unsupported":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassUnsupported, "The Platform product report cannot be normalized under the declared structure.")
		}
	}
	// A historical recoverable outage leaves the current daily combination healthy.
	if d.ID == "daily-bybit" && !at.Before(DemoEventTime(anchor, -6)) && at.Before(DemoEventTime(anchor, -5)) {
		return adapters.NewError(adapters.ClassTemporary, "A historical Platform outage delayed collection.")
	}
	return nil
}

func demoReport(d DemoIdentity, anchor, at time.Time, kind adapters.AccountType) adapters.AccountReport {
	if err := demoFailure(d, anchor, at); err != nil {
		return adapters.FailedReport(kind, err)
	}
	r := adapters.AccountReport{AccountType: kind, Status: adapters.StatusSucceeded, CollectedAt: at}
	if d.Profile == "empty" {
		return r
	}
	//nolint:exhaustive // Manual is application-owned and never enters private Adapter collection.
	switch kind {
	case "earn":
		demoEarn(&r, d, anchor, at)
	case adapters.TypeDualInvestment:
		demoDual(&r, d, anchor, at)
	case "spot", "main":
		r.Balances = []adapters.Balance{demoBalance("USDT", "2500"), demoBalance("BTC", "0.06"), demoBalance("USDC", "0"), demoBalance("SHIB", "0.00000000000002")}
		if d.ID == "daily-classic" {
			if !at.Before(DemoEventTime(anchor, -60)) {
				r.Balances[1].Equity = "0.16"
			}
			if !at.Before(DemoEventTime(anchor, -3)) {
				r.Balances[0].Equity = "3250"
			}
		}
		if d.Profile == "large" {
			r.Balances = nil
			for _, asset := range demoAssets {
				r.Balances = append(r.Balances, demoBalance(asset.Symbol, demoDiv("450", asset.Price)))
			}
		}
		if d.Profile == "unpriced" && !at.Before(DemoEventTime(anchor, -5)) || d.Profile == "stale_incomplete" {
			r.Balances = append(r.Balances, adapters.Balance{AssetSymbol: "DEMO-NOPRICE", Equity: "1000"})
		}
		if d.Profile == "policy" {
			r.Balances = append(r.Balances, adapters.Balance{AssetSymbol: "DEMO-BTC", Equity: "0.01"}, adapters.Balance{AssetSymbol: "DEMO-DUST", Equity: "0.0000002"})
		}
	case "funding", "derivatives_holding":
		r.Balances = []adapters.Balance{demoBalance("USDT", "1800"), demoBalance("USDC", "3000")}
		if d.ID == "daily-classic" {
			if !at.Before(DemoEventTime(anchor, -30)) {
				r.Balances[0].Equity = "6800"
			}
			if !at.Before(DemoEventTime(anchor, -5)) {
				r.Balances[1].Equity = "2000"
			}
			if !at.Before(DemoEventTime(anchor, -3)) {
				r.Balances[0].Equity = demoSub(r.Balances[0].Equity, "750")
			}
		}
		// Principal sits in Funding before subscription and after redemption.
		// Edge scenarios have no unrelated product lifecycle mutations.
		if d.Portfolio != "edge-cases" {
			if at.Before(DemoEventTime(anchor, -33)) || !at.Before(DemoEventTime(anchor, -3)) {
				r.Balances[0].Equity = demoAdd(r.Balances[0].Equity, "400")
			}
			if d.Platform != adapters.PlatformKraken && (at.Before(DemoEventTime(anchor, -10)) || !at.Before(DemoEventTime(anchor, -3))) {
				r.Balances[0].Equity = demoAdd(r.Balances[0].Equity, "200")
			}
		}
	case "cross_margin", "isolated_margin":
		r.Balances = []adapters.Balance{{AssetSymbol: "USDT", Equity: "-150", Liability: adapters.OptionalText("150"), Interest: adapters.OptionalText("0.25")}, demoBalance("BTC", "0.04")}
		if kind == "cross_margin" {
			r.ReportedMarginRatio = adapters.OptionalText("4.2")
		}
	case "portfolio_margin", "trading", "unified", "um_futures", "cm_futures", "multi_m", "coin_m":
		demoTrading(&r, d, anchor, at)
	}
	// Equity/availability describe the final quantities, including event deltas.
	for i := range r.Balances {
		b := &r.Balances[i]
		if b.AvailableToWithdraw != nil {
			b.AvailableToWithdraw = adapters.OptionalText(b.Equity)
			b.AvailableToTrade = adapters.OptionalText(b.Equity)
		}
	}
	return r
}

func demoBalance(symbol, amount string) adapters.Balance {
	return adapters.Balance{AssetSymbol: symbol, Equity: amount, AvailableToTrade: adapters.OptionalText(amount), AvailableToWithdraw: adapters.OptionalText(amount)}
}

func demoTrading(r *adapters.AccountReport, d DemoIdentity, anchor, at time.Time) {
	mode := demoMode(d, anchor, at)
	inverse := r.AccountType == "cm_futures" || r.AccountType == "coin_m"
	if mode != adapters.MarginOKXSpot {
		count := 3
		if d.Profile == "large" && r.AccountType == "um_futures" {
			count = len(demoAssets)
		}
		for i := 0; i < count; i++ {
			asset := demoAssets[i%len(demoAssets)]
			if count > 3 && (asset.Symbol == "USD" || asset.Symbol == "USDT" || asset.Symbol == "USDC" || asset.Symbol == "DAI") {
				continue
			}
			if count == 3 {
				asset = demoAssets[0]
			}
			p := demoPosition(d, anchor, at, asset, i, inverse)
			if d.Profile == "metadata" && i == 0 {
				p.MarginAssetSymbol, p.MarginBasis = "", ""
				p.FuturesContract.ContractSize, p.MarkPrice, p.Margin = nil, nil, nil
			}
			r.Positions = append(r.Positions, p)
		}
	}
	// Historical opening and closing are visible; the final cash includes realized PnL.
	realizedETH := ""
	closed := !at.Before(DemoEventTime(anchor, -4))
	if !at.Before(DemoEventTime(anchor, -8)) && !closed {
		p := demoPosition(d, anchor, at, demoAssets[1], 80, inverse)
		p.PositionKey = "historical-hedge"
		p.EntryPrice = adapters.OptionalText(DemoPrice(d.Platform, "ETH", DemoEventTime(anchor, -8)))
		p.UnrealizedPnl = adapters.OptionalText(demoPositionPnl(p))
		r.Positions = append(r.Positions, p)
	}
	settle := "USDT"
	base := "6500"
	if inverse {
		settle, base = "BTC", "0.06"
	}
	if closed && mode != adapters.MarginOKXSpot {
		p := demoPosition(d, anchor, DemoEventTime(anchor, -4), demoAssets[1], 80, inverse)
		p.EntryPrice = adapters.OptionalText(DemoPrice(d.Platform, "ETH", DemoEventTime(anchor, -8)))
		// Inverse contracts settle in their own Base, not a shared BTC wallet.
		if inverse {
			realizedETH = demoAdd("0.05", demoPositionPnl(p))
		} else {
			base = demoAdd(base, demoPositionPnl(p))
		}
	}
	amounts := map[string]string{settle: base}
	if realizedETH != "" {
		amounts["ETH"] = realizedETH
	}
	for _, p := range r.Positions {
		coin := p.FuturesContract.SettlementAssetSymbol
		if _, ok := amounts[coin]; !ok {
			amounts[coin] = "0.05"
		}
		amounts[coin] = demoAdd(amounts[coin], demoString(p.UnrealizedPnl))
	}
	for _, asset := range demoAssets {
		if amount, ok := amounts[asset.Symbol]; ok {
			r.Balances = append(r.Balances, demoBalance(asset.Symbol, amount))
		}
	}
	if d.Profile == "large" && r.AccountType == "um_futures" {
		// Scale collateral with the deliberately large contract collection.
		r.Balances[0] = demoBalance("USDT", demoAdd(r.Balances[0].Equity, "250000"))
	}
	if !inverse && mode != adapters.MarginBybitIsolated {
		value := "0"
		for _, b := range r.Balances {
			value = demoAdd(value, demoMul(b.Equity, DemoPrice(d.Platform, b.AssetSymbol, at)))
		}
		r.Equity = adapters.OptionalText(value)
		if mode != adapters.MarginOKXFutures {
			r.AdjustedEquity, r.AvailableEquity = adapters.OptionalText(value), adapters.OptionalText(demoMul(value, "0.7"))
			r.ReportedInitialMarginRatio, r.ReportedMaintenanceMarginRatio = adapters.OptionalText("0.12"), adapters.OptionalText("0.04")
		}
	}
}

func demoPosition(d DemoIdentity, anchor, at time.Time, asset demoAsset, i int, inverse bool) adapters.Position {
	// Rolling 28-day cohorts represent explicit close/reopen at the same mark;
	// references, entry price and delivery time remain stable within each cohort.
	cohort := demoCohort(anchor, at, i%4)
	mark := DemoPrice(d.Platform, asset.Symbol, at)
	entry := DemoPrice(d.Platform, asset.Symbol, cohort)
	qty := demoDiv("1200", asset.Price)
	kind, unit, settle, contract := adapters.KindLinearPerpetual, adapters.UnitBase, "USDT", "1"
	if inverse {
		kind, unit, settle, contract, qty = adapters.KindInversePerpetual, adapters.UnitContracts, asset.Symbol, "100", "12"
	}
	var expiry *time.Time
	if i%3 == 2 {
		end := cohort.AddDate(0, 0, 35)
		expiry = &end
		kind = adapters.KindLinearDelivery
		if inverse {
			kind = adapters.KindInverseDelivery
		}
	}
	symbol := demoFuturesContractSymbol(d.Platform, asset.Symbol, inverse, expiry)
	p := adapters.Position{
		PositionKey:     fmt.Sprintf("%s-%d-%d", symbol, i, cohort.Unix()),
		FuturesContract: adapters.FuturesContract{Symbol: symbol, Kind: kind, BaseAssetSymbol: asset.Symbol, QuoteAssetSymbol: "USDT", SettlementAssetSymbol: settle, ContractSize: adapters.OptionalText(contract), ExpiryAt: expiry},
		Side:            adapters.SideLong, PositionMode: "hedged", MarginSetting: "cross", MarginAssetSymbol: settle, MarginBasis: "initial",
		SourceQuantity: qty, SourceQuantityUnit: unit, EntryPrice: adapters.OptionalText(entry), MarkPrice: adapters.OptionalText(mark), Leverage: adapters.OptionalText("3"),
	}
	if inverse {
		p.FuturesContract.QuoteAssetSymbol = "USD"
	}
	if i%2 == 1 {
		p.Side = adapters.SideShort
	}
	if i%3 == 2 {
		p.PositionMode = "one_way"
	}
	if demoMode(d, anchor, at) == adapters.MarginBybitIsolated || i%4 == 3 && d.Structure != adapters.StructureBinancePM {
		p.MarginSetting, p.MarginBasis = "isolated", "allocated"
	}
	notional := demoMul(qty, mark)
	if inverse {
		notional = demoMul(qty, contract)
	}
	margin := demoDiv(notional, "3")
	if inverse {
		margin = demoDiv(margin, mark)
	}
	p.Margin = adapters.OptionalText(margin)
	p.LiquidationPrice = adapters.OptionalText(demoMul(entry, "0.7"))
	if p.Side == adapters.SideShort {
		p.LiquidationPrice = adapters.OptionalText(demoMul(entry, "1.3"))
	}
	if d.Structure == adapters.StructureBinancePM || d.Profile == "large" && i%7 == 0 || inverse && d.Platform == adapters.PlatformBinance {
		p.Margin = nil // A supported Binance cross row uses calculated initial margin.
	}
	if demoMode(d, anchor, at) == adapters.MarginOKXPM || demoMode(d, anchor, at) == adapters.MarginBybitPM {
		p.Margin, p.Leverage, p.LiquidationPrice = nil, nil, nil
	}
	p.UnrealizedPnl = adapters.OptionalText(demoPositionPnl(p))
	return p
}

func demoPositionPnl(p adapters.Position) string {
	entry, mark := demoString(p.EntryPrice), demoString(p.MarkPrice)
	value := demoMul(p.SourceQuantity, demoSub(mark, entry))
	if p.SourceQuantityUnit == adapters.UnitContracts {
		value = demoMul(demoMul(p.SourceQuantity, demoString(p.FuturesContract.ContractSize)), demoSub(demoDiv("1", entry), demoDiv("1", mark)))
	}
	if p.Side == adapters.SideShort {
		value = demoMul(value, "-1")
	}
	return value
}

func demoFuturesContractSymbol(platform adapters.Platform, base string, inverse bool, expiry *time.Time) string {
	quote, term := "USDT", "PERP"
	if inverse {
		quote = "USD"
	}
	if expiry != nil {
		term = expiry.Format("060102")
	}
	switch platform {
	case adapters.PlatformBinance:
		if !inverse && expiry == nil {
			return base + quote
		}
		return base + quote + "_" + term
	case adapters.PlatformOKX:
		if expiry == nil {
			term = "SWAP"
		}
		return base + "-" + quote + "-" + term
	case adapters.PlatformBybit:
		if expiry == nil {
			return base + quote
		}
		return base + quote + "-" + term
	case adapters.PlatformKraken:
		prefix := "PF_"
		if inverse {
			prefix = "PI_"
		}
		if expiry != nil {
			prefix = "FI_"
		}
		return prefix + base + quote + "_" + term
	}
	return ""
}

func demoCohort(anchor, at time.Time, offset int) time.Time {
	start := anchor.UTC().Truncate(24*time.Hour).AddDate(0, 0, -7-offset)
	period := int64(28 * 24 * time.Hour / time.Second)
	delta := at.Unix() - start.Unix()
	n := delta / period
	if delta < 0 && delta%period != 0 {
		n--
	}
	return start.Add(time.Duration(n*period) * time.Second)
}

func demoString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Inputs here are validated static scenario literals or exact generated values.
// Invalid arithmetic is a programming error, never an unknown-to-zero fallback.
func demoNumber(s string) *big.Rat {
	n, ok := decimal.Parse(s)
	if !decimal.Valid(s) || !ok {
		panic("invalid demo decimal " + s)
	}
	return n
}

func demoAdd(a, b string) string { return decimal.Format(decimal.Add(demoNumber(a), demoNumber(b))) }

func demoSub(a, b string) string { return decimal.Format(decimal.Sub(demoNumber(a), demoNumber(b))) }

func demoMul(a, b string) string { return decimal.Format(decimal.Mul(demoNumber(a), demoNumber(b))) }

func demoDiv(a, b string) string {
	return decimal.Format(new(big.Rat).Quo(demoNumber(a), demoNumber(b)))
}

func demoAmounts(r *adapters.AccountReport) {
	totals := adapters.Totals{}
	for _, p := range r.EarnPositions {
		for _, a := range p.Assets {
			if err := totals.Add("demo Earn", a.AssetSymbol, demoNumber(a.Amount)); err != nil {
				panic(err)
			}
		}
	}
	for _, p := range r.DualPositions {
		if err := totals.Add("demo Dual", p.PrincipalAssetSymbol, demoNumber(p.Principal)); err != nil {
			panic(err)
		}
	}
	r.Balances = totals.Balances()
}
