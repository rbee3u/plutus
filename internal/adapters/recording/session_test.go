package recording

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/offline"
)

func TestMain(m *testing.M) { offline.Install(); os.Exit(m.Run()) }

func TestSelectionPreservesValuesAndRequiresParametersBeforeNetworking(t *testing.T) {
	c := Catalogue{}
	r := fixture.Request{Service: "spot", Method: "GET", Path: "/account", Query: url.Values{"limit": {"100"}}}
	c.Add(r, true)
	ep := c["account"]
	ep.Required = []string{"status"}
	c["account"] = ep
	if _, err := SelectRequest(c, "account", nil, nil); err == nil {
		t.Fatal("missing required parameter accepted")
	}
	got, err := SelectRequest(c, "account", []string{"status=LIVE", "limit=2", "cursor=a%2Fb", "coin=BTC", "coin=ETH"}, nil)
	if err != nil || got.Query.Get("cursor") != "a%2Fb" || got.Query.Get("limit") != "2" || len(got.Query["coin"]) != 2 {
		t.Fatalf("selection: %+v %v", got, err)
	}
	if r.Query.Get("limit") != "100" {
		t.Fatal("catalogue default mutated")
	}
	for _, params := range [][]string{{"signature=secret"}, {"nonce=1"}, {"status"}} {
		if _, err = SelectRequest(c, "account", params, nil); err == nil {
			t.Fatal("invalid parameter accepted")
		}
	}
	form := fixture.Request{Service: "spot", Method: "POST", Path: "/0/private/Account", Form: url.Values{}}
	c.Add(form, true)
	got, err = SelectRequest(c, fixture.Name(form.Path, ""), nil, []string{"cursor=next%2Fpage"})
	if err != nil || got.Form.Get("cursor") != "next%2Fpage" {
		t.Fatalf("form: %+v %v", got, err)
	}
	if _, err = SelectRequest(c, fixture.Name(form.Path, ""), []string{"cursor=x"}, nil); err == nil {
		t.Fatal("wrong parameter channel accepted")
	}
	if _, err = SelectRequest(c, "account", nil, []string{"status=LIVE"}); err == nil {
		t.Fatal("unexpected form accepted")
	}
}

func TestExtraRequestsFailEvenWhenTheirErrorsAreSwallowed(t *testing.T) {
	for _, extra := range []string{"/account", "/configuration"} {
		t.Run(extra, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; _, _ = io.WriteString(w, `{"balance":"1"}`) }))
			defer server.Close()
			request := fixture.Request{Service: "spot", Method: "GET", Path: "/account"}
			s := New(adapters.PlatformBinance, request)
			client := s.Client(offline.Client(t, server))
			f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
				resp, e := client.Get(server.URL + "/account")
				if e != nil {
					return e
				}
				_ = resp.Body.Close()
				_, _ = client.Get(server.URL + extra)
				return nil
			})
			if calls != 1 || f == nil || err == nil {
				t.Fatalf("calls=%d file=%v error=%v", calls, f, err)
			}
		})
	}
}

func TestRedirectIsSavedWithoutFollowingIt(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/additional", http.StatusFound)
	}))
	defer server.Close()
	s := New(adapters.PlatformBinance, fixture.Request{Service: "spot", Method: "GET", Path: "/account"})
	client := s.Client(offline.Client(t, server))
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		resp, e := client.Get(server.URL + "/account")
		if e != nil {
			return e
		}
		_ = resp.Body.Close()
		return errors.New("HTTP redirect refused")
	})
	if err == nil || calls != 1 || f == nil || f.Status != 302 || f.BodyFormat != "text" {
		t.Fatalf("calls=%d file=%+v error=%v", calls, f, err)
	}
}

func TestSanitizationPreservesBusinessPrecisionAndFailureEvidence(t *testing.T) {
	s := New(adapters.PlatformOKX, fixture.Request{Service: "v5", Method: "GET", Path: "/api/v5/account/config"})
	secret := "private-key-value"
	f, err := s.Run(context.Background(), adapters.Credentials{APIKey: secret}, func(context.Context) error {
		s.Add(s.request, 400, []byte(`{"uid":"987654321012345678","mainUid":"987654321012345678","balance":"987654321012345678","price":1.234567890123456789,"msg":"private-key-value 198.51.100.2"}`))
		return errors.New("failed for " + secret)
	})
	if f == nil || err == nil || strings.Contains(err.Error(), secret) || strings.Contains(string(f.Body), secret) {
		t.Fatalf("unsafe failure: %+v %v", f, err)
	}
	var body map[string]json.RawMessage
	if e := json.Unmarshal(f.Body, &body); e != nil {
		t.Fatal(e)
	}
	if string(body["uid"]) != string(body["mainUid"]) || string(body["uid"]) == `"987654321012345678"` || string(body["balance"]) != `"987654321012345678"` || string(body["price"]) != "1.234567890123456789" || f.RecordedAt == nil || f.Notes == "" {
		t.Fatalf("business values or identity relationship changed: %s", f.Body)
	}
}

func TestRedactionFailureCannotReturnWritableEvidence(t *testing.T) {
	for _, requestLeak := range []bool{false, true} {
		t.Run(map[bool]string{false: "numeric body secret", true: "business parameter secret"}[requestLeak], func(t *testing.T) {
			request := fixture.Request{Service: "spot", Method: "GET", Path: "/account"}
			if requestLeak {
				request.Query = url.Values{"symbol": {"12345678"}}
			}
			s := New(adapters.PlatformBinance, request)
			f, err := s.Run(context.Background(), adapters.Credentials{APIKey: "12345678"}, func(context.Context) error { s.Add(request, 200, []byte(`{"amount":"12345678.001"}`)); return nil })
			if f != nil || err == nil {
				t.Fatal("unsafe response returned for writing")
			}
		})
	}
}

func TestLogicalSnapshotAndEmptyStates(t *testing.T) {
	request := fixture.Request{Service: "futures-ws", Method: "WS", Path: "/ws/v1/balances_snapshot"}
	s := New(adapters.PlatformKraken, request)
	f, err := s.Run(context.Background(), adapters.Credentials{}, func(context.Context) error {
		s.Add(request, 0, []byte(`{"feed":"balances_snapshot","account":"customer-123456789","holding":{}}`))
		return nil
	})
	if err != nil || f == nil || f.Status != 0 || strings.Contains(string(f.Body), "customer-") {
		t.Fatalf("snapshot %+v %v", f, err)
	}
	for _, test := range []struct {
		body  string
		empty bool
	}{{`{"result":{"list":[],"nextPageCursor":"x"}}`, true}, {`{"rows":[]}`, true}, {`{"rows":[{"amount":"0"}]}`, false}, {`{"error":["denied"]}`, false}, {`[]`, true}, {`""`, true}, {`null`, true}} {
		if Empty(&fixture.File{Body: []byte(test.body)}) != test.empty {
			t.Fatalf("empty state: %s", test.body)
		}
	}
}
