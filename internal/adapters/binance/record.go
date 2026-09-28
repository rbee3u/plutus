package binance

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Placeholders for identifying values in recorded fixtures: the account UID, and the request IP
// Binance echoes in -2015 messages (a TEST-NET-3 documentation address).
const (
	RedactedUID = 123456789
	RedactedIP  = "203.0.113.1"
)

var ipv4 = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

// recordCall is one account endpoint the recorder calls, with the parameters the adapter sends.
type recordCall struct {
	ep     endpoint
	params url.Values
}

// recordCalls lists the account calls of an Account Structure, with the first page of every paged
// query. A Classic wallet Binance says is not opened is skipped, as the adapter skips it.
func recordCalls(structure adapters.AccountStructure, info *accountInfo) []recordCall {
	calls := []recordCall{
		{epUserAsset, nil},
		{epFundingAsset, nil},
		{epFlexibleEarn, simpleEarnQuery(1)},
		{epLockedEarn, simpleEarnQuery(1)},
	}
	for _, status := range runningDualInvestmentStates() {
		calls = append(calls, recordCall{epDualInvest, dualInvestmentQuery(status, 1)})
	}
	calls = append(calls, recordCall{epWalletValue, walletValueQuery()})
	if structure == adapters.StructureBinancePM {
		return append(calls, recordCall{epPMAccount, nil}, recordCall{epPMBalance, nil},
			recordCall{epPMUMPositionRisk, nil}, recordCall{epPMCMPositionRisk, nil})
	}
	if !info.marginClosed() {
		calls = append(calls, recordCall{epCrossMargin, nil}, recordCall{epIsolatedMargin, nil})
	}
	if !info.futuresClosed() {
		calls = append(calls, recordCall{epUMMultiAssets, nil}, recordCall{epUMAccount, nil}, recordCall{epUMPositionRisk, nil},
			recordCall{epCMAccount, nil}, recordCall{epCMPositionRisk, nil})
	}
	return calls
}

// target is the path and query of a call without the signing parameters. Fixture headers name a
// call by its method and target, and fixture files by fixture.Name of the target.
func target(ep endpoint, params url.Values) string {
	if len(params) == 0 {
		return ep.path
	}
	return ep.path + "?" + params.Encode()
}

// recorder collects the recordings of one Record run.
type recorder struct {
	c         *client
	structure adapters.AccountStructure
	bodies    map[endpoint][][]byte // every answer of an endpoint, in call order
	out       []fixture.Recording
}

// fetch calls one endpoint and requires a 200 answer.
func (r *recorder) fetch(ctx context.Context, ep endpoint, params url.Values) (response, error) {
	resp, err := r.c.fetch(ctx, ep, params)
	if err != nil {
		return response{}, fmt.Errorf("%s: %w", ep, err)
	}
	if resp.status != http.StatusOK {
		return response{}, classify(ep, resp)
	}
	r.bodies[ep] = append(r.bodies[ep], resp.body)
	return resp, nil
}

func (r *recorder) add(ep endpoint, params url.Values, scenario string, resp response, trimmed *string, redacted []string) {
	suffix := scenario
	if scenario == fixture.ScenarioRecorded {
		suffix = ""
	}
	request := target(ep, params)
	r.out = append(r.out, fixture.Recording{
		Name: fixture.Name(request, suffix), Endpoint: ep.method + " " + request, AccountStructure: string(r.structure),
		Scenario: scenario, Status: resp.status, Trimmed: trimmed, Redacted: redacted, Body: json.RawMessage(resp.body),
	})
}

