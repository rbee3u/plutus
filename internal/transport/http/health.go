package httpserver

import (
	"errors"

	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/worker"
)

func readyReason(err error) string {
	switch {
	case errors.Is(err, mysql.ErrSchemaMismatch):
		return mysql.ErrSchemaMismatch.Error()
	case errors.Is(err, mysql.ErrMySQLIncompatible):
		return mysql.ErrMySQLIncompatible.Error()
	case errors.Is(err, worker.ErrUnavailable):
		return worker.ErrUnavailable.Error()
	default:
		return mysql.ErrDatabaseUnavailable.Error()
	}
}
