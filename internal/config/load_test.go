package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func envMap(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func filesMap(values map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		v, ok := values[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(v), nil
	}
}

func TestLoadDefaultsWithoutFile(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		LookupEnv: envMap(nil),
		ReadFile:  filesMap(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ListenAddress != "0.0.0.0" || cfg.Server.Port != 8080 {
		t.Fatalf("unexpected listen %s:%d", cfg.Server.ListenAddress, cfg.Server.Port)
	}
	if cfg.Auth.SessionCookieName() != "plutus_session" || cfg.Auth.CSRFCookieName() != "plutus_csrf" {
		t.Fatalf("cookie defaults %s %s", cfg.Auth.SessionCookieName(), cfg.Auth.CSRFCookieName())
	}
	if cfg.Database.DSN != "" || cfg.Crypto.HasMasterKey() {
		t.Fatal("secrets should be absent by default")
	}
}

func TestLoadTOMLAndEnvSecrets(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path: "plutus.toml",
		LookupEnv: envMap(map[string]string{
			"PLUTUS_SERVER_PORT": "9090",
			"PLUTUS_DSN":         "user:pass@tcp(127.0.0.1:3306)/plutus",
		}),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[server]
listen_address = "127.0.0.1"
port = 8080
[auth]
session_idle_lifetime = "24h"
session_absolute_lifetime = "48h"
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("TOML port: got %d", cfg.Server.Port)
	}
	if cfg.Auth.SessionIdleLifetime != 24*time.Hour {
		t.Fatalf("idle lifetime: %s", cfg.Auth.SessionIdleLifetime)
	}
	if !strings.Contains(cfg.Database.DSN, "plutus") {
		t.Fatalf("dsn: %s", cfg.Database.DSN)
	}
}

func TestExclusiveDSNSources(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		LookupEnv: envMap(map[string]string{
			envDSN:     "dsn",
			envDSNFile: "dsn.txt",
		}),
		ReadFile: filesMap(map[string]string{"dsn.txt": "other"}),
	})
	if err == nil || !strings.Contains(err.Error(), "PLUTUS_DSN or PLUTUS_DSN_FILE") {
		t.Fatalf("expected exclusive DSN error naming the supported variables, got %v", err)
	}
}

