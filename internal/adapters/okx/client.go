package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
)

// DefaultHost is the OKX REST host. Accounts registered in the United States or Australia must
// use us.okx.com and European accounts eea.okx.com; those regions need Options.Host.
const DefaultHost = "https://openapi.okx.com"

// endpoint is one read-only OKX call the adapter makes. OKX distinguishes several endpoints by
// query alone (public/instruments per instType), so the query is part of the endpoint identity.
type endpoint struct {
	method string
	path   string
	query  string
	signed bool
}

// request is the path a request is sent to and signed over, query included.
func (e endpoint) request() string {
	if e.query == "" {
		return e.path
	}
	return e.path + "?" + e.query
}

func (e endpoint) String() string { return e.method + " " + e.request() }

// Every endpoint the adapter and the recorder call. All are read-only. epDCD is the first page of
// the dual investment order history; dcdPage builds the next ones.
var (
	epTime      = endpoint{http.MethodGet, "/api/v5/public/time", "", false}
	epConfig    = endpoint{http.MethodGet, "/api/v5/account/config", "", true}
	epBalance   = endpoint{http.MethodGet, "/api/v5/account/balance", "", true}
	epPositions = endpoint{http.MethodGet, "/api/v5/account/positions", "", true}
	epFunding   = endpoint{http.MethodGet, "/api/v5/asset/balances", "", true}
	epSavings   = endpoint{http.MethodGet, "/api/v5/finance/savings/balance", "", true}
	epStaking   = endpoint{http.MethodGet, "/api/v5/finance/staking-defi/orders-active", "", true}
	epDCD       = endpoint{http.MethodGet, "/api/v5/finance/sfp/dcd/order-history", "limit=" + strconv.Itoa(dcdPageSize), true}
	epValuation = endpoint{http.MethodGet, "/api/v5/asset/asset-valuation", "ccy=USDT", true}
	epSwaps     = endpoint{http.MethodGet, "/api/v5/public/instruments", "instType=SWAP", false}
	epFutures   = endpoint{http.MethodGet, "/api/v5/public/instruments", "instType=FUTURES", false}
	epTickers   = endpoint{http.MethodGet, "/api/v5/market/tickers", "instType=SPOT", false}
)

const (
	requestTimeout   = 10 * time.Second
	maxResponseBytes = 16 << 20
	codeOK           = "0"
	// timestampLayout is the ISO 8601 millisecond UTC format OKX signs and rejects beyond 30s.
	timestampLayout = "2006-01-02T15:04:05.000Z"
	codeExpiredTS   = "50102"
	codeInvalidTS   = "50112"
)

