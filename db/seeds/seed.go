package seeds

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
)

// Seed contains the development SQL and the Identities to activate afterwards.
type Seed struct {
	SQL string
	// Live Identities receive sealed credentials and are enabled after the SQL runs.
	Live []LiveIdentity
	// Portfolios lists every seeded Portfolio whose Valuation Points need rebuilding.
	Portfolios []uuid.UUID
}

// Build generates the development seed anchored at the current time.
func Build() Seed {
	return BuildAt(time.Now())
}

// BuildAt generates the development seed with its latest cycle at the rounded anchor.
func BuildAt(now time.Time) Seed {
	b := newBuilder()
	b.comment("Deterministic development seed")
	b.comment("Scratch logins: alice / Alice=123456 ; bob / Bob=123456")
	b.comment("Account Reports are inserted without valuation columns; `plutus db seed` rebuilds every Valuation Point afterwards.")
	portfolios := walkthrough(b, now.UTC().Truncate(15*time.Minute))
	return Seed{SQL: b.SQL(), Live: b.live, Portfolios: portfolios}
}

const (
	keyAliceBinance        = "seed-alice-binance"
	keyAliceBinanceClassic = "seed-alice-binance-classic"
	keyAliceOKX            = "seed-alice-okx"
	keyAliceBybit          = "seed-alice-bybit"
	keyBobBinance          = "seed-bob-binance"
)

// The Margin Modes the fake adapter reports for the seeded OKX and Bybit keys, so that seeded
// history, stored Identities, and later live cycles agree.
var (
	modeAliceOKX   = fake.MarginMode(adapters.StructureOKXUnified, keyAliceOKX, time.Time{})
	modeAliceBybit = fake.MarginMode(adapters.StructureBybitUTA2, keyAliceBybit, time.Time{})
)

var shanghai = mustLocation("Asia/Shanghai")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// localMidnight returns the local midnight of the day daysAgo days before t.
func localMidnight(t time.Time, daysAgo int, loc *time.Location) time.Time {
	local := t.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return day.AddDate(0, 0, -daysAgo)
}

func (b *builder) usersWithInterval(now time.Time, interval int) {
	created := now.AddDate(0, 0, -60)
	b.user(userSpec{ID: UserAlice, Username: "alice", Hash: alicePasswordHash, CreatedAt: created})
	b.user(userSpec{ID: UserBob, Username: "bob", Hash: bobPasswordHash, CreatedAt: created})
	b.portfolio(&portfolioSpec{ID: PortfolioAliceMain, User: UserAlice, Name: "default", Timezone: "Asia/Shanghai", Interval: interval, Undeletable: true, CreatedAt: created})
	b.portfolio(&portfolioSpec{ID: PortfolioAliceTrading, User: UserAlice, Name: "trading", Timezone: "Asia/Shanghai", Interval: interval, CreatedAt: created.Add(time.Hour)})
	b.portfolio(&portfolioSpec{ID: PortfolioBobMain, User: UserBob, Name: "default", Timezone: "Europe/Berlin", Interval: interval, Undeletable: true, CreatedAt: created})
}

// fakeReports builds ok reports from the fake adapter scenario for the given Account Types under
// the Identity's Margin Mode (empty for structures without one).
func fakeReports(apiKey string, platform adapters.Platform, mode adapters.MarginMode, at time.Time, accounts map[adapters.AccountType]uuid.UUID, types ...adapters.AccountType) []reportSpec {
	scenario := fake.Scenario(apiKey, at)
	out := make([]reportSpec, 0, len(types))
	for _, t := range types {
		report := scenario.Report(mode, t, at.Add(2*time.Second), false)
		out = append(out, reportSpec{Account: accounts[t], AccountType: t, Platform: platform, MarginMode: mode, Report: &report})
	}
	return out
}

func failedReports(platform adapters.Platform, class, detail string, accounts map[adapters.AccountType]uuid.UUID, types ...adapters.AccountType) []reportSpec {
	out := make([]reportSpec, 0, len(types))
	for _, t := range types {
		out = append(out, reportSpec{Account: accounts[t], AccountType: t, Platform: platform, Class: class, Detail: detail})
	}
	return out
}

