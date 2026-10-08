package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/testsupport/offline"
	"github.com/rbee3u/plutus/internal/testsupport/proxytest"
)

func TestMain(m *testing.M) {
	offline.Install()
	os.Exit(m.Run())
}

func TestConfiguredProxyTunnels(t *testing.T) {
	// NO_PROXY must not bypass an explicitly configured proxy, including loopback targets.
	t.Setenv("NO_PROXY", "*")
	for _, scheme := range []string{"http", "https", "socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			defer origin.Close()
			// The target hostname must reach SOCKS unchanged, without local resolution.
			target := strings.TrimPrefix(origin.URL, "https://")
			requestURL := origin.URL
			if strings.HasPrefix(scheme, "socks") {
				_, port, _ := net.SplitHostPort(target)
				target = net.JoinHostPort("exchange.example.com", port)
				requestURL = "https://" + target
			}
			var address string
			var count *atomic.Int64
			if strings.HasPrefix(scheme, "socks") {
				address, count = proxytest.SOCKS(t, origin, target)
				address = strings.Replace(address, "socks5:", scheme+":", 1)
			} else {
				server, n := proxytest.HTTP(t, origin, target, scheme == "https")
				address, count = server.URL, n
			}
			proxy, err := url.Parse(address)
			if err != nil {
				t.Fatal(err)
			}
			original := http.DefaultTransport.(*http.Transport).Proxy
			client := NewIPv4WithProxy(time.Second, http.ProxyURL(proxy))
			defer client.CloseIdleConnections()
			transport := client.Transport.(*http.Transport)
			transport.TLSClientConfig = origin.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			response, err := testGet(t, client, requestURL)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusNoContent || count.Load() != 1 {
				t.Fatalf("status=%d tunnels=%d", response.StatusCode, count.Load())
			}
			if transport == http.DefaultTransport || http.DefaultTransport.(*http.Transport).Proxy != nil && original == nil {
				t.Fatal("default transport mutated")
			}
		})
	}
}

func TestDirectAndFailedProxy(t *testing.T) {
	var requests atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNoContent) }))
	defer origin.Close()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := "http://" + listener.Addr().String()
	_ = listener.Close()
	t.Setenv("HTTP_PROXY", address)
	t.Setenv("HTTPS_PROXY", address)
	t.Setenv("NO_PROXY", "*")
	direct := NewIPv4WithProxy(time.Second, nil)
	defer direct.CloseIdleConnections()
	response, err := testGet(t, direct, origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	target, _ := url.Parse(address)
	proxied := NewIPv4WithProxy(time.Second, http.ProxyURL(target))
	defer proxied.CloseIdleConnections()
	response, err = testGet(t, proxied, origin.URL)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("unavailable proxy succeeded")
	}
	if requests.Load() != 1 {
		t.Fatalf("origin requests = %d; proxy failure fell back to direct", requests.Load())
	}
	// Context cancellation must still bound requests through the configured proxy.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin.URL, nil)
	if response, err = proxied.Do(req); err == nil {
		_ = response.Body.Close()
		t.Fatal("cancelled proxy request succeeded")
	}
}

func TestDefaultClientIgnoresInheritedProxy(t *testing.T) {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(key, "http://127.0.0.1:1")
	}
	for _, key := range []string{"NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
	base := http.DefaultTransport.(*http.Transport)
	original := base.Proxy
	var proxyCalls atomic.Int64
	base.Proxy = func(*http.Request) (*url.URL, error) {
		proxyCalls.Add(1)
		return url.Parse("http://127.0.0.1:1")
	}
	t.Cleanup(func() { base.Proxy = original })
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer origin.Close()
	client := NewIPv4(time.Second)
	defer client.CloseIdleConnections()
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("default client inherited a proxy callback")
	}
	response, err := testGet(t, client, origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent || proxyCalls.Load() != 0 || base.Proxy == nil {
		t.Fatal("default client must use direct access without mutating the default transport")
	}
}

func TestCancellationDuringProxyHandshake(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { close(started); <-release }))
	defer proxy.Close()
	defer close(release)
	target, _ := url.Parse(proxy.URL)
	client := NewIPv4WithProxy(time.Second, http.ProxyURL(target))
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://exchange.invalid/account", nil)
	done := make(chan error, 1)
	go func() {
		response, err := client.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("proxy handshake did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled handshake succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop stalled handshake")
	}
}
