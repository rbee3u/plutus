package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
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

func TestEnvironmentProxyIsolation(t *testing.T) {
	if os.Getenv("PLUTUS_PROXY_HELPER") != "" {
		// Restore the standard environment callback only in this isolated helper process.
		http.DefaultTransport.(*http.Transport).Proxy = http.ProxyFromEnvironment
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://exchange.invalid/account", nil)
		client := NewIPv4(time.Second)
		proxy, err := client.Transport.(*http.Transport).Proxy(req)
		if err != nil {
			t.Fatal(err)
		}
		want := os.Getenv("PLUTUS_PROXY_EXPECT")
		if want == "" {
			if proxy != nil {
				t.Fatalf("unexpected proxy %v", proxy)
			}
		} else if proxy == nil || proxy.String() != want {
			t.Fatalf("proxy=%v, want=%s", proxy, want)
		}
		return
	}
	for _, bypass := range []bool{false, true} {
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestEnvironmentProxyIsolation$") // #nosec G204 G702 -- Re-execute this test binary with a fixed test selector.
		var env []string
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			switch strings.ToUpper(key) {
			case "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY":
				continue
			}
			env = append(env, entry)
		}
		want := "http://127.0.0.1:7890"
		noProxy := ""
		if bypass {
			want = ""
			noProxy = "*"
		}
		env = append(env, "PLUTUS_PROXY_HELPER=1", "HTTPS_PROXY=http://127.0.0.1:7890", "NO_PROXY="+noProxy, "PLUTUS_PROXY_EXPECT="+want)
		command.Env = env
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("environment case bypass=%t: %v\n%s", bypass, err, out)
		}
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
