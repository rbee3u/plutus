// Package financial answers financial presentation reads from stored valuations.
package financial

import (
	"database/sql"
	"time"

	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/infra/mysql/sqlcgen"
)

// Service owns financial reads without owning valuation writes or rebuilds.
type Service struct {
	db        *sql.DB
	q         *sqlcgen.Queries
	valuation *valuation.Service
	now       func() time.Time
}

// NewService constructs a financial reader on the shared valuation service.
func NewService(db *sql.DB, values *valuation.Service) *Service {
	return &Service{db: db, q: sqlcgen.New(db), valuation: values, now: time.Now}
}
