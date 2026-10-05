package offline_test

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/binance"
	"github.com/rbee3u/plutus/internal/platform/bybit"
	"github.com/rbee3u/plutus/internal/platform/kraken"
	"github.com/rbee3u/plutus/internal/platform/offline"
	"github.com/rbee3u/plutus/internal/platform/okx"
)

// The panic happens in net/http's dial goroutine. A subprocess demonstrates that converting an
// ordinary HTTP error into a failed Account Report cannot hide a forbidden connection.
func TestDefaultClientsCannotReachExchanges(t *testing.T) {
	for _, target := range []string{"http", "https", "websocket", "binance", "okx", "bybit", "kraken"} {
		t.Run(target, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestForbiddenDefaultConnectionHelper$") // #nosec G204 G702 -- Re-execute this test binary with a fixed test selector.
			cmd.Env = append(os.Environ(), "PLUTUS_OFFLINE_PROBE="+target, "HTTP_PROXY=http://exchange.invalid:8080", "HTTPS_PROXY=http://exchange.invalid:8080")
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "offline test attempted external connection") {
				t.Fatalf("guard did not stop %s: %v\n%s", target, err, out)
			}
		})
	}
}

func TestForbiddenDefaultConnectionHelper(t *testing.T) {
	target := os.Getenv("PLUTUS_OFFLINE_PROBE")
	if target == "" {
		t.Skip("subprocess helper")
	}
	offline.Install()
	creds := adapters.Credentials{APIKey: "offline", APISecret: "c2VjcmV0", Passphrase: "offline"}
	ctx := context.Background()
	switch target {
	case "http", "https":
		_, _ = http.Get(target + "://exchange.invalid/account") //nolint:bodyclose,noctx // The offline guard must reject this convenience entrypoint before returning a response.
	case "websocket":
		_, _, _ = websocket.Dial(ctx, "wss://exchange.invalid/ws", nil) //nolint:bodyclose // The offline guard rejects the handshake before returning a response.
	case "binance":
		_, _ = binance.New(&binance.Options{Hosts: binance.Hosts{API: "https://exchange.invalid", PAPI: "https://exchange.invalid", FAPI: "https://exchange.invalid", DAPI: "https://exchange.invalid"}}).Validate(ctx, creds, adapters.StructureBinancePM)
	case "okx":
		_, _ = okx.New(&okx.Options{Host: "https://exchange.invalid"}).Validate(ctx, creds, adapters.StructureOKXUnified)
	case "bybit":
		_, _ = bybit.New(&bybit.Options{Host: "https://exchange.invalid"}).Validate(ctx, creds, adapters.StructureBybitUTA2)
	case "kraken":
		_, _ = kraken.New(&kraken.Options{SpotHost: "https://exchange.invalid", FuturesHost: "https://exchange.invalid", WebSocketURL: "wss://exchange.invalid/ws"}).Validate(ctx, creds, adapters.StructureKrakenUnified)
	}
}
