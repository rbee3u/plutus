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
		{w.u(IdentityAliceOKXMain), w.u(PortfolioAliceMain), w.u(catalog.PlatformOKX), w.n(), w.s("uid-alice-okx-main"), w.s("Alice OKX Main"), w.s("okx_mcm"), w.s("main"), w.s("enabled"), "1", w.t(created), w.t(created), w.n()},
	})
}

func (w *writer) connections() {
	created := ts(8, 1, 0)
	w.insert("connections", "id, portfolio_id, identity_id, label, status, credentials_ciphertext, credentials_nonce, credential_key_version, config_json, collection_interval_seconds, freshness_after_seconds, last_validated_at, version, created_at, updated_at", [][]string{
		{w.u(ConnectionAliceBinance), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("Binance read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "900", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceOKX), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("OKX read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "300", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionBobBinance), w.u(PortfolioBobMain), w.u(IdentityBobBinance), w.s("Binance read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "900", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceBinanceSub), w.u(PortfolioAliceMain), w.u(IdentityAliceBinanceSub), w.s("Binance sub read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "1800", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceBybit), w.u(PortfolioAliceMain), w.u(IdentityAliceBybit), w.s("Bybit read"), w.s("invalid"), w.n(), w.n(), "0", w.s("{}"), "3600", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
		{w.u(ConnectionAliceOKXMain), w.u(PortfolioAliceMain), w.u(IdentityAliceOKXMain), w.s("OKX Main read"), w.s("disabled"), w.n(), w.n(), "0", w.s("{}"), "900", w.i(yearFreshness), w.n(), "1", w.t(created), w.t(created)},
	})
}

func (w *writer) accounts() {
	created := ts(8, 1, 0)
	latest := ts(9, 20, 16)
	okxLatest := ts(9, 20, 8)
	w.insert("accounts", "id, portfolio_id, identity_id, external_ref, display_name, account_type, status, first_seen_at, last_seen_at, created_at, updated_at", [][]string{
		{w.u(AccountAliceUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("unified"), w.s("Unified"), w.s("unified"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceFunding), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("funding"), w.s("Funding"), w.s("funding"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceOKXTrading), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("trading"), w.s("Trading"), w.s("trading"), w.s("active"), w.t(created), w.t(okxLatest), w.t(created), w.t(okxLatest)},
		{w.u(AccountBobUnified), w.u(PortfolioBobMain), w.u(IdentityBobBinance), w.s("unified"), w.s("Unified"), w.s("unified"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceEarn), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("earn"), w.s("Earn"), w.s("earn"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceSubUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBinanceSub), w.s("unified"), w.s("Sub unified"), w.s("unified"), w.s("active"), w.t(ts(9, 1, 0)), w.t(latest), w.t(ts(9, 1, 0)), w.t(latest)},
		{w.u(AccountAliceBybitUnified), w.u(PortfolioAliceMain), w.u(IdentityAliceBybit), w.s("unified"), w.s("UTA"), w.s("unified"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceOKXFunding), w.u(PortfolioAliceTrading), w.u(IdentityAliceOKX), w.s("funding"), w.s("Funding"), w.s("funding"), w.s("active"), w.t(created), w.t(okxLatest), w.t(created), w.t(okxLatest)},
		{w.u(AccountAliceDisappeared), w.u(PortfolioAliceMain), w.u(IdentityAliceBinance), w.s("legacy-futures"), w.s("Legacy futures"), w.s("futures"), w.s("disappeared"), w.t(created), w.t(ts(8, 20, 0)), w.t(created), w.t(ts(8, 20, 0))},
		{w.u(AccountAliceOKXMainTrading), w.u(PortfolioAliceMain), w.u(IdentityAliceOKXMain), w.s("trading"), w.s("OKX Trading"), w.s("trading"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(AccountAliceOKXMainFunding), w.u(PortfolioAliceMain), w.u(IdentityAliceOKXMain), w.s("funding"), w.s("OKX Funding"), w.s("funding"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
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
		{w.u(AssetBNB), w.s("BNB"), w.s("BNB"), w.s("crypto"), "8", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetXRP), w.s("XRP"), w.s("XRP"), w.s("crypto"), "6", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetDOGE), w.s("DOGE"), w.s("Dogecoin"), w.s("crypto"), "8", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetADA), w.s("ADA"), w.s("Cardano"), w.s("crypto"), "6", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetAVAX), w.s("AVAX"), w.s("Avalanche"), w.s("crypto"), "9", w.s("active"), w.t(created), w.t(created)},
		{w.u(AssetLINK), w.s("LINK"), w.s("Chainlink"), w.s("crypto"), "8", w.s("active"), w.t(created), w.t(created)},
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
		{w.u(ListingBinanceBNB), w.u(catalog.PlatformBinance), w.s("BNB"), w.s("BNB"), w.s("BNB"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceXRP), w.u(catalog.PlatformBinance), w.s("XRP"), w.s("XRP"), w.s("XRP"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceDOGE), w.u(catalog.PlatformBinance), w.s("DOGE"), w.s("DOGE"), w.s("Dogecoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceADA), w.u(catalog.PlatformBinance), w.s("ADA"), w.s("ADA"), w.s("Cardano"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceAVAX), w.u(catalog.PlatformBinance), w.s("AVAX"), w.s("AVAX"), w.s("Avalanche"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBinanceLINK), w.u(catalog.PlatformBinance), w.s("LINK"), w.s("LINK"), w.s("Chainlink"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXBNB), w.u(catalog.PlatformOKX), w.s("BNB"), w.s("BNB"), w.s("BNB"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXSOL), w.u(catalog.PlatformOKX), w.s("SOL"), w.s("SOL"), w.s("Solana"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXXRP), w.u(catalog.PlatformOKX), w.s("XRP"), w.s("XRP"), w.s("XRP"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXDOGE), w.u(catalog.PlatformOKX), w.s("DOGE"), w.s("DOGE"), w.s("Dogecoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXADA), w.u(catalog.PlatformOKX), w.s("ADA"), w.s("ADA"), w.s("Cardano"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXAVAX), w.u(catalog.PlatformOKX), w.s("AVAX"), w.s("AVAX"), w.s("Avalanche"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingOKXLINK), w.u(catalog.PlatformOKX), w.s("LINK"), w.s("LINK"), w.s("Chainlink"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitBNB), w.u(catalog.PlatformBybit), w.s("BNB"), w.s("BNB"), w.s("BNB"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitSOL), w.u(catalog.PlatformBybit), w.s("SOL"), w.s("SOL"), w.s("Solana"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitXRP), w.u(catalog.PlatformBybit), w.s("XRP"), w.s("XRP"), w.s("XRP"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitDOGE), w.u(catalog.PlatformBybit), w.s("DOGE"), w.s("DOGE"), w.s("Dogecoin"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitADA), w.u(catalog.PlatformBybit), w.s("ADA"), w.s("ADA"), w.s("Cardano"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitAVAX), w.u(catalog.PlatformBybit), w.s("AVAX"), w.s("AVAX"), w.s("Avalanche"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(ListingBybitLINK), w.u(catalog.PlatformBybit), w.s("LINK"), w.s("LINK"), w.s("Chainlink"), w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
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
		{w.u(InstrumentBinanceSOLUSDT), w.u(catalog.PlatformBinance), w.s("SOLUSDT"), w.s("SOLUSDT"), w.s("linear_perpetual"), w.u(ListingBinanceSOL), w.u(ListingBinanceUSDT), w.u(ListingBinanceSOL), w.u(ListingBinanceUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBinanceBNBUSDT), w.u(catalog.PlatformBinance), w.s("BNBUSDT"), w.s("BNBUSDT"), w.s("linear_perpetual"), w.u(ListingBinanceBNB), w.u(ListingBinanceUSDT), w.u(ListingBinanceBNB), w.u(ListingBinanceUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentOKXSOLUSDT), w.u(catalog.PlatformOKX), w.s("SOL-USDT-SWAP"), w.s("SOL-USDT-SWAP"), w.s("linear_perpetual"), w.u(ListingOKXSOL), w.u(ListingOKXUSDT), w.u(ListingOKXSOL), w.u(ListingOKXUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentOKXBNBUSDT), w.u(catalog.PlatformOKX), w.s("BNB-USDT-SWAP"), w.s("BNB-USDT-SWAP"), w.s("linear_perpetual"), w.u(ListingOKXBNB), w.u(ListingOKXUSDT), w.u(ListingOKXBNB), w.u(ListingOKXUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBybitETHUSDT), w.u(catalog.PlatformBybit), w.s("ETHUSDT"), w.s("ETHUSDT"), w.s("linear_perpetual"), w.u(ListingBybitETH), w.u(ListingBybitUSDT), w.u(ListingBybitETH), w.u(ListingBybitUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBybitSOLUSDT), w.u(catalog.PlatformBybit), w.s("SOLUSDT"), w.s("SOLUSDT"), w.s("linear_perpetual"), w.u(ListingBybitSOL), w.u(ListingBybitUSDT), w.u(ListingBybitSOL), w.u(ListingBybitUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
		{w.u(InstrumentBybitBNBUSDT), w.u(catalog.PlatformBybit), w.s("BNBUSDT"), w.s("BNBUSDT"), w.s("linear_perpetual"), w.u(ListingBybitBNB), w.u(ListingBybitUSDT), w.u(ListingBybitBNB), w.u(ListingBybitUSDT), w.n(), "1", w.s("active"), w.t(created), w.t(latest), w.t(created), w.t(latest)},
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
	specs := make([]mapSpec, 0, 58)
	specs = append(specs, []mapSpec{
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
	}...)
	type extraMap struct {
		listing, asset, portfolio uuid.UUID
		user                      uuid.UUID
	}
	n := 22
	for _, extra := range []extraMap{
		{ListingBinanceBNB, AssetBNB, PortfolioAliceMain, UserAlice},
		{ListingBinanceXRP, AssetXRP, PortfolioAliceMain, UserAlice},
		{ListingBinanceDOGE, AssetDOGE, PortfolioAliceMain, UserAlice},
		{ListingBinanceADA, AssetADA, PortfolioAliceMain, UserAlice},
		{ListingBinanceAVAX, AssetAVAX, PortfolioAliceMain, UserAlice},
		{ListingBinanceLINK, AssetLINK, PortfolioAliceMain, UserAlice},
		{ListingBybitBNB, AssetBNB, PortfolioAliceMain, UserAlice},
		{ListingBybitSOL, AssetSOL, PortfolioAliceMain, UserAlice},
		{ListingBybitXRP, AssetXRP, PortfolioAliceMain, UserAlice},
		{ListingBybitDOGE, AssetDOGE, PortfolioAliceMain, UserAlice},
		{ListingBybitADA, AssetADA, PortfolioAliceMain, UserAlice},
		{ListingBybitAVAX, AssetAVAX, PortfolioAliceMain, UserAlice},
		{ListingBybitLINK, AssetLINK, PortfolioAliceMain, UserAlice},
		{ListingOKXUSDT, AssetUSDT, PortfolioAliceMain, UserAlice},
		{ListingOKXBTC, AssetBTC, PortfolioAliceMain, UserAlice},
		{ListingOKXETH, AssetETH, PortfolioAliceMain, UserAlice},
		{ListingOKXUSD, catalog.AssetUSD, PortfolioAliceMain, UserAlice},
		{ListingOKXCNY, catalog.AssetCNY, PortfolioAliceMain, UserAlice},
		{ListingOKXBNB, AssetBNB, PortfolioAliceMain, UserAlice},
		{ListingOKXSOL, AssetSOL, PortfolioAliceMain, UserAlice},
		{ListingOKXXRP, AssetXRP, PortfolioAliceMain, UserAlice},
		{ListingOKXDOGE, AssetDOGE, PortfolioAliceMain, UserAlice},
		{ListingOKXADA, AssetADA, PortfolioAliceMain, UserAlice},
		{ListingOKXAVAX, AssetAVAX, PortfolioAliceMain, UserAlice},
		{ListingOKXLINK, AssetLINK, PortfolioAliceMain, UserAlice},
		{ListingOKXBNB, AssetBNB, PortfolioAliceTrading, UserAlice},
		{ListingOKXSOL, AssetSOL, PortfolioAliceTrading, UserAlice},
		{ListingOKXXRP, AssetXRP, PortfolioAliceTrading, UserAlice},
		{ListingOKXDOGE, AssetDOGE, PortfolioAliceTrading, UserAlice},
		{ListingOKXADA, AssetADA, PortfolioAliceTrading, UserAlice},
		{ListingOKXAVAX, AssetAVAX, PortfolioAliceTrading, UserAlice},
		{ListingOKXLINK, AssetLINK, PortfolioAliceTrading, UserAlice},
		{ListingBinanceBTC, AssetBTC, PortfolioBobMain, UserBob},
		{ListingBinanceETH, AssetETH, PortfolioBobMain, UserBob},
		{ListingBinanceSOL, AssetSOL, PortfolioBobMain, UserBob},
		{ListingBinanceBNB, AssetBNB, PortfolioBobMain, UserBob},
		{ListingBinanceUSDC, AssetUSDC, PortfolioBobMain, UserBob},
	} {
		specs = append(specs, mapSpec{
			SeedID(kindMapping, uint64(n)), extra.listing, extra.asset, extra.portfolio,
			"confirmed", "walkthrough seed", "", "", extra.user, 1,
		})
		n++
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
	for i, at := range eachDay(ts(8, 4, 16), ts(9, 20, 16), 1) {
		flavor := binanceFlavor(at)
		jobStatus, runStatus, snapStatus, complete := "succeeded", "succeeded", "complete", "complete"
		success, failed := 18, 0
		if flavor == "partial" {
			jobStatus, runStatus, snapStatus, complete = "succeeded", "partial", "partial", "partial"
			success, failed = 10, 3
		}
		trigger := "schedule"
		if i%7 == 2 {
			trigger = "manual"
		}
		run, snap := w.addCollection(uint64(100+i), PortfolioAliceMain, ConnectionAliceBinance, IdentityAliceBinance, UserAlice, at, trigger, jobStatus, runStatus, snapStatus, complete, success, failed, "", "")
		w.binanceFacts(i, at, run, snap, flavor)
	}

	for i, at := range eachDay(ts(8, 18, 16), ts(9, 20, 16), 1) {
		flavor := "complete"
		success, failed := 10, 0
		runStatus, snapStatus, complete := "succeeded", "complete", "complete"
		if at.Equal(ts(9, 8, 16)) {
			flavor = "partial"
			success, failed = 6, 1
			runStatus, snapStatus, complete = "partial", "partial", "partial"
		}
		run, snap := w.addCollection(uint64(200+i), PortfolioAliceMain, ConnectionAliceBybit, IdentityAliceBybit, UserAlice, at, "schedule", "succeeded", runStatus, snapStatus, complete, success, failed, "", "")
		w.bybitFacts(i, at, run, snap, flavor)
	}

	failAt := ts(9, 17, 12)
	failRun, failSnap := w.addCollection(24, PortfolioAliceMain, ConnectionAliceBybit, IdentityAliceBybit, UserAlice, failAt, "retry", "failed", "failed", "failed", "partial", 0, 1, "adapter_unavailable", "No Platform adapter is registered.")
	w.fails = append(w.fails, []string{
		w.u(SeedID(kindFailure, 10)), w.u(PortfolioAliceMain), w.u(failRun), w.u(failSnap), w.n(),
		w.s("identity"), w.s("uid-alice-bybit"), w.s("collect"), w.s("unsupported"), w.s("adapter_unavailable"),
		w.s("No Platform adapter is registered."), "0", w.t(failAt.Add(5 * time.Second)),
	})

	for i, at := range eachDay(ts(9, 1, 16), ts(9, 20, 16), 1) {
		_, snap := w.addCollection(uint64(400+i), PortfolioAliceMain, ConnectionAliceBinanceSub, IdentityAliceBinanceSub, UserAlice, at, "manual", "succeeded", "succeeded", "complete", "complete", 4, 0, "", "")
		w.subFacts(i, at, snap)
	}

	for i, at := range eachDay(ts(8, 10, 12), ts(9, 20, 12), 1) {
		_, snap := w.addCollection(uint64(300+i), PortfolioAliceMain, ConnectionAliceOKXMain, IdentityAliceOKXMain, UserAlice, at, "schedule", "succeeded", "succeeded", "complete", "complete", 12, 0, "", "")
		w.okxMainFacts(i, at, snap)
	}

	for i, at := range eachDay(ts(8, 15, 8), ts(9, 20, 8), 1) {
		_, snap := w.addCollection(uint64(500+i), PortfolioAliceTrading, ConnectionAliceOKX, IdentityAliceOKX, UserAlice, at, "schedule", "succeeded", "succeeded", "complete", "complete", 12, 0, "", "")
		w.okxFacts(i, at, snap)
	}

	for i, at := range eachDay(ts(8, 8, 0), ts(9, 20, 0), 2) {
		_, snap := w.addCollection(uint64(600+i), PortfolioBobMain, ConnectionBobBinance, IdentityBobBinance, UserBob, at, "schedule", "succeeded", "succeeded", "complete", "complete", 8, 0, "", "")
		w.bobFacts(i, at, snap)
	}

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
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceBNB, decimal(12+0.2*float64(i)), decimal(12+0.2*float64(i)), "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceXRP, decimal(800+10*float64(i)), decimal(800+10*float64(i)), "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceDOGE, decimal(4000+50*float64(i)), decimal(4000+50*float64(i)), "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceADA, decimal(1500+20*float64(i)), decimal(1500+20*float64(i)), "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceAVAX, decimal(40+0.5*float64(i)), decimal(40+0.5*float64(i)), "0")
	w.balance(portfolio, snap, ident, unified, at, collected, ListingBinanceLINK, decimal(80+float64(i)), decimal(80+float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceFunding, at, collected, ListingBinanceUSDT, "2500", "2500", "0")
	w.balance(portfolio, snap, ident, AccountAliceEarn, at, collected, ListingBinanceUSDT, "1800", "0", "1800")

	if flavor != "partial" {
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBTCUSDT, "pos-alice-btc", "long", longQty, "45000", btcPx, unrealBTC, "2")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceETHUSDT, "pos-alice-eth", "short", "1.5", "3000", ethPx, unrealETH, "3")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBTCUSDInv, "pos-alice-btc-inv", "long", "100", btcPx, btcPx, "80", "1")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceSOLUSDT, "pos-alice-sol", "long", "15", solPx, solPx, "40", "2")
		w.position(portfolio, snap, ident, unified, at, collected, InstrumentBinanceBNBUSDT, "pos-alice-bnb", "long", "4", strconv.Itoa(580+i), strconv.Itoa(580+i), "25", "2")
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
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceBNB, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(580+i))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceXRP, ListingBinanceUSDT, "exchange_rate", decimal(2.4+0.01*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceDOGE, ListingBinanceUSDT, "exchange_rate", decimal(0.18+0.001*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceADA, ListingBinanceUSDT, "exchange_rate", decimal(0.45+0.002*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceAVAX, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(28+i/4))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBinanceLINK, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(14+i/3))
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceSOLUSDT, nil, ListingBinanceUSDT, "mark", solPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBinanceBNBUSDT, nil, ListingBinanceUSDT, "mark", strconv.Itoa(580+i))
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
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitSOL, decimal(8+0.2*float64(i)), decimal(8+0.2*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitBNB, decimal(3+0.05*float64(i)), decimal(3+0.05*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitXRP, decimal(200+4*float64(i)), decimal(200+4*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitDOGE, decimal(900+10*float64(i)), decimal(900+10*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitADA, decimal(400+5*float64(i)), decimal(400+5*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitAVAX, decimal(12+0.1*float64(i)), decimal(12+0.1*float64(i)), "0")
	w.balance(portfolio, snap, ident, acct, at, collected, ListingBybitLINK, decimal(20+0.2*float64(i)), decimal(20+0.2*float64(i)), "0")
	if flavor != "partial" {
		w.position(portfolio, snap, ident, acct, at, collected, InstrumentBybitBTCUSDT, "pos-alice-bybit-btc", "long", "0.05", "47000", btcPx, "50", "2")
		w.position(portfolio, snap, ident, acct, at, collected, InstrumentBybitETHUSDT, "pos-alice-bybit-eth", "short", "0.8", "3100", strconv.Itoa(3210+i*8), "-20", "2")
		w.position(portfolio, snap, ident, acct, at, collected, InstrumentBybitSOLUSDT, "pos-alice-bybit-sol", "long", "6", strconv.Itoa(140+i), strconv.Itoa(140+i), "12", "2")
		w.position(portfolio, snap, ident, acct, at, collected, InstrumentBybitBNBUSDT, "pos-alice-bybit-bnb", "long", "1.2", strconv.Itoa(575+i), strconv.Itoa(575+i), "8", "2")
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBybitBTCUSDT, nil, ListingBybitUSDT, "mark", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBybitETHUSDT, nil, ListingBybitUSDT, "mark", strconv.Itoa(3210+i*8))
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBybitSOLUSDT, nil, ListingBybitUSDT, "mark", strconv.Itoa(140+i))
		w.price(portfolio, snap, ident, priceAt, collected, &InstrumentBybitBNBUSDT, nil, ListingBybitUSDT, "mark", strconv.Itoa(575+i))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitUSDT, ListingBybitUSD, "exchange_rate", "1")
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitBTC, ListingBybitUSDT, "exchange_rate", btcPx)
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitETH, ListingBybitUSDT, "exchange_rate", strconv.Itoa(3210+i*8))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitSOL, ListingBybitUSDT, "exchange_rate", strconv.Itoa(140+i))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitBNB, ListingBybitUSDT, "exchange_rate", strconv.Itoa(575+i))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitXRP, ListingBybitUSDT, "exchange_rate", decimal(2.35+0.01*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitDOGE, ListingBybitUSDT, "exchange_rate", decimal(0.17+0.001*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitADA, ListingBybitUSDT, "exchange_rate", decimal(0.44+0.002*float64(i)))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitAVAX, ListingBybitUSDT, "exchange_rate", strconv.Itoa(27+i/5))
		w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingBybitLINK, ListingBybitUSDT, "exchange_rate", strconv.Itoa(13+i/4))
	} else {
		w.fails = append(w.fails, []string{
			w.u(SeedID(kindFailure, 4)), w.u(portfolio), w.u(run), w.u(snap), w.n(),
			w.s("price"), w.s("ETHUSDT"), w.s("collect"), w.s("platform_error"), w.s("mark_price_missing"),
			w.s("ETH mark price was not returned."), "1", w.t(collected),
		})
	}
}

