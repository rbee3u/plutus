package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// DefaultHost is the global Bybit REST host. Regional services such as Bybit EU, KZ, and GE use
// hosts of their own and are not supported.
const DefaultHost = "https://api.bybit.com"

// endpoint is one read-only Bybit V5 call the adapter makes. Bybit tells several reads apart by
// query alone (wallet-balance?accountType=UNIFIED), so the fixed query is part of the endpoint;
// the caller adds per-call filters and paging cursors.
type endpoint struct {
	path   string
	query  string
	signed bool
}

func (e endpoint) String() string {
	if e.query == "" {
		return http.MethodGet + " " + e.path
	}
	return http.MethodGet + " " + e.path + "?" + e.query
}

// request is the path and query one call is sent to and signed over. The query is sorted by
// key, so the same call always produces the same request and fixture name.
func (e endpoint) request(params url.Values) string {
	query, _ := url.ParseQuery(e.query)
	for key, values := range params {
		query[key] = append([]string(nil), values...)
	}
	if len(query) == 0 {
		return e.path
	}
	return e.path + "?" + query.Encode()
}

// Every endpoint the adapter and the recorder call. All are read-only GETs.
var (
	epTime          = endpoint{"/v5/market/time", "", false}
	epAccountInfo   = endpoint{"/v5/account/info", "", true}
	epAPIKey        = endpoint{"/v5/user/query-api", "", true}
	epWallet        = endpoint{"/v5/account/wallet-balance", "accountType=UNIFIED", true}
	epPositions     = endpoint{"/v5/position/list", "limit=200", true}
	epFunding       = endpoint{"/v5/asset/transfer/query-account-coins-balance", "accountType=FUND", true}
	epEarnProduct   = endpoint{"/v5/earn/product", "category=FlexibleSaving", false}
	epFlexible      = endpoint{"/v5/earn/position", "category=FlexibleSaving", true}
	epFixedTerm     = endpoint{"/v5/earn/fixed-term/position", "category=FixedTermSaving", true}
	epDualAssets    = endpoint{"/v5/earn/advance/position", "category=DualAssets&limit=20", true}
	epAssetOverview = endpoint{"/v5/asset/asset-overview", "valuationCurrency=USD", true}
	epInstruments   = endpoint{"/v5/market/instruments-info", "limit=1000", false}
	epTickers       = endpoint{"/v5/market/tickers", "category=spot", false}
)

const (
	requestTimeout   = 15 * time.Second
	maxResponseBytes = 16 << 20
	// recvWindowMillis is how long after its timestamp Bybit still accepts a signed request.
	recvWindowMillis = "5000"
	codeOK           = 0
	// Bybit refuses a signed request whose timestamp is outside the receive window with 10002,
	// and with -1 when it arrives after the window has passed.
	codeTimeWindow = 10002
	codeExpired    = -1
)

// envelope is every Bybit V5 answer: a numeric code, a message, the result, and the server time
// in milliseconds. A failed request may still arrive with HTTP 200, so the code is what decides
// success.
type envelope struct {
	Code   *int            `json:"retCode"`
	Result json.RawMessage `json:"result"`
	Time   int64           `json:"time"`
}

// clock keeps the offset between the local clock and Bybit server time so signed requests stay
// inside the receive window on a machine whose clock drifts.
type clock struct {
	mu     sync.Mutex
	now    func() time.Time
	offset time.Duration
	synced bool
}

// corrected is the current time on Bybit's clock, as last measured.
func (c *clock) corrected() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Add(c.offset)
}

func (c *clock) timestamp() string {
	return strconv.FormatInt(c.corrected().UnixMilli(), 10)
}

func (c *clock) needsSync() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.synced
}

func (c *clock) set(serverMillis int64, sent, received time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	midpoint := sent.Add(received.Sub(sent) / 2)
	c.offset = time.UnixMilli(serverMillis).Sub(midpoint)
	c.synced = true
}

// client issues Bybit requests for one credential set. Raw bodies stay inside this package; only
// the recorder sets capture to keep each exchange.
type client struct {
	http    *http.Client
	host    string
	clock   *clock
	creds   adapters.Credentials
	capture func(request string, resp response)
}

// response is one raw HTTP exchange. Only the recorder keeps the body beyond decoding.
type response struct {
	status int
	body   []byte
}

// call performs one request and decodes its result into out. A signed request first synchronizes
// the clock; an answer that refuses the timestamp resynchronizes it and retries once.
func (c *client) call(ctx context.Context, ep endpoint, params url.Values, out any) error {
	if ep.signed && c.clock.needsSync() {
		if err := c.syncClock(ctx); err != nil {
			return err
		}
	}
	request := ep.request(params)
	resp, err := c.send(ctx, ep, request, c.creds)
	if err != nil {
		return err
	}
	if ep.signed && clockRejected(resp) {
		if err := c.syncClock(ctx); err != nil {
			return err
		}
		if resp, err = c.send(ctx, ep, request, c.creds); err != nil {
			return err
		}
	}
	return decode(ep, resp, out)
}

