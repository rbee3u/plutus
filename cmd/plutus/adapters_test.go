package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/config"
)

func TestRuntimeAdaptersUseConfiguredProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "*")
	var requests atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			t.Errorf("method = %s, want CONNECT", r.Method)
		}
		requests.Add(1)
		http.Error(w, "synthetic proxy unavailable", http.StatusBadGateway)
	}))
	defer proxy.Close()
	registry := buildAdapters(&config.Config{Collection: config.Collection{Proxy: config.Proxy{Mode: "proxy", URL: proxy.URL}}})
	creds := adapters.Credentials{APIKey: "synthetic", APISecret: "c2VjcmV0", Passphrase: "synthetic"}
	for _, tc := range []struct {
		platform  adapters.Platform
		structure adapters.AccountStructure
	}{
		{adapters.PlatformBinance, adapters.StructureBinanceClassic}, {adapters.PlatformOKX, adapters.StructureOKXUnified}, {adapters.PlatformBybit, adapters.StructureBybitUTA2}, {adapters.PlatformKraken, adapters.StructureKrakenSeparate},
	} {
		t.Run(string(tc.platform), func(t *testing.T) {
			adapter, ok := registry.Lookup(tc.platform)
			if !ok {
				t.Fatal("missing runtime Adapter")
			}
			before := requests.Load()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := adapter.Validate(ctx, creds, tc.structure)
			class, _ := adapters.Classify(err)
			if class != adapters.ClassTemporary || requests.Load() != before+1 {
				t.Fatalf("validation error=%v requests=%d", err, requests.Load()-before)
			}
		})
	}
}

func TestRuntimeProxyPolicyAndFake(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7890")
	t.Setenv("NO_PROXY", "*")
	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://exchange.invalid/account", nil)
	for _, mode := range []string{"environment", "direct", "proxy"} {
		policy := config.Proxy{Mode: mode}
		if mode == "proxy" {
			policy.URL = "socks5h://127.0.0.1:1080"
		}
		client := platformHTTPFactory(policy)(time.Second)
		transport := client.Transport.(*http.Transport)
		if mode == "direct" {
			if transport.Proxy != nil {
				t.Fatal("direct mode inherited a proxy")
			}
			continue
		}
		target, err := transport.Proxy(request)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "proxy" {
			want, _ := url.Parse(policy.URL)
			if target == nil || *target != *want {
				t.Fatalf("proxy=%v, want=%v", target, want)
			}
		}
	}
	// Fake registration must not create clients or attempt to reach an unavailable proxy.
	registry := buildAdapters(&config.Config{Collection: config.Collection{FakeAdapter: true, Proxy: config.Proxy{Mode: "proxy", URL: "http://127.0.0.1:1"}}})
	for _, platform := range registry.Platforms() {
		adapter, ok := registry.Lookup(platform.ID)
		if !ok {
			t.Fatal("missing fake Adapter")
		}
		if _, err := adapter.Validate(context.Background(), adapters.Credentials{APIKey: "synthetic"}, platform.Structures[0].ID); err != nil {
			t.Fatal(err)
		}
	}
}
