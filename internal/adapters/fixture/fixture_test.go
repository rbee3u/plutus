package fixture

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func synthetic() File {
	return File{Source: "synthetic", Request: Request{Service: "spot", Method: "GET", Path: "/account"}, Status: 200, Body: []byte(`{"balances":[]}`)}
}

func TestParametersRoundTripWithoutCursorDecoding(t *testing.T) {
	request := Request{Service: "spot", Method: "POST", Path: "/account", Query: url.Values{"single": {"1"}, "multi": {"a", "b"}}, Form: url.Values{"cursor": {"next%2Fpage"}}}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"single":"1"`)) || !bytes.Contains(raw, []byte(`"multi":["a","b"]`)) {
		t.Fatalf("wrong parameter shape: %s", raw)
	}
	var got Request
	if err = json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got, request) {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if got.Form.Get("cursor") == "next/page" {
		t.Fatal("cursor decoded twice")
	}
	for _, raw := range []string{`{"service":"s","method":"GET","path":"/a","query":{"x":1}}`, `{"service":"s","method":"GET","path":"/a","query":{"x":null}}`, `{"service":"s","method":"GET","path":"/a","query":{"x":[]}}`, `{"service":"s","method":"GET","path":"/a","query":{"x":[null]}}`, `{"service":"s","method":"GET","path":"/a","unknown":true}`} {
		if json.Unmarshal([]byte(raw), &got) == nil {
			t.Fatalf("invalid request accepted: %s", raw)
		}
	}
}

func TestSourceAndProtocolValidation(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name   string
		change func(*File)
		valid  bool
	}{
		{"legacy documented source", func(f *File) { f.Source = "documented" }, false},
		{"recorded", func(f *File) { f.Source = "recorded"; f.RecordedAt = &now }, true},
		{"synthetic", func(f *File) { f.Source = "synthetic" }, true},
		{"WS", func(f *File) { f.Request.Method = "WS"; f.Status = 0 }, true},
		{"text", func(f *File) { f.BodyFormat = "text"; f.Body = []byte(`"too frequent"`); f.Status = 403 }, true},
		{"unknown source", func(f *File) { f.Source = "unknown" }, false},
		{"invented date", func(f *File) { f.RecordedAt = &now }, false},
		{"missing date", func(f *File) { f.Source = "recorded" }, false},
		{"missing status", func(f *File) { f.Status = 0 }, false},
		{"fake WS status", func(f *File) { f.Request.Method = "WS" }, false},
		{"object text", func(f *File) { f.BodyFormat = "text" }, false},
		{"signature", func(f *File) { f.Request.Query = url.Values{"signature": {"secret"}} }, false},
		{"missing body", func(f *File) { f.Body = nil }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := synthetic()
			tc.change(&f)
			if (f.Validate() == nil) != tc.valid {
				t.Fatalf("unexpected validation: %v", f.Validate())
			}
		})
	}
}

func TestAtomicWriteCannotOverwriteOrLeavePartialSample(t *testing.T) {
	dir := t.TempDir()
	f := synthetic()
	path, err := Write(dir, "account", &f)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Body = []byte(`{"balances":[1]}`)
	if _, err = Write(dir, "account", &f); err == nil {
		t.Fatal("existing file overwritten")
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatal("existing file changed")
	}
	f.Body = []byte(`broken`)
	if _, err = Write(dir, "invalid", &f); err == nil {
		t.Fatal("invalid sample written")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".incomplete-*"))
	if err != nil || len(leftovers) > 0 {
		t.Fatalf("partial write exposed: %v %v", leftovers, err)
	}
	if _, err = Write(filepath.Join(path, "not-a-directory"), "account", &syntheticFile); err == nil {
		t.Fatal("disk failure ignored")
	}
}

var syntheticFile = synthetic()

func TestLoadHasNoFallbackOrNestedPaths(t *testing.T) {
	dir := t.TempDir()
	f := synthetic()
	if _, err := Write(dir, "account", &f); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"account.pm.json", "../account.json", "nested/account.json"} {
		if _, err := Load(dir, name); err == nil {
			t.Fatalf("unexpected fallback for %s", name)
		}
	}
	legacy := []byte(`{"fixture":{"scenario":"recorded"},"status":200,"body":{}}`)
	path := filepath.Join(dir, "legacy.json")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("legacy wrapper accepted")
	}
}

func TestSequenceMatchesEveryBusinessParameterAndConsumesEveryStep(t *testing.T) {
	first := synthetic()
	first.Request.Form = url.Values{"cursor": {"a%2Fb"}, "coin": {"BTC", "ETH"}}
	first.Request.Method = "POST"
	second := first
	second.Body = []byte(`{"rows":[2]}`)
	sequence, err := NewSequence(first, second)
	if err != nil {
		t.Fatal(err)
	}
	if sequence.Done() == nil {
		t.Fatal("unused expected responses accepted")
	}
	for _, want := range []File{first, second} {
		got, e := sequence.Next(want.Request)
		if e != nil || !bytes.Equal(got.Body, want.Body) {
			t.Fatal("repeated response order lost")
		}
	}
	if sequence.Done() != nil {
		t.Fatal(sequence.Done())
	}
	for _, change := range []func(*Request){func(r *Request) { r.Service = "other" }, func(r *Request) { r.Method = "GET" }, func(r *Request) { r.Path = "/other" }, func(r *Request) { r.Form = url.Values{"cursor": {"a/b"}} }, func(r *Request) { r.Query = url.Values{"page": {"2"}} }} {
		sequence, _ = NewSequence(first)
		request := first.Request
		change(&request)
		if _, e := sequence.Next(request); e == nil || sequence.Done() == nil {
			t.Fatal("request mismatch hidden")
		}
	}
}

func TestSnapshotCannotStoreEvenAZeroHTTPStatus(t *testing.T) {
	var f File
	raw := []byte(`{"source":"synthetic","request":{"service":"futures-ws","method":"WS","path":"/ws/v1/balances_snapshot"},"status":0,"body":{}}`)
	if json.Unmarshal(raw, &f) == nil {
		t.Fatal("explicit WS status accepted")
	}
}

func TestEvidenceCannotBecomeATestInput(t *testing.T) {
	dir := t.TempDir()
	f := synthetic()
	now := time.Now().UTC()
	f.Source, f.RecordedAt = "recorded", &now
	path, err := Write(dir, "account", &f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err != nil {
		t.Fatalf("evidence envelope: %v", err)
	}
	if _, err := ReadSynthetic(path); err == nil {
		t.Fatal("recorded response accepted as a test input")
	}
	if _, err := Load(dir, "account.json"); err == nil {
		t.Fatal("test loader accepted recorded evidence")
	}
	if _, err := NewSequence(f); err == nil {
		t.Fatal("test sequence accepted recorded evidence")
	}
	// Documentation is maintained in the inventory, not in a third response-file source.
	var value File
	if json.Unmarshal([]byte(`{"source":"synthetic","documentation":"https://docs.kraken.com/","request":{"service":"spot","method":"GET","path":"/account"},"status":200,"body":{}}`), &value) == nil {
		t.Fatal("legacy documentation provenance accepted")
	}
	if json.Unmarshal([]byte(`{"source":"synthetic","recordedAt":null,"request":{"service":"spot","method":"GET","path":"/account"},"status":200,"body":{}}`), &value) == nil {
		t.Fatal("synthetic response retained recording metadata")
	}
}