// syncClock measures the offset to Bybit server time, taking the midpoint of the round trip as
// the moment the server read its clock.
func (c *client) syncClock(ctx context.Context) error {
	sent := c.clock.now()
	resp, err := c.send(ctx, epTime, epTime.request(nil), c.creds)
	if err != nil {
		return err
	}
	var result struct {
		TimeSecond string `json:"timeSecond"`
	}
	if err := decode(epTime, resp, &result); err != nil {
		return err
	}
	var env envelope
	_ = json.Unmarshal(resp.body, &env)
	millis := env.Time
	if millis <= 0 {
		seconds, err := strconv.ParseInt(strings.TrimSpace(result.TimeSecond), 10, 64)
		if err != nil || seconds <= 0 {
			return adapters.NewError(adapters.ClassNormalization, "Bybit returned no server time.")
		}
		millis = seconds * 1000
	}
	c.clock.set(millis, sent, c.clock.now())
	return nil
}

// send builds, signs and performs one request using the client clock.
func (c *client) send(ctx context.Context, ep endpoint, request string, creds adapters.Credentials) (response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.host+request, http.NoBody)
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the request could not be built.", ep))
	}
	if ep.signed {
		timestamp := c.clock.timestamp()
		_, query, _ := strings.Cut(request, "?")
		mac := hmac.New(sha256.New, []byte(creds.APISecret))
		_, _ = mac.Write([]byte(timestamp + creds.APIKey + recvWindowMillis + query))
		// Bybit documents these as X-BAPI-*; header names are case-insensitive.
		req.Header.Set("X-Bapi-Api-Key", creds.APIKey)
		req.Header.Set("X-Bapi-Timestamp", timestamp)
		req.Header.Set("X-Bapi-Recv-Window", recvWindowMillis)
		req.Header.Set("X-Bapi-Sign", hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: Bybit could not be reached.", ep))
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: the Bybit response was cut off.", ep))
	}
	resp := response{status: res.StatusCode, body: body}
	if c.capture != nil {
		c.capture(request, resp)
	}
	return resp, nil
}

// decode converts a raw exchange into out, or into a classified error. Both a non-2xx status and
// a non-zero code are failures, and Bybit uses both.
func decode(ep endpoint, resp response, out any) error {
	var env envelope
	if json.Unmarshal(resp.body, &env) != nil || env.Code == nil {
		return adapters.NewError(statusClass(resp.status, adapters.ClassNormalization),
			fmt.Sprintf("%s: HTTP %d without a Bybit answer.", ep, resp.status))
	}
	if *env.Code != codeOK || resp.status < 200 || resp.status > 299 {
		// Bybit messages may echo request parameters; the detail carries only the code.
		return adapters.NewError(classOf(resp.status, *env.Code), fmt.Sprintf("%s: HTTP %d, Bybit code %d.", ep, resp.status, *env.Code))
	}
	if len(env.Result) == 0 || string(env.Result) == "null" || json.Unmarshal(env.Result, out) != nil {
		return adapters.Malformed(ep.String(), "the result")
	}
	return nil
}

// clockRejected reports whether Bybit refused a signed request because of its timestamp.
func clockRejected(resp response) bool {
	var env envelope
	if json.Unmarshal(resp.body, &env) != nil || env.Code == nil {
		return false
	}
	return *env.Code == codeTimeWindow || *env.Code == codeExpired
}

// classOf maps a Bybit answer to an adapter error class.
func classOf(status, code int) string {
	switch code {
	case 10003, 10004, 10007, 33004, -2015:
		// Unknown key, wrong signature, failed authentication, or an expired key.
		return adapters.ClassAuthentication
	case 10005, 10010:
		// The key lacks the permission, or the request comes from an IP the key does not allow.
		return adapters.ClassAuthorization
	case 10006, 10018, 429:
		return adapters.ClassRateLimited
	case 10009, 10028, 100028, 180004, 180024:
		// A region Bybit does not serve, endpoints for another account structure, regional
		// sites, or sub-accounts: retrying cannot help and the key itself is fine.
		return adapters.ClassUnsupported
	}
	return statusClass(status, adapters.ClassTemporary)
}

// statusClass classifies by HTTP status alone, falling back to fallback.
func statusClass(status int, fallback string) string {
	switch {
	case status == http.StatusUnauthorized:
		return adapters.ClassAuthentication
	case status == http.StatusTooManyRequests:
		return adapters.ClassRateLimited
	case status == http.StatusForbidden, status >= 500:
		// Bybit documents 403 for an IP over its request budget, which is then refused for at
		// least ten minutes, and for a U.S. IP.
		return adapters.ClassTemporary
	default:
		return fallback
	}
}
