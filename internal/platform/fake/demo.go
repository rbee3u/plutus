package fake

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

const demoPrefix = "plutus-demo-v1:"

// DemoIdentity describes one offline Platform login. Definitions contain no
// database IDs, credentials, or derived valuation fields.
type DemoIdentity struct {
	ID         string
	Portfolio  string
	Name       string
	Platform   adapters.Platform
	Structure  adapters.AccountStructure
	Mode       adapters.MarginMode
	Profile    string
	Connection string
}

// DemoIdentities returns an independent copy in stable ownership order.
func DemoIdentities() []DemoIdentity {
	return []DemoIdentity{
		{"daily-classic", "default", "bn-sub-04", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "daily", "enabled"},
		{"daily-pm", "default", "bn-main", adapters.PlatformBinance, adapters.StructureBinancePM, "", "daily", "enabled"},
		{"daily-okx", "default", "ok-main", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXPM, "daily", "enabled"},
		{"daily-bybit", "default", "by-main", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitPM, "daily", "enabled"},
		{"daily-kraken", "default", "kkSum", adapters.PlatformKraken, adapters.StructureKrakenSeparate, "", "daily", "enabled"},
		{"daily-kraken-unified", "default", "kr-main", adapters.PlatformKraken, adapters.StructureKrakenUnified, "", "daily", "enabled"},
		{"daily-sub-01", "default", "bn-sub-01", adapters.PlatformBinance, adapters.StructureBinancePM, "", "daily", "enabled"},
		{"daily-sub-02", "default", "bn-sub-02", adapters.PlatformBinance, adapters.StructureBinancePM, "", "daily", "enabled"},
		{"daily-sub-03", "default", "bn-sub-03", adapters.PlatformBinance, adapters.StructureBinancePM, "", "daily", "enabled"},
		{"daily-okx-sum", "default", "okxSum", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "daily", "enabled"},
		{"daily-okx-coin", "default", "okxCoinM", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "daily", "enabled"},
		{"daily-okx-usd", "default", "okxUsdM", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "daily", "enabled"},
		{"retired-classic", "default", "bn-retired", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "archived", "unconfigured"},
		{"edge-unpriced", "edge-cases", "Binance unpriced airdrop", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "unpriced", "enabled"},
		{"edge-stale", "edge-cases", "Binance delayed reports", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "stale", "enabled"},
		{"edge-missing", "edge-cases", "Bybit never collected", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitCross, "missing", "enabled"},
	}
}

// ValidateDemoIdentity catches definition mistakes before producing SQL or facts.
func ValidateDemoIdentity(d DemoIdentity) error {
	if d.ID == "" || strings.Contains(d.ID, ":") || d.Name == "" ||
		!adapters.ValidStructure(d.Platform, d.Structure) || !adapters.ValidMarginMode(d.Structure, d.Mode) {
		return fmt.Errorf("invalid demo identity %q: identity or capability", d.ID)
	}
	if d.Portfolio != "default" && d.Portfolio != "edge-cases" {
		return fmt.Errorf("invalid demo identity %q: portfolio", d.ID)
	}
	switch d.Connection {
	case "enabled", "disabled", "invalid", "unconfigured":
	default:
		return fmt.Errorf("invalid demo identity %q: connection", d.ID)
	}
	switch d.Profile {
	case "daily", "large", "unpriced", "stale", "stale_incomplete", "missing", "invalid", "disabled", "unconfigured", "archived", "boundary", "policy", "metadata", "mode", "empty", "unsupported", "normalization":
	default:
		return fmt.Errorf("invalid demo identity %q: profile", d.ID)
	}
	return nil
}

// DemoKey carries the original anchor across processes without a database hook.
func DemoKey(id string, anchor time.Time) string {
	return demoPrefix + id + ":" + strconv.FormatInt(anchor.UTC().Unix(), 10)
}

func parseDemoKey(key string) (DemoIdentity, time.Time, bool, error) {
	if !strings.HasPrefix(key, demoPrefix) {
		return DemoIdentity{}, time.Time{}, false, nil
	}
	parts := strings.Split(strings.TrimPrefix(key, demoPrefix), ":")
	if len(parts) != 2 {
		return DemoIdentity{}, time.Time{}, true, adapters.NewError(adapters.ClassAuthentication, "The offline scenario key is invalid.")
	}
	seconds, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || seconds < 0 || seconds > 253402300799 {
		return DemoIdentity{}, time.Time{}, true, adapters.NewError(adapters.ClassAuthentication, "The offline scenario anchor is invalid.")
	}
	for _, d := range DemoIdentities() {
		if d.ID == parts[0] {
			return d, time.Unix(seconds, 0).UTC(), true, nil
		}
	}
	return DemoIdentity{}, time.Time{}, true, adapters.NewError(adapters.ClassUnsupported, "The offline scenario is not supported.")
}

