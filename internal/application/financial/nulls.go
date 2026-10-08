package financial

import (
	"database/sql"

	"github.com/rbee3u/plutus/internal/shared/decimal"
)

func nullDecimal(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(decimal.Compact(v.String))
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}
