package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/domain/listingexclusion"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

const (
	minLimit     = 1
	maxLimit     = 500
	defaultLimit = 50

	sparklineWindow = 7 * 24 * time.Hour
)

// PortfolioContext is what reads need to know about the Portfolio.
type PortfolioContext struct {
	ID       uuid.UUID
	Timezone string
}

func (p PortfolioContext) location() *time.Location {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// PointByJob returns the point written by one Collection Cycle job.
func (s *Service) PointByJob(ctx context.Context, portfolioID, jobID uuid.UUID) (Point, error) {
	return s.pointByJob(ctx, portfolioID, jobID)
}

func (s *Service) pointByJob(ctx context.Context, portfolioID, jobID uuid.UUID) (Point, error) {
	row, err := s.q.GetValuationPointByJob(ctx, sqlcgen.GetValuationPointByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNotFound
	}
	if err != nil {
		return Point{}, err
	}
	return pointFromRow(&row)
}

// Latest returns the newest point of a Portfolio.
func (s *Service) Latest(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	row, err := s.q.GetLatestValuationPoint(ctx, portfolioID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNoPoint
	}
	if err != nil {
		return Point{}, err
	}
	return pointFromRow(&row)
}

// First returns the earliest point of the Portfolio.
func (s *Service) First(ctx context.Context, portfolioID uuid.UUID) (Point, error) {
	row, err := s.q.GetFirstValuationPoint(ctx, portfolioID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNoPoint
	}
	if err != nil {
		return Point{}, err
	}
	return pointFromRow(&row)
}

// Get returns one point by ID.
func (s *Service) Get(ctx context.Context, portfolioID, pointID uuid.UUID) (Point, error) {
	row, err := s.q.GetValuationPoint(ctx, sqlcgen.GetValuationPointParams{PortfolioID: portfolioID[:], ID: pointID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return Point{}, ErrNotFound
	}
	if err != nil {
		return Point{}, err
	}
	return pointFromRow(&row)
}

// Accounts returns the stored Account Valuations of a point.
func (s *Service) Accounts(ctx context.Context, portfolioID, jobID uuid.UUID) ([]AccountValuation, error) {
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if err != nil {
		return nil, err
	}
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		item, err := accountFromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	sortAccounts(out)
	return out, nil
}

func (s *Service) latestContext(ctx context.Context, p PortfolioContext) (Context, *Point, []AccountValuation, error) {
	c := Context{PortfolioID: p.ID, Timezone: p.Timezone}
	point, err := s.Latest(ctx, p.ID)
	if errors.Is(err, ErrNoPoint) {
		return c, nil, nil, nil
	}
	if err != nil {
		return c, nil, nil, err
	}
	c.PointID, c.JobID, c.EvaluatedAt = &point.ID, &point.JobID, &point.EvaluatedAt
	accounts, err := s.Accounts(ctx, p.ID, point.JobID)
	if err != nil {
		return c, nil, nil, err
	}
	return c, &point, accounts, nil
}

// Overview answers the landing page from the latest point.
func (s *Service) Overview(ctx context.Context, p PortfolioContext) (Overview, error) {
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Context: c, Completeness: CompletenessComplete, Issues: []Issue{}, ByPlatform: []GroupTotal{}, ByIdentity: []GroupTotal{}, Sparkline: []Point{}}
	if point == nil {
		out.Completeness = CompletenessIncomplete
		out.Issues = append(out.Issues, Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	out.Point = point
	total := point.TotalUSD
	out.TotalUSD = &total
	out.Completeness = point.Completeness()
	out.AccountCount, out.StaleAccountCount, out.IncompleteAccountCount = point.AccountCount, point.StaleAccountCount, point.IncompleteAccountCount
	out.Issues = point.Issues
	out.ByPlatform = groupAccounts(accounts, func(a *AccountValuation) (string, string, string) {
		return a.Platform, adapters.PlatformName(adapters.Platform(a.Platform)), a.Platform
	})
	out.ByIdentity = groupAccounts(accounts, func(a *AccountValuation) (string, string, string) {
		return a.IdentityID.String(), a.IdentityName, a.Platform
	})
	series, err := s.q.ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: point.EvaluatedAt.Add(-sparklineWindow), ToAt: point.EvaluatedAt})
	if err != nil {
		return Overview{}, err
	}
	points, err := pointsFromRows(series)
	if err != nil {
		return Overview{}, err
	}
	out.Sparkline = sample(points, "hour", p.location())
	return out, nil
}

func groupAccounts(accounts []AccountValuation, keyFn func(*AccountValuation) (key, label, platform string)) []GroupTotal {
	order := []string{}
	groups := map[string]*GroupTotal{}
	sums := map[string]*big.Rat{}
	for i := range accounts {
		a := &accounts[i]
		key, label, platform := keyFn(a)
		g, ok := groups[key]
		if !ok {
			g = &GroupTotal{Key: key, Label: label, Platform: platform, Completeness: CompletenessComplete}
			groups[key] = g
			sums[key] = decimal.Zero()
			order = append(order, key)
		}
		g.AccountCount++
		g.RowCount++
		if a.valueRat != nil {
			sums[key] = decimal.Add(sums[key], a.valueRat)
		}
		if a.Completeness() == CompletenessIncomplete {
			g.Completeness = CompletenessIncomplete
		}
	}
	out := make([]GroupTotal, 0, len(order))
	for _, key := range order {
		g := groups[key]
		g.ValueUSD = decimal.FormatPtr(sums[key])
		out = append(out, *g)
	}
	return out
}

// Tree answers the Platform → Identity → Account view from the latest point.
func (s *Service) Tree(ctx context.Context, p PortfolioContext) (Tree, error) {
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return Tree{}, err
	}
	out := Tree{Context: c, Completeness: CompletenessIncomplete, Platforms: []TreePlatform{}, Issues: []Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	total := point.TotalUSD
	out.TotalUSD = &total
	out.Completeness = point.Completeness()
	out.Issues = point.Issues
	platformIndex := map[string]int{}
	for i := range accounts {
		a := &accounts[i]
		pi, ok := platformIndex[a.Platform]
		if !ok {
			pi = len(out.Platforms)
			platformIndex[a.Platform] = pi
			out.Platforms = append(out.Platforms, TreePlatform{Platform: a.Platform, Label: adapters.PlatformName(adapters.Platform(a.Platform)), Completeness: CompletenessComplete})
		}
		platform := &out.Platforms[pi]
		var identity *TreeIdentity
		for j := range platform.Identities {
			if platform.Identities[j].ID == a.IdentityID {
				identity = &platform.Identities[j]
				break
			}
		}
		if identity == nil {
			platform.Identities = append(platform.Identities, TreeIdentity{ID: a.IdentityID, DisplayName: a.IdentityName, Status: a.IdentityStatus, ConnectionStatus: a.ConnectionStatus, Completeness: CompletenessComplete})
			identity = &platform.Identities[len(platform.Identities)-1]
		}
		empty, err := s.confirmedEmpty(ctx, a)
		if err != nil {
			return Tree{}, err
		}
		if !empty {
			identity.Accounts = append(identity.Accounts, TreeAccount{AccountValuation: *a})
		}
	}
	for pi := range out.Platforms {
		platform := &out.Platforms[pi]
		platformSum := decimal.Zero()
		for ii := range platform.Identities {
			identity := &platform.Identities[ii]
			sum := decimal.Zero()
			for k := range identity.Accounts {
				a := &identity.Accounts[k]
				if a.valueRat != nil {
					sum = decimal.Add(sum, a.valueRat)
				}
				if a.Completeness() == CompletenessIncomplete {
					identity.Completeness = CompletenessIncomplete
					platform.Completeness = CompletenessIncomplete
				}
			}
			identity.ValueUSD = decimal.FormatPtr(sum)
			platformSum = decimal.Add(platformSum, sum)
		}
		platform.ValueUSD = decimal.FormatPtr(platformSum)
	}
	return out, nil
}

// AccountBalances returns the Balances of an Account's current report with USD values.
func (s *Service) AccountBalances(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	view, source, err := s.accountView(ctx, p, accountID)
	if err != nil {
		return AccountView{}, err
	}
	view.Balances = []BalanceRow{}
	if source == nil {
		return view, nil
	}
	ignored := map[uuid.UUID]bool{}
	if view.Account.Method == MethodCalculated {
		point, err := s.Latest(ctx, p.ID)
		if err != nil {
			return AccountView{}, err
		}
		ignored, err = listingexclusion.ActiveForJob(ctx, s.db, p.ID, point.JobID)
		if err != nil {
			return AccountView{}, err
		}
	}
	rows, err := s.q.ListBalancesByReport(ctx, source[:])
	if err != nil {
		return AccountView{}, err
	}
	book := &priceBook{s: s, cache: map[priceKey]map[string]pricePoint{}}
	at := time.Time{}
	if view.Account.ReportObservedAt != nil {
		at = *view.Account.ReportObservedAt
	}
	for i := range rows {
		listingID, err := uuid.FromBytes(rows[i].ListingID)
		if err != nil {
			return AccountView{}, err
		}
		if ignored[listingID] {
			continue
		}
		row, err := balanceRow(ctx, book, &view.Account, at, rows[i].ListingID, rows[i].Platform, rows[i].Symbol, rows[i].AssetSymbol, rows[i].Equity, rows[i].AvailableToWithdraw, rows[i].AvailableToTrade, rows[i].Liability, rows[i].Interest, rows[i].UnrealizedPnl)
		if err != nil {
			return AccountView{}, err
		}
		view.Balances = append(view.Balances, row)
	}
	return view, nil
}

// AccountPositions returns the Positions of an Account's current report with USD notionals.
func (s *Service) AccountPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return s.AccountPositionsAt(ctx, p, accountID, nil)
}

