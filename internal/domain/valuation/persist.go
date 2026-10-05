package valuation

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
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
