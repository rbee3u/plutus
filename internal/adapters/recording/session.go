// Package recording captures one explicit read-only request, never runtime collection.
package recording

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/adapters/fixture"
)

// Session blocks every request except one selected exchange, including retries and redirects.
type Session struct {
	platform    adapters.Platform
	request     fixture.Request
	mu          sync.Mutex
	used        bool
	blocked     error
	files       []fixture.File
	keepSymbols map[string]bool
}

// Options affect local evidence selection only; they never add or change a network request.
type Options struct {
	KeepSymbols []string
}

func New(platform adapters.Platform, request fixture.Request, options ...Options) *Session {
	s := &Session{platform: platform, request: request, keepSymbols: map[string]bool{}}
	for _, option := range options {
		for _, symbol := range option.KeepSymbols {
			s.keepSymbols[symbol] = true
		}
	}
	return s
}

func Service(platform adapters.Platform, path string) string {
	if platform == adapters.PlatformBinance {
		for prefix, name := range map[string]string{"/papi/": "portfolio", "/fapi/": "um", "/dapi/": "cm"} {
			if strings.HasPrefix(path, prefix) {
				return name
			}
		}
		return "spot"
	}
	if platform == adapters.PlatformKraken {
		if strings.HasPrefix(path, "/0/") {
			return "spot"
		}
		if strings.HasPrefix(path, "/api/auth/") {
			return "futures-auth"
		}
		return "futures"
	}
	return "v5"
}

func business(values url.Values) url.Values {
	out := url.Values{}
	for key, value := range values {
		if fixture.AuthenticationParameter(key) {
			continue
		}
		out[key] = slices.Clone(value)
	}
	return out
}

// HTTPRequest reads the replayable business identity without consuming the signed request body.
func HTTPRequest(platform adapters.Platform, req *http.Request) (fixture.Request, error) {
	r := fixture.Request{Service: Service(platform, req.URL.Path), Method: req.Method, Path: req.URL.Path, Query: business(req.URL.Query())}
	if req.Body != nil && strings.HasPrefix(req.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if req.GetBody == nil {
			return r, errors.New("recording needs a replayable form body")
		}
		body, err := req.GetBody()
		if err != nil {
			return r, err
		}
		raw, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			return r, err
		}
		values, err := url.ParseQuery(string(raw))
		if err != nil {
			return r, err
		}
		r.Form = business(values)
	}
	return r, nil
}

type transport struct {
	session *Session
	base    http.RoundTripper
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	identity, err := HTTPRequest(t.session.platform, req)
	if err != nil {
		return nil, err
	}
	if err := t.session.allow(identity); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode == http.StatusSwitchingProtocols {
		return resp, err
	}
	const maximum = 16 << 20
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maximum+1))
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if len(raw) > maximum {
		return nil, errors.New("response exceeds recording limit; incomplete response discarded")
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	t.session.Add(identity, resp.StatusCode, raw)
	return resp, nil
}

func (s *Session) Client(base *http.Client) *http.Client {
	client := *base
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	client.Transport = transport{s, rt}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client
}

func (s *Session) allow(request fixture.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	match := request.Key() == s.request.Key()
	if s.request.Method == "WS" {
		match = request.Method == "GET" && request.Path == "/ws/v1" && len(request.Query) == 0
	}
	if !match || s.used {
		s.blocked = errors.New("unselected or repeated request blocked before networking")
		return s.blocked
	}
	s.used = true
	return nil
}

func (s *Session) Add(request fixture.Request, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Key() != s.request.Key() || len(s.files) != 0 {
		s.blocked = errors.New("unexpected or repeated recording response")
		return
	}
	now := time.Now().UTC()
	if request.Method == "WS" {
		status = 0
	}
	s.files = append(s.files, fixture.File{Source: "recorded", RecordedAt: &now, Request: request, Status: status, Body: bytes.Clone(body)})
}

