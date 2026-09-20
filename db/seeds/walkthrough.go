package seeds

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/catalog"
)

const (
	walkthroughAliceHash = `$argon2id$v=19$m=65536,t=3,p=2$4ZKp/JFnZmxIWjXk18zPWA$3wK/YAxLVIGX9PZpBe9MxZGrZpC+pw3nVpWfK6q2s2I`
	walkthroughBobHash   = `$argon2id$v=19$m=65536,t=3,p=2$2Iobo6Ik3eLKnVXVTJvdKg$6yClSIX/cdFRxSZXqB1LUHEDpU0ah/bdDgC+pIu4EW8`
	yearFreshness        = 31536000
	dayFreshness         = 86400
	seedTs               = "2006-01-02 15:04:05.000000"
)

// WalkthroughSQL returns the rich local walkthrough source-fact script.
func WalkthroughSQL() (string, error) {
	w := newWriter()
	w.comment("Deterministic walkthrough source-fact seed. Scratch logins:")
	w.comment("  alice / Alice=123456")
	w.comment("  bob / Bob=123456")
	w.comment("Do not insert projection rows.")
	w.users()
	w.portfolios()
	w.identities()
	w.connections()
	w.accounts()
	w.catalog()
	w.mappings()
	w.pins()
	w.collectionsAndFacts()
	w.capital()
	w.audits()
	return w.b.String(), nil
}

type writer struct {
	b         strings.Builder
	obs       uint64
	headers   [][]string
	balances  [][]string
	positions [][]string
	equities  [][]string
	coveredI  [][]string
	coveredA  [][]string
	metrics   [][]string
	prices    [][]string
	jobs      [][]string
	details   [][]string
	runs      [][]string
	snaps     [][]string
	fails     [][]string
	attempts  [][]string
}

func newWriter() *writer {
	return &writer{}
}

func (w *writer) comment(line string) {
	w.b.WriteString("-- ")
	w.b.WriteString(line)
	w.b.WriteByte('\n')
}

func (w *writer) insert(table, cols string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	w.b.WriteString("INSERT INTO ")
	w.b.WriteString(table)
	w.b.WriteString(" (")
	w.b.WriteString(cols)
	w.b.WriteString(") VALUES\n")
	for i, row := range rows {
		if i > 0 {
			w.b.WriteString(",\n")
		}
		w.b.WriteString("    (")
		w.b.WriteString(strings.Join(row, ", "))
		w.b.WriteByte(')')
	}
	w.b.WriteString(";\n\n")
}

func (w *writer) exec(stmt string) {
	w.b.WriteString(stmt)
	w.b.WriteString(";\n\n")
}

func (w *writer) u(id uuid.UUID) string {
	return "UNHEX('" + strings.ToUpper(hex.EncodeToString(id[:])) + "')"
}

