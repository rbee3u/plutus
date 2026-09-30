package kraken

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
	return []endpoint{epAssets, epAssetPairs, epSpotTicker, epAPIKeyInfo, epBalanceEx, epTradeBalance, epEarnStrategies, epEarnAllocations, epFuturesKey, epOpenPositions, epInstruments, epFuturesTickers, epBalancesSnapshot}
}

func RecordingRequests() recording.Catalogue {
	out := recording.Catalogue{}
	for _, ep := range selectedEndpoints() {
		service := recording.Service(adapters.PlatformKraken, ep.path)
		var form, query url.Values
		if ep.family == familyWebSocket {
			service = "futures-ws"
		} else if ep.family == familySpot && ep.signed {
			form = url.Values{}
		}
		if ep == epTradeBalance {
			form.Set("asset", "ZUSD")
		}
		if ep == epEarnAllocations {
			form.Set("converted_asset", "USD")
			form.Set("hide_zero_allocations", "true")
		}
		if ep == epInstruments {
			query = url.Values{"contractType": {"all"}}
		}
		out.Add(fixture.Request{Service: service, Method: ep.method, Path: ep.path, Query: query, Form: form}, ep.signed)
	}
	return out
}

func (a *Adapter) captureRequest(ctx context.Context, creds adapters.Credentials, request fixture.Request) error {
	for _, ep := range selectedEndpoints() {
		service := recording.Service(adapters.PlatformKraken, ep.path)
		if ep.family == familyWebSocket {
			service = "futures-ws"
		}
		if request.Service != service || request.Method != ep.method || request.Path != ep.path {
			continue
		}
		c := a.client(creds)
		if ep.family == familyWebSocket {
			_, err := c.balancesSnapshot(ctx)
			return err
		}
		params := request.Query
		if ep.family == familySpot && ep.signed {
			params = request.Form
		}
		resp, err := c.exchange(ctx, ep, params)
		if err != nil {
			return err
		}
		return decode(ep, resp, &json.RawMessage{})
	}
	return errors.New("unknown read-only Kraken recording request")
}