func (w *writer) subFacts(i int, at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	w.balance(PortfolioAliceMain, snap, IdentityAliceBinanceSub, AccountAliceSubUnified, at, collected, ListingBinanceUSDT, strconv.Itoa(400+i*80), strconv.Itoa(400+i*80), "0")
	w.balance(PortfolioAliceMain, snap, IdentityAliceBinanceSub, AccountAliceSubUnified, at, collected, ListingBinanceBTC, decimal(0.02+0.001*float64(i)), decimal(0.02+0.001*float64(i)), "0")
	w.price(PortfolioAliceMain, snap, IdentityAliceBinanceSub, at, collected, nil, &ListingBinanceUSDT, ListingBinanceUSD, "exchange_rate", "1")
	w.price(PortfolioAliceMain, snap, IdentityAliceBinanceSub, at, collected, nil, &ListingBinanceBTC, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(48000+i*50))
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
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXETH, decimal(4+0.05*float64(i)), decimal(4+0.05*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXSOL, decimal(18+0.2*float64(i)), decimal(18+0.2*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXBNB, decimal(6+0.1*float64(i)), decimal(6+0.1*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXXRP, decimal(500+8*float64(i)), decimal(500+8*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXDOGE, decimal(2000+30*float64(i)), decimal(2000+30*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXADA, decimal(900+12*float64(i)), decimal(900+12*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXAVAX, decimal(25+0.3*float64(i)), decimal(25+0.3*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, ListingOKXLINK, decimal(35+0.4*float64(i)), decimal(35+0.4*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXFunding, at, collected, ListingOKXUSDT, "800", "800", "0")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXBTCUSDT, "pos-alice-okx-btc", "long", "0.08", "44000", btcPx, strconv.Itoa(120+i*30), "2")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXETHUSDT, "pos-alice-okx-eth", "short", "2.0", "3100", ethPx, strconv.Itoa(-40-i*8), "2")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXSOLUSDT, "pos-alice-okx-sol", "long", "10", strconv.Itoa(145+i), strconv.Itoa(145+i), "18", "2")
	w.position(portfolio, snap, ident, AccountAliceOKXTrading, at, collected, InstrumentOKXBNBUSDT, "pos-alice-okx-bnb", "long", "2", strconv.Itoa(570+i), strconv.Itoa(570+i), "9", "2")
	idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingOKXUSDT, strconv.Itoa(22000+i*1500))
	w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXBTCUSDT, nil, ListingOKXUSDT, "mark", btcPx)
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXETHUSDT, nil, ListingOKXUSDT, "mark", ethPx)
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXSOLUSDT, nil, ListingOKXUSDT, "mark", strconv.Itoa(145+i))
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXBNBUSDT, nil, ListingOKXUSDT, "mark", strconv.Itoa(570+i))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXUSDT, ListingOKXCNY, "exchange_rate", "7.24")
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXUSDT, ListingOKXUSD, "exchange_rate", "1")
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXETH, ListingOKXUSDT, "exchange_rate", ethPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXBTC, ListingOKXUSDT, "exchange_rate", btcPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXSOL, ListingOKXUSDT, "exchange_rate", strconv.Itoa(145+i))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXBNB, ListingOKXUSDT, "exchange_rate", strconv.Itoa(570+i))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXXRP, ListingOKXUSDT, "exchange_rate", decimal(2.38+0.01*float64(i)))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXDOGE, ListingOKXUSDT, "exchange_rate", decimal(0.175+0.001*float64(i)))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXADA, ListingOKXUSDT, "exchange_rate", decimal(0.46+0.002*float64(i)))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXAVAX, ListingOKXUSDT, "exchange_rate", strconv.Itoa(29+i/5))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXLINK, ListingOKXUSDT, "exchange_rate", strconv.Itoa(15+i/4))
	metric := w.header(portfolio, snap, ident, &AccountAliceOKXTrading, "account_metric", at, collected, "complete", 1, collected)
	w.metrics = append(w.metrics, []string{w.u(portfolio), w.u(metric), w.s("mgnRatio"), w.s("ratio"), decimal(8.5 - 0.2*float64(i)), w.n(), w.s("okx_mcm_v1")})
}

