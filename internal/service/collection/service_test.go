package collection

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/service/identity"
)

func TestCollectRetryPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		failures      []string
		cancelBackoff bool
		wantCalls     int
		wantWaits     []time.Duration
		wantClass     string
	}{
		{name: "success", wantCalls: 1},
		{name: "temporary then success", failures: []string{adapter.ClassTemporary}, wantCalls: 2, wantWaits: []time.Duration{time.Second}},
		{name: "rate limit then success", failures: []string{adapter.ClassRateLimited}, wantCalls: 2, wantWaits: []time.Duration{time.Second}},
		{name: "temporary exhausted", failures: []string{adapter.ClassTemporary, adapter.ClassTemporary, adapter.ClassTemporary}, wantCalls: 3, wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantClass: adapter.ClassTemporary},
		{name: "rate limit exhausted", failures: []string{adapter.ClassRateLimited, adapter.ClassRateLimited, adapter.ClassRateLimited}, wantCalls: 3, wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantClass: adapter.ClassRateLimited},
		{name: "authentication is terminal", failures: []string{adapter.ClassAuthentication}, wantCalls: 1, wantClass: adapter.ClassAuthentication},
		{name: "authorization is terminal", failures: []string{adapter.ClassAuthorization}, wantCalls: 1, wantClass: adapter.ClassAuthorization},
		{name: "unsupported is terminal", failures: []string{adapter.ClassUnsupported}, wantCalls: 1, wantClass: adapter.ClassUnsupported},
		{name: "normalization is terminal", failures: []string{adapter.ClassNormalization}, wantCalls: 1, wantClass: adapter.ClassNormalization},
		{name: "cancelled backoff", failures: []string{adapter.ClassTemporary}, cancelBackoff: true, wantCalls: 1, wantWaits: []time.Duration{time.Second}, wantClass: adapter.ClassTemporary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantBatch := adapter.Batch{Reports: []adapter.AccountReport{{AccountType: "spot", Status: adapter.StatusSucceeded}}}
			platformAdapter := &testAdapter{}
			platformAdapter.collect = func([]adapter.AccountType) (adapter.Batch, error) {
				attempt := len(platformAdapter.calls) - 1
				if attempt < len(tc.failures) {
					return adapter.Batch{}, adapter.NewError(tc.failures[attempt], "Upstream unavailable.")
				}
				return wantBatch, nil
			}
			registry := adapter.NewRegistry()
			registry.Register(adapter.PlatformBinance, platformAdapter)
			service := NewService(nil, Options{Adapters: registry, RetryDelays: []time.Duration{time.Second, 3 * time.Second}})
			var waits []time.Duration
			service.sleep = func(ctx context.Context, delay time.Duration) error {
				waits = append(waits, delay)
				if tc.cancelBackoff {
					cancel()
				}
				return ctx.Err()
			}
			item := &identity.Collectable{Identity: identity.Identity{Platform: adapter.PlatformBinance, AccountStructure: adapter.StructureBinanceClassic}}
			batch, class, detail := service.collectWithRetry(ctx, item, []adapter.AccountType{"spot"})
			if len(platformAdapter.calls) != tc.wantCalls || !slices.Equal(waits, tc.wantWaits) {
				t.Fatalf("attempts = %d, waits = %v; want %d, %v", len(platformAdapter.calls), waits, tc.wantCalls, tc.wantWaits)
			}
			if class != tc.wantClass || (detail == "") != (tc.wantClass == "") {
				t.Fatalf("outcome = %q, %q; want class %q", class, detail, tc.wantClass)
			}
			if tc.wantClass != "" {
				wantBatch = adapter.Batch{}
			}
			if !reflect.DeepEqual(batch, wantBatch) {
				t.Fatalf("batch = %+v, want %+v", batch, wantBatch)
			}
		})
	}
}
