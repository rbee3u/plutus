package valuation

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/shared/decimal"
)

// Stored decoding is shared by financial reads and valuation input preparation.
func nullDecimal(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(decimal.Compact(v.String))
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
		AccountStructure: row.AccountStructure,
		Selected:         row.Selected,
		IdentityID:       identityID,
		IdentityName:     row.IdentityName,
		IdentityStatus:   row.IdentityStatus,
		ConnectionStatus: row.ConnectionStatus,
		Platform:         row.Platform,
		Status:           row.Status,
		ErrorClass:       nullStringPtr(row.ErrorClass),
		ErrorDetail:      nullStringPtr(row.ErrorDetail),
		Method:           row.Method,
		Freshness:        FreshnessStale,
		ValueUSD:         nullDecimal(row.ValueUsd),
	}
	if row.Status == "ok" {
		item.SourceMarginMode = nullStringPtr(row.MarginMode)
		item.Freshness = FreshnessFresh
		item.Equity = nullDecimal(row.Equity)
		item.AdjustedEquity = nullDecimal(row.AdjustedEquity)
		item.AvailableEquity = nullDecimal(row.AvailableEquity)
		item.InitialMarginRatio = nullDecimal(row.InitialMarginRatio)
		item.MaintenanceMarginRatio = nullDecimal(row.MaintenanceMarginRatio)
		item.UnifiedMarginRatio = nullDecimal(row.UnifiedMarginRatio)
		if row.ObservedAt.Valid {
			t := row.ObservedAt.Time
			item.ReportObservedAt = &t
		}
	} else {
		item.SourceMarginMode = nullStringPtr(row.SourceMarginMode)
		item.Equity = nullDecimal(row.SourceEquity)
		item.AdjustedEquity = nullDecimal(row.SourceAdjustedEquity)
		item.AvailableEquity = nullDecimal(row.SourceAvailableEquity)
		item.InitialMarginRatio = nullDecimal(row.SourceInitialMarginRatio)
		item.MaintenanceMarginRatio = nullDecimal(row.SourceMaintenanceMarginRatio)
		item.UnifiedMarginRatio = nullDecimal(row.SourceUnifiedMarginRatio)
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

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}
