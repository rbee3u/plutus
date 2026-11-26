package financial

import (
	"context"
	"database/sql"
	"math/big"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// AccountPositions returns the positions of an account's current report with USD notionals.
func (s *Service) AccountPositions(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		return s.AccountPositionsAt(ctx, p, accountID, nil)
	})
}

// AccountPositionsAt reads the selected point, retaining its compatible source report.
func (s *Service) AccountPositionsAt(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, source, err := s.accountView(ctx, p, accountID, cycleID)
		if err != nil {
			return AccountView{}, err
		}
		return s.readAccountPositions(ctx, p, view, source)
	})
}

func (s *Service) readAccountPositions(ctx context.Context, p PortfolioContext, view AccountView, source *uuid.UUID) (AccountView, error) {
	positions, issues, err := s.loadPositions(ctx, p, &view, source)
	if err != nil {
		return AccountView{}, err
	}
	view.Positions = positions
	view.Issues = append(view.Issues, issues...)
	return view, nil
}

// loadPositions maps one source report's positions with their display facts.
// Balance reads reuse it so risk exposure and the Futures table describe the
// same retained position set.
func (s *Service) loadPositions(ctx context.Context, p PortfolioContext, view *AccountView, source *uuid.UUID) ([]PositionRow, []valuation.Issue, error) {
	positions := []PositionRow{}
	issues := []valuation.Issue{}
	if source == nil {
		return positions, issues, nil
	}
	rows, err := mysql.Queries(ctx, s.db).ListPositionsByReport(ctx, sqlcgen.ListPositionsByReportParams{PortfolioID: p.ID[:], ReportID: source[:]})
	if err != nil {
		return nil, nil, err
	}
	for i := range rows {
		r := &rows[i]
		row, err := positionRow(&view.Account, r)
		if err != nil {
			return nil, nil, err
		}
		completePosition(&row, &view.Account, positionMetadata{sql.NullString{String: r.PositionMode, Valid: true}, r.MarginSetting, r.MarginAssetSymbol, r.MarginBasis})
		issues = append(issues, positionDataIssues(row.Issues)...)
		positions = append(positions, row)
	}
	sortPositions(positions, nil)
	return positions, issues, nil
}

func positionRow(account *valuation.AccountValuation, source *sqlcgen.ListPositionsByReportRow) (PositionRow, error) {
	id, err := uuid.FromBytes(source.FuturesContractID)
	if err != nil {
		return PositionRow{}, err
	}
	row := PositionRow{
		AccountID:             account.AccountID,
		AccountName:           account.AccountName,
		AccountType:           account.AccountType,
		IdentityID:            account.IdentityID,
		IdentityName:          account.IdentityName,
		Platform:              source.Platform,
		PositionKey:           source.PositionKey,
		FuturesContractID:     id,
		FuturesSymbol:         source.FuturesSymbol,
		Kind:                  source.Kind,
		BaseAssetSymbol:       nullStringPtr(source.BaseAssetSymbol),
		QuoteAssetSymbol:      nullStringPtr(source.QuoteAssetSymbol),
		SettlementAssetSymbol: nullStringPtr(source.SettlementAssetSymbol),
		ContractSize:          nullDecimal(source.ContractSize),
		Side:                  source.Side,
		SourceQuantity:        decimal.Compact(source.SourceQuantity),
		SourceQuantityUnit:    source.SourceQuantityUnit,
		EntryPrice:            nullDecimal(source.EntryPrice),
		MarkPrice:             nullDecimal(source.MarkPrice),
		LiquidationPrice:      nullDecimal(source.LiquidationPrice),
		UnrealizedPnl:         nullDecimal(source.UnrealizedPnl),
		Leverage:              nullDecimal(source.Leverage),
		Margin:                nullDecimal(source.Margin),
		Freshness:             account.Freshness,
		ReportObservedAt:      account.ReportObservedAt,
	}
	if source.ExpiryAt.Valid {
		t := source.ExpiryAt.Time
		row.ExpiryAt = &t
	}
	size, _ := normalizedPositionSize(&row)
	if size == nil || !source.QuoteAssetSymbol.Valid || !valuation.USDQuote(source.QuoteAssetSymbol.String) {
		return row, nil
	}
	qty, ok := decimal.Parse(*size)
	if !ok {
		return row, nil
	}
	if inverseKind(source.Kind) {
		// Inverse size is already the quote face value; no mark is required.
		row.NotionalUSD = decimal.FormatPtr(decimal.Abs(qty))
	} else if source.MarkPrice.Valid {
		if price, valid := decimal.Parse(source.MarkPrice.String); valid && price.Sign() > 0 {
			row.NotionalUSD = decimal.FormatPtr(decimal.Abs(decimal.Mul(qty, price)))
		}
	}
	return row, nil
}

