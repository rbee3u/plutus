package financial

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/ordering"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Portfolio exposure sums each selected account's signed coin exposure across
// every platform, by canonical asset. It is a display derivation over retained
// report facts; it never changes valuation, totals, or account-level exposure.
const (
	// ExposureComplete means every contribution to the asset is known.
	ExposureComplete = "complete"
	// ExposurePartial means the row is a known sum with at least one unknown contribution.
	ExposurePartial = "partial"
	// ExposureUnknown means no contribution to the asset is known.
	ExposureUnknown = "unknown"

	// ExposureSortValue orders rows by absolute reference value, unknown last.
	ExposureSortValue = "value"
	// ExposureSortSymbol orders rows by canonical asset symbol.
	ExposureSortSymbol = "symbol"
)

// ExposureSummary counts the filtered exposure rows before pagination and
// reports their signed USD composition.
type ExposureSummary struct {
	AssetCount   int
	LongCount    int
	ShortCount   int
	FlatCount    int
	PartialCount int
	UnknownCount int
	// StablecoinValueUSD is the known stablecoin row value sum; nil means no
	// known stablecoin value exists, so zero is not proven.
	StablecoinValueUSD *string
	// NonStablecoinValueUSD is the known non-stablecoin row value sum.
	NonStablecoinValueUSD *string
	// TotalValueUSD is the known row value sum. It may stay a known partial sum
	// while one class is unknown.
	TotalValueUSD *string
	// UnpricedAssetCount counts filtered rows without a USD value.
	UnpricedAssetCount int
	// UnpricedAssetSymbols names those rows in canonical symbol order.
	UnpricedAssetSymbols []string
	// StablecoinSymbols names classified rows present in the filtered scope.
	StablecoinSymbols []string
	Completeness      string
	Freshness         string
}

// ExposureCoverage qualifies the account set behind the exposure read.
type ExposureCoverage struct {
	Complete                  bool
	MissingAccountIDs         []uuid.UUID
	MissingAccountCount       int
	StaleAccountCount         int
	ExcludedAssetCount        int
	UnattributedPositionCount int
}

// ExposureRow is one canonical asset's portfolio-level signed quantity.
type ExposureRow struct {
	CanonicalAssetSymbol string
	PlatformSymbols      []string
	NetQuantity          *string
	State                string
	PriceUSD             *string
	PriceSource          string
	PriceSourceSymbol    *string
	PriceSourcePlatform  string
	ValueUSD             *string
	AccountCount         int
	UnknownAccountCount  int
	Freshness            string
	Issues               []valuation.Issue
	valueRat             *big.Rat
}

// ExposureList is one page of portfolio exposure rows.
type ExposureList struct {
	Context  Context
	Summary  ExposureSummary
	Coverage ExposureCoverage
	Items    []ExposureRow
	Offset   int
	Limit    int
	Total    int
	Issues   []valuation.Issue
}

// ExposureDetail lists the account contributions behind one canonical asset.
type ExposureDetail struct {
	Context              Context
	CanonicalAssetSymbol string
	PlatformSymbols      []string
	NetQuantity          *string
	State                string
	AccountCount         int
	UnknownAccountCount  int
	Freshness            string
	Coverage             ExposureCoverage
	Contributions        []ExposureContribution
	Issues               []valuation.Issue
}

// ExposureContribution is one account's known or unknown contribution.
type ExposureContribution struct {
	Platform         string
	PlatformLabel    string
	IdentityID       uuid.UUID
	IdentityName     string
	AccountID        uuid.UUID
	AccountName      string
	AccountType      string
	AccountStructure string
	AssetSymbol      string
	Quantity         *string
	State            string
	ContractOnly     bool
	Freshness        string
	ReportObservedAt *time.Time
	Issues           []valuation.Issue
}

type exposureAsset struct {
	symbol    string
	canonical string
}

