// Package app is the composition root: it builds the application's services on
// one database handle and hands them to the transport and CLI layers.
package app

import (
	"database/sql"
	"log/slog"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/domain/audit"
	"github.com/rbee3u/plutus/internal/domain/auth"
	"github.com/rbee3u/plutus/internal/domain/capital"
	"github.com/rbee3u/plutus/internal/domain/catalog"
	"github.com/rbee3u/plutus/internal/domain/collection"
	"github.com/rbee3u/plutus/internal/domain/identity"
	"github.com/rbee3u/plutus/internal/domain/jobs"
	"github.com/rbee3u/plutus/internal/domain/manual"
	"github.com/rbee3u/plutus/internal/domain/portfolio"
	"github.com/rbee3u/plutus/internal/domain/valuation"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// Services holds every application service built on one database handle.
// Transport and CLI layers depend on this aggregate instead of wiring each
// service themselves.
type Services struct {
	DB       *sql.DB
	Adapters *adapters.Registry

	Auth        *auth.Service
	Portfolios  *portfolio.Service
	Identities  *identity.Service
	Catalog     *catalog.Service
	Capital     *capital.Service
	Manual      *manual.Service
	Valuation   *valuation.Service
	Collection  *collection.Service
	Jobs        *jobs.Service
	Audit       *audit.Service
	Idempotency *idempotency.Store
}

// New builds every service on db. registry may be nil; Identities then build
// their own default registry through identity.Options.
func New(db *sql.DB, cfg *config.Config, registry *adapters.Registry, logger *slog.Logger) (*Services, error) {
	identities, err := identity.NewService(db, identity.Options{
		MasterKey:       cfg.Crypto.MasterKey,
		Adapters:        registry,
		ValidateTimeout: cfg.Collection.ConnectionValidationTimeout,
	})
	if err != nil {
		return nil, err
	}
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	return &Services{
		DB:          db,
		Adapters:    registry,
		Auth:        auth.NewService(db, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime),
		Portfolios:  portfolio.NewService(db),
		Identities:  identities,
		Catalog:     catalogSvc,
		Capital:     capital.NewService(db),
		Manual:      manual.NewService(db),
		Valuation:   valuationSvc,
		Collection:  collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: catalogSvc, Valuation: valuationSvc, Stagger: collection.IdentityDelay, Logger: logger}),
		Jobs:        jobs.NewService(db),
		Audit:       audit.NewService(db),
		Idempotency: idempotency.NewStore(db),
	}, nil
}