func (w *writer) s(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func (w *writer) t(at time.Time) string {
	return w.s(at.UTC().Format(seedTs))
}

func (w *writer) n() string { return "NULL" }

func (w *writer) i(n int) string { return strconv.Itoa(n) }

func (w *writer) users() {
	created := ts(8, 1, 0)
	w.insert("users", "id, username, username_normalized, status, created_at, updated_at", [][]string{
		{w.u(UserAlice), w.s("alice"), w.s("alice"), w.s("active"), w.t(created), w.t(created)},
		{w.u(UserBob), w.s("bob"), w.s("bob"), w.s("active"), w.t(created), w.t(created)},
	})
	w.insert("user_credentials", "user_id, password_hash, password_changed_at, created_at, updated_at", [][]string{
		{w.u(UserAlice), w.s(walkthroughAliceHash), w.t(created), w.t(created), w.t(created)},
		{w.u(UserBob), w.s(walkthroughBobHash), w.t(created), w.t(created), w.t(created)},
	})
}

func (w *writer) portfolios() {
	created := ts(8, 1, 0)
	w.insert("portfolios", "id, user_id, name, timezone, display_asset_id, status, version, created_at, updated_at, archived_at", [][]string{
		{w.u(PortfolioAliceMain), w.u(UserAlice), w.s("Main"), w.s("Asia/Shanghai"), w.u(catalog.AssetUSD), w.s("active"), "1", w.t(created), w.t(created), w.n()},
		{w.u(PortfolioAliceTrading), w.u(UserAlice), w.s("Trading"), w.s("Asia/Shanghai"), w.u(catalog.AssetCNY), w.s("active"), "1", w.t(created), w.t(created), w.n()},
		{w.u(PortfolioBobMain), w.u(UserBob), w.s("Main"), w.s("UTC"), w.u(catalog.AssetUSD), w.s("active"), "1", w.t(created), w.t(created), w.n()},
	})
}

func (w *writer) identities() {
	created := ts(8, 1, 0)
	w.insert("identities", "id, portfolio_id, platform_id, parent_identity_id, external_ref, display_name, account_mode, kind, status, version, created_at, updated_at, archived_at", [][]string{
		{w.u(IdentityAliceBinance), w.u(PortfolioAliceMain), w.u(catalog.PlatformBinance), w.n(), w.s("uid-alice-binance"), w.s("Alice Binance"), w.s("binance_pm"), w.s("main"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
		{w.u(IdentityAliceOKX), w.u(PortfolioAliceTrading), w.u(catalog.PlatformOKX), w.n(), w.s("uid-alice-okx"), w.s("Alice OKX"), w.s("okx_mcm"), w.s("main"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
		{w.u(IdentityBobBinance), w.u(PortfolioBobMain), w.u(catalog.PlatformBinance), w.n(), w.s("uid-bob-binance"), w.s("Bob Binance"), w.s("binance_pm"), w.s("main"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
		{w.u(IdentityAliceBinanceSub), w.u(PortfolioAliceMain), w.u(catalog.PlatformBinance), w.u(IdentityAliceBinance), w.s("uid-alice-binance-sub"), w.s("Alice Binance Sub"), w.s("binance_pm"), w.s("subordinate"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
		{w.u(IdentityAliceBybit), w.u(PortfolioAliceMain), w.u(catalog.PlatformBybit), w.n(), w.s("uid-alice-bybit"), w.s("Alice Bybit"), w.s("bybit_uta_2_0"), w.s("main"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
		{w.u(IdentityAliceOKXPM), w.u(PortfolioAliceTrading), w.u(catalog.PlatformOKX), w.n(), w.s("uid-alice-okx-pm"), w.s("Alice OKX PM unused"), w.s("okx_pm"), w.s("main"), w.s("disabled"), "1", w.t(created), w.t(created), w.n()},
	})
}

func (w *writer) connections() {
	created := ts(8, 1, 0)
	w.insert("connections", "id, portfolio_id, identity_id, label, status, credentials_ciphertext, credentials_nonce, credential_key_version, config_json, collection_interval_seconds, freshness_after_seconds, last_validated_at, version, created_at, updated_at", [][]string{
		{w.u(ConnectionAliceBinance), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("Binance read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "900", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceOKX), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("OKX read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "300", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionBobBinance), w.u(PortfolioBobMain), w.u(IdentityBobBinance), w.s("Binance read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "900", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceBinanceSub), w.u(PortfolioAliceMain), w.u(IdentityAliceBinanceSub), w.s("Binance sub read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "1800", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceBybit), w.u(PortfolioAliceMain), w.u(IdentityAliceBybit), w.s("Bybit read"), w.s("invalid"), w.n(), w.n(), "0", w.s("{}"), "3600", w.i(dayFreshness), w.n(), "1", w.t(created), w.t(created)},
	})
}

func (w *writer) accounts() {
	created := ts(8, 1, 0)
	latest := ts(9, 18, 16)
	okxLatest := ts(9, 18, 8)
	w.insert("accounts", "id, portfolio_id, identity_id, external_ref, display_name, account_type, status, first_seen_at, last_seen_at, created_at, updated_at", [][]string{
		{w.u(AccountAliceUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("unified"), w.s("Unified"), w.s("unified"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceFunding), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("funding"), w.s("Funding"), w.s("funding"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceOKXTrading), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("trading"), w.s("Trading"), w.s("trading"), w.s("active"), w.t(created), w.t(okxLatest), w.t(created), w.t(okxLatest)},
		{w.u(AccountBobUnified), w.u(PortfolioBobMain), w.u(IdentityBobBinance), w.s("unified"), w.s("Unified"), w.s("unified"), w.s("active"), w.t(created), w.t(ts(9, 15, 0)), w.t(created), w.t(ts(9, 15, 0))},
		{w.u(AccountAliceEarn), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("earn"), w.s("Earn"), w.s("earn"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceSubUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBinanceSub), w.s("unified"), w.s("Sub unified"), w.s("unified"), w.s("active"), w.t(ts(9, 1, 0)), w.t(latest), w.t(ts(9, 1, 0)), w.t(latest)},
		{w.u(AccountAliceBybitUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBybit), w.s("unified"), w.s("UTA"), w.s("unified"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceOKXFunding), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("funding"), w.s("Funding"), w.s("funding"), w.s("active"), w.t(created), w.t(okxLatest), w.t(created), w.t(okxLatest)},
		{w.u(AccountAliceDisappeared), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("legacy-futures"), w.s("Legacy futures"), w.s("futures"), w.s("disappeared"), w.t(created), w.t(ts(8, 20, 0)), w.t(created), w.t(ts(8, 20, 0))},
	})
}

func (w *writer) catalog() {
	created := ts(8, 1, 0)
	latest := ts(9, 18, 16)
	w.insert("assets", "id, code, name, kind, decimals, status, created_at, updated_at", [][]string{
		{w.u(AssetUSDT), w.s("USDT"), w.s("Tether USD"), w.s("crypto"), "6", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetBTC), w.s("BTC"), w.s("Bitcoin"), w.s("crypto"), "8", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetTOKENX), w.s("TOKENX"), w.s("Provisional token"), w.s("crypto"), "8", w.s("provisional"), w.t(created), w.t(created)},
		{w.u(AssetETH), w.s("ETH"), w.s("Ethereum"), w.s("crypto"), "18", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetSOL), w.s("SOL"), w.s("Solana"), w.s("crypto"), "9", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetUSDC), w.s("USDC"), w.s("USD Coin"), w.s("crypto"), "6", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetWBTC), w.s("WBTC"), w.s("Wrapped Bitcoin"), w.s("crypto"), "8", w.s("active"), w.t(created), w.t(created)},
	})
	w.insert("asset_listings", "id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at", [][]string{
		{w.u(ListingBinanceUSDT), w.u(catalog.PlatformBinance), w.s("USDT"), w.s("USDT"), w.s("Tether"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceBTC), w.u(catalog.PlatformBinance), w.s("BTC"), w.s("BTC"), w.s("Bitcoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceUSD), w.u(catalog.PlatformBinance), w.s("USD"), w.s("USD"), w.s("US Dollar"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceTOKENX), w.u(catalog.PlatformBinance), w.s("TOKENX"), w.s("TOKENX"), w.s("Provisional token"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceETH), w.u(catalog.PlatformBinance), w.s("ETH"), w.s("ETH"), w.s("Ethereum"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceSOL), w.u(catalog.PlatformBinance), w.s("SOL"), w.s("SOL"), w.s("Solana"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceUSDC), w.u(catalog.PlatformBinance), w.s("USDC"), w.s("USDC"), w.s("USD Coin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceCNY), w.u(catalog.PlatformBinance), w.s("CNY"), w.s("CNY"), w.s("Chinese Yuan"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceWBTC), w.u(catalog.PlatformBinance), w.s("WBTC"), w.s("WBTC"), w.s("Wrapped Bitcoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceUnknown), w.u(catalog.PlatformBinance), w.s("UNK"), w.s("UNK"), w.s("Unmapped listing"), w.s("unknown"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXUSDT), w.u(catalog.PlatformOKX), w.s("USDT"), w.s("USDT"), w.s("Tether"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXBTC), w.u(catalog.PlatformOKX), w.s("BTC"), w.s("BTC"), w.s("Bitcoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXETH), w.u(catalog.PlatformOKX), w.s("ETH"), w.s("ETH"), w.s("Ethereum"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXUSD), w.u(catalog.PlatformOKX), w.s("USD"), w.s("USD"), w.s("US Dollar"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXCNY), w.u(catalog.PlatformOKX), w.s("CNY"), w.s("CNY"), w.s("Chinese Yuan"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitUSDT), w.u(catalog.PlatformBybit), w.s("USDT"), w.s("USDT"), w.s("Tether"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitBTC), w.u(catalog.PlatformBybit), w.s("BTC"), w.s("BTC"), w.s("Bitcoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitETH), w.u(catalog.PlatformBybit), w.s("ETH"), w.s("ETH"), w.s("Ethereum"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitUSD), w.u(catalog.PlatformBybit), w.s("USD"), w.s("USD"), w.s("US Dollar"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitTOKENX), w.u(catalog.PlatformBybit), w.s("TOKENX"), w.s("TOKENX"), w.s("Provisional token"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
	})
	expiry := ts(9, 25, 8)
	w.insert("instruments", "id, platform_id, external_ref, symbol, kind, base_listing_id, quote_listing_id, underlying_listing_id, settlement_listing_id, expiry_at, contract_size, status, first_seen_at, last_seen_at, created_at, updated_at", [][]string{
		{w.u(InstrumentBinanceBTCUSDT), w.u(catalog.PlatformBinance), w.s("BTCUSDT"), w.s("BTCUSDT"), w.s("linear_perpetual"), w.u(ListingBinanceBTC), w.u(ListingBinanceUSDT), w.u(ListingBinanceBTC), w.u(ListingBinanceUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBinanceETHUSDT), w.u(catalog.PlatformBinance), w.s("ETHUSDT"), w.s("ETHUSDT"), w.s("linear_perpetual"), w.u(ListingBinanceETH), w.u(ListingBinanceUSDT), w.u(ListingBinanceETH), w.u(ListingBinanceUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBinanceBTCUSDInv), w.u(catalog.PlatformBinance), w.s("BTCUSD_PERP"), w.s("BTCUSD"), w.s("inverse_perpetual"), w.u(ListingBinanceBTC), w.u(ListingBinanceUSD), w.u(ListingBinanceBTC), w.u(ListingBinanceBTC), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBinanceBTCDelivery), w.u(catalog.PlatformBinance), w.s("BTCUSDT_250925"), w.s("BTCUSDT_250925"), w.s("linear_delivery"), w.u(ListingBinanceBTC), w.u(ListingBinanceUSDT), w.u(ListingBinanceBTC), w.u(ListingBinanceUSDT), w.t(expiry), "1", w.s("expired"), w.t(created), w.t(expiry), w.t(created), w.t(expiry)},
		{w.u(InstrumentBinanceBTCSpot), w.u(catalog.PlatformBinance), w.s("BTCUSDT_SPOT"), w.s("BTCUSDT"), w.s("spot"), w.u(ListingBinanceBTC), w.u(ListingBinanceUSDT), w.n(), w.n(), w.n(), w.n(), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentOKXBTCUSDT), w.u(catalog.PlatformOKX), w.s("BTC-USDT-SWAP"), w.s("BTC-USDT-SWAP"), w.s("linear_perpetual"), w.u(ListingOKXBTC), w.u(ListingOKXUSDT), w.u(ListingOKXBTC), w.u(ListingOKXUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentOKXETHUSDT), w.u(catalog.PlatformOKX), w.s("ETH-USDT-SWAP"), w.s("ETH-USDT-SWAP"), w.s("linear_perpetual"), w.u(ListingOKXETH), w.u(ListingOKXUSDT), w.u(ListingOKXETH), w.u(ListingOKXUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBybitBTCUSDT), w.u(catalog.PlatformBybit), w.s("BTCUSDT"), w.s("BTCUSDT"), w.s("linear_perpetual"), w.u(ListingBybitBTC), w.u(ListingBybitUSDT), w.u(ListingBybitBTC), w.u(ListingBybitUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
	})
}

func (w *writer) mappings() {
	created := ts(8, 1, 0)
	ethConfirmed := ts(8, 3, 0)
	type mapSpec struct {
		mapping, listing, asset, portfolio uuid.UUID
		state, reason, override, class     string
		user                               uuid.UUID
		version                            int
	}
	specs := []mapSpec{
		{SeedID(kindMapping, 1), ListingBinanceUSDT, AssetUSDT, PortfolioAliceMain, "confirmed", "walkthrough seed", "", `{"role":"stablecoin"}`, UserAlice, 1},
		{SeedID(kindMapping, 2), ListingBinanceBTC, AssetBTC, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 3), ListingBinanceUSD, catalog.AssetUSD, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 4), ListingBinanceUSDT, AssetUSDT, PortfolioBobMain, "confirmed", "walkthrough seed", "", "", UserBob, 1},
		{SeedID(kindMapping, 5), ListingBinanceUSD, catalog.AssetUSD, PortfolioBobMain, "confirmed", "walkthrough seed", "", "", UserBob, 1},
		{SeedID(kindMapping, 6), ListingBinanceTOKENX, AssetTOKENX, PortfolioAliceMain, "provisional", "edge-cases provisional listing", "", "", UserAlice, 1},
		{SeedID(kindMapping, 7), ListingBinanceETH, AssetETH, PortfolioAliceMain, "confirmed", "confirmed after review", "", "", UserAlice, 2},
		{SeedID(kindMapping, 8), ListingBinanceSOL, AssetSOL, PortfolioAliceMain, "confirmed", "walkthrough seed", "Solana", "", UserAlice, 1},
		{SeedID(kindMapping, 9), ListingBinanceUSDC, AssetUSDC, PortfolioAliceMain, "confirmed", "walkthrough seed", "", `{"role":"stablecoin"}`, UserAlice, 1},
		{SeedID(kindMapping, 10), ListingBinanceCNY, catalog.AssetCNY, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 11), ListingBinanceWBTC, AssetWBTC, PortfolioAliceMain, "confirmed", "wrapped remains distinct", "", "", UserAlice, 1},
		{SeedID(kindMapping, 12), ListingBybitUSDT, AssetUSDT, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 13), ListingBybitBTC, AssetBTC, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 14), ListingBybitETH, AssetETH, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 15), ListingBybitUSD, catalog.AssetUSD, PortfolioAliceMain, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 16), ListingBybitTOKENX, AssetTOKENX, PortfolioAliceMain, "provisional", "provisional Bybit listing", "", "", UserAlice, 1},
		{SeedID(kindMapping, 17), ListingOKXUSDT, AssetUSDT, PortfolioAliceTrading, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 18), ListingOKXBTC, AssetBTC, PortfolioAliceTrading, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 19), ListingOKXETH, AssetETH, PortfolioAliceTrading, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 20), ListingOKXUSD, catalog.AssetUSD, PortfolioAliceTrading, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
		{SeedID(kindMapping, 21), ListingOKXCNY, catalog.AssetCNY, PortfolioAliceTrading, "confirmed", "walkthrough seed", "", "", UserAlice, 1},
	}
	var heads [][]string
	var revs [][]string
	var updates []string
	revN := uint64(1)
	for i := range specs {
		spec := specs[i]
		heads = append(heads, []string{w.u(spec.mapping), w.u(spec.portfolio), w.u(spec.listing), w.n(), w.i(spec.version), w.t(created), w.t(created)})
		if spec.mapping == SeedID(kindMapping, 7) {
			provisional := SeedID(kindMappingRevision, revN)
			revN++
			revs = append(revs, []string{w.u(provisional), w.u(spec.portfolio), w.u(spec.mapping), w.u(spec.asset), w.s("provisional"), w.n(), w.n(), w.s("newly discovered"), w.u(spec.user), w.t(created)})
			confirmed := SeedID(kindMappingRevision, revN)
			revN++
			override := w.n()
			class := w.n()
			revs = append(revs, []string{w.u(confirmed), w.u(spec.portfolio), w.u(spec.mapping), w.u(spec.asset), w.s("confirmed"), override, class, w.s(spec.reason), w.u(spec.user), w.t(ethConfirmed)})
			updates = append(updates, "UPDATE asset_listing_mappings SET current_revision_id = "+w.u(confirmed)+", updated_at = "+w.t(ethConfirmed)+" WHERE id = "+w.u(spec.mapping))
			continue
		}
		rev := SeedID(kindMappingRevision, revN)
		revN++
		override := w.n()
		if spec.override != "" {
			override = w.s(spec.override)
		}
		class := w.n()
		if spec.class != "" {
			class = w.s(spec.class)
		}
		revs = append(revs, []string{w.u(rev), w.u(spec.portfolio), w.u(spec.mapping), w.u(spec.asset), w.s(spec.state), override, class, w.s(spec.reason), w.u(spec.user), w.t(created)})
		updates = append(updates, "UPDATE asset_listing_mappings SET current_revision_id = "+w.u(rev)+" WHERE id = "+w.u(spec.mapping))
	}
	w.insert("asset_listing_mappings", "id, portfolio_id, asset_listing_id, current_revision_id, version, created_at, updated_at", heads)
	w.insert("asset_listing_mapping_revisions", "id, portfolio_id, mapping_id, asset_id, state, display_name_override, classification_json, reason, created_by_user_id, created_at", revs)
	for _, stmt := range updates {
		w.exec(stmt)
	}
}

