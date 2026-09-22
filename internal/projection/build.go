package projection

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type builtGeneration struct {
	id            uuid.UUID
	portfolioID   uuid.UUID
	displayAsset  uuid.UUID
	balances      []latestBalance
	positions     []latestPosition
	valuations    []valuationRow
	contributions []contributionRow
	exposures     []exposureRow
	capitals      []capitalRow
	changes       []changeRow
	summaries     []summaryRow
	issues        []issueRow
}

type latestBalance struct {
	IdentityID, AccountID, ListingID, ObservationID uuid.UUID
	AssetID                                         *uuid.UUID
	ObservedAt                                      time.Time
	Total, Available, Locked                        *string
	Freshness, Completeness                         string
}

type latestPosition struct {
	IdentityID, AccountID, InstrumentID, ObservationID uuid.UUID
	Ref, Side, Quantity, Freshness, Completeness       string
	ObservedAt                                         time.Time
	Mark, Unrealized                                   *string
	Long, Short                                        string
}

type valuationRow struct {
	ID                                   uuid.UUID
	EvaluatedAt                          time.Time
	Amount                               *string
	Completeness, Fingerprint, Freshness string
}

type contributionRow struct {
	ID, ValuationID                uuid.UUID
	Kind                           string
	IdentityID, AccountID, AssetID *uuid.UUID
	Amount                         *string
	Completeness, Fingerprint      string
	Sources                        []uuid.UUID
}

type exposureRow struct {
	EvaluatedAt                     time.Time
	AssetID                         uuid.UUID
	Long, Short, Net                string
	LongValue, ShortValue, NetValue *string
	Completeness, Freshness         string
}

type capitalRow struct {
	EvaluatedAt time.Time
	Amount      *string
	Complete    string
	Watermark   uint64
}

type changeRow struct {
	ID, BeginID, EndID        uuid.UUID
	Kind                      string
	Start, End                time.Time
	AsOf                      time.Time
	Contrib, Withdraw, Amount *string
	State, Fingerprint        string
}

type summaryRow struct {
	Kind, Freshness, Completeness           string
	Start, End, AsOf                        time.Time
	Open, Close, High, Low, Capital, Change *string
}

type issueRow struct {
	Kind, Key, Severity, Code, Summary   string
	IdentityID, AccountID, ObservationID *uuid.UUID
	First, Last                          time.Time
}

func (s *Service) build(f *facts, generationID uuid.UUID, asOf time.Time) *builtGeneration {
	loc, err := time.LoadLocation(f.timezone)
	if err != nil {
		loc = time.UTC
	}
	b := &builder{facts: f, genID: generationID, asOf: asOf, loc: loc, valuations: map[int64]*valuationRow{}}
	out := &builtGeneration{id: generationID, portfolioID: f.portfolioID, displayAsset: f.displayAsset}
	latest := b.latest(asOf)
	out.balances = latest.balances
	out.positions = latest.positions
	times := b.evalTimes()
	for _, at := range times {
		val := b.valuation(at)
		out.valuations = append(out.valuations, val.row)
		out.contributions = append(out.contributions, val.contribs...)
		out.exposures = append(out.exposures, b.exposure(at)...)
		out.capitals = append(out.capitals, b.contributed(at))
	}
	out.changes, out.summaries = b.periods(out.valuations, out.capitals)
	out.issues = b.issues
	return out
}

type builder struct {
	facts      *facts
	genID      uuid.UUID
	asOf       time.Time
	loc        *time.Location
	valuations map[int64]*valuationRow
	issues     []issueRow
}

type latestSet struct {
	balances  []latestBalance
	positions []latestPosition
}

func (b *builder) latest(cutoff time.Time) latestSet {
	out := latestSet{}
	for _, item := range b.selectedBalances(cutoff) {
		mapped, assetID := b.mapListing(item.ListingID, item.ObservedAt)
		fresh, complete := b.freshness(item.IdentityID, item.AccountID, "balance", item.ObservedAt, cutoff, item.Completeness)
		row := latestBalance{
			IdentityID: item.IdentityID, AccountID: item.AccountID, ListingID: item.ListingID, ObservationID: item.ID,
			ObservedAt: item.ObservedAt, Total: item.Total, Available: item.Available, Locked: item.Locked,
			Freshness: fresh, Completeness: complete,
		}
		if mapped {
			row.AssetID = &assetID
		}
		out.balances = append(out.balances, row)
	}
	for _, item := range b.selectedPositions(cutoff) {
		fresh, complete := b.freshness(item.IdentityID, item.AccountID, "position", item.ObservedAt, cutoff, item.Completeness)
		inst := b.facts.instruments[item.InstrumentID]
		long, short := positionExposure(item, &inst)
		out.positions = append(out.positions, latestPosition{
			IdentityID: item.IdentityID, AccountID: item.AccountID, InstrumentID: item.InstrumentID, ObservationID: item.ID,
			Ref: item.Ref, Side: item.Side, Quantity: item.Quantity, Freshness: fresh, Completeness: complete,
			ObservedAt: item.ObservedAt, Mark: item.Mark, Unrealized: item.Unrealized, Long: formatRat(long), Short: formatRat(short),
		})
	}
	return out
}

