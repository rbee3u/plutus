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

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/testsupport/offline"
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
			s := New(adapter.PlatformBinance, request)
			client := s.Client(offline.Client(t, server))
			f, err := s.Run(context.Background(), adapter.Credentials{}, func(ctx context.Context) error {
				req, e := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/account", nil)
				if e != nil {
					return e
				}
				resp, e := client.Do(req)
				if e != nil {
					return e
				}
				_ = resp.Body.Close()
				extraReq, e := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+extra, nil)
				if e != nil {
					return e
				}
				if extraResp, extraErr := client.Do(extraReq); extraErr == nil {
					_ = extraResp.Body.Close()
				}
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
	s := New(adapter.PlatformBinance, fixture.Request{Service: "spot", Method: "GET", Path: "/account"})
	client := s.Client(offline.Client(t, server))
	f, err := s.Run(context.Background(), adapter.Credentials{}, func(ctx context.Context) error {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/account", nil)
		if e != nil {
			return e
		}
		resp, e := client.Do(req)
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
	s := New(adapter.PlatformOKX, fixture.Request{Service: "v5", Method: "GET", Path: "/api/v5/account/config"})
	secret := "private-key-value"
	f, err := s.Run(context.Background(), adapter.Credentials{APIKey: secret}, func(context.Context) error {
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
			s := New(adapter.PlatformBinance, request)
			f, err := s.Run(context.Background(), adapter.Credentials{APIKey: "12345678"}, func(context.Context) error { s.Add(request, 200, []byte(`{"amount":"12345678.001"}`)); return nil })
			if f != nil || err == nil {
				t.Fatal("unsafe response returned for writing")
			}
		})
	}
}

func TestFundingMemberIdentityIsRedactedWithoutChangingBalances(t *testing.T) {
	request := fixture.Request{Service: "v5", Method: "GET", Path: "/v5/asset/transfer/query-account-coins-balance"}
	s := New(adapter.PlatformBybit, request)
	f, err := s.Run(context.Background(), adapter.Credentials{}, func(context.Context) error {
		s.Add(request, 200, []byte(`{"retCode":0,"result":{"memberId":"987654321","userID":"987654321","balance":[{"coin":"USDT","walletBalance":"987654321.00000001","transferBalance":"4.0193"}]},"retMsg":"member 987654321"}`))
		return nil
	})
	if err != nil || f == nil {
		t.Fatalf("recording failed: %v", err)
	}
	var body struct {
		Result struct {
			MemberID string `json:"memberId"`
			UserID   string `json:"userID"`
			Balance  []struct {
				WalletBalance   string `json:"walletBalance"`
				TransferBalance string `json:"transferBalance"`
			} `json:"balance"`
		} `json:"result"`
		RetMsg string `json:"retMsg"`
	}
	if err := json.Unmarshal(f.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Result.MemberID == "987654321" || body.Result.MemberID != body.Result.UserID || strings.Contains(body.RetMsg, "987654321") {
		t.Fatal("funding identity or echo survived redaction, or equal references were separated")
	}
	if len(body.Result.Balance) != 1 || body.Result.Balance[0].WalletBalance != "987654321.00000001" || body.Result.Balance[0].TransferBalance != "4.0193" || !strings.Contains(f.Notes, "memberId") {
		t.Fatal("business balances changed or memberId redaction was not disclosed")
	}
}

func TestLogicalSnapshotAndEmptyStates(t *testing.T) {
	request := fixture.Request{Service: "futures-ws", Method: "WS", Path: "/ws/v1/balances_snapshot"}
	s := New(adapter.PlatformKraken, request)
	f, err := s.Run(context.Background(), adapter.Credentials{}, func(context.Context) error {
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
