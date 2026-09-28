// Package httpclient provides the default HTTP client for live Platform adapters.
package httpclient

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NewIPv4 returns a client whose direct connections use IPv4. It retains the default transport's
// TLS, connection reuse, and proxy behavior. A configured proxy determines the IP family used
// between that proxy and a Platform.
func NewIPv4(timeout time.Duration) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	transport := base.Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, _ string, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", address)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
