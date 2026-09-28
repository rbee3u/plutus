package okx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Placeholders for identifying values in recorded fixtures: the account and main-account UIDs,
// the API key label, and any IP address (a TEST-NET-3 documentation address).
const (
	RedactedUID     = "100000000000000001"
	RedactedMainUID = "100000000000000002"
	RedactedLabel   = "plutus-fixture"
	RedactedIP      = "203.0.113.1"
)

var ipv4 = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

// recordEnvelope keeps every top-level field of an answer while its rows are trimmed or redacted.
type recordEnvelope map[string]json.RawMessage

// Record calls every endpoint the adapter uses once, every page of the dual investment order
// history the adapter would read, trims the large market-data lists to the symbols the account
// needs plus a fixed coverage sample, replaces identifying values, and captures four safe error
// answers. Each recording names the Account Structure and the Margin Mode the account reports. It
// never runs inside serve, the worker, or collect.
func (a *Adapter) Record(ctx context.Context, creds adapters.Credentials) ([]fixture.Recording, error) {
	c := a.client(creds)
	var out []fixture.Recording
	add := func(ep endpoint, scenario string, resp response, trimmed *string, redacted []string) {
		out = append(out, fixture.Recording{
			Name: recordingName(ep, scenario), Endpoint: ep.String(), Scenario: scenario,
			Status: resp.status, Trimmed: trimmed, Redacted: redacted, Body: json.RawMessage(resp.body),
		})
	}
	take := func(ep endpoint) (response, error) {
		resp, err := c.fetch(ctx, ep)
		if err != nil {
			return response{}, fmt.Errorf("%s: %w", ep, err)
		}
		if err := decode(ep, resp, &json.RawMessage{}); err != nil {
			return response{}, err
		}
		return resp, nil
	}

	serverTime, err := take(epTime)
	if err != nil {
		return nil, err
	}
	add(epTime, fixture.ScenarioRecorded, serverTime, nil, nil)

	config, err := take(epConfig)
	if err != nil {
		return nil, err
	}
	uids, mode, err := redactConfig(&config)
	if err != nil {
		return nil, err
	}
	add(epConfig, fixture.ScenarioRecorded, config, nil, []string{"uid", "mainUid", "ip", "label"})

	bodies := map[endpoint][]byte{}
	for _, ep := range []endpoint{epBalance, epPositions, epFunding, epSavings, epStaking, epValuation} {
		resp, err := take(ep)
		if err != nil {
			return nil, err
		}
		bodies[ep] = resp.body
		add(ep, fixture.ScenarioRecorded, resp, nil, nil)
	}

	// Every page the adapter reads, spaced as the adapter spaces them.
	var orders []dcdOrder
	for before, page := "", 0; page < dcdMaxPages; page++ {
		if page > 0 {
			if err := a.wait(ctx); err != nil {
				return nil, err
			}
		}
		ep := dcdPage(before)
		resp, err := take(ep)
		if err != nil {
			return nil, err
		}
		add(ep, fixture.ScenarioRecorded, resp, nil, nil)
		rows, err := rowsOf[dcdOrder](ep, resp.body)
		if err != nil {
			return nil, err
		}
		if before, err = dcdCursor(rows, before); err != nil {
			return nil, err
		}
		orders = append(orders, rows...)
		if len(rows) < dcdPageSize {
			break
		}
	}

	assets, err := recordedAssets(bodies, orders)
	if err != nil {
		return nil, err
	}
	tickers, err := take(epTickers)
	if err != nil {
		return nil, err
	}
	note, err := trimTickers(&tickers, assets)
	if err != nil {
		return nil, err
	}
	add(epTickers, fixture.ScenarioRecorded, tickers, &note, nil)

	held, err := heldContracts(bodies[epPositions])
	if err != nil {
		return nil, err
	}
	for _, m := range []market{marketSwap, marketFutures} {
		ep := epSwaps
		if m == marketFutures {
			ep = epFutures
		}
		resp, err := take(ep)
		if err != nil {
			return nil, err
		}
		note, err := trimInstruments(&resp, m, held)
		if err != nil {
			return nil, err
		}
		add(ep, fixture.ScenarioRecorded, resp, &note, nil)
	}

	// Four error answers that are safe to provoke: a key OKX does not know, a wrong signature, a
	// wrong passphrase, and a timestamp far outside the 30-second window.
	for _, scenario := range []struct {
		name, key, secret, passphrase, timestamp string
	}{
		{"invalid_key", "00000000-0000-0000-0000-000000000000", creds.APISecret, creds.Passphrase, ""},
		{"bad_signature", creds.APIKey, "plutus-fixture-wrong-secret", creds.Passphrase, ""},
		{"wrong_passphrase", creds.APIKey, creds.APISecret, "plutus-fixture-wrong-passphrase", ""},
		{"stale_timestamp", creds.APIKey, creds.APISecret, creds.Passphrase, a.clock.now().Add(-5 * time.Minute).UTC().Format(timestampLayout)},
	} {
		attempt := adapters.Credentials{APIKey: scenario.key, APISecret: scenario.secret, Passphrase: scenario.passphrase}
		resp, err := c.send(ctx, epConfig, attempt, scenario.timestamp)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", epConfig, scenario.name, err)
		}
		if decode(epConfig, resp, &json.RawMessage{}) == nil {
			return nil, fmt.Errorf("%s %s: OKX accepted the request", epConfig, scenario.name)
		}
		add(epConfig, scenario.name, resp, nil, nil)
	}

	for i := range out {
		out[i].AccountStructure = string(adapters.StructureOKXUnified)
		out[i].MarginMode = string(mode)
		if ipv4.Match(out[i].Body) {
			out[i].Body = ipv4.ReplaceAll(out[i].Body, []byte(RedactedIP))
			out[i].Redacted = append(out[i].Redacted, "ip addresses")
		}
		for _, secret := range append([]string{creds.APIKey, creds.APISecret, creds.Passphrase}, uids...) {
			if secret != "" && bytes.Contains(out[i].Body, []byte(secret)) {
				return nil, fmt.Errorf("%s: a credential or account UID survived redaction; nothing was written", out[i].Name)
			}
		}
	}
	return out, nil
}

