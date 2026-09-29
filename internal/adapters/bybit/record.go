package bybit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

// Placeholders for identifying values in recorded fixtures. redactedIP is a TEST-NET-3
// documentation address.
const (
	redactedNumber = json.Number("100001")
	redactedString = "redacted"
	redactedIP     = "203.0.113.1"
	// keyEchoes, identifierEchoes, and ipAddresses name, in a fixture header, the replacement of
	// the API key or secret, of an identifier value, and of an IP address wherever a response
	// repeats it.
	keyEchoes        = "API key and secret echoes"
	identifierEchoes = "identifier echoes"
	ipAddresses      = "IP addresses"
)

// The scenarios of the three error answers the recorder provokes, each named after its cause.
const (
	scenarioInvalidKey     = "invalid_key"
	scenarioBadSignature   = "bad_signature"
	scenarioStaleTimestamp = "stale_timestamp"
)

// ipv4 matches an IPv4 address inside a recorded string.
var ipv4 = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

// trimNote is the header note of every market-data list the recorder shortened.
const trimNote = "Keeps the recorded account's coins and contracts plus BTC and ETH examples."

// identifierFields are the response fields that identify the account, the key, or an order. The
// recorder replaces their values, and every other occurrence of those values, in every fixture.
var identifierFields = map[string]bool{
	"id": true, "note": true, "apiKey": true, "secret": true, "ips": true, "userID": true,
	"inviterID": true, "affiliateID": true, "parentUid": true, "rsaPublicKey": true, "kycRegion": true,
	"memberId": true, "positionId": true, "orderId": true, "smpGroup": true,
}

// held are the coins and contracts the recorder keeps in the market-data lists.
type held struct {
	coins     map[string]bool
	contracts map[string]bool
}

// exchange is one recorded request and its answer.
type exchange struct {
	request  string
	scenario string
	resp     response
}

// Record calls every endpoint the adapter uses as Validate and Collect call them with every
// Account selected, plus both contract lists, and provokes three refusals. It trims the
// market-data lists to the recorded account's coins and contracts plus BTC and ETH examples, and
// replaces identifying values. It never runs inside serve, the worker, or collect.
func (a *Adapter) Record(ctx context.Context, creds adapters.Credentials) ([]fixture.Recording, error) {
	c := a.client(creds)
	var exchanges []exchange
	index := map[string]int{}
	c.capture = func(request string, resp response) {
		if i, ok := index[request]; ok {
			exchanges[i].resp = resp
			return
		}
		index[request] = len(exchanges)
		exchanges = append(exchanges, exchange{request: request, scenario: fixture.ScenarioRecorded, resp: resp})
	}
	structure := adapters.StructureBybitUTA2
	validation, err := a.validate(ctx, c, structure)
	if err != nil {
		return nil, err
	}
	batch, err := a.collect(ctx, c, structure, adapters.AccountTypeIDs(structure))
	if err != nil {
		return nil, err
	}
	// Collect reads contracts only for categories with an open position; both lists are recorded
	// so that replayed positions of any category find their contract.
	for _, category := range []string{"linear", "inverse"} {
		if _, err := a.instruments(ctx, c, category); err != nil {
			return nil, err
		}
	}
	c.capture = nil
	refused, err := refusals(ctx, c)
	if err != nil {
		return nil, err
	}
	exchanges = append(exchanges, refused...)

	markets := heldMarkets(&batch)
	identifiers := map[string]bool{}
	decoded := make([]any, len(exchanges))
	for i := range exchanges {
		value, err := decodeRaw(exchanges[i].resp.body)
		if err != nil {
			if exchanges[i].scenario == fixture.ScenarioRecorded {
				return nil, fmt.Errorf("GET %s: %w", exchanges[i].request, err)
			}
			// A refusal without a JSON body, such as a gateway page, is kept as one JSON string.
			value = string(exchanges[i].resp.body)
		}
		collectIdentifiers(value, false, identifiers)
		decoded[i] = value
	}
	secrets := []string{creds.APIKey, creds.APISecret}
	recordings := make([]fixture.Recording, 0, len(exchanges))
	for i := range exchanges {
		ex := &exchanges[i]
		r := redactor{identifiers: identifiers, secrets: secrets, markets: markets, fields: map[string]bool{}}
		body, err := json.Marshal(r.clean(decoded[i]))
		if err != nil {
			return nil, err
		}
		for _, secret := range append([]string{validation.ExternalRef}, secrets...) {
			if len(secret) >= minEchoLength && bytes.Contains(body, []byte(secret)) {
				return nil, fmt.Errorf("GET %s: a credential or the account UID survived redaction; nothing was written", ex.request)
			}
		}
		var trimmed *string
		if r.trimmed {
			note := trimNote
			trimmed = &note
		}
		scenario := ""
		if ex.scenario != fixture.ScenarioRecorded {
			scenario = ex.scenario
		}
		recordings = append(recordings, fixture.Recording{
			Name:             fixtureName(ex.request, scenario),
			Endpoint:         "GET " + ex.request,
			AccountStructure: string(structure),
			MarginMode:       string(validation.MarginMode),
			Scenario:         ex.scenario,
			Status:           ex.resp.status,
			Trimmed:          trimmed,
			Redacted:         r.redacted(),
			Body:             body,
		})
	}
	return recordings, nil
}

