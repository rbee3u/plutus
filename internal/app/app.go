// Package app is the composition root: it builds the application's services on
// one database handle and hands them to the transport and CLI layers.
package app

import (
	"database/sql"
	"log/slog"

	"github.com/rbee3u/plutus/internal/application/audit"
	"github.com/rbee3u/plutus/internal/application/auth"
	"github.com/rbee3u/plutus/internal/application/capital"
	"github.com/rbee3u/plutus/internal/application/catalog"
	"github.com/rbee3u/plutus/internal/application/collection"
	"github.com/rbee3u/plutus/internal/application/cycles"
	"github.com/rbee3u/plutus/internal/application/financial"
	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/application/manual"
	"github.com/rbee3u/plutus/internal/application/portfolio"
	"github.com/rbee3u/plutus/internal/application/retention"
	"github.com/rbee3u/plutus/internal/application/valuation"
	"github.com/rbee3u/plutus/internal/config"
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
	Financial   *financial.Service
	Collection  *collection.Service
	Cycles      *cycles.Service
	Retention   *retention.Service
	Audit       *audit.Service
	Idempotency *idempotency.Store
}

// New builds every service on db and shares one registry across its consumers.
func New(db *sql.DB, cfg *config.Config, registry *adapters.Registry, logger *slog.Logger) (*Services, error) {
	if registry == nil {
		registry = adapters.NewRegistry()
	}
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
	manualSvc := manual.NewService(db, catalogSvc)
	return &Services{
		DB:          db,
		Adapters:    registry,
		Auth:        auth.NewService(db, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime),
		Portfolios:  portfolio.NewService(db),
		Identities:  identities,
		Catalog:     catalogSvc,
		Capital:     capital.NewService(db),
		Manual:      manualSvc,
		Valuation:   valuationSvc,
		Financial:   financial.NewService(db, valuationSvc),
		Collection:  collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: catalogSvc, Manual: manualSvc, Valuation: valuationSvc, Stagger: collection.IdentityDelay, Logger: logger}),
		Cycles:      cycles.NewService(db),
		Retention:   retention.NewService(db),
		Audit:       audit.NewService(db),
		Idempotency: idempotency.NewStore(db),
	}, nil
}
