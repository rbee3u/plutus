package kraken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/offline"
)

func TestSingleRecordingHasNoPreflightRetryOrPagination(t *testing.T) {
	for name, ep := range RecordingRequests() {
		if ep.Request.Method == "WS" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			for _, key := range ep.Required {
				value := map[string]string{"category": "linear", "status": "PURCHASE_SUCCESS", "instType": "SWAP"}[key]
				ep.Request.Query.Set(key, value)
			}
			if ep.Request.Path == "/v5/position/list" {
				ep.Request.Query.Set("settleCoin", "USDT")
			}
			if ep.Request.Path == "/0/private/Earn/Strategies" {
				ep.Request.Form.Set("cursor", "next%2Fpage")
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != ep.Request.Method || req.URL.Path != ep.Request.Path {
					t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				for key, values := range ep.Request.Query {
					if !reflect.DeepEqual(req.URL.Query()[key], values) {
						t.Errorf("query changed: %s", key)
					}
				}
				if err := req.ParseForm(); err != nil {
					t.Error(err)
				}
				for key, values := range ep.Request.Form {
					if !reflect.DeepEqual(req.PostForm[key], values) {
						t.Errorf("form changed: %s", key)
					}
				}
				body := `{"error":[],"result":{}}`
				if ep.Request.Service != "spot" {
					body = `{"result":"success"}`
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			a := New(&Options{SpotHost: server.URL, FuturesHost: server.URL, HTTP: offline.Client(t, server)})
			f, err := a.RecordRequest(context.Background(), testCreds, ep.Request)
			if err != nil || calls != 1 || f == nil {
				t.Fatalf("calls=%d file=%v error=%v", calls, f, err)
			}
			if f.Request.Key() != ep.Request.Key() || f.Source != "recorded" {
				t.Fatal("request or provenance changed")
			}
			path, err := fixture.Write(t.TempDir(), name, f)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := fixture.Read(path)
			if err != nil || restored.Request.Key() != ep.Request.Key() {
				t.Fatalf("save changed request: %v", err)
			}
		})
	}
}

func TestSingleWebSocketRecordingNeedsNoRESTOrSpotKey(t *testing.T) {
	r := newReplay(t, "", nil)
	request := RecordingRequests()["ws_v1_balances_snapshot"].Request
	f, err := r.adapter.RecordRequest(context.Background(), adapters.Credentials{Futures: testCreds.Futures}, request)
	if err != nil || f == nil || f.Status != 0 {
		t.Fatalf("snapshot: %v %v", f, err)
	}
	if len(r.seen) != 1 || r.seen[0] != epBalancesSnapshot.String() {
		t.Fatalf("unexpected REST reads: %v", r.seen)
	}
}

func TestSingleRecordingRetainsSafeFailureWithoutRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(429)
		_, _ = w.Write([]byte("too frequent"))
	}))
	defer server.Close()
	a := New(&Options{SpotHost: server.URL, FuturesHost: server.URL, HTTP: offline.Client(t, server)})
	for _, ep := range RecordingRequests() {
		if ep.Request.Method == "WS" || len(ep.Required) > 0 {
			continue
		}
		f, err := a.RecordRequest(context.Background(), testCreds, ep.Request)
		if err == nil || f == nil || f.Status != 429 || f.BodyFormat != "text" || calls != 1 {
			t.Fatalf("failure lost or retried: %v %v calls=%d", f, err, calls)
		}
		return
	}
	t.Fatal("no selected HTTP interface")
}