type exposureContribution struct {
	account             *valuation.AccountValuation
	canonical           string
	symbols             map[string]bool
	spot                *big.Rat
	contract            *big.Rat
	hasBalance          bool
	hasPosition         bool
	unknown             bool
	unknownDetail       string
	quantity            *big.Rat
	contractOnly        bool
	price               *big.Rat
	priceSource         string
	priceSourceSymbol   string
	priceSourcePlatform string
	priceWeight         *big.Rat
	issues              []valuation.Issue
}

type exposureGroup struct {
	canonical     string
	contributions []*exposureContribution
}

type exposureFacts struct {
	context            Context
	point              *valuation.Point
	accounts           []valuation.AccountValuation
	groups             map[string]*exposureGroup
	missingAccountIDs  []uuid.UUID
	missingIssues      []valuation.Issue
	unattributedIssues []valuation.Issue
	unattributed       int
	excluded           map[uuid.UUID]exposureAsset
}

// Exposure lists portfolio-level exposure for the selected point.
func (s *Service) Exposure(ctx context.Context, p PortfolioContext, f *ListFilter) (ExposureList, error) {
	if f == nil {
		f = &ListFilter{}
	}
	limit, offset, err := normalizePage(f.Limit, f.Offset)
	if err != nil {
		return ExposureList{}, err
	}
	sort := f.Sort
	if sort == "" {
		sort = ExposureSortValue
	}
	if sort != ExposureSortValue && sort != ExposureSortSymbol {
		return ExposureList{}, ErrInvalid
	}
	filter := *f
	filter.Limit, filter.Offset = limit, offset
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (ExposureList, error) {
		facts, err := s.loadExposureFacts(ctx, p, &filter)
		if err != nil {
			return ExposureList{}, err
		}
		rows := facts.rows()
		if filter.Symbol != "" {
			rows = slices.DeleteFunc(rows, func(row ExposureRow) bool {
				return !matchesSymbol(filter.Symbol, append([]string{row.CanonicalAssetSymbol}, row.PlatformSymbols...)...)
			})
		}
		sortExposureRows(rows, sort)
		summary := summarizeExposure(rows, facts.coverageComplete(), facts.unattributed, facts.point != nil)
		coverage := facts.coverageForRows(rows, facts.excludedCount(filter.Symbol))
		issues := facts.listIssues(rows)
		if facts.point == nil {
			issues = append(issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
		}
		out := ExposureList{
			Context:  facts.context,
			Summary:  summary,
			Coverage: coverage,
			Items:    []ExposureRow{},
			Offset:   offset,
			Limit:    limit,
			Total:    len(rows),
			Issues:   issues,
		}
		if offset < len(rows) {
			out.Items = rows[offset:min(offset+limit, len(rows))]
		}
		return out, nil
	})
}

// ExposureDetail lists the account contributions behind one canonical asset.
func (s *Service) ExposureDetail(ctx context.Context, p PortfolioContext, f *ListFilter, symbol string) (ExposureDetail, error) {
	if f == nil {
		f = &ListFilter{}
	}
	normalized, err := catalog.NormalizeSymbol(symbol)
	if err != nil {
		return ExposureDetail{}, valuation.ErrNotFound
	}
	filter := *f
	filter.Symbol = ""
	filter.Sort = ""
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (ExposureDetail, error) {
		facts, err := s.loadExposureFacts(ctx, p, &filter)
		if err != nil {
			return ExposureDetail{}, err
		}
		group := facts.groups[normalized]
		if group == nil {
			return ExposureDetail{}, valuation.ErrNotFound
		}
		row := exposureRow(group)
		return ExposureDetail{
			Context:              facts.context,
			CanonicalAssetSymbol: row.CanonicalAssetSymbol,
			PlatformSymbols:      row.PlatformSymbols,
			NetQuantity:          row.NetQuantity,
			State:                row.State,
			AccountCount:         row.AccountCount,
			UnknownAccountCount:  row.UnknownAccountCount,
			Freshness:            row.Freshness,
			Coverage:             facts.coverageForGroup(group),
			Contributions:        facts.contributionsFor(group),
			Issues:               facts.detailIssues(group),
		}, nil
	})
}