func (w *writer) pins() {
	created := ts(8, 2, 0)
	disabledAt := ts(9, 1, 0)
	pinBTC := SeedID(kindPin, 1)
	pinInst := SeedID(kindPin, 2)
	pinETH := SeedID(kindPin, 3)
	rev1 := SeedID(kindPinRevision, 1)
	rev2 := SeedID(kindPinRevision, 2)
	rev3 := SeedID(kindPinRevision, 3)
	rev4 := SeedID(kindPinRevision, 4)
	w.insert("catalog_pins", "id, portfolio_id, asset_listing_id, instrument_id, current_revision_id, version, created_at, updated_at", [][]string{
		{w.u(pinBTC), w.u(PortfolioAliceMain), w.u(ListingBinanceBTC), w.n(), w.n(), "1", w.t(created), w.t(created)},
		{w.u(pinInst), w.u(PortfolioAliceMain), w.n(), w.u(InstrumentBinanceBTCUSDT), w.n(), "1", w.t(created), w.t(created)},
		{w.u(pinETH), w.u(PortfolioAliceMain), w.u(ListingBinanceETH), w.n(), w.n(), "2", w.t(created), w.t(disabledAt)},
	})
	w.insert("catalog_pin_revisions", "id, portfolio_id, catalog_pin_id, enabled, reason, created_by_user_id, created_at", [][]string{
		{w.u(rev1), w.u(PortfolioAliceMain), w.u(pinBTC), "1", w.s("track BTC prices"), w.u(UserAlice), w.t(created)},
		{w.u(rev2), w.u(PortfolioAliceMain), w.u(pinInst), "1", w.s("track BTC perpetual mark"), w.u(UserAlice), w.t(created)},
		{w.u(rev3), w.u(PortfolioAliceMain), w.u(pinETH), "1", w.s("track ETH"), w.u(UserAlice), w.t(created)},
		{w.u(rev4), w.u(PortfolioAliceMain), w.u(pinETH), "0", w.s("stop extra ETH tracking"), w.u(UserAlice), w.t(disabledAt)},
	})
	w.exec("UPDATE catalog_pins SET current_revision_id = " + w.u(rev1) + " WHERE id = " + w.u(pinBTC))
	w.exec("UPDATE catalog_pins SET current_revision_id = " + w.u(rev2) + " WHERE id = " + w.u(pinInst))
	w.exec("UPDATE catalog_pins SET current_revision_id = " + w.u(rev4) + ", updated_at = " + w.t(disabledAt) + " WHERE id = " + w.u(pinETH))
}