// RecordSelected reads only one account or product request after the clock, structure, and key
// checks needed for its provenance. Prerequisite answers are used for redaction but not returned.
func (a *Adapter) RecordSelected(ctx context.Context, creds adapters.Credentials, name string) ([]fixture.Recording, error) {
	c := a.client(creds)
	var captured []exchange
	c.capture = func(request string, resp response) {
		captured = append(captured, exchange{request: request, scenario: fixture.ScenarioRecorded, resp: resp})
	}
	validation, err := a.validate(ctx, c, adapters.StructureBybitUTA2)
	if err != nil {
		return nil, err
	}
	selected := -1
	for i := range captured {
		if fixtureName(captured[i].request, "") == name {
			selected = i
			break
		}
	}
	if selected < 0 {
		for _, call := range []struct {
			ep     endpoint
			params url.Values
		}{
			{epWallet, nil},
			{epFunding, nil},
			{epFlexible, nil},
			{epFixedTerm, nil},
			{epDualAssets, nil},
			{epAssetOverview, nil},
			{epPositions, url.Values{"category": {"linear"}, "settleCoin": {"USDT"}}},
			{epPositions, url.Values{"category": {"linear"}, "settleCoin": {"USDC"}}},
			{epPositions, url.Values{"category": {"inverse"}}},
			{epPositions, url.Values{"category": {"option"}}},
		} {
			if fixtureName(call.ep.request(call.params), "") != name {
				continue
			}
			var result json.RawMessage
			if err := c.call(ctx, call.ep, call.params, &result); err != nil {
				return nil, err
			}
			selected = len(captured) - 1
			break
		}
	}
	if selected < 0 {
		return nil, fmt.Errorf("%q is not a first-page account or product fixture; market-data, later pages, and refusal fixtures need a full recording", name)
	}
	identifiers := map[string]bool{}
	for i := range captured {
		value, err := decodeRaw(captured[i].resp.body)
		if err != nil {
			return nil, err
		}
		collectIdentifiers(value, false, identifiers)
	}
	ex := captured[selected]
	value, err := decodeRaw(ex.resp.body)
	if err != nil {
		return nil, err
	}
	r := redactor{identifiers: identifiers, secrets: []string{creds.APIKey, creds.APISecret}, fields: map[string]bool{}}
	body, err := json.Marshal(r.clean(value))
	if err != nil {
		return nil, err
	}
	for _, secret := range []string{validation.ExternalRef, creds.APIKey, creds.APISecret} {
		if len(secret) >= minEchoLength && bytes.Contains(body, []byte(secret)) {
			return nil, fmt.Errorf("GET %s: a credential or the account UID survived redaction; nothing was written", ex.request)
		}
	}
	return []fixture.Recording{{
		Name: name, Endpoint: "GET " + ex.request, AccountStructure: string(adapters.StructureBybitUTA2),
		MarginMode: string(validation.MarginMode), Scenario: fixture.ScenarioRecorded,
		Status: ex.resp.status, Redacted: r.redacted(), Body: body,
	}}, nil
}

// fixtureName is the file stem of a recorded request. fixture.Name appends each query value as
// sent, but Bybit returns a page cursor percent-encoded and the request escapes it once more, so
// the stem takes each value unescaped once, as the adapter passed it. A value that would then
// hold a path separator or an ampersand keeps its escaped form.
func fixtureName(request, scenario string) string {
	path, query, found := strings.Cut(request, "?")
	if !found {
		return fixture.Name(request, scenario)
	}
	params := strings.Split(query, "&")
	for i, param := range params {
		key, value, _ := strings.Cut(param, "=")
		if raw, err := url.QueryUnescape(value); err == nil && !strings.ContainsAny(raw, `/\&`) {
			params[i] = key + "=" + raw
		}
	}
	return fixture.Name(path+"?"+strings.Join(params, "&"), scenario)
}

// refusals provokes the three error answers that are safe to provoke, on the call every validation
// and Collection Run starts with: a key Bybit does not know, a wrong signature, and a timestamp a
// minute old, far outside the receive window. The clock was measured by the calls before.
func refusals(ctx context.Context, c *client) ([]exchange, error) {
	request := epAccountInfo.request(nil)
	stale := strconv.FormatInt(c.clock.corrected().Add(-time.Minute).UnixMilli(), 10)
	var out []exchange
	for _, attempt := range []struct {
		scenario, key, secret, timestamp string
	}{
		{scenarioInvalidKey, strings.Repeat("0", 18), c.creds.APISecret, ""},
		{scenarioBadSignature, c.creds.APIKey, "plutus-fixture-wrong-secret", ""},
		{scenarioStaleTimestamp, c.creds.APIKey, c.creds.APISecret, stale},
	} {
		creds := adapters.Credentials{APIKey: attempt.key, APISecret: attempt.secret}
		resp, err := c.send(ctx, epAccountInfo, request, creds, attempt.timestamp)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", epAccountInfo, attempt.scenario, err)
		}
		if decode(epAccountInfo, resp, &json.RawMessage{}) == nil {
			return nil, fmt.Errorf("%s %s: Bybit accepted the request", epAccountInfo, attempt.scenario)
		}
		out = append(out, exchange{request: request, scenario: attempt.scenario, resp: resp})
	}
	return out, nil
}

