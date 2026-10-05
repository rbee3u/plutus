package okx

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/rbee3u/plutus/internal/platform/adapters"
	"github.com/rbee3u/plutus/internal/platform/fixture"
	"github.com/rbee3u/plutus/internal/platform/recording"
)

func selectedEndpoints() []endpoint {
	return []endpoint{epTime, epConfig, epBalance, epPositions, epFunding, epSavings, epStaking, epDCD, epValuation, epSwaps, epTickers}
}

func RecordingRequests() recording.Catalogue {
	out := recording.Catalogue{}
	for _, ep := range selectedEndpoints() {
		query, _ := url.ParseQuery(ep.query)
		request := fixture.Request{Service: "v5", Method: ep.method, Path: ep.path, Query: query}
		if ep.path == epSwaps.path {
			request.Query.Del("instType")
		}
		out.Add(request, ep.signed)
		if ep.path == epSwaps.path {
			name := fixture.Name(ep.path, "")
			entry := out[name]
			entry.Required = []string{"instType"}
			out[name] = entry
		}
	}
	return out
}

func (a *Adapter) captureRequest(ctx context.Context, creds adapters.Credentials, request fixture.Request) error {
	for _, ep := range selectedEndpoints() {
		if request.Service != "v5" || request.Method != ep.method || request.Path != ep.path || len(request.Form) != 0 {
			continue
		}
		ep.query = request.Query.Encode()
		resp, err := a.client(creds).send(ctx, ep, creds)
		if err != nil {
			return err
		}
		return decode(ep, resp, &json.RawMessage{})
	}
	return errors.New("unknown read-only OKX recording request")
}
