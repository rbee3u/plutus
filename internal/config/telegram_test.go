package config

import (
	"strings"
	"testing"
)

func TestTelegramDefaultsDisabled(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		EnvFile:   "empty.env",
		LookupEnv: envMap(nil),
		ReadFile:  filesMap(map[string]string{"empty.env": ""}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Telegram.Enabled() {
		t.Fatal("telegram should be disabled without a token and chat id")
	}
	if cfg.Telegram.Proxy.Mode != "direct" {
		t.Fatalf("telegram proxy default: %q", cfg.Telegram.Proxy.Mode)
	}
}

func TestTelegramLoadsChatIDAndToken(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path: "plutus.toml",
		LookupEnv: envMap(map[string]string{
			envTelegramToken: "123:token",
		}),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[telegram]
chat_id = "-1001234567890"
[telegram.proxy]
mode = "proxy"
url = "http://127.0.0.1:7893"
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Telegram.Enabled() {
		t.Fatal("expected telegram to be enabled")
	}
	if cfg.Telegram.BotToken != "123:token" || cfg.Telegram.ChatID != "-1001234567890" {
		t.Fatalf("telegram settings: token=%q chat=%q", cfg.Telegram.BotToken, cfg.Telegram.ChatID)
	}
	if cfg.Telegram.Proxy.Mode != "proxy" || cfg.Telegram.Proxy.URL != "http://127.0.0.1:7893" {
		t.Fatalf("telegram proxy: %+v", cfg.Telegram.Proxy)
	}
}

func TestTelegramTokenFromEnvFile(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path:      "plutus.toml",
		EnvFile:   "secrets.env",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"secrets.env": "TG_RISK_TOKEN=123:from-file\n",
			"plutus.toml": "[telegram]\nchat_id = \"-1001234567890\"\n",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Telegram.Enabled() || cfg.Telegram.BotToken != "123:from-file" {
		t.Fatalf("telegram token from env file: enabled=%v token=%q", cfg.Telegram.Enabled(), cfg.Telegram.BotToken)
	}
}

func TestTelegramRequiresBothSettings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		toml string
	}{
		{name: "token only", env: map[string]string{envTelegramToken: "123:token"}},
		{name: "chat only", toml: "[telegram]\nchat_id = \"-1001234567890\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load(Options{
				Path:      "plutus.toml",
				LookupEnv: envMap(tc.env),
				ReadFile:  filesMap(map[string]string{"plutus.toml": tc.toml}),
			})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Telegram.Enabled() {
				t.Fatal("telegram must stay disabled with only one of token and chat id")
			}
		})
	}
}

func TestTelegramRejectsEmptyToken(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(map[string]string{envTelegramToken: "  "}),
		ReadFile:  filesMap(map[string]string{"plutus.toml": "[telegram]\nchat_id = \"-1001234567890\"\n"}),
	})
	if err == nil || !strings.Contains(err.Error(), "TG_RISK_TOKEN is empty") {
		t.Fatalf("expected empty token error, got %v", err)
	}
}

func TestTelegramProxyValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, toml, want string
	}{
		{
			name: "proxy without url",
			toml: "[telegram.proxy]\nmode = \"proxy\"\n",
			want: "telegram.proxy.url is required",
		},
		{
			name: "direct with url",
			toml: "[telegram.proxy]\nmode = \"direct\"\nurl = \"http://127.0.0.1:7893\"\n",
			want: "telegram.proxy.url must be empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(Options{
				Path:      "plutus.toml",
				LookupEnv: envMap(nil),
				ReadFile:  filesMap(map[string]string{"plutus.toml": tc.toml}),
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}
