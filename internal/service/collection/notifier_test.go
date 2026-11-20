package collection

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rbee3u/plutus/internal/platform/adapter"
)

type recordingNotifier struct {
	calls       int
	portfolioID uuid.UUID
	cycleID     uuid.UUID
	err         error
}

func (n *recordingNotifier) NotifyCycle(_ context.Context, portfolioID, cycleID uuid.UUID) error {
	n.calls++
	n.portfolioID = portfolioID
	n.cycleID = cycleID
	return n.err
}

func TestRunCycleNotifiesAfterPointCommit(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.collection.logger = slog.New(slog.DiscardHandler)
	notifier := &recordingNotifier{}
	h.collection.notifier = notifier
	item := h.createIdentity("main", adapter.PlatformBinance, adapter.StructureBinancePM, "portfolio_margin")
	anchor := time.Now().UTC().Unix()
	h.storeCredentials(item, "plutus-demo-v1:leverage-warning:"+strconv.FormatInt(anchor, 10))

	_, cycleID := h.runCycle()
	if notifier.calls != 1 {
		t.Fatalf("notifier calls: %d", notifier.calls)
	}
	if notifier.portfolioID != h.portfolioID || notifier.cycleID != cycleID {
		t.Fatalf("notifier IDs: %s %s", notifier.portfolioID, notifier.cycleID)
	}

	// A replay with an existing point still evaluates the condition.
	if _, err := h.collection.RunCycle(h.ctx, h.portfolioID, cycleID, nil); err != nil {
		t.Fatal(err)
	}
	if notifier.calls != 2 {
		t.Fatalf("replay notifier calls: %d", notifier.calls)
	}
}

func TestNotifierFailureDoesNotFailCycle(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.collection.logger = slog.New(slog.DiscardHandler)
	notifier := &recordingNotifier{err: errors.New("telegram unavailable")}
	h.collection.notifier = notifier
	item := h.createIdentity("main", adapter.PlatformBinance, adapter.StructureBinancePM, "portfolio_margin")
	h.storeCredentials(item, "main")

	h.runCycle()
	if notifier.calls != 1 {
		t.Fatalf("notifier calls: %d", notifier.calls)
	}
}
