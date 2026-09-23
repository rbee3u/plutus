package valuation

import (
	"database/sql"
	"errors"
)

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
