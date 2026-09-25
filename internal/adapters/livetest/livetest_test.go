package livetest

import (
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
)

func TestCredentialsSkipWithoutKeys(t *testing.T) {
	t.Setenv(EnvPrefix+"BINANCE_APIKEY", "")
	t.Setenv(EnvPrefix+"BINANCE_SECRET", "")
	skipped := false
	t.Run("missing", func(t *testing.T) {
		t.Cleanup(func() { skipped = t.Skipped() })
		Credentials(t, adapters.PlatformBinance, false)
	})
	if !skipped {
		t.Fatal("missing keys must skip the live test rather than fail it")
	}
}

func TestCredentialsReadProcessEnvironment(t *testing.T) {
	t.Setenv(EnvPrefix+"OKX_APIKEY", " key ")
	t.Setenv(EnvPrefix+"OKX_SECRET", "secret")
	t.Setenv(EnvPrefix+"OKX_PASSPHRASE", "pass")
	creds := Credentials(t, adapters.PlatformOKX, true)
	if creds.APIKey != "key" || creds.APISecret != "secret" || creds.Passphrase != "pass" {
		t.Fatalf("credentials %+v", creds)
	}
}