// DemoEventTime is a stable UTC event instant relative to the seed anchor.
func DemoEventTime(anchor time.Time, days int) time.Time {
	return anchor.UTC().AddDate(0, 0, days)
}

// DemoAccounts returns the historical Portfolio Boundary for one login. Only
// seeds use this timeline; runtime always honors the owner's requested Accounts.
func DemoAccounts(d DemoIdentity, anchor, at time.Time) []adapters.AccountType {
	switch d.Profile {
	case "daily", "large":
		if d.Profile == "daily" && d.Structure == adapters.StructureBinanceClassic {
			return []adapters.AccountType{"spot", "funding", "um_futures", "cm_futures", "earn", adapters.TypeDualInvestment}
		}
		return adapters.AccountTypeIDs(d.Structure)
	case "archived":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return nil
		}
		return []adapters.AccountType{"spot", "funding"}
	case "boundary":
		if at.Before(DemoEventTime(anchor, -5)) {
			return []adapters.AccountType{"spot"}
		}
		if at.Before(DemoEventTime(anchor, -3)) {
			return []adapters.AccountType{"spot", "funding"}
		}
		return []adapters.AccountType{"funding"}
	case "stale":
		return []adapters.AccountType{"cm_futures", "spot"}
	case "unpriced":
		if !at.Before(DemoEventTime(anchor, -5)) && at.Before(DemoEventTime(anchor, -3)) {
			return []adapters.AccountType{"spot", "funding"}
		}
		return []adapters.AccountType{"spot"}
	case "missing", "mode":
		return []adapters.AccountType{"unified"}
	case "invalid", "unsupported", "metadata":
		return []adapters.AccountType{"trading", "earn", adapters.TypeDualInvestment}
	case "disabled":
		return []adapters.AccountType{"main"}
	case "unconfigured":
		return []adapters.AccountType{"spot"}
	case "empty":
		return []adapters.AccountType{"spot", "earn", adapters.TypeDualInvestment}
	default:
		return []adapters.AccountType{"spot"}
	}
}

func demoMode(d DemoIdentity, anchor, at time.Time) adapters.MarginMode {
	if (d.Profile == "mode" || d.ID == "daily-bybit") && at.Before(DemoEventTime(anchor, -3)) {
		return adapters.MarginBybitCross
	}
	return d.Mode
}

// DemoBatch is shared by history seeding and the fake Adapter. It emits only
// normalized observations; it never mutates application configuration or flows.
func DemoBatch(d DemoIdentity, anchor, at time.Time, accounts []adapters.AccountType) (adapters.Batch, error) {
	if err := ValidateDemoIdentity(d); err != nil {
		return adapters.Batch{}, err
	}
	batch := adapters.Batch{CollectedAt: at, MarginMode: demoMode(d, anchor, at)}
	for _, kind := range accounts {
		if !adapters.ValidAccountType(d.Structure, kind) {
			return adapters.Batch{}, adapters.UnknownAccountType(d.Platform, d.Structure, kind)
		}
		r := demoReport(d, anchor, at, kind)
		batch.Reports = append(batch.Reports, r)
	}
	needed := map[string]bool{}
	for _, r := range batch.Reports {
		for _, b := range r.Balances {
			needed[b.AssetSymbol] = true
		}
		for _, p := range r.Positions {
			needed[p.FuturesContract.BaseAssetSymbol], needed[p.FuturesContract.SettlementAssetSymbol] = true, true
		}
		for _, p := range r.DualPositions {
			if p.BaseAssetSymbol != nil {
				needed[*p.BaseAssetSymbol] = true
			}
		}
	}
	for _, asset := range demoAssets {
		if !needed[asset.Symbol] {
			continue
		}
		batch.Prices = append(batch.Prices, adapters.Price{CanonicalAssetSymbol: asset.Symbol, Price: DemoPrice(d.Platform, asset.Symbol, at), SourceMarketRef: asset.Symbol + "USDT", QuoteCanonicalAssetSymbol: "USDT", RetrievedAt: at})
	}
	return batch, nil
}
