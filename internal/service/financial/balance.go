package financial

import (
	"context"
	"errors"
	"math/big"

	"github.com/rbee3u/plutus/internal/infra/mysql"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/valuation"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// AccountBalances returns the balances of an account's current report with USD values.
func (s *Service) AccountBalances(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	return s.AccountBalancesAt(ctx, p, accountID, nil)
}

func (s *Service) AccountBalancesAt(ctx context.Context, p PortfolioContext, accountID uuid.UUID, cycleID *uuid.UUID) (AccountView, error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (AccountView, error) {
		view, source, err := s.accountView(ctx, p, accountID, cycleID)
		if err != nil {
			return AccountView{}, err
		}
		return s.readAccountBalances(ctx, p, view, source, true)
	})
}

func (s *Service) readAccountBalances(ctx context.Context, p PortfolioContext, view AccountView, source *uuid.UUID, includePending bool) (AccountView, error) {
	view.Balances = []BalanceRow{}

	var rows []sqlcgen.ListBalancesByReportRow
	var err error
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
	decisions := balanceDecisions(&view.Account)
	for i := range rows {
		assetID, err := uuid.FromBytes(rows[i].AssetID)
		if err != nil {
			return AccountView{}, err
		}
		if isExcluded(decisions[assetID]) && view.Account.AccountType != string(adapter.TypeManual) {
			continue
		}
		row, err := balanceRow(ctx, book, &view.Account, &rows[i], decisions[assetID])
		if err != nil {
			return AccountView{}, err
		}
		if isExcluded(decisions[assetID]) {
			excludeManualBalance(&row)
		}
		view.Balances = append(view.Balances, row)
	}
	if includePending && view.Account.AccountType == string(adapter.TypeManual) {
		current, overlay, err := s.manualOverlayBalances(ctx, p, map[uuid.UUID]bool{view.Account.AccountID: true})
		if err != nil {
			return AccountView{}, err
		}
		view.ManualOverlay = overlay
		view.Balances = mergeManualBalances(view.Balances, current[view.Account.AccountID], &view.Account)
	}
	sortBalances(view.Balances, accountRanks([]valuation.AccountValuation{view.Account}))
	return view, nil
}