// heldMarkets lists the coins and contracts of a batch, plus BTC and ETH as examples.
func heldMarkets(batch *adapters.Batch) held {
	out := held{coins: map[string]bool{"BTC": true, "ETH": true}, contracts: map[string]bool{}}
	for i := range batch.Reports {
		report := &batch.Reports[i]
		for _, balance := range report.Balances {
			out.coins[balance.Symbol] = true
		}
		for j := range report.Positions {
			out.contracts[report.Positions[j].Instrument.Symbol] = true
		}
	}
	return out
}

func decodeRaw(raw []byte) (any, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, adapters.NewError(adapters.ClassNormalization, "the response is not JSON")
	}
	return value, nil
}

// minEchoLength is the shortest identifier value replaced wherever else it appears. UIDs, UUIDs,
// and keys are longer; a short id such as a position number could equal an unrelated amount.
const minEchoLength = 8

// collectIdentifiers gathers every long value found under an identifier field, so the same value
// is replaced wherever else it appears as a whole string or number.
func collectIdentifiers(value any, identifying bool, out map[string]bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			collectIdentifiers(item, identifying || identifierFields[key], out)
		}
	case []any:
		for _, item := range v {
			collectIdentifiers(item, identifying, out)
		}
	case string:
		if identifying && len(v) >= minEchoLength {
			out[v] = true
		}
	case json.Number:
		if identifying && len(v) >= minEchoLength {
			out[string(v)] = true
		}
	}
}

// redactor rewrites one decoded response.
type redactor struct {
	identifiers map[string]bool
	secrets     []string
	markets     held
	fields      map[string]bool
	trimmed     bool
}

// clean replaces identifier fields, identifier values, credential echoes, and IP addresses, and
// trims market-data lists.
func (r *redactor) clean(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if identifierFields[key] {
				v[key] = placeholder(item)
				r.fields[key] = true
				continue
			}
			v[key] = r.clean(item)
		}
		if list, ok := v["list"].([]any); ok {
			v["list"] = r.trim(list)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = r.clean(item)
		}
		return v
	case string:
		if r.identifiers[v] {
			r.fields[identifierEchoes] = true
			return redactedString
		}
		for _, secret := range r.secrets {
			if len(secret) >= minEchoLength && strings.Contains(v, secret) {
				v = strings.ReplaceAll(v, secret, redactedString)
				r.fields[keyEchoes] = true
			}
		}
		if ipv4.MatchString(v) {
			v = ipv4.ReplaceAllString(v, redactedIP)
			r.fields[ipAddresses] = true
		}
		return v
	case json.Number:
		if r.identifiers[string(v)] {
			r.fields[identifierEchoes] = true
			return redactedNumber
		}
		return v
	default:
		return v
	}
}

// trim keeps the rows of a ticker or contract list that the recorded account needs: its coins'
// USDT and USDC pairs, its contracts, and BTC and ETH examples. Other lists are kept whole.
func (r *redactor) trim(list []any) []any {
	kept := make([]any, 0, len(list))
	for _, item := range list {
		row, ok := item.(map[string]any)
		symbol, _ := row["symbol"].(string)
		_, isContract := row["contractType"]
		_, isTicker := row["lastPrice"]
		if !ok || symbol == "" || (!isContract && !isTicker) || r.keeps(symbol) {
			kept = append(kept, item)
		}
	}
	if len(kept) < len(list) {
		r.trimmed = true
	}
	return kept
}

func (r *redactor) keeps(symbol string) bool {
	if r.markets.contracts[symbol] || strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH") {
		return true
	}
	for _, quote := range quotes {
		if base, ok := strings.CutSuffix(symbol, quote); ok && r.markets.coins[base] {
			return true
		}
	}
	return false
}

// redacted lists, sorted, the fields and echoes the redactor replaced.
func (r *redactor) redacted() []string {
	out := make([]string, 0, len(r.fields))
	for field := range r.fields {
		out = append(out, field)
	}
	sort.Strings(out)
	return out
}

// placeholder replaces an identifying value with one of the same JSON type.
func placeholder(value any) any {
	switch value.(type) {
	case json.Number:
		return redactedNumber
	case []any:
		return []any{}
	case map[string]any:
		return map[string]any{}
	case bool, nil:
		return value
	default:
		return redactedString
	}
}
