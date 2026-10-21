// Package app is the composition root: it builds the application's services on
// one database handle and hands them to the transport and CLI layers. Every
// service is constructed here and nowhere else; other packages receive the
// collaborators they need as parameters.
package app

import (
	"database/sql"
	"log/slog"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/idempotency"
	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/audit"
	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/service/capital"
	"github.com/rbee3u/plutus/internal/service/catalog"
	"github.com/rbee3u/plutus/internal/service/collection"
	"github.com/rbee3u/plutus/internal/service/cycle"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/manual"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/retention"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// Services holds every application service built on one database handle.
// Transport and CLI layers depend on this aggregate instead of wiring each
// service themselves.
type Services struct {
	DB       *sql.DB
	Adapters *adapter.Registry

	Auth        *auth.Service
	Portfolios  *portfolio.Service
	Identities  *identity.Service
	Catalog     *catalog.Service
	Capital     *capital.Service
	Manual      *manual.Service
	Valuation   *valuation.Service
	Financial   *financial.Service
	Collection  *collection.Service
	Cycles      *cycle.Service
	Retention   *retention.Service
	Audit       *audit.Service
	Idempotency *idempotency.Store
}

// Auth builds the session service on db.
func Auth(db *sql.DB, cfg *config.Config) *auth.Service {
	return auth.NewService(db, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime)
}

// Identity builds the identity service on db and registry. A missing master key
// disables credential writes.
func Identity(db *sql.DB, cfg *config.Config, registry *adapter.Registry) (*identity.Service, error) {
	return identity.NewService(db, identity.Options{
		MasterKey:       cfg.Crypto.MasterKey,
		Adapters:        registry,
		ValidateTimeout: cfg.Collection.ConnectionValidationTimeout,
	})
}

// New builds every service on db and shares one registry across its consumers.
func New(db *sql.DB, cfg *config.Config, registry *adapter.Registry, logger *slog.Logger) (*Services, error) {
	if registry == nil {
		registry = adapter.NewRegistry()
	}
	identities, err := Identity(db, cfg, registry)
	if err != nil {
		return nil, err
	}
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	manualSvc := manual.NewService(db, catalogSvc)
	cycleSvc := cycle.NewService(db)
	return &Services{
		DB:          db,
		Adapters:    registry,
		Auth:        Auth(db, cfg),
		Portfolios:  portfolio.NewService(db),
		Identities:  identities,
		Catalog:     catalogSvc,
		Capital:     capital.NewService(db),
		Manual:      manualSvc,
		Valuation:   valuationSvc,
		Financial:   financial.NewService(db, valuationSvc),
		Collection:  collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: catalogSvc, Manual: manualSvc, Valuation: valuationSvc, Cycles: cycleSvc, Stagger: collection.IdentityDelay, Logger: logger}),
		Cycles:      cycleSvc,
		Retention:   retention.NewService(db),
		Audit:       audit.NewService(db),
		Idempotency: idempotency.NewStore(db),
	}, nil
}
