package main

import (
	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/binance"
	"github.com/rbee3u/plutus/internal/adapters/bybit"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/adapters/kraken"
	"github.com/rbee3u/plutus/internal/adapters/okx"
	"github.com/rbee3u/plutus/internal/config"
)

// buildAdapters registers the Platform adapters selected by configuration.
//
// `[collection] fake_adapter = true` (dev environments and seeds) registers the deterministic fake
// for every Platform. `false` (test and prod) registers each implemented live adapter; Platforms
// without one stay unregistered and report `adapterAvailable = false`, so their Identities can be
// created but not validated or collected. Binance, OKX, Bybit, and Kraken have live adapters.
func buildAdapters(cfg *config.Config) *adapters.Registry {
	registry := adapters.NewRegistry()
	if cfg.Collection.FakeAdapter {
		for _, platform := range adapters.OfferedPlatforms() {
			registry.Register(platform, fake.New(platform))
		}
		return registry
	}
	registry.Register(adapters.PlatformBinance, binance.New(nil))
	registry.Register(adapters.PlatformOKX, okx.New(nil))
	registry.Register(adapters.PlatformBybit, bybit.New(nil))
	registry.Register(adapters.PlatformKraken, kraken.New(nil))
	return registry
}