func (s *Service) loadExposureFacts(ctx context.Context, p PortfolioContext, f *ListFilter) (*exposureFacts, error) {
	c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
	if err != nil {
		return nil, err
	}
	coverage := reportCoverage(accounts, f, "", point != nil)
	facts := &exposureFacts{
		context:           c,
		point:             point,
		accounts:          accounts,
		groups:            map[string]*exposureGroup{},
		missingAccountIDs: coverage.MissingAccountIDs,
		excluded:          map[uuid.UUID]exposureAsset{},
	}
	byID := indexAccounts(accounts)
	for _, id := range coverage.MissingAccountIDs {
		account := byID[id]
		issue := valuation.Issue{Code: "missing", AccountID: &id, Detail: "No compatible successful account report supplies holdings for the filtered exposure total."}
		if account != nil {
			issue.AccountName = account.AccountName
		}
		facts.missingIssues = append(facts.missingIssues, issue)
	}
	if point == nil {
		return facts, nil
	}

	q := mysql.Queries(ctx, s.db)
	rows, err := q.ListBalancesByCycle(ctx, sqlcgen.ListBalancesByCycleParams{
		PortfolioID: p.ID[:],
		CycleID:     point.CycleID[:],
		Platform:    f.Platform,
		IdentityID:  optionalFilterID(f.IdentityID),
		Symbol:      "",
	})
	if err != nil {
		return nil, err
	}
	positionsByAccount, err := s.riskPositionsByCycle(ctx, p, point.CycleID, byID)
	if err != nil {
		return nil, err
	}
	filteredAccounts := make([]valuation.AccountValuation, 0, len(accounts))
	decisions := make(map[uuid.UUID]map[uuid.UUID]*valuation.BalanceValuation, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !matchesAccount(f, account) {
			continue
		}
		filteredAccounts = append(filteredAccounts, *account)
		decisions[account.AccountID] = balanceDecisions(account)
	}
	book := valuation.NewPriceBook(q, p.ID)
	if err := book.PreloadObservations(ctx, filteredAccounts); err != nil {
		return nil, err
	}

	assets := map[string]*sqlcgen.Asset{}
	assetByKey := func(platform, symbol string) (*sqlcgen.Asset, error) {
		key := platform + "\x00" + symbol
		if stored, ok := assets[key]; ok {
			return stored, nil
		}
		stored, readErr := q.GetAssetByPlatformSymbol(ctx, sqlcgen.GetAssetByPlatformSymbolParams{
			PortfolioID: p.ID[:],
			Platform:    platform,
			Symbol:      symbol,
		})
		if errors.Is(readErr, sql.ErrNoRows) {
			assets[key] = nil
			return nil, nil
		}
		if readErr != nil {
			return nil, readErr
		}
		assets[key] = &stored
		return &stored, nil
	}
	contributions := map[uuid.UUID]map[string]*exposureContribution{}
	contribution := func(account *valuation.AccountValuation, canonical string) *exposureContribution {
		byCanonical := contributions[account.AccountID]
		if byCanonical == nil {
			byCanonical = map[string]*exposureContribution{}
			contributions[account.AccountID] = byCanonical
		}
		c := byCanonical[canonical]
		if c == nil {
			c = &exposureContribution{account: account, canonical: canonical, symbols: map[string]bool{}, spot: decimal.Zero(), contract: decimal.Zero()}
			byCanonical[canonical] = c
		}
		return c
	}
	countExcluded := func(assetID uuid.UUID, symbol, canonical string) {
		if _, ok := facts.excluded[assetID]; ok {
			return
		}
		facts.excluded[assetID] = exposureAsset{symbol: symbol, canonical: canonical}
	}

	for i := range rows {
		raw := &rows[i]
		accountID, err := uuid.FromBytes(raw.AccountID)
		if err != nil {
			return nil, err
		}
		account := byID[accountID]
		if account == nil || !matchesAccount(f, account) {
			continue
		}
		assetID, err := uuid.FromBytes(raw.AssetID)
		if err != nil {
			return nil, err
		}
		decision := decisions[accountID][assetID]
		if isExcluded(decision) {
			countExcluded(assetID, raw.AssetSymbol, raw.CanonicalAssetSymbol)
			continue
		}
		row, err := balanceRow(ctx, book, account, &sqlcgen.ListBalancesByReportRow{
			AssetID:              raw.AssetID,
			Platform:             raw.Platform,
			AssetSymbol:          raw.AssetSymbol,
			CanonicalAssetSymbol: raw.CanonicalAssetSymbol,
			Equity:               raw.Equity,
		}, decision)
		if err != nil {
			return nil, err
		}
		if row.Excluded {
			countExcluded(assetID, raw.AssetSymbol, row.CanonicalAssetSymbol)
			continue
		}
		canonical := row.CanonicalAssetSymbol
		if canonical == "" {
			canonical = row.AssetSymbol
		}
		c := contribution(account, canonical)
		c.hasBalance = true
		c.symbols[row.AssetSymbol] = true
		equity, ok := decimal.Parse(row.Equity)
		if !ok {
			c.unknown = true
			c.unknownDetail = "a balance quantity is invalid"
			continue
		}
		c.spot = decimal.Add(c.spot, equity)
		if row.PriceUSD != nil {
			if price, ok := decimal.Parse(*row.PriceUSD); ok && price.Sign() > 0 {
				c.considerPrice(price, "spot", row.AssetSymbol, account.Platform, decimal.Abs(equity))
			}
		}
	}

	for i := range accounts {
		account := &accounts[i]
		if !matchesAccount(f, account) {
			continue
		}
		positions := positionsByAccount[account.AccountID]
		if len(positions) == 0 {
			continue
		}
		for j := range positions {
			position := &positions[j]
			native, ok := riskSymbol(position.BaseAssetSymbol)
			if !ok {
				facts.addUnattributed(account, position, "The position has no usable base asset.")
				continue
			}
			stored, err := assetByKey(account.Platform, native)
			if err != nil {
				return nil, err
			}
			if stored == nil {
				facts.addUnattributed(account, position, "The position's base asset is not mapped in the catalog.")
				continue
			}
			storedID, err := uuid.FromBytes(stored.ID)
			if err != nil {
				return nil, err
			}
			if isExcluded(decisions[account.AccountID][storedID]) {
				countExcluded(storedID, stored.Symbol, stored.CanonicalAssetSymbol)
			}
		}
		grouped := riskAssets(positions)
		nativeSymbols := make([]string, 0, len(grouped))
		for native := range grouped {
			nativeSymbols = append(nativeSymbols, native)
		}
		slices.Sort(nativeSymbols)
		for _, native := range nativeSymbols {
			fund := grouped[native]
			stored, err := assetByKey(account.Platform, native)
			if err != nil {
				return nil, err
			}
			if stored == nil {
				continue
			}
			storedID, err := uuid.FromBytes(stored.ID)
			if err != nil {
				return nil, err
			}
			if isExcluded(decisions[account.AccountID][storedID]) {
				continue
			}
			canonical := stored.CanonicalAssetSymbol
			if canonical == "" {
				canonical = stored.Symbol
			}
			c := contribution(account, canonical)
			c.hasPosition = true
			c.symbols[native] = true
			if fund.unknown {
				c.unknown = true
				if c.unknownDetail == "" {
					c.unknownDetail = "a position size, side, or inverse mark price is unavailable"
				}
				continue
			}
			c.contract = decimal.Add(c.contract, fund.contract)
			if mark, symbol := riskPrice(fund); mark != nil {
				c.considerPrice(mark, "contract", symbol, account.Platform, riskPriceWeight(fund))
			}
		}
	}

	add := func(canonical string, c *exposureContribution) {
		if c.unknown {
			c.quantity = nil
			c.issues = append(c.issues, valuation.Issue{
				Code:             "exposure_unknown_position",
				AccountID:        &c.account.AccountID,
				AccountName:      c.account.AccountName,
				Symbol:           canonical,
				Detail:           "The account's " + canonical + " exposure is unknown because " + c.unknownDetail + ".",
				ReportObservedAt: c.account.ReportObservedAt,
			})
		} else {
			c.quantity = decimal.Add(c.spot, c.contract)
			c.contractOnly = c.hasPosition && (c.spot == nil || c.spot.Sign() == 0)
		}
		group := facts.groups[canonical]
		if group == nil {
			group = &exposureGroup{canonical: canonical}
			facts.groups[canonical] = group
		}
		group.contributions = append(group.contributions, c)
	}
	type zeroContribution struct {
		canonical string
		c         *exposureContribution
	}
	var zeroOnly []zeroContribution
	for _, byCanonical := range contributions {
		for canonical, c := range byCanonical {
			if c.unknown || c.hasPosition || (c.spot != nil && c.spot.Sign() != 0) {
				add(canonical, c)
				continue
			}
			// A known zero balance joins an existing row for transparency, but
			// does not create a zero-only asset row.
			if c.hasBalance {
				zeroOnly = append(zeroOnly, zeroContribution{canonical: canonical, c: c})
			}
		}
	}
	for _, item := range zeroOnly {
		if facts.groups[item.canonical] == nil {
			continue
		}
		add(item.canonical, item.c)
	}
	for _, group := range facts.groups {
		slices.SortStableFunc(group.contributions, func(a, b *exposureContribution) int {
			return ordering.CompareAccounts(accountOrder(a.account), accountOrder(b.account))
		})
	}
	return facts, nil
}