func (w *writer) collectionsAndFacts() {
	binanceTimes := []time.Time{
		ts(8, 4, 0),
		ts(8, 11, 0),
		ts(8, 18, 0),
		ts(8, 25, 0),
		ts(8, 31, 16),
		ts(9, 4, 0),
		ts(9, 8, 0),
		ts(9, 11, 0),
		ts(9, 15, 0),
		ts(9, 18, 16),
	}
	flavors := []string{"complete", "complete", "complete", "complete", "containment", "complete", "complete", "complete", "overlap", "partial"}
	triggers := []string{"schedule", "schedule", "manual", "schedule", "schedule", "manual", "schedule", "retry", "schedule", "manual"}
	for i, at := range binanceTimes {
		n := uint64(i + 1)
		jobStatus, runStatus, snapStatus, complete := "succeeded", "succeeded", "complete", "complete"
		success, failed := 12, 0
		if flavors[i] == "partial" {
			jobStatus, runStatus, snapStatus, complete = "succeeded", "partial", "partial", "partial"
			success, failed = 8, 3
		}
		run, snap := w.addCollection(n, PortfolioAliceMain, ConnectionAliceBinance, IdentityAliceBinance, UserAlice, at, triggers[i], jobStatus, runStatus, snapStatus, complete, success, failed, "", "")
		w.binanceFacts(i, at, run, snap, flavors[i])
	}

	bybitTimes := []time.Time{ts(8, 18, 0), ts(9, 8, 0), ts(9, 18, 16)}
	for i, at := range bybitTimes {
		n := uint64(20 + i)
		flavor := "complete"
		success, failed := 6, 0
		runStatus, snapStatus, complete := "succeeded", "complete", "complete"
		if i == 2 {
			flavor = "tokenx"
			success, failed = 4, 1
			runStatus, snapStatus, complete = "partial", "partial", "partial"
		}
		run, snap := w.addCollection(n, PortfolioAliceMain, ConnectionAliceBybit, IdentityAliceBybit, UserAlice, at, "schedule", "succeeded", runStatus, snapStatus, complete, success, failed, "", "")
		w.bybitFacts(i, at, run, snap, flavor)
	}

	failAt := ts(9, 17, 12)
	failRun, failSnap := w.addCollection(24, PortfolioAliceMain, ConnectionAliceBybit, IdentityAliceBybit, UserAlice, failAt, "retry", "failed", "failed", "failed", "partial", 0, 1, "adapter_unavailable", "No Platform adapter is registered.")
	w.fails = append(w.fails, []string{
		w.u(SeedID(kindFailure, 10)), w.u(PortfolioAliceMain), w.u(failRun), w.u(failSnap), w.n(),
		w.s("identity"), w.s("uid-alice-bybit"), w.s("collect"), w.s("unsupported"), w.s("adapter_unavailable"),
		w.s("No Platform adapter is registered."), "0", w.t(failAt.Add(5 * time.Second)),
	})

	subTimes := []time.Time{ts(9, 8, 0), ts(9, 18, 16)}
	for i, at := range subTimes {
		_, snap := w.addCollection(uint64(40+i), PortfolioAliceMain, ConnectionAliceBinanceSub, IdentityAliceBinanceSub, UserAlice, at, "manual", "succeeded", "succeeded", "complete", "complete", 2, 0, "", "")
		w.subFacts(i, at, snap)
	}

	okxTimes := []time.Time{ts(8, 15, 0), ts(9, 1, 0), ts(9, 12, 0), ts(9, 18, 8)}
	for i, at := range okxTimes {
		_, snap := w.addCollection(uint64(30+i), PortfolioAliceTrading, ConnectionAliceOKX, IdentityAliceOKX, UserAlice, at, "schedule", "succeeded", "succeeded", "complete", "complete", 8, 0, "", "")
		w.okxFacts(i, at, snap)
	}

	bobAt := ts(9, 15, 0)
	_, bobSnap := w.addCollection(50, PortfolioBobMain, ConnectionBobBinance, IdentityBobBinance, UserBob, bobAt, "manual", "succeeded", "succeeded", "complete", "complete", 3, 0, "", "")
	w.bobFacts(bobAt, bobSnap)

	cancelAt := ts(9, 16, 10)
	w.jobs = append(w.jobs, []string{
		w.u(SeedID(kindJob, 61)), w.u(PortfolioAliceMain), w.s("collection"), w.s("cancelled"), "10",
		w.s("collection:identity:cancelled"), w.u(UserAlice), w.t(cancelAt), w.n(), w.n(), "0", "5",
		w.n(), w.n(), w.t(cancelAt), w.t(cancelAt), w.n(), w.t(cancelAt),
	})
	w.details = append(w.details, []string{w.u(PortfolioAliceMain), w.u(SeedID(kindJob, 61)), w.u(ConnectionAliceBinance), w.s("manual"), w.s("identity")})

	w.flushFacts()
}

