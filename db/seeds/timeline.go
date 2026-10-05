package seeds

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/decimal"
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

func intString(n int) string { return strconv.Itoa(n) }

func (b *builder) demoEvents(anchor time.Time) {
	for _, f := range []flowSpec{
		{ID: namedID("flow/btc-deposit"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "BTC", Quantity: "0.1", At: anchor.AddDate(0, 0, -60), Note: "Cold-wallet deposit into Binance Spot"},
		{ID: namedID("flow/usdt-deposit"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "contribution", Asset: "USDT", Quantity: "5000", ValueUSD: "5000", At: anchor.AddDate(0, 0, -30), Note: "External funding into Binance Funding"},
		{ID: namedID("flow/usdc-withdrawal"), Portfolio: PortfolioAliceMain, User: UserAlice, Direction: "withdrawal", Asset: "USDC", Quantity: "1000", ValueUSD: "1000", At: anchor.AddDate(0, 0, -5), Note: "Withdrawal from Binance Funding"},
		{ID: namedID("flow/unpriced-airdrop"), Portfolio: PortfolioAliceEdge, User: UserAlice, Direction: "contribution", Asset: "DEMO-NOPRICE", Quantity: "1000", At: anchor.AddDate(0, 0, -5), Note: "Unpriced airdrop; USD value remains unknown"},
	} {
		if f.Asset == "BTC" {
			price := fake.DemoPrice(adapters.PlatformBinance, "BTC", f.At)
			f.ValueUSD = decimal.Format(decimal.Mul(decimal.MustParse(price), decimal.MustParse(f.Quantity)))
			b.price(adapters.PlatformBinance, "BTC", price, f.At)
		}
		id := accountID("daily-classic", "funding")
		if f.Asset == "BTC" {
			id = accountID("daily-classic", "spot")
		}
		if f.Portfolio == PortfolioAliceEdge {
			id = accountID("edge-unpriced", "spot")
		}
		f.Account = &id
		b.flow(&f)
		b.audit(&auditSpec{Portfolio: f.Portfolio, Actor: UserAlice, Action: "capital_flow.create", ResourceType: "capital_flow", ResourceID: f.ID, At: f.At, After: map[string]any{"direction": f.Direction, "assetSymbol": f.Asset, "quantity": f.Quantity, "valueUsd": f.ValueUSD, "note": f.Note}})
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
	b.demoListingPolicy(anchor)
	// Independent Catalog reviews provide useful audit pagination without
	// inventing dozens of meaningless Identity renames.
	reviewed := 0
	for _, key := range b.listingKeys {
		row := b.listings[key]
		if row.platform != adapters.PlatformBinance || row.source != catalog.MappingAuto || reviewed >= 60 {
			continue
		}
		reviewed++
		row.source = catalog.MappingManual
		b.audit(&auditSpec{Portfolio: PortfolioAliceLarge, Actor: UserAlice, Action: "catalog.listing.map", ResourceType: "asset_listing", ResourceID: row.id, At: anchor.AddDate(0, 0, -7).Add(time.Duration(reviewed) * 2 * time.Hour), Before: map[string]any{"platform": row.platform, "symbol": row.symbol, "assetSymbol": row.asset, "mappingSource": catalog.MappingAuto, "version": 1}, After: map[string]any{"platform": row.platform, "symbol": row.symbol, "assetSymbol": row.asset, "mappingSource": catalog.MappingManual, "version": 2}})
	}
	if reviewed != 60 {
		b.fail(fmt.Errorf("large scenario needs 60 independent Catalog reviews"))
	}
	for _, status := range []string{"failed", "cancelled"} {
		at := anchor.AddDate(0, 0, -1).Add(-time.Hour)
		id := namedID("job/" + status)
		b.add("jobs", bin(id), bin(PortfolioAliceEdge), q("valuation_recalc"), q(status), q("manual"), q("recalc"), "NULL", bin(UserAlice), ts(at), "3", "3", q(adapters.ClassTemporary), q("Offline example of a terminal rebuild job."), ts(at), ts(at.Add(time.Minute)), ts(at), ts(at.Add(time.Minute)))
	}
}

func (b *builder) demoListingPolicy(anchor time.Time) {
	alias := b.listings["binance|DEMO-BTC"]
	if alias == nil {
		b.fail(fmt.Errorf("demo alias listing is missing"))
		return
	}
	alias.asset, alias.source = "BTC", catalog.MappingManual
	b.audit(&auditSpec{Portfolio: PortfolioAliceEdge, Actor: UserAlice, Action: "catalog.listing.map", ResourceType: "asset_listing", ResourceID: alias.id, At: anchor.AddDate(0, 0, -7), Before: map[string]any{"symbol": alias.symbol, "assetSymbol": alias.symbol, "mappingSource": catalog.MappingAuto}, After: map[string]any{"symbol": alias.symbol, "assetSymbol": "BTC", "mappingSource": catalog.MappingManual}})
	dust := b.listings["binance|DEMO-DUST"]
	if dust == nil {
		b.fail(fmt.Errorf("demo dust listing is missing"))
		return
	}
	for i, days := range []int{-6, -4, -2} {
		ignored := i != 1
		reason := ""
		if ignored {
			reason = "Reviewed synthetic dust; intentionally omitted from calculated value."
		}
		at := anchor.AddDate(0, 0, days)
		b.add("portfolio_listing_exclusion_events", bin(PortfolioAliceEdge), bin(dust.id), intString(i+1), boolean(ignored), qn(reason), bin(UserAlice), ts(at))
		action := "catalog.listing.restore"
		if ignored {
			action = "catalog.listing.exclude"
		}
		b.audit(&auditSpec{Portfolio: PortfolioAliceEdge, Actor: UserAlice, Action: action, ResourceType: "asset_listing", ResourceID: dust.id, At: at, Before: map[string]any{"symbol": dust.symbol, "ignored": !ignored, "version": i}, After: map[string]any{"symbol": dust.symbol, "ignored": ignored, "reason": reason, "version": i + 1}})
		if i == 2 {
			b.add("portfolio_listing_exclusions", bin(PortfolioAliceEdge), bin(dust.id), boolean(ignored), qn(reason), "3", bin(UserAlice), ts(at))
		}
	}
}
