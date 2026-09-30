package collection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/bybit"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/kraken"
	"github.com/rbee3u/plutus/internal/adapters/offline"
	"github.com/rbee3u/plutus/internal/adapters/recording"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

// This exercises the database's time-constrained price lookup, not only adapter return values.
func TestNewAdapterFixturesStoreAndValue(t *testing.T) {
	for _, platform := range []adapters.Platform{adapters.PlatformBybit, adapters.PlatformKraken} {
		t.Run(string(platform), func(t *testing.T) {
			h := newHarness(t)
			var err error
			answers := map[string]fixture.File{}
			files := loadAdapterSamples(t, platform, "")
			for _, f := range files {
				answers[f.Request.Key()] = f
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ws/v1" {
					conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
					if err != nil {
						t.Error(err)
						return
					}
					defer func() { _ = conn.CloseNow() }()
					ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
					defer cancel()
					if _, _, err := conn.Read(ctx); err != nil {
						return
					}
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"event":"challenge","message":"test"}`))
					if _, _, err := conn.Read(ctx); err != nil {
						return
					}
					_ = conn.Write(ctx, websocket.MessageText, answers[(fixture.Request{Service: "futures-ws", Method: "WS", Path: "/ws/v1/balances_snapshot"}).Key()].Body)
					return
				}

				raw, decodeErr := io.ReadAll(r.Body)
				if decodeErr != nil {
					t.Error(decodeErr)
					w.WriteHeader(500)
					return
				}
				r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil }
				request, decodeErr := recording.HTTPRequest(platform, r)
				if decodeErr != nil {
					t.Error(decodeErr)
					w.WriteHeader(500)
					return
				}
				f, ok := answers[request.Key()]
				if !ok {
					t.Errorf("unexpected fixture route %s", r.URL.Path)
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(f.Status)
				_, _ = w.Write(f.Body)
			}))
			t.Cleanup(server.Close)
			structure := adapters.StructureBybitUTA2
			types := []adapters.AccountType{bybit.TypeUnified, bybit.TypeFunding, bybit.TypeEarn}
			creds := adapters.Credentials{APIKey: "fixture-key", APISecret: "fixture-secret"}
			if platform == adapters.PlatformBybit {
				h.registry.Register(platform, bybit.New(&bybit.Options{Host: server.URL, HTTP: offline.Client(t, server)}))
			} else {
				structure = adapters.StructureKrakenUnified
				types = []adapters.AccountType{kraken.TypeMain, kraken.TypeEarn, kraken.TypeCoinM, kraken.TypeDerivativesHolding}
				creds = adapters.Credentials{APIKey: "spot-fixture", APISecret: "c2VjcmV0", Futures: &adapters.KeyPair{APIKey: "futures-fixture", APISecret: "c2VjcmV0"}}
				var snap struct {
					Timestamp int64 `json:"timestamp"`
				}
				if json.Unmarshal(answers[(fixture.Request{Service: "futures-ws", Method: "WS", Path: "/ws/v1/balances_snapshot"}).Key()].Body, &snap) != nil {
					t.Fatal("invalid snapshot fixture")
				}
				h.registry.Register(platform, kraken.New(&kraken.Options{SpotHost: server.URL, FuturesHost: server.URL, WebSocketURL: strings.Replace(server.URL, "http:", "ws:", 1) + "/ws/v1", HTTP: offline.Client(t, server), Now: func() time.Time { return time.UnixMilli(snap.Timestamp).Add(-time.Second) }}))
			}
			item := h.createIdentity("documented "+string(platform), platform, structure, types...)
			item, err = h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}, creds)
			if err != nil {
				t.Fatal(err)
			}
			if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
				t.Fatalf("enable: %v", err)
			}
			summary, jobID := h.runCycle()
			if summary.ReportFailed != 0 || summary.Point.Completeness() != valuation.CompletenessComplete {
				t.Fatalf("fixture cycle incomplete: %s", summary.String())
			}
			for _, v := range h.accountsByType(jobID, item.ID) {
				if len(v.Omitted) != 0 || v.ValueUSD == nil {
					t.Fatalf("%s omitted values despite documented prices", v.AccountType)
				}
			}
		})
	}
}

func TestKrakenSelectionRequiresCredentialGroups(t *testing.T) {
	h := newHarness(t)
	item := h.createIdentity("Earn only", adapters.PlatformKraken, adapters.StructureKrakenUnified, "earn")
	item = h.storeCredentials(item, "spot-only")
	item, _, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil {
		t.Fatal(err)
	}
	selected := true
	var patch identity.AccountPatch
	for _, a := range item.Accounts {
		if a.AccountType == "main" {
			patch = identity.AccountPatch{ID: a.ID, Selected: &selected}
		}
	}
	_, err = h.identities.Patch(h.ctx, h.portfolioID, item.ID, item.Version, identity.PatchInput{ActorUserID: h.userID, Accounts: []identity.AccountPatch{patch}})
	if !errors.Is(err, identity.ErrInvalid) {
		t.Fatalf("enabled selection missing Futures credentials: %v", err)
	}
	current, err := h.identities.Get(h.ctx, h.portfolioID, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range current.Accounts {
		if a.AccountType == "main" && a.Selected {
			t.Fatal("rejected edit mutated account selection")
		}
	}
	item, err = h.identities.Disable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if err != nil {
		t.Fatal(err)
	}
	item, err = h.identities.Patch(h.ctx, h.portfolioID, item.ID, item.Version, identity.PatchInput{ActorUserID: h.userID, Accounts: []identity.AccountPatch{patch}})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err = h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID})
	if !errors.Is(err, identity.ErrValidationFailed) {
		t.Fatal("enabled newly selected Main without Futures key")
	}
	// The same Spot key with a Futures key added reaches the same Kraken account.
	item, err = h.identities.PutCredentials(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}, adapters.Credentials{APIKey: "spot-only", APISecret: "fake", Futures: &adapters.KeyPair{APIKey: "futures", APISecret: "fake"}})
	if err != nil {
		t.Fatal(err)
	}
	if item.Connection.Status != identity.ConnectionDisabled {
		t.Fatal("replacement did not disable Connection")
	}
	if _, outcome, err := h.identities.Enable(h.ctx, h.portfolioID, item.ID, item.Version, identity.ActionInput{ActorUserID: h.userID}); err != nil || !outcome.OK {
		t.Fatalf("both groups cannot enable: %v", err)
	}
}
