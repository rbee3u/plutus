package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/decimal"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func canonicalManualState(raw []byte) string {
	if len(raw) == 0 {
		return "{}"
	}
	var amounts map[string]string
	if json.Unmarshal(raw, &amounts) != nil {
		return string(raw)
	}
	encoded, err := json.Marshal(amounts)
	if err != nil {
		return string(raw)
	}
	return string(encoded)
}

func identityWasPresent(previous map[uuid.UUID]previousAccount, identityID uuid.UUID) bool {
	for _, account := range previous {
		if account.identityID == identityID {
			return true
		}
	}
	return false
}

func (s *Service) confirmedEmpty(ctx context.Context, a *AccountValuation) (bool, error) {
	if a.Status != "ok" || a.Freshness != FreshnessFresh || a.ValueUSD == nil || a.Completeness() != CompletenessComplete {
		return false, nil
	}
	value, ok := decimal.Parse(*a.ValueUSD)
	if !ok || value.Sign() != 0 {
		return false, nil
	}
	for _, metric := range []*string{a.Equity, a.AdjustedEquity, a.AvailableEquity} {
		if metric != nil {
			amount, ok := decimal.Parse(*metric)
			if !ok || amount.Sign() != 0 {
				return false, nil
			}
		}
	}
	id := a.ReportID
	held, err := s.q.ReportHasHoldings(ctx, sqlcgen.ReportHasHoldingsParams{SourceID: id[:]})
	if err != nil {
		return false, err
	}
	return held.Valid && !held.Bool, nil
}

func manualTime(at sql.NullTime) *time.Time {
	if !at.Valid {
		return nil
	}
	return &at.Time
}
