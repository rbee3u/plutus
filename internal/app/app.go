// Package app is the composition root: it builds the application's services on
// one database handle and hands them to the transport and CLI layers. Every
// service is constructed here and nowhere else; other packages receive the
// collaborators they need as parameters.
package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"

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
	"github.com/rbee3u/plutus/internal/service/notification"
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
	if logger == nil {
		logger = slog.Default()
	}
	identities, err := Identity(db, cfg, registry)
	if err != nil {
		return nil, err
	}
	catalogSvc := catalog.NewService(db)
	valuationSvc := valuation.NewService(db)
	manualSvc := manual.NewService(db, catalogSvc)
	cycleSvc := cycle.NewService(db)
	portfolioSvc := portfolio.NewService(db)
	financialSvc := financial.NewService(db, valuationSvc)
	notifier, err := buildNotifier(cfg, portfolioSvc, valuationSvc, financialSvc, logger)
	if err != nil {
		return nil, err
	}
	return &Services{
		DB:          db,
		Adapters:    registry,
		Auth:        Auth(db, cfg),
		Portfolios:  portfolioSvc,
		Identities:  identities,
		Catalog:     catalogSvc,
		Capital:     capital.NewService(db),
		Manual:      manualSvc,
		Valuation:   valuationSvc,
		Financial:   financialSvc,
		Collection:  collection.NewService(db, collection.Options{Adapters: registry, Identities: identities, Catalog: catalogSvc, Manual: manualSvc, Valuation: valuationSvc, Cycles: cycleSvc, Notifier: notifier, Stagger: collection.IdentityDelay, Logger: logger}),
		Cycles:      cycleSvc,
		Retention:   retention.NewService(db),
		Audit:       audit.NewService(db),
		Idempotency: idempotency.NewStore(db),
	}, nil
}

// buildNotifier enables Telegram alerts only when both the secret token and
// the TOML chat id are present. Partial configuration logs a warning instead of
// failing startup.
func buildNotifier(cfg *config.Config, portfolios *portfolio.Service, values *valuation.Service, financials *financial.Service, logger *slog.Logger) (collection.CycleNotifier, error) {
	if !cfg.Telegram.Enabled() {
		if cfg.Telegram.BotToken != "" || cfg.Telegram.ChatID != "" {
			logger.Warn("telegram risk alerts disabled: TG_RISK_TOKEN and telegram.chat_id are both required")
		} else {
			logger.Info("telegram risk alerts disabled: set TG_RISK_TOKEN and telegram.chat_id to enable")
		}
		return nil, nil
	}
	proxyURL, err := telegramProxyURL(cfg.Telegram.Proxy)
	if err != nil {
		return nil, err
	}
	sender, err := notification.NewTelegramSender(notification.TelegramOptions{
		BotToken: cfg.Telegram.BotToken,
		ChatID:   cfg.Telegram.ChatID,
		ProxyURL: proxyURL,
	})
	if err != nil {
		return nil, err
	}
	notifier, err := notification.NewService(notification.Options{
		Portfolios: portfolios,
		Valuation:  values,
		Financial:  financials,
		Sender:     sender,
		Logger:     logger,
	})
	if err != nil {
		return nil, err
	}
	logger.Info("telegram risk alerts enabled", slog.String("chat_id", cfg.Telegram.ChatID))
	return notifier, nil
}

func telegramProxyURL(proxy config.Proxy) (*url.URL, error) {
	if proxy.Mode != "proxy" {
		return nil, nil
	}
	parsed, err := url.Parse(proxy.URL)
	if err != nil {
		return nil, fmt.Errorf("telegram.proxy.url: %w", err)
	}
	return parsed, nil
}
