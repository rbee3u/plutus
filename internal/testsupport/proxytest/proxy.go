// Package proxytest provides loopback-only proxy servers for synthetic adapter tests.
package proxytest

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tunnels owns connections hijacked from HTTP or accepted by SOCKS.
type tunnels struct {
	mu          sync.Mutex
	connections []net.Conn
	closed      bool
}

func (ts *tunnels) add(c net.Conn) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		_ = c.Close()
		return
	}
	ts.connections = append(ts.connections, c)
}

func (ts *tunnels) close() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.closed = true
	for _, c := range ts.connections {
		_ = c.Close()
	}
}

func (ts *tunnels) forward(ctx context.Context, client net.Conn, input io.Reader, origin string) {
	defer func() { _ = client.Close() }()
	// The caller has already checked the requested target; only dial this fixed loopback origin.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	upstream, err := (&net.Dialer{}).DialContext(ctx, "tcp4", origin)
	if err != nil {
		return
	}
	ts.add(upstream)
	defer func() { _ = upstream.Close() }()
	done := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, input); close(done) }()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
	_ = upstream.Close()
	<-done
}

// HTTP accepts CONNECT only to target and forwards to the fixed loopback origin.
// TLS enables HTTPS proxy transport, using the standard httptest certificate.
func HTTP(t *testing.T, origin *httptest.Server, target string, tls bool) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	count := new(atomic.Int64)
	ts := new(tunnels)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != target {
			t.Errorf("unexpected proxy request: %s %s", r.Method, r.Host)
			http.Error(w, "unexpected target", http.StatusForbidden)
			return
		}
		count.Add(1)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("proxy response does not support hijacking")
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		ts.add(conn)
		if _, err = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			_ = conn.Close()
			return
		}
		if err = rw.Flush(); err != nil {
			_ = conn.Close()
			return
		}
		ts.forward(r.Context(), conn, rw.Reader, origin.Listener.Addr().String())
	})
	server := httptest.NewUnstartedServer(handler)
	if tls {
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(func() { ts.close(); server.Close() })
	return server, count
}

// SOCKS accepts unauthenticated SOCKS5 CONNECT only to target, then tunnels to origin.
func SOCKS(t *testing.T, origin *httptest.Server, target string) (string, *atomic.Int64) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ts := new(tunnels)
	count := new(atomic.Int64)
	var workers sync.WaitGroup
	workers.Go(func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			ts.add(conn)
			workers.Go(func() {
				defer func() { _ = conn.Close() }()
				socks(t.Context(), t, conn, origin.Listener.Addr().String(), target, count, ts)
			})
		}
	})
	t.Cleanup(func() { _ = listener.Close(); ts.close(); workers.Wait() })
	return "socks5://" + listener.Addr().String(), count
}

func socks(ctx context.Context, t *testing.T, conn net.Conn, origin, target string, count *atomic.Int64, ts *tunnels) {
	reader := bufio.NewReader(conn)
	var head [2]byte
	if _, err := io.ReadFull(reader, head[:]); err != nil {
		return
	}
	if head[0] != 5 {
		t.Error("SOCKS version is not 5")
		return
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return
	}
	noAuth := false
	for _, method := range methods {
		if method == 0 {
			noAuth = true
		}
	}
	if !noAuth {
		t.Error("SOCKS client requested authentication")
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	var request [4]byte
	if _, err := io.ReadFull(reader, request[:]); err != nil {
		return
	}
	if request[0] != 5 || request[1] != 1 || request[2] != 0 {
		t.Error("invalid SOCKS CONNECT")
		return
	}
	var host string
	switch request[3] {
	case 1:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(reader, ip); err != nil {
			return
		}
		host = net.IP(ip).String()
	case 3:
		n, err := reader.ReadByte()
		if err != nil {
			return
		}
		name := make([]byte, int(n))
		if _, err = io.ReadFull(reader, name); err != nil {
			return
		}
		host = string(name)
	default:
		t.Error("unexpected SOCKS address family")
		return
	}
	var port [2]byte
	if _, err := io.ReadFull(reader, port[:]); err != nil {
		return
	}
	if net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))) != target {
		t.Error("SOCKS requested an unexpected target")
		return
	}
	count.Add(1)
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	ts.forward(ctx, conn, reader, origin)
}
