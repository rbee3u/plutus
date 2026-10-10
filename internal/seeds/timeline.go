package seeds

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func localMidnight(t time.Time, daysAgo int, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysAgo)
}

func sameLocalDay(a, b time.Time, loc *time.Location) bool {
	return a.In(loc).Format("2006-01-02") == b.In(loc).Format("2006-01-02")
}

func cycleTimes(anchor time.Time, loc *time.Location, large bool) []time.Time {
	local := anchor.In(loc)
	month := time.Date(local.Year(), local.Month()-3, 1, 0, 0, 0, 0, loc)
	day := min(local.Day(), month.AddDate(0, 1, -1).Day())
	start := time.Date(month.Year(), month.Month(), day, 0, 0, 0, 0, loc)
	if large {
		start = localMidnight(anchor, 7, loc)
	}
	dense := localMidnight(anchor, 14, loc)
	times := map[int64]time.Time{anchor.Unix(): anchor}
	for at := start; at.Before(anchor); {
		times[at.Unix()] = at.UTC()
		if at.Before(dense) {
			at = at.AddDate(0, 0, 1)
		} else {
			at = at.Add(2 * time.Hour)
		}
	}
	for _, daysAgo := range []int{60, 33, 30, 10, 8, 6, 5, 4, 3, 2} {
		event := anchor.AddDate(0, 0, -daysAgo)
		if event.Before(start) {
			continue
		}
		for _, at := range []time.Time{event.Add(-time.Minute), event, event.Add(time.Minute)} {
			times[at.Unix()] = at
		}
	}
	out := make([]time.Time, 0, len(times))
	for _, at := range times {
		out = append(out, at)
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

func (b *builder) demoEvents(anchor time.Time) {
	for _, f := range []flowSpec{
		{ID: namedID("flow/btc-deposit"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", CanonicalAssetSymbol: "BTC", Quantity: "0.1", At: anchor.AddDate(0, 0, -60), Note: "Cold-wallet deposit into Binance Spot"},
		{ID: namedID("flow/usdt-deposit"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", CanonicalAssetSymbol: "USDT", Quantity: "5000", ValueUSD: "5000", At: anchor.AddDate(0, 0, -30), Note: "External funding into Binance Funding"},
		{ID: namedID("flow/usdc-withdrawal"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "withdrawal", CanonicalAssetSymbol: "USDC", Quantity: "1000", ValueUSD: "1000", At: anchor.AddDate(0, 0, -5), Note: "Withdrawal from Binance Funding"},
		{ID: namedID("flow/unpriced-airdrop"), Portfolio: PortfolioAliceEdge, User: UserAlice, Direction: "contribution", CanonicalAssetSymbol: "DEMO-NOPRICE", Quantity: "1000", At: anchor.AddDate(0, 0, -5), Note: "Unpriced airdrop; USD value remains unknown"},
	} {
		if f.CanonicalAssetSymbol == "BTC" {
			price := fake.DemoPrice(adapters.PlatformBinance, "BTC", f.At)
			f.ValueUSD = decimal.Format(decimal.Mul(decimal.ParseOrZero(price), decimal.ParseOrZero(f.Quantity)))
			b.price(f.Portfolio, adapters.PlatformBinance, "BTC", price, f.At)
		}
		id := accountID("daily-classic", "funding")
		if f.CanonicalAssetSymbol == "BTC" {
			id = accountID("daily-classic", "spot")
		}
		if f.Portfolio == PortfolioAliceEdge {
			id = accountID("edge-unpriced", "spot")
		}
		f.Account = &id
		b.flow(&f)
		b.audit(&auditSpec{Portfolio: f.Portfolio, Actor: UserAlice, Action: "capital_flow.create", ResourceType: "capital_flow", ResourceID: f.ID, At: f.At, After: map[string]any{"direction": f.Direction, "canonicalAssetSymbol": f.CanonicalAssetSymbol, "quantity": f.Quantity, "valueUsd": f.ValueUSD, "note": f.Note}})
	}
	for _, event := range []struct {
		identity, action string
		days             int
		before, after    any
	}{
		{"edge-boundary", "identity.update", -5, map[string]any{"accounts": map[string]bool{"spot": true, "funding": false}}, map[string]any{"accounts": map[string]bool{"spot": true, "funding": true}}},
		{"edge-boundary", "identity.update", -3, map[string]any{"accounts": map[string]bool{"spot": true, "funding": true}}, map[string]any{"accounts": map[string]bool{"spot": false, "funding": true}}},
		{"edge-archive", "identity.archive", -2, map[string]any{"status": "enabled"}, map[string]any{"status": "archived"}},
		{"edge-mode", "identity.margin_mode.change", -3, map[string]any{"marginMode": adapters.MarginBybitCross}, map[string]any{"marginMode": adapters.MarginBybitPM}},
	} {
		b.audit(&auditSpec{Portfolio: PortfolioAliceEdge, Actor: UserAlice, Action: event.action, ResourceType: "identity", ResourceID: identityID(event.identity), At: anchor.AddDate(0, 0, event.days), Before: event.before, After: event.after, System: event.action == "identity.margin_mode.change"})
	}
	b.demoAssetPolicy(anchor)
	// Independent Catalog reviews provide useful audit pagination without
	// inventing dozens of meaningless Identity renames.
	reviewed := 0
	for _, key := range b.assetKeys {
		row := b.assets[key]
		if row.portfolio != PortfolioAliceLarge || row.platform != adapters.PlatformBinance || row.source != catalog.MappingAuto || reviewed >= 60 {
			continue
		}
		reviewed++
		row.source = catalog.MappingManual
		b.audit(&auditSpec{Portfolio: PortfolioAliceLarge, Actor: UserAlice, Action: "catalog.asset.map", ResourceType: "asset", ResourceID: row.id, At: anchor.AddDate(0, 0, -7).Add(time.Duration(reviewed) * 2 * time.Hour), Before: map[string]any{"platform": row.platform, "symbol": row.symbol, "canonicalAssetSymbol": row.canonicalAssetSymbol, "mappingSource": catalog.MappingAuto, "version": 1}, After: map[string]any{"platform": row.platform, "symbol": row.symbol, "canonicalAssetSymbol": row.canonicalAssetSymbol, "mappingSource": catalog.MappingManual, "version": 2}})
	}
	if reviewed != 60 {
		b.fail(fmt.Errorf("large scenario needs 60 independent Catalog reviews"))
	}
}

func (b *builder) demoAssetPolicy(anchor time.Time) {
	alias := b.assets[PortfolioAliceEdge.String()+"|binance|DEMO-BTC"]
	if alias == nil {
		b.fail(fmt.Errorf("demo alias asset is missing"))
		return
	}
	alias.canonicalAssetSymbol, alias.source = "BTC", catalog.MappingManual
	b.audit(&auditSpec{Portfolio: PortfolioAliceEdge, Actor: UserAlice, Action: "catalog.asset.map", ResourceType: "asset", ResourceID: alias.id, At: anchor.AddDate(0, 0, -7), Before: map[string]any{"symbol": alias.symbol, "canonicalAssetSymbol": alias.symbol, "mappingSource": catalog.MappingAuto}, After: map[string]any{"symbol": alias.symbol, "canonicalAssetSymbol": "BTC", "mappingSource": catalog.MappingManual}})
	dust := b.assets[PortfolioAliceEdge.String()+"|binance|DEMO-DUST"]
	if dust == nil {
		b.fail(fmt.Errorf("demo dust asset is missing"))
		return
	}
	for i, days := range []int{-6, -4, -2} {
		excluded := i != 1
		reason := ""
		if excluded {
			reason = "Reviewed synthetic dust; intentionally omitted from calculated value."
		}
		at := anchor.AddDate(0, 0, days)
		b.add("asset_exclusion_versions", bin(PortfolioAliceEdge), bin(dust.id), strconv.Itoa(i+1), boolean(excluded), qn(reason), ts(at))
		action := "catalog.asset.restore"
		if excluded {
			action = "catalog.asset.exclude"
		}
		b.audit(&auditSpec{Portfolio: PortfolioAliceEdge, Actor: UserAlice, Action: action, ResourceType: "asset", ResourceID: dust.id, At: at, Before: map[string]any{"symbol": dust.symbol, "excluded": !excluded, "version": i}, After: map[string]any{"symbol": dust.symbol, "excluded": excluded, "reason": reason, "version": i + 1}})
	}
}