func (b *builder) fakePrices(apiKey string, platform adapters.Platform, at time.Time) {
	scenario := fake.Scenario(apiKey, at)
	for _, symbol := range []string{"BTC", "ETH", "SOL", "BNB"} {
		b.price(platform, symbol, scenario.Price(symbol), at)
	}
}

func fakePrice(apiKey, symbol string, at time.Time) string {
	return fake.Scenario(apiKey, at).Price(symbol)
}

func selectedSet(types ...adapters.AccountType) map[adapters.AccountType]bool {
	out := map[adapters.AccountType]bool{}
	for _, t := range types {
		out[t] = true
	}
	return out
}

// Account Type identifiers used by the seed.
const (
	typePortfolioMargin adapters.AccountType = "portfolio_margin"
	typeSpot            adapters.AccountType = "spot"
	typeFunding         adapters.AccountType = "funding"
	typeEarn            adapters.AccountType = "earn"
	typeDual                                 = adapters.TypeDualInvestment
	typeTrading         adapters.AccountType = "trading"
	typeUnified         adapters.AccountType = "unified"
	typeCrossMargin     adapters.AccountType = "cross_margin"
	typeIsolatedMargin  adapters.AccountType = "isolated_margin"
	typeUMFutures       adapters.AccountType = "um_futures"
	typeCMFutures       adapters.AccountType = "cm_futures"
)

var (
	aliceBinanceAccounts = map[adapters.AccountType]uuid.UUID{typePortfolioMargin: AccountAliceBinancePM, typeSpot: AccountAliceBinanceSpot, typeFunding: AccountAliceBinanceFunding, typeDual: AccountAliceBinanceDual, typeEarn: AccountAliceBinanceEarn}
	aliceOKXAccounts     = map[adapters.AccountType]uuid.UUID{typeTrading: AccountAliceOKXTrading, typeFunding: AccountAliceOKXFunding, typeDual: AccountAliceOKXDual, typeEarn: AccountAliceOKXEarn}
	aliceBybitAccounts   = map[adapters.AccountType]uuid.UUID{typeUnified: AccountAliceBybitUnified, typeFunding: AccountAliceBybitFunding, typeDual: AccountAliceBybitDual, typeEarn: AccountAliceBybitEarn}
	aliceOldAccounts     = map[adapters.AccountType]uuid.UUID{typePortfolioMargin: AccountAliceOldPM, typeSpot: AccountAliceOldSpot, typeFunding: AccountAliceOldFunding, typeDual: AccountAliceOldDual, typeEarn: AccountAliceOldEarn}
	bobBinanceAccounts   = map[adapters.AccountType]uuid.UUID{typePortfolioMargin: AccountBobBinancePM, typeSpot: AccountBobBinanceSpot, typeFunding: AccountBobBinanceFunding, typeDual: AccountBobBinanceDual, typeEarn: AccountBobBinanceEarn}
	aliceClassicAccounts = map[adapters.AccountType]uuid.UUID{
		typeSpot: AccountAliceClassicSpot, typeFunding: AccountAliceClassicFunding, typeCrossMargin: AccountAliceClassicCrossMargin,
		typeIsolatedMargin: AccountAliceClassicIsolatedMargin, typeUMFutures: AccountAliceClassicUMFutures,
		typeCMFutures: AccountAliceClassicCMFutures, typeDual: AccountAliceClassicDual, typeEarn: AccountAliceClassicEarn,
	}
)