func (w *writer) addCollection(n uint64, portfolio, connection, identity, actor uuid.UUID, at time.Time, trigger, jobStatus, runStatus, snapStatus, completeness string, success, failed int, errCode, errDetail string) (uuid.UUID, uuid.UUID) {
	job := SeedID(kindJob, n)
	run := SeedID(kindRun, n)
	snap := SeedID(kindSnapshot, n)
	attempt := SeedID(kindAttempt, n)
	finished := at.Add(5 * time.Second)
	errC, errD := w.n(), w.n()
	if errCode != "" {
		errC, errD = w.s(errCode), w.s(errDetail)
	}
	started := w.t(at)
	finishedS := w.t(finished)
	if jobStatus == "cancelled" {
		started, finishedS = w.n(), w.t(at)
	}
	w.jobs = append(w.jobs, []string{
		w.u(job), w.u(portfolio), w.s("collection"), w.s(jobStatus), "10",
		w.s("collection:identity:" + strconv.FormatUint(n, 10)), w.u(actor), w.t(at), w.n(), w.n(),
		"1", "5", errC, errD, w.t(at), w.t(finished), started, finishedS,
	})
	w.details = append(w.details, []string{w.u(portfolio), w.u(job), w.u(connection), w.s(trigger), w.s("identity")})
	w.runs = append(w.runs, []string{
		w.u(run), w.u(portfolio), w.u(job), w.u(connection), w.u(identity), w.s("seed"), w.s("1"),
		w.t(at), w.t(finished), w.s(runStatus), w.i(success), w.i(failed), w.t(at), w.t(finished),
	})
	w.snaps = append(w.snaps, []string{
		w.u(snap), w.u(portfolio), w.u(run), w.u(identity), w.t(at), w.t(finished),
		w.s(snapStatus), w.s(completeness), w.i(success + failed), w.i(success), w.i(failed), w.t(at), w.t(finished),
	})
	outcome := "succeeded"
	if jobStatus == "failed" {
		outcome = "failed"
	}
	if jobStatus != "cancelled" {
		class, code, detail := w.n(), w.n(), w.n()
		if jobStatus == "failed" {
			class, code, detail = w.s("unsupported"), w.s(errCode), w.s(errDetail)
		}
		w.attempts = append(w.attempts, []string{
			w.u(attempt), w.u(portfolio), w.u(job), "1", w.s("seed-worker"),
			w.t(at), w.t(finished), w.s(outcome), class, code, detail, w.n(),
		})
	}
	return run, snap
}

func (w *writer) binanceFacts(i int, at time.Time, run, snap uuid.UUID, flavor string) {
	collected := at.Add(5 * time.Second)
	priceAt := at.Add(-3 * time.Minute)
	portfolio := PortfolioAliceMain
	ident := IdentityAliceBinance
	unified := AccountAliceUnified
	btcPx := strconv.Itoa(48000 + i*800)
	ethPx := strconv.Itoa(3200 + i*70)
	solPx := strconv.Itoa(140 + i*6)
	eq := strconv.Itoa(28000 + i*1800)
	usdt := strconv.Itoa(8000 + i*350)
	if i == 0 {
		usdt = "8050"
	}
	locked := strconv.Itoa(400 + i*40)
	avail := strconv.Itoa(7600 + i*310)
	btc := decimal(0.40 + 0.02*float64(i))
	eth := decimal(3.0 + 0.15*float64(i))
	sol := decimal(20 + float64(i))
	usdc := strconv.Itoa(1500 + i*50)
	wbtc := decimal(0.05 + 0.002*float64(i))
	unrealBTC := strconv.Itoa(300 + i*40)
	unrealETH := strconv.Itoa(-80 - i*10)
	mmr := decimal(4.2 - 0.08*float64(i))
	longQty := "0.10"
	if i >= 2 {
		longQty = "0.12"
	}

	version := 1
	updated := collected
	if i == 0 {
		version = 2
		updated = ts(8, 5, 0)
	}
	usdtObs := w.header(portfolio, snap, ident, &unified, "balance", at, collected, "complete", version, updated)
	w.balances = append(w.balances, []string{w.u(portfolio), w.u(usdtObs), w.u(ListingBinanceUSDT), usdt, avail, locked})
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceBTC, btc, btc, "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceETH, eth, eth, "0")
	if flavor == "partial" {
		obs := w.header(portfolio, snap, ident, &unified, "balance", at, collected, "partial", 1, collected)
		w.balances = append(w.balances, []string{w.u(portfolio), w.u(obs), w.u(ListingBinanceSOL), w.n(), sol, w.n()})
	} else {
		w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceSOL, sol, sol, "0")
	}
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceUSDC, usdc, usdc, "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceWBTC, wbtc, wbtc, "0")
	w.balance(portfolio, snap, ident, AccountAliceFunding, at, collected, ListingBinanceUSDT, "2500", "2500", "0")
	w.balance(portfolio, snap, ident, AccountAliceEarn, at, collected, ListingBinanceUSDT, "1800", "0", "1800")

	if flavor != "partial" {
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBTCUSDT, "pos-alice-btc", "long", longQty, "45000", btcPx, unrealBTC, "2")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceETHUSDT, "pos-alice-eth", "short", "1.5", "3000", ethPx, unrealETH, "3")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBTCUSDInv, "pos-alice-btc-inv", "long", "100", btcPx, btcPx, "80", "1")
	}
	if flavor == "overlap" {
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBTCUSDT, "pos-alice-btc-short", "short", "0.10", btcPx, btcPx, "0", "2")
	}

	switch flavor {
	case "containment":
		idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingBinanceUSD, eq)
		w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
		acctRE := w.equity(portfolio, snap, ident, &unified, at, collected, ListingBinanceUSD, strconv.Itoa(25000+i*100))
		w.coveredA = append(w.coveredA, []string{w.u(portfolio), w.u(acctRE), w.u(unified)})
	case "overlap":
		a := w.equity(portfolio, snap, ident, &unified, at, collected, ListingBinanceUSD, "20000")
		w.coveredA = append(w.coveredA, []string{w.u(portfolio), w.u(a), w.u(unified)}, []string{w.u(portfolio), w.u(a), w.u(AccountAliceFunding)})
		b := w.equity(portfolio, snap, ident, &AccountAliceFunding, at, collected, ListingBinanceUSD, "18000")
		w.coveredA = append(w.coveredA, []string{w.u(portfolio), w.u(b), w.u(AccountAliceFunding)}, []string{w.u(portfolio), w.u(b), w.u(AccountAliceEarn)})
	case "partial":
		idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingBinanceUSDT, "48000")
		w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
		unk := w.header(portfolio, snap, ident, &unified, "balance", at, collected, "complete", 1, collected)
		w.balances = append(w.balances, []string{w.u(portfolio), w.u(unk), w.u(ListingBinanceUnknown), "12", "12", "0"})
		w.fails = append(w.fails,
			[]string{w.u(SeedID(kindFailure, 1)), w.u(portfolio), w.u(run), w.u(snap), w.u(unified), w.s("position"), w.s("pos-alice-btc"), w.s("collect"), w.s("platform_error"), w.s("position_unavailable"), w.s("Position collection failed for the unified account."), "1", w.t(collected)},
			[]string{w.u(SeedID(kindFailure, 2)), w.u(portfolio), w.u(run), w.u(snap), w.n(), w.s("price"), w.s("BTCUSDT"), w.s("collect"), w.s("platform_error"), w.s("mark_price_missing"), w.s("Mark price was not returned."), "1", w.t(collected)},
			[]string{w.u(SeedID(kindFailure, 3)), w.u(portfolio), w.u(run), w.u(snap), w.n(), w.s("price"), w.s("USDTUSD"), w.s("collect"), w.s("platform_error"), w.s("fx_missing"), w.s("USDT to USD exchange rate was not returned."), "1", w.t(collected)},
		)
	default:
		idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingBinanceUSD, eq)
		w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
	}

	metric := w.header(portfolio, snap, ident, &unified, "account_metric", at, collected, "complete", 1, collected)
	w.metrics = append(w.metrics, []string{w.u(portfolio), w.u(metric), w.s("UniMMR"), w.s("ratio"), mmr, w.n(), w.s("binance_pm_v1")})
	if i == 2 {
		text := w.header(portfolio, snap, ident, &unified, "account_metric", at, collected, "complete", 1, collected)
		w.metrics = append(w.metrics, []string{w.u(portfolio), w.u(text), w.s("marginMode"), w.s("enum"), w.n(), w.s("portfolio"), w.s("binance_pm_v1")})
	}

	if flavor != "partial" {
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceBTCUSDT, nil, ListingBinanceUSDT, "mark", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceBTCUSDT, nil, ListingBinanceUSDT, "market", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceBTCUSDT, nil, ListingBinanceUSDT, "index", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceETHUSDT, nil, ListingBinanceUSDT, "mark", ethPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceBTCUSDInv, nil, ListingBinanceUSD, "mark", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceUSDT, ListingBinanceUSD, "exchange_rate", "1")
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceUSDT, ListingBinanceCNY, "exchange_rate", "7.25")
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceETH, ListingBinanceUSDT, "exchange_rate", ethPx)
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceSOL, ListingBinanceUSDT, "exchange_rate", solPx)
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceUSDC, ListingBinanceUSDT, "exchange_rate", "1")
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceWBTC, ListingBinanceUSDT, "exchange_rate", btcPx)
	}
}

