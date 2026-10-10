package collection

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/application/identity"
	"github.com/rbee3u/plutus/internal/platform/adapters"
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
		{name: "temporary then success", failures: []string{adapters.ClassTemporary}, wantCalls: 2, wantWaits: []time.Duration{time.Second}},
		{name: "rate limit then success", failures: []string{adapters.ClassRateLimited}, wantCalls: 2, wantWaits: []time.Duration{time.Second}},
		{name: "temporary exhausted", failures: []string{adapters.ClassTemporary, adapters.ClassTemporary, adapters.ClassTemporary}, wantCalls: 3, wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantClass: adapters.ClassTemporary},
		{name: "rate limit exhausted", failures: []string{adapters.ClassRateLimited, adapters.ClassRateLimited, adapters.ClassRateLimited}, wantCalls: 3, wantWaits: []time.Duration{time.Second, 3 * time.Second}, wantClass: adapters.ClassRateLimited},
		{name: "authentication is terminal", failures: []string{adapters.ClassAuthentication}, wantCalls: 1, wantClass: adapters.ClassAuthentication},
		{name: "authorization is terminal", failures: []string{adapters.ClassAuthorization}, wantCalls: 1, wantClass: adapters.ClassAuthorization},
		{name: "unsupported is terminal", failures: []string{adapters.ClassUnsupported}, wantCalls: 1, wantClass: adapters.ClassUnsupported},
		{name: "normalization is terminal", failures: []string{adapters.ClassNormalization}, wantCalls: 1, wantClass: adapters.ClassNormalization},
		{name: "cancelled backoff", failures: []string{adapters.ClassTemporary}, cancelBackoff: true, wantCalls: 1, wantWaits: []time.Duration{time.Second}, wantClass: adapters.ClassTemporary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantBatch := adapters.Batch{Reports: []adapters.AccountReport{{AccountType: "spot", Status: adapters.StatusSucceeded}}}
			adapter := &testAdapter{}
			adapter.collect = func([]adapters.AccountType) (adapters.Batch, error) {
				attempt := len(adapter.calls) - 1
				if attempt < len(tc.failures) {
					return adapters.Batch{}, adapters.NewError(tc.failures[attempt], "Upstream unavailable.")
				}
				return wantBatch, nil
			}
			registry := adapters.NewRegistry()
			registry.Register(adapters.PlatformBinance, adapter)
			service := NewService(nil, Options{Adapters: registry, RetryDelays: []time.Duration{time.Second, 3 * time.Second}})
			var waits []time.Duration
			service.sleep = func(ctx context.Context, delay time.Duration) error {
				waits = append(waits, delay)
				if tc.cancelBackoff {
					cancel()
				}
				return ctx.Err()
			}
			item := &identity.Collectable{Identity: identity.Identity{Platform: adapters.PlatformBinance, AccountStructure: adapters.StructureBinanceClassic}}
			batch, class, detail := service.collectWithRetry(ctx, item, []adapters.AccountType{"spot"})
			if len(adapter.calls) != tc.wantCalls || !slices.Equal(waits, tc.wantWaits) {
				t.Fatalf("attempts = %d, waits = %v; want %d, %v", len(adapter.calls), waits, tc.wantCalls, tc.wantWaits)
			}
			if class != tc.wantClass || (detail == "") != (tc.wantClass == "") {
				t.Fatalf("outcome = %q, %q; want class %q", class, detail, tc.wantClass)
			}
			if tc.wantClass != "" {
				wantBatch = adapters.Batch{}
			}
			if !reflect.DeepEqual(batch, wantBatch) {
				t.Fatalf("batch = %+v, want %+v", batch, wantBatch)
			}
		})
	}
}