func (w *writer) okxMainFacts(i int, at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	priceAt := at.Add(-90 * time.Second)
	portfolio := PortfolioAliceMain
	ident := IdentityAliceOKXMain
	btcPx := strconv.Itoa(47850 + i*40)
	ethPx := strconv.Itoa(3170 + i*6)
	w.balance(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, ListingOKXUSDT, strconv.Itoa(3500+i*80), strconv.Itoa(3500+i*80), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, ListingOKXBTC, decimal(0.12+0.002*float64(i)), decimal(0.12+0.002*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, ListingOKXETH, decimal(2.1+0.03*float64(i)), decimal(2.1+0.03*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, ListingOKXSOL, decimal(9+0.1*float64(i)), decimal(9+0.1*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, ListingOKXBNB, decimal(2.5+0.04*float64(i)), decimal(2.5+0.04*float64(i)), "0")
	w.balance(portfolio, snap, ident, AccountAliceOKXMainFunding, at, collected, ListingOKXUSDT, "600", "600", "0")
	w.position(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, InstrumentOKXBTCUSDT, "pos-alice-okx-main-btc", "long", "0.04", "44200", btcPx, strconv.Itoa(60+i*4), "2")
	w.position(portfolio, snap, ident, AccountAliceOKXMainTrading, at, collected, InstrumentOKXETHUSDT, "pos-alice-okx-main-eth", "short", "0.6", "3120", ethPx, strconv.Itoa(-15-i), "2")
	idRE := w.equity(portfolio, snap, ident, nil, at, collected, ListingOKXUSD, strconv.Itoa(14000+i*120))
	w.coveredI = append(w.coveredI, []string{w.u(portfolio), w.u(idRE), w.u(ident)})
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXBTCUSDT, nil, ListingOKXUSDT, "mark", btcPx)
	w.price(portfolio, snap, ident, priceAt, collected, &InstrumentOKXETHUSDT, nil, ListingOKXUSDT, "mark", ethPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXUSDT, ListingOKXUSD, "exchange_rate", "1")
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXBTC, ListingOKXUSDT, "exchange_rate", btcPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXETH, ListingOKXUSDT, "exchange_rate", ethPx)
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXSOL, ListingOKXUSDT, "exchange_rate", strconv.Itoa(144+i))
	w.price(portfolio, snap, ident, priceAt, collected, nil, &ListingOKXBNB, ListingOKXUSDT, "exchange_rate", strconv.Itoa(572+i))
}