// AccountPositionsAt reads the selected point, retaining its compatible source Report.
func (s *Service) AccountPositionsAt(ctx context.Context, p PortfolioContext, accountID uuid.UUID, pointID *uuid.UUID) (AccountView, error) {
	view, source, err := s.positionView(ctx, p, accountID, pointID)
	if err != nil {
		return AccountView{}, err
	}
	view.Positions = []PositionRow{}
	view.Issues = positionAccountIssues(&view.Account)
	if source == nil {
		return view, nil
	}
	rows, err := s.q.ListPositionsByReport(ctx, source[:])
	if err != nil {
		return AccountView{}, err
	}
	for i := range rows {
		r := &rows[i]
		row, err := positionRow(&view.Account, r.InstrumentID, r.Platform, r.Symbol, r.Kind, r.BaseSymbol, r.QuoteSymbol, r.SettlementSymbol, r.ExpiryAt, r.ContractSize, r.PlatformPositionRef, r.Side, r.Quantity, r.QuantityUnit, r.EntryPrice, r.MarkPrice, r.LiquidationPrice, r.UnrealizedPnl, r.Leverage, r.Margin)
		if err != nil {
			return AccountView{}, err
		}
		completePosition(&row, &view.Account, positionMetadata{r.PositionMode, r.MarginMode, r.MarginCurrency, r.MarginBasis})
		view.Issues = append(view.Issues, positionDataIssues(row.Issues)...)
		view.Positions = append(view.Positions, row)
	}
	sortPositions(view.Positions, nil)
	return view, nil
}