// walkthrough: forty days of two-hour cycles across three Platforms, one stale day, flows, and an archived Identity.
func walkthrough(b *builder, now time.Time) []uuid.UUID {
	b.usersWithInterval(now, 7200)
	created := now.AddDate(0, 0, -45)
	end := now
	b.identity(&identitySpec{ID: IdentityAliceBinance, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinancePM, Name: "Binance main", APIKey: keyAliceBinance, CreatedAt: created, CollectedAt: end})
	b.accounts(IdentityAliceBinance, aliceBinanceAccounts, selectedSet(typePortfolioMargin, typeSpot, typeFunding), created)
	b.identity(&identitySpec{ID: IdentityAliceOKX, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformOKX, Structure: adapters.StructureOKXUnified, MarginMode: modeAliceOKX, Name: "OKX main", APIKey: keyAliceOKX, CreatedAt: created, CollectedAt: end})
	b.accounts(IdentityAliceOKX, aliceOKXAccounts, selectedSet(typeTrading, typeFunding, typeEarn, typeDual), created)
	b.identity(&identitySpec{ID: IdentityAliceBybit, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBybit, Structure: adapters.StructureBybitUTA2, MarginMode: modeAliceBybit, Name: "Bybit main", APIKey: keyAliceBybit, CreatedAt: created, CollectedAt: end})
	b.accounts(IdentityAliceBybit, aliceBybitAccounts, selectedSet(typeUnified), created)
	b.identity(&identitySpec{ID: IdentityAliceBinanceOld, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinancePM, Name: "Binance old sub-account", Archived: true, ArchivedAt: now.AddDate(0, 0, -30), CreatedAt: created})
	b.accounts(IdentityAliceBinanceOld, aliceOldAccounts, selectedSet(), created)
	b.identity(&identitySpec{ID: IdentityBobBinance, Portfolio: PortfolioBobMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinancePM, Name: "Binance", APIKey: keyBobBinance, CreatedAt: created, CollectedAt: end})
	b.accounts(IdentityBobBinance, bobBinanceAccounts, selectedSet(typePortfolioMargin, typeSpot), created)
	classicCreated := now.AddDate(0, 0, -20)
	b.identity(&identitySpec{ID: IdentityAliceBinanceClassic, Portfolio: PortfolioAliceMain, Platform: adapters.PlatformBinance, Structure: adapters.StructureBinanceClassic, Name: "Binance classic", APIKey: keyAliceBinanceClassic, CreatedAt: classicCreated, CollectedAt: end})
	b.accounts(IdentityAliceBinanceClassic, aliceClassicAccounts, selectedSet(typeCrossMargin, typeIsolatedMargin, typeUMFutures, typeCMFutures), classicCreated)

	start := localMidnight(now, 40, shanghai)
	staleDay := localMidnight(now, 10, shanghai)
	for at := start; at.Before(end); at = at.Add(2 * time.Hour) {
		b.fakePrices(keyAliceBinance, adapters.PlatformBinance, at)
		b.fakePrices(keyAliceOKX, adapters.PlatformOKX, at)
		b.fakePrices(keyAliceBybit, adapters.PlatformBybit, at)
		reports := fakeReports(keyAliceBinance, adapters.PlatformBinance, "", at, aliceBinanceAccounts, typePortfolioMargin, typeSpot, typeFunding)
		reports = append(reports, fakeReports(keyAliceOKX, adapters.PlatformOKX, modeAliceOKX, at, aliceOKXAccounts, typeTrading, typeFunding, typeEarn, typeDual)...)
		if !at.Before(staleDay) && at.Before(staleDay.AddDate(0, 0, 1)) {
			reports = append(reports, failedReports(adapters.PlatformBybit, adapters.ClassRateLimited, "The platform rate limit was exceeded.", aliceBybitAccounts, typeUnified)...)
		} else {
			reports = append(reports, fakeReports(keyAliceBybit, adapters.PlatformBybit, modeAliceBybit, at, aliceBybitAccounts, typeUnified)...)
		}
		if !at.Before(classicCreated) {
			b.fakePrices(keyAliceBinanceClassic, adapters.PlatformBinance, at)
			reports = append(reports, fakeReports(keyAliceBinanceClassic, adapters.PlatformBinance, "", at, aliceClassicAccounts, typeCrossMargin, typeIsolatedMargin, typeUMFutures, typeCMFutures)...)
		}
		b.cycle(PortfolioAliceMain, at, reports)
	}

	// The latest cycle keeps exactly one stale Futures row; later fake collection recovers it.
	{
		at := end
		b.fakePrices(keyAliceBinance, adapters.PlatformBinance, at)
		b.fakePrices(keyAliceOKX, adapters.PlatformOKX, at)
		b.fakePrices(keyAliceBybit, adapters.PlatformBybit, at)
		reports := fakeReports(keyAliceBinance, adapters.PlatformBinance, "", at, aliceBinanceAccounts, typePortfolioMargin, typeSpot, typeFunding)
		reports = append(reports, fakeReports(keyAliceOKX, adapters.PlatformOKX, modeAliceOKX, at, aliceOKXAccounts, typeTrading, typeFunding, typeEarn, typeDual)...)
		reports = append(reports, fakeReports(keyAliceBybit, adapters.PlatformBybit, modeAliceBybit, at, aliceBybitAccounts, typeUnified)...)
		b.fakePrices(keyAliceBinanceClassic, adapters.PlatformBinance, at)
		reports = append(reports, fakeReports(keyAliceBinanceClassic, adapters.PlatformBinance, "", at, aliceClassicAccounts, typeCrossMargin, typeIsolatedMargin, typeUMFutures)...)
		reports = append(reports, failedReports(adapters.PlatformBinance, adapters.ClassRateLimited, "The platform rate limit was exceeded.", aliceClassicAccounts, typeCMFutures)...)
		b.cycle(PortfolioAliceMain, at, reports)
	}
	bobStart := localMidnight(now, 3, mustLocation("Europe/Berlin"))
	for at := bobStart; !at.After(end); at = at.Add(2 * time.Hour) {
		b.fakePrices(keyBobBinance, adapters.PlatformBinance, at)
		b.cycle(PortfolioBobMain, at, fakeReports(keyBobBinance, adapters.PlatformBinance, "", at, bobBinanceAccounts, typePortfolioMargin, typeSpot))
	}

	b.flow(&flowSpec{ID: FlowAliceUSDT, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "USDT", Quantity: "5000", ValueUSD: "5000", At: start.AddDate(0, 0, 2).Add(9 * time.Hour), Note: "Initial funding"})
	btcAt := start.AddDate(0, 0, 15).Add(14 * time.Hour)
	b.flow(&flowSpec{ID: FlowAliceBTC, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "BTC", Quantity: "0.2", ValueUSD: scale(fakePrice(keyAliceBinance, "BTC", btcAt), 0.2), At: btcAt, Note: "Deposit from cold wallet"})
	b.flow(&flowSpec{ID: FlowAliceUSDC, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "withdrawal", Asset: "USDC", Quantity: "1500", ValueUSD: "1500", At: start.AddDate(0, 0, 28).Add(11 * time.Hour), Note: "Withdrawn to bank"})
	ethAt := start.AddDate(0, 0, 35).Add(16 * time.Hour)
	b.flow(&flowSpec{ID: FlowAliceETH, Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "ETH", Quantity: "2", ValueUSD: scale(fakePrice(keyAliceBinance, "ETH", ethAt), 2), At: ethAt, Note: "Deposit"})
	b.flow(&flowSpec{ID: FlowBobUSDT, Portfolio: PortfolioBobMain, User: UserBob, Direction: "contribution", Asset: "USDT", Quantity: "2500", ValueUSD: "2500", At: bobStart.Add(6 * time.Hour), Note: "Initial funding"})
	for _, id := range []uuid.UUID{IdentityAliceBinance, IdentityAliceOKX, IdentityAliceBybit, IdentityAliceBinanceOld, IdentityAliceBinanceClassic} {
		at := created
		if id == IdentityAliceBinanceClassic {
			at = classicCreated
		}
		spec := b.identities[id]
		b.audit(&auditSpec{Portfolio: PortfolioAliceMain, Actor: UserAlice, Action: "identity.create", ResourceType: "identity", ResourceID: id, After: map[string]any{"displayName": spec.Name, "platform": string(spec.Platform), "accountStructure": string(spec.Structure)}, At: at})
	}
	b.audit(&auditSpec{Portfolio: PortfolioAliceMain, Actor: UserAlice, Action: "identity.archive", ResourceType: "identity", ResourceID: IdentityAliceBinanceOld, After: map[string]any{"status": "archived"}, At: now.AddDate(0, 0, -30)})
	b.audit(&auditSpec{Portfolio: PortfolioBobMain, Actor: UserBob, Action: "identity.create", ResourceType: "identity", ResourceID: IdentityBobBinance, After: map[string]any{"displayName": "Binance", "platform": "binance"}, At: created})
	return []uuid.UUID{PortfolioAliceMain, PortfolioAliceTrading, PortfolioBobMain}
}

// scale multiplies a decimal price string by a quantity and formats it with two decimals.
func scale(price string, quantity float64) string {
	var value float64
	if _, err := fmt.Sscanf(price, "%g", &value); err != nil {
		return ""
	}
	return fmt.Sprintf("%.2f", value*quantity)
}
