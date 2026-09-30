package binance

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/adapters/recording"
)

// recordCall is one account endpoint the recorder calls, with the parameters the adapter sends.
type recordCall struct {
	ep     endpoint
	params url.Values
}

func selectedCalls() []recordCall {
	calls := make([]recordCall, 0, 28)
	calls = append(calls, []recordCall{
		{epServerTime, nil},
		{epAccountInfo, nil},
		{epSpotAccount, spotAccountQuery()},
		{epUserAsset, nil},
		{epFundingAsset, nil},
		{epFlexibleEarn, simpleEarnQuery(1)},
		{epLockedEarn, simpleEarnQuery(1)},
		{epWalletValue, walletValueQuery()},
		{epPMAccount, nil},
		{epPMBalance, nil},
		{epPMUMPositionRisk, nil},
		{epPMCMPositionRisk, nil},
		{epCrossMargin, nil},
		{epIsolatedMargin, nil},
		{epUMAccount, nil},
		{epUMPositionRisk, nil},
		{epUMMultiAssets, nil},
		{epUMSymbolConfig, nil},
		{epCMAccount, nil},
		{epCMPositionRisk, nil},
		{epTickerPrice, nil},
		{epUMExchangeInfo, nil},
		{epCMExchangeInfo, nil},
		{endpoint{method: "GET", host: hostAPI, path: "/sapi/v1/dci/product/accounts", signed: true}, nil},
	}...)
	params := dualInvestmentQuery("PURCHASE_SUCCESS", 1)
	params.Del("status")
	calls = append(calls, recordCall{epDualInvest, params})
	return calls
}

// RecordingRequests lists read-only targets without observing or requesting an account.
func RecordingRequests() recording.Catalogue {
	out := recording.Catalogue{}
	for _, call := range selectedCalls() {
		request := fixture.Request{Service: recording.Service(adapters.PlatformBinance, call.ep.path), Method: call.ep.method, Path: call.ep.path, Query: call.params}
		out.Add(request, call.ep.signed)
		if call.ep.path == epDualInvest.path {
			name := fixture.Name(request.Path, "")
			ep := out[name]
			ep.Required = []string{"status"}
			out[name] = ep
		}
	}
	return out
}

// captureRequest uses native signing with the local clock, without preflight, retry or pagination.
func (a *Adapter) captureRequest(ctx context.Context, creds adapters.Credentials, request fixture.Request) error {
	for _, call := range selectedCalls() {
		if request.Service != recording.Service(adapters.PlatformBinance, call.ep.path) || request.Method != call.ep.method || request.Path != call.ep.path || len(request.Form) != 0 {
			continue
		}
		resp, err := a.client(creds).send(ctx, call.ep, request.Query, creds.APIKey, creds.APISecret)
		if err != nil {
			return err
		}
		return decode(call.ep, resp, &json.RawMessage{})
	}
	return errors.New("unknown read-only Binance recording request")
}
