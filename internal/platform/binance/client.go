package binance

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

// Hosts are the Binance API base URLs. Paths never collide across hosts (/api, /sapi, /papi,
// /fapi, /dapi), so tests point every host at one httptest server.
type Hosts struct {
	API  string
	PAPI string
	FAPI string
	DAPI string
}

// DefaultHosts are the production Binance endpoints.
var DefaultHosts = Hosts{
	API:  "https://api.binance.com",
	PAPI: "https://papi.binance.com",
	FAPI: "https://fapi.binance.com",
	DAPI: "https://dapi.binance.com",
}

type host int

const (
	hostAPI host = iota
	hostPAPI
	hostFAPI
	hostDAPI
)

// endpoint is one read-only Binance call the adapter makes.
type endpoint struct {
	method string
	host   host
	path   string
	signed bool
}

func (e endpoint) String() string { return e.method + " " + e.path }

// Every endpoint the adapter and the fixture recorder call. All are read-only: market data,
// account, balance, position, and Earn position queries a key with only "Enable Reading" may call.
var (
	epServerTime   = endpoint{http.MethodGet, hostAPI, "/api/v3/time", false}
	epTickerPrice  = endpoint{http.MethodGet, hostAPI, "/api/v3/ticker/price", false}
	epSpotAccount  = endpoint{http.MethodGet, hostAPI, "/api/v3/account", true}
	epAccountInfo  = endpoint{http.MethodGet, hostAPI, "/sapi/v1/account/info", true}
	epUserAsset    = endpoint{http.MethodPost, hostAPI, "/sapi/v3/asset/getUserAsset", true}
	epFundingAsset = endpoint{http.MethodPost, hostAPI, "/sapi/v1/asset/get-funding-asset", true}
	epWalletValue  = endpoint{http.MethodGet, hostAPI, "/sapi/v1/asset/wallet/balance", true}
	epFlexibleEarn = endpoint{http.MethodGet, hostAPI, "/sapi/v1/simple-earn/flexible/position", true}
	epLockedEarn   = endpoint{http.MethodGet, hostAPI, "/sapi/v1/simple-earn/locked/position", true}
	epDualInvest   = endpoint{http.MethodGet, hostAPI, "/sapi/v1/dci/product/positions", true}

	// Portfolio Margin.
	epPMAccount        = endpoint{http.MethodGet, hostPAPI, "/papi/v1/account", true}
	epPMBalance        = endpoint{http.MethodGet, hostPAPI, "/papi/v1/balance", true}
	epPMUMPositionRisk = endpoint{http.MethodGet, hostPAPI, "/papi/v1/um/positionRisk", true}
	epPMCMPositionRisk = endpoint{http.MethodGet, hostPAPI, "/papi/v1/cm/positionRisk", true}

	// Classic Trading.
	epCrossMargin    = endpoint{http.MethodGet, hostAPI, "/sapi/v1/margin/account", true}
	epIsolatedMargin = endpoint{http.MethodGet, hostAPI, "/sapi/v1/margin/isolated/account", true}
	epUMAccount      = endpoint{http.MethodGet, hostFAPI, "/fapi/v3/account", true}
	epUMPositionRisk = endpoint{http.MethodGet, hostFAPI, "/fapi/v3/positionRisk", true}
	epUMMultiAssets  = endpoint{http.MethodGet, hostFAPI, "/fapi/v1/multiAssetsMargin", true}
	epUMSymbolConfig = endpoint{http.MethodGet, hostFAPI, "/fapi/v1/symbolConfig", true}
	epCMAccount      = endpoint{http.MethodGet, hostDAPI, "/dapi/v1/account", true}
	epCMPositionRisk = endpoint{http.MethodGet, hostDAPI, "/dapi/v1/positionRisk", true}

	// Instruments, for both Account Structures.
	epUMExchangeInfo = endpoint{http.MethodGet, hostFAPI, "/fapi/v1/exchangeInfo", false}
	epCMExchangeInfo = endpoint{http.MethodGet, hostDAPI, "/dapi/v1/exchangeInfo", false}
)

const (
	recvWindowMillis    = "10000"
	requestTimeout      = 10 * time.Second
	maxResponseBytes    = 16 << 20
	errTimestampOutside = -1021
)

// apiError is a Binance error body: {"code": -2015, "msg": "..."}.
type apiError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// clock keeps the offset between the local clock and Binance server time so signed requests
// stay inside recvWindow on a machine whose clock drifts.
type clock struct {
	mu     sync.Mutex
	now    func() time.Time
	offset time.Duration
	synced bool
}

func (c *clock) timestamp() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strconv.FormatInt(c.now().Add(c.offset).UnixMilli(), 10)
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

// client issues Binance requests for one credential set. Raw bodies stay inside this package.
type client struct {
	http  *http.Client
	hosts Hosts
	clock *clock
	creds adapters.Credentials
}

// response is one raw HTTP exchange. Only the recorder keeps the body beyond decoding.
type response struct {
	status int
	body   []byte
}

