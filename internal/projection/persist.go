package projection

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

func (s *Service) persist(ctx context.Context, built *builtGeneration) error {
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := s.insertLatest(ctx, q, built); err != nil {
		return err
	}
	if err := s.insertValuations(ctx, q, built); err != nil {
		return err
	}
	if err := s.insertExposures(ctx, q, built); err != nil {
		return err
	}
	if err := s.insertCapitals(ctx, q, built); err != nil {
		return err
	}
	if err := s.insertPeriods(ctx, q, built); err != nil {
		return err
	}
	if err := s.insertIssues(ctx, q, built); err != nil {
		return err
	}
	n, err := q.MarkProjectionGenerationReady(ctx, sqlcgen.MarkProjectionGenerationReadyParams{
		CompletedAt: sql.NullTime{Time: now, Valid: true},
		PortfolioID: mustBin(built.portfolioID),
		ID:          mustBin(built.id),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrFailed
	}
	return tx.Commit()
}

func (s *Service) insertLatest(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.balances {
		item := &built.balances[i]
		params := sqlcgen.InsertLatestBalanceProjectionParams{
			GenerationID:      mustBin(built.id),
			PortfolioID:       mustBin(built.portfolioID),
			IdentityID:        mustBin(item.IdentityID),
			AccountID:         mustBin(item.AccountID),
			AssetListingID:    mustBin(item.ListingID),
			ObservationID:     mustBin(item.ObservationID),
			SourceObservedAt:  item.ObservedAt,
			FreshnessState:    item.Freshness,
			Completeness:      item.Completeness,
			TotalQuantity:     ptrNull(item.Total),
			AvailableQuantity: ptrNull(item.Available),
			LockedQuantity:    ptrNull(item.Locked),
		}
		if item.AssetID != nil {
			params.AssetID = mustBin(*item.AssetID)
		}
		if err := q.InsertLatestBalanceProjection(ctx, params); err != nil {
			return err
		}
	}
	for i := range built.positions {
		item := &built.positions[i]
		if err := q.InsertLatestPositionProjection(ctx, sqlcgen.InsertLatestPositionProjectionParams{
			GenerationID:            mustBin(built.id),
			PortfolioID:             mustBin(built.portfolioID),
			IdentityID:              mustBin(item.IdentityID),
			AccountID:               mustBin(item.AccountID),
			InstrumentID:            mustBin(item.InstrumentID),
			PlatformPositionRef:     item.Ref,
			Side:                    item.Side,
			Quantity:                item.Quantity,
			SelectedMarkPrice:       ptrNull(item.Mark),
			UnderlyingLongQuantity:  item.Long,
			UnderlyingShortQuantity: item.Short,
			ObservationID:           mustBin(item.ObservationID),
			SourceObservedAt:        item.ObservedAt,
			FreshnessState:          item.Freshness,
			Completeness:            item.Completeness,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertValuations(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.valuations {
		item := &built.valuations[i]
		if err := q.InsertPortfolioValuationProjection(ctx, sqlcgen.InsertPortfolioValuationProjectionParams{
			ID:                  mustBin(item.ID),
			GenerationID:        mustBin(built.id),
			PortfolioID:         mustBin(built.portfolioID),
			EvaluatedAt:         item.EvaluatedAt,
			DenominationAssetID: mustBin(built.displayAsset),
			Amount:              ptrNull(item.Amount),
			Completeness:        item.Completeness,
			CoverageFingerprint: item.Fingerprint,
			FreshnessState:      item.Freshness,
			SourceCutoffAt:      item.EvaluatedAt,
		}); err != nil {
			return err
		}
	}
	for i := range built.contributions {
		item := &built.contributions[i]
		params := sqlcgen.InsertValuationContributionProjectionParams{
			ID:                   mustBin(item.ID),
			GenerationID:         mustBin(built.id),
			PortfolioID:          mustBin(built.portfolioID),
			PortfolioValuationID: mustBin(item.ValuationID),
			ContributionKind:     item.Kind,
			Amount:               ptrNull(item.Amount),
			Completeness:         item.Completeness,
			CoverageFingerprint:  item.Fingerprint,
		}
		if item.IdentityID != nil {
			params.IdentityID = mustBin(*item.IdentityID)
		}
		if item.AccountID != nil {
			params.AccountID = mustBin(*item.AccountID)
		}
		if item.AssetID != nil {
			params.AssetID = mustBin(*item.AssetID)
		}
		if err := q.InsertValuationContributionProjection(ctx, params); err != nil {
			return err
		}
		for _, source := range item.Sources {
			if err := q.InsertValuationContributionSource(ctx, sqlcgen.InsertValuationContributionSourceParams{
				GenerationID:            mustBin(built.id),
				PortfolioID:             mustBin(built.portfolioID),
				ValuationContributionID: mustBin(item.ID),
				ObservationID:           mustBin(source),
				SourceRole:              item.Kind,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) insertExposures(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.exposures {
		item := &built.exposures[i]
		if err := q.InsertExposureProjection(ctx, sqlcgen.InsertExposureProjectionParams{
			GenerationID:        mustBin(built.id),
			PortfolioID:         mustBin(built.portfolioID),
			EvaluatedAt:         item.EvaluatedAt,
			AssetID:             mustBin(item.AssetID),
			LongQuantity:        item.Long,
			ShortQuantity:       item.Short,
			NetQuantity:         item.Net,
			LongValue:           ptrNull(item.LongValue),
			ShortValue:          ptrNull(item.ShortValue),
			NetValue:            ptrNull(item.NetValue),
			DenominationAssetID: mustBin(built.displayAsset),
			Completeness:        item.Completeness,
			FreshnessState:      item.Freshness,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertCapitals(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.capitals {
		item := &built.capitals[i]
		if err := q.InsertContributedCapitalProjection(ctx, sqlcgen.InsertContributedCapitalProjectionParams{
			GenerationID:            mustBin(built.id),
			PortfolioID:             mustBin(built.portfolioID),
			EvaluatedAt:             item.EvaluatedAt,
			DenominationAssetID:     mustBin(built.displayAsset),
			Amount:                  ptrNull(item.Amount),
			Completeness:            item.Complete,
			SourceRevisionWatermark: item.Watermark,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertPeriods(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.changes {
		item := &built.changes[i]
		if err := q.InsertValueChangeProjection(ctx, sqlcgen.InsertValueChangeProjectionParams{
			ID:                   mustBin(item.ID),
			GenerationID:         mustBin(built.id),
			PortfolioID:          mustBin(built.portfolioID),
			PeriodKind:           item.Kind,
			PeriodStartDate:      item.Start,
			PeriodEndDate:        item.End,
			AsOfAt:               item.AsOf,
			BeginningValuationID: mustBin(item.BeginID),
			EndingValuationID:    mustBin(item.EndID),
			ContributionAmount:   ptrNull(item.Contrib),
			WithdrawalAmount:     ptrNull(item.Withdraw),
			Amount:               ptrNull(item.Amount),
			State:                item.State,
			CoverageFingerprint:  item.Fingerprint,
		}); err != nil {
			return err
		}
	}
	for i := range built.summaries {
		item := &built.summaries[i]
		if err := q.InsertPeriodSummaryProjection(ctx, sqlcgen.InsertPeriodSummaryProjectionParams{
			GenerationID:            mustBin(built.id),
			PortfolioID:             mustBin(built.portfolioID),
			PeriodKind:              item.Kind,
			PeriodStartDate:         item.Start,
			PeriodEndDate:           item.End,
			AsOfAt:                  item.AsOf,
			ValuationOpen:           ptrNull(item.Open),
			ValuationClose:          ptrNull(item.Close),
			ValuationHigh:           ptrNull(item.High),
			ValuationLow:            ptrNull(item.Low),
			ContributedCapitalClose: ptrNull(item.Capital),
			ValueChange:             ptrNull(item.Change),
			FreshnessState:          item.Freshness,
			Completeness:            item.Completeness,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertIssues(ctx context.Context, q *sqlcgen.Queries, built *builtGeneration) error {
	for i := range built.issues {
		item := &built.issues[i]
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		params := sqlcgen.InsertProjectionIssueParams{
			ID:               mustBin(id),
			GenerationID:     mustBin(built.id),
			PortfolioID:      mustBin(built.portfolioID),
			ProjectionKind:   item.Kind,
			ProjectionKey:    item.Key,
			Severity:         item.Severity,
			Code:             item.Code,
			Summary:          item.Summary,
			FirstEffectiveAt: item.First,
			LastEffectiveAt:  item.Last,
		}
		if item.IdentityID != nil {
			params.IdentityID = mustBin(*item.IdentityID)
		}
		if item.AccountID != nil {
			params.AccountID = mustBin(*item.AccountID)
		}
		if item.ObservationID != nil {
			params.ObservationID = mustBin(*item.ObservationID)
		}
		raw, err := json.Marshal(struct {
			Code string `json:"code"`
		}{Code: item.Code})
		if err != nil {
			return err
		}
		params.DetailsJson = raw
		if err := q.InsertProjectionIssue(ctx, params); err != nil {
			return err
		}
	}
	return nil
}

func ptrNull(value *string) sql.NullString {
	if value == nil || *value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
