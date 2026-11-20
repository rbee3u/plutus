package integration

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/identity"
	"github.com/rbee3u/plutus/internal/service/notification"
)

type captureSender struct {
	messages []string
}

func (s *captureSender) Send(_ context.Context, text string) error {
	s.messages = append(s.messages, text)
	return nil
}

func TestTelegramNotifierEvaluatesHighLeveragePoint(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	item := h.createIdentity("main", adapter.PlatformBinance, adapter.StructureBinancePM, "portfolio_margin")
	anchor := time.Now().UTC().Unix()
	item = h.storeCredentials(item, "plutus-demo-v1:leverage-warning:"+strconv.FormatInt(anchor, 10))
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("enable: err=%v outcome=%+v", err, outcome)
	}
	_, cycleID := h.runCycle()
	sender := &captureSender{}
	notifier, err := notification.NewService(notification.Options{
		Portfolios: h.services.Portfolios,
		Valuation:  h.services.Valuation,
		Financial:  h.services.Financial,
		Sender:     sender,
		Logger:     slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := notifier.NotifyCycle(h.ctx, h.portfolioID, cycleID); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages: %d", len(sender.messages))
	}
	message := sender.messages[0]
	for _, part := range []string{
		"High leverage",
		"default · Binance / main / Portfolio margin",
		"Real leverage: ",
		"threshold: above 3x",
		"Adjusted equity: $1,000.00",
		"Gross notional: $",
		"Add margin to reduce risk.",
	} {
		if !strings.Contains(message, part) {
			t.Fatalf("missing %q in:\n%s", part, message)
		}
	}
}