func (c *exposureContribution) considerPrice(price *big.Rat, source, symbol, platform string, weight *big.Rat) {
	if price == nil || price.Sign() <= 0 {
		return
	}
	if c.price != nil && c.priceWeight != nil && weight != nil && weight.Cmp(c.priceWeight) <= 0 {
		return
	}
	c.price = price
	c.priceSource = source
	c.priceSourceSymbol = symbol
	c.priceSourcePlatform = platform
	c.priceWeight = weight
}

func riskPriceWeight(asset *riskAsset) *big.Rat {
	var best *big.Rat
	for i := range asset.positions {
		position := &asset.positions[i]
		if position.mark == nil || position.notional == nil || !valuation.USDQuote(position.quote) {
			continue
		}
		notional := decimal.Abs(position.notional)
		if best == nil || notional.Cmp(best) > 0 {
			best = notional
		}
	}
	return best
}

func (f *exposureFacts) addUnattributed(account *valuation.AccountValuation, position *PositionRow, detail string) {
	f.unattributed++
	f.unattributedIssues = append(f.unattributedIssues, valuation.Issue{
		Code:             "exposure_unattributed_position",
		AccountID:        &account.AccountID,
		AccountName:      account.AccountName,
		Symbol:           position.FuturesSymbol,
		Detail:           detail + " Its exposure is omitted from the portfolio total.",
		ReportObservedAt: account.ReportObservedAt,
	})
}

