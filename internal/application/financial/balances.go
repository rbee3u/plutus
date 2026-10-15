package financial

import (
	"context"
	"errors"
	"math/big"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// AccountBalances returns the balances of an account's current report with USD values.
func (s *Service) AccountBalances(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, source, err := s.accountView(ctx, p, accountID)
		if err != nil {
			return AccountView{}, err
		}
		view.Balances = []BalanceRow{}

		var rows []sqlcgen.ListBalancesByReportRow
		if source != nil {
			rows, err = mysql.Queries(ctx, s.db).ListBalancesByReport(ctx, sqlcgen.ListBalancesByReportParams{PortfolioID: p.ID[:], ReportID: source[:]})
			if err != nil {
				return AccountView{}, err
			}
		}
		book := valuation.NewPriceBook(mysql.Queries(ctx, s.db), p.ID)
		if err := book.PreloadObservations(ctx, []valuation.AccountValuation{view.Account}); err != nil {
			return AccountView{}, err
		}
		for i := range rows {
			assetID, err := uuid.FromBytes(rows[i].AssetID)
			if err != nil {
				return AccountView{}, err
			}
			if balanceExcluded(&view.Account, assetID) && view.Account.AccountType != string(adapters.TypeManual) {
				continue
			}
			row, err := balanceRow(ctx, book, &view.Account, &rows[i])
			if err != nil {
				return AccountView{}, err
			}
			if balanceExcluded(&view.Account, assetID) {
				excludeManualBalance(&row)
			}
			view.Balances = append(view.Balances, row)
		}
		if view.Account.AccountType == string(adapters.TypeManual) {
			current, err := s.currentManualBalances(ctx, p)
			if err != nil {
				return AccountView{}, err
			}
			view.Balances = mergeManualBalances(view.Balances, current[accountID], &view.Account)
		}
		sortBalances(view.Balances, accountRanks([]valuation.AccountValuation{view.Account}))
		return view, nil
	})
}

func balanceRow(ctx context.Context, book *valuation.PriceBook, account *valuation.AccountValuation, source *sqlcgen.ListBalancesByReportRow) (BalanceRow, error) {
	id, err := uuid.FromBytes(source.AssetID)
	if err != nil {
		return BalanceRow{}, err
	}
	row := BalanceRow{
		AccountID:            account.AccountID,
		AccountName:          account.AccountName,
		AccountType:          account.AccountType,
		IdentityID:           account.IdentityID,
		IdentityName:         account.IdentityName,
		Platform:             source.Platform,
		AssetID:              id,
		AssetSymbol:          source.AssetSymbol,
		CanonicalAssetSymbol: source.CanonicalAssetSymbol,
		Equity:               decimal.Compact(source.Equity),
		AvailableToWithdraw:  nullDecimal(source.AvailableToWithdraw),
		AvailableToTrade:     nullDecimal(source.AvailableToTrade),
		Liability:            nullDecimal(source.Liability),
		Interest:             nullDecimal(source.Interest),
		UnrealizedPnl:        nullDecimal(source.UnrealizedPnl),
		Freshness:            account.Freshness,
		ReportObservedAt:     account.ReportObservedAt,
		ManualModifiedAt:     account.ManualModifiedAt,
	}
	// Outside-boundary source reports have no adopted value at this point.
	if account.Method == valuation.MethodMissing {
		return row, nil
	}
	var adopted *valuation.BalanceValuation
	for i := range account.BalanceValuations {
		if account.BalanceValuations[i].AssetID == id {
			adopted = &account.BalanceValuations[i]
			break
		}
	}
	if adopted == nil {
		return BalanceRow{}, errors.New("balance lacks persisted valuation evidence")
	}
	row.CanonicalAssetSymbol = adopted.CanonicalAssetSymbol
	if adopted.Treatment == "excluded" {
		excludeManualBalance(&row)
		return row, nil
	}
	row.EquityUSD = adopted.ValueUSD
	if adopted.ValueUSD == nil {
		row.NoPrice = adopted.MissingReason == "no_price"
		return row, nil
	}
	quantity, ok := decimal.Parse(source.Equity)
	if !ok {
		return BalanceRow{}, errors.New("invalid source balance quantity")
	}
	var price *big.Rat
	switch adopted.PricingMethod {
	case "usd_parity":
		price = big.NewRat(1, 1)
	case "zero_equity":
		row.equityUSDRat = decimal.Zero()
		return row, nil
	case "market_price":
		if adopted.PriceObservationID == nil {
			return BalanceRow{}, errors.New("missing adopted price")
		}
		price, err = book.Observation(ctx, *adopted.PriceObservationID)
		if err != nil {
			return BalanceRow{}, err
		}
	default:
		return BalanceRow{}, errors.New("invalid adopted pricing method")
	}
	row.PriceUSD = decimal.FormatPtr(price)
	row.equityUSDRat = decimal.Mul(quantity, price)
	return row, nil
}