// recordingName is the file stem of a recording: the endpoint alone for a successful answer, and
// with the scenario appended for an error answer, so both live side by side in testdata.
func recordingName(ep endpoint, scenario string) string {
	if scenario == fixture.ScenarioRecorded {
		scenario = ""
	}
	return fixture.Name(ep.request(), scenario)
}

// redactConfig replaces the UIDs, the key's IP whitelist, and its label, and returns the raw UIDs
// for the final leak check and the Margin Mode of the recorded account.
func redactConfig(resp *response) ([]string, adapters.MarginMode, error) {
	var top recordEnvelope
	if err := json.Unmarshal(resp.body, &top); err != nil {
		return nil, "", fmt.Errorf("decode %s for redaction: %w", epConfig, err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(top["data"], &rows); err != nil {
		return nil, "", fmt.Errorf("decode %s rows for redaction: %w", epConfig, err)
	}
	if len(rows) == 0 {
		return nil, "", fmt.Errorf("%s has no rows to redact", epConfig)
	}
	row := rows[0]
	var cfg accountConfig
	raw, err := json.Marshal(row)
	if err != nil {
		return nil, "", err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, "", err
	}
	mode, err := marginModeOf(cfg.AcctLv)
	if err != nil {
		return nil, "", fmt.Errorf("the recorded account: %w", err)
	}
	mainUID := RedactedMainUID
	if cfg.MainUID == cfg.UID {
		mainUID = RedactedUID
	}
	for name, value := range map[string]string{"uid": RedactedUID, "mainUid": mainUID, "ip": "", "label": RedactedLabel} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, "", err
		}
		row[name] = encoded
	}
	if top["data"], err = json.Marshal(rows); err != nil {
		return nil, "", err
	}
	if resp.body, err = json.Marshal(top); err != nil {
		return nil, "", err
	}
	return []string{cfg.UID, cfg.MainUID}, mode, nil
}

