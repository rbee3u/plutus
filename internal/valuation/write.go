package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
	"github.com/rbee3u/plutus/internal/listingexclusion"
)

// Service computes and reads valuations.
type Service struct {
	db  *sql.DB
	q   *sqlcgen.Queries
	now func() time.Time
}

// NewService constructs a valuation service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db, q: sqlcgen.New(db), now: time.Now}
}

// WritePoint values every Account Report of the job and upserts the job's Valuation Point.
// evaluatedAt is the instant recorded for the point; zero means now.
func (s *Service) WritePoint(ctx context.Context, portfolioID, jobID uuid.UUID, evaluatedAt time.Time) (Point, error) {
	if evaluatedAt.IsZero() {
		evaluatedAt = s.now()
	}
	evaluatedAt = evaluatedAt.UTC().Truncate(time.Microsecond)
	accounts, err := s.valueJob(ctx, portfolioID, jobID)
	if err != nil {
		return Point{}, err
	}
	previous, err := s.previousAccounts(ctx, portfolioID, evaluatedAt, jobID)
	if err != nil {
		return Point{}, err
	}
	point := buildPoint(portfolioID, jobID, evaluatedAt, accounts, previous)

	if err := s.persistPoint(ctx, point, accounts); err != nil {
		return Point{}, err
	}
	return s.pointByJob(ctx, portfolioID, jobID)
}

