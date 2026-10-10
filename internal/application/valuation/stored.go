package valuation

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Stored decoding is shared by financial reads and valuation input preparation.
func nullDecimal(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(decimal.Compact(v.String))
}

func reportFromRow(row *sqlcgen.ListReportsByCycleRow) (AccountValuation, error) {
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
		ReportID: reportID,

		ManualModifiedAt: manualTime(row.ManualChangedAt),
		AccountID:        accountID,
		AccountName:      adapters.AccountName(adapters.AccountType(row.AccountType)),
		AccountType:      row.AccountType,
		AccountStructure: row.AccountStructure,
		Selected:         row.Selected,
		IdentityID:       identityID,
		IdentityName:     row.IdentityName,
		IdentityStatus:   "enabled",
		ConnectionStatus: row.ConnectionStatus,
		Platform:         row.Platform,
		Status:           row.Status,
		ErrorClass:       nullStringPtr(row.ErrorClass),
		ErrorDetail:      nullStringPtr(row.ErrorDetail),
		Method:           row.ValuationMethod.String,
		Freshness:        FreshnessStale,
		ValueUSD:         nullDecimal(row.ValueUsd),
	}
	if row.Status == adapters.StatusSucceeded {
		item.SourceMarginMode = nullStringPtr(row.MarginMode)
		item.Freshness = FreshnessFresh
		item.Equity = nullDecimal(row.ReportedEquityUsd)
		item.AdjustedEquity = nullDecimal(row.AdjustedEquityUsd)
		item.AvailableEquity = nullDecimal(row.AvailableEquityUsd)
		item.InitialMarginRatio = MarginRatio(row.ReportedInitialMarginRatio, row.InitialMarginRequirementUsd, row.AdjustedEquityUsd)
		item.MaintenanceMarginRatio = MarginRatio(row.ReportedMaintenanceMarginRatio, row.MaintenanceMarginRequirementUsd, row.AdjustedEquityUsd)
		item.MarginRatio = nullDecimal(row.ReportedMarginRatio)
		if row.CollectedAt.Valid {
			t := effectiveObservedAt(row.CollectedAt, row.SourceObservedAt)
			item.ReportObservedAt = &t
		}
	} else {
		item.SourceMarginMode = nullStringPtr(row.SourceMarginMode)
		item.Equity = nullDecimal(row.SourceEquity)
		item.AdjustedEquity = nullDecimal(row.SourceAdjustedEquity)
		item.AvailableEquity = nullDecimal(row.SourceAvailableEquity)
		item.InitialMarginRatio = MarginRatio(row.SourceInitialMarginRatio, row.SourceInitialMarginRequirementUsd, row.SourceAdjustedEquity)
		item.MaintenanceMarginRatio = MarginRatio(row.SourceMaintenanceMarginRatio, row.SourceMaintenanceMarginRequirementUsd, row.SourceAdjustedEquity)
		item.MarginRatio = nullDecimal(row.SourceMarginRatio)
		if row.EffectiveSourceObservedAt.Valid {
			t := row.EffectiveSourceObservedAt.Time
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
	if row.IdentityArchivedAt.Valid {
		item.IdentityStatus = "archived"
	}
	if item.Method == "" {
		item.Method = MethodMissing
	}
	return item, nil
}

func appendStoredBalance(item *AccountValuation, line *sqlcgen.ListBalanceValuationsByCyclesRow) error {
	id, err := uuid.FromBytes(line.AssetID)
	if err != nil {
		return err
	}
	v := BalanceValuation{AssetID: id, CanonicalAssetSymbol: line.CanonicalAssetSymbol, PricingMethod: line.PricingMethod, ValueUSD: nullDecimal(line.ValueUsd), Treatment: "included"}
	if item.Method == MethodReported {
		v.Treatment = "descriptive"
	} else if line.PricingMethod == "excluded" {
		v.Treatment = "excluded"
	}
	if len(line.PriceObservationID) != 0 {
		id, err := uuid.FromBytes(line.PriceObservationID)
		if err != nil {
			return err
		}
		v.PriceObservationID = &id
	}
	if v.PricingMethod == "unavailable" {
		v.MissingReason = "no_price"
	}
	item.BalanceValuations = append(item.BalanceValuations, v)
	if item.Method == MethodCalculated && v.Treatment == "included" && v.MissingReason != "" {
		item.Omitted = append(item.Omitted, Omitted{
			AssetID:              id,
			AssetSymbol:          line.AssetSymbol,
			CanonicalAssetSymbol: mappedCanonicalAssetSymbol(line.AssetSymbol, line.CanonicalAssetSymbol),
			Equity:               decimal.Compact(line.Equity), Reason: v.MissingReason,
		})
	}
	return nil
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}

// MarginRatio prefers a reported ratio and otherwise derives it from source facts.
func MarginRatio(reported, requirement, equity sql.NullString) *string {
	if reported.Valid {
		return nullDecimal(reported)
	}
	if !requirement.Valid || !equity.Valid {
		return nil
	}
	numerator, _ := decimal.Parse(requirement.String)
	denominator, _ := decimal.Parse(equity.String)
	if denominator == nil || denominator.Sign() <= 0 {
		return nil
	}
	return adapters.Ratio(numerator, denominator)
}

func validateBalanceValuations(lines []BalanceValuation) error {
	seen := map[uuid.UUID]bool{}
	for _, line := range lines {
		if line.AssetID == uuid.Nil || seen[line.AssetID] || line.CanonicalAssetSymbol == "" {
			return errors.New("invalid balance valuation identity")
		}
		seen[line.AssetID] = true
		if line.Treatment != "included" && line.Treatment != "excluded" && line.Treatment != "descriptive" {
			return errors.New("invalid balance treatment")
		}
		if line.ValueUSD != nil && !decimal.Valid(*line.ValueUSD) {
			return errors.New("invalid balance value")
		}
		switch line.PricingMethod {
		case "market_price", "usd_parity", "zero_equity", "unavailable", "excluded":
		default:
			return errors.New("invalid balance pricing method")
		}
		if line.PricingMethod != "market_price" && line.PriceObservationID != nil {
			return errors.New("non-market value references a price")
		}
		if line.PricingMethod == "usd_parity" && !USDQuote(line.CanonicalAssetSymbol) {
			return errors.New("non-USD asset uses parity")
		}
		if line.ValueUSD != nil && line.MissingReason != "" {
			return errors.New("contradictory balance valuation")
		}
		if line.PricingMethod == "market_price" && (line.PriceObservationID == nil || line.ValueUSD == nil) {
			return errors.New("market valuation lacks evidence")
		}
		if (line.Treatment == "excluded") != (line.PricingMethod == "excluded") || (line.PricingMethod == "excluded" && line.ValueUSD != nil) ||
			(line.PricingMethod == "unavailable" && line.ValueUSD != nil) ||
			(line.PricingMethod == "usd_parity" && line.ValueUSD == nil) ||
			(line.PricingMethod == "zero_equity" && (line.ValueUSD == nil || decimal.Compact(*line.ValueUSD) != "0")) {
			return errors.New("balance value contradicts its pricing method")
		}
		if line.Treatment != "excluded" && line.ValueUSD == nil && line.MissingReason == "" {
			return errors.New("unknown balance valuation lacks reason")
		}
	}
	return nil
}

func effectiveObservedAt(collected, source sql.NullTime) time.Time {
	if source.Valid {
		return source.Time
	}
	return collected.Time
}