func positionExposure(item *positionFact, inst *instrumentFact) (*big.Rat, *big.Rat) {
	if inst == nil {
		inst = &instrumentFact{}
	}
	qty, ok := parseRat(item.Quantity)
	if !ok {
		qty = new(big.Rat)
	}
	size, ok := parseRat(inst.ContractSize)
	if !ok {
		size = big.NewRat(1, 1)
	}
	notional := mulRat(qty, size)
	long, short := new(big.Rat), new(big.Rat)
	switch item.Side {
	case "short":
		short = absRat(notional)
	case "net":
		if ratSign(notional) < 0 {
			short = absRat(notional)
		} else {
			long = absRat(notional)
		}
	default:
		long = absRat(notional)
	}
	return long, short
}

type valBundle struct {
	row      valuationRow
	contribs []contributionRow
}

func (b *builder) valuation(cutoff time.Time) valBundle {
	if existing, ok := b.valuations[cutoff.UnixMicro()]; ok {
		return valBundle{row: *existing}
	}
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	accounts := b.facts.accounts
	selected, overlapping := b.selectReported(cutoff)
	covered := map[uuid.UUID]bool{}
	var contribs []contributionRow
	sum := new(big.Rat)
	complete := true
	fresh := "fresh"
	if overlapping {
		complete = false
		b.addIssue("portfolio_valuation", "coverage", "warning", "partial_equity_overlap", "Reported Equity coverage overlaps without containment.", cutoff, nil, nil, nil)
	}
	for _, eq := range selected {
		set := b.coverageSet(&eq)
		for accountID := range set {
			covered[accountID] = true
		}
		amount, obsID, ok := b.convertAmount(eq.Amount, eq.DenomListingID, eq.IdentityID, cutoff)
		cid, _ := uuid.NewV7()
		row := contributionRow{
			ID: cid, ValuationID: id, Kind: "reported_equity", IdentityID: &eq.IdentityID,
			Completeness: eq.Completeness, Fingerprint: hashFingerprint(coverageFingerprint(set, "reported_equity")),
			Sources: []uuid.UUID{eq.ID},
		}
		if obsID != nil {
			row.Sources = append(row.Sources, *obsID)
		}
		if !ok || amount == nil {
			complete = false
			row.Completeness = "missing"
			b.addIssue("portfolio_valuation", eq.ID.String(), "error", "missing_price", "Reported Equity could not be converted to the display asset.", cutoff, &eq.IdentityID, nil, &eq.ID)
		} else {
			value := formatRat(amount)
			row.Amount = &value
			sum = addRat(sum, amount)
		}
		if eq.Completeness != "complete" {
			complete = false
		}
		contribs = append(contribs, row)
		if f, _ := b.freshness(eq.IdentityID, uuid.Nil, "reported_equity", eq.ObservedAt, cutoff, eq.Completeness); f == "stale" {
			fresh = "stale"
		}
	}
	for _, account := range accounts {
		if covered[account.ID] {
			continue
		}
		calc := b.calculateAccount(account, cutoff)
		if calc.skip {
			continue
		}
		calc.row.ValuationID = id
		contribs = append(contribs, calc.row)
		if calc.row.Amount != nil {
			if parsed, ok := parseRat(*calc.row.Amount); ok {
				sum = addRat(sum, parsed)
			}
		} else {
			complete = false
		}
		if calc.row.Completeness != "complete" {
			complete = false
		}
		if calc.fresh == "stale" {
			fresh = "stale"
		}
		covered[account.ID] = true
	}
	for _, ident := range b.requiredMembers(cutoff) {
		if !b.hasSuccessfulSnapshot(ident.ID, cutoff) {
			b.addIssue("portfolio_valuation", ident.ID.String(), "warning", "stale_scope", "Identity has no successful snapshot at the cutoff.", cutoff, &ident.ID, nil, nil)
			complete = false
			if fresh == "fresh" {
				fresh = "unknown"
			}
		}
	}
	status := "complete"
	var amount *string
	switch {
	case len(contribs) == 0:
		status = "missing"
		fresh = "unknown"
		b.addIssue("portfolio_valuation", cutoff.UTC().Format(time.RFC3339Nano), "warning", "incomplete_inputs", "No defensible Portfolio Valuation inputs were available.", cutoff, nil, nil, nil)
	case !complete:
		status = "partial"
		if sum != nil && overlapping {
			amount = nil
		} else if completeAmounts(contribs) {
			value := formatRat(sum)
			amount = &value
		}
	default:
		value := formatRat(sum)
		amount = &value
	}
	row := valuationRow{ID: id, EvaluatedAt: cutoff, Amount: amount, Completeness: status, Fingerprint: fingerprintFromContribs(contribs, b.requiredMembers(cutoff), cutoff, b), Freshness: fresh}
	b.valuations[cutoff.UnixMicro()] = &row
	return valBundle{row: row, contribs: contribs}
}