// persistPoint commits report values and their aggregate atomically.
func (s *Service) persistPoint(ctx context.Context, point Point, accounts []AccountValuation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := updateReportValuations(ctx, tx, point.PortfolioID, accounts); err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	boundaryJSON, err := marshalOrNil(point.BoundaryEvents)
	if err != nil {
		return err
	}
	issuesJSON, err := marshalOrNil(point.Issues)
	if err != nil {
		return err
	}
	if err := q.UpsertValuationPoint(ctx, sqlcgen.UpsertValuationPointParams{
		ID:                      id[:],
		PortfolioID:             point.PortfolioID[:],
		JobID:                   point.JobID[:],
		EvaluatedAt:             point.EvaluatedAt,
		TotalUsd:                point.TotalUSD,
		AccountCount:            uint32(point.AccountCount),           // #nosec G115 -- The count is bounded by the in-memory Account slice, far below 2^32.
		StaleAccountCount:       uint32(point.StaleAccountCount),      // #nosec G115 -- The count is a subset of the in-memory Account slice.
		IncompleteAccountCount:  uint32(point.IncompleteAccountCount), // #nosec G115 -- The count is a subset of the in-memory Account slice.
		BoundaryContributionUsd: point.BoundaryContributionUSD,
		BoundaryWithdrawalUsd:   point.BoundaryWithdrawalUSD,
		BoundaryJson:            boundaryJSON,
		IssuesJson:              issuesJSON,
		CreatedAt:               s.now().UTC(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

type priceResolver interface {
	lookup(context.Context, string, string, time.Time) (*big.Rat, bool, error)
	lookupPlatform(context.Context, string, string, time.Time) (*big.Rat, bool, error)
}

type priceKey struct {
	platform string
	at       time.Time
}

type priceBook struct {
	s     *Service
	cache map[priceKey]map[string]pricePoint
}

type pricePoint struct {
	value      *big.Rat
	observedAt time.Time
}

func (b *priceBook) lookup(ctx context.Context, platform, assetSymbol string, at time.Time) (*big.Rat, bool, error) {
	return b.lookupWithPolicy(ctx, platform, assetSymbol, at, false)
}

func (b *priceBook) lookupPlatform(ctx context.Context, platform, assetSymbol string, at time.Time) (*big.Rat, bool, error) {
	return b.lookupWithPolicy(ctx, platform, assetSymbol, at, true)
}

func (b *priceBook) lookupWithPolicy(ctx context.Context, platform, assetSymbol string, at time.Time, ownPlatform bool) (*big.Rat, bool, error) {
	switch assetSymbol {
	case "USD", "USDT", "USDC":
		return big.NewRat(1, 1), true, nil
	}
	key := priceKey{platform: platform, at: at}
	book, ok := b.cache[key]
	if !ok {
		rows, err := b.s.q.ListLatestPricesByPlatformAt(ctx, sqlcgen.ListLatestPricesByPlatformAtParams{Platform: platform, At: at})
		if err != nil {
			return nil, false, err
		}
		book = map[string]pricePoint{}
		for i := range rows {
			if !usdQuote(rows[i].QuoteSymbol) {
				continue
			}
			if value, ok := decimal.Parse(rows[i].Price); ok {
				book[rows[i].AssetSymbol] = pricePoint{value: value, observedAt: rows[i].ObservedAt}
			}
		}
		b.cache[key] = book
	}
	if p, ok := book[assetSymbol]; ok && at.Sub(p.observedAt) <= maxPriceAge {
		return p.value, true, nil
	}
	if ownPlatform {
		return nil, false, nil
	}
	row, err := b.s.q.GetNearestPriceAt(ctx, sqlcgen.GetNearestPriceAtParams{AssetSymbol: assetSymbol, FilterPlatform: 0, At: at})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !usdQuote(row.QuoteSymbol) || at.Sub(row.ObservedAt) > maxPriceAge {
		return nil, false, nil
	}
	value, ok := decimal.Parse(row.Price)
	return value, ok, nil
}

func usdQuote(symbol string) bool {
	switch symbol {
	case "USD", "USDT", "USDC":
		return true
	}
	return false
}

// inverseKind reports whether an Instrument settles in its base Asset, which makes its contract
// size a USD face value rather than a quantity of the base Asset.
func inverseKind(kind string) bool {
	return kind == adapters.KindInversePerp || kind == adapters.KindInverseDlv
}

// valueJob computes the Account Valuation of every report in the job without writing.
func (s *Service) valueJob(ctx context.Context, portfolioID, jobID uuid.UUID) ([]AccountValuation, error) {
	ignored, err := listingexclusion.ActiveForJob(ctx, s.db, portfolioID, jobID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: jobID[:]})
	if err != nil {
		return nil, err
	}
	book := &priceBook{s: s, cache: map[priceKey]map[string]pricePoint{}}
	out := make([]AccountValuation, 0, len(rows))
	for i := range rows {
		item, err := s.valueReport(ctx, book, &rows[i], ignored, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	sortAccounts(out)
	return out, nil
}

func (s *Service) valueReport(ctx context.Context, book priceResolver, row *sqlcgen.ListReportsByJobRow, ignored map[uuid.UUID]bool, inputs *reportInputs) (AccountValuation, error) {
	item, err := accountFromRow(row)
	if err != nil {
		return AccountValuation{}, err
	}
	item.Omitted = nil
	item.ValueUSD = nil
	item.valueRat = nil
	item.SourceReportID = nil
	var sourceID uuid.UUID
	var observedAt time.Time
	var metrics [6]sql.NullString
	if row.Status == "ok" {
		sourceID = item.ReportID
		observedAt = row.ObservedAt.Time
		metrics = [6]sql.NullString{row.Equity, row.AdjustedEquity, row.AvailableEquity, row.InitialMarginRatio, row.MaintenanceMarginRatio, row.UnifiedMarginRatio}
		item.Freshness = FreshnessFresh
		item.sourceMarginMode = nullStr(row.MarginMode)
	} else {
		var src sqlcgen.GetLatestOkReportForAccountRow
		if inputs == nil {
			src, err = s.q.GetLatestOkReportForAccount(ctx, sqlcgen.GetLatestOkReportForAccountParams{PortfolioID: row.PortfolioID, AccountID: row.AccountID, ScopeVersion: row.ScopeVersion, Before: row.CreatedAt})
		} else if previous := inputs.sources[item.ReportID]; previous != nil {
			src = *previous
		} else {
			err = sql.ErrNoRows
		}
		if errors.Is(err, sql.ErrNoRows) {
			item.Method = MethodMissing
			item.Freshness = FreshnessStale
			item.Equity, item.AdjustedEquity, item.AvailableEquity, item.InitialMarginRatio, item.MaintenanceMarginRatio, item.UnifiedMarginRatio = nil, nil, nil, nil, nil, nil
			item.ReportObservedAt = nil
			return item, nil
		}
		if err != nil {
			return AccountValuation{}, err
		}
		sourceID, err = uuid.FromBytes(src.ID)
		if err != nil {
			return AccountValuation{}, err
		}
		observedAt = src.ObservedAt.Time
		item.sourceMarginMode = nullStr(src.MarginMode)
		metrics = [6]sql.NullString{src.Equity, src.AdjustedEquity, src.AvailableEquity, src.InitialMarginRatio, src.MaintenanceMarginRatio, src.UnifiedMarginRatio}
		item.Freshness = FreshnessStale
	}
	item.SourceReportID = &sourceID
	item.ReportObservedAt = &observedAt
	item.Equity, item.AdjustedEquity, item.AvailableEquity = nullDecimal(metrics[0]), nullDecimal(metrics[1]), nullDecimal(metrics[2])
	item.InitialMarginRatio, item.MaintenanceMarginRatio, item.UnifiedMarginRatio = nullDecimal(metrics[3]), nullDecimal(metrics[4]), nullDecimal(metrics[5])

	if metrics[0].Valid {
		if value, ok := decimal.Parse(metrics[0].String); ok {
			item.Method = MethodReported
			item.valueRat = value
			item.ValueUSD = decimal.FormatPtr(value)
			return item, nil
		}
	}
	var balances []sqlcgen.ListBalancesByReportRow
	if inputs == nil {
		balances, err = s.q.ListBalancesByReport(ctx, sourceID[:])
		if err != nil {
			return AccountValuation{}, err
		}
	} else {
		var loaded bool
		balances, loaded = inputs.balances[sourceID]
		if !loaded {
			return AccountValuation{}, fmt.Errorf("balances were not loaded for source report %s", sourceID)
		}
	}
	total := decimal.Zero()
	for i := range balances {
		listingID, err := uuid.FromBytes(balances[i].ListingID)
		if err != nil {
			return AccountValuation{}, err
		}
		if ignored[listingID] {
			continue
		}
		equity, ok := decimal.Parse(balances[i].Equity)
		if !ok {
			item.Omitted = append(item.Omitted, Omitted{Symbol: balances[i].Symbol, AssetSymbol: mappedSymbol(balances[i].Symbol, balances[i].AssetSymbol), Equity: balances[i].Equity, Reason: "invalid_equity"})
			continue
		}
		if equity.Sign() == 0 {
			continue
		}
		lookup := book.lookup
		if item.AccountType == string(adapters.TypeManual) {
			lookup = book.lookupPlatform
		}
		price, found, err := lookup(ctx, balances[i].Platform, balances[i].AssetSymbol, observedAt)
		if err != nil {
			return AccountValuation{}, err
		}
		if !found {
			item.Omitted = append(item.Omitted, Omitted{Symbol: balances[i].Symbol, AssetSymbol: mappedSymbol(balances[i].Symbol, balances[i].AssetSymbol), Equity: decimal.Compact(balances[i].Equity), Reason: "no_price"})
			continue
		}
		total = decimal.Add(total, decimal.Mul(equity, price))
	}
	item.Method = MethodCalculated
	item.valueRat = total
	item.ValueUSD = decimal.FormatPtr(total)
	return item, nil
}

// mappedSymbol reports the Asset symbol only when a listing is mapped away from its Platform symbol.
func mappedSymbol(listingSymbol, assetSymbol string) string {
	if assetSymbol == listingSymbol {
		return ""
	}
	return assetSymbol
}

type previousAccount struct {
	manualState  string
	scopeVersion uint16
	accountType  string
	name         string
	identityID   uuid.UUID
	identityName string
	platform     string
	method       string
	value        *big.Rat
}

// previousAccounts returns the Account values of the point immediately before evaluatedAt.
func (s *Service) previousAccounts(ctx context.Context, portfolioID uuid.UUID, evaluatedAt time.Time, jobID uuid.UUID) (map[uuid.UUID]previousAccount, error) {
	prev, err := s.q.GetValuationPointBefore(ctx, sqlcgen.GetValuationPointBeforeParams{PortfolioID: portfolioID[:], At: evaluatedAt})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // a nil map means "no previous point", which disables Boundary Event derivation.
	}
	if err != nil {
		return nil, err
	}
	prevJob, err := uuid.FromBytes(prev.JobID)
	if err != nil {
		return nil, err
	}
	if prevJob == jobID {
		return nil, nil //nolint:nilnil // rebuilding the first point: nothing precedes it.
	}
	rows, err := s.q.ListReportsByJob(ctx, sqlcgen.ListReportsByJobParams{PortfolioID: portfolioID[:], JobID: prev.JobID})
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]previousAccount, len(rows))
	for i := range rows {
		accountID, err := uuid.FromBytes(rows[i].AccountID)
		if err != nil {
			return nil, err
		}
		identityID, err := uuid.FromBytes(rows[i].IdentityID)
		if err != nil {
			return nil, err
		}
		entry := previousAccount{manualState: canonicalManualState(rows[i].ManualStateJson), scopeVersion: rows[i].ScopeVersion, accountType: rows[i].AccountType, name: rows[i].AccountName, identityID: identityID, identityName: rows[i].IdentityName, platform: rows[i].Platform, method: rows[i].Method}
		if rows[i].ValueUsd.Valid {
			if value, ok := decimal.Parse(rows[i].ValueUsd.String); ok {
				entry.value = value
			}
		}
		out[accountID] = entry
	}
	return out, nil
}

// buildPoint aggregates Account Valuations into a point with issues and Boundary Events.
func buildPoint(portfolioID, jobID uuid.UUID, evaluatedAt time.Time, accounts []AccountValuation, previous map[uuid.UUID]previousAccount) Point {
	point := Point{PortfolioID: portfolioID, JobID: jobID, EvaluatedAt: evaluatedAt, AccountCount: len(accounts)}
	total := decimal.Zero()
	current := map[uuid.UUID]bool{}
	transitions := scopeTransitions(accounts, previous)
	markedTransitions := map[uuid.UUID]bool{}
	contributions := decimal.Zero()
	withdrawals := decimal.Zero()
	for i := range accounts {
		a := &accounts[i]
		current[a.AccountID] = true
		if a.valueRat != nil {
			total = decimal.Add(total, a.valueRat)
		}
		if a.Freshness == FreshnessStale && a.Method != MethodMissing {
			point.StaleAccountCount++
		}
		if a.Completeness() == CompletenessIncomplete {
			point.IncompleteAccountCount++
		}
		point.Issues = append(point.Issues, accountIssues(a)...)
		if a.AccountType == string(adapters.TypeManual) && previous != nil && identityWasPresent(previous, a.IdentityID) {
			old := previous[a.AccountID].manualState
			if canonicalManualState([]byte(old)) != canonicalManualState([]byte(a.manualState)) {
				point.Issues = append(point.Issues, Issue{Code: "manual_balance_changed", AccountID: &a.AccountID, AccountName: a.AccountName, Detail: "Manual quantities changed for " + a.IdentityName + "; value changes across this point include a user adjustment."})
			}
		}
		if transitions[a.IdentityID] && (a.AccountType == "earn" || a.AccountType == "dual_investment") {
			if !markedTransitions[a.IdentityID] {
				markedTransitions[a.IdentityID] = true
				point.Issues = append(point.Issues, Issue{Code: "account_scope_changed", AccountID: &a.AccountID, Detail: "Earn and Dual Investment were separated for " + a.IdentityName + "; value changes across this point use different account scopes."})
			}
		}
		if previous != nil {
			if _, existed := previous[a.AccountID]; !existed && !(a.AccountType == string(adapters.TypeManual) && identityWasPresent(previous, a.IdentityID)) && (!transitions[a.IdentityID] || a.AccountType != "dual_investment") {
				event := BoundaryEvent{AccountID: a.AccountID, AccountName: a.AccountName, IdentityID: a.IdentityID, IdentityName: a.IdentityName, Platform: a.Platform, Direction: DirectionContribution, Method: a.Method, ValueUSD: a.ValueUSD, valueRat: a.valueRat}
				point.BoundaryEvents = append(point.BoundaryEvents, event)
				if a.valueRat != nil {
					contributions = decimal.Add(contributions, a.valueRat)
				} else {
					id := a.AccountID
					point.Issues = append(point.Issues, Issue{Code: "boundary_unvalued", AccountID: &id, AccountName: a.AccountName, Detail: "The account entered the boundary without a value; the boundary event is unvalued."})
				}
			}
		}
	}
	if previous != nil {
		ids := make([]uuid.UUID, 0, len(previous))
		for id := range previous {
			if !current[id] {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		for _, id := range ids {
			prev := previous[id]
			if transitions[prev.identityID] && prev.accountType == "earn" {
				continue
			}
			event := BoundaryEvent{AccountID: id, AccountName: prev.name, IdentityID: prev.identityID, IdentityName: prev.identityName, Platform: prev.platform, Direction: DirectionWithdrawal, Method: MethodMissing}
			if prev.value != nil {
				event.Method = prev.method
				event.ValueUSD = decimal.FormatPtr(prev.value)
				event.valueRat = prev.value
				withdrawals = decimal.Add(withdrawals, prev.value)
			} else {
				accountID := id
				point.Issues = append(point.Issues, Issue{Code: "boundary_unvalued", AccountID: &accountID, AccountName: prev.name, Detail: "The account left the boundary without a value; the boundary event is unvalued."})
			}
			point.BoundaryEvents = append(point.BoundaryEvents, event)
		}
	}
	point.TotalUSD = decimal.Format(total)
	point.BoundaryContributionUSD = decimal.Format(contributions)
	point.BoundaryWithdrawalUSD = decimal.Format(withdrawals)
	return point
}

func accountIssues(a *AccountValuation) []Issue {
	id := a.AccountID
	var out []Issue
	switch {
	case a.Method == MethodMissing:
		detail := "No successful report exists for the current account scope; its value is unknown."
		if a.ScopeVersion == 2 && a.AccountType == "earn" {
			detail += " Earlier combined Earn reports include Dual Investment and cannot be reused."
		}
		if a.ErrorDetail != nil {
			detail += " Last error: " + *a.ErrorDetail
		}
		out = append(out, Issue{Code: "missing", AccountID: &id, AccountName: a.AccountName, Detail: detail})
	case a.Freshness == FreshnessStale:
		detail := "The account report failed this cycle; the previous successful report is carried forward."
		if a.ErrorDetail != nil {
			detail = "The account report failed this cycle (" + *a.ErrorDetail + "); the previous successful report is carried forward."
		}
		if a.ReportObservedAt != nil {
			detail += " Observed at " + a.ReportObservedAt.UTC().Format(time.RFC3339) + "."
		}
		out = append(out, Issue{Code: "stale", AccountID: &id, AccountName: a.AccountName, Detail: detail})
	}
	for _, o := range a.Omitted {
		detail := fmt.Sprintf("%s %s was excluded from the account value (%s).", o.Equity, o.Symbol, o.Reason)
		if o.AssetSymbol != "" {
			detail = fmt.Sprintf("%s %s (mapped to %s) was excluded from the account value (%s).", o.Equity, o.Symbol, o.AssetSymbol, o.Reason)
		}
		out = append(out, Issue{Code: o.Reason, AccountID: &id, AccountName: a.AccountName, Symbol: o.Symbol, Detail: detail})
	}
	return out
}

func marshalOrNil(value any) (sqljson.Column, error) {
	switch v := value.(type) {
	case []BoundaryEvent:
		if len(v) == 0 {
			return nil, nil
		}
	case []Issue:
		if len(v) == 0 {
			return nil, nil
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return sqljson.Column(raw), nil
}

func nullDecimal(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := decimal.Compact(v.String)
	return &s
}

func accountFromRow(row *sqlcgen.ListReportsByJobRow) (AccountValuation, error) {
	reportID, err := uuid.FromBytes(row.ID)
	if err != nil {
		return AccountValuation{}, err
	}
	accountID, err := uuid.FromBytes(row.AccountID)
	if err != nil {
		return AccountValuation{}, err
	}
	identityID, err := uuid.FromBytes(row.IdentityID)
	if err != nil {
		return AccountValuation{}, err
	}
	item := AccountValuation{
		ReportID:         reportID,
		manualState:      canonicalManualState(row.ManualStateJson),
		ManualModifiedAt: manualTime(row.ManualModifiedAt),
		ScopeVersion:     row.ScopeVersion,
		AccountID:        accountID,
		AccountName:      row.AccountName,
		AccountType:      row.AccountType,
		accountStructure: row.AccountStructure,
		Selected:         row.Selected,
		IdentityID:       identityID,
		IdentityName:     row.IdentityName,
		IdentityStatus:   row.IdentityStatus,
		ConnectionStatus: row.ConnectionStatus,
		Platform:         row.Platform,
		Status:           row.Status,
		ErrorClass:       nullStr(row.ErrorClass),
		ErrorDetail:      nullStr(row.ErrorDetail),
		Method:           row.Method,
		Freshness:        FreshnessStale,
		ValueUSD:         nullDecimal(row.ValueUsd),
	}
	if row.Status == "ok" {
		item.sourceMarginMode = nullStr(row.MarginMode)
		item.Freshness = FreshnessFresh
		item.Equity, item.AdjustedEquity, item.AvailableEquity = nullDecimal(row.Equity), nullDecimal(row.AdjustedEquity), nullDecimal(row.AvailableEquity)
		item.InitialMarginRatio, item.MaintenanceMarginRatio, item.UnifiedMarginRatio = nullDecimal(row.InitialMarginRatio), nullDecimal(row.MaintenanceMarginRatio), nullDecimal(row.UnifiedMarginRatio)
		if row.ObservedAt.Valid {
			t := row.ObservedAt.Time
			item.ReportObservedAt = &t
		}
	} else {
		item.sourceMarginMode = nullStr(row.SourceMarginMode)
		item.Equity, item.AdjustedEquity, item.AvailableEquity = nullDecimal(row.SourceEquity), nullDecimal(row.SourceAdjustedEquity), nullDecimal(row.SourceAvailableEquity)
		item.InitialMarginRatio, item.MaintenanceMarginRatio, item.UnifiedMarginRatio = nullDecimal(row.SourceInitialMarginRatio), nullDecimal(row.SourceMaintenanceMarginRatio), nullDecimal(row.SourceUnifiedMarginRatio)
		if row.SourceObservedAt.Valid {
			t := row.SourceObservedAt.Time
			item.ReportObservedAt = &t
		}
	}
	if len(row.SourceReportID) == 16 {
		src, err := uuid.FromBytes(row.SourceReportID)
		if err == nil {
			item.SourceReportID = &src
		}
	}
	if row.ValueUsd.Valid {
		if value, ok := decimal.Parse(row.ValueUsd.String); ok {
			item.valueRat = value
		}
	}
	if len(row.OmittedJson) > 0 {
		_ = json.Unmarshal(row.OmittedJson, &item.Omitted)
	}
	return item, nil
}

func nullStr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

// scopeTransitions identifies reclassification of an already observed combined Earn.
func scopeTransitions(accounts []AccountValuation, previous map[uuid.UUID]previousAccount) map[uuid.UUID]bool {
	legacy := map[uuid.UUID]bool{}
	for _, old := range previous {
		if old.accountType == "earn" && old.scopeVersion == 1 && (old.platform == "binance" || old.platform == "okx" || old.platform == "bybit") {
			legacy[old.identityID] = true
		}
	}
	out := map[uuid.UUID]bool{}
	for i := range accounts {
		a := &accounts[i]
		if legacy[a.IdentityID] && a.ScopeVersion == 2 && (a.AccountType == "earn" || a.AccountType == "dual_investment") {
			out[a.IdentityID] = true
		}
	}
	return out
}