func (s *Service) accountView(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, *uuid.UUID, error) {
	account, err := s.q.GetAccount(ctx, sqlcgen.GetAccountParams{PortfolioID: p.ID[:], ID: accountID[:]})
	if errors.Is(err, sql.ErrNoRows) {
		return AccountView{}, nil, ErrNotFound
	}
	if err != nil {
		return AccountView{}, nil, err
	}
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return AccountView{}, nil, err
	}
	view := AccountView{Context: c, Issues: []Issue{}}
	if point != nil {
		for i := range accounts {
			if accounts[i].AccountID == accountID {
				view.Account = accounts[i]
				view.Issues = accountIssues(&view.Account)
				return view, view.Account.SourceReportID, nil
			}
		}
	}
	identityID, err := uuid.FromBytes(account.IdentityID)
	if err != nil {
		return AccountView{}, nil, err
	}
	view.Account = AccountValuation{
		AccountID: accountID, AccountName: account.DisplayName, AccountType: account.AccountType, Selected: account.Selected,
		IdentityID: identityID, IdentityName: account.IdentityName, IdentityStatus: account.IdentityStatus, Platform: account.Platform,
		accountStructure: account.AccountStructure,
		Status:           "failed", Method: MethodMissing, Freshness: FreshnessStale, Omitted: nil,
	}
	view.Issues = append(view.Issues, Issue{Code: "outside_boundary", AccountID: &accountID, AccountName: account.DisplayName, Detail: "The account is not part of the latest valuation point; the last successful report is shown without a value."})
	src, err := s.q.GetLatestOkReportForAccount(ctx, sqlcgen.GetLatestOkReportForAccountParams{PortfolioID: p.ID[:], AccountID: accountID[:], ScopeVersion: adapters.ReportScope(adapters.Platform(account.Platform), adapters.AccountType(account.AccountType)), Before: s.now().UTC().Add(time.Hour)})
	if errors.Is(err, sql.ErrNoRows) {
		return view, nil, nil
	}
	if err != nil {
		return AccountView{}, nil, err
	}
	sourceID, err := uuid.FromBytes(src.ID)
	if err != nil {
		return AccountView{}, nil, err
	}
	observed := src.ObservedAt.Time
	view.Account.Status = "ok"
	view.Account.SourceReportID = &sourceID
	view.Account.ReportObservedAt = &observed
	view.Account.sourceMarginMode = nullStr(src.MarginMode)
	view.Account.Equity, view.Account.AdjustedEquity, view.Account.AvailableEquity = nullDecimal(src.Equity), nullDecimal(src.AdjustedEquity), nullDecimal(src.AvailableEquity)
	view.Account.InitialMarginRatio, view.Account.MaintenanceMarginRatio, view.Account.UnifiedMarginRatio = nullDecimal(src.InitialMarginRatio), nullDecimal(src.MaintenanceMarginRatio), nullDecimal(src.UnifiedMarginRatio)
	return view, &sourceID, nil
}