// FlatPositions lists every position in the boundary at the latest point.
func (s *Service) FlatPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (PositionList, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (PositionList, error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return PositionList{}, err
		}
		c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
		if err != nil {
			return PositionList{}, err
		}
		out := PositionList{FlatList: FlatList[PositionRow]{Context: c, Items: []PositionRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}}
		out.Coverage = reportCoverage(accounts, f, "positions", point != nil)
		total := newHoldingTotal(accounts, f, "positions", point != nil)
		linear := newNotionalTotal()
		inverse := newNotionalTotal()
		out.setTotal(total)
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
			return out, nil
		}
		out.Completeness = valuation.CompletenessComplete
		for i := range accounts {
			account := &accounts[i]
			if positionAccount(account) && matchesAccount(f, account) {
				if account.Method == valuation.MethodMissing {
					out.Completeness = valuation.CompletenessIncomplete
				}
				out.Issues = append(out.Issues, positionAccountIssues(account)...)
			}
		}
		byAccount := indexAccounts(accounts)
		rows, err := mysql.Queries(ctx, s.db).ListPositionsByCycle(ctx, sqlcgen.ListPositionsByCycleParams{PortfolioID: p.ID[:], CycleID: point.CycleID[:]})
		if err != nil {
			return PositionList{}, err
		}
		all := make([]PositionRow, 0, len(rows))
		for i := range rows {
			r := &rows[i]
			accountID, err := uuid.FromBytes(r.AccountID)
			if err != nil {
				return PositionList{}, err
			}
			account, ok := byAccount[accountID]
			if !ok || !matchesAccount(f, account) || !matchesSymbol(f.Symbol, r.FuturesSymbol) {
				continue
			}
			row, err := positionRow(account, &sqlcgen.ListPositionsByReportRow{
				FuturesContractID:     r.FuturesContractID,
				Platform:              r.Platform,
				FuturesSymbol:         r.FuturesSymbol,
				Kind:                  r.Kind,
				BaseAssetSymbol:       r.BaseAssetSymbol,
				QuoteAssetSymbol:      r.QuoteAssetSymbol,
				SettlementAssetSymbol: r.SettlementAssetSymbol,
				ExpiryAt:              r.ExpiryAt,
				ContractSize:          r.ContractSize,
				PositionKey:           r.PositionKey,
				Side:                  r.Side,
				SourceQuantity:        r.SourceQuantity,
				SourceQuantityUnit:    r.SourceQuantityUnit,
				EntryPrice:            r.EntryPrice,
				MarkPrice:             r.MarkPrice,
				LiquidationPrice:      r.LiquidationPrice,
				UnrealizedPnl:         r.UnrealizedPnl,
				Leverage:              r.Leverage,
				Margin:                r.Margin,
			})
			if err != nil {
				return PositionList{}, err
			}
			completePositionFacts(&row, account, positionMetadata{sql.NullString{String: r.PositionMode, Valid: true}, r.MarginSetting, r.MarginAssetSymbol, r.MarginBasis})
			out.Issues = append(out.Issues, positionDataIssues(row.Issues)...)
			if row.NotionalUSD == nil {
				out.Completeness = valuation.CompletenessIncomplete
				total.completeness = valuation.CompletenessIncomplete
				for _, issue := range row.Issues {
					if issue.Code == "position_notional_unknown" {
						total.issues = append(total.issues, issue)
					}
				}
			}
			if row.NotionalUSD != nil {
				notional := decimal.ParseOrZero(*row.NotionalUSD)
				total.add(notional)
				if inverseKind(row.Kind) {
					inverse.add(notional)
				} else {
					linear.add(notional)
				}
			}
			all = append(all, row)
		}
		out.setTotal(total)
		complete := out.TotalCompleteness == valuation.CompletenessComplete
		out.TotalLinearUSD = linear.format(complete)
		out.TotalInverseUSD = inverse.format(complete)
		sortPositions(all, accountRanks(accounts))
		out.paginate(all)
		for i := range out.Items {
			completePositionAmounts(&out.Items[i])
		}
		return out, nil
	})
}

// inverseKind identifies contracts whose size is a quote face amount.
func inverseKind(kind string) bool {
	return kind == adapter.KindInversePerpetual || kind == adapter.KindInverseDelivery
}

// notionalTotal accumulates one contract family's absolute USD notional. The
// filtered total's completeness decides whether an empty accumulator is a
// confirmed zero or an unknown value.
type notionalTotal struct {
	value *big.Rat
	known bool
}

func newNotionalTotal() *notionalTotal {
	return &notionalTotal{value: decimal.Zero()}
}

func (t *notionalTotal) add(value *big.Rat) {
	if value == nil {
		return
	}
	t.value = decimal.Add(t.value, value)
	t.known = true
}

func (t *notionalTotal) format(complete bool) *string {
	if !t.known && !complete {
		return nil
	}
	return decimal.FormatPtr(t.value)
}
