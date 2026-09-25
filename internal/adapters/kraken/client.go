package kraken

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/decimal"
)

// Production Kraken hosts. Spot and Futures are separate services with separate keys.
const (
	DefaultSpotHost     = "https://api.kraken.com"
	DefaultFuturesHost  = "https://futures.kraken.com"
	DefaultWebSocketURL = "wss://futures.kraken.com/ws/v1"
)

// family is the Kraken service an endpoint belongs to. Each has its own host, signature, and
// response envelope.
type family int

const (
	familySpot family = iota
	familyFutures
	familyWebSocket
)

// endpoint is one read-only Kraken call the adapter makes.
type endpoint struct {
	family family
	method string
	path   string
	signed bool
}

func (e endpoint) String() string { return e.method + " " + e.path }

// Every endpoint the adapter and the recorder call. All are read-only: the Spot private calls are
// POST only because Kraken signs a form body, and the WebSocket entry is the one balances_snapshot
// message of the Futures balances feed, read and closed at once.
var (
	epAssets           = endpoint{familySpot, http.MethodGet, "/0/public/Assets", false}
	epAssetPairs       = endpoint{familySpot, http.MethodGet, "/0/public/AssetPairs", false}
	epSpotTicker       = endpoint{familySpot, http.MethodGet, "/0/public/Ticker", false}
	epAPIKeyInfo       = endpoint{familySpot, http.MethodPost, "/0/private/GetApiKeyInfo", true}
	epBalanceEx        = endpoint{familySpot, http.MethodPost, "/0/private/BalanceEx", true}
	epTradeBalance     = endpoint{familySpot, http.MethodPost, "/0/private/TradeBalance", true}
	epEarnStrategies   = endpoint{familySpot, http.MethodPost, "/0/private/Earn/Strategies", true}
	epEarnAllocations  = endpoint{familySpot, http.MethodPost, "/0/private/Earn/Allocations", true}
	epFuturesKey       = endpoint{familyFutures, http.MethodGet, "/api/auth/v1/api-keys/v3/check", true}
	epOpenPositions    = endpoint{familyFutures, http.MethodGet, "/derivatives/api/v3/openpositions", true}
	epInstruments      = endpoint{familyFutures, http.MethodGet, "/derivatives/api/v3/instruments", false}
	epFuturesTickers   = endpoint{familyFutures, http.MethodGet, "/derivatives/api/v3/tickers", false}
	epBalancesSnapshot = endpoint{familyWebSocket, "WS", "/ws/v1/balances_snapshot", true}
)

const (
	requestTimeout   = 15 * time.Second
	snapshotTimeout  = 20 * time.Second
	maxResponseBytes = 16 << 20
	// snapshotMessages bounds how many feed messages may precede the balances snapshot.
	snapshotMessages = 30
)

// nonces issues the strictly increasing nonce every Spot private request carries. Kraken rejects
// a nonce that is not greater than the key's previous one, so one sequence serves every key.
type nonces struct {
	mu   sync.Mutex
	last int64
}

func (n *nonces) next(now time.Time) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	value := now.UnixMicro()
	if value <= n.last {
		value = n.last + 1
	}
	n.last = value
	return strconv.FormatInt(value, 10)
}

// number is a Kraken amount as a plain decimal literal. Futures and the WebSocket feed send
// amounts as JSON numbers, sometimes in exponent form, and Spot sends strings; a well-formed value
// of either kind becomes a plain literal. null and an absent field stay empty, and anything else is
// kept as sent, so the mapper that reads the field fails naming it instead of guessing.
type number string