func balanceRow(ctx context.Context, book *priceBook, account *AccountValuation, at time.Time, listingID []byte, platform, symbol, assetSymbol, equity string, availableToWithdraw, availableToTrade, liability, interest, unrealized sql.NullString) (BalanceRow, error) {
	id, err := uuid.FromBytes(listingID)
	if err != nil {
		return BalanceRow{}, err
	}
	row := BalanceRow{
		AccountID: account.AccountID, AccountName: account.AccountName, AccountType: account.AccountType,
		IdentityID: account.IdentityID, IdentityName: account.IdentityName, Platform: platform,
		ListingID: id, Symbol: symbol, AssetSymbol: assetSymbol, Equity: decimal.Compact(equity),
		AvailableToWithdraw: nullDecimal(availableToWithdraw), AvailableToTrade: nullDecimal(availableToTrade),
		Liability: nullDecimal(liability), Interest: nullDecimal(interest), UnrealizedPnl: nullDecimal(unrealized),
		Freshness: account.Freshness, ReportObservedAt: account.ReportObservedAt, ManualModifiedAt: account.ManualModifiedAt,
	}
	quantity, ok := decimal.Parse(equity)
	if !ok {
		row.NoPrice = true
		return row, nil
	}
	lookup := book.lookup
	if account.AccountType == string(adapters.TypeManual) {
		lookup = book.lookupPlatform
	}
	price, found, err := lookup(ctx, platform, assetSymbol, at)
	if err != nil {
		return BalanceRow{}, err
	}
	if !found {
		row.NoPrice = quantity.Sign() != 0
		return row, nil
	}
	row.PriceUSD = decimal.FormatPtr(price)
	row.equityUSDRat = decimal.Mul(quantity, price)
	row.EquityUSD = decimal.FormatPtr(row.equityUSDRat)
	return row, nil
}

