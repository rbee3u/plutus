package httpclient

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewIPv4ConnectsToIPv4AndRejectsIPv6(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")

	server4 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server4.Close()

	client := NewIPv4(time.Second)
	response, err := testGet(t, client, server4.URL)
	if err != nil {
		t.Fatalf("IPv4 request: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("IPv4 status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	listener6, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is unavailable: %v", err)
	}
	server6 := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server6.Listener = listener6
	server6.Start()
	defer server6.Close()

	response, err = testGet(t, client, server6.URL)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("IPv6 request succeeded; the default adapter client must use IPv4 only")
	}
}

func testGet(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}