// Record reads the account's Account Structure, calls every endpoint the adapter uses for that
// structure once, trims the large market-data lists to the symbols the account needs plus a fixed
// coverage sample, replaces the account UID, and captures three safe error answers. It never runs
// inside serve, the worker, or collect.
func (a *Adapter) Record(ctx context.Context, creds adapters.Credentials) ([]fixture.Recording, error) {
	r := &recorder{c: a.client(creds), bodies: map[endpoint][][]byte{}}

	serverTime, err := r.fetch(ctx, epServerTime, nil)
	if err != nil {
		return nil, err
	}
	infoResp, err := r.fetch(ctx, epAccountInfo, nil)
	if err != nil {
		return nil, err
	}
	var info accountInfo
	if err := json.Unmarshal(infoResp.body, &info); err != nil {
		return nil, fmt.Errorf("decode %s: %w", epAccountInfo, err)
	}
	if r.structure, err = info.structure(); err != nil {
		return nil, err
	}
	r.add(epServerTime, nil, fixture.ScenarioRecorded, serverTime, nil, nil)
	r.add(epAccountInfo, nil, fixture.ScenarioRecorded, infoResp, nil, nil)

	for _, call := range recordCalls(r.structure, &info) {
		resp, err := r.fetch(ctx, call.ep, call.params)
		if err != nil {
			return nil, err
		}
		r.add(call.ep, call.params, fixture.ScenarioRecorded, resp, nil, nil)
	}

	spot, err := r.fetch(ctx, epSpotAccount, spotAccountQuery())
	if err != nil {
		return nil, err
	}
	if spot.body, err = redactUID(spot.body); err != nil {
		return nil, err
	}
	r.add(epSpotAccount, spotAccountQuery(), fixture.ScenarioRecorded, spot, nil, []string{"uid"})

	if err := r.recordMarkets(ctx); err != nil {
		return nil, err
	}

	// Three error answers that are safe to provoke, on the call every run starts with: a key
	// Binance does not know, a wrong signature, and a timestamp far outside recvWindow.
	badSigningMaterial := strings.Repeat("invalid", 8)
	if _, ed, err := adapters.BinancePrivateKey(creds.APISecret); err != nil {
		return nil, err
	} else if ed {
		encoded, err := x509.MarshalPKCS8PrivateKey(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)))
		if err != nil {
			return nil, err
		}
		badSigningMaterial = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))
	}
	for _, scenario := range []struct {
		name, key, secret, timestamp string
	}{
		{"invalid_key", strings.Repeat("0", 64), creds.APISecret, ""},
		{"bad_signature", creds.APIKey, badSigningMaterial, ""},
		{"stale_timestamp", creds.APIKey, creds.APISecret, strconv.FormatInt(a.clock.now().Add(-time.Minute).UnixMilli(), 10)},
	} {
		resp, err := r.c.send(ctx, epAccountInfo, nil, scenario.key, scenario.secret, scenario.timestamp)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", epAccountInfo, scenario.name, err)
		}
		if resp.status == http.StatusOK {
			return nil, fmt.Errorf("%s %s: Binance accepted the request", epAccountInfo, scenario.name)
		}
		var redacted []string
		if ipv4.Match(resp.body) {
			resp.body = ipv4.ReplaceAll(resp.body, []byte(RedactedIP))
			redacted = []string{"msg request ip"}
		}
		r.add(epAccountInfo, nil, scenario.name, resp, nil, redacted)
	}
	return r.out, nil
}

// recordMarkets records the market data the recorded account needs: tickers, the exchangeInfo of
// both futures markets, and, in Classic Trading, the USDⓈ-M symbol configuration.
func (r *recorder) recordMarkets(ctx context.Context) error {
	assets, err := recordedAssets(r.bodies)
	if err != nil {
		return err
	}
	tickers, err := r.fetch(ctx, epTickerPrice, nil)
	if err != nil {
		return err
	}
	note, err := trimTickers(&tickers, assets)
	if err != nil {
		return err
	}
	r.add(epTickerPrice, nil, fixture.ScenarioRecorded, tickers, &note, nil)

	umSource, cmSource := epPMUMPositionRisk, epPMCMPositionRisk
	if r.structure == adapters.StructureBinanceClassic {
		umSource, cmSource = epUMPositionRisk, epCMPositionRisk
	}
	heldUM, err := heldSymbols(r.bodies, umSource)
	if err != nil {
		return err
	}
	heldCM, err := heldSymbols(r.bodies, cmSource)
	if err != nil {
		return err
	}
	for _, info := range []struct {
		ep   endpoint
		m    market
		held map[string]bool
	}{{epUMExchangeInfo, marketUM, heldUM}, {epCMExchangeInfo, marketCM, heldCM}} {
		resp, err := r.fetch(ctx, info.ep, nil)
		if err != nil {
			return err
		}
		note, err := trimExchangeInfo(&resp, info.m, info.held)
		if err != nil {
			return err
		}
		r.add(info.ep, nil, fixture.ScenarioRecorded, resp, &note, nil)
	}

	if len(r.bodies[epUMPositionRisk]) == 0 {
		return nil
	}
	config, err := r.fetch(ctx, epUMSymbolConfig, nil)
	if err != nil {
		return err
	}
	keep := map[string]bool{"BTCUSDT": true}
	for symbol := range heldUM {
		keep[symbol] = true
	}
	kept, total, err := trimSymbols(&config, func(symbol string) bool { return keep[symbol] })
	if err != nil {
		return err
	}
	note = fmt.Sprintf("%d of %d symbols: held contracts plus BTCUSDT; kept %s", len(kept), total, strings.Join(kept, ", "))
	r.add(epUMSymbolConfig, nil, fixture.ScenarioRecorded, config, &note, nil)
	return nil
}