func positionRow(account *AccountValuation, instrumentID []byte, platform, symbol, kind string, base, quote, settlement sql.NullString, expiry sql.NullTime, contractSize sql.NullString, ref, side, quantity, unit string, entry, mark, liq, upnl, leverage, margin sql.NullString) (PositionRow, error) {
	id, err := uuid.FromBytes(instrumentID)
	if err != nil {
		return PositionRow{}, err
	}
	row := PositionRow{
		AccountID: account.AccountID, AccountName: account.AccountName, AccountType: account.AccountType,
		IdentityID: account.IdentityID, IdentityName: account.IdentityName, Platform: platform,
		PlatformPositionRef: ref, InstrumentID: id, Symbol: symbol, Kind: kind,
		BaseSymbol: nullStr(base), QuoteSymbol: nullStr(quote), SettlementSymbol: nullStr(settlement), ContractSize: nullDecimal(contractSize),
		Side: side, Quantity: decimal.Compact(quantity), QuantityUnit: unit,
		EntryPrice: nullDecimal(entry), MarkPrice: nullDecimal(mark), LiquidationPrice: nullDecimal(liq), UnrealizedPnl: nullDecimal(upnl),
		Leverage: nullDecimal(leverage), Margin: nullDecimal(margin),
		Freshness: account.Freshness, ReportObservedAt: account.ReportObservedAt,
	}
	if expiry.Valid {
		t := expiry.Time
		row.ExpiryAt = &t
	}
	size, _ := normalizedPositionSize(&row)
	if size == nil || !quote.Valid || !usdQuote(quote.String) {
		return row, nil
	}
	qty, ok := decimal.Parse(*size)
	if !ok {
		return row, nil
	}
	if inverseKind(kind) {
		// Inverse Size is already the Quote face value; no Mark is required.
		row.NotionalUSD = decimal.FormatPtr(decimal.Abs(qty))
	} else if mark.Valid {
		if price, valid := decimal.Parse(mark.String); valid && price.Sign() > 0 {
			row.NotionalUSD = decimal.FormatPtr(decimal.Abs(decimal.Mul(qty, price)))
		}
	}
	return row, nil
}

// FlatBalances lists every Balance in the boundary at the latest point.
func (s *Service) FlatBalances(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[BalanceRow], error) {
	limit, offset, err := normalizePage(f.Limit, f.Offset)
	if err != nil {
		return FlatList[BalanceRow]{}, err
	}
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return FlatList[BalanceRow]{}, err
	}
	out := FlatList[BalanceRow]{Context: c, Items: []BalanceRow{}, Offset: offset, Limit: limit, Completeness: CompletenessIncomplete, Issues: []Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	ignored, err := listingexclusion.ActiveForJob(ctx, s.db, p.ID, point.JobID)
	if err != nil {
		return FlatList[BalanceRow]{}, err
	}
	out.Completeness = point.Completeness()
	out.Issues = point.Issues
	byAccount := map[uuid.UUID]*AccountValuation{}
	for i := range accounts {
		byAccount[accounts[i].AccountID] = &accounts[i]
	}
	rows, err := s.q.ListBalancesByJob(ctx, sqlcgen.ListBalancesByJobParams{PortfolioID: p.ID[:], JobID: point.JobID[:]})
	if err != nil {
		return FlatList[BalanceRow]{}, err
	}
	book := &priceBook{s: s, cache: map[priceKey]map[string]pricePoint{}}
	all := make([]BalanceRow, 0, len(rows))
	total := decimal.Zero()
	for i := range rows {
		accountID, err := uuid.FromBytes(rows[i].AccountID)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		account, ok := byAccount[accountID]
		if !ok {
			continue
		}
		if !matchesAccount(f, account) || !matchesSymbol(f.Symbol, rows[i].AssetSymbol, rows[i].Symbol) {
			continue
		}
		listingID, err := uuid.FromBytes(rows[i].ListingID)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		if account.Method == MethodCalculated && ignored[listingID] {
			continue
		}
		at := time.Time{}
		if account.ReportObservedAt != nil {
			at = *account.ReportObservedAt
		}
		row, err := balanceRow(ctx, book, account, at, rows[i].ListingID, rows[i].Platform, rows[i].Symbol, rows[i].AssetSymbol, rows[i].Equity, rows[i].AvailableToWithdraw, rows[i].AvailableToTrade, rows[i].Liability, rows[i].Interest, rows[i].UnrealizedPnl)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		if row.equityUSDRat != nil {
			total = decimal.Add(total, row.equityUSDRat)
		}
		all = append(all, row)
	}
	out.TotalUSD = decimal.FormatPtr(total)
	out.Total = len(all)
	sortBalances(all, accountRanks(accounts))
	end := min(offset+limit, len(all))
	if offset < len(all) {
		out.Items = all[offset:end]
	}
	return out, nil
}