func (w *writer) bybitFacts(i int, at time.Time, run, snap uuid.UUID, flavor string) {
	collected := at.Add(5 * time.Second)
	priceAt := at.Add(-2 * time.Minute)
	portfolio := PortfolioAliceMain
	ident := IdentityAliceBybit
	acct := AccountAliceBybitUnified
	btcPx := strconv.Itoa(48100 + i*900)
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitUSDT, strconv.Itoa(2000+i*200), strconv.Itoa(2000+i*200), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitBTC, decimal(0.04+0.005*float64(i)), decimal(0.04+0.005*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitETH, decimal(1.2+0.1*float64(i)), decimal(1.2+0.1*float64(i)), "0")
	if flavor != "tokenx" {
		w.position(portfolio, snap, ident, acct, at, collected, InstrumentBybitBTCUSDT, "pos-alice-bybit-btc", "long", "0.05", "47000", btcPx, "50", "2")
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBybitBTCUSDT, nil, ListingBybitUSDT, "mark", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitUSDT, ListingBybitUSD, "exchange_rate", "1")
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitETH, ListingBybitUSDT, "exchange_rate", strconv.Itoa(3210+i*80))
	} else {
		w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitTOKENX, "12", "12", "0")
		w.fails = append(w.fails, []string{
			w.u(SeedID(kindFailure, 4)), w.u(portfolio), w.u(run), w.u(snap), w.n(),
			w.s("price"), w.s("TOKENX"), w.s("collect"), w.s("platform_error"), w.s("mark_price_missing"),
			w.s("TOKENX has no observed price."), "1", w.t(collected),
		})
	}
}

func (w *writer) subFacts(i int, at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	w.balance(PortfolioAliceMain, snap, IdentityAliceBinanceSub, AccountAliceSubUnified, at, collected, ListingBinanceUSDT, strconv.Itoa(400+i*80), strconv.Itoa(400+i*80), "0")
	w.price(PortfolioAliceMain, snap, IdentityAliceBinanceSub, at, collected, nil, &ListingBinanceUSDT, ListingBinanceUSD, "exchange_rate", "1")
}

func (w *writer) okxFacts(i int, at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	priceAt := at.Add(-1 * time.Minute)
	portfolio := PortfolioAliceTrading
	ident := IdentityAliceOKX
	btcPx := strconv.Itoa(47900 + i*850)
	ethPx := strconv.Itoa(3180 + i*90)
	usdt := strconv.Itoa(6000 + i*400)
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXUSDT, usdt, usdt, "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXBTC, decimal(0.22+0.03*float64(i)), decimal(0.22+0.03*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXFunding, at, collected, ListingOKXUSDT, "800", "800", "0")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXBTCUSDT, "pos-alice-okx-btc", "long", "0.08", "44000", btcPx, strconv.Itoa(120+i*30), "2")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXETHUSDT, "pos-alice-okx-eth", "short", "2.0", "3100", ethPx, strconv.Itoa(-40-i*8), "2")
	idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingOKXUSDT, strconv.Itoa(22000+i*1500))
	w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXBTCUSDT, nil, ListingOKXUSDT, "mark", btcPx)
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXETHUSDT, nil, ListingOKXUSDT, "mark", ethPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXUSDT, ListingOKXCNY, "exchange_rate", "7.24")
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXUSDT, ListingOKXUSD, "exchange_rate", "1")
	metric := w.header(portfolio, snap, ident, &AccountAliceOKXTrading, "account_metric", at, collected, "complete", 1, collected)
	w.metrics = append(w.metrics, []string{w.u(portfolio), w.u(metric), w.s("mgnRatio"), w.s("ratio"), decimal(8.5 - 0.2*float64(i)), w.n(), w.s("okx_mcm_v1")})
}

func (w *writer) bobFacts(at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceUSDT, "1000", "1000", "0")
	idRE := w.equity(PortfolioBobMain, snap, IdentityBobBinance, &AccountBobUnified, at, collected, ListingBinanceUSD, "1000")
	w.coveredI = append(w.coveredI, []string{w.u(PortfolioBobMain), w.u(idRE), w.u(IdentityBobBinance)})
	w.coveredA = append(w.coveredA, []string{w.u(PortfolioBobMain), w.u(idRE), w.u(AccountBobUnified)})
	w.price(PortfolioBobMain, snap, IdentityBobBinance, at, collected, nil, &ListingBinanceUSDT, ListingBinanceUSD, "exchange_rate", "1")
}

