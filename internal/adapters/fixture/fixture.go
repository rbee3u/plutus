// Package fixture defines the shared envelope for recorded evidence and synthetic test responses.
package fixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Request identifies a service and the business parameters before signing.
// Parameters are strings on disk for one value and arrays for multiple values.
type Request struct {
	Service string     `json:"service"`
	Method  string     `json:"method"`
	Path    string     `json:"path"`
	Query   url.Values `json:"-"`
	Form    url.Values `json:"-"`
}

type requestJSON struct {
	Service string                     `json:"service"`
	Method  string                     `json:"method"`
	Path    string                     `json:"path"`
	Query   map[string]json.RawMessage `json:"query,omitempty"`
	Form    map[string]json.RawMessage `json:"form,omitempty"`
}

func encodeParameters(values url.Values) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for key, values := range values {
		if len(values) == 0 {
			return nil, fmt.Errorf("parameter %s has no values", key)
		}
		var value any = values
		if len(values) == 1 {
			value = values[0]
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		out[key] = raw
	}
	return out, nil
}

func decodeParameters(values map[string]json.RawMessage) (url.Values, error) {
	out := url.Values{}
	for key, raw := range values {
		var single string
		if len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &single) == nil {
			out[key] = []string{single}
			continue
		}
		var multiple []string
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &multiple) != nil || len(multiple) == 0 {
			return nil, fmt.Errorf("parameter %s must be a string or nonempty string array", key)
		}
		// null elements must not be coerced to empty strings by encoding/json.
		var elements []json.RawMessage
		if err := json.Unmarshal(raw, &elements); err != nil {
			return nil, err
		}
		for _, element := range elements {
			if len(element) == 0 || element[0] != '"' {
				return nil, fmt.Errorf("parameter %s has a non-string value", key)
			}
		}
		out[key] = multiple
	}
	return out, nil
}

func (r Request) MarshalJSON() ([]byte, error) {
	q, err := encodeParameters(r.Query)
	if err != nil {
		return nil, err
	}
	f, err := encodeParameters(r.Form)
	if err != nil {
		return nil, err
	}
	return json.Marshal(requestJSON{r.Service, r.Method, r.Path, q, f})
}

func (r *Request) UnmarshalJSON(raw []byte) error {
	var wire requestJSON
	if err := decode(raw, &wire); err != nil {
		return err
	}
	q, err := decodeParameters(wire.Query)
	if err != nil {
		return err
	}
	f, err := decodeParameters(wire.Form)
	if err != nil {
		return err
	}
	*r = Request{wire.Service, wire.Method, wire.Path, q, f}
	return nil
}

func (r Request) Key() string {
	return r.Service + " " + r.Method + " " + r.Path + "?" + r.Query.Encode() + "#" + r.Form.Encode()
}

// AuthenticationParameter is deliberately shared by capture and replay matching.
func AuthenticationParameter(key string) bool {
	switch strings.ToLower(key) {
	case "timestamp", "signature", "nonce", "recvwindow", "apikey", "api_key", "secret", "passphrase", "authorization", "api-secret", "api-sign", "api-key", "x-mbx-apikey", "ok-access-key", "ok-access-sign", "ok-access-passphrase", "x-bapi-api-key", "x-bapi-sign", "x-bapi-timestamp", "x-bapi-recv-window", "ok-access-timestamp", "apikeyname":
		return true
	}
	return false
}

func (r Request) Validate() error {
	if r.Service == "" || (r.Method != "GET" && r.Method != "POST" && r.Method != "WS") || !strings.HasPrefix(r.Path, "/") || strings.ContainsAny(r.Path, "?#") {
		return errors.New("request requires a service, read-only method and path without query or fragment")
	}
	if r.Method == "WS" && (len(r.Query) > 0 || len(r.Form) > 0) {
		return errors.New("logical WebSocket requests do not take HTTP parameters")
	}
	if r.Method != "POST" && len(r.Form) > 0 {
		return errors.New("form requires a POST request")
	}
	for _, params := range []url.Values{r.Query, r.Form} {
		for key, values := range params {
			if key == "" || AuthenticationParameter(key) || len(values) == 0 {
				return errors.New("request contains an empty or authentication parameter")
			}
		}
	}
	return nil
}

// File is one response. Recorded evidence and synthetic test inputs have independent lifecycles.
type File struct {
	Source     string          `json:"source"`
	RecordedAt *time.Time      `json:"recordedAt,omitempty"`
	Notes      string          `json:"notes,omitempty"`
	Request    Request         `json:"request"`
	Status     int             `json:"status,omitempty"`
	Body       json.RawMessage `json:"body"`
	BodyFormat string          `json:"bodyFormat,omitempty"`
}