func completeAmounts(rows []contributionRow) bool {
	for i := range rows {
		if rows[i].Amount == nil {
			return false
		}
	}
	return true
}

func (b *builder) selectReported(cutoff time.Time) ([]equityFact, bool) {
	candidates := b.selectedEquities(cutoff)
	type scored struct {
		eq  equityFact
		set map[uuid.UUID]bool
	}
	items := make([]scored, 0, len(candidates))
	for i := range candidates {
		items = append(items, scored{eq: candidates[i], set: b.coverageSet(&candidates[i])})
	}
	sort.Slice(items, func(i, j int) bool { return len(items[i].set) > len(items[j].set) })
	var selected []equityFact
	used := map[uuid.UUID]bool{}
	overlapping := false
	for i := range items {
		item := &items[i]
		contained := true
		overlap := false
		for accountID := range item.set {
			if used[accountID] {
				overlap = true
			} else {
				contained = false
			}
		}
		if contained && len(item.set) > 0 {
			continue
		}
		if overlap {
			overlapping = true
			continue
		}
		for accountID := range item.set {
			used[accountID] = true
		}
		selected = append(selected, item.eq)
	}
	return selected, overlapping
}

func (b *builder) coverageSet(eq *equityFact) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, id := range b.facts.equityAccts[eq.ID] {
		out[id] = true
	}
	if len(out) > 0 {
		return out
	}
	identities := b.facts.equityIDs[eq.ID]
	if len(identities) == 0 {
		identities = []uuid.UUID{eq.IdentityID}
	}
	wanted := map[uuid.UUID]bool{}
	for _, id := range identities {
		wanted[id] = true
	}
	for _, account := range b.facts.accounts {
		if wanted[account.IdentityID] {
			out[account.ID] = true
		}
	}
	return out
}

type calcResult struct {
	row   contributionRow
	skip  bool
	fresh string
}

func (b *builder) calculateAccount(account accountFact, cutoff time.Time) calcResult {
	latest := b.latest(cutoff)
	var sources []uuid.UUID
	sum := new(big.Rat)
	okAll := true
	fresh := "fresh"
	cid, _ := uuid.NewV7()
	row := contributionRow{ID: cid, Kind: "calculated", AccountID: &account.ID, IdentityID: &account.IdentityID, Completeness: "complete"}
	found := false
	for i := range latest.balances {
		item := &latest.balances[i]
		if item.AccountID != account.ID {
			continue
		}
		found = true
		sources = append(sources, item.ObservationID)
		qty := quantityPref(item.Total, item.Available, item.Locked)
		if qty == nil || item.AssetID == nil {
			okAll = false
			continue
		}
		platform := b.platformOf(account.IdentityID)
		converted, _, ok := b.convert(qty, *item.AssetID, b.facts.displayAsset, platform, cutoff)
		if !ok {
			okAll = false
			b.addIssue("portfolio_valuation", item.ObservationID.String(), "error", "missing_price", "A Balance could not be converted to the display asset.", cutoff, &account.IdentityID, &account.ID, &item.ObservationID)
			continue
		}
		sum = addRat(sum, converted)
		if item.Freshness == "stale" {
			fresh = "stale"
		}
		if item.Completeness != "complete" {
			row.Completeness = "partial"
		}
	}
	for i := range latest.positions {
		item := &latest.positions[i]
		if item.AccountID != account.ID || item.Unrealized == nil {
			continue
		}
		found = true
		sources = append(sources, item.ObservationID)
		inst := b.facts.instruments[item.InstrumentID]
		quoteAsset := b.assetOfListing(inst.QuoteID)
		if quoteAsset == uuid.Nil {
			okAll = false
			continue
		}
		converted, _, ok := b.convert(parseOrNil(*item.Unrealized), quoteAsset, b.facts.displayAsset, inst.PlatformID, cutoff)
		if !ok {
			okAll = false
			b.addIssue("portfolio_valuation", item.ObservationID.String(), "error", "missing_price", "A Position unrealized value could not be converted.", cutoff, &account.IdentityID, &account.ID, &item.ObservationID)
			continue
		}
		sum = addRat(sum, converted)
	}
	if !found {
		return calcResult{skip: true}
	}
	row.Sources = sources
	row.Fingerprint = hashFingerprint(coverageFingerprint(map[uuid.UUID]bool{account.ID: true}, "calculated"))
	if !okAll {
		row.Completeness = "missing"
		return calcResult{row: row, fresh: fresh}
	}
	value := formatRat(sum)
	row.Amount = &value
	return calcResult{row: row, fresh: fresh}
}