var numeral = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?$`)

func (n *number) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if text == "null" {
		*n = ""
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
	}
	if numeral.MatchString(text) {
		if rat, ok := new(big.Rat).SetString(text); ok {
			text = decimal.Format(rat)
		}
	}
	*n = number(text)
	return nil
}

// client issues Kraken requests for one credential set. Raw bodies stay inside this package.
type client struct {
	a     *Adapter
	creds adapters.Credentials
	// record receives every raw exchange while the fixture recorder runs; it is nil otherwise.
	record func(ep endpoint, resp response) error
	// nonce replaces the nonce sequence when set. Only the recorder sets it, to provoke an
	// invalid-nonce answer.
	nonce string
}

// response is one raw exchange. Only the recorder keeps the body beyond decoding.
type response struct {
	status int
	body   []byte
}

// call performs one REST request and decodes its result into out.
func (c *client) call(ctx context.Context, ep endpoint, params url.Values, out any) error {
	resp, err := c.exchange(ctx, ep, params)
	if err != nil {
		return err
	}
	return decode(ep, resp, out)
}

// exchange performs one REST request and returns the raw answer.
func (c *client) exchange(ctx context.Context, ep endpoint, params url.Values) (response, error) {
	if ep.family == familySpot {
		return c.spot(ctx, ep, params)
	}
	return c.futures(ctx, ep, params)
}

// spot sends one Spot request. A private request carries a fresh nonce in its form body and the
// API-Sign header: HMAC-SHA512 of the path and SHA-256(nonce + body), keyed by the decoded secret.
func (c *client) spot(ctx context.Context, ep endpoint, params url.Values) (response, error) {
	values := url.Values{}
	for key, value := range params {
		values[key] = value
	}
	target := c.a.spotHost + ep.path
	body := ""
	if ep.signed {
		nonce := c.nonce
		if nonce == "" {
			nonce = c.a.nonces.next(c.a.clock())
		}
		values.Set("nonce", nonce)
		body = values.Encode()
	} else if len(values) > 0 {
		target += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, ep.method, target, strings.NewReader(body))
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("Kraken %s: the request could not be built.", ep))
	}
	if ep.signed {
		signature, err := spotSignature(ep.path, values.Get("nonce"), body, c.creds.APISecret)
		if err != nil {
			return response{}, err
		}
		req.Header.Set("Api-Key", c.creds.APIKey)
		req.Header.Set("Api-Sign", signature)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.send(ctx, ep, req)
}

// futures sends one Futures request. A private request carries the Authent header: HMAC-SHA512
// of SHA-256(query + path without /derivatives), keyed by the decoded Futures secret.
func (c *client) futures(ctx context.Context, ep endpoint, params url.Values) (response, error) {
	query := params.Encode()
	target := c.a.futuresHost + ep.path
	if query != "" {
		target += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, ep.method, target, http.NoBody)
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("Kraken %s: the request could not be built.", ep))
	}
	if ep.signed {
		signature, err := futuresSignature(strings.TrimPrefix(ep.path, "/derivatives"), query, c.creds.FuturesSecret())
		if err != nil {
			return response{}, err
		}
		req.Header.Set("Apikey", c.creds.FuturesKey())
		req.Header.Set("Authent", signature)
	}
	return c.send(ctx, ep, req)
}

func (c *client) send(ctx context.Context, ep endpoint, req *http.Request) (response, error) {
	res, err := c.a.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("Kraken %s: Kraken could not be reached.", ep))
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("Kraken %s: the response could not be read.", ep))
	}
	resp := response{status: res.StatusCode, body: body}
	if c.record != nil {
		if err := c.record(ep, resp); err != nil {
			return response{}, err
		}
	}
	return resp, nil
}

// decode converts a REST answer into out, or into a classified error.
func decode(ep endpoint, resp response, out any) error {
	if ep.family == familySpot {
		return decodeSpot(ep, resp, out)
	}
	return decodeFutures(ep, resp, out)
}

// decodeSpot converts a Spot exchange into out, or into a classified error. Spot reports failures
// in its error list, often with HTTP 200, so both the status and the list decide.
func decodeSpot(ep endpoint, resp response, out any) error {
	var env struct {
		Errors *[]string       `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	parsed := json.Unmarshal(resp.body, &env) == nil
	if failedStatus(resp.status) {
		// The status decides the class; a documented error name only sharpens the detail.
		name := ""
		if env.Errors != nil && len(*env.Errors) > 0 {
			name = (*env.Errors)[0]
		}
		return classify(ep, resp.status, name)
	}
	if !parsed || env.Errors == nil {
		return adapters.Malformed("Kraken "+ep.String(), "the response envelope")
	}
	if len(*env.Errors) > 0 {
		return classify(ep, resp.status, (*env.Errors)[0])
	}
	if resp.status < 200 || resp.status >= 300 {
		return classify(ep, resp.status, "")
	}
	if len(env.Result) == 0 || string(env.Result) == "null" || json.Unmarshal(env.Result, out) != nil {
		return adapters.Malformed("Kraken "+ep.String(), "result")
	}
	return nil
}

