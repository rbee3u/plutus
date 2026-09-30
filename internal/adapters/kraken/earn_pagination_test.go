package kraken

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/offline"
)

func TestEarnStrategiesReplaysExactCursorSequence(t *testing.T) {
	for _, repeated := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "repeated cursor"}[repeated], func(t *testing.T) {
			request := fixture.Request{Service: "spot", Method: "POST", Path: epEarnStrategies.path}
			first := fixture.File{Source: "synthetic", Request: request, Status: 200, Body: []byte(`{"error":[],"result":{"items":[{"id":"S1","asset":"USD","lock_type":{"type":"instant"}}],"next_cursor":"next%2Fpage"}}`)}
			second := first
			second.Request.Form = url.Values{"cursor": {"next%2Fpage"}}
			cursor := `null`
			if repeated {
				cursor = `"next%2Fpage"`
			}
			second.Body = []byte(`{"error":[],"result":{"items":[{"id":"S2","asset":"BTC","lock_type":{"type":"bonded"}}],"next_cursor":` + cursor + `}}`)
			sequence, err := fixture.NewSequence(first, second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if e := sequence.Done(); e != nil {
					t.Error(e)
				}
			})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, e := io.ReadAll(r.Body)
				if e != nil {
					t.Error(e)
					w.WriteHeader(500)
					return
				}
				params, e := url.ParseQuery(string(raw))
				if e != nil {
					t.Error(e)
					w.WriteHeader(500)
					return
				}
				for key := range params {
					if fixture.AuthenticationParameter(key) {
						params.Del(key)
					}
				}
				got := fixture.Request{Service: "spot", Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Form: params}
				file, e := sequence.Next(got)
				if e != nil {
					t.Error(e)
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(file.Status)
				_, _ = io.Copy(w, bytes.NewReader(file.Body))
			}))
			defer server.Close()
			adapter := New(&Options{SpotHost: server.URL, HTTP: offline.Client(t, server)})
			runner := &run{c: adapter.client(testCreds)}
			strategies, err := runner.earnStrategies(context.Background())
			if repeated {
				if class, _ := adapters.Classify(err); err == nil || class != adapters.ClassNormalization {
					t.Fatalf("repeated cursor classification: %v", err)
				}
				return
			}
			if err != nil || len(strategies) != 2 || strategies["S1"] != "instant" || strategies["S2"] != "bonded" {
				t.Fatalf("strategies: %v %v", strategies, err)
			}
		})
	}
}
