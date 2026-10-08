package financial

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/application/listingexclusion"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// AccountBalances returns the Balances of an Account's current report with USD values.
func (s *Service) AccountBalances(ctx context.Context, p PortfolioContext, accountID uuid.UUID) (AccountView, error) {
	view, source, err := s.accountView(ctx, p, accountID)
	if err != nil {
		return AccountView{}, err
	}
	view.Balances = []BalanceRow{}

	ignored := map[uuid.UUID]bool{}
	if view.Account.Method == valuation.MethodCalculated && view.Context.JobID != nil {
		ignored, err = listingexclusion.ActiveForJob(ctx, s.db, p.ID, *view.Context.JobID)
		if err != nil {
			return AccountView{}, err
		}
	}
	var rows []sqlcgen.ListBalancesByReportRow
	if source != nil {
		rows, err = s.q.ListBalancesByReport(ctx, source[:])
		if err != nil {
			return AccountView{}, err
		}
	}
	book := valuation.NewPriceBook(s.q)
	at := time.Time{}
	if view.Account.ReportObservedAt != nil {
		at = *view.Account.ReportObservedAt
	}
	for i := range rows {
		listingID, err := uuid.FromBytes(rows[i].ListingID)
		if err != nil {
			return AccountView{}, err
		}
		if ignored[listingID] && view.Account.AccountType != string(adapters.TypeManual) {
			continue
		}
		row, err := balanceRow(ctx, book, &view.Account, at, &rows[i])
		if err != nil {
			return AccountView{}, err
		}
		if ignored[listingID] {
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
}

func balanceRow(ctx context.Context, book *valuation.PriceBook, account *valuation.AccountValuation, at time.Time, source *sqlcgen.ListBalancesByReportRow) (BalanceRow, error) {
	id, err := uuid.FromBytes(source.ListingID)
	if err != nil {
		return BalanceRow{}, err
	}
	row := BalanceRow{
		AccountID:           account.AccountID,
		AccountName:         account.AccountName,
		AccountType:         account.AccountType,
		IdentityID:          account.IdentityID,
		IdentityName:        account.IdentityName,
		Platform:            source.Platform,
		ListingID:           id,
		Symbol:              source.Symbol,
		AssetSymbol:         source.AssetSymbol,
		Equity:              decimal.Compact(source.Equity),
		AvailableToWithdraw: nullDecimal(source.AvailableToWithdraw),
		AvailableToTrade:    nullDecimal(source.AvailableToTrade),
		Liability:           nullDecimal(source.Liability),
		Interest:            nullDecimal(source.Interest),
		UnrealizedPnl:       nullDecimal(source.UnrealizedPnl),
		Freshness:           account.Freshness,
		ReportObservedAt:    account.ReportObservedAt,
		ManualModifiedAt:    account.ManualModifiedAt,
	}
	quantity, ok := decimal.Parse(source.Equity)
	if !ok {
		row.NoPrice = true
		return row, nil
	}
	lookup := book.Lookup
	if account.AccountType == string(adapters.TypeManual) {
		lookup = book.LookupPlatform
	}
	price, found, err := lookup(ctx, source.Platform, source.AssetSymbol, at)
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
	out := FlatList[BalanceRow]{Context: c, Items: []BalanceRow{}, Offset: offset, Limit: limit, Completeness: valuation.CompletenessIncomplete, Issues: []valuation.Issue{}}
	if point == nil {
		out.Issues = append(out.Issues, valuation.Issue{Code: "no_point", Detail: "No Collection Cycle has produced a Valuation Point yet."})
	} else {
		out.Completeness = point.Completeness()
		out.Issues = point.Issues
	}
	accounts, err = s.withManualAccounts(ctx, p, accounts)
	if err != nil {
		return FlatList[BalanceRow]{}, err
	}
	ignored := map[uuid.UUID]bool{}
	var rows []sqlcgen.ListBalancesByJobRow
	if point != nil {
		ignored, err = listingexclusion.ActiveForJob(ctx, s.db, p.ID, point.JobID)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		rows, err = s.q.ListBalancesByJob(ctx, sqlcgen.ListBalancesByJobParams{PortfolioID: p.ID[:], JobID: point.JobID[:]})
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
	}
	byAccount := indexAccounts(accounts)

	book := valuation.NewPriceBook(s.q)
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
		listingID, err := uuid.FromBytes(rows[i].ListingID)
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		if account.Method == valuation.MethodCalculated && ignored[listingID] && account.AccountType != string(adapters.TypeManual) {
			continue
		}
		at := time.Time{}
		if account.ReportObservedAt != nil {
			at = *account.ReportObservedAt
		}
		row, err := balanceRow(ctx, book, account, at, &sqlcgen.ListBalancesByReportRow{
			ListingID:           rows[i].ListingID,
			Platform:            rows[i].Platform,
			Symbol:              rows[i].Symbol,
			AssetSymbol:         rows[i].AssetSymbol,
			Equity:              rows[i].Equity,
			AvailableToWithdraw: rows[i].AvailableToWithdraw,
			AvailableToTrade:    rows[i].AvailableToTrade,
			Liability:           rows[i].Liability,
			Interest:            rows[i].Interest,
			UnrealizedPnl:       rows[i].UnrealizedPnl,
		})
		if err != nil {
			return FlatList[BalanceRow]{}, err
		}
		if account.AccountType == string(adapters.TypeManual) {
			if ignored[listingID] {
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
		if !matchesAccount(f, byAccount[row.AccountID]) || !matchesSymbol(f.Symbol, row.AssetSymbol, row.Symbol) {
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
}

func excludeManualBalance(row *BalanceRow) {
	row.Excluded = true
	row.PriceUSD, row.EquityUSD, row.equityUSDRat = nil, nil, nil
	row.NoPrice = false
}
