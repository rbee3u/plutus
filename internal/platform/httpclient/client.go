// Package httpclient provides the default HTTP client for live Platform adapters.
package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"
)

// NewIPv4 returns a direct IPv4 client, retaining the default transport's TLS and
// connection reuse without inheriting its proxy policy.
func NewIPv4(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: ipv4Transport()}
}

// NewIPv4WithProxy retains the default transport's TLS and connection reuse but replaces its
// proxy policy. A nil callback forces direct access; a fixed callback ignores environment bypasses.
func NewIPv4WithProxy(timeout time.Duration, proxy func(*http.Request) (*url.URL, error)) *http.Client {
	transport := ipv4Transport()
	transport.Proxy = proxy
	return &http.Client{Timeout: timeout, Transport: transport}
}

func ipv4Transport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	dial := transport.DialContext
	if dial == nil {
		dial = dialer.DialContext
	}
	transport.DialContext = func(ctx context.Context, _ string, address string) (net.Conn, error) {
		return dial(ctx, "tcp4", address)
	}
	return transport
}
