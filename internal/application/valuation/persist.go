package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
	"github.com/rbee3u/plutus/internal/infra/sqljson"
)

const reportWriteBatch = 256

// updateReportValuations changes only derived fields. All batches belong to the
// point's transaction, so a failure cannot leave a partially valued point.
func updateReportValuations(ctx context.Context, tx *sql.Tx, portfolioID uuid.UUID, accounts []AccountValuation) error {
	for start := 0; start < len(accounts); start += reportWriteBatch {
		batch := accounts[start:min(start+reportWriteBatch, len(accounts))]
		values := make([][4]any, len(batch))
		for i := range batch {
			a := &batch[i]
			values[i][1] = a.Method
			if a.ValueUSD != nil {
				values[i][0] = *a.ValueUSD
			}
			if a.SourceReportID != nil {
				values[i][2] = a.SourceReportID[:]
			}
			if len(a.Omitted) > 0 {
				raw, err := json.Marshal(a.Omitted)
				if err != nil {
					return err
				}
				values[i][3] = string(raw)
			}
		}
		var query strings.Builder
		query.WriteString("UPDATE account_reports SET ")
		args := make([]any, 0, 9*len(batch)+1)
		for column, name := range []string{"value_usd", "method", "source_report_id", "omitted_json"} {
			if column > 0 {
				query.WriteString(", ")
			}
			query.WriteString(name + " = CASE id")
			for i := range batch {
				if column == 3 {
					query.WriteString(" WHEN ? THEN CAST(? AS JSON)")
				} else {
					query.WriteString(" WHEN ? THEN ?")
				}
				args = append(args, batch[i].ReportID[:], values[i][column])
			}
			query.WriteString(" END")
		}
		query.WriteString(" WHERE portfolio_id = ? AND id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + ")")
		args = append(args, portfolioID[:])
		for i := range batch {
			args = append(args, batch[i].ReportID[:])
		}
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

// persistPoint commits report values and their aggregate atomically.
func (s *Service) persistPoint(ctx context.Context, point Point, accounts []AccountValuation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := updateReportValuations(ctx, tx, point.PortfolioID, accounts); err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	boundaryJSON, err := marshalSlice(point.BoundaryEvents)
	if err != nil {
		return err
	}
	issuesJSON, err := marshalSlice(point.Issues)
	if err != nil {
		return err
	}
	if err := q.UpsertValuationPoint(ctx, sqlcgen.UpsertValuationPointParams{
		ID:                      id[:],
		PortfolioID:             point.PortfolioID[:],
		JobID:                   point.JobID[:],
		EvaluatedAt:             point.EvaluatedAt,
		TotalUsd:                point.TotalUSD,
		AccountCount:            uint32(point.AccountCount),           // #nosec G115 -- The count is bounded by the in-memory Account slice, far below 2^32.
		StaleAccountCount:       uint32(point.StaleAccountCount),      // #nosec G115 -- The count is a subset of the in-memory Account slice.
		IncompleteAccountCount:  uint32(point.IncompleteAccountCount), // #nosec G115 -- The count is a subset of the in-memory Account slice.
		BoundaryContributionUsd: point.BoundaryContributionUSD,
		BoundaryWithdrawalUsd:   point.BoundaryWithdrawalUSD,
		BoundaryJson:            boundaryJSON,
		IssuesJson:              issuesJSON,
		CreatedAt:               s.now().UTC(),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// marshalSlice stores empty collections as SQL NULL.
func marshalSlice[T any](items []T) (sqljson.Column, error) {
	if len(items) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return sqljson.Column(raw), nil
}