func balanceExcluded(account *valuation.AccountValuation, id uuid.UUID) bool {
	for _, line := range account.BalanceValuations {
		if line.AssetID == id {
			return line.Treatment == "excluded"
		}
	}
	return false
}

// FlatBalances lists every balance in the boundary at the latest point.
func (s *Service) FlatBalances(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[BalanceRow], error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (FlatList[BalanceRow], error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		c, point, accounts, err := s.latestContext(ctx, p)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		out := FlatList[BalanceRow]{Context: c, Items: []BalanceRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
		} else {
			out.Completeness = point.Completeness()
			out.Issues = point.Issues
		}
		accounts, err = s.withManualAccounts(ctx, p, accounts)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		var rows []sqlcgen.ListBalancesByCycleRow
		if point != nil {
			rows, err = mysql.Queries(ctx, s.db).ListBalancesByCycle(ctx, sqlcgen.ListBalancesByCycleParams{PortfolioID: p.ID[:], CycleID: point.CycleID[:]})
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
		}
		byAccount := indexAccounts(accounts)

		book := valuation.NewPriceBook(mysql.Queries(ctx, s.db), p.ID)
		if err := book.PreloadObservations(ctx, accounts); err != nil {
			return FlatList[BalanceRow]{}, err
		}
		all := make([]BalanceRow, 0, len(rows))
		manualRows := make(map[uuid.UUID][]BalanceRow)
		for i := range rows {
			accountID, err := uuid.FromBytes(rows[i].AccountID)
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
			account, ok := byAccount[accountID]
			if !ok {
				continue
			}
			assetID, err := uuid.FromBytes(rows[i].AssetID)
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
			if account.Method == valuation.MethodCalculated && balanceExcluded(account, assetID) && account.AccountType != string(adapters.TypeManual) {
				continue
			}
			row, err := balanceRow(ctx, book, account, &sqlcgen.ListBalancesByReportRow{
				AssetID:              rows[i].AssetID,
				Platform:             rows[i].Platform,
				AssetSymbol:          rows[i].AssetSymbol,
				CanonicalAssetSymbol: rows[i].CanonicalAssetSymbol,
				Equity:               rows[i].Equity,
				AvailableToWithdraw:  rows[i].AvailableToWithdraw,
				AvailableToTrade:     rows[i].AvailableToTrade,
				Liability:            rows[i].Liability,
				Interest:             rows[i].Interest,
				UnrealizedPnl:        rows[i].UnrealizedPnl,
			})
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
			if account.AccountType == string(adapters.TypeManual) {
				if balanceExcluded(account, assetID) {
					excludeManualBalance(&row)
				}
				manualRows[accountID] = append(manualRows[accountID], row)
			} else {
				all = append(all, row)
			}
		}
		current, err := s.currentManualBalances(ctx, p)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		for i := range accounts {
			account := &accounts[i]
			if account.AccountType == string(adapters.TypeManual) {
				all = append(all, mergeManualBalances(manualRows[account.AccountID], current[account.AccountID], account)...)
			}
		}
		filtered := all[:0]
		total := decimal.Zero()
		for _, row := range all {
			if !matchesAccount(f, byAccount[row.AccountID]) || !matchesSymbol(f.Symbol, row.CanonicalAssetSymbol, row.AssetSymbol) {
				continue
			}
			filtered = append(filtered, row)
			if row.equityUSDRat != nil {
				total = decimal.Add(total, row.equityUSDRat)
			}
		}
		all = filtered
		if point != nil {
			out.TotalUSD = decimal.FormatPtr(total)
		}
		sortBalances(all, accountRanks(accounts))
		out.paginate(all)
		return out, nil
	})
}

func excludeManualBalance(row *BalanceRow) {
	row.Excluded = true
	row.PriceUSD, row.EquityUSD, row.equityUSDRat = nil, nil, nil
	row.NoPrice = false
}