func balanceRow(ctx context.Context, book *valuation.PriceBook, account *valuation.AccountValuation, source *sqlcgen.ListBalancesByReportRow, adopted *valuation.BalanceValuation) (BalanceRow, error) {
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

func balanceDecisions(account *valuation.AccountValuation) map[uuid.UUID]*valuation.BalanceValuation {
	out := make(map[uuid.UUID]*valuation.BalanceValuation, len(account.BalanceValuations))
	for i := range account.BalanceValuations {
		v := &account.BalanceValuations[i]
		out[v.AssetID] = v
	}
	return out
}

func isExcluded(v *valuation.BalanceValuation) bool { return v != nil && v.Treatment == "excluded" }

// FlatBalances lists every balance in the boundary at the latest point.
func (s *Service) FlatBalances(ctx context.Context, p PortfolioContext, f *ListFilter) (FlatList[BalanceRow], error) {
	return mysql.WithReadContext(ctx, s.db, func(ctx context.Context) (FlatList[BalanceRow], error) {
		limit, offset, err := normalizePage(f.Limit, f.Offset)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		c, point, accounts, err := s.contextAt(ctx, p, f.CycleID)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		out := FlatList[BalanceRow]{TotalCompleteness: valuation.CompletenessIncomplete, TotalIssues: []valuation.Issue{}, Context: c, Items: []BalanceRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
		if point == nil {
			out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No collection cycle has produced a valuation point yet."})
			out.TotalIssues = append(out.TotalIssues, out.Issues[0])
		} else {
			out.Completeness = point.Completeness()
			out.Issues = point.Issues
		}
		out.Coverage = reportCoverage(accounts, f, "", point != nil)
		total := newHoldingTotal(accounts, f, "", point != nil)
		if f.IncludePendingManual {
			accounts, err = s.withManualAccounts(ctx, p, accounts)
		}
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		var rows []sqlcgen.ListBalancesByCycleRow
		if point != nil {
			rows, err = mysql.Queries(ctx, s.db).ListBalancesByCycle(ctx, sqlcgen.ListBalancesByCycleParams{PortfolioID: p.ID[:], CycleID: point.CycleID[:], Platform: f.Platform, IdentityID: optionalFilterID(f.IdentityID), Symbol: normalizedSearch(f.Symbol)})
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
		}
		byAccount := indexAccounts(accounts)
		decisions := make(map[uuid.UUID]map[uuid.UUID]*valuation.BalanceValuation, len(accounts))
		selectedManual := map[uuid.UUID]bool{}
		filteredAccounts := []valuation.AccountValuation{}
		for i := range accounts {
			a := &accounts[i]
			if !matchesAccount(f, a) {
				continue
			}
			decisions[a.AccountID] = balanceDecisions(a)
			filteredAccounts = append(filteredAccounts, *a)
			if a.AccountType == string(adapter.TypeManual) {
				selectedManual[a.AccountID] = true
			}
		}

		book := valuation.NewPriceBook(mysql.Queries(ctx, s.db), p.ID)
		if err := book.PreloadObservations(ctx, filteredAccounts); err != nil {
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
			if !ok || !matchesAccount(f, account) {
				continue
			}
			assetID, err := uuid.FromBytes(rows[i].AssetID)
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
			if account.Method == valuation.MethodCalculated && isExcluded(decisions[accountID][assetID]) && account.AccountType != string(adapter.TypeManual) {
				continue
			}
			row, err := balanceRow(ctx, book, account, &sqlcgen.ListBalancesByReportRow{
				AssetID:              rows[i].AssetID,
				Platform:             rows[i].Platform,
				AssetSymbol:          rows[i].AssetSymbol,
				CanonicalAssetSymbol: rows[i].CanonicalAssetSymbol,
				Equity:               rows[i].Equity,
			}, decisions[accountID][assetID])
			if err != nil {
				return FlatList[BalanceRow]{}, err
			}
			if account.AccountType == string(adapter.TypeManual) {
				if isExcluded(decisions[accountID][assetID]) {
					excludeManualBalance(&row)
				}
				manualRows[accountID] = append(manualRows[accountID], row)
			} else {
				all = append(all, row)
			}
		}
		current := map[uuid.UUID][]BalanceRow{}
		if f.IncludePendingManual {
			current, out.ManualOverlay, err = s.manualOverlayBalances(ctx, p, selectedManual)
		}
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		for i := range accounts {
			account := &accounts[i]
			if account.AccountType == string(adapter.TypeManual) && matchesAccount(f, account) {
				manual := manualRows[account.AccountID]
				if f.IncludePendingManual {
					manual = mergeManualBalances(manual, current[account.AccountID], account)
				}
				all = append(all, manual...)
			}
		}
		filtered := all[:0]
		for _, row := range all {
			if !matchesAccount(f, byAccount[row.AccountID]) || !matchesSymbol(f.Symbol, row.CanonicalAssetSymbol, row.AssetSymbol) {
				continue
			}
			filtered = append(filtered, row)
			if row.EquityUSD == nil && !row.Excluded && row.ManualChange != "added" {
				total.completeness = valuation.CompletenessIncomplete
				code := "balance_value_unknown"
				if row.NoPrice {
					code = "no_price"
				}
				omitted := &valuation.Omitted{AssetID: row.AssetID, AssetSymbol: row.AssetSymbol, Equity: row.Equity, Reason: code}
				if row.CanonicalAssetSymbol != row.AssetSymbol {
					omitted.CanonicalAssetSymbol = row.CanonicalAssetSymbol
				}
				total.issues = append(total.issues, valuation.Issue{Code: code, AccountID: &row.AccountID, AccountName: row.AccountName, Symbol: row.AssetSymbol, OmittedBalance: omitted, ReportObservedAt: row.ReportObservedAt, Detail: "The balance has no known USD value and is omitted from the filtered total."})
			}
			if row.equityUSDRat != nil {
				total.add(row.equityUSDRat)
			}
		}
		all = filtered
		out.setTotal(total)
		sortBalances(all, accountRanks(accounts))
		out.paginate(all)
		if err := s.balancePageDetails(ctx, p, &out, byAccount); err != nil {
			return FlatList[BalanceRow]{}, err
		}
		return out, nil
	})
}

func excludeManualBalance(row *BalanceRow) {
	row.Excluded = true
	row.PriceUSD, row.EquityUSD, row.equityUSDRat = nil, nil, nil
	row.NoPrice = false
}

func (s *Service) balancePageDetails(ctx context.Context, p PortfolioContext, out *FlatList[BalanceRow], accounts map[uuid.UUID]*valuation.AccountValuation) error {
	reports, assets := [][]byte{}, [][]byte{}
	type key struct{ report, asset uuid.UUID }
	page := map[key]*BalanceRow{}
	for i := range out.Items {
		row := &out.Items[i]
		a := accounts[row.AccountID]
		if row.ManualChange == "added" || a == nil || a.SourceReportID == nil {
			continue
		}
		reports = append(reports, a.SourceReportID[:])
		assets = append(assets, row.AssetID[:])
		page[key{*a.SourceReportID, row.AssetID}] = row
	}
	if len(reports) == 0 {
		return nil
	}
	rows, err := mysql.Queries(ctx, s.db).ListBalanceDetailsByReportsAndAssets(ctx, sqlcgen.ListBalanceDetailsByReportsAndAssetsParams{PortfolioID: p.ID[:], ReportIds: reports, AssetIds: assets})
	if err != nil {
		return err
	}
	for _, source := range rows {
		report, err := uuid.FromBytes(source.ReportID)
		if err != nil {
			return err
		}
		asset, err := uuid.FromBytes(source.AssetID)
		if err != nil {
			return err
		}
		if row := page[key{report, asset}]; row != nil {
			row.AvailableToWithdraw, row.AvailableToTrade = nullDecimal(source.AvailableToWithdraw), nullDecimal(source.AvailableToTrade)
			row.Liability, row.Interest, row.UnrealizedPnl = nullDecimal(source.Liability), nullDecimal(source.Interest), nullDecimal(source.UnrealizedPnl)
		}
	}
	return nil
}
