package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestProxyConfiguration(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, mode, address, want string }{
		{"default", "environment", "", ""},
		{"direct", "direct", "", ""},
		{"http", "proxy", "http://127.0.0.1:7890", ""},
		{"https", "proxy", "https://proxy.example:443/", ""},
		{"socks5", "proxy", "socks5://proxy.example:1080", ""},
		{"socks5h", "proxy", "socks5h://127.0.0.1:1080", ""},
		{"empty mode", "", "", "collection.proxy.mode"},
		{"unknown mode", "auto", "", "collection.proxy.mode"},
		{"missing URL", "proxy", "", "collection.proxy.url"},
		{"unused URL", "direct", "http://127.0.0.1:7890", "collection.proxy.url"},
		{"environment URL", "environment", "http://127.0.0.1:7890", "collection.proxy.url"},
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

func TestProxyConfigPrecedence(t *testing.T) {
	t.Parallel()
	files := map[string]string{"app.toml": "[collection.proxy]\nmode = 'proxy'\nurl = 'http://127.0.0.1:7890'\n", "app.env": "PLUTUS_COLLECTION_PROXY_MODE=proxy\nPLUTUS_COLLECTION_PROXY_URL=socks5h://127.0.0.1:1080\n"}
	for _, tc := range []struct {
		name    string
		env     map[string]string
		envFile string
		want    Proxy
	}{
		{"defaults", nil, "", Proxy{Mode: "environment"}},
		{"TOML", nil, "", Proxy{Mode: "proxy", URL: "http://127.0.0.1:7890"}},
		{"env file", nil, "app.env", Proxy{Mode: "proxy", URL: "socks5h://127.0.0.1:1080"}},
		{"process overrides", map[string]string{"PLUTUS_COLLECTION_PROXY_MODE": "direct", "PLUTUS_COLLECTION_PROXY_URL": ""}, "app.env", Proxy{Mode: "direct"}},
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
