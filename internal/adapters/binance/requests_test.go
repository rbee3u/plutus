package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/offline"
	"github.com/rbee3u/plutus/internal/adapters/recording"
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
				body := `{"sample":true}`
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			a := New(&Options{Hosts: Hosts{API: server.URL, PAPI: server.URL, FAPI: server.URL, DAPI: server.URL}, HTTP: offline.Client(t, server)})
			f, err := a.RecordRequest(context.Background(), testCreds, ep.Request, recording.Options{KeepSymbols: []string{"DOGEUSDT"}})
			if err != nil || calls != 1 || f == nil {
				t.Fatalf("calls=%d file=%v error=%v", calls, f, err)
			}
			if f.Request.Key() != ep.Request.Key() || f.Source != "recorded" {
				t.Fatal("request or provenance changed")
			}
			if f.Request.Query.Has("keep-symbol") || f.Request.Query.Has("keepSymbols") {
				t.Fatal("local market hint reached the exchange request")
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

func TestSingleRecordingRetainsSafeFailureWithoutRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(429)
		_, _ = w.Write([]byte("too frequent"))
	}))
	defer server.Close()
	a := New(&Options{Hosts: Hosts{API: server.URL, PAPI: server.URL, FAPI: server.URL, DAPI: server.URL}, HTTP: offline.Client(t, server)})
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
