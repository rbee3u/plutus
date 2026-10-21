package fake

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

const demoPrefix = "plutus-demo-v1:"

// DemoIdentity describes one offline platform login. Definitions contain no
// database IDs, credentials, or derived valuation fields.
type DemoIdentity struct {
	ID         string
	Portfolio  string
	Name       string
	Platform   adapter.Platform
	Structure  adapter.AccountStructure
	Mode       adapter.MarginMode
	Profile    string
	Connection string
}

// DemoIdentities returns an independent copy in stable ownership order.
func DemoIdentities() []DemoIdentity {
	return []DemoIdentity{
		{"daily-classic", "default", "sub-04", adapter.PlatformBinance, adapter.StructureBinanceClassic, "", "daily", "enabled"},
		{"daily-pm", "default", "main", adapter.PlatformBinance, adapter.StructureBinancePM, "", "daily", "enabled"},
		{"daily-okx", "default", "main", adapter.PlatformOKX, adapter.StructureOKXUnified, adapter.MarginOKXPM, "daily", "enabled"},
		{"daily-bybit", "default", "main", adapter.PlatformBybit, adapter.StructureBybitUTA2, adapter.MarginBybitPM, "daily", "enabled"},
		{"daily-kraken", "default", "sum", adapter.PlatformKraken, adapter.StructureKrakenSeparate, "", "daily", "enabled"},
		{"daily-kraken-unified", "default", "main", adapter.PlatformKraken, adapter.StructureKrakenUnified, "", "daily", "enabled"},
		{"daily-sub-01", "default", "sub-01", adapter.PlatformBinance, adapter.StructureBinancePM, "", "daily", "enabled"},
		{"daily-sub-02", "default", "sub-02", adapter.PlatformBinance, adapter.StructureBinancePM, "", "daily", "enabled"},
		{"daily-sub-03", "default", "sub-03", adapter.PlatformBinance, adapter.StructureBinancePM, "", "daily", "enabled"},
		{"daily-okx-sum", "default", "sum", adapter.PlatformOKX, adapter.StructureOKXUnified, adapter.MarginOKXMCM, "daily", "enabled"},
		{"daily-okx-coin", "default", "coinM", adapter.PlatformOKX, adapter.StructureOKXUnified, adapter.MarginOKXMCM, "daily", "enabled"},
		{"daily-okx-usd", "default", "usdM", adapter.PlatformOKX, adapter.StructureOKXUnified, adapter.MarginOKXMCM, "daily", "enabled"},
		{"retired-classic", "default", "retired", adapter.PlatformBinance, adapter.StructureBinanceClassic, "", "archived", "unconfigured"},
		{"edge-unpriced", "edge-cases", "Binance unpriced airdrop", adapter.PlatformBinance, adapter.StructureBinanceClassic, "", "unpriced", "enabled"},
		{"edge-stale", "edge-cases", "Binance delayed reports", adapter.PlatformBinance, adapter.StructureBinanceClassic, "", "stale", "enabled"},
		{"edge-missing", "edge-cases", "Bybit never collected", adapter.PlatformBybit, adapter.StructureBybitUTA2, adapter.MarginBybitCross, "missing", "enabled"},
	}
}

// ValidateDemoIdentity catches definition mistakes before producing SQL or facts.
func ValidateDemoIdentity(d DemoIdentity) error {
	if d.ID == "" || strings.Contains(d.ID, ":") || d.Name == "" ||
		!adapter.ValidStructure(d.Platform, d.Structure) || !adapter.ValidMarginMode(d.Structure, d.Mode) {
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
		return DemoIdentity{}, time.Time{}, true, adapter.NewError(adapter.ClassAuthentication, "The offline scenario key is invalid.")
	}
	seconds, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || seconds < 0 || seconds > 253402300799 {
		return DemoIdentity{}, time.Time{}, true, adapter.NewError(adapter.ClassAuthentication, "The offline scenario anchor is invalid.")
	}
	for _, d := range DemoIdentities() {
		if d.ID == parts[0] {
			return d, time.Unix(seconds, 0).UTC(), true, nil
		}
	}
	return DemoIdentity{}, time.Time{}, true, adapter.NewError(adapter.ClassUnsupported, "The offline scenario is not supported.")
}

// DemoEventTime is a stable UTC event instant relative to the seed anchor.
func DemoEventTime(anchor time.Time, days int) time.Time {
	return anchor.UTC().AddDate(0, 0, days)
}

// DemoAccounts returns the historical portfolio boundary for one login. Only
// seeds use this timeline; runtime always honors the owner's requested accounts.
func DemoAccounts(d DemoIdentity, anchor, at time.Time) []adapter.AccountType {
	switch d.Profile {
	case "daily", "large":
		if d.Profile == "daily" && d.Structure == adapter.StructureBinanceClassic {
			return []adapter.AccountType{"spot", "funding", "um_futures", "cm_futures", "earn", adapter.TypeDualInvestment}
		}
		return adapter.AccountTypeIDs(d.Structure)
	case "archived":
		if !at.Before(DemoEventTime(anchor, -2)) {
			return nil
		}
		return []adapter.AccountType{"spot", "funding"}
	case "boundary":
		if at.Before(DemoEventTime(anchor, -5)) {
			return []adapter.AccountType{"spot"}
		}
		if at.Before(DemoEventTime(anchor, -3)) {
			return []adapter.AccountType{"spot", "funding"}
		}
		return []adapter.AccountType{"funding"}
	case "stale":
		return []adapter.AccountType{"cm_futures", "spot"}
	case "unpriced":
		if !at.Before(DemoEventTime(anchor, -5)) && at.Before(DemoEventTime(anchor, -3)) {
			return []adapter.AccountType{"spot", "funding"}
		}
		return []adapter.AccountType{"spot"}
	case "missing", "mode":
		return []adapter.AccountType{"unified"}
	case "invalid", "unsupported", "metadata":
		return []adapter.AccountType{"trading", "earn", adapter.TypeDualInvestment}
	case "disabled":
		return []adapter.AccountType{"main"}
	case "unconfigured":
		return []adapter.AccountType{"spot"}
	case "empty":
		return []adapter.AccountType{"spot", "earn", adapter.TypeDualInvestment}
	default:
		return []adapter.AccountType{"spot"}
	}
}

func demoMode(d DemoIdentity, anchor, at time.Time) adapter.MarginMode {
	if (d.Profile == "mode" || d.ID == "daily-bybit") && at.Before(DemoEventTime(anchor, -3)) {
		return adapter.MarginBybitCross
	}
	return d.Mode
}

// DemoBatch is shared by history seeding and the fake adapter. It emits only
// normalized observations; it never mutates application configuration or flows.
func DemoBatch(d DemoIdentity, anchor, at time.Time, accounts []adapter.AccountType) (adapter.Batch, error) {
	if err := ValidateDemoIdentity(d); err != nil {
		return adapter.Batch{}, err
	}
	batch := adapter.Batch{CollectedAt: at, MarginMode: demoMode(d, anchor, at)}
	for _, kind := range accounts {
		if !adapter.ValidAccountType(d.Structure, kind) {
			return adapter.Batch{}, adapter.UnknownAccountType(d.Platform, d.Structure, kind)
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
		batch.Prices = append(batch.Prices, adapter.Price{CanonicalAssetSymbol: asset.Symbol, Price: DemoPrice(d.Platform, asset.Symbol, at), SourceMarketRef: asset.Symbol + "USDT", QuoteCanonicalAssetSymbol: "USDT", RetrievedAt: at})
	}
	return batch, nil
}