// rowsOf decodes the data rows of a recorded answer.
func rowsOf[T any](ep endpoint, body []byte) ([]T, error) {
	var rows []T
	if err := decode(ep, response{status: http.StatusOK, body: body}, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// recordedAssets lists every coin that appears in the recorded accounts and dual investment
// orders.
func recordedAssets(bodies map[endpoint][]byte, orders []dcdOrder) (map[string]bool, error) {
	assets := map[string]bool{}
	accounts, err := rowsOf[tradingAccount](epBalance, bodies[epBalance])
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		balances, err := mapTradingBalances(accounts[i].Details)
		if err != nil {
			return nil, err
		}
		for _, b := range balances {
			assets[strings.ToUpper(b.Symbol)] = true
		}
	}
	funding, err := rowsOf[fundingBalance](epFunding, bodies[epFunding])
	if err != nil {
		return nil, err
	}
	for _, row := range funding {
		assets[strings.ToUpper(row.Ccy)] = true
	}
	savings, err := rowsOf[savingsBalance](epSavings, bodies[epSavings])
	if err != nil {
		return nil, err
	}
	for _, row := range savings {
		assets[strings.ToUpper(row.Ccy)] = true
	}
	staking, err := rowsOf[stakingOrder](epStaking, bodies[epStaking])
	if err != nil {
		return nil, err
	}
	for _, order := range staking {
		for _, invested := range order.InvestData {
			assets[strings.ToUpper(invested.Ccy)] = true
		}
	}
	for i := range orders {
		order := &orders[i]
		assets[strings.ToUpper(strings.TrimSpace(order.NotionalCcy))] = true
	}
	delete(assets, "")
	return assets, nil
}

// trimTickers keeps the -USDT and -USDC pairs of recorded coins plus a coverage sample: BTC-USDT,
// ETH-USDT, and the first coin quoted in USDC but not in USDT.
func trimTickers(resp *response, assets map[string]bool) (string, error) {
	return trimRows(resp, func(ids []string, _ []map[string]string) map[string]bool {
		listed := map[string]bool{}
		for _, id := range ids {
			listed[id] = true
		}
		keep := map[string]bool{"BTC-USDT": true, "ETH-USDT": true}
		for asset := range assets {
			keep[asset+"-USDT"] = true
			keep[asset+"-USDC"] = true
		}
		sorted := append([]string(nil), ids...)
		sort.Strings(sorted)
		for _, id := range sorted {
			base, ok := strings.CutSuffix(id, "-USDC")
			if ok && base != "" && !listed[base+"-USDT"] && !adapters.USDEquivalent(base) {
				keep[id] = true
				break
			}
		}
		return keep
	}, "-USDT/-USDC pairs of recorded coins plus BTC-USDT, ETH-USDT, and the first USDC-only coin")
}

// heldContracts lists the swap and futures contracts of the recorded open positions.
func heldContracts(body []byte) (map[string]bool, error) {
	rows, err := rowsOf[openPosition](epPositions, body)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for i := range rows {
		if rat, ok := decimal.Parse(rows[i].Pos); ok && rat.Sign() != 0 {
			held[rows[i].InstID] = true
		}
	}
	return held, nil
}

// trimInstruments keeps the held contracts plus a kind coverage sample: for SWAP, BTC-USDT-SWAP
// (linear) and BTC-USD-SWAP (inverse); for FUTURES, the first linear and inverse BTC contract.
func trimInstruments(resp *response, m market, held map[string]bool) (string, error) {
	return trimRows(resp, func(ids []string, rows []map[string]string) map[string]bool {
		keep := map[string]bool{}
		for id := range held {
			keep[id] = true
		}
		if m == marketSwap {
			keep["BTC-USDT-SWAP"] = true
			keep["BTC-USD-SWAP"] = true
			return keep
		}
		order := make([]int, len(ids))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(i, j int) bool { return ids[order[i]] < ids[order[j]] })
		for _, want := range []string{"linear", "inverse"} {
			for _, i := range order {
				if rows[i]["ctType"] == want && strings.HasPrefix(ids[i], "BTC-") {
					keep[ids[i]] = true
					break
				}
			}
		}
		return keep
	}, "held contracts plus the kind coverage sample")
}

// trimRows keeps the data rows whose instId the pick function selects. Other top-level fields are
// kept as recorded.
func trimRows(resp *response, pick func(ids []string, rows []map[string]string) map[string]bool, rule string) (string, error) {
	var top recordEnvelope
	if err := json.Unmarshal(resp.body, &top); err != nil {
		return "", err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(top["data"], &raw); err != nil {
		return "", err
	}
	ids := make([]string, len(raw))
	rows := make([]map[string]string, len(raw))
	for i := range raw {
		var fields map[string]any
		if err := json.Unmarshal(raw[i], &fields); err != nil {
			return "", err
		}
		rows[i] = map[string]string{}
		for k, v := range fields {
			if s, ok := v.(string); ok {
				rows[i][k] = s
			}
		}
		ids[i] = rows[i]["instId"]
	}
	keep := pick(ids, rows)
	kept := make([]json.RawMessage, 0, len(keep))
	var names []string
	for i := range raw {
		if ids[i] != "" && keep[ids[i]] {
			kept = append(kept, raw[i])
			names = append(names, ids[i])
		}
	}
	var err error
	if top["data"], err = json.Marshal(kept); err != nil {
		return "", err
	}
	if resp.body, err = json.Marshal(top); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d of %d rows: %s; kept %s", len(kept), len(raw), rule, strings.Join(names, ", ")), nil
}