func quantityPref(values ...*string) *big.Rat {
	for _, value := range values {
		if value == nil {
			continue
		}
		if parsed, ok := parseRat(*value); ok {
			return parsed
		}
	}
	return nil
}

func parseOrNil(value string) *big.Rat {
	parsed, ok := parseRat(value)
	if !ok {
		return nil
	}
	return parsed
}

func (b *builder) convertAmount(amount string, listing, identity uuid.UUID, cutoff time.Time) (*big.Rat, *uuid.UUID, bool) {
	qty, ok := parseRat(amount)
	if !ok {
		return nil, nil, false
	}
	mapped, assetID := b.mapListing(listing, cutoff)
	if !mapped {
		return nil, nil, false
	}
	return b.convert(qty, assetID, b.facts.displayAsset, b.platformOf(identity), cutoff)
}

func (b *builder) convert(qty *big.Rat, from, to, platform uuid.UUID, cutoff time.Time) (*big.Rat, *uuid.UUID, bool) {
	if qty == nil {
		return nil, nil, false
	}
	if from == to {
		return new(big.Rat).Set(qty), nil, true
	}
	if rate, obs, ok := b.rate(from, to, platform, cutoff); ok {
		return mulRat(qty, rate), obs, true
	}
	return nil, nil, false
}

func (b *builder) rate(from, to, platform uuid.UUID, cutoff time.Time) (*big.Rat, *uuid.UUID, bool) {
	if direct, obs, ok := b.fxRate(from, to, platform, cutoff); ok {
		return direct, obs, true
	}
	if mark, quote, obs, ok := b.markRate(from, platform, cutoff); ok {
		if quote == to {
			return mark, obs, true
		}
		if fx, fxObs, ok := b.fxRate(quote, to, platform, cutoff); ok {
			_ = fxObs
			return mulRat(mark, fx), obs, true
		}
	}
	return nil, nil, false
}

func (b *builder) fxRate(from, to, platform uuid.UUID, cutoff time.Time) (*big.Rat, *uuid.UUID, bool) {
	fromListing := b.listingOf(from, platform)
	toListing := b.listingOf(to, platform)
	if fromListing == uuid.Nil || toListing == uuid.Nil {
		return nil, nil, false
	}
	var best *priceFact
	for i := range b.facts.prices {
		item := &b.facts.prices[i]
		if item.ObservedAt.After(cutoff) || item.Type != "exchange_rate" || item.BaseListingID == nil {
			continue
		}
		if platform != uuid.Nil && b.platformOf(item.IdentityID) != platform {
			continue
		}
		if *item.BaseListingID == fromListing && item.QuoteListingID == toListing {
			if best == nil || item.ObservedAt.After(best.ObservedAt) {
				best = item
			}
		}
	}
	if best == nil {
		return nil, nil, false
	}
	rate, ok := parseRat(best.Price)
	if !ok {
		return nil, nil, false
	}
	id := best.ID
	return rate, &id, true
}

func (b *builder) markRate(from, platform uuid.UUID, cutoff time.Time) (*big.Rat, uuid.UUID, *uuid.UUID, bool) {
	var best *priceFact
	var quote uuid.UUID
	for i := range b.facts.prices {
		item := &b.facts.prices[i]
		if item.ObservedAt.After(cutoff) || item.InstrumentID == nil || (item.Type != "mark" && item.Type != "market") {
			continue
		}
		inst, ok := b.facts.instruments[*item.InstrumentID]
		if !ok || inst.PlatformID != platform {
			continue
		}
		underlying := inst.UnderlyingID
		if underlying == nil {
			underlying = inst.BaseID
		}
		mapped, assetID := b.mapListingPtr(underlying)
		if !mapped || assetID != from {
			continue
		}
		quoteMapped, quoteAsset := b.mapListingPtr(inst.QuoteID)
		if !quoteMapped {
			continue
		}
		if best == nil || item.ObservedAt.After(best.ObservedAt) {
			best = item
			quote = quoteAsset
		}
	}
	if best == nil {
		return nil, uuid.Nil, nil, false
	}
	rate, ok := parseRat(best.Price)
	if !ok {
		return nil, uuid.Nil, nil, false
	}
	id := best.ID
	return rate, quote, &id, true
}