func (s *Session) Run(ctx context.Context, creds adapters.Credentials, call func(context.Context) error) (*fixture.File, error) {
	if err := s.request.Validate(); err != nil {
		return nil, err
	}
	failure := call(ctx)
	if s.blocked != nil {
		failure = s.blocked
	}
	return s.finish(creds, failure)
}

var ipPattern = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

// finish returns only sanitized evidence and sanitized errors. Business numbers remain intact.
func (s *Session) finish(creds adapters.Credentials, failure error) (*fixture.File, error) {
	secrets := []string{creds.APIKey, creds.APISecret, creds.Passphrase, creds.FuturesKey(), creds.FuturesSecret()}
	values := make([]any, len(s.files))
	identifiers := map[string]bool{}
	for i := range s.files {
		f := &s.files[i]
		d := json.NewDecoder(bytes.NewReader(f.Body))
		d.UseNumber()
		var extra any
		if d.Decode(&values[i]) != nil || !errors.Is(d.Decode(&extra), io.EOF) {
			values[i] = string(f.Body)
			f.BodyFormat = "text"
		}
		visit(values[i], func(key string, value any) {
			if sensitiveResponse(f.Request.Path, key) {
				switch value := value.(type) {
				case string:
					if len(value) >= 8 {
						identifiers[value] = true
					}
				case json.Number:
					if len(value) >= 8 {
						identifiers[string(value)] = true
					}
				}
			}
		})
	}
	var replace []string
	for _, secret := range secrets {
		if secret != "" {
			replace = append(replace, secret)
		}
	}
	for identifier := range identifiers {
		replace = append(replace, identifier)
	}
	slices.SortFunc(replace, func(a, b string) int { return len(b) - len(a) })
	cleanText := func(value string) string {
		for _, secret := range replace {
			value = strings.ReplaceAll(value, secret, "REDACTED")
		}
		return ipPattern.ReplaceAllString(value, "203.0.113.1")
	}
	s.trimMarkets(values)
	s.trimZeroRows(values)
	mask := identityMask()
	for i := range s.files {
		f := &s.files[i]
		fields := map[string]bool{}
		values[i] = clean(values[i], cleanText, fields, func(key string) bool { return sensitiveResponse(f.Request.Path, key) }, mask)
		raw, err := json.Marshal(values[i])
		if err != nil {
			return nil, errors.New("redaction failed; no response may be written")
		}
		requestRaw, err := json.Marshal(f.Request)
		if err != nil {
			return nil, errors.New("request redaction failed; no response may be written")
		}
		for _, secret := range secrets {
			if secret != "" && (bytes.Contains(raw, []byte(secret)) || bytes.Contains(requestRaw, []byte(secret))) {
				return nil, errors.New("credential survived redaction; no response may be written")
			}
		}
		f.Body = raw
		var names []string
		for key := range fields {
			names = append(names, key)
		}
		slices.Sort(names)
		if len(names) > 0 {
			f.Notes = strings.TrimSpace(f.Notes + " Redacted: " + strings.Join(names, ", ") + ".")
		}
		if err := f.Validate(); err != nil {
			return nil, err
		}
	}
	var file *fixture.File
	if len(s.files) == 1 {
		file = &s.files[0]
	} else if failure == nil {
		failure = errors.New("request produced no response")
	}
	if failure != nil {
		return file, errors.New(cleanText(failure.Error()))
	}
	return file, nil
}

// Empty inspects supported response containers without treating zero business values as empty.
func Empty(f *fixture.File) bool {
	var body any
	if json.Unmarshal(f.Body, &body) != nil {
		return false
	}
	var empty func(any) bool
	empty = func(value any) bool {
		switch v := value.(type) {
		case nil:
			return true
		case string:
			return v == ""
		case []any:
			return len(v) == 0
		case map[string]any:
			for _, key := range []string{"rows", "list", "data", "result", "balances", "positions", "openPositions", "allocations", "items"} {
				if child, ok := v[key]; ok {
					return empty(child)
				}
			}
			return len(v) == 0
		}
		return false
	}
	return empty(body)
}