func (w *writer) flushFacts() {
	w.insert("jobs", "id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id, available_at, lease_owner, lease_expires_at, attempt_count, max_attempts, last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at", w.jobs)
	w.insert("collection_job_details", "portfolio_id, job_id, connection_id, `trigger`, scope", w.details)
	w.insert("collection_runs", "id, portfolio_id, job_id, connection_id, identity_id, adapter_name, adapter_version, started_at, finished_at, status, successful_scope_count, failed_scope_count, created_at, updated_at", w.runs)
	w.insert("identity_snapshots", "id, portfolio_id, collection_run_id, identity_id, started_at, completed_at, status, completeness, expected_scope_count, successful_scope_count, failed_scope_count, created_at, updated_at", w.snaps)
	w.insert("collection_failures", "id, portfolio_id, collection_run_id, identity_snapshot_id, account_id, scope_kind, scope_ref, phase, error_class, error_code, safe_detail, retriable, occurred_at", w.fails)
	w.insert("observations", "id, portfolio_id, identity_snapshot_id, identity_id, account_id, kind, source_observed_at, collected_at, completeness, adapter_name, adapter_version, schema_version, version, created_at, updated_at", w.headers)
	w.insert("balance_observations", "portfolio_id, observation_id, asset_listing_id, total_quantity, available_quantity, locked_quantity", w.balances)
	w.insert("position_observations", "portfolio_id, observation_id, instrument_id, platform_position_ref, side, quantity, entry_price, mark_price, unrealized_value, leverage", w.positions)
	w.insert("reported_equity_observations", "portfolio_id, observation_id, denomination_listing_id, amount", w.equities)
	w.insert("reported_equity_covered_identities", "portfolio_id, observation_id, identity_id", w.coveredI)
	w.insert("reported_equity_covered_accounts", "portfolio_id, observation_id, account_id", w.coveredA)
	w.insert("account_metric_observations", "portfolio_id, observation_id, metric_code, unit, decimal_value, text_value, semantics_version", w.metrics)
	w.insert("price_observations", "portfolio_id, observation_id, instrument_id, base_listing_id, quote_listing_id, price_type, price", w.prices)
	w.insert("job_attempts", "id, portfolio_id, job_id, attempt_number, worker_id, started_at, finished_at, outcome, error_class, error_code, safe_detail, retry_at", w.attempts)
}

func (w *writer) capital() {
	created := ts(8, 1, 0)
	baseMain := SeedID(kindCapitalBaseline, 1)
	baseTrade := SeedID(kindCapitalBaseline, 2)
	baseBob := SeedID(kindCapitalBaseline, 3)
	revMain := SeedID(kindBaselineRevision, 1)
	revTrade := SeedID(kindBaselineRevision, 2)
	revBob := SeedID(kindBaselineRevision, 3)
	w.insert("capital_baselines", "id, portfolio_id, current_revision_id, version, created_at, updated_at", [][]string{
		{w.u(baseMain), w.u(PortfolioAliceMain), w.n(), "1", w.t(created), w.t(created)},
		{w.u(baseTrade), w.u(PortfolioAliceTrading), w.n(), "1", w.t(created), w.t(created)},
		{w.u(baseBob), w.u(PortfolioBobMain), w.n(), "1", w.t(created), w.t(created)},
	})
	w.insert("capital_baseline_revisions", "id, portfolio_id, capital_baseline_id, effective_at, asset_id, quantity, note, created_by_user_id, created_at", [][]string{
		{w.u(revMain), w.u(PortfolioAliceMain), w.u(baseMain), w.t(created), w.u(catalog.AssetUSD), "30000", w.s("initial baseline"), w.u(UserAlice), w.t(created)},
		{w.u(revTrade), w.u(PortfolioAliceTrading), w.u(baseTrade), w.t(created), w.u(catalog.AssetCNY), "200000", w.s("CNY trading baseline"), w.u(UserAlice), w.t(created)},
		{w.u(revBob), w.u(PortfolioBobMain), w.u(baseBob), w.t(created), w.u(catalog.AssetUSD), "1000", w.s("bob baseline"), w.u(UserBob), w.t(created)},
	})
	w.exec("UPDATE capital_baselines SET current_revision_id = " + w.u(revMain) + " WHERE id = " + w.u(baseMain))
	w.exec("UPDATE capital_baselines SET current_revision_id = " + w.u(revTrade) + " WHERE id = " + w.u(baseTrade))
	w.exec("UPDATE capital_baselines SET current_revision_id = " + w.u(revBob) + " WHERE id = " + w.u(baseBob))

	flow1 := SeedID(kindCapitalFlow, 1)
	flow2 := SeedID(kindCapitalFlow, 2)
	flow3 := SeedID(kindCapitalFlow, 3)
	flow4 := SeedID(kindCapitalFlow, 4)
	flow5 := SeedID(kindCapitalFlow, 5)
	r1 := SeedID(kindFlowRevision, 1)
	r2 := SeedID(kindFlowRevision, 2)
	r3 := SeedID(kindFlowRevision, 3)
	r4 := SeedID(kindFlowRevision, 4)
	r5 := SeedID(kindFlowRevision, 5)
	r6 := SeedID(kindFlowRevision, 6)
	r7 := SeedID(kindFlowRevision, 7)
	w.insert("capital_flows", "id, portfolio_id, current_revision_id, version, created_at, updated_at", [][]string{
		{w.u(flow1), w.u(PortfolioAliceMain), w.n(), "2", w.t(ts(9, 10, 0)), w.t(ts(9, 11, 0))},
		{w.u(flow2), w.u(PortfolioAliceMain), w.n(), "1", w.t(ts(9, 16, 0)), w.t(ts(9, 16, 0))},
		{w.u(flow3), w.u(PortfolioAliceMain), w.n(), "2", w.t(ts(8, 20, 0)), w.t(ts(8, 21, 0))},
		{w.u(flow4), w.u(PortfolioAliceMain), w.n(), "1", w.t(ts(9, 4, 0)), w.t(ts(9, 4, 0))},
		{w.u(flow5), w.u(PortfolioAliceTrading), w.n(), "1", w.t(ts(9, 5, 0)), w.t(ts(9, 5, 0))},
	})
	w.insert("capital_flow_revisions", "id, portfolio_id, capital_flow_id, direction, quantity, asset_id, effective_at, account_id, note, state, created_by_user_id, created_at", [][]string{
		{w.u(r1), w.u(PortfolioAliceMain), w.u(flow1), w.s("contribution"), "5000", w.u(catalog.AssetUSD), w.t(ts(9, 10, 0)), w.u(AccountAliceFunding), w.s("external contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 10, 0))},
		{w.u(r3), w.u(PortfolioAliceMain), w.u(flow1), w.s("contribution"), "4000", w.u(catalog.AssetUSD), w.t(ts(9, 10, 0)), w.u(AccountAliceFunding), w.s("edited contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 11, 0))},
		{w.u(r2), w.u(PortfolioAliceMain), w.u(flow2), w.s("withdrawal"), "1000", w.u(catalog.AssetUSD), w.t(ts(9, 16, 0)), w.u(AccountAliceFunding), w.s("external withdrawal"), w.s("active"), w.u(UserAlice), w.t(ts(9, 16, 0))},
		{w.u(r4), w.u(PortfolioAliceMain), w.u(flow3), w.s("contribution"), "2500", w.u(catalog.AssetUSD), w.t(ts(8, 20, 0)), w.u(AccountAliceFunding), w.s("mistaken contribution"), w.s("active"), w.u(UserAlice), w.t(ts(8, 20, 0))},
		{w.u(r5), w.u(PortfolioAliceMain), w.u(flow3), w.s("contribution"), "2500", w.u(catalog.AssetUSD), w.t(ts(8, 20, 0)), w.u(AccountAliceFunding), w.s("deleted mistaken contribution"), w.s("deleted"), w.u(UserAlice), w.t(ts(8, 21, 0))},
		{w.u(r6), w.u(PortfolioAliceMain), w.u(flow4), w.s("contribution"), "2000", w.u(catalog.AssetUSD), w.t(ts(9, 4, 0)), w.u(AccountAliceFunding), w.s("September top-up"), w.s("active"), w.u(UserAlice), w.t(ts(9, 4, 0))},
		{w.u(r7), w.u(PortfolioAliceTrading), w.u(flow5), w.s("contribution"), "10000", w.u(catalog.AssetCNY), w.t(ts(9, 5, 0)), w.u(AccountAliceOKXFunding), w.s("CNY contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 5, 0))},
	})
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r3) + " WHERE id = " + w.u(flow1))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r2) + " WHERE id = " + w.u(flow2))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r5) + " WHERE id = " + w.u(flow3))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r6) + " WHERE id = " + w.u(flow4))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r7) + " WHERE id = " + w.u(flow5))
}