func (b *builder) exposure(cutoff time.Time) []exposureRow {
	latest := b.latest(cutoff)
	type acc struct {
		long, short *big.Rat
		complete    bool
		fresh       string
	}
	byAsset := map[uuid.UUID]*acc{}
	ensure := func(id uuid.UUID) *acc {
		item, ok := byAsset[id]
		if !ok {
			item = &acc{long: new(big.Rat), short: new(big.Rat), complete: true, fresh: "fresh"}
			byAsset[id] = item
		}
		return item
	}
	for i := range latest.balances {
		item := &latest.balances[i]
		if item.AssetID == nil {
			b.addIssue("exposure", item.ListingID.String(), "warning", "missing_mapping", "A Balance listing is not mapped to an Asset.", cutoff, &item.IdentityID, &item.AccountID, &item.ObservationID)
			continue
		}
		qty := quantityPref(item.Total, item.Available, item.Locked)
		if qty == nil {
			continue
		}
		bucket := ensure(*item.AssetID)
		if ratSign(qty) < 0 {
			bucket.short = addRat(bucket.short, absRat(qty))
		} else {
			bucket.long = addRat(bucket.long, qty)
		}
		if item.Freshness == "stale" {
			bucket.fresh = "stale"
		}
		if item.Completeness != "complete" {
			bucket.complete = false
		}
	}
	for i := range latest.positions {
		item := &latest.positions[i]
		inst := b.facts.instruments[item.InstrumentID]
		mapped, assetID := b.mapListingPtr(inst.UnderlyingID)
		if !mapped {
			mapped, assetID = b.mapListingPtr(inst.BaseID)
		}
		if !mapped {
			b.addIssue("exposure", item.InstrumentID.String(), "warning", "missing_mapping", "A Position instrument is not mapped to an underlying Asset.", cutoff, &item.IdentityID, &item.AccountID, &item.ObservationID)
			continue
		}
		long := parseRatMust(item.Long)
		short := parseRatMust(item.Short)
		bucket := ensure(assetID)
		bucket.long = addRat(bucket.long, long)
		bucket.short = addRat(bucket.short, short)
		if item.Freshness == "stale" {
			bucket.fresh = "stale"
		}
	}
	out := make([]exposureRow, 0, len(byAsset))
	for assetID, item := range byAsset {
		net := subRat(item.long, item.short)
		row := exposureRow{
			EvaluatedAt: cutoff, AssetID: assetID,
			Long: formatRat(item.long), Short: formatRat(item.short), Net: formatRat(net),
			Freshness: item.fresh, Completeness: "complete",
		}
		if !item.complete {
			row.Completeness = "partial"
		}
		if lv, _, ok := b.convert(item.long, assetID, b.facts.displayAsset, b.platformForAsset(assetID), cutoff); ok {
			value := formatRat(lv)
			row.LongValue = &value
		} else {
			row.Completeness = "partial"
			b.addIssue("exposure", assetID.String(), "error", "missing_price", "Exposure quantity could not be valued in the display asset.", cutoff, nil, nil, nil)
		}
		if sv, _, ok := b.convert(item.short, assetID, b.facts.displayAsset, b.platformForAsset(assetID), cutoff); ok {
			value := formatRat(sv)
			row.ShortValue = &value
		}
		if row.LongValue != nil && row.ShortValue != nil {
			nv := subRat(parseOrNil(*row.LongValue), parseOrNil(*row.ShortValue))
			value := formatRat(nv)
			row.NetValue = &value
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssetID.String() < out[j].AssetID.String() })
	return out
}

func parseRatMust(value string) *big.Rat {
	parsed, ok := parseRat(value)
	if !ok {
		return new(big.Rat)
	}
	return parsed
}

func (b *builder) contributed(cutoff time.Time) capitalRow {
	watermark := uint64(0)
	if b.facts.baseline != nil {
		watermark += b.facts.baseline.Version * 1_000_000
	}
	for _, flow := range b.facts.flows {
		watermark += flow.Version
	}
	row := capitalRow{EvaluatedAt: cutoff, Complete: "complete", Watermark: watermark}
	sum := new(big.Rat)
	okAll := true
	if b.facts.baseline != nil && !b.facts.baseline.EffectiveAt.After(cutoff) {
		qty, ok := parseRat(b.facts.baseline.Quantity)
		if !ok {
			okAll = false
		} else if converted, _, ok := b.convert(qty, b.facts.baseline.AssetID, b.facts.displayAsset, uuid.Nil, b.facts.baseline.EffectiveAt); ok {
			sum = addRat(sum, converted)
		} else {
			okAll = false
			b.addIssue("contributed_capital", "baseline", "error", "missing_capital_flow_conversion", "The Capital Baseline could not be converted at its effective time.", cutoff, nil, nil, nil)
		}
	} else if b.facts.baseline == nil {
		row.Complete = "missing"
		b.addIssue("contributed_capital", "missing", "warning", "incomplete_inputs", "No Capital Baseline is recorded.", cutoff, nil, nil, nil)
		return row
	}
	for _, flow := range b.facts.flows {
		if flow.State != "active" || flow.EffectiveAt.After(cutoff) {
			continue
		}
		qty, ok := parseRat(flow.Quantity)
		if !ok {
			okAll = false
			continue
		}
		converted, _, ok := b.convert(qty, flow.AssetID, b.facts.displayAsset, uuid.Nil, flow.EffectiveAt)
		if !ok {
			okAll = false
			b.addIssue("contributed_capital", flow.EffectiveAt.UTC().Format(time.RFC3339Nano), "error", "missing_capital_flow_conversion", "A Capital Flow could not be converted at its effective time.", cutoff, nil, nil, nil)
			continue
		}
		if flow.Direction == "withdrawal" {
			sum = subRat(sum, converted)
		} else {
			sum = addRat(sum, converted)
		}
	}
	if !okAll {
		row.Complete = "partial"
		return row
	}
	value := formatRat(sum)
	row.Amount = &value
	return row
}