func (f *exposureFacts) rows() []ExposureRow {
	out := make([]ExposureRow, 0, len(f.groups))
	for _, group := range f.groups {
		out = append(out, exposureRow(group))
	}
	return out
}

func exposureRow(group *exposureGroup) ExposureRow {
	row := ExposureRow{
		CanonicalAssetSymbol: group.canonical,
		PlatformSymbols:      []string{},
		State:                ExposureComplete,
		Freshness:            valuation.FreshnessFresh,
		Issues:               []valuation.Issue{},
		AccountCount:         len(group.contributions),
	}
	symbols := map[string]bool{}
	known := decimal.Zero()
	knownCount, unknownCount := 0, 0
	for _, c := range group.contributions {
		for symbol := range c.symbols {
			symbols[symbol] = true
		}
		if c.account.Freshness == valuation.FreshnessStale {
			row.Freshness = valuation.FreshnessStale
		}
		if c.unknown {
			unknownCount++
			row.Issues = append(row.Issues, c.issues...)
			continue
		}
		knownCount++
		known = decimal.Add(known, c.quantity)
	}
	row.UnknownAccountCount = unknownCount
	switch {
	case unknownCount == 0:
		row.State = ExposureComplete
	case knownCount > 0:
		row.State = ExposurePartial
	default:
		row.State = ExposureUnknown
	}
	var net *big.Rat
	if row.State != ExposureUnknown {
		net = known
		row.NetQuantity = decimal.FormatPtr(net)
	}
	row.PlatformSymbols = sortedStrings(symbols)
	row.Issues = dedupeIssues(row.Issues)

	price, source, symbol, platform := referenceExposurePrice(group.contributions)
	row.PriceUSD = decimal.FormatPtr(price)
	row.PriceSource = source
	if symbol != "" {
		row.PriceSourceSymbol = &symbol
	}
	row.PriceSourcePlatform = platform
	var value *big.Rat
	switch {
	case net == nil:
	case net.Sign() == 0:
		value = decimal.Zero()
	case price != nil:
		value = decimal.Mul(net, price)
	}
	row.ValueUSD = decimal.FormatPtr(value)
	row.valueRat = value
	return row
}

