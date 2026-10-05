package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/collection"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/manual"
	"github.com/rbee3u/plutus/internal/portfolio"
	httpserver "github.com/rbee3u/plutus/internal/transport/http"
	"github.com/rbee3u/plutus/internal/valuation"
	"github.com/rbee3u/plutus/internal/worker"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API, the embedded web app, the scheduler, and the job worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd)
		},
	}
}

func runServe(cmd *cobra.Command) error {
	runtime := runtimeFrom(cmd)
	logger := runtime.logs.App
	restoreGin := httpserver.BridgeGinLogs(logger)
	defer restoreGin()
	cfg, err := loadProcessConfig(cmd)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var db *sql.DB
	if cfg.Database.DSN != "" {
		opened, err := mysql.Open(cfg.Database.DSN, mysql.Loggers{App: logger, SQL: runtime.logs.SQL})
		if err != nil {
			return err
		}
		defer func() { _ = opened.Close() }()
		db = opened
	}

	options := httpserver.EngineOptions{
		Logger:            logger,
		AccessLogger:      runtime.logs.Access,
		Ready:             httpserver.StaticReady{Err: errors.New("database DSN is not configured")},
		SecureCookies:     cfg.Server.TLS.HTTPSEnabled(),
		SessionCookieName: cfg.Auth.SessionCookieName(),
		CSRFCookieName:    cfg.Auth.CSRFCookieName(),
		AllowedOrigins:    []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		Adapters:          buildAdapters(&cfg),
	}
	if db != nil {
		services, err := newServices(db, &cfg, options.Adapters, logger)
		if err != nil {
			return err
		}
		loop := worker.New(&worker.Options{
			Jobs:       services.jobs,
			Collection: services.collection,
			Valuation:  services.valuation,
			Logger:     logger,
			Scheduler:  true,
			Retention:  cfg.Jobs.FinishedRetention,
		})
		workerCtx, stopWorker := context.WithCancel(ctx)
		workerDone := make(chan struct{})
		go func() {
			defer close(workerDone)
			_ = loop.Run(workerCtx)
		}()
		// Deferred after the database close, so it runs first: the worker stops before its handle disappears.
		defer func() {
			stopWorker()
			<-workerDone
		}()
		options.Ready = serveReady(db, loop, cfg.Crypto.HasMasterKey())
		options.Auth = services.auth
		options.Portfolios = services.portfolios
		options.Identities = services.identities
		options.Catalog = services.catalog
		options.Capital = services.capital
		options.Manual = services.manual
		options.Valuation = services.valuation
		options.Jobs = services.jobs
		options.Audit = services.audit
		options.Idempotency = services.idempotency
	}
	return httpserver.Run(ctx, &cfg, logger, httpserver.Engine(&options))
}

// services holds every application service built on one database handle.
type services struct {
	auth        *auth.Service
	portfolios  *portfolio.Service
	identities  *identity.Service
	catalog     *catalog.Service
	capital     *capital.Service
	manual      *manual.Service
	valuation   *valuation.Service
	collection  *collection.Service
	jobs        *jobs.Service
	audit       *audit.Service
	idempotency *idempotency.Store
}

func newServices(db *sql.DB, cfg *config.Config, registry *adapters.Registry, logger *slog.Logger) (*services, error) {
	identities, err := identity.NewService(db, identity.Options{MasterKey: cfg.Crypto.MasterKey, Adapters: registry, ValidateTimeout: cfg.Collection.ConnectionValidationTimeout})
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(db, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime)
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	return &services{
		auth:       authSvc,
		portfolios: portfolio.NewService(db),
		identities: identities,
		catalog:    catalogSvc,
		capital:    capital.NewService(db),
		manual:     manual.NewService(db),
		valuation:  valuationSvc,
		collection: collection.NewService(db, collection.Options{
			Adapters:   registry,
			Identities: identities,
			Catalog:    catalogSvc,
			Valuation:  valuationSvc,
			Stagger:    collection.IdentityDelay,
			Logger:     logger,
		}),
		jobs:        jobs.NewService(db),
		audit:       audit.NewService(db),
		idempotency: idempotency.NewStore(db),
	}, nil
}

func serveReady(db *sql.DB, loop *worker.Worker, hasMasterKey bool) httpserver.ReadyChecker {
	if db == nil {
		return httpserver.StaticReady{Err: errors.New("database DSN is not configured")}
	}
	return httpserver.FuncReady(func(callCtx context.Context) error {
		if err := mysql.CheckReady(callCtx, db); err != nil {
			return err
		}
		if err := mysql.CheckEncryptedCredentials(callCtx, db, hasMasterKey); err != nil {
			return err
		}
		if loop == nil || !loop.Healthy(time.Now()) {
			return worker.ErrUnavailable
		}
		return nil
	})
}