func redactUID(body []byte) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("decode %s for redaction: %w", epSpotAccount, err)
	}
	fields["uid"] = json.RawMessage(strconv.Itoa(RedactedUID))
	return json.Marshal(fields)
}

// recordedAssets lists every coin the recorded Accounts hold, through the adapter's own mappings.
func recordedAssets(bodies map[endpoint][][]byte) (map[string]bool, error) {
	var balances []adapters.Balance
	pmPages, err := decodeAll[[]pmBalance](bodies, epPMBalance)
	if err != nil {
		return nil, err
	}
	for _, rows := range pmPages {
		mapped, err := mapPMBalances(rows)
		if err != nil {
			return nil, err
		}
		balances = append(balances, mapped...)
	}
	for _, ep := range []endpoint{epUserAsset, epFundingAsset} {
		pages, err := decodeAll[[]walletAsset](bodies, ep)
		if err != nil {
			return nil, err
		}
		for _, rows := range pages {
			mapped, err := mapWallet(ep, rows)
			if err != nil {
				return nil, err
			}
			balances = append(balances, mapped...)
		}
	}
	for _, ep := range []endpoint{epUMAccount, epCMAccount} {
		accounts, err := decodeAll[futuresAccount](bodies, ep)
		if err != nil {
			return nil, err
		}
		for i := range accounts {
			mapped, err := mapFuturesAssets(ep, accounts[i].Assets)
			if err != nil {
				return nil, err
			}
			balances = append(balances, mapped...)
		}
	}
	margin := newMarginTotals()
	crosses, err := decodeAll[crossMarginAccount](bodies, epCrossMargin)
	if err != nil {
		return nil, err
	}
	for i := range crosses {
		for j := range crosses[i].UserAssets {
			if err := margin.add(epCrossMargin, &crosses[i].UserAssets[j]); err != nil {
				return nil, err
			}
		}
	}
	isolated, err := decodeAll[isolatedMarginAccount](bodies, epIsolatedMargin)
	if err != nil {
		return nil, err
	}
	for i := range isolated {
		for j := range isolated[i].Assets {
			for _, side := range []*marginAsset{&isolated[i].Assets[j].Base, &isolated[i].Assets[j].Quote} {
				if err := margin.add(epIsolatedMargin, side); err != nil {
					return nil, err
				}
			}
		}
	}
	balances = append(balances, margin.balances()...)

	assets := map[string]bool{}
	for _, b := range balances {
		assets[strings.ToUpper(b.Symbol)] = true
	}
	for _, ep := range []endpoint{epFlexibleEarn, epLockedEarn} {
		pages, err := decodeAll[earnPage](bodies, ep)
		if err != nil {
			return nil, err
		}
		for _, page := range pages {
			for _, row := range page.Rows {
				assets[strings.ToUpper(row.Asset)] = true
			}
		}
	}
	dciPages, err := decodeAll[dualInvestmentPage](bodies, epDualInvest)
	if err != nil {
		return nil, err
	}
	for _, page := range dciPages {
		for _, row := range page.List {
			assets[strings.ToUpper(row.InvestCoin)] = true
		}
	}
	delete(assets, "")
	return assets, nil
}