// decodeFutures converts a Futures exchange into out, or into a classified error. Futures marks
// success with result = "success"; the key check alone answers with the key and no result field.
func decodeFutures(ep endpoint, resp response, out any) error {
	var env struct {
		Result string `json:"result"`
		Error  string `json:"error"`
	}
	parsed := json.Unmarshal(resp.body, &env) == nil
	if failedStatus(resp.status) {
		// The status decides the class; a documented error name only sharpens the detail.
		return classify(ep, resp.status, env.Error)
	}
	if !parsed {
		return adapters.Malformed("Kraken "+ep.String(), "the response envelope")
	}
	if resp.status < 200 || resp.status >= 300 || env.Error != "" || (env.Result != "success" && ep != epFuturesKey) {
		return classify(ep, resp.status, env.Error)
	}
	if json.Unmarshal(resp.body, out) != nil {
		return adapters.Malformed("Kraken "+ep.String(), "the response body")
	}
	return nil
}

// failedStatus reports the statuses that fail a request whatever its body says.
func failedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden ||
		status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

// errorNames classify the error names Kraken documents for both services; any other name is
// temporary. A detail carries only the matched name, never arbitrary Platform text or an echoed
// input.
var errorNames = []struct{ token, class, detail string }{
	{"Invalid key", adapters.ClassAuthentication, "Invalid key."},
	{"Invalid signature", adapters.ClassAuthentication, "Invalid signature."},
	{"authenticationError", adapters.ClassAuthentication, "Authentication failed."},
	{"api_key_not_found", adapters.ClassAuthentication, "API key not found."},
	{"request_signature_invalid", adapters.ClassAuthentication, "Invalid signature."},
	{"Permission denied", adapters.ClassAuthorization, "Read permission denied."},
	{"Rate limit", adapters.ClassRateLimited, "Rate limit exceeded."},
	{"Too many requests", adapters.ClassRateLimited, "Too many requests."},
	{"Throttled", adapters.ClassRateLimited, "Too many concurrent requests."},
	{"apiLimitExceeded", adapters.ClassRateLimited, "Rate limit exceeded."},
	{"Invalid nonce", adapters.ClassTemporary, "Invalid nonce."},
	{"Unavailable", adapters.ClassTemporary, "Service unavailable."},
}

// classify maps a failed Kraken answer to an adapter error class; the HTTP status decides over
// the error name for 401, 403, and 429.
func classify(ep endpoint, status int, name string) error {
	class, detail := adapters.ClassTemporary, "Platform request failed."
	for _, known := range errorNames {
		if strings.Contains(name, known.token) {
			class, detail = known.class, known.detail
			break
		}
	}
	switch status {
	case http.StatusUnauthorized:
		class = adapters.ClassAuthentication
	case http.StatusForbidden:
		class = adapters.ClassAuthorization
	case http.StatusTooManyRequests:
		class = adapters.ClassRateLimited
	}
	return adapters.NewError(class, fmt.Sprintf("Kraken %s: HTTP %d. %s", ep, status, detail))
}

