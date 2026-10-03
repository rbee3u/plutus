package offline

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

type failureCounter struct{ failures int }

func (f *failureCounter) Errorf(string, ...any) { f.failures++ }

type forbiddenTransport struct{}

func (forbiddenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("guard allowed a forbidden request")
}

func TestReplayRejectsRedirectsAndWebSocketsEvenWhenErrorsAreIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://exchange.invalid/account", http.StatusFound)
	}))
	defer server.Close()
	reporter := &failureCounter{}
	client := Client(t, server)
	guard := client.Transport.(guarded)
	guard.t = reporter
	client.Transport = guard
	_, _ = client.Get(server.URL)                                                                                           //nolint:bodyclose,noctx // The offline guard must reject this convenience entrypoint before returning a response.
	_, _, _ = websocket.Dial(context.Background(), "wss://exchange.invalid/ws", &websocket.DialOptions{HTTPClient: client}) //nolint:bodyclose // The offline guard rejects the handshake before returning a response.
	if reporter.failures != 2 {
		t.Fatalf("ignored prohibited requests produced %d failures", reporter.failures)
	}
}

func TestOriginGuardRejectsBothSchemesBeforeTransport(t *testing.T) {
	for _, target := range []string{"http://exchange.invalid/account", "https://exchange.invalid/account"} {
		reporter := &failureCounter{}
		client := &http.Client{Transport: guarded{t: reporter, origin: "http://127.0.0.1:1", base: forbiddenTransport{}}}
		_, _ = client.Get(target) //nolint:bodyclose,noctx // The offline guard must reject this convenience entrypoint before returning a response.
		if reporter.failures != 1 {
			t.Fatal("prohibited request did not fail its test")
		}
	}
}
