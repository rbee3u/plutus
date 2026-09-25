package kraken

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

// exchange is one raw request and answer captured by the recorder.
type exchange struct {
	ep   endpoint
	resp response
}

// Credentials the recorder provokes refusals with. Neither is a real credential: no account has
// the key, and the secret signs wrongly.
var (
	refusedKey    = strings.Repeat("A", 56)
	refusedSecret = base64.StdEncoding.EncodeToString([]byte("plutus-fixture-wrong-secret"))
)

// refusal is one error answer the recorder provokes on a key check.
type refusal struct {
	scenario string
	ep       endpoint
	creds    adapters.Credentials
	nonce    string
}

// refusals are the error answers that are safe to provoke: each request is refused before it
// reads anything. Spot is asked with a key no account has, a wrong secret, and a nonce below the
// key's last one; Futures, which takes no nonce, with the first two. Rate-limit, permission, and
// service errors cannot be provoked safely; documented answers stand in for them.
func refusals(creds adapters.Credentials) []refusal {
	return []refusal{
		{"invalid_key", epAPIKeyInfo, adapters.Credentials{APIKey: refusedKey, APISecret: creds.APISecret}, ""},
		{"bad_signature", epAPIKeyInfo, adapters.Credentials{APIKey: creds.APIKey, APISecret: refusedSecret}, ""},
		{"invalid_nonce", epAPIKeyInfo, adapters.Credentials{APIKey: creds.APIKey, APISecret: creds.APISecret}, "1"},
		{"invalid_key", epFuturesKey, adapters.Credentials{Futures: &adapters.KeyPair{APIKey: refusedKey, APISecret: creds.FuturesSecret()}}, ""},
		{"bad_signature", epFuturesKey, adapters.Credentials{Futures: &adapters.KeyPair{APIKey: creds.FuturesKey(), APISecret: refusedSecret}}, ""},
	}
}

// Record calls every endpoint the adapter uses, with the Account Structure the account reports,
// and returns one trimmed and redacted Recording per endpoint, followed by the refusals above. It
// is used only by `plutus adapter record` (ADR 0023); a raw body leaves this package through
// nothing else.
//
// On a Unified Wallet account it also records BalanceEx and TradeBalance, which only Main in
// Separate wallets reads; their header keeps the observed structure, so they do not claim to prove
// the Separate mapping. A recording in which a credential or an account identifier survives
// redaction fails instead of being returned.
func (a *Adapter) Record(ctx context.Context, creds adapters.Credentials) ([]fixture.Recording, error) {
	if creds.FuturesKey() == "" || creds.APIKey == "" {
		return nil, adapters.NewError(adapters.ClassAuthentication, "Recording Kraken needs both key groups: the Account Structure is read from the Futures balances feed.")
	}
	var captured []exchange
	c := a.client(creds)
	c.record = func(ep endpoint, resp response) error {
		captured = append(captured, exchange{ep: ep, resp: resp})
		return nil
	}
	s, err := c.balancesSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	structure, err := structureOf(&s)
	if err != nil {
		return nil, err
	}
	if _, err := a.validate(ctx, c, structure); err != nil {
		return nil, err
	}
	batch, err := a.collect(ctx, c, structure, adapters.AccountTypeIDs(structure))
	if err != nil {
		return nil, err
	}
	if structure == adapters.StructureKrakenUnified {
		var ignored json.RawMessage
		_ = c.call(ctx, epBalanceEx, nil, &ignored)
		_ = c.call(ctx, epTradeBalance, url.Values{"asset": {"ZUSD"}}, &ignored)
	}
	pages := 0
	for _, ex := range captured {
		if ex.ep == epEarnStrategies {
			pages++
		}
	}
	if pages > 1 {
		// One answer is kept per endpoint, so a paged list would lose all but its last page.
		return nil, fmt.Errorf("record %s: the strategy list spans %d pages, and the recorder keeps one answer per endpoint", epEarnStrategies, pages)
	}
	latest := latestExchanges(captured)
	filter, err := newRecordingFilter(latest, &batch)
	if err != nil {
		return nil, err
	}
	r := &recorder{structure: structure, filter: filter, secrets: secretsOf(creds, latest)}
	for _, ex := range latest {
		if err := r.add(ex.ep, fixture.ScenarioRecorded, ex.resp); err != nil {
			return nil, err
		}
	}
	attempts := refusals(creds)
	for i := range attempts {
		refused := &attempts[i]
		attempt := a.client(refused.creds)
		attempt.nonce = refused.nonce
		resp, err := attempt.exchange(ctx, refused.ep, nil)
		if err != nil {
			return nil, fmt.Errorf("record %s %s: %w", refused.ep, refused.scenario, err)
		}
		var ignored json.RawMessage
		if decode(refused.ep, resp, &ignored) == nil {
			return nil, fmt.Errorf("record %s %s: Kraken accepted the request", refused.ep, refused.scenario)
		}
		if err := r.add(refused.ep, refused.scenario, resp); err != nil {
			return nil, err
		}
	}
	return r.out, nil
}