func referenceExposurePrice(contributions []*exposureContribution) (*big.Rat, string, string, string) {
	type candidate struct {
		contribution *exposureContribution
		quantity     *big.Rat
	}
	candidates := make([]candidate, 0, len(contributions))
	for _, c := range contributions {
		if c.unknown || c.quantity == nil || c.quantity.Sign() == 0 || c.price == nil {
			continue
		}
		candidates = append(candidates, candidate{contribution: c, quantity: decimal.Abs(c.quantity)})
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if c := a.quantity.Cmp(b.quantity); c != 0 {
			return -c
		}
		return ordering.CompareAccounts(accountOrder(a.contribution.account), accountOrder(b.contribution.account))
	})
	if len(candidates) > 0 {
		c := candidates[0].contribution
		return c.price, c.priceSource, c.priceSourceSymbol, c.priceSourcePlatform
	}
	// A known zero has no reference quantity but may still show its price.
	ordered := slices.Clone(contributions)
	slices.SortStableFunc(ordered, func(a, b *exposureContribution) int {
		return ordering.CompareAccounts(accountOrder(a.account), accountOrder(b.account))
	})
	for _, c := range ordered {
		if c.price != nil {
			return c.price, c.priceSource, c.priceSourceSymbol, c.priceSourcePlatform
		}
	}
	return nil, "", "", ""
}

func sortExposureRows(rows []ExposureRow, sort string) {
	slices.SortStableFunc(rows, func(a, b ExposureRow) int {
		if sort == ExposureSortSymbol {
			return strings.Compare(a.CanonicalAssetSymbol, b.CanonicalAssetSymbol)
		}
		av, bv := a.valueRat, b.valueRat
		switch {
		case av == nil && bv == nil:
		case av == nil:
			return 1
		case bv == nil:
			return -1
		default:
			if c := decimal.Abs(av).Cmp(decimal.Abs(bv)); c != 0 {
				return -c
			}
		}
		return strings.Compare(a.CanonicalAssetSymbol, b.CanonicalAssetSymbol)
	})
}