// FlatPositions lists every Position in the boundary at the latest point.
func (s *Service) FlatPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[PositionRow], error) {
	limit, offset, err := normalizePage(f.Limit, f.Offset)
	if err != nil {
		return FlatList[PositionRow]{}, err
	}
	c, point, accounts, err := s.latestContext(ctx, p)
	if err != nil {
		return FlatList[PositionRow]{}, err
	}
	out := FlatList[PositionRow]{Context: c, Items: []PositionRow{}, Offset: offset, Limit: limit, Completeness: CompletenessIncomplete, Issues: []Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
		return out, nil
	}
	out.Completeness = CompletenessComplete
	for i := range accounts {
		account := &accounts[i]
		if positionAccount(account) {
			if account.Method == MethodMissing {
				out.Completeness = CompletenessIncomplete
			}
			out.Issues = append(out.Issues, positionAccountIssues(account)...)
		}
	}
	byAccount := map[uuid.UUID]*AccountValuation{}
	for i := range accounts {
		byAccount[accounts[i].AccountID] = &accounts[i]
	}
	rows, err := s.q.ListPositionsByJob(ctx, sqlcgen.ListPositionsByJobParams{PortfolioID: p.ID[:], JobID: point.JobID[:]})
	if err != nil {
		return FlatList[PositionRow]{}, err
	}
	all := make([]PositionRow, 0, len(rows))
	total := decimal.Zero()
	for i := range rows {
		r := &rows[i]
		accountID, err := uuid.FromBytes(r.AccountID)
		if err != nil {
			return FlatList[PositionRow]{}, err
		}
		account, ok := byAccount[accountID]
		if !ok {
			continue
		}
		row, err := positionRow(account, r.InstrumentID, r.Platform, r.Symbol, r.Kind, r.BaseSymbol, r.QuoteSymbol, r.SettlementSymbol, r.ExpiryAt, r.ContractSize, r.PlatformPositionRef, r.Side, r.Quantity, r.QuantityUnit, r.EntryPrice, r.MarkPrice, r.LiquidationPrice, r.UnrealizedPnl, r.Leverage, r.Margin)
		if err != nil {
			return FlatList[PositionRow]{}, err
		}
		completePosition(&row, account, positionMetadata{r.PositionMode, r.MarginMode, r.MarginCurrency, r.MarginBasis})
		out.Issues = append(out.Issues, positionDataIssues(row.Issues)...)
		if row.NotionalUSD == nil {
			out.Completeness = CompletenessIncomplete
		}
		if row.NotionalUSD != nil {
			total = decimal.Add(total, decimal.MustParse(*row.NotionalUSD))
		}
		if !matchesAccount(f, account) || !matchesSymbol(f.Symbol, r.Symbol) {
			continue
		}
		all = append(all, row)
	}
	out.TotalUSD = decimal.FormatPtr(total)
	out.Total = len(all)
	sortPositions(all, accountRanks(accounts))
	end := min(offset+limit, len(all))
	if offset < len(all) {
		out.Items = all[offset:end]
	}
	return out, nil
}

func matchesAccount(f *ListFilter, a *AccountValuation) bool {
	if f.Platform != "" && f.Platform != a.Platform {
		return false
	}
	if f.IdentityID != nil && *f.IdentityID != a.IdentityID {
		return false
	}
	return true
}

// matchesSymbol applies a literal, case-insensitive substring search.
func matchesSymbol(query string, symbols ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, symbol := range symbols {
		if strings.Contains(strings.ToLower(symbol), query) {
			return true
		}
	}
	return false
}

// Points returns the series between from and to, sampled by granularity (raw, hour, day).
func (s *Service) Points(ctx context.Context, p PortfolioContext, from, to time.Time, granularity string) ([]Point, error) {
	if !to.After(from) {
		return nil, ErrInvalid
	}
	switch granularity {
	case "", "raw", "hour", "day":
	default:
		return nil, ErrInvalid
	}
	return s.pointSeries(ctx, p, from, to, granularity)
}

func sample(points []Point, granularity string, loc *time.Location) []Point {
	if granularity == "" || granularity == "raw" || len(points) == 0 {
		return points
	}
	out := make([]Point, 0, len(points))
	var lastBucket time.Time
	for i := range points {
		local := points[i].EvaluatedAt.In(loc)
		var bucket time.Time
		if granularity == "hour" {
			bucket = time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
		} else {
			bucket = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
		}
		if len(out) > 0 && bucket.Equal(lastBucket) {
			out[len(out)-1] = points[i]
			continue
		}
		out = append(out, points[i])
		lastBucket = bucket
	}
	return out
}