func (w *writer) audits() {
	w.insert("audit_events", "id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id, before_json, after_json, metadata_json, occurred_at", [][]string{
		{w.u(SeedID(kindAudit, 1)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("catalog_mapping.create"), w.s("catalog_mapping"), w.u(SeedID(kindMapping, 1)), w.s("seed-walkthrough-1"), w.n(), w.s(`{"state":"confirmed","asset":"USDT"}`), w.n(), w.t(ts(8, 1, 0))},
		{w.u(SeedID(kindAudit, 2)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("catalog_mapping.revise"), w.s("catalog_mapping"), w.u(SeedID(kindMapping, 7)), w.s("seed-walkthrough-2"), w.s(`{"state":"provisional"}`), w.s(`{"state":"confirmed"}`), w.n(), w.t(ts(8, 3, 0))},
		{w.u(SeedID(kindAudit, 3)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("catalog_pin.create"), w.s("catalog_pin"), w.u(SeedID(kindPin, 1)), w.s("seed-walkthrough-3"), w.n(), w.s(`{"enabled":true,"subject":"BTC"}`), w.n(), w.t(ts(8, 2, 0))},
		{w.u(SeedID(kindAudit, 4)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("catalog_pin.disable"), w.s("catalog_pin"), w.u(SeedID(kindPin, 3)), w.s("seed-walkthrough-4"), w.s(`{"enabled":true}`), w.s(`{"enabled":false}`), w.n(), w.t(ts(9, 1, 0))},
		{w.u(SeedID(kindAudit, 5)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("capital_flow.create"), w.s("capital_flow"), w.u(SeedID(kindCapitalFlow, 1)), w.s("seed-walkthrough-5"), w.n(), w.s(`{"direction":"contribution","quantity":"5000"}`), w.n(), w.t(ts(9, 10, 0))},
		{w.u(SeedID(kindAudit, 6)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("capital_flow.correct"), w.s("capital_flow"), w.u(SeedID(kindCapitalFlow, 1)), w.s("seed-walkthrough-6"), w.s(`{"quantity":"5000"}`), w.s(`{"quantity":"4000","note":"edited contribution"}`), w.n(), w.t(ts(9, 11, 0))},
		{w.u(SeedID(kindAudit, 7)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("capital_flow.delete"), w.s("capital_flow"), w.u(SeedID(kindCapitalFlow, 3)), w.s("seed-walkthrough-7"), w.s(`{"state":"active"}`), w.s(`{"state":"deleted"}`), w.n(), w.t(ts(8, 21, 0))},
		{w.u(SeedID(kindAudit, 8)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("observation.correct"), w.s("observation"), w.u(SeedID(kindObservation, 1)), w.s("seed-walkthrough-8"), w.s(`{"totalQuantity":"8000"}`), w.s(`{"totalQuantity":"8000","version":2}`), w.n(), w.t(ts(8, 5, 0))},
		{w.u(SeedID(kindAudit, 9)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("connection.create"), w.s("connection"), w.u(ConnectionAliceBinance), w.s("seed-walkthrough-9"), w.n(), w.s(`{"label":"Binance read","status":"disabled","credentialsConfigured":false}`), w.n(), w.t(ts(8, 1, 0))},
		{w.u(SeedID(kindAudit, 10)), w.u(PortfolioAliceTrading), w.u(UserAlice), w.s("capital_flow.create"), w.s("capital_flow"), w.u(SeedID(kindCapitalFlow, 5)), w.s("seed-walkthrough-10"), w.n(), w.s(`{"direction":"contribution","quantity":"10000","asset":"CNY"}`), w.n(), w.t(ts(9, 5, 0))},
	})
}

func (w *writer) header(portfolio, snap, ident uuid.UUID, account *uuid.UUID, kind string, observed, collected time.Time, completeness string, version int, updated time.Time) uuid.UUID {
	w.obs++
	id := SeedID(kindObservation, w.obs)
	acc := w.n()
	if account != nil {
		acc = w.u(*account)
	}
	w.headers = append(w.headers, []string{
		w.u(id), w.u(portfolio), w.u(snap), w.u(ident), acc, w.s(kind),
		w.t(observed), w.t(collected), w.s(completeness), w.s("seed"), w.s("1"), w.s("1"),
		w.i(version), w.t(collected), w.t(updated),
	})
	return id
}

func (w *writer) balance(portfolio, snap, ident, account uuid.UUID, at, collected time.Time, listing uuid.UUID, total, available, locked string) {
	obs := w.header(portfolio, snap, ident, &account, "balance", at, collected, "complete", 1, collected)
	w.balances = append(w.balances, []string{w.u(portfolio), w.u(obs), w.u(listing), total, available, locked})
}

func (w *writer) position(portfolio, snap, ident, account uuid.UUID, at, collected time.Time, instrument uuid.UUID, ref, side, qty, entry, mark, unreal, lev string) {
	obs := w.header(portfolio, snap, ident, &account, "position", at, collected, "complete", 1, collected)
	w.positions = append(w.positions, []string{w.u(portfolio), w.u(obs), w.u(instrument), w.s(ref), w.s(side), qty, entry, mark, unreal, lev})
}

func (w *writer) equity(portfolio, snap, ident uuid.UUID, account *uuid.UUID, at, collected time.Time, listing uuid.UUID, amount string) uuid.UUID {
	obs := w.header(portfolio, snap, ident, account, "reported_equity", at, collected, "complete", 1, collected)
	w.equities = append(w.equities, []string{w.u(portfolio), w.u(obs), w.u(listing), amount})
	return obs
}

func (w *writer) price(portfolio, snap, ident uuid.UUID, at, collected time.Time, instrument, base *uuid.UUID, quote uuid.UUID, priceType, price string) {
	obs := w.header(portfolio, snap, ident, nil, "price", at, collected, "complete", 1, collected)
	inst, baseS := w.n(), w.n()
	if instrument != nil {
		inst = w.u(*instrument)
	}
	if base != nil {
		baseS = w.u(*base)
	}
	w.prices = append(w.prices, []string{w.u(portfolio), w.u(obs), inst, baseS, w.u(quote), w.s(priceType), price})
}

func ts(month, day, hour int) time.Time {
	return time.Date(2026, time.Month(month), day, hour, 0, 0, 0, time.UTC)
}

func decimal(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}