func TestDSNFromFile(t *testing.T) {
	t.Parallel()
	cfg, err := Load(Options{
		LookupEnv: envMap(map[string]string{envDSNFile: "dsn.txt"}),
		ReadFile:  filesMap(map[string]string{"dsn.txt": "file-dsn\n"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "file-dsn" {
		t.Fatalf("expected DSN from secret file")
	}
}

func TestMasterKeyFromFile(t *testing.T) {
	t.Parallel()

	raw := make([]byte, masterKeySize)
	for i := range raw {
		raw[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	cfg, err := Load(Options{
		LookupEnv: envMap(map[string]string{envMasterKeyFile: "key.txt"}),
		ReadFile:  filesMap(map[string]string{"key.txt": encoded + "\n"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Crypto.HasMasterKey() {
		t.Fatal("expected master key")
	}
}

func TestMasterKeyRejectsWrongLength(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		LookupEnv: envMap(map[string]string{envMasterKey: base64.StdEncoding.EncodeToString([]byte("short"))}),
		ReadFile:  filesMap(nil),
	})
	if err == nil {
		t.Fatal("expected master key length error")
	}
}

func TestTLSMatrix(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	certPEM, keyPEM := mustCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	expiredPEM, expiredKey := mustCert(t, now.Add(-48*time.Hour), now.Add(-time.Hour))
	_, otherKey := mustCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	files := map[string]string{
		"cert.pem": string(certPEM), "key.pem": string(keyPEM),
		"expired.pem": string(expiredPEM), "expired.key": string(expiredKey),
		"other.key": string(otherKey), "ca.pem": string(certPEM),
		"bad-ca.pem": "not a certificate", "bad-cert.pem": "not a certificate",
	}
	cases := []struct {
		name, cert, key, ca, wantError string
		https, mtls                    bool
	}{
		{name: "http"},
		{name: "https", cert: "cert.pem", key: "key.pem", https: true},
		{name: "mtls", cert: "cert.pem", key: "key.pem", ca: "ca.pem", https: true, mtls: true},
		{name: "certificate only", cert: "cert.pem", wantError: "configured together"},
		{name: "key only", key: "key.pem", wantError: "configured together"},
		{name: "ca without https", ca: "ca.pem", wantError: "requires cert_file and key_file"},
		{name: "expired", cert: "expired.pem", key: "expired.key", wantError: "expired"},
		{name: "mismatch", cert: "cert.pem", key: "other.key", wantError: "do not match"},
		{name: "missing certificate", cert: "missing.pem", key: "key.pem", wantError: "read TLS certificate"},
		{name: "malformed certificate", cert: "bad-cert.pem", key: "key.pem", wantError: "malformed"},
		{name: "invalid ca", cert: "cert.pem", key: "key.pem", ca: "bad-ca.pem", wantError: "contains no certificates"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			readFile := func(path string) ([]byte, error) {
				if path == "plutus.toml" {
					return []byte(fmt.Sprintf("[server.tls]\ncert_file = %q\nkey_file = %q\nclient_ca_file = %q\n", tc.cert, tc.key, tc.ca)), nil
				}
				return filesMap(files)(path)
			}
			cfg, err := Load(Options{Path: "plutus.toml", LookupEnv: envMap(nil), ReadFile: readFile, Now: now})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %q, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Server.TLS.HTTPSEnabled() != tc.https || cfg.Server.TLS.MTLSEnabled() != tc.mtls {
				t.Fatalf("wrong transport mode: https=%v mtls=%v", cfg.Server.TLS.HTTPSEnabled(), cfg.Server.TLS.MTLSEnabled())
			}
		})
	}
}

func loadExampleTOML(t *testing.T, path string) Config {
	t.Helper()
	now := time.Now()
	certPEM, keyPEM := mustCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	cfg, err := Load(Options{
		Path:      path,
		LookupEnv: envMap(nil),
		EnvFile:   "empty.env",
		Now:       now,
		ReadFile: func(name string) ([]byte, error) {
			switch name {
			case "empty.env":
				return []byte{}, nil
			case "server.pem", "client-ca.pem":
				return certPEM, nil
			case "server-key.pem":
				return keyPEM, nil
			}
			return os.ReadFile(name)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestExampleTOMLLoads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, listen, suffix string
		port                 int
		fake, reset, mtls    bool
	}{
		{"local", "0.0.0.0", "local", 8080, true, true, false},
		{"agent", "0.0.0.0", "agent", 8081, true, true, false},
		{"test", "0.0.0.0", "test", 8082, false, true, true},
		{"prod", "0.0.0.0", "", 8083, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := loadExampleTOML(t, "../../config/"+tc.name+"-example.toml")
			if cfg.Server.ListenAddress != tc.listen || cfg.Server.Port != tc.port {
				t.Fatalf("unexpected listener: %s", cfg.Server.Addr())
			}
			if cfg.Database.DSN != "" || cfg.Crypto.HasMasterKey() {
				t.Fatal("examples must keep secrets out of TOML")
			}
			if cfg.Collection.FakeAdapter != tc.fake || cfg.Database.AllowReset != tc.reset {
				t.Fatal("example does not preserve its environment boundaries")
			}
			suffix := ""
			if tc.suffix != "" {
				suffix = "_" + tc.suffix
			}
			if cfg.Auth.SessionCookieName() != "plutus_session"+suffix || cfg.Auth.CSRFCookieName() != "plutus_csrf"+suffix {
				t.Fatal("example does not isolate its session and CSRF cookies")
			}
			if cfg.Server.TLS.HTTPSEnabled() != tc.mtls || cfg.Server.TLS.MTLSEnabled() != tc.mtls {
				t.Fatal("remote examples must require mTLS; dev examples must use HTTP")
			}
		})
	}
}

func TestUnsupportedTOMLFields(t *testing.T) {
	t.Parallel()
	cases := []struct{ table, key, value string }{
		{"collection", "fake_adaptor", "true"},
		{"unknown", "option", "true"},
		{"database", "dsn", "\"secret-dsn\""},
		{"server.tls", "https_enabled", "false"},
		{"server.tls", "https_cert_path", "\"\""},
		{"server.tls", "https_key_path", "\"\""},
		{"server.tls", "mtls_enabled", "false"},
		{"server.tls", "mtls_ca_path", "\"\""},
		{"auth", "session_cookie_name", "\"old\""},
		{"auth", "csrf_cookie_name", "\"old\""},
		{"jobs", "worker_poll_interval", "\"1s\""},
		{"jobs", "lease_duration", "\"2m\""},
		{"jobs", "completed_retention", "\"2160h\""},
		{"collection", "default_interval", "\"15m\""},
		{"collection", "stagger", "\"3s\""},
		{"collection", "validate_timeout", "\"15s\""},
	}
	for _, tc := range cases {
		t.Run(tc.table+"."+tc.key, func(t *testing.T) {
			t.Parallel()
			_, err := Load(Options{
				Path: "plutus.toml", LookupEnv: envMap(nil),
				ReadFile: filesMap(map[string]string{
					"plutus.toml": "[" + tc.table + "]\n" + tc.key + " = " + tc.value,
				}),
			})
			want := tc.table + "." + tc.key
			if tc.table == "unknown" {
				want = tc.table
			}
			if err == nil || !strings.Contains(err.Error(), "unsupported fields") || !strings.Contains(err.Error(), want) {
				t.Fatalf("expected unsupported field error, got %v", err)
			}
			if strings.Contains(err.Error(), "secret-dsn") {
				t.Fatal("unsupported-field diagnostics must not include values")
			}
		})
	}
}

func TestAllowResetFromTOML(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[database]
allow_reset = true
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Database.AllowReset {
		t.Fatal("allow_reset should load from TOML")
	}
}

func TestCookieSuffixValidation(t *testing.T) {
	t.Parallel()
	_, err := Load(Options{
		Path: "plutus.toml", LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{"plutus.toml": "[auth]\ncookie_suffix = \"bad name\""}),
	})
	if err == nil || !strings.Contains(err.Error(), "auth.cookie_suffix") {
		t.Fatalf("expected invalid cookie suffix, got %v", err)
	}
}

func TestDSNFromEnv(t *testing.T) {
	t.Parallel()
	cfg, err := Load(Options{
		LookupEnv: envMap(map[string]string{envDSN: "env-dsn"}),
		ReadFile:  filesMap(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "env-dsn" {
		t.Fatal("expected DSN from environment")
	}
}

func TestInvalidExplicitValuesRejected(t *testing.T) {
	t.Parallel()
	cases := []struct{ table, key, value string }{
		{"server", "port", "0"},
		{"server", "listen_address", `""`},
		{"auth", "session_idle_lifetime", `""`},
		{"auth", "session_absolute_lifetime", `" "`},
		{"jobs", "finished_retention", `""`},
		{"jobs", "finished_retention", `"0s"`},
		{"jobs", "finished_retention", `"1h"`},
		{"collection", "connection_validation_timeout", `""`},
		{"collection", "connection_validation_timeout", `"0s"`},
	}
	for _, tc := range cases {
		t.Run(tc.table+"."+tc.key+"="+tc.value, func(t *testing.T) {
			t.Parallel()
			_, err := Load(Options{
				Path: "plutus.toml", LookupEnv: envMap(nil),
				ReadFile: filesMap(map[string]string{"plutus.toml": "[" + tc.table + "]\n" + tc.key + " = " + tc.value}),
			})
			if err == nil || !strings.Contains(err.Error(), tc.table+"."+tc.key) {
				t.Fatalf("expected invalid field diagnostic, got %v", err)
			}
		})
	}
}

func TestOrdinaryEnvironmentSettingsIgnored(t *testing.T) {
	t.Parallel()
	keys := []string{
		"PLUTUS_SERVER_LISTEN_ADDRESS", "PLUTUS_SERVER_PORT",
		"PLUTUS_SERVER_TLS_CERT_FILE", "PLUTUS_SERVER_TLS_KEY_FILE", "PLUTUS_SERVER_TLS_CLIENT_CA_FILE",
		"PLUTUS_AUTH_COOKIE_SUFFIX", "PLUTUS_AUTH_SESSION_IDLE_LIFETIME", "PLUTUS_AUTH_SESSION_ABSOLUTE_LIFETIME",
		"PLUTUS_JOBS_FINISHED_RETENTION", "PLUTUS_COLLECTION_CONNECTION_VALIDATION_TIMEOUT",
		"PLUTUS_COLLECTION_FAKE_ADAPTER", "PLUTUS_COLLECTION_PROXY_MODE", "PLUTUS_COLLECTION_PROXY_URL",
		"PLUTUS_DATABASE_ALLOW_RESET",
		"PLUTUS_SERVER_TLS_HTTPS_ENABLED", "PLUTUS_SERVER_TLS_HTTPS_CERT_PATH",
		"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH", "PLUTUS_SERVER_TLS_MTLS_ENABLED",
		"PLUTUS_SERVER_TLS_MTLS_CA_PATH", "PLUTUS_AUTH_SESSION_COOKIE_NAME",
		"PLUTUS_AUTH_CSRF_COOKIE_NAME", "PLUTUS_JOBS_WORKER_POLL_INTERVAL",
		"PLUTUS_JOBS_LEASE_DURATION", "PLUTUS_JOBS_COMPLETED_RETENTION",
		"PLUTUS_COLLECTION_DEFAULT_INTERVAL", "PLUTUS_COLLECTION_STAGGER", "PLUTUS_COLLECTION_VALIDATE_TIMEOUT",
	}
	for _, toml := range []string{"", `[server]
listen_address = "127.0.0.1"
port = 8081
[auth]
cookie_suffix = "agent"
session_idle_lifetime = "24h"
session_absolute_lifetime = "48h"
[jobs]
finished_retention = "48h"
[collection]
connection_validation_timeout = "20s"
fake_adapter = true
[collection.proxy]
mode = "proxy"
url = "http://127.0.0.1:7890"
[database]
allow_reset = true
`} {
		for _, source := range []string{"process", "dotenv"} {
			for _, value := range []string{"", "invalid", "true", "9090"} {
				t.Run(fmt.Sprintf("toml=%t/%s/%s", toml != "", source, value), func(t *testing.T) {
					t.Parallel()
					files := map[string]string{"app.toml": toml, "app.env": ""}
					opts := Options{Path: "app.toml", EnvFile: "app.env", LookupEnv: envMap(nil), ReadFile: filesMap(files)}
					want, err := Load(opts)
					if err != nil {
						t.Fatal(err)
					}
					env := map[string]string{}
					for _, key := range keys {
						env[key] = value
						if source == "dotenv" {
							files["app.env"] += key + "=" + value + "\n"
						}
					}
					if source == "process" {
						opts.LookupEnv = envMap(env)
					}
					got, err := Load(opts)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatal("ordinary environment settings changed TOML or defaults")
					}
				})
			}
		}
	}
}

func mustCert(t *testing.T, notBefore, notAfter time.Time) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "plutus-test"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM
}
