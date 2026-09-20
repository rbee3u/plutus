package httpserver

import (
	"errors"

	"github.com/rbee3u/plutus/internal/infra/mysql"
)

func readyReason(err error) string {
	switch {
	case errors.Is(err, mysql.ErrSchemaMismatch):
		return mysql.ErrSchemaMismatch.Error()
	case errors.Is(err, mysql.ErrMySQLIncompatible):
		return mysql.ErrMySQLIncompatible.Error()
	default:
		return mysql.ErrDatabaseUnavailable.Error()
	}
}
