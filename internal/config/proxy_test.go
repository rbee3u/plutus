package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestProxyConfiguration(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, mode, address, want string }{
		{"removed environment", "environment", "", "set direct or configure proxy"},
		{"direct", "direct", "", ""},
		{"http", "proxy", "http://127.0.0.1:7890", ""},
		{"https", "proxy", "https://proxy.example:443/", ""},
		{"socks5", "proxy", "socks5://proxy.example:1080", ""},
		{"socks5h", "proxy", "socks5h://127.0.0.1:1080", ""},
		{"empty mode", "", "", "collection.proxy.mode"},
		{"unknown mode", "auto", "", "collection.proxy.mode"},
		{"missing URL", "proxy", "", "collection.proxy.url"},
		{"unused URL", "direct", "http://127.0.0.1:7890", "collection.proxy.url"},
		{"environment URL", "environment", "http://127.0.0.1:7890", "collection.proxy.mode"},
		{"relative", "proxy", "127.0.0.1:7890", "collection.proxy.url"},
		{"SS", "proxy", "ss://proxy.example:443", "collection.proxy.url"},
		{"missing host", "proxy", "http://:7890", "collection.proxy.url"},
		{"missing port", "proxy", "http://proxy.example", "collection.proxy.url"},
		{"zero port", "proxy", "http://proxy.example:0", "collection.proxy.url"},
		{"large port", "proxy", "http://proxy.example:65536", "collection.proxy.url"},
		{"bad port", "proxy", "http://proxy.example:bad", "collection.proxy.url"},
		{"path", "proxy", "http://proxy.example:7890/path", "collection.proxy.url"},
		{"encoded path", "proxy", "http://proxy.example:7890/%2f", "collection.proxy.url"},
		{"query", "proxy", "http://proxy.example:7890/?x=1", "collection.proxy.url"},
		{"empty query", "proxy", "http://proxy.example:7890/?", "collection.proxy.url"},
		{"fragment", "proxy", "http://proxy.example:7890/#x", "collection.proxy.url"},
		{"empty fragment", "proxy", "http://proxy.example:7890/#", "collection.proxy.url"},
		{"credentials", "proxy", "http://privateuser:privatepassword@proxy.example:7890", "collection.proxy.url"},
		{"malformed credentials", "proxy", "http://privateuser:privatepassword@proxy.example:bad", "collection.proxy.url"},
		{"malformed escape", "proxy", "http://privateuser:privatepassword@proxy.example:7890/%zz", "collection.proxy.url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(Options{Path: "app.toml", LookupEnv: envMap(nil), ReadFile: filesMap(map[string]string{"app.toml": fmt.Sprintf("[collection.proxy]\nmode = %q\nurl = %q\n", tc.mode, tc.address)})})
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if cfg.Collection.Proxy != (Proxy{Mode: tc.mode, URL: tc.address}) {
					t.Fatalf("proxy = %+v", cfg.Collection.Proxy)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %s", err, tc.want)
			}
			for _, secret := range []string{"privateuser", "privatepassword"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatal("proxy error exposed credentials")
				}
			}
		})
	}
}

func TestProxyConfigIgnoresEnvironment(t *testing.T) {
	t.Parallel()
	files := map[string]string{"app.toml": "[collection.proxy]\nmode = 'proxy'\nurl = 'http://127.0.0.1:7890'\n", "app.env": "PLUTUS_COLLECTION_PROXY_MODE=proxy\nPLUTUS_COLLECTION_PROXY_URL=socks5h://127.0.0.1:1080\n"}
	for _, tc := range []struct {
		name    string
		env     map[string]string
		envFile string
		want    Proxy
	}{
		{"defaults", nil, "", Proxy{Mode: "direct"}},
		{"TOML", nil, "", Proxy{Mode: "proxy", URL: "http://127.0.0.1:7890"}},
		{"env file", nil, "app.env", Proxy{Mode: "proxy", URL: "http://127.0.0.1:7890"}},
		{"process overrides", map[string]string{"PLUTUS_COLLECTION_PROXY_MODE": "direct", "PLUTUS_COLLECTION_PROXY_URL": ""}, "app.env", Proxy{Mode: "proxy", URL: "http://127.0.0.1:7890"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "app.toml"
			if tc.name == "defaults" {
				path = ""
			}
			cfg, err := Load(Options{Path: path, EnvFile: tc.envFile, LookupEnv: envMap(tc.env), ReadFile: filesMap(files)})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Collection.Proxy != tc.want {
				t.Fatalf("proxy = %+v, want %+v", cfg.Collection.Proxy, tc.want)
			}
		})
	}
}

func TestRecordingProxyLoading(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, contents, wantError string
		missing                         bool
		want                            Proxy
	}{
		{name: "missing default", missing: true, want: Proxy{Mode: "direct"}},
		{name: "empty default", want: Proxy{Mode: "direct"}},
		{name: "default proxy", contents: "[proxy]\nmode = 'proxy'\nurl = 'socks5h://127.0.0.1:1080'", want: Proxy{Mode: "proxy", URL: "socks5h://127.0.0.1:1080"}},
		{name: "explicit direct", path: "chosen.toml", contents: "[proxy]\nmode = 'direct'", want: Proxy{Mode: "direct"}},
		{name: "explicit missing", path: "missing.toml", missing: true, wantError: "read config file missing.toml"},
		{name: "syntax", contents: "[proxy", wantError: "parse config file"},
		{name: "unknown field", contents: "[proxy]\nextra = 'hidden-value'", wantError: "unsupported fields: proxy.extra"},
		{name: "runtime config", contents: "[collection.proxy]\nmode = 'direct'", wantError: "unsupported fields: collection"},
		{name: "secrets", contents: "[database]\ndsn = 'hidden-value'", wantError: "unsupported fields: database"},
		{name: "legacy mode", contents: "[proxy]\nmode = 'environment'", wantError: "set direct or configure proxy"},
		{name: "empty mode", contents: "[proxy]\nmode = ''", wantError: "proxy.mode"},
		{name: "missing URL", contents: "[proxy]\nmode = 'proxy'", wantError: "proxy.url"},
		{name: "unused URL", contents: "[proxy]\nurl = 'http://127.0.0.1:1080'", wantError: "proxy.url"},
		{name: "credentials", contents: "[proxy]\nmode = 'proxy'\nurl = 'http://hidden-value:password@127.0.0.1:1080'", wantError: "proxy.url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path
			if path == "" {
				path = defaultRecordConfigFileName
			}
			policy, err := LoadRecordingProxy(Options{
				Path: tc.path, EnvFile: "must-not-read.env",
				LookupEnv: func(key string) (string, bool) {
					t.Fatalf("recording proxy consulted environment: %s", key)
					return "", false
				},
				ReadFile: func(name string) ([]byte, error) {
					if name != path {
						t.Fatalf("recording proxy read unrelated file: %s", name)
					}
					if tc.missing {
						return nil, os.ErrNotExist
					}
					return []byte(tc.contents), nil
				},
			})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || strings.Contains(err.Error(), "hidden-value") {
					t.Fatalf("expected safe error containing %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil || policy != tc.want {
				t.Fatalf("proxy=%+v error=%v, want %+v", policy, err, tc.want)
			}
		})
	}
}

func TestRecordingProxyExample(t *testing.T) {
	t.Parallel()
	policy, err := LoadRecordingProxy(Options{Path: "../../config/record-example.toml"})
	if err != nil || policy != (Proxy{Mode: "direct"}) {
		t.Fatalf("example proxy=%+v error=%v", policy, err)
	}
}