func summarizeExposure(rows []ExposureRow, coverageComplete bool, unattributed int, hasPoint bool) ExposureSummary {
	summary := ExposureSummary{
		AssetCount:   len(rows),
		Completeness: valuation.CompletenessComplete,
		Freshness:    valuation.FreshnessFresh,
	}
	stableSum, nonStableSum := decimal.Zero(), decimal.Zero()
	stableKnown, nonStableKnown := 0, 0
	stableRows, nonStableRows := 0, 0
	for i := range rows {
		row := &rows[i]
		if row.Freshness == valuation.FreshnessStale {
			summary.Freshness = valuation.FreshnessStale
		}
		isStable := valuation.Stablecoin(row.CanonicalAssetSymbol)
		if isStable {
			summary.StablecoinSymbols = append(summary.StablecoinSymbols, row.CanonicalAssetSymbol)
			stableRows++
		} else {
			nonStableRows++
		}
		if row.valueRat == nil {
			summary.UnpricedAssetCount++
			summary.UnpricedAssetSymbols = append(summary.UnpricedAssetSymbols, row.CanonicalAssetSymbol)
		} else if isStable {
			stableSum = decimal.Add(stableSum, row.valueRat)
			stableKnown++
		} else {
			nonStableSum = decimal.Add(nonStableSum, row.valueRat)
			nonStableKnown++
		}
		switch row.State {
		case ExposurePartial:
			summary.PartialCount++
		case ExposureUnknown:
			summary.UnknownCount++
		default:
			quantity, _ := decimal.Parse(optionalSymbol(row.NetQuantity))
			switch {
			case quantity == nil || quantity.Sign() == 0:
				summary.FlatCount++
			case quantity.Sign() > 0:
				summary.LongCount++
			default:
				summary.ShortCount++
			}
		}
	}
	slices.Sort(summary.StablecoinSymbols)
	slices.Sort(summary.UnpricedAssetSymbols)
	summary.StablecoinValueUSD = exposureClassValue(stableSum, stableKnown, stableRows, coverageComplete)
	summary.NonStablecoinValueUSD = exposureClassValue(nonStableSum, nonStableKnown, nonStableRows, coverageComplete)
	summary.TotalValueUSD = exposureClassValue(decimal.Add(stableSum, nonStableSum), stableKnown+nonStableKnown, len(rows), coverageComplete)
	if !hasPoint || !coverageComplete || unattributed > 0 || summary.PartialCount > 0 || summary.UnknownCount > 0 || summary.UnpricedAssetCount > 0 {
		summary.Completeness = valuation.CompletenessIncomplete
	}
	return summary
}

// exposureClassValue keeps unknown distinct from zero: known values sum, an
// empty class is zero only when coverage is complete, and anything else is
// unknown.
func exposureClassValue(sum *big.Rat, known, rows int, coverageComplete bool) *string {
	switch {
	case known > 0:
		return decimal.FormatPtr(sum)
	case rows == 0 && coverageComplete:
		return decimal.FormatPtr(decimal.Zero())
	default:
		return nil
	}
}

func (f *exposureFacts) coverageComplete() bool {
	return f.point != nil && len(f.missingAccountIDs) == 0 && f.unattributed == 0
}

func (f *exposureFacts) excludedCount(query string) int {
	count := 0
	for _, asset := range f.excluded {
		if matchesSymbol(query, asset.canonical, asset.symbol) {
			count++
		}
	}
	return count
}

func (f *exposureFacts) coverageForRows(rows []ExposureRow, excluded int) ExposureCoverage {
	stale := map[uuid.UUID]bool{}
	for i := range rows {
		group := f.groups[rows[i].CanonicalAssetSymbol]
		if group == nil {
			continue
		}
		for _, c := range group.contributions {
			if c.account.Freshness == valuation.FreshnessStale {
				stale[c.account.AccountID] = true
			}
		}
	}
	return f.coverage(len(stale), excluded)
}

func (f *exposureFacts) coverageForGroup(group *exposureGroup) ExposureCoverage {
	stale := map[uuid.UUID]bool{}
	for _, c := range group.contributions {
		if c.account.Freshness == valuation.FreshnessStale {
			stale[c.account.AccountID] = true
		}
	}
	excluded := 0
	for _, asset := range f.excluded {
		if asset.canonical == group.canonical {
			excluded++
		}
	}
	return f.coverage(len(stale), excluded)
}