// recorder turns captured answers into Recordings of one account.
type recorder struct {
	structure adapters.AccountStructure
	filter    *recordingFilter
	secrets   []string
	out       []fixture.Recording
}

// add trims and redacts one answer and appends its Recording. A refusal's file name carries its
// scenario, so it sits beside the endpoint's successful answer.
func (r *recorder) add(ep endpoint, scenario string, resp response) error {
	body, redacted, err := r.filter.apply(ep, resp.body, r.secrets)
	if err != nil {
		return err
	}
	for _, secret := range r.secrets {
		if bytes.Contains(body, []byte(secret)) {
			return fmt.Errorf("record %s: a credential or account identifier survived redaction", ep)
		}
	}
	name, trimmed := fixture.Name(ep.path, ""), trimNote(ep)
	if scenario != fixture.ScenarioRecorded {
		name, trimmed = fixture.Name(ep.path, scenario), nil
	}
	r.out = append(r.out, fixture.Recording{
		Name:             name,
		Endpoint:         ep.String(),
		AccountStructure: string(r.structure),
		Scenario:         scenario,
		Status:           resp.status,
		Trimmed:          trimmed,
		Redacted:         redacted,
		Body:             body,
	})
	return nil
}

// latestExchanges keeps the last answer of every endpoint, in the order the endpoints were first
// called: the balances snapshot is read by the structure check, validation, and collection, and
// the collection's snapshot is the one its reports come from.
func latestExchanges(captured []exchange) []exchange {
	index := map[endpoint]int{}
	var out []exchange
	for _, ex := range captured {
		if i, ok := index[ex.ep]; ok {
			out[i] = ex
			continue
		}
		index[ex.ep] = len(out)
		out = append(out, ex)
	}
	return out
}

// trimNote describes how the recorder shortened a public list, or is nil for a complete body.
func trimNote(ep endpoint) *string {
	var note string
	switch ep {
	case epAssets:
		note = "Kept the assets of recorded balances plus BTC, ETH, USD, USDT, and USDC."
	case epAssetPairs:
		note = "Kept the USD, USDT, and USDC pairs of the kept assets."
	case epSpotTicker:
		note = "Kept the tickers of the kept pairs."
	case epInstruments, epFuturesTickers:
		note = "Kept held contracts plus BTC and ETH futures; unheld options dropped."
	case epEarnStrategies:
		note = "Kept the strategies of recorded allocations plus the first strategy of each lock type."
	default:
		return nil
	}
	return &note
}

// recordingFilter decides what the recorder keeps of each public list: the recorded coins plus
// BTC, ETH, and the USD quotes, the held contracts plus BTC and ETH futures, and the strategies of
// recorded allocations, so a fixture stays tens of kilobytes.
type recordingFilter struct {
	assets     assetNames
	coins      map[string]bool
	pairs      map[string]bool
	contracts  map[string]bool
	options    map[string]bool
	strategies map[string]bool
}

