package main

import (
	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fake"
	"github.com/rbee3u/plutus/internal/config"
)

// buildAdapters registers the Platform adapters selected by configuration.
// No live Platform adapter exists yet; the deterministic fake adapter is opt-in per lane.
func buildAdapters(cfg *config.Config) *adapters.Registry {
	registry := adapters.NewRegistry()
	if cfg.Collection.FakeAdapter {
		for _, platform := range []adapters.Platform{adapters.PlatformBinance, adapters.PlatformOKX, adapters.PlatformBybit} {
			registry.Register(platform, fake.New(platform))
		}
	}
	return registry
}
