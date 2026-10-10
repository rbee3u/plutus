package valuation

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Scoped foreign keys establish ownership. Validate semantic evidence in the
// same transaction that commits all parts of the point.
func validateEvidence(ctx context.Context, q *sqlcgen.Queries, portfolioID uuid.UUID, account *AccountValuation, prices map[uuid.UUID]sqlcgen.PriceObservation) error {
	if account.SourceReportID == nil {
		if len(account.BalanceValuations) != 0 || account.Method != MethodMissing || account.ValueUSD != nil {
			return errors.New("missing report cannot own balance valuations")
		}
		return nil
	}
	source, err := q.GetReport(ctx, sqlcgen.GetReportParams{PortfolioID: portfolioID[:], ID: account.SourceReportID[:]})
	if err != nil {
		return err
	}
	if source.Status != adapters.StatusSucceeded || !bytes.Equal(source.AccountID, account.AccountID[:]) {
		return errors.New("valuation source must be a successful report of the same Account")
	}
	if (account.Method == MethodReported) != source.ReportedEquityUsd.Valid || account.Method == MethodMissing {
		return errors.New("valuation method contradicts source equity")
	}
	if account.Method == MethodReported && (account.ValueUSD == nil || decimal.Compact(*account.ValueUSD) != decimal.Compact(source.ReportedEquityUsd.String)) {
		return errors.New("reported valuation changed source equity")
	}
	balances, err := q.ListBalancesByReport(ctx, sqlcgen.ListBalancesByReportParams{PortfolioID: portfolioID[:], ReportID: account.SourceReportID[:]})
	if err != nil {
		return err
	}
	if len(balances) != len(account.BalanceValuations) {
		return errors.New("balance valuation coverage differs from source report")
	}
	assets := map[uuid.UUID]sqlcgen.ListBalancesByReportRow{}
	for _, balance := range balances {
		id, err := uuid.FromBytes(balance.AssetID)
		if err != nil {
			return err
		}
		assets[id] = balance
	}
	for _, line := range account.BalanceValuations {
		balance, exists := assets[line.AssetID]
		if !exists {
			return errors.New("valuation references an asset outside its source report")
		}
		if (line.Treatment == "descriptive") != (account.Method == MethodReported) {
			return errors.New("balance treatment contradicts account method")
		}
		quantity, ok := decimal.Parse(balance.Equity)
		if !ok {
			return errors.New("invalid source quantity")
		}
		if line.PricingMethod == "zero_equity" && quantity.Sign() != 0 {
			return errors.New("nonzero balance uses zero-quantity pricing")
		}
		if line.PricingMethod == "usd_parity" && (line.ValueUSD == nil || decimal.Compact(*line.ValueUSD) != decimal.Format(quantity)) {
			return errors.New("parity value differs from source quantity")
		}
		if line.PriceObservationID == nil {
			continue
		}
		id := *line.PriceObservationID
		price, found := prices[id]
		if !found {
			price, err = q.GetPriceObservation(ctx, sqlcgen.GetPriceObservationParams{PortfolioID: portfolioID[:], ID: id[:]})
			if err != nil {
				return err
			}
			prices[id] = price
		}
		cutoff := source.RecordedAt
		if price.Platform != account.Platform || price.RequestedCanonicalAssetSymbol != line.CanonicalAssetSymbol || !USDQuote(price.QuoteCanonicalAssetSymbol) || price.RetrievedAt.After(cutoff) || price.PriceAt.Time.After(cutoff) || cutoff.Sub(price.PriceAt.Time) > maxPriceAge {
			return errors.New("balance valuation references ineligible price evidence")
		}
		priceValue, ok := decimal.Parse(price.Price)
		if !ok || priceValue.Sign() <= 0 || line.ValueUSD == nil || decimal.Compact(*line.ValueUSD) != decimal.Format(decimal.Mul(quantity, priceValue)) {
			return errors.New("market valuation differs from adopted price and source quantity")
		}
	}
	return nil
}