// balancesSnapshot subscribes to the Futures balances feed, reads the first balances_snapshot,
// and closes the connection. The subscription is authenticated by signing Kraken's challenge.
func (c *client) balancesSnapshot(ctx context.Context) (snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()
	conn, res, err := websocket.Dial(ctx, c.a.wsURL, &websocket.DialOptions{HTTPClient: c.a.http})
	if err != nil {
		if res != nil && res.Body != nil {
			_ = res.Body.Close()
		}
		return snapshot{}, adapters.NewError(adapters.ClassTemporary, "Kraken "+epBalancesSnapshot.String()+": the balances feed could not be reached.")
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(maxResponseBytes)
	if err := writeMessage(ctx, conn, map[string]string{"event": "challenge", "api_key": c.creds.FuturesKey()}); err != nil {
		return snapshot{}, err
	}
	subscribed := false
	for range snapshotMessages {
		_, body, err := conn.Read(ctx)
		if err != nil {
			return snapshot{}, adapters.NewError(adapters.ClassTemporary, "Kraken "+epBalancesSnapshot.String()+": the balances feed ended before its snapshot.")
		}
		var env struct {
			Event   string `json:"event"`
			Message string `json:"message"`
			Feed    string `json:"feed"`
		}
		if json.Unmarshal(body, &env) != nil {
			return snapshot{}, adapters.Malformed("Kraken "+epBalancesSnapshot.String(), "the feed message")
		}
		switch {
		case env.Event == "error" || env.Event == "subscribed_failed":
			return snapshot{}, adapters.NewError(adapters.ClassAuthorization, "Kraken "+epBalancesSnapshot.String()+": Kraken refused the read-only balances subscription.")
		case env.Event == "challenge" && !subscribed:
			signature, err := challengeSignature(env.Message, c.creds.FuturesSecret())
			if err != nil {
				return snapshot{}, err
			}
			subscription := map[string]string{
				"event": "subscribe", "feed": "balances", "api_key": c.creds.FuturesKey(),
				"original_challenge": env.Message, "signed_challenge": signature,
			}
			if err := writeMessage(ctx, conn, subscription); err != nil {
				return snapshot{}, err
			}
			subscribed = true
		case env.Feed == "balances_snapshot":
			if c.record != nil {
				if err := c.record(epBalancesSnapshot, response{status: http.StatusOK, body: body}); err != nil {
					return snapshot{}, err
				}
			}
			var out snapshot
			if json.Unmarshal(body, &out) != nil {
				return snapshot{}, adapters.Malformed("Kraken "+epBalancesSnapshot.String(), "the balances snapshot")
			}
			return out, nil
		}
	}
	return snapshot{}, adapters.Malformed("Kraken "+epBalancesSnapshot.String(), "a balances snapshot within the first messages")
}

func writeMessage(ctx context.Context, conn *websocket.Conn, value map[string]string) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if conn.Write(ctx, websocket.MessageText, body) != nil {
		return adapters.NewError(adapters.ClassTemporary, "Kraken "+epBalancesSnapshot.String()+": the balances feed request failed.")
	}
	return nil
}

func spotSignature(path, nonce, body, secret string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", adapters.NewError(adapters.ClassAuthentication, "The Kraken Spot secret is not base64.")
	}
	hash := sha256.Sum256([]byte(nonce + body))
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write([]byte(path))
	_, _ = mac.Write(hash[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func futuresSignature(path, query, secret string) (string, error) {
	return futuresDigest(query+path, secret)
}

func challengeSignature(challenge, secret string) (string, error) {
	return futuresDigest(challenge, secret)
}

// futuresDigest is the Futures signature scheme: HMAC-SHA512 of SHA-256(message), keyed by the
// decoded Futures secret, in base64.
func futuresDigest(message, secret string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", adapters.NewError(adapters.ClassAuthentication, "The Kraken Futures secret is not base64.")
	}
	hash := sha256.Sum256([]byte(message))
	mac := hmac.New(sha512.New, key)
	_, _ = mac.Write(hash[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
