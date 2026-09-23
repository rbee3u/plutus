// Package httpserver hosts the Gin HTTP process.
package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// ReadyChecker reports process readiness.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}

// StaticReady fails or succeeds with a fixed error.
type StaticReady struct {
	Err error
}

// Ready implements ReadyChecker.
func (s StaticReady) Ready(context.Context) error {
	return s.Err
}

// FuncReady adapts a function to ReadyChecker.
type FuncReady func(context.Context) error

// Ready implements ReadyChecker.
func (f FuncReady) Ready(ctx context.Context) error {
	return f(ctx)
}

// EngineOptions configures the HTTP engine.
type EngineOptions struct {
	Logger            *slog.Logger
	Ready             ReadyChecker
	Auth              *auth.Service
	SecureCookies     bool
	SessionCookieName string
	CSRFCookieName    string
	AllowedOrigins    []string
	Portfolios        *portfolio.Service
	Identities        *identity.Service
	Adapters          *adapters.Registry
	Catalog           *catalog.Service
	Capital           *capital.Service
	Valuation         *valuation.Service
	Jobs              *jobs.Service
	Audit             *audit.Service
	Idempotency       *idempotency.Store
}

// Engine returns the HTTP handler for the process.
func Engine(opts *EngineOptions) *gin.Engine {
	if opts == nil {
		opts = &EngineOptions{}
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(bindGinContext())
	engine.Use(securityHeaders())
	engine.Use(noStoreSensitive())
	engine.Use(limitBody(1 << 20))
	engine.Use(captureBody())
	engine.Use(rejectBadOrigin(opts.AllowedOrigins))
	engine.Use(requestLog(opts.Logger))
	impl := &API{
		Ready:             opts.Ready,
		Auth:              opts.Auth,
		SecureCookies:     opts.SecureCookies,
		SessionCookieName: cookieName(opts.SessionCookieName, auth.SessionCookieName),
		CSRFCookieName:    cookieName(opts.CSRFCookieName, auth.CSRFCookieName),
		Portfolios:        opts.Portfolios,
		Identities:        opts.Identities,
		Adapters:          opts.Adapters,
		Catalog:           opts.Catalog,
		Capital:           opts.Capital,
		Valuation:         opts.Valuation,
		Jobs:              opts.Jobs,
		Audit:             opts.Audit,
		Idempotency:       opts.Idempotency,
	}
	if impl.Adapters == nil {
		impl.Adapters = adapters.NewRegistry()
	}
	strict := api.NewStrictHandlerWithOptions(impl, nil, api.StrictGinServerOptions{
		RequestErrorHandlerFunc:  requestError,
		HandlerErrorFunc:         handlerError,
		ResponseErrorHandlerFunc: handlerError,
	})
	api.RegisterHandlers(engine, strict)
	mountWeb(engine)
	return engine
}

func cookieName(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// Run serves HTTP or HTTPS until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, handler http.Handler) error {
	server := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	server.TLSConfig = tlsServerConfig(cfg)

	errCh := make(chan error, 1)
	go func() {
		var err error
		if cfg.Server.TLS.HTTPSEnabled {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		errCh <- err
	}()

	logger.Info("http server started",
		slog.String("addr", cfg.Server.Addr()),
		slog.Bool("https", cfg.Server.TLS.HTTPSEnabled),
		slog.Bool("mtls", cfg.Server.TLS.MTLSEnabled),
		slog.Bool("database_configured", cfg.Database.DSN != ""),
		slog.Bool("master_key_configured", cfg.Crypto.HasMasterKey()),
	)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func tlsServerConfig(cfg *config.Config) *tls.Config {
	if cfg == nil || !cfg.Server.TLS.HTTPSEnabled {
		return nil
	}
	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{*cfg.Server.TLS.Certificate},
	}
	if cfg.Server.TLS.MTLSEnabled {
		tlsCfg.ClientCAs = cfg.Server.TLS.ClientCAs
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tlsCfg
}
