package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/financial"
	"github.com/rbee3u/plutus/internal/service/portfolio"
	"github.com/rbee3u/plutus/internal/service/valuation"
)

// Options configures the cycle notifier.
type Options struct {
	Portfolios *portfolio.Service
	Valuation  *valuation.Service
	Financial  *financial.Service
	Sender     Sender
	Logger     *slog.Logger
}

// Service evaluates one completed cycle and sends high-leverage alerts.
type Service struct {
	portfolios *portfolio.Service
	valuation  *valuation.Service
	financial  *financial.Service
	sender     Sender
	logger     *slog.Logger
}

// NewService constructs a cycle notifier.
func NewService(opts Options) (*Service, error) {
	if opts.Portfolios == nil || opts.Valuation == nil || opts.Financial == nil || opts.Sender == nil {
		return nil, errors.New("telegram notifier requires portfolios, valuation, financial, and a sender")
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		portfolios: opts.Portfolios,
		valuation:  opts.Valuation,
		financial:  opts.Financial,
		sender:     opts.Sender,
		logger:     logger,
	}, nil
}

// NotifyCycle sends one message when any supported account at the point has a
// known, unrounded real leverage strictly above three. A failed position read
// skips only that account; the cycle itself is never failed.
func (s *Service) NotifyCycle(ctx context.Context, portfolioID, cycleID uuid.UUID) error {
	item, err := s.portfolios.GetByID(ctx, portfolioID)
	if err != nil {
		return fmt.Errorf("load portfolio: %w", err)
	}
	point, accounts, err := s.valuation.PointByCycleWithAccounts(ctx, portfolioID, cycleID)
	if err != nil {
		return fmt.Errorf("load valuation point: %w", err)
	}
	pctx := financial.PortfolioContext{ID: item.ID, Timezone: item.Timezone}
	alerts := make([]leverageAlert, 0)
	for i := range accounts {
		account := &accounts[i]
		if !supportsRealLeverage(adapter.Platform(account.Platform), account.AccountType) {
			continue
		}
		if account.SourceReportID == nil {
			continue
		}
		view, err := s.financial.AccountPositionsAt(ctx, pctx, account.AccountID, &cycleID)
		if err != nil {
			s.logger.WarnContext(ctx, "read positions for telegram alert failed",
				slog.String("account_id", account.AccountID.String()),
				slog.String("platform", account.Platform),
				slog.Any("err", err))
			continue
		}
		if alert, ok := evaluate(view.Account, view.Positions); ok {
			alerts = append(alerts, alert)
		}
	}
	if len(alerts) == 0 {
		return nil
	}
	messages := formatMessages(item.Name, item.Location(), point.EvaluatedAt, alerts)
	for _, message := range messages {
		if err := s.sender.Send(ctx, message); err != nil {
			return err
		}
	}
	s.logger.InfoContext(ctx, "telegram risk alert sent",
		slog.Int("accounts", len(alerts)), slog.Int("messages", len(messages)),
		slog.String("portfolio_id", portfolioID.String()), slog.String("cycle_id", cycleID.String()))
	return nil
}