// Detail returns one point with its Account Valuations.
func (s *Service) Detail(ctx context.Context, p PortfolioContext, pointID uuid.UUID) (PointDetail, error) {
	point, err := s.Get(ctx, p.ID, pointID)
	if err != nil {
		return PointDetail{}, err
	}
	accounts, err := s.Accounts(ctx, p.ID, point.JobID)
	if err != nil {
		return PointDetail{}, err
	}
	return PointDetail{Point: point, Accounts: accounts}, nil
}

// BoundaryEvents lists derived Boundary Events between two instants.
func (s *Service) BoundaryEvents(ctx context.Context, p PortfolioContext, from, to time.Time) ([]BoundaryEvent, error) {
	if !to.After(from) {
		return nil, ErrInvalid
	}
	rows, err := s.q.ListValuationPointsBetween(ctx, sqlcgen.ListValuationPointsBetweenParams{PortfolioID: p.ID[:], FromAt: from.UTC(), ToAt: to.UTC()})
	if err != nil {
		return nil, err
	}
	points, err := pointsFromRows(rows)
	if err != nil {
		return nil, err
	}
	out := []BoundaryEvent{}
	for i := range points {
		out = append(out, points[i].BoundaryEvents...)
	}
	return out, nil
}

func pointsFromRows(rows []sqlcgen.ValuationPoint) ([]Point, error) {
	out := make([]Point, 0, len(rows))
	for i := range rows {
		point, err := pointFromRow(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, nil
}

func pointFromRow(row *sqlcgen.ValuationPoint) (Point, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return Point{}, err
	}
	portfolioID, err := uuid.FromBytes(row.PortfolioID)
	if err != nil {
		return Point{}, err
	}
	jobID, err := uuid.FromBytes(row.JobID)
	if err != nil {
		return Point{}, err
	}
	point := Point{
		ID:                      id,
		PortfolioID:             portfolioID,
		JobID:                   jobID,
		EvaluatedAt:             row.EvaluatedAt,
		TotalUSD:                decimal.Compact(row.TotalUsd),
		AccountCount:            int(row.AccountCount),
		StaleAccountCount:       int(row.StaleAccountCount),
		IncompleteAccountCount:  int(row.IncompleteAccountCount),
		BoundaryContributionUSD: decimal.Compact(row.BoundaryContributionUsd),
		BoundaryWithdrawalUSD:   decimal.Compact(row.BoundaryWithdrawalUsd),
		BoundaryEvents:          []BoundaryEvent{},
		Issues:                  []Issue{},
		CreatedAt:               row.CreatedAt,
	}
	if row.RecalculatedAt.Valid {
		t := row.RecalculatedAt.Time
		point.RecalculatedAt = &t
	}
	if len(row.BoundaryJson) > 0 {
		if err := json.Unmarshal(row.BoundaryJson, &point.BoundaryEvents); err != nil {
			return Point{}, err
		}
		for i := range point.BoundaryEvents {
			point.BoundaryEvents[i].PointID = id
			point.BoundaryEvents[i].EvaluatedAt = row.EvaluatedAt
			if point.BoundaryEvents[i].ValueUSD != nil {
				point.BoundaryEvents[i].valueRat = decimal.MustParse(*point.BoundaryEvents[i].ValueUSD)
			}
		}
	}
	if len(row.IssuesJson) > 0 {
		if err := json.Unmarshal(row.IssuesJson, &point.Issues); err != nil {
			return Point{}, err
		}
	}
	return point, nil
}

func normalizePage(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < minLimit || limit > maxLimit || offset < 0 {
		return 0, 0, ErrInvalid
	}
	return limit, offset, nil
}

func (s *Service) positionView(ctx context.Context, p PortfolioContext, accountID uuid.UUID, pointID *uuid.UUID) (AccountView, *uuid.UUID, error) {
	if pointID == nil {
		return s.accountView(ctx, p, accountID)
	}
	detail, err := s.Detail(ctx, p, *pointID)
	if err != nil {
		return AccountView{}, nil, err
	}
	for i := range detail.Accounts {
		account := &detail.Accounts[i]
		if account.AccountID == accountID {
			view := AccountView{Context: Context{PortfolioID: p.ID, Timezone: p.Timezone, PointID: &detail.Point.ID, JobID: &detail.Point.JobID, EvaluatedAt: &detail.Point.EvaluatedAt}, Account: *account, Issues: positionAccountIssues(account)}
			return view, account.SourceReportID, nil
		}
	}
	return AccountView{}, nil, ErrNotFound
}