func (w *writer) bobFacts(i int, at time.Time, snap uuid.UUID) {
	collected := at.Add(5 * time.Second)
	priceAt := at.Add(-2 * time.Minute)
	btcPx := strconv.Itoa(47000 + i*120)
	ethPx := strconv.Itoa(3100 + i*15)
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceUSDT, strconv.Itoa(1800+i*40), strconv.Itoa(1800+i*40), "0")
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceBTC, decimal(0.08+0.002*float64(i)), decimal(0.08+0.002*float64(i)), "0")
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceETH, decimal(1.4+0.02*float64(i)), decimal(1.4+0.02*float64(i)), "0")
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceSOL, decimal(11+0.15*float64(i)), decimal(11+0.15*float64(i)), "0")
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceBNB, decimal(4+0.05*float64(i)), decimal(4+0.05*float64(i)), "0")
	w.balance(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, ListingBinanceUSDC, strconv.Itoa(700+i*10), strconv.Itoa(700+i*10), "0")
	w.position(PortfolioBobMain, snap, IdentityBobBinance, AccountBobUnified, at, collected, InstrumentBinanceBTCUSDT, "pos-bob-btc", "long", "0.03", "46000", btcPx, strconv.Itoa(40+i*3), "2")
	idRE := w.equity(PortfolioBobMain, snap, IdentityBobBinance, &AccountBobUnified, at, collected, ListingBinanceUSD, strconv.Itoa(9000+i*180))
	w.coveredI = append(w.coveredI, []string{w.u(PortfolioBobMain), w.u(idRE), w.u(IdentityBobBinance)})
	w.coveredA = append(w.coveredA, []string{w.u(PortfolioBobMain), w.u(idRE), w.u(AccountBobUnified)})
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, &InstrumentBinanceBTCUSDT, nil, ListingBinanceUSDT, "mark", btcPx)
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceUSDT, ListingBinanceUSD, "exchange_rate", "1")
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceBTC, ListingBinanceUSDT, "exchange_rate", btcPx)
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceETH, ListingBinanceUSDT, "exchange_rate", ethPx)
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceSOL, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(138+i))
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceBNB, ListingBinanceUSDT, "exchange_rate", strconv.Itoa(560+i))
	w.price(PortfolioBobMain, snap, IdentityBobBinance, priceAt, collected, nil, &ListingBinanceUSDC, ListingBinanceUSDT, "exchange_rate", "1")
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
	flow6 := SeedID(kindCapitalFlow, 6)
	r1 := SeedID(kindFlowRevision, 1)
	r2 := SeedID(kindFlowRevision, 2)
	r3 := SeedID(kindFlowRevision, 3)
	r4 := SeedID(kindFlowRevision, 4)
	r5 := SeedID(kindFlowRevision, 5)
	r6 := SeedID(kindFlowRevision, 6)
	r7 := SeedID(kindFlowRevision, 7)
	r8 := SeedID(kindFlowRevision, 8)
	w.insert("capital_flows", "id, portfolio_id, current_revision_id, version, created_at, updated_at", [][]string{
		{w.u(flow1), w.u(PortfolioAliceMain), w.n(), "2", w.t(ts(9, 10, 0)), w.t(ts(9, 11, 0))},
		{w.u(flow2), w.u(PortfolioAliceMain), w.n(), "1", w.t(ts(9, 16, 0)), w.t(ts(9, 16, 0))},
		{w.u(flow3), w.u(PortfolioAliceMain), w.n(), "2", w.t(ts(8, 20, 0)), w.t(ts(8, 21, 0))},
		{w.u(flow4), w.u(PortfolioAliceMain), w.n(), "1", w.t(ts(9, 4, 0)), w.t(ts(9, 4, 0))},
		{w.u(flow5), w.u(PortfolioAliceTrading), w.n(), "1", w.t(ts(9, 5, 0)), w.t(ts(9, 5, 0))},
		{w.u(flow6), w.u(PortfolioBobMain), w.n(), "1", w.t(ts(9, 1, 0)), w.t(ts(9, 1, 0))},
	})
	w.insert("capital_flow_revisions", "id, portfolio_id, capital_flow_id, direction, quantity, asset_id, effective_at, account_id, note, state, created_by_user_id, created_at", [][]string{
		{w.u(r1), w.u(PortfolioAliceMain), w.u(flow1), w.s("contribution"), "5000", w.u(catalog.AssetUSD), w.t(ts(9, 10, 0)), w.u(AccountAliceFunding), w.s("external contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 10, 0))},
		{w.u(r3), w.u(PortfolioAliceMain), w.u(flow1), w.s("contribution"), "4000", w.u(catalog.AssetUSD), w.t(ts(9, 10, 0)), w.u(AccountAliceFunding), w.s("edited contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 11, 0))},
		{w.u(r2), w.u(PortfolioAliceMain), w.u(flow2), w.s("withdrawal"), "1000", w.u(catalog.AssetUSD), w.t(ts(9, 16, 0)), w.u(AccountAliceFunding), w.s("external withdrawal"), w.s("active"), w.u(UserAlice), w.t(ts(9, 16, 0))},
		{w.u(r4), w.u(PortfolioAliceMain), w.u(flow3), w.s("contribution"), "2500", w.u(catalog.AssetUSD), w.t(ts(8, 20, 0)), w.u(AccountAliceFunding), w.s("mistaken contribution"), w.s("active"), w.u(UserAlice), w.t(ts(8, 20, 0))},
		{w.u(r5), w.u(PortfolioAliceMain), w.u(flow3), w.s("contribution"), "2500", w.u(catalog.AssetUSD), w.t(ts(8, 20, 0)), w.u(AccountAliceFunding), w.s("deleted mistaken contribution"), w.s("deleted"), w.u(UserAlice), w.t(ts(8, 21, 0))},
		{w.u(r6), w.u(PortfolioAliceMain), w.u(flow4), w.s("contribution"), "2000", w.u(catalog.AssetUSD), w.t(ts(9, 4, 0)), w.u(AccountAliceFunding), w.s("September top-up"), w.s("active"), w.u(UserAlice), w.t(ts(9, 4, 0))},
		{w.u(r7), w.u(PortfolioAliceTrading), w.u(flow5), w.s("contribution"), "10000", w.u(catalog.AssetCNY), w.t(ts(9, 5, 0)), w.u(AccountAliceOKXFunding), w.s("CNY contribution"), w.s("active"), w.u(UserAlice), w.t(ts(9, 5, 0))},
		{w.u(r8), w.u(PortfolioBobMain), w.u(flow6), w.s("contribution"), "2500", w.u(catalog.AssetUSD), w.t(ts(9, 1, 0)), w.u(AccountBobUnified), w.s("September contribution"), w.s("active"), w.u(UserBob), w.t(ts(9, 1, 0))},
	})
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r3) + " WHERE id = " + w.u(flow1))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r2) + " WHERE id = " + w.u(flow2))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r5) + " WHERE id = " + w.u(flow3))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r6) + " WHERE id = " + w.u(flow4))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r7) + " WHERE id = " + w.u(flow5))
	w.exec("UPDATE capital_flows SET current_revision_id = " + w.u(r8) + " WHERE id = " + w.u(flow6))
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
		{w.u(SeedID(kindAudit, 11)), w.u(PortfolioAliceMain), w.u(UserAlice), w.s("connection.create"), w.s("connection"), w.u(ConnectionAliceOKXMain), w.s("seed-walkthrough-11"), w.n(), w.s(`{"label":"OKX Main read","status":"disabled","credentialsConfigured":false}`), w.n(), w.t(ts(8, 1, 0))},
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

func eachDay(from, to time.Time, step int) []time.Time {
	if step < 1 {
		step = 1
	}
	var out []time.Time
	cur := from
	for !cur.After(to) {
		out = append(out, cur)
		cur = cur.AddDate(0, 0, step)
	}
	return out
}

func binanceFlavor(at time.Time) string {
	switch {
	case at.Equal(ts(8, 31, 16)):
		return "containment"
	case at.Equal(ts(9, 10, 16)):
		return "partial"
	case at.Equal(ts(9, 15, 16)):
		return "overlap"
	default:
		return "complete"
	}
}

func decimal(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}
