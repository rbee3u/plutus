package kraken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/adapters/httpclient"
	"github.com/rbee3u/plutus/internal/adapters/proxytest"
)

func TestHTTPFactoryAndOverride(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: requestTimeout}
	calls := 0
	factory := func(timeout time.Duration) *http.Client {
		calls++
		if timeout != requestTimeout {
			t.Errorf("factory timeout=%s, want=%s", timeout, requestTimeout)
		}
		return client
	}
	a := New(&Options{HTTPFactory: factory})
	if a.http != client || calls != 1 {
		t.Fatal("factory client was not selected")
	}
	override := &http.Client{Timeout: time.Second}
	a = New(&Options{HTTP: override, HTTPFactory: factory})
	if a.http != override || calls != 1 {
		t.Fatal("explicit HTTP override did not win")
	}
	a = New(nil)
	if a.http.Timeout != requestTimeout {
		t.Fatal("default timeout changed")
	}
}

func TestBalancesWebSocketThroughConfiguredProxy(t *testing.T) {
	t.Parallel()
	replay := &replay{t: t, files: loadFixtures(t, "")}
	replay.snapshot, _, _ = replay.answer(epBalancesSnapshot.String())
	origin := httptest.NewTLSServer(replay)
	defer origin.Close()
	proxy, count := proxytest.HTTP(t, origin, strings.TrimPrefix(origin.URL, "https://"), false)
	target, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	var clients []*http.Client
	adapter := New(&Options{WebSocketURL: strings.Replace(origin.URL, "https:", "wss:", 1) + "/ws/v1", HTTPFactory: func(timeout time.Duration) *http.Client {
		client := httpclient.NewIPv4WithProxy(timeout, http.ProxyURL(target))
		client.Transport.(*http.Transport).TLSClientConfig = origin.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
		clients = append(clients, client)
		return client
	}})
	defer func() {
		for _, client := range clients {
			client.CloseIdleConnections()
		}
	}()
	snapshot, err := adapter.client(testCreds).balancesSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 || snapshot.Timestamp == 0 || snapshot.Flex.UnifiedBalances == nil || !*snapshot.Flex.UnifiedBalances {
		t.Fatalf("tunnels=%d snapshot=%+v", count.Load(), snapshot)
	}
}