// envelope is every OKX answer: a code, a message, and the payload rows. A failed request may
// still arrive with HTTP 200, so the code is what decides success.
type envelope struct {
	Code answerCode      `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// answerCode is OKX's answer code. OKX documents it as a string, but the dual investment order
// history sends a number.
type answerCode string

func (c *answerCode) UnmarshalJSON(raw []byte) error {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		*c = answerCode(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	*c = answerCode(number.String())
	return nil
}

// clock keeps the offset between the local clock and OKX server time so signed requests stay
// inside the 30-second window on a machine whose clock drifts.
type clock struct {
	mu     sync.Mutex
	now    func() time.Time
	offset time.Duration
	synced bool
}

func (c *clock) timestamp() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Add(c.offset).UTC().Format(timestampLayout)
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

// client issues OKX requests for one credential set. Raw bodies stay inside this package.
type client struct {
	http  *http.Client
	host  string
	clock *clock
	creds adapters.Credentials
}

// response is one raw HTTP exchange. Only the recorder keeps the body beyond decoding.
type response struct {
	status int
	body   []byte
}

// call performs one request and decodes its data rows into out.
func (c *client) call(ctx context.Context, ep endpoint, out any) error {
	resp, err := c.fetch(ctx, ep)
	if err != nil {
		return err
	}
	return decode(ep, resp, out)
}

// fetch performs one request with a synchronized clock and returns the raw exchange. An expired
// timestamp resynchronizes the clock and retries once.
func (c *client) fetch(ctx context.Context, ep endpoint) (response, error) {
	if ep.signed && c.clock.needsSync() {
		if err := c.syncClock(ctx); err != nil {
			return response{}, err
		}
	}
	resp, err := c.send(ctx, ep, c.creds)
	if err != nil {
		return response{}, err
	}
	if ep.signed && clockRejected(resp) {
		if err := c.syncClock(ctx); err != nil {
			return response{}, err
		}
		return c.send(ctx, ep, c.creds)
	}
	return resp, nil
}

func (c *client) syncClock(ctx context.Context) error {
	sent := c.clock.now()
	resp, err := c.send(ctx, epTime, adapters.Credentials{})
	if err != nil {
		return err
	}
	var rows []struct {
		TS string `json:"ts"`
	}
	if err := decode(epTime, resp, &rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		return adapters.NewError(adapters.ClassNormalization, "OKX returned no server time.")
	}
	millis, err := strconv.ParseInt(strings.TrimSpace(rows[0].TS), 10, 64)
	if err != nil || millis <= 0 {
		return adapters.NewError(adapters.ClassNormalization, "OKX returned no server time.")
	}
	c.clock.set(millis, sent, c.clock.now())
	return nil
}

// send builds, signs and performs one request using the client clock.
func (c *client) send(ctx context.Context, ep endpoint, creds adapters.Credentials) (response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, ep.method, c.host+ep.request(), http.NoBody)
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the request could not be built.", ep))
	}
	req.Header.Set("Content-Type", "application/json")
	if ep.signed {
		timestamp := c.clock.timestamp()
		mac := hmac.New(sha256.New, []byte(creds.APISecret))
		_, _ = mac.Write([]byte(timestamp + ep.method + ep.request()))
		// OKX documents these as OK-ACCESS-*; header names are case-insensitive.
		req.Header.Set("Ok-Access-Key", creds.APIKey)
		req.Header.Set("Ok-Access-Sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		req.Header.Set("Ok-Access-Timestamp", timestamp)
		req.Header.Set("Ok-Access-Passphrase", creds.Passphrase)
	}
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: OKX could not be reached.", ep))
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: the OKX response was cut off.", ep))
	}
	return response{status: res.StatusCode, body: body}, nil
}

// decode converts a raw exchange into out, or into a classified error. Both a non-200 status and
// a non-zero code are failures, and OKX uses both.
func decode(ep endpoint, resp response, out any) error {
	env, ok := parse(resp)
	if !ok {
		if resp.status != http.StatusOK {
			return adapters.NewError(classOf(resp.status, ""), fmt.Sprintf("%s: HTTP %d", ep, resp.status))
		}
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the OKX response has an unexpected shape.", ep))
	}
	if env.Code != codeOK || resp.status != http.StatusOK {
		return classify(ep, resp.status, env)
	}
	if len(env.Data) == 0 {
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the OKX response carries no data.", ep))
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the OKX response has an unexpected shape.", ep))
	}
	return nil
}

func parse(resp response) (envelope, bool) {
	var env envelope
	if json.Unmarshal(resp.body, &env) != nil {
		return envelope{}, false
	}
	return env, true
}

// clockRejected reports whether OKX refused a signed request because of its timestamp.
func clockRejected(resp response) bool {
	env, ok := parse(resp)
	if !ok {
		return false
	}
	code := mainCode(string(env.Code))
	return code == codeExpiredTS || code == codeInvalidTS
}

// classify maps an OKX answer to an adapter error class. The detail carries the endpoint and
// OKX's own code and message, never the body.
func classify(ep endpoint, status int, env envelope) error {
	code := mainCode(string(env.Code))
	detail := fmt.Sprintf("%s: HTTP %d", ep, status)
	if code != "" && code != codeOK {
		detail = fmt.Sprintf("%s: OKX %s %s", ep, env.Code, truncate(env.Msg, 200))
	}
	return adapters.NewError(classOf(status, code), strings.TrimSpace(detail))
}

// mainCode drops the sub-code of a compound answer such as "51008_1000".
func mainCode(code string) string {
	main, _, _ := strings.Cut(strings.TrimSpace(code), "_")
	return main
}

func classOf(status int, code string) string {
	switch code {
	case "50101", "50103", "50104", "50105", "50106", "50107", "50110", "50111", "50113", "50114", "50119", "50121", "50035", "50100":
		// Wrong key, secret, passphrase, environment, or an IP the key does not allow.
		return adapters.ClassAuthentication
	case "50030", "50120", "59500":
		// The key reaches OKX but is not allowed to use this endpoint.
		return adapters.ClassAuthorization
	case "50011", "50013", "50040":
		return adapters.ClassRateLimited
	case codeExpiredTS, codeInvalidTS, "50001", "50004", "50026", "50046", "51767":
		// 50004 means OKX does not know whether the request succeeded; a read is safe to retry.
		return adapters.ClassTemporary
	}
	switch status {
	case http.StatusUnauthorized:
		return adapters.ClassAuthentication
	case http.StatusTooManyRequests:
		return adapters.ClassRateLimited
	default:
		return adapters.ClassTemporary
	}
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
