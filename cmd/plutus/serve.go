package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/infra/mysql"
	httpserver "github.com/rbee3u/plutus/internal/transport/http"
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
	ready, cleanup := newReadyChecker(cfg.Database.DSN, &db)
	defer cleanup()

	var authSvc *auth.Service
	if db != nil {
		authSvc = auth.NewService(db, cfg.Auth.RegistrationEnabled, cfg.Auth.SessionIdleLifetime, cfg.Auth.SessionAbsoluteLifetime)
	}

	engine := httpserver.Engine(httpserver.EngineOptions{
		Logger:              logger,
		Ready:               ready,
		Auth:                authSvc,
		RegistrationEnabled: cfg.Auth.RegistrationEnabled,
		SecureCookies:       cfg.Server.TLS.HTTPSEnabled,
		AllowedOrigins:      []string{"http://127.0.0.1:5173", "http://localhost:5173"},
	})
	return httpserver.Run(ctx, &cfg, logger, engine)
}

func newReadyChecker(dsn string, db **sql.DB) (httpserver.ReadyChecker, func()) {
	noop := func() {}
	if dsn == "" {
		return httpserver.StaticReady{Err: errors.New("database DSN is not configured")}, noop
	}
	opened, err := mysql.Open(dsn)
	if err != nil {
		return httpserver.StaticReady{Err: err}, noop
	}
	*db = opened
	return httpserver.FuncReady(func(callCtx context.Context) error {
		return mysql.CheckReady(callCtx, opened)
	}), func() { _ = opened.Close() }
}