func (b *builder) periods(vals []valuationRow, capitals []capitalRow) ([]changeRow, []summaryRow) {
	byTime := map[int64]valuationRow{}
	for _, item := range vals {
		byTime[item.EvaluatedAt.UnixMicro()] = item
	}
	capByTime := map[int64]capitalRow{}
	for _, item := range capitals {
		capByTime[item.EvaluatedAt.UnixMicro()] = item
	}
	var changes []changeRow
	var summaries []summaryRow
	for _, spec := range b.periodSpecs() {
		beginAt := spec.start
		endAt := spec.end
		provisional := false
		if endAt.After(b.asOf) {
			endAt = b.asOf
			provisional = true
		}
		begin, okBegin := byTime[beginAt.UnixMicro()]
		end, okEnd := byTime[endAt.UnixMicro()]
		if !okBegin || !okEnd {
			continue
		}
		id, _ := uuid.NewV7()
		contrib, withdraw, convOK := b.flowTotals(beginAt, endAt)
		state := "complete"
		var amount *string
		switch {
		case begin.Fingerprint != end.Fingerprint:
			state = "incomparable"
			b.addIssue("value_change", spec.kind+":"+spec.startDate.Format(time.DateOnly), "warning", "changed_coverage", "Value Change endpoints have different coverage fingerprints.", endAt, nil, nil, nil)
		case begin.Completeness != "complete" || end.Completeness != "complete" || begin.Amount == nil || end.Amount == nil || !convOK:
			state = "incomplete"
		default:
			delta := subRat(parseOrNil(*end.Amount), parseOrNil(*begin.Amount))
			delta = subRat(delta, contrib)
			delta = addRat(delta, withdraw)
			value := formatRat(delta)
			amount = &value
			if provisional {
				state = "provisional"
			}
		}
		var contribStr, withdrawStr *string
		if convOK {
			c, w := formatRat(contrib), formatRat(withdraw)
			contribStr, withdrawStr = &c, &w
		}
		changes = append(changes, changeRow{
			ID: id, BeginID: begin.ID, EndID: end.ID, Kind: spec.kind, Start: spec.startDate, End: spec.endDate, AsOf: b.asOf,
			Contrib: contribStr, Withdraw: withdrawStr, Amount: amount, State: state, Fingerprint: hashFingerprint(begin.Fingerprint + "->" + end.Fingerprint),
		})
		capitalClose := capByTime[endAt.UnixMicro()]
		open, closeAmt := begin.Amount, end.Amount
		high, low := closeAmt, closeAmt
		for _, item := range vals {
			if item.EvaluatedAt.Before(beginAt) || item.EvaluatedAt.After(endAt) || item.Amount == nil {
				continue
			}
			if high == nil || ratGreater(*item.Amount, *high) {
				copied := *item.Amount
				high = &copied
			}
			if low == nil || ratGreater(*low, *item.Amount) {
				copied := *item.Amount
				low = &copied
			}
		}
		sumComplete := "complete"
		if state == "incomplete" || state == "incomparable" {
			sumComplete = "partial"
		}
		if begin.Completeness == "missing" && end.Completeness == "missing" {
			sumComplete = "missing"
		}
		summaries = append(summaries, summaryRow{
			Kind: spec.kind, Start: spec.startDate, End: spec.endDate, AsOf: b.asOf,
			Open: open, Close: closeAmt, High: high, Low: low, Capital: capitalClose.Amount, Change: amount,
			Freshness: worseFresh(begin.Freshness, end.Freshness), Completeness: sumComplete,
		})
	}
	return changes, summaries
}

func ratGreater(a, b string) bool {
	left, okA := parseRat(a)
	right, okB := parseRat(b)
	if !okA || !okB {
		return false
	}
	return left.Cmp(right) > 0
}

func worseFresh(a, b string) string {
	if a == "stale" || b == "stale" {
		return "stale"
	}
	if a == "unknown" || b == "unknown" {
		return "unknown"
	}
	return "fresh"
}