// UnmarshalJSON rejects legacy fields and an explicitly stored HTTP status on WS snapshots.
func (f *File) UnmarshalJSON(raw []byte) error {
	type wire File
	var value wire
	if err := decode(raw, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if value.Source == "synthetic" {
		if _, present := fields["recordedAt"]; present {
			return errors.New("synthetic responses must omit recordedAt, including null")
		}
	}
	if value.Request.Method == "WS" {
		if _, present := fields["status"]; present {
			return errors.New("WebSocket snapshots must omit HTTP status")
		}
	}
	*f = File(value)
	return nil
}

func (f *File) Validate() error {
	if err := f.Request.Validate(); err != nil {
		return err
	}
	switch f.Source {
	case "recorded":
		if f.RecordedAt == nil || f.RecordedAt.IsZero() || f.RecordedAt.Location() != time.UTC {
			return errors.New("recorded evidence requires an actual UTC recording time")
		}
	case "synthetic":
		if f.RecordedAt != nil {
			return errors.New("synthetic responses cannot claim a recording time")
		}
	default:
		return errors.New("unknown or missing sample source")
	}
	if f.Request.Method == "WS" {
		if f.Status != 0 || f.BodyFormat != "" {
			return errors.New("WebSocket snapshots have no HTTP status or text body format")
		}
	} else if f.Status < 100 || f.Status > 599 {
		return errors.New("HTTP samples require a valid status")
	}
	if len(f.Body) == 0 || !json.Valid(f.Body) {
		return errors.New("sample body must be valid JSON")
	}
	if f.BodyFormat == "text" {
		var text string
		if len(f.Body) == 0 || f.Body[0] != '"' || json.Unmarshal(f.Body, &text) != nil {
			return errors.New("text bodies must be JSON strings")
		}
	} else if f.BodyFormat != "" {
		return errors.New("unknown body format")
	}
	return nil
}

func decode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func Read(path string) (File, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // Explicit local sample path.
	if err != nil {
		return File{}, err
	}
	var f File
	if err := decode(raw, &f); err != nil {
		return File{}, fmt.Errorf("decode sample %s: %w", path, err)
	}
	if err := f.Validate(); err != nil {
		return File{}, fmt.Errorf("invalid sample %s: %w", path, err)
	}
	return f, nil
}

// ReadSynthetic prevents tests from depending on recorded evidence, even through an explicit path.
func ReadSynthetic(path string) (File, error) {
	f, err := Read(path)
	if err != nil {
		return File{}, err
	}
	if f.Source != "synthetic" {
		return File{}, fmt.Errorf("test input %s must be synthetic; recorded evidence is reviewed independently", path)
	}
	return f, nil
}

// Name uses the endpoint path and an optional semantic variant, never query values.
func Name(path, variant string) string {
	path, _, _ = strings.Cut(path, "?")
	name := strings.NewReplacer("/", "_", "-", "_").Replace(strings.TrimPrefix(path, "/"))
	if variant != "" {
		name += "." + variant
	}
	return name
}

func ValidVariant(value string) bool {
	if value == "" {
		return true
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

// Write publishes a complete file atomically, without replacing an existing sample.
func Write(dir, name string, f *File) (string, error) {
	if !filepath.IsLocal(name) || filepath.Base(name) != name {
		return "", errors.New("invalid sample basename")
	}
	if err := f.Validate(); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(dir, ".incomplete-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err = temp.Write(append(raw, '\n')); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err = temp.Sync(); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".json")
	// Linking the complete temporary inode publishes atomically with exclusive creation.
	// Rename alone can overwrite a concurrent writer's existing file.
	if err := os.Link(temp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// Load reads exactly the named flat synthetic responses, with no evidence search or fallback.
func Load(root string, names ...string) ([]File, error) {
	files := make([]File, 0, len(names))
	for _, name := range names {
		if !filepath.IsLocal(name) || filepath.Base(name) != name || filepath.Ext(name) != ".json" {
			return nil, fmt.Errorf("invalid flat sample name %q", name)
		}
		f, err := ReadSynthetic(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// Target and Endpoint are derived labels for local test dispatch, not duplicate stored metadata.
func (r Request) Target() string {
	if len(r.Query) == 0 {
		return r.Path
	}
	return r.Path + "?" + r.Query.Encode()
}
func (r Request) Endpoint() string { return r.Method + " " + r.Target() }
