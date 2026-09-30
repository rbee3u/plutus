package bybit

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/recording"
)

func selectedEndpoints() []endpoint {
	return []endpoint{epTime, epAccountInfo, epAPIKey, epWallet, epFunding, epFlexible, epFixedTerm, epDualAssets, epAssetOverview, epTickers, epPositions, epInstruments}
}

func RecordingRequests() recording.Catalogue {
	out := recording.Catalogue{}
	for _, ep := range selectedEndpoints() {
		query, _ := url.ParseQuery(ep.query)
		request := fixture.Request{Service: "v5", Method: "GET", Path: ep.path, Query: query}
		if ep.path == epPositions.path {
			request.Query = url.Values{"limit": {"200"}}
		}
		if ep.path == epInstruments.path {
			request.Query = url.Values{"limit": {"1000"}}
		}
		out.Add(request, ep.signed)
		if ep.path == epPositions.path || ep.path == epInstruments.path {
			name := fixture.Name(ep.path, "")
			entry := out[name]
			entry.Required = []string{"category"}
			entry.ValidateParameters = func(r fixture.Request) error {
				if r.Path == epPositions.path && r.Query.Get("category") == "linear" && r.Query.Get("settleCoin") == "" && r.Query.Get("symbol") == "" {
					return errors.New("linear position recording requires settleCoin or symbol")
				}
				return nil
			}
			out[name] = entry
		}
	}
	return out
}

func (a *Adapter) captureRequest(ctx context.Context, creds adapters.Credentials, request fixture.Request) error {
	for _, ep := range selectedEndpoints() {
		if request.Service != "v5" || request.Method != "GET" || request.Path != ep.path || len(request.Form) != 0 {
			continue
		}
		ep.query = request.Query.Encode()
		resp, err := a.client(creds).send(ctx, ep, ep.request(nil), creds)
		if err != nil {
			return err
		}
		return decode(ep, resp, &json.RawMessage{})
	}
	return errors.New("unknown read-only Bybit recording request")
}
