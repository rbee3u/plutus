package cli

import (
	"net/http"
	"net/url"
	"time"

	"github.com/rbee3u/plutus/internal/config"
	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/binance"
	"github.com/rbee3u/plutus/internal/platform/bybit"
	"github.com/rbee3u/plutus/internal/platform/fake"
	"github.com/rbee3u/plutus/internal/platform/httpclient"
	"github.com/rbee3u/plutus/internal/platform/kraken"
	"github.com/rbee3u/plutus/internal/platform/okx"
)

func buildAdapters(cfg *config.Config) *adapters.Registry {
	registry := adapters.NewRegistry()
	if cfg.Collection.FakeAdapter {
		for _, platform := range adapters.OfferedPlatforms() {
			registry.Register(platform, fake.New(platform))
		}
		return registry
	}
	factory := platformHTTPFactory(cfg.Collection.Proxy)
	registry.Register(adapters.PlatformBinance, binance.New(&binance.Options{HTTPFactory: factory}))
	registry.Register(adapters.PlatformOKX, okx.New(&okx.Options{HTTPFactory: factory}))
	registry.Register(adapters.PlatformBybit, bybit.New(&bybit.Options{HTTPFactory: factory}))
	registry.Register(adapters.PlatformKraken, kraken.New(&kraken.Options{HTTPFactory: factory}))
	return registry
}

// The proxy policy must already be validated by config.Load.
func platformHTTPFactory(policy config.Proxy) func(time.Duration) *http.Client {
	proxy := http.ProxyFromEnvironment
	switch policy.Mode {
	case "direct":
		proxy = nil
	case "proxy":
		// Config validation has already checked the scheme, authority, and absence of credentials.
		target, _ := url.Parse(policy.URL)
		proxy = http.ProxyURL(target)
	}
	return func(timeout time.Duration) *http.Client {
		return httpclient.NewIPv4WithProxy(timeout, proxy)
	}
}
