// Package offline enforces the exchange-network boundary of adapter tests (ADR
// 0009).
package offline

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Install must run before tests, including before mysql.RunTestMain. A panic cannot be swallowed
// as an ordinary failed Account Report. No proxy or alternate TLS dialer can bypass this guard.
func Install() {
	http.DefaultTransport = &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsLoopback() {
			panic("offline test attempted external connection: " + address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
}

type guarded struct {
	t      interface{ Errorf(string, ...any) }
	origin string
	base   http.RoundTripper
}

func (g guarded) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme+"://"+r.URL.Host != g.origin {
		g.t.Errorf("offline replay attempted a request outside its server: %s://%s", r.URL.Scheme, r.URL.Host)
		return nil, errors.New("request outside the replay server")
	}
	return g.base.RoundTrip(r)
}

// Client confines even redirects and WebSocket handshakes to this test's own server.
func Client(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	client := *server.Client()
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	transport, ok := base.(*http.Transport)
	if ok {
		cloned := transport.Clone()
		cloned.Proxy = nil
		base = cloned
	}
	client.Transport = guarded{t: t, origin: server.URL, base: base}
	return &client
}