func (c *client) baseURL(h host) string {
	switch h {
	case hostPAPI:
		return c.hosts.PAPI
	case hostFAPI:
		return c.hosts.FAPI
	case hostDAPI:
		return c.hosts.DAPI
	case hostAPI:
		return c.hosts.API
	default:
		return c.hosts.API
	}
}

// call performs one request and decodes a 200 body into out. A -1021 answer resynchronizes the
// clock and retries once.
func (c *client) call(ctx context.Context, ep endpoint, params url.Values, out any) error {
	resp, err := c.fetch(ctx, ep, params)
	if err != nil {
		return err
	}
	return decode(ep, resp, out)
}

// fetch performs one request with a synchronized clock and returns the raw exchange, classifying
// transport failures. Non-200 answers are returned, not converted to errors.
func (c *client) fetch(ctx context.Context, ep endpoint, params url.Values) (response, error) {
	if ep.signed && c.clock.needsSync() {
		if err := c.syncClock(ctx); err != nil {
			return response{}, err
		}
	}
	resp, err := c.send(ctx, ep, params, c.creds.APIKey, c.creds.APISecret)
	if err != nil {
		return response{}, err
	}
	if ep.signed && errorCode(resp) == errTimestampOutside {
		if err := c.syncClock(ctx); err != nil {
			return response{}, err
		}
		return c.send(ctx, ep, params, c.creds.APIKey, c.creds.APISecret)
	}
	return resp, nil
}

func (c *client) syncClock(ctx context.Context) error {
	sent := c.clock.now()
	resp, err := c.send(ctx, epServerTime, nil, "", "")
	if err != nil {
		return err
	}
	var body struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := decode(epServerTime, resp, &body); err != nil {
		return err
	}
	if body.ServerTime <= 0 {
		return adapters.NewError(adapters.ClassNormalization, "Binance returned no server time.")
	}
	c.clock.set(body.ServerTime, sent, c.clock.now())
	return nil
}

// send builds, signs and performs one request using the client clock.
func (c *client) send(ctx context.Context, ep endpoint, params url.Values, key, secret string) (response, error) {
	query := url.Values{}
	for k, v := range params {
		query[k] = append([]string(nil), v...)
	}
	encoded := query.Encode()
	if ep.signed {
		timestamp := c.clock.timestamp()
		query.Set("timestamp", timestamp)
		query.Set("recvWindow", recvWindowMillis)
		encoded = query.Encode()
		signature, err := sign(secret, encoded)
		if err != nil {
			return response{}, err
		}
		encoded += "&signature=" + signature
	}
	target := c.baseURL(ep.host) + ep.path
	if encoded != "" {
		target += "?" + encoded
	}
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, ep.method, target, http.NoBody)
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the request could not be built.", ep))
	}
	if ep.signed {
		req.Header.Set("X-Mbx-Apikey", key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: Binance could not be reached.", ep))
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return response{}, adapters.NewError(adapters.ClassTemporary, fmt.Sprintf("%s: the Binance response was cut off.", ep))
	}
	return response{status: res.StatusCode, body: body}, nil
}

// sign signs the exact encoded query that will be sent. Binance expects Ed25519 signatures in
// Base64, percent-encoded as a query parameter; the existing HMAC representation is hexadecimal.
func sign(secret, payload string) (string, error) {
	key, ed, err := adapters.BinancePrivateKey(secret)
	if err != nil {
		return "", err
	}
	if ed {
		return url.QueryEscape(base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(payload)))), nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// decode converts a raw exchange into out, or into a classified error.
func decode(ep endpoint, resp response, out any) error {
	if resp.status != http.StatusOK {
		return classify(ep, resp)
	}
	if err := json.Unmarshal(resp.body, out); err != nil {
		return adapters.NewError(adapters.ClassNormalization, fmt.Sprintf("%s: the Binance response has an unexpected shape.", ep))
	}
	return nil
}

func errorCode(resp response) int {
	if resp.status == http.StatusOK {
		return 0
	}
	var body apiError
	if json.Unmarshal(resp.body, &body) != nil {
		return 0
	}
	return body.Code
}

// classify maps a non-200 Binance answer to an adapter error class. The detail carries the
// endpoint and Binance's own error code and message, never the body.
func classify(ep endpoint, resp response) error {
	var body apiError
	_ = json.Unmarshal(resp.body, &body)
	detail := fmt.Sprintf("%s: HTTP %d", ep, resp.status)
	if body.Code != 0 || body.Msg != "" {
		detail = fmt.Sprintf("%s: Binance %d %s", ep, body.Code, truncate(body.Msg, 200))
	}
	return adapters.NewError(classOf(resp.status, body.Code), detail)
}

func classOf(status, code int) string {
	switch code {
	// The key, its IP whitelist, or its permissions refused; a malformed key; a key Binance does
	// not know, which /sapi/v1/account/info answers for a deleted key; a wrong signature.
	case -2015, -2014, -2008, -1022:
		return adapters.ClassAuthentication
	case -1003:
		return adapters.ClassRateLimited
	case -1001, errTimestampOutside:
		return adapters.ClassTemporary
	}
	switch status {
	case http.StatusUnauthorized:
		return adapters.ClassAuthentication
	case http.StatusTooManyRequests, http.StatusTeapot, http.StatusForbidden:
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