func (f *exposureFacts) coverage(stale, excluded int) ExposureCoverage {
	ids := f.missingAccountIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ExposureCoverage{
		Complete:                  f.coverageComplete(),
		MissingAccountIDs:         ids,
		MissingAccountCount:       len(ids),
		StaleAccountCount:         stale,
		ExcludedAssetCount:        excluded,
		UnattributedPositionCount: f.unattributed,
	}
}

func (f *exposureFacts) listIssues(rows []ExposureRow) []valuation.Issue {
	issues := append([]valuation.Issue{}, f.missingIssues...)
	issues = append(issues, f.unattributedIssues...)
	stale := map[uuid.UUID]bool{}
	for i := range rows {
		group := f.groups[rows[i].CanonicalAssetSymbol]
		if group == nil {
			continue
		}
		for _, c := range group.contributions {
			if c.account.Freshness == valuation.FreshnessStale && !stale[c.account.AccountID] {
				stale[c.account.AccountID] = true
				issues = append(issues, staleAccountIssue(c.account))
			}
		}
		issues = append(issues, rows[i].Issues...)
	}
	return dedupeIssues(issues)
}

func (f *exposureFacts) detailIssues(group *exposureGroup) []valuation.Issue {
	issues := append([]valuation.Issue{}, f.missingIssues...)
	issues = append(issues, f.unattributedIssues...)
	stale := map[uuid.UUID]bool{}
	for _, c := range group.contributions {
		if c.account.Freshness == valuation.FreshnessStale && !stale[c.account.AccountID] {
			stale[c.account.AccountID] = true
			issues = append(issues, staleAccountIssue(c.account))
		}
		issues = append(issues, c.issues...)
	}
	return dedupeIssues(issues)
}

func (f *exposureFacts) contributionsFor(group *exposureGroup) []ExposureContribution {
	out := make([]ExposureContribution, 0, len(group.contributions))
	for _, c := range group.contributions {
		symbols := make([]string, 0, len(c.symbols))
		for symbol := range c.symbols {
			symbols = append(symbols, symbol)
		}
		slices.Sort(symbols)
		state := ExposureComplete
		if c.unknown {
			state = ExposureUnknown
		}
		out = append(out, ExposureContribution{
			Platform:         c.account.Platform,
			PlatformLabel:    adapter.PlatformName(adapter.Platform(c.account.Platform)),
			IdentityID:       c.account.IdentityID,
			IdentityName:     c.account.IdentityName,
			AccountID:        c.account.AccountID,
			AccountName:      c.account.AccountName,
			AccountType:      c.account.AccountType,
			AccountStructure: c.account.AccountStructure,
			AssetSymbol:      strings.Join(symbols, ", "),
			Quantity:         decimal.FormatPtr(c.quantity),
			State:            state,
			ContractOnly:     c.contractOnly,
			Freshness:        c.account.Freshness,
			ReportObservedAt: c.account.ReportObservedAt,
			Issues:           c.issues,
		})
	}
	return out
}

func accountOrder(account *valuation.AccountValuation) ordering.Account {
	return ordering.Account{
		Identity:  ordering.Identity{Platform: account.Platform, Name: account.IdentityName, ID: account.IdentityID},
		Structure: adapter.AccountStructure(account.AccountStructure),
		Type:      adapter.AccountType(account.AccountType),
		ID:        account.AccountID,
	}
}

func sortedStrings(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func dedupeIssues(issues []valuation.Issue) []valuation.Issue {
	out := make([]valuation.Issue, 0, len(issues))
	seen := map[string]bool{}
	for i := range issues {
		issue := issues[i]
		key := issue.Code + "\x00" + uuidString(issue.AccountID) + "\x00" + issue.Symbol
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, issue)
	}
	return out
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
