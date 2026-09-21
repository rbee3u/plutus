package httpserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rbee3u/plutus/internal/config"
)

func TestDefaultHTTPServesHealthAPIAndShell(t *testing.T) {
	t.Parallel()

	engine := Engine(&EngineOptions{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:  StaticReady{},
	})
	ts := httptest.NewServer(engine)
	t.Cleanup(ts.Close)

	for _, path := range []string{"/health/live", "/login", "/api/v1/auth/csrf"} {
		res, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d", path, res.StatusCode)
		}
	}
}

func TestHTTPSNegotiatesTLS13AndMTLSGates(t *testing.T) {
	t.Parallel()

	now := time.Now()
	caPEM, caKeyPEM := mustCertificate(t, nil, nil, now.Add(-time.Hour), now.Add(24*time.Hour), true, false)
	serverPEM, serverKeyPEM := mustCertificate(t, caPEM, caKeyPEM, now.Add(-time.Hour), now.Add(24*time.Hour), false, false)
	clientPEM, clientKeyPEM := mustCertificate(t, caPEM, caKeyPEM, now.Add(-time.Hour), now.Add(24*time.Hour), false, true)
	expiredPEM, expiredKeyPEM := mustCertificate(t, caPEM, caKeyPEM, now.Add(-48*time.Hour), now.Add(-time.Hour), false, true)
	otherCAPEM, otherCAKeyPEM := mustCertificate(t, nil, nil, now.Add(-time.Hour), now.Add(24*time.Hour), true, false)
	untrustedPEM, untrustedKeyPEM := mustCertificate(t, otherCAPEM, otherCAKeyPEM, now.Add(-time.Hour), now.Add(24*time.Hour), false, true)

	cfg, err := config.Load(config.Options{
		LookupEnv: envMap(map[string]string{
			"PLUTUS_SERVER_TLS_HTTPS_ENABLED":   "true",
			"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH": "cert.pem",
			"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH":  "key.pem",
			"PLUTUS_SERVER_TLS_MTLS_ENABLED":    "true",
			"PLUTUS_SERVER_TLS_MTLS_CA_PATH":    "ca.pem",
		}),
		ReadFile: filesMap(map[string]string{
			"cert.pem": string(serverPEM),
			"key.pem":  string(serverKeyPEM),
			"ca.pem":   string(caPEM),
		}),
		Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	engine := Engine(&EngineOptions{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ready:  StaticReady{},
	})
	ts := httptest.NewUnstartedServer(engine)
	ts.TLS = tlsServerConfig(&cfg)
	ts.StartTLS()
	t.Cleanup(ts.Close)

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("ca")
	}
	host := ts.Listener.Addr().String()

	tls12, err := tls.Dial("tcp", host, &tls.Config{
		RootCAs:            roots,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
		ServerName:         "127.0.0.1",
	})
	if err == nil {
		_ = tls12.Close()
		t.Fatal("TLS 1.2 should be rejected")
	}

	conn, err := tls.Dial("tcp", host, &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS13,
		ServerName: "127.0.0.1",
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := tls.X509KeyPair(clientPEM, clientKeyPEM)
			if err != nil {
				return nil, err
			}
			return &cert, nil
		},
	})
	if err != nil {
		t.Fatalf("tls 1.3 dial: %v", err)
	}
	if conn.ConnectionState().Version != tls.VersionTLS13 {
		t.Fatalf("negotiated 0x%x", conn.ConnectionState().Version)
	}
	_ = conn.Close()

	validClient, err := tls.X509KeyPair(clientPEM, clientKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	expiredClient, err := tls.X509KeyPair(expiredPEM, expiredKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	untrustedClient, err := tls.X509KeyPair(untrustedPEM, untrustedKeyPEM)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{"/health/live", "/login", "/api/v1/auth/csrf"}
	for _, path := range paths {
		status, err := tlsRequest(ts.URL+path, &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS13,
			ServerName: "127.0.0.1",
			Certificates: []tls.Certificate{
				validClient,
			},
		})
		if err != nil {
			t.Fatalf("valid mTLS %s: %v", path, err)
		}
		if status != http.StatusOK {
			t.Fatalf("valid mTLS %s status %d", path, status)
		}

		if _, err := tlsRequest(ts.URL+path, &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS13,
			ServerName: "127.0.0.1",
		}); err == nil {
			t.Fatalf("missing client cert should fail %s", path)
		}
		if _, err := tlsRequest(ts.URL+path, &tls.Config{
			RootCAs:      roots,
			MinVersion:   tls.VersionTLS13,
			ServerName:   "127.0.0.1",
			Certificates: []tls.Certificate{expiredClient},
		}); err == nil {
			t.Fatalf("expired client cert should fail %s", path)
		}
		if _, err := tlsRequest(ts.URL+path, &tls.Config{
			RootCAs:      roots,
			MinVersion:   tls.VersionTLS13,
			ServerName:   "127.0.0.1",
			Certificates: []tls.Certificate{untrustedClient},
		}); err == nil {
			t.Fatalf("untrusted client cert should fail %s", path)
		}
	}
}

func tlsRequest(url string, tlsCfg *tls.Config) (int, error) {
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
	res, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	_ = res.Body.Close()
	return res.StatusCode, nil
}

func envMap(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := values[key]
		return v, ok
	}
}

func filesMap(values map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		return []byte(values[path]), nil
	}
}

func mustCertificate(t *testing.T, parentPEM, parentKeyPEM []byte, notBefore, notAfter time.Time, isCA, client bool) ([]byte, []byte) {
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
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost", "127.0.0.1"},
	}
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		template.ExtKeyUsage = nil
	} else if client {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}

	parent := template
	parentKey := key
	if len(parentPEM) > 0 {
		block, _ := pem.Decode(parentPEM)
		parsed, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		parent = parsed
		keyBlock, _ := pem.Decode(parentKeyPEM)
		parsedKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		parentKey = parsedKey
	}

	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
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
