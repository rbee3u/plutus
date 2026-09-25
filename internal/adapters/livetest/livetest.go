// Package livetest holds the shared helpers for live Platform smoke tests.
//
// Live tests carry the `live` build tag and run only through `make test-live`. They read one
// read-only key set per Platform from `PLUTUS_GOTEST_<PLATFORM>_APIKEY`, `_SECRET`, and, where
// the Platform needs one, `_PASSPHRASE` (process environment or `config/gotest.env`), and skip
// the Platform whose variables are missing. `make test` never compiles them, so it never reaches
// the network.
package livetest

import (
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/config"
)

// EnvPrefix is the prefix of every `go test` variable.
const EnvPrefix = "PLUTUS_GOTEST_"

// Credentials returns the read-only key set for a Platform or skips the test when any required
// variable is missing. needPassphrase is true for Platforms whose keys carry a passphrase.
func Credentials(tb testing.TB, platform adapters.Platform, needPassphrase bool) adapters.Credentials {
	tb.Helper()
	upper := strings.ToUpper(string(platform))
	creds := adapters.Credentials{
		APIKey:    lookup(tb, EnvPrefix+upper+"_APIKEY"),
		APISecret: lookup(tb, EnvPrefix+upper+"_SECRET"),
	}
	if needPassphrase {
		creds.Passphrase = lookup(tb, EnvPrefix+upper+"_PASSPHRASE")
	}
	missing := make([]string, 0, 3)
	if creds.APIKey == "" {
		missing = append(missing, EnvPrefix+upper+"_APIKEY")
	}
	if creds.APISecret == "" {
		missing = append(missing, EnvPrefix+upper+"_SECRET")
	}
	if needPassphrase && creds.Passphrase == "" {
		missing = append(missing, EnvPrefix+upper+"_PASSPHRASE")
	}
	if len(missing) > 0 {
		tb.Skipf("%s live smoke skipped: %s not set in the environment or config/gotest.env", platform, strings.Join(missing, ", "))
	}
	return creds
}

func lookup(tb testing.TB, key string) string {
	tb.Helper()
	value, err := config.Lookup(key)
	if err != nil {
		tb.Fatal(err)
	}
	return strings.TrimSpace(value)
}

// KrakenCredentials loads the two independent test credential groups.
func KrakenCredentials(tb testing.TB) adapters.Credentials {
	tb.Helper()
	creds := adapters.Credentials{
		APIKey: lookup(tb, EnvPrefix+"KRAKEN_SPOT_APIKEY"), APISecret: lookup(tb, EnvPrefix+"KRAKEN_SPOT_SECRET"),
		Futures: &adapters.KeyPair{APIKey: lookup(tb, EnvPrefix+"KRAKEN_FUTURES_APIKEY"), APISecret: lookup(tb, EnvPrefix+"KRAKEN_FUTURES_SECRET")},
	}
	if creds.APIKey == "" || creds.APISecret == "" || creds.FuturesKey() == "" || creds.FuturesSecret() == "" {
		tb.Skip("Kraken full live smoke requires both KRAKEN_SPOT and KRAKEN_FUTURES key groups in config/gotest.env")
	}
	return creds
}