func (b *builder) flowTotals(from, to time.Time) (*big.Rat, *big.Rat, bool) {
	contrib, withdraw := new(big.Rat), new(big.Rat)
	okAll := true
	for _, flow := range b.facts.flows {
		if flow.State != "active" || !flow.EffectiveAt.After(from) || flow.EffectiveAt.After(to) {
			continue
		}
		qty, ok := parseRat(flow.Quantity)
		if !ok {
			okAll = false
			continue
		}
		converted, _, ok := b.convert(qty, flow.AssetID, b.facts.displayAsset, uuid.Nil, flow.EffectiveAt)
		if !ok {
			okAll = false
			continue
		}
		if flow.Direction == "withdrawal" {
			withdraw = addRat(withdraw, converted)
		} else {
			contrib = addRat(contrib, converted)
		}
	}
	return contrib, withdraw, okAll
}

type periodSpec struct {
	kind               string
	start, end         time.Time
	startDate, endDate time.Time
}

func (b *builder) periodSpecs() []periodSpec {
	start := b.firstInstant()
	localStart := start.In(b.loc)
	localAsOf := b.asOf.In(b.loc)
	out := make([]periodSpec, 0, 5)
	out = append(out, iterateDays(localStart, localAsOf, b.loc)...)
	out = append(out, iterateCalendar(localStart, localAsOf, b.loc, "iso_week")...)
	out = append(out, iterateCalendar(localStart, localAsOf, b.loc, "month")...)
	out = append(out, iterateCalendar(localStart, localAsOf, b.loc, "quarter")...)
	out = append(out, iterateCalendar(localStart, localAsOf, b.loc, "year")...)
	return out
}

func iterateDays(from, to time.Time, loc *time.Location) []periodSpec {
	cur := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, loc)
	var out []periodSpec
	for !cur.After(end) {
		next := cur.AddDate(0, 0, 1)
		out = append(out, periodSpec{kind: "day", start: cur.UTC(), end: next.UTC(), startDate: dateUTC(cur), endDate: dateUTC(cur)})
		cur = next
	}
	return out
}

func iterateCalendar(from, to time.Time, loc *time.Location, kind string) []periodSpec {
	cur := periodStart(from, loc, kind)
	end := periodStart(to, loc, kind)
	var out []periodSpec
	for !cur.After(end) {
		next := periodNext(cur, kind)
		last := next.Add(-time.Nanosecond).In(loc)
		out = append(out, periodSpec{kind: kind, start: cur.UTC(), end: next.UTC(), startDate: dateUTC(cur.In(loc)), endDate: dateUTC(last)})
		cur = next
	}
	return out
}

func periodStart(at time.Time, loc *time.Location, kind string) time.Time {
	at = at.In(loc)
	switch kind {
	case "iso_week":
		weekday := int(at.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, loc)
		return day.AddDate(0, 0, -(weekday - 1))
	case "month":
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, loc)
	case "quarter":
		month := ((int(at.Month())-1)/3)*3 + 1
		return time.Date(at.Year(), time.Month(month), 1, 0, 0, 0, 0, loc)
	default:
		return time.Date(at.Year(), 1, 1, 0, 0, 0, 0, loc)
	}
}

func periodNext(start time.Time, kind string) time.Time {
	switch kind {
	case "iso_week":
		return start.AddDate(0, 0, 7)
	case "month":
		return start.AddDate(0, 1, 0)
	case "quarter":
		return start.AddDate(0, 3, 0)
	default:
		return start.AddDate(1, 0, 0)
	}
}

func dateUTC(at time.Time) time.Time {
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
}

