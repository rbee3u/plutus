package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestCollectIsDeterministic(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	a := New(adapters.PlatformBinance).WithClock(func() time.Time { return at })
	creds := adapters.Credentials{APIKey: "key-alice", APISecret: "secret"}
	first, err := a.Collect(context.Background(), creds, adapters.ModeBinancePM, []adapters.AccountType{"portfolio_margin", "spot"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.Collect(context.Background(), creds, adapters.ModeBinancePM, []adapters.AccountType{"portfolio_margin", "spot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Reports) != 2 || first.Reports[0].Equity == nil || *first.Reports[0].Equity != *second.Reports[0].Equity {
		t.Fatalf("reports differ: %+v vs %+v", first.Reports, second.Reports)
	}
	if len(first.Prices) != 4 {
		t.Fatalf("prices=%d", len(first.Prices))
	}
}

func TestKeyPrefixesReproduceFailureClasses(t *testing.T) {
	t.Parallel()
	a := New(adapters.PlatformOKX)
	for key, class := range map[string]string{"invalid-x": adapters.ClassAuthentication, "forbidden-x": adapters.ClassAuthorization, "ratelimit-x": adapters.ClassRateLimited} {
		_, err := a.Validate(context.Background(), adapters.Credentials{APIKey: key, APISecret: "s"}, adapters.ModeOKXMCM)
		var adapterErr *adapters.Error
		if !errors.As(err, &adapterErr) || adapterErr.Class != class {
			t.Fatalf("%s: got %v", key, err)
		}
	}
	if _, err := a.Validate(context.Background(), adapters.Credentials{APIKey: "k", APISecret: "s"}, adapters.ModeBinancePM); err == nil {
		t.Fatal("mode of another platform must be unsupported")
	}
}
