package financial

import (
	"context"
	"database/sql"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
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
	view.Positions = []PositionRow{}
	if source == nil {
		return view, nil
	}
	rows, err := mysql.Queries(ctx, s.db).ListPositionsByReport(ctx, sqlcgen.ListPositionsByReportParams{PortfolioID: p.ID[:], ReportID: source[:]})
	if err != nil {
		return AccountView{}, err
	}
	for i := range rows {
		r := &rows[i]
		row, err := positionRow(&view.Account, r)
		if err != nil {
			return AccountView{}, err
		}
		completePosition(&row, &view.Account, positionMetadata{sql.NullString{String: r.PositionMode, Valid: true}, r.MarginSetting, r.MarginAssetSymbol, r.MarginBasis})
		view.Issues = append(view.Issues, positionDataIssues(row.Issues)...)
		view.Positions = append(view.Positions, row)
	}
	sortPositions(view.Positions, nil)
	return view, nil
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
func (s *Service) FlatPositions(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[PositionRow], error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (FlatList[PositionRow], error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return FlatList[PositionRow]{}, err
		}
		c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
		if err != nil {
			return FlatList[PositionRow]{}, err
		}
		out := FlatList[PositionRow]{Context: c, Items: []PositionRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
		out.Coverage = reportCoverage(accounts, f, "positions", point != nil)
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
			return out, nil
		}
		out.Completeness = valuation.CompletenessComplete
		for i := range accounts {
			account := &accounts[i]
			if positionAccount(account) {
				if account.Method == valuation.MethodMissing {
					out.Completeness = valuation.CompletenessIncomplete
				}
				out.Issues = append(out.Issues, positionAccountIssues(account)...)
			}
		}
		byAccount := indexAccounts(accounts)
		rows, err := mysql.Queries(ctx, s.db).ListPositionsByCycle(ctx, sqlcgen.ListPositionsByCycleParams{PortfolioID: p.ID[:], CycleID: point.CycleID[:]})
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
				return FlatList[PositionRow]{}, err
			}
			completePositionFacts(&row, account, positionMetadata{sql.NullString{String: r.PositionMode, Valid: true}, r.MarginSetting, r.MarginAssetSymbol, r.MarginBasis})
			out.Issues = append(out.Issues, positionDataIssues(row.Issues)...)
			if row.NotionalUSD == nil {
				out.Completeness = valuation.CompletenessIncomplete
			}
			if row.NotionalUSD != nil {
				total = decimal.Add(total, decimal.ParseOrZero(*row.NotionalUSD))
			}
			if !matchesAccount(f, account) || !matchesSymbol(f.Symbol, r.FuturesSymbol) {
				continue
			}
			all = append(all, row)
		}
		out.TotalUSD = decimal.FormatPtr(total)
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
	return kind == adapters.KindInversePerpetual || kind == adapters.KindInverseDelivery
}
