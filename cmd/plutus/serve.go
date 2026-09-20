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

	"github.com/rbee3u/plutus/internal/access"
	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	httpserver "github.com/rbee3u/plutus/internal/transport/http"
	"github.com/rbee3u/plutus/internal/worker"
)

func newServeCommand() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API and embedded web process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd, configPath)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "path to a non-secret TOML config file")
	return cmd
}

func runServe(cmd *cobra.Command, configPath string) error {
	logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(config.Options{Path: configPath})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var db *sql.DB
	var loop *worker.Worker
	if cfg.Database.DSN != "" {
		opened, err := mysql.Open(cfg.Database.DSN)
		if err != nil {
			return err
		}
		defer func() { _ = opened.Close() }()
		db = opened
	}

	var authSvc *auth.Service
	var portfolios *portfolio.Service
	var accessSvc *access.Service
	var idem *idempotency.Store
	var jobSvc *jobs.Service
	if db != nil {
		authSvc = auth.NewService(db, cfg.Auth.RegistrationEnabled, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime)
		portfolios = portfolio.NewService(db)
		fresh := cfg.Collection.DefaultInterval * time.Duration(cfg.Collection.DefaultFreshnessMultiplier)
		accessSvc = access.NewService(db, cfg.Collection.DefaultInterval, fresh)
		idem = idempotency.NewStore(db)
		jobSvc = jobs.NewService(db)
		loop = worker.New(worker.Options{
			Jobs:     jobSvc,
			Adapters: adapters.NewRegistry(),
			Poll:     cfg.Jobs.WorkerPollInterval,
			Lease:    cfg.Jobs.LeaseDuration,
			Logger:   logger,
		})
		go func() { _ = loop.Run(ctx) }()
	}

	engine := httpserver.Engine(&httpserver.EngineOptions{
		Logger:              logger,
		Ready:               serveReady(db, loop),
		Auth:                authSvc,
		RegistrationEnabled: cfg.Auth.RegistrationEnabled,
		SecureCookies:       cfg.Server.TLS.HTTPSEnabled,
		AllowedOrigins:      []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		Portfolios:          portfolios,
		Access:              accessSvc,
		Idempotency:         idem,
		Jobs:                jobSvc,
	})
	return httpserver.Run(ctx, &cfg, logger, engine)
}

func serveReady(db *sql.DB, loop *worker.Worker) httpserver.ReadyChecker {
	if db == nil {
		return httpserver.StaticReady{Err: errors.New("database DSN is not configured")}
	}
	return httpserver.FuncReady(func(callCtx context.Context) error {
		if err := mysql.CheckReady(callCtx, db); err != nil {
			return err
		}
		if loop == nil || !loop.Healthy(time.Now()) {
			return worker.ErrUnavailable
		}
		return nil
	})
}
