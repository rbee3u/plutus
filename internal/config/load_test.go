package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
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
	if cfg.Server.ListenAddress != "127.0.0.1" || cfg.Server.Port != 8080 {
		t.Fatalf("unexpected listen %s:%d", cfg.Server.ListenAddress, cfg.Server.Port)
	}
	if cfg.Auth.SessionCookieName != "plutus_session" || cfg.Auth.CSRFCookieName != "plutus_csrf" {
		t.Fatalf("cookie defaults %s %s", cfg.Auth.SessionCookieName, cfg.Auth.CSRFCookieName)
	}
	if cfg.Database.DSN != "" || cfg.Crypto.HasMasterKey() {
		t.Fatal("secrets should be absent by default")
	}
}

func TestLoadTOMLAndEnvOverride(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path: "plutus.toml",
		LookupEnv: envMap(map[string]string{
			"PLUTUS_SERVER_PORT":  "9090",
			"PLUTUS_DATABASE_DSN": "user:pass@tcp(127.0.0.1:3306)/plutus",
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
	if cfg.Server.Port != 9090 {
		t.Fatalf("port override: got %d", cfg.Server.Port)
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
			envDatabaseDSN:     "dsn",
			envDatabaseDSNFile: "dsn.txt",
		}),
		ReadFile: filesMap(map[string]string{"dsn.txt": "other"}),
	})
	if err == nil {
		t.Fatal("expected exclusive DSN error")
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
	otherCert, otherKey := mustCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))
	caPEM, _ := mustCert(t, now.Add(-time.Hour), now.Add(24*time.Hour))

	files := filesMap(map[string]string{
		"cert.pem":    string(certPEM),
		"key.pem":     string(keyPEM),
		"expired.pem": string(expiredPEM),
		"expired.key": string(expiredKey),
		"other.pem":   string(otherCert),
		"other.key":   string(otherKey),
		"ca.pem":      string(caPEM),
	})

	t.Run("https missing paths", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{"PLUTUS_SERVER_TLS_HTTPS_ENABLED": "true"}),
			ReadFile:  files,
			Now:       now,
		})
		if err == nil {
			t.Fatal("expected missing cert paths")
		}
	})

	t.Run("https valid", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "key.pem",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.TLS.Certificate == nil {
			t.Fatal("expected loaded certificate")
		}
	})

	t.Run("expired certificate", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "expired.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "expired.key",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err == nil {
			t.Fatal("expected expired certificate")
		}
	})

	t.Run("mismatched key", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "other.key",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err == nil {
			t.Fatal("expected mismatched key")
		}
	})

	t.Run("paths while https disabled", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "key.pem",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err == nil {
			t.Fatal("expected ambiguous TLS paths")
		}
	})

	t.Run("mtls without https", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{"PLUTUS_SERVER_TLS_MTLS_ENABLED": "true"}),
			ReadFile:  files,
			Now:       now,
		})
		if err == nil {
			t.Fatal("expected mTLS to require HTTPS")
		}
	})

	t.Run("mtls without ca", func(t *testing.T) {
		t.Parallel()
		_, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "key.pem",
				"PLUTUS_SERVER_TLS_MTLS_ENABLED":    "true",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err == nil {
			t.Fatal("expected missing CA")
		}
	})

	t.Run("mtls valid", func(t *testing.T) {
		t.Parallel()
		cfg, err := Load(Options{
			LookupEnv: envMap(map[string]string{
				"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
				"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
				"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "key.pem",
				"PLUTUS_SERVER_TLS_MTLS_ENABLED":    "true",
				"PLUTUS_SERVER_TLS_MTLS_CA_PATH":    "ca.pem",
			}),
			ReadFile: files,
			Now:      now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.TLS.ClientCAs == nil {
			t.Fatal("expected CA pool")
		}
	})
}

func loadExampleTOML(t *testing.T, path string) Config {
	t.Helper()
	cfg, err := Load(Options{
		Path:      path,
		LookupEnv: envMap(nil),
		EnvFile:   "empty.env",
		ReadFile: func(name string) ([]byte, error) {
			if name == "empty.env" {
				return []byte{}, nil
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

	cfg := loadExampleTOML(t, "../../config/local-example.toml")
	if cfg.Server.Port != 8080 {
		t.Fatalf("unexpected example config: %+v", cfg.Server)
	}
	if cfg.Database.DSN != "" {
		t.Fatal("local example must keep the DSN out of TOML")
	}
	if !cfg.Database.AllowReset {
		t.Fatal("local example must enable db reset")
	}
	if cfg.Auth.SessionCookieName != "plutus_session_local" {
		t.Fatalf("session cookie %s", cfg.Auth.SessionCookieName)
	}
}

func TestLaneExampleTOMLLoads(t *testing.T) {
	t.Parallel()

	agent := loadExampleTOML(t, "../../config/agent-example.toml")
	if agent.Server.Port != 8081 || !agent.Database.AllowReset || agent.Auth.SessionCookieName != "plutus_session_agent" || !agent.Collection.FakeAdapter {
		t.Fatalf("agent example %+v %+v %+v", agent.Server, agent.Auth, agent.Collection)
	}
	test := loadExampleTOML(t, "../../config/test-example.toml")
	if test.Server.Port != 8082 || !test.Database.AllowReset || test.Auth.SessionCookieName != "plutus_session_test" || test.Collection.FakeAdapter {
		t.Fatalf("test example %+v %+v %+v", test.Server, test.Auth, test.Collection)
	}
	prod := loadExampleTOML(t, "../../config/prod-example.toml")
	if prod.Server.Port != 8083 || prod.Database.AllowReset || prod.Auth.SessionCookieName != "plutus_session" || prod.Collection.FakeAdapter {
		t.Fatalf("prod example %+v %+v %+v", prod.Server, prod.Auth, prod.Collection)
	}
}

func TestDSNFromTOML(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[database]
dsn = "user:pass@tcp(127.0.0.1:3306)/plutus"
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "user:pass@tcp(127.0.0.1:3306)/plutus" {
		t.Fatalf("dsn: %s", cfg.Database.DSN)
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
dsn = "user:pass@tcp(127.0.0.1:3306)/plutus"
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

func TestCookieNamesFromTOML(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[auth]
session_cookie_name = "plutus_session_agent"
csrf_cookie_name = "plutus_csrf_agent"
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.SessionCookieName != "plutus_session_agent" || cfg.Auth.CSRFCookieName != "plutus_csrf_agent" {
		t.Fatalf("cookies %s %s", cfg.Auth.SessionCookieName, cfg.Auth.CSRFCookieName)
	}
}

func TestCookieNameValidation(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[auth]
session_cookie_name = "bad name"
`,
		}),
	})
	if err == nil {
		t.Fatal("expected invalid cookie name")
	}

	_, err = Load(Options{
		Path:      "plutus.toml",
		LookupEnv: envMap(nil),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[auth]
session_cookie_name = "same"
csrf_cookie_name = "same"
`,
		}),
	})
	if err == nil {
		t.Fatal("expected cookie names to differ")
	}
}

func TestDSNEnvOverridesTOML(t *testing.T) {
	t.Parallel()

	cfg, err := Load(Options{
		Path: "plutus.toml",
		LookupEnv: envMap(map[string]string{
			envDatabaseDSN: "env-dsn@tcp(127.0.0.1:3306)/plutus",
		}),
		ReadFile: filesMap(map[string]string{
			"plutus.toml": `
[database]
dsn = "file-dsn@tcp(127.0.0.1:3306)/plutus"
`,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.DSN != "env-dsn@tcp(127.0.0.1:3306)/plutus" {
		t.Fatalf("dsn: %s", cfg.Database.DSN)
	}
}

func TestNonPositiveDurationRejected(t *testing.T) {
	t.Parallel()

	_, err := Load(Options{
		LookupEnv: envMap(map[string]string{"PLUTUS_JOBS_LEASE_DURATION": "0s"}),
		ReadFile:  filesMap(nil),
	})
	if err == nil {
		t.Fatal("expected duration error")
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