func newRecordingFilter(latest []exchange, batch *adapters.Batch) (*recordingFilter, error) {
	f := &recordingFilter{
		assets:     assetNames{},
		coins:      map[string]bool{"BTC": true, "ETH": true, "USD": true, "USDT": true, "USDC": true},
		pairs:      map[string]bool{},
		contracts:  map[string]bool{},
		options:    map[string]bool{},
		strategies: map[string]bool{},
	}
	for i := range batch.Reports {
		for _, balance := range batch.Reports[i].Balances {
			f.coins[balance.Symbol] = true
		}
		for j := range batch.Reports[i].Positions {
			f.contracts[batch.Reports[i].Positions[j].Instrument.Symbol] = true
		}
	}
	bodies := map[endpoint][]byte{}
	for _, ex := range latest {
		bodies[ex.ep] = ex.resp.body
	}
	var assets struct {
		Result map[string]struct {
			Altname string `json:"altname"`
		} `json:"result"`
	}
	var pairs struct {
		Result map[string]struct {
			Base  string `json:"base"`
			Quote string `json:"quote"`
		} `json:"result"`
	}
	var instruments struct {
		Instruments []instrument `json:"instruments"`
	}
	var allocations struct {
		Result struct {
			Items []earnAllocation `json:"items"`
		} `json:"result"`
	}
	for ep, out := range map[endpoint]any{epAssets: &assets, epAssetPairs: &pairs, epInstruments: &instruments, epEarnAllocations: &allocations} {
		if body, ok := bodies[ep]; ok && json.Unmarshal(body, out) != nil {
			return nil, fmt.Errorf("record %s: the response is not the documented JSON", ep)
		}
	}
	for code, row := range assets.Result {
		f.assets[code] = row.Altname
	}
	for name, pair := range pairs.Result {
		base, errBase := f.assets.symbol("", pair.Base)
		quote, errQuote := f.assets.symbol("", pair.Quote)
		if errBase == nil && errQuote == nil && f.coins[base] && isPriceQuote(quote) {
			f.pairs[name] = true
		}
	}
	for _, row := range instruments.Instruments {
		if row.Type == typeOptions {
			f.options[row.Symbol] = true
		}
	}
	for _, row := range allocations.Result.Items {
		f.strategies[row.StrategyID] = true
	}
	return f, nil
}

// apply trims one body and redacts it. It returns the body and the names of the redacted fields.
func (f *recordingFilter) apply(ep endpoint, raw []byte, secrets []string) (json.RawMessage, []string, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil, nil, fmt.Errorf("record %s: the response is not a JSON object", ep)
	}
	result, _ := value["result"].(map[string]any)
	switch ep {
	case epAssets:
		for code := range result {
			if symbol, err := f.assets.symbol("", code); err != nil || !f.coins[symbol] {
				delete(result, code)
			}
		}
	case epAssetPairs, epSpotTicker:
		for name := range result {
			if !f.pairs[name] {
				delete(result, name)
			}
		}
	case epInstruments:
		value["instruments"] = f.keepContracts(value["instruments"])
	case epFuturesTickers:
		value["tickers"] = f.keepContracts(value["tickers"])
	case epEarnStrategies:
		if items, ok := result["items"].([]any); ok {
			result["items"] = f.keepStrategies(items)
		}
	}
	redactor := &redactor{secrets: secrets, fields: map[string]bool{}}
	body, err := json.Marshal(redactor.clean(value))
	if err != nil {
		return nil, nil, err
	}
	return body, redactor.names(), nil
}

