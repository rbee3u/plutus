package fake

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
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
		{"daily-classic", "default", "Binance trading desk", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "daily", "enabled"},
		{"daily-pm", "default", "Binance hedging desk", adapters.PlatformBinance, adapters.StructureBinancePM, "", "daily", "enabled"},
		{"daily-okx", "default", "OKX multi-currency", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "daily", "enabled"},
		{"daily-bybit", "default", "Bybit cross collateral", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitCross, "daily", "enabled"},
		{"daily-kraken", "default", "Kraken separate wallets", adapters.PlatformKraken, adapters.StructureKrakenSeparate, "", "daily", "enabled"},
		{"daily-kraken-unified", "default", "Kraken unified treasury", adapters.PlatformKraken, adapters.StructureKrakenUnified, "", "daily", "enabled"},
		{"large-classic", "large", "Binance diversified contracts and subscriptions", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "large", "enabled"},
		{"large-okx-spot", "large", "OKX spot mode", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXSpot, "daily", "enabled"},
		{"large-okx-futures", "large", "OKX futures mode", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXFutures, "daily", "enabled"},
		{"large-okx-pm", "large", "OKX portfolio margin", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXPM, "daily", "enabled"},
		{"large-bybit-isolated", "large", "Bybit isolated collateral", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitIsolated, "daily", "enabled"},
		{"large-bybit-pm", "large", "Bybit portfolio margin", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitPM, "daily", "enabled"},
		{"edge-unpriced", "edge-cases", "Binance unpriced airdrop", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "unpriced", "enabled"},
		{"edge-stale", "edge-cases", "Binance delayed COIN-M", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "stale", "enabled"},
		{"edge-stale-incomplete", "edge-cases", "Binance stale unpriced holdings", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "stale_incomplete", "enabled"},
		{"edge-missing", "edge-cases", "Bybit never collected", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitCross, "missing", "enabled"},
		{"edge-invalid", "edge-cases", "OKX revoked credentials", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "invalid", "invalid"},
		{"edge-disabled", "edge-cases", "Kraken collection paused", adapters.PlatformKraken, adapters.StructureKrakenSeparate, "", "disabled", "disabled"},
		{"edge-unconfigured", "edge-cases", "Binance awaiting credentials", adapters.PlatformBinance, adapters.StructureBinancePM, "", "unconfigured", "unconfigured"},
		{"edge-archive", "edge-cases", "Binance retired sub-account", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "archived", "unconfigured"},
		{"edge-boundary", "edge-cases", "Binance wallet boundary", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "boundary", "enabled"},
		{"edge-policy", "edge-cases", "Binance reviewed listing policy", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "policy", "enabled"},
		{"edge-metadata", "edge-cases", "OKX optional metadata unavailable", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "metadata", "enabled"},
		{"edge-legacy", "edge-cases", "Kraken legacy Earn summary", adapters.PlatformKraken, adapters.StructureKrakenSeparate, "", "legacy", "disabled"},
		{"edge-scope", "edge-cases", "OKX Earn scope transition", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "scope", "enabled"},
		{"edge-mode", "edge-cases", "Bybit observed mode change", adapters.PlatformBybit, adapters.StructureBybitUTA2, adapters.MarginBybitPM, "mode", "enabled"},
		{"edge-empty", "edge-cases", "Binance successful empty wallets", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "empty", "enabled"},
		{"edge-unsupported", "edge-cases", "OKX unsupported product report", adapters.PlatformOKX, adapters.StructureOKXUnified, adapters.MarginOKXMCM, "unsupported", "enabled"},
		{"edge-normalization", "edge-cases", "Binance unavailable core equity", adapters.PlatformBinance, adapters.StructureBinanceClassic, "", "normalization", "enabled"},
	}
}

// ValidateDemoIdentity catches definition mistakes before producing SQL or facts.
func ValidateDemoIdentity(d DemoIdentity) error {
	if d.ID == "" || strings.Contains(d.ID, ":") || d.Name == "" ||
		!adapters.ValidStructure(d.Platform, d.Structure) || !adapters.ValidMarginMode(d.Structure, d.Mode) {
		return fmt.Errorf("invalid demo identity %q: identity or capability", d.ID)
	}
	if d.Portfolio != "default" && d.Portfolio != "edge-cases" && d.Portfolio != "large" {
		return fmt.Errorf("invalid demo identity %q: portfolio", d.ID)
	}
	switch d.Connection {
	case "enabled", "disabled", "invalid", "unconfigured":
	default:
		return fmt.Errorf("invalid demo identity %q: connection", d.ID)
	}
	switch d.Profile {
	case "daily", "large", "unpriced", "stale", "stale_incomplete", "missing", "invalid", "disabled", "unconfigured", "archived", "boundary", "policy", "metadata", "legacy", "scope", "mode", "empty", "unsupported", "normalization":
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
	if d.Portfolio != "edge-cases" {
		return adapters.AccountTypeIDs(d.Structure)
	}
	switch d.Profile {
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
	case "scope":
		if at.Before(DemoEventTime(anchor, -4)) {
			return []adapters.AccountType{"earn"}
		}
		return []adapters.AccountType{"earn", adapters.TypeDualInvestment}
	case "stale":
		return []adapters.AccountType{"cm_futures"}
	case "missing", "mode":
		return []adapters.AccountType{"unified"}
	case "invalid", "unsupported", "metadata":
		return []adapters.AccountType{"trading", "earn", adapters.TypeDualInvestment}
	case "legacy":
		return []adapters.AccountType{"earn"}
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

// DemoScope preserves deliberately old combined Earn reports, never backfills them.
func DemoScope(d DemoIdentity, anchor, at time.Time, kind adapters.AccountType) uint16 {
	if d.Profile == "scope" && at.Before(DemoEventTime(anchor, -4)) && kind == "earn" {
		return 1
	}
	return adapters.ReportScope(d.Platform, kind)
}

func demoMode(d DemoIdentity, anchor, at time.Time) adapters.MarginMode {
	if d.Profile == "mode" && at.Before(DemoEventTime(anchor, -3)) {
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
	batch := adapters.Batch{ObservedAt: at, MarginMode: demoMode(d, anchor, at)}
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
			needed[b.Symbol] = true
		}
		for _, p := range r.Positions {
			needed[p.Instrument.BaseSymbol], needed[p.Instrument.SettlementSymbol] = true, true
		}
		for _, p := range r.DualPositions {
			if p.BaseSymbol != nil {
				needed[*p.BaseSymbol] = true
			}
		}
	}
	for _, asset := range demoAssets {
		if !needed[asset.Symbol] {
			continue
		}
		batch.Prices = append(batch.Prices, adapters.Price{AssetSymbol: asset.Symbol, Price: DemoPrice(d.Platform, asset.Symbol, at), QuoteSymbol: "USDT", ObservedAt: at})
	}
	return batch, nil
}
