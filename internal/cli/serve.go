package cli

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	httpserver "github.com/rbee3u/plutus/internal/transport/http"
	"github.com/rbee3u/plutus/internal/worker"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API, the embedded web app, the scheduler, and the collection worker",
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
	}
	if db != nil {
		services, err := app.New(db, &cfg, buildAdapters(&cfg), logger)
		if err != nil {
			return err
		}
		options.Services = services
		loop := worker.New(&worker.Options{
			Cycles:      services.Cycles,
			Auth:        services.Auth,
			Idempotency: services.Idempotency,
			Collection:  services.Collection,
			Portfolios:  services.Portfolios,
			Maintenance: services.Retention,
			Logger:      logger,
			Scheduler:   true,
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
	}
	return httpserver.Run(ctx, &cfg, logger, httpserver.Engine(&options))
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