func (b *builder) evalTimes() []time.Time {
	seen := map[int64]time.Time{}
	add := func(at time.Time) {
		at = at.UTC()
		seen[at.UnixMicro()] = at
	}
	add(b.asOf)
	if b.facts.baseline != nil {
		add(b.facts.baseline.EffectiveAt)
	}
	for _, flow := range b.facts.flows {
		add(flow.EffectiveAt)
	}
	for i := range b.facts.balances {
		add(b.facts.balances[i].ObservedAt)
	}
	for i := range b.facts.positions {
		add(b.facts.positions[i].ObservedAt)
	}
	for _, item := range b.facts.equities {
		add(item.ObservedAt)
	}
	for _, spec := range b.periodSpecs() {
		add(spec.start)
		if spec.end.After(b.asOf) {
			add(b.asOf)
		} else {
			add(spec.end)
		}
	}
	out := make([]time.Time, 0, len(seen))
	for _, at := range seen {
		out = append(out, at)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (b *builder) firstInstant() time.Time {
	first := b.asOf
	consider := func(at time.Time) {
		if at.Before(first) {
			first = at
		}
	}
	if b.facts.baseline != nil {
		consider(b.facts.baseline.EffectiveAt)
	}
	for i := range b.facts.flows {
		consider(b.facts.flows[i].EffectiveAt)
	}
	for i := range b.facts.balances {
		consider(b.facts.balances[i].ObservedAt)
	}
	for i := range b.facts.equities {
		consider(b.facts.equities[i].ObservedAt)
	}
	return first
}

func (b *builder) freshness(identity, account uuid.UUID, scope string, observed, cutoff time.Time, completeness string) (string, string) {
	fresh := "fresh"
	if seconds, ok := b.facts.connections[identity]; ok && seconds > 0 && cutoff.Sub(observed) > time.Duration(seconds)*time.Second {
		fresh = "stale"
		b.addIssue("latest_"+scope, identity.String(), "warning", "stale_scope", "The carried-forward Observation exceeded the freshness threshold.", cutoff, &identity, optionalUUID(account), nil)
	}
	for i := range b.facts.failures {
		fail := &b.facts.failures[i]
		if fail.IdentityID != identity || fail.OccurredAt.After(cutoff) || fail.OccurredAt.Before(observed) {
			continue
		}
		if fail.AccountID != nil && account != uuid.Nil && *fail.AccountID != account {
			continue
		}
		if fail.ScopeKind != "" && fail.ScopeKind != scope && fail.ScopeKind != "identity" {
			continue
		}
		fresh = "stale"
		b.addIssue("latest_"+scope, identity.String(), "warning", "collection_failure", fail.SafeDetail, cutoff, &identity, fail.AccountID, nil)
	}
	complete := completeness
	if complete == "" {
		complete = "complete"
	}
	if complete == "complete" && fresh == "stale" {
		complete = "complete"
	}
	return fresh, complete
}

func optionalUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func (b *builder) mapListing(listing uuid.UUID, at time.Time) (bool, uuid.UUID) {
	mapped, assetID := b.mapListingPtr(&listing)
	if !mapped {
		b.addIssue("latest_balance", listing.String(), "warning", "missing_mapping", "An Asset Listing has no current mapping.", at, nil, nil, nil)
		return false, uuid.Nil
	}
	if b.facts.mappings[listing].State == "provisional" {
		b.addIssue("latest_balance", listing.String(), "info", "provisional_asset_mapping", "The listing is mapped to a provisional Asset.", at, nil, nil, nil)
	}
	return true, assetID
}

func (b *builder) mapListingPtr(listing *uuid.UUID) (bool, uuid.UUID) {
	if listing == nil {
		return false, uuid.Nil
	}
	mapped, ok := b.facts.mappings[*listing]
	if !ok {
		return false, uuid.Nil
	}
	return true, mapped.AssetID
}

func (b *builder) assetOfListing(listing *uuid.UUID) uuid.UUID {
	ok, id := b.mapListingPtr(listing)
	if !ok {
		return uuid.Nil
	}
	return id
}

func (b *builder) listingOf(asset, platform uuid.UUID) uuid.UUID {
	for listingID, mapping := range b.facts.mappings {
		if mapping.AssetID != asset {
			continue
		}
		if listing, ok := b.facts.listings[listingID]; ok && (platform == uuid.Nil || listing.PlatformID == platform) {
			return listingID
		}
	}
	return uuid.Nil
}

func (b *builder) platformOf(identity uuid.UUID) uuid.UUID {
	for _, item := range b.facts.identities {
		if item.ID == identity {
			return item.PlatformID
		}
	}
	return uuid.Nil
}

func (b *builder) platformForAsset(asset uuid.UUID) uuid.UUID {
	listing := b.listingOf(asset, uuid.Nil)
	if listing == uuid.Nil {
		return uuid.Nil
	}
	return b.facts.listings[listing].PlatformID
}

func (b *builder) hasState(identity uuid.UUID, cutoff time.Time) bool {
	return b.hasSuccessfulSnapshot(identity, cutoff)
}

func (b *builder) addIssue(kind, key, severity, code, summary string, at time.Time, identity, account, observation *uuid.UUID) {
	for i := range b.issues {
		if b.issues[i].Kind == kind && b.issues[i].Key == key && b.issues[i].Code == code {
			if at.Before(b.issues[i].First) {
				b.issues[i].First = at
			}
			if at.After(b.issues[i].Last) {
				b.issues[i].Last = at
			}
			return
		}
	}
	b.issues = append(b.issues, issueRow{Kind: kind, Key: key, Severity: severity, Code: code, Summary: summary, IdentityID: identity, AccountID: account, ObservationID: observation, First: at, Last: at})
}

func coverageFingerprint(set map[uuid.UUID]bool, kind string) string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, kind+":"+id.String())
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func hashFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func fingerprintFromContribs(rows []contributionRow, identities []identityFact, cutoff time.Time, b *builder) string {
	parts := make([]string, 0, len(rows)+len(identities))
	for i := range rows {
		parts = append(parts, rows[i].Kind+":"+rows[i].Fingerprint)
	}
	for _, ident := range identities {
		if !b.hasState(ident.ID, cutoff) {
			parts = append(parts, "missing:"+ident.ID.String())
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}