// decodeAll decodes every recorded answer of an endpoint.
func decodeAll[T any](bodies map[endpoint][][]byte, ep endpoint) ([]T, error) {
	out := make([]T, 0, len(bodies[ep]))
	for _, body := range bodies[ep] {
		var v T
		if err := json.Unmarshal(body, &v); err != nil {
			return nil, fmt.Errorf("decode %s: %w", ep, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// trimSymbols keeps the rows of a JSON array whose "symbol" keep accepts, in their recorded order,
// and returns the kept symbols and the recorded row count.
func trimSymbols(resp *response, keep func(symbol string) bool) (kept []string, total int, err error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(resp.body, &rows); err != nil {
		return nil, 0, err
	}
	out := make([]json.RawMessage, 0, len(rows))
	for _, raw := range rows {
		var row struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, 0, err
		}
		if keep(row.Symbol) {
			out = append(out, raw)
			kept = append(kept, row.Symbol)
		}
	}
	if resp.body, err = json.Marshal(out); err != nil {
		return nil, 0, err
	}
	return kept, len(rows), nil
}

// trimTickers keeps the USDT and USDC pairs of recorded assets plus a coverage sample: BTCUSDT,
// ETHUSDT, FDUSDUSDT, and the first coin quoted in USDC but not in USDT.
func trimTickers(resp *response, assets map[string]bool) (string, error) {
	var rows []struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(resp.body, &rows); err != nil {
		return "", err
	}
	listed := map[string]bool{}
	sorted := make([]string, 0, len(rows))
	for _, row := range rows {
		listed[row.Symbol] = true
		sorted = append(sorted, row.Symbol)
	}
	sort.Strings(sorted)
	keep := map[string]bool{"BTCUSDT": true, "ETHUSDT": true, "FDUSDUSDT": true}
	for asset := range assets {
		for _, quote := range quotes {
			keep[asset+quote] = true
		}
	}
	usdcOnly := ""
	for _, s := range sorted {
		base, ok := strings.CutSuffix(s, "USDC")
		if ok && base != "" && !listed[base+"USDT"] && !adapters.USDEquivalent(base) {
			usdcOnly = s
			break
		}
	}
	if usdcOnly != "" {
		keep[usdcOnly] = true
	}
	kept, total, err := trimSymbols(resp, func(symbol string) bool { return keep[symbol] })
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d of %d symbols: USDT/USDC pairs of recorded assets plus BTCUSDT, ETHUSDT, FDUSDUSDT, and USDC-only %s; kept %s",
		len(kept), total, usdcOnly, strings.Join(kept, ", ")), nil
}

// heldSymbols lists the contracts with an open position in the recorded answers of a position
// endpoint.
func heldSymbols(bodies map[endpoint][][]byte, ep endpoint) (map[string]bool, error) {
	pages, err := decodeAll[[]positionRisk](bodies, ep)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for _, rows := range pages {
		for i := range rows {
			if rat, ok := decimal.Parse(rows[i].PositionAmt); ok && rat.Sign() != 0 {
				held[rows[i].Symbol] = true
			}
		}
	}
	return held, nil
}

// trimExchangeInfo keeps the held contracts plus a coverage sample: for UM, BTCUSDT and the first
// CURRENT_QUARTER and TRADIFI_PERPETUAL contracts; for CM, BTCUSD_PERP and the first
// CURRENT_QUARTER contract. Other top-level fields are kept as recorded.
func trimExchangeInfo(resp *response, m market, held map[string]bool) (string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(resp.body, &top); err != nil {
		return "", err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(top["symbols"], &rows); err != nil {
		return "", err
	}
	type meta struct {
		Symbol       string `json:"symbol"`
		ContractType string `json:"contractType"`
	}
	metas := make([]meta, len(rows))
	for i, raw := range rows {
		if err := json.Unmarshal(raw, &metas[i]); err != nil {
			return "", err
		}
	}
	keep := map[string]bool{}
	for s := range held {
		keep[s] = true
	}
	samples := []string{"CURRENT_QUARTER"}
	if m == marketUM {
		keep["BTCUSDT"] = true
		samples = append(samples, "TRADIFI_PERPETUAL")
	} else {
		keep["BTCUSD_PERP"] = true
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return metas[order[i]].Symbol < metas[order[j]].Symbol })
	for _, contractType := range samples {
		for _, i := range order {
			if metas[i].ContractType == contractType {
				keep[metas[i].Symbol] = true
				break
			}
		}
	}
	kept := make([]json.RawMessage, 0, len(keep))
	var names []string
	for i, raw := range rows {
		if keep[metas[i].Symbol] {
			kept = append(kept, raw)
			names = append(names, metas[i].Symbol)
		}
	}
	symbols, err := json.Marshal(kept)
	if err != nil {
		return "", err
	}
	top["symbols"] = symbols
	if resp.body, err = json.Marshal(top); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d of %d symbols: held contracts plus the coverage sample; kept %s", len(kept), len(rows), strings.Join(names, ", ")), nil
}
