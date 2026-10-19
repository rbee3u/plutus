package fake

import (
	"fmt"
	"math/big"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

type demoAsset struct{ Symbol, Name, Price string }

type demoHolding struct{ Symbol, Amount string }

// Synthetic quantities borrow only the production dataset's concentration and
// wallet shapes. Missing entries are successful empty wallets, not failed reads.
var demoWallets = map[string]map[adapters.AccountType][]demoHolding{
	"daily-classic": {
		"spot":       {{"USDT", "2500"}, {"BTC", "0.06"}, {"USDC", "0"}, {"SHIB", "0.00000000000002"}},
		"funding":    {{"USDT", "1800"}, {"USDC", "3000"}},
		"um_futures": {{"USDT", "2800"}, {"ETH", "0.4"}},
		"cm_futures": {{"BTC", "0.035"}},
		"earn":       {{"USDT", "800"}, {"ETH", "0.12"}},
	},
	"daily-pm": {
		"portfolio_margin": {{"USDT", "-1200"}, {"BTC", "0.62"}, {"ETH", "6.4"}, {"BNB", "8"}},
		"earn":             {{"USDT", "36000"}, {"USDC", "6000"}, {"ETH", "1.6"}, {"BTC", "0.025"}, {"BNB", "1.4"}},
	},
	"daily-okx": {
		"trading": {{"USDT", "-750"}, {"BTC", "0.12"}, {"ETH", "1.8"}, {"SOL", "20"}},
		"earn":    {{"USDT", "5000"}, {"ETH", "0.4"}},
	},
	"daily-bybit": {"unified": {{"USDT", "2400"}, {"BTC", "0.09"}, {"ETH", "0.5"}}},
	"daily-kraken": {
		"main":    {{"USD", "900"}, {"LTC", "7"}, {"ADA", "750"}, {"UNI", "12"}},
		"multi_m": {{"USD", "-200"}, {"BTC", "0.045"}},
		"coin_m":  {{"BTC", "0.015"}},
		"earn":    {{"DOT", "120"}, {"ETH", "0.3"}, {"ADA", "240"}},
	},
	"daily-kraken-unified": {
		"main": {{"USD", "-850"}, {"BTC", "0.13"}, {"ETH", "2.1"}, {"LINK", "90"}, {"DOGE", "600"}},
		"earn": {{"ETH", "1.2"}, {"SOL", "8"}, {"LTC", "2"}},
	},
	"daily-sub-01": {
		"portfolio_margin": {{"USDT", "-300"}, {"BTC", "0.055"}, {"ETH", "0.65"}, {"BNB", "1.2"}},
		"earn":             {{"USDT", "9000"}},
	},
	"daily-sub-02": {
		"portfolio_margin": {{"USDT", "500"}, {"BTC", "0.085"}, {"XRP", "2400"}},
		"earn":             {{"ETH", "0.8"}, {"BNB", "3"}},
	},
	"daily-sub-03": {"portfolio_margin": {{"USDT", "1500"}, {"ETH", "1.3"}, {"SOL", "12"}, {"HYPE", "40"}}},
	"daily-okx-sum": {
		"trading": {{"USDT", "-220"}, {"BTC", "0.07"}, {"ETH", "0.9"}},
		"earn":    {{"USDT", "2000"}},
	},
	"daily-okx-coin": {"trading": {{"BTC", "0.06"}, {"USD", "-400"}}},
	"daily-okx-usd": {
		"trading": {{"USDT", "3600"}, {"ETH", "0.8"}},
		"earn":    {{"USDT", "7500"}},
	},
}

func demoWallet(d DemoIdentity, kind adapters.AccountType) []adapters.Balance {
	var balances []adapters.Balance
	for _, h := range demoWallets[d.ID][kind] {
		b := demoBalance(h.Symbol, h.Amount)
		if demoNumber(h.Amount).Sign() < 0 {
			b.AvailableToTrade, b.AvailableToWithdraw = nil, nil
			b.Liability = adapters.OptionalText(demoMul(h.Amount, "-1"))
		}
		balances = append(balances, b)
	}
	return balances
}

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
	{"HYPE", "Hyperliquid", "22"},
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
		return adapters.NewError(adapters.ClassTemporary, "The first complete account report is not available.")
	case "unconfigured":
		return adapters.NewError(adapters.ClassAuthentication, "The connection has no credentials.")
	case "normalization":
		return adapters.NewError(adapters.ClassNormalization, "Required balance equity is unavailable; no partial complete set was stored.")
	case "stale", "stale_incomplete":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassRateLimited, "The platform is withholding this account's report.")
		}
	case "invalid":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassAuthentication, "The platform rejected the revoked read-only key.")
		}
	case "disabled":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassTemporary, "Collection is disabled; compatible earlier holdings remain stale.")
		}
	case "unsupported":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return adapters.NewError(adapters.ClassUnsupported, "The platform product report cannot be normalized under the declared structure.")
		}
	}
	// A historical recoverable outage leaves the current daily combination healthy.
	if d.ID == "daily-bybit" && !at.Before(DemoEventTime(anchor, -6)) && at.Before(DemoEventTime(anchor, -5)) {
		return adapters.NewError(adapters.ClassTemporary, "A historical platform outage delayed collection.")
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
	//nolint:exhaustive // manual is application-owned and never enters private adapter collection.
	switch kind {
	case "earn":
		demoEarn(&r, d, anchor, at)
	case adapters.TypeDualInvestment:
		demoDual(&r, d, anchor, at)
	case "spot", "main":
		r.Balances = []adapters.Balance{demoBalance("USDT", "2500"), demoBalance("BTC", "0.06"), demoBalance("USDC", "0"), demoBalance("SHIB", "0.00000000000002")}
		if d.Profile == "daily" {
			r.Balances = demoWallet(d, kind)
			if kind == "main" && d.Structure == adapters.StructureKrakenUnified {
				r.Balances = nil
				demoTrading(&r, d, anchor, at)
			}
		}
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
		if d.Profile == "unpriced" && !at.Before(DemoEventTime(anchor, -5)) || d.Profile == "stale_incomplete" || d.Profile == "stale" {
			r.Balances = append(r.Balances, adapters.Balance{AssetSymbol: "DEMO-NOPRICE", Equity: "1000"})
		}
		if d.Profile == "policy" || d.Profile == "unpriced" {
			r.Balances = append(r.Balances, adapters.Balance{AssetSymbol: "DEMO-BTC", Equity: "0.01"}, adapters.Balance{AssetSymbol: "DEMO-DUST", Equity: "0.0000002"})
		}
	case "funding", "derivatives_holding":
		r.Balances = []adapters.Balance{demoBalance("USDT", "1800"), demoBalance("USDC", "3000")}
		if d.Profile == "daily" {
			r.Balances = demoWallet(d, kind)
		}
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
		// Principal sits in funding before subscription and after redemption.
		// Edge scenarios have no unrelated product lifecycle mutations.
		if d.ID == "daily-classic" {
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
		if d.Profile == "daily" {
			r.Balances, r.ReportedMarginRatio = nil, nil
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
	inverse := r.AccountType == "cm_futures" || r.AccountType == "coin_m" || d.ID == "daily-okx-coin"
	wallet := demoWallet(d, r.AccountType)
	if d.Profile == "daily" && len(wallet) == 0 {
		return
	}
	if mode != adapters.MarginOKXSpot {
		count := 3
		if d.Profile == "large" && r.AccountType == "um_futures" {
			count = len(demoAssets)
		}
		if d.Profile == "daily" {
			count = len(wallet)
		}
		for i := 0; i < count; i++ {
			asset := demoAssets[i%len(demoAssets)]
			notional := "1200"
			if d.Profile == "daily" {
				balance := wallet[i]
				if balance.AssetSymbol == "USD" || balance.AssetSymbol == "USDT" || balance.AssetSymbol == "USDC" || demoNumber(balance.Equity).Sign() <= 0 {
					continue
				}
				for _, candidate := range demoAssets {
					if candidate.Symbol == balance.AssetSymbol {
						asset = candidate
						break
					}
				}
				notional = demoMul(demoMul(balance.Equity, asset.Price), "0.65")
			} else {
				if count > 3 && (asset.Symbol == "USD" || asset.Symbol == "USDT" || asset.Symbol == "USDC" || asset.Symbol == "DAI") {
					continue
				}
				if count == 3 {
					asset = demoAssets[0]
				}
			}
			p := demoPosition(d, anchor, at, asset, i, inverse, notional)
			if d.Profile == "metadata" && i == 0 {
				p.MarginAssetSymbol, p.MarginBasis = "", ""
				p.FuturesContract.ContractSize, p.MarkPrice, p.Margin = nil, nil, nil
			}
			r.Positions = append(r.Positions, p)
		}
	}
	// Only the classic futures wallet owns this synthetic open/close event.
	historical := d.ID == "daily-classic" && r.AccountType == "um_futures"
	closed := !at.Before(DemoEventTime(anchor, -4))
	if historical && !at.Before(DemoEventTime(anchor, -8)) && !closed {
		p := demoPosition(d, anchor, at, demoAssets[2], 80, inverse, "1200")
		p.PositionKey = "historical-hedge"
		p.EntryPrice = adapters.OptionalText(DemoPrice(d.Platform, "SOL", DemoEventTime(anchor, -8)))
		p.UnrealizedPnl = adapters.OptionalText(demoPositionPnl(p))
		r.Positions = append(r.Positions, p)
	}
	amounts := map[string]string{}
	if d.Profile == "daily" {
		for _, b := range wallet {
			amounts[b.AssetSymbol] = b.Equity
		}
	} else if inverse {
		amounts["BTC"] = "0.06"
	} else {
		amounts["USDT"] = "6500"
	}
	if historical && closed {
		p := demoPosition(d, anchor, DemoEventTime(anchor, -4), demoAssets[2], 80, inverse, "1200")
		p.EntryPrice = adapters.OptionalText(DemoPrice(d.Platform, "SOL", DemoEventTime(anchor, -8)))
		amounts["USDT"] = demoAdd(amounts["USDT"], demoPositionPnl(p))
	}
	for _, p := range r.Positions {
		coin := p.FuturesContract.SettlementAssetSymbol
		if _, ok := amounts[coin]; !ok {
			amounts[coin] = "0"
			if d.Profile != "daily" {
				amounts[coin] = "0.05"
			}
		}
		amounts[coin] = demoAdd(amounts[coin], demoString(p.UnrealizedPnl))
	}
	for _, asset := range demoAssets {
		if amount, ok := amounts[asset.Symbol]; ok {
			balance := demoBalance(asset.Symbol, amount)
			if demoNumber(amount).Sign() < 0 {
				balance.AvailableToTrade, balance.AvailableToWithdraw = nil, nil
			}
			r.Balances = append(r.Balances, balance)
		}
	}
	if d.Profile == "large" && r.AccountType == "um_futures" {
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

func demoPosition(d DemoIdentity, anchor, at time.Time, asset demoAsset, i int, inverse bool, nominal string) adapters.Position {
	// Rolling 28-day cohorts represent explicit close/reopen at the same mark;
	// references, entry price and delivery time remain stable within each cohort.
	cohort := demoCohort(anchor, at, i%4)
	mark := DemoPrice(d.Platform, asset.Symbol, at)
	entry := DemoPrice(d.Platform, asset.Symbol, cohort)
	qty := demoDiv(nominal, asset.Price)
	kind, unit, settle, contract := adapters.KindLinearPerpetual, adapters.UnitBase, "USDT", "1"
	if inverse {
		kind, unit, settle, contract, qty = adapters.KindInversePerpetual, adapters.UnitContracts, asset.Symbol, "100", demoDiv(nominal, "100")
	} else if d.Platform == adapters.PlatformKraken {
		settle = "USD"
	}
	var expiry *time.Time
	if demoDatedLeg(d, inverse, asset.Symbol) || d.Profile != "daily" && i%3 == 2 {
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
	if inverse || d.Platform == adapters.PlatformKraken {
		p.FuturesContract.QuoteAssetSymbol = "USD"
	}
	if d.Profile == "daily" || i%2 == 1 {
		p.Side = adapters.SideShort
	}
	if d.Profile == "daily" && i%2 == 0 || d.Profile != "daily" && i%3 == 2 {
		p.PositionMode = "one_way"
	}
	if demoMode(d, anchor, at) == adapters.MarginBybitIsolated || d.Profile != "daily" && i%4 == 3 && d.Structure != adapters.StructureBinancePM {
		p.MarginSetting, p.MarginBasis = "isolated", "allocated"
	}
	notional := demoMul(qty, mark)
	if inverse {
		notional = demoMul(qty, contract)
	}
	if d.Profile == "daily" {
		p.Leverage = adapters.OptionalText([]string{"3", "5", "4"}[i%3])
	}
	margin := demoDiv(notional, demoString(p.Leverage))
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

// demoDatedLeg keeps a fixed delivery minority in the daily walkthrough, so
// current holdings and the Catalog cover perpetual and delivery contracts on
// every platform. Each named account holds one dated leg; the Binance classic
// COIN-M and Kraken Coin-M wallets use inverse settlement, the others linear.
func demoDatedLeg(d DemoIdentity, inverse bool, symbol string) bool {
	if d.Profile != "daily" {
		return false
	}
	switch d.ID {
	case "daily-classic":
		return inverse && symbol == "BTC" || !inverse && symbol == "ETH"
	case "daily-okx-usd", "daily-bybit":
		return !inverse && symbol == "ETH"
	case "daily-kraken":
		return inverse && symbol == "BTC"
	}
	return false
}

func demoFuturesContractSymbol(platform adapters.Platform, base string, inverse bool, expiry *time.Time) string {
	quote, term := "USDT", "PERP"
	if inverse || platform == adapters.PlatformKraken {
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
			if err := totals.Add("Demo earn", a.AssetSymbol, demoNumber(a.Amount)); err != nil {
				panic(err)
			}
		}
	}
	for _, p := range r.DualPositions {
		if err := totals.Add("Demo dual", p.PrincipalAssetSymbol, demoNumber(p.Principal)); err != nil {
			panic(err)
		}
	}
	r.Balances = totals.Balances()
}
