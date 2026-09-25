package collection

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/bybit"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/kraken"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/valuation"
)

// This exercises the database's time-constrained price lookup, not only adapter return values.
func TestNewAdapterFixturesStoreAndValue(t *testing.T) {
	for _, platform := range []adapters.Platform{adapters.PlatformBybit, adapters.PlatformKraken} {
		t.Run(string(platform), func(t *testing.T) {
			h := newHarness(t)
			answers := map[string]fixture.File{}
			paths, err := filepath.Glob(filepath.Join("..", "adapters", string(platform), "testdata", "*.json"))
			if err != nil || len(paths) == 0 {
				t.Fatal("missing fixture corpus")
			}
			for _, path := range paths {
				f, err := fixture.Read(path)
				if err != nil {
					t.Fatal(err)
				}
				if f.Fixture.Scenario != fixture.ScenarioRecorded {
					continue
				}
				_, name, _ := strings.Cut(f.Fixture.Endpoint, " ")
				answers[name] = f
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
					_ = conn.Write(ctx, websocket.MessageText, answers["/ws/v1/balances_snapshot"].Body)
					return
				}
				name := r.URL.RequestURI()
				if platform == adapters.PlatformKraken {
					name = r.URL.Path
				}
				f, ok := answers[name]
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
				h.registry.Register(platform, bybit.New(&bybit.Options{Host: server.URL, HTTP: server.Client()}))
			} else {
				structure = adapters.StructureKrakenUnified
				types = []adapters.AccountType{kraken.TypeMain, kraken.TypeEarn, kraken.TypeCoinM, kraken.TypeDerivativesHolding}
				creds = adapters.Credentials{APIKey: "spot-fixture", APISecret: "c2VjcmV0", Futures: &adapters.KeyPair{APIKey: "futures-fixture", APISecret: "c2VjcmV0"}}
				var snap struct {
					Timestamp int64 `json:"timestamp"`
				}
				if json.Unmarshal(answers["/ws/v1/balances_snapshot"].Body, &snap) != nil {
					t.Fatal("invalid snapshot fixture")
				}
				h.registry.Register(platform, kraken.New(&kraken.Options{SpotHost: server.URL, FuturesHost: server.URL, WebSocketURL: strings.Replace(server.URL, "http:", "ws:", 1) + "/ws/v1", HTTP: server.Client(), Now: func() time.Time { return time.UnixMilli(snap.Timestamp).Add(-time.Second) }}))
			}
			item := h.createIdentity("recorded "+string(platform), platform, structure, types...)
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
					t.Fatalf("%s omitted values despite recorded prices", v.AccountType)
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