// keepContracts keeps held contracts and the BTC and ETH futures that are not options.
func (f *recordingFilter) keepContracts(list any) []any {
	rows, _ := list.([]any)
	kept := []any{}
	for _, row := range rows {
		fields, ok := row.(map[string]any)
		if !ok {
			continue
		}
		symbol, _ := fields["symbol"].(string)
		pair, _ := fields["pair"].(string)
		kind, _ := fields["type"].(string)
		benchmark := strings.HasPrefix(pair, "BTC:") || strings.HasPrefix(pair, "XBT:") || strings.HasPrefix(pair, "ETH:")
		option := kind == typeOptions || f.options[symbol]
		if f.contracts[symbol] || (benchmark && !option) {
			kept = append(kept, row)
		}
	}
	return kept
}

// keepStrategies keeps the strategies of recorded allocations and the first strategy of each lock
// type, so every documented lock type keeps an example.
func (f *recordingFilter) keepStrategies(items []any) []any {
	kept := []any{}
	examples := map[string]bool{}
	for _, item := range items {
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := fields["id"].(string)
		lock, _ := fields["lock_type"].(map[string]any)
		lockType, _ := lock["type"].(string)
		if f.strategies[id] || !examples[lockType] {
			examples[lockType] = true
			kept = append(kept, item)
		}
	}
	return kept
}

// sensitiveFields are replaced wherever they appear with a value: account and key identifiers,
// key names and dates, and IP restrictions. A null or an empty list is kept.
var sensitiveFields = map[string]bool{
	"api_key": true, "apiKey": true, "api_key_name": true, "apiKeyName": true,
	"iban": true, "iiban": true, "accountUid": true, "account": true,
	"ip_allowlist": true, "ipAllowlist": true, "allowedCidrBlock": true, "allowedCidrBlocks": true,
	"last_used": true, "created_time": true, "modified_time": true, "createdAt": true,
}

// secretsOf lists what must never appear in a recording: the credentials and the account
// identifiers the answers carried before redaction. Short values and the redaction placeholder
// are skipped, so a common substring or an already redacted replay cannot block a recording.
func secretsOf(creds adapters.Credentials, latest []exchange) []string {
	candidates := []string{creds.APIKey, creds.APISecret, creds.FuturesKey(), creds.FuturesSecret()}
	var identifiers struct {
		IBAN       string `json:"iban"`
		IIBAN      string `json:"iiban"`
		AccountUID string `json:"accountUid"`
		Account    string `json:"account"`
		Result     struct {
			IBAN string `json:"iban"`
		} `json:"result"`
	}
	for _, ex := range latest {
		identifiers.IBAN, identifiers.IIBAN, identifiers.AccountUID, identifiers.Account, identifiers.Result.IBAN = "", "", "", "", ""
		if json.Unmarshal(ex.resp.body, &identifiers) != nil {
			continue
		}
		for _, id := range []string{identifiers.IBAN, identifiers.IIBAN, identifiers.AccountUID, identifiers.Account, identifiers.Result.IBAN} {
			candidates = append(candidates, id, normalizeRef(id))
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if len(candidate) < 8 || candidate == redactedText || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

// redactedText replaces a sensitive string and every echo of a credential or identifier.
const redactedText = "REDACTED"

// redactor replaces sensitive fields and credential or identifier echoes and remembers what it
// replaced.
type redactor struct {
	secrets []string
	fields  map[string]bool
}

func (r *redactor) clean(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if !sensitiveFields[key] {
				typed[key] = r.clean(item)
				continue
			}
			switch item := item.(type) {
			case nil:
				// A null names nothing and keeps Kraken's shape, such as a key check without iiban.
				continue
			case json.Number:
				typed[key] = json.Number("100001")
			case []any:
				if len(item) == 0 {
					continue
				}
				typed[key] = []any{}
			default:
				typed[key] = redactedText
			}
			r.fields[key] = true
		}
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = r.clean(item)
		}
		return typed
	case string:
		for _, secret := range r.secrets {
			if strings.Contains(typed, secret) {
				typed = strings.ReplaceAll(typed, secret, redactedText)
				r.fields["credential and identifier echoes"] = true
			}
		}
		return typed
	default:
		return value
	}
}

func (r *redactor) names() []string {
	out := make([]string, 0, len(r.fields))
	for name := range r.fields {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
