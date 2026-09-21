package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Validate checks durations, listen settings, the TLS matrix, and registration transport rules.
func Validate(cfg *Config, opts *Options) error {
	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		return errors.New("server.listen_address is required")
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if err := durationInRange("auth.session_idle_lifetime", cfg.Auth.SessionIdleLifetime, minSessionIdle, maxSessionIdle); err != nil {
		return err
	}
	if err := durationInRange("auth.session_absolute_lifetime", cfg.Auth.SessionAbsoluteLifetime, minSessionAbsolute, maxSessionAbsolute); err != nil {
		return err
	}
	if cfg.Auth.SessionAbsoluteLifetime < cfg.Auth.SessionIdleLifetime {
		return errors.New("auth.session_absolute_lifetime must be greater than or equal to auth.session_idle_lifetime")
	}
	if err := validateCookieName("auth.session_cookie_name", cfg.Auth.SessionCookieName); err != nil {
		return err
	}
	if err := validateCookieName("auth.csrf_cookie_name", cfg.Auth.CSRFCookieName); err != nil {
		return err
	}
	if cfg.Auth.SessionCookieName == cfg.Auth.CSRFCookieName {
		return errors.New("auth.session_cookie_name and auth.csrf_cookie_name must differ")
	}
	if err := durationInRange("jobs.worker_poll_interval", cfg.Jobs.WorkerPollInterval, minWorkerPoll, maxWorkerPoll); err != nil {
		return err
	}
	if err := durationInRange("jobs.lease_duration", cfg.Jobs.LeaseDuration, minLease, maxLease); err != nil {
		return err
	}
	if err := durationInRange("jobs.completed_retention", cfg.Jobs.CompletedRetention, minCompletedRetain, maxCompletedRetain); err != nil {
		return err
	}
	if err := durationInRange("collection.default_interval", cfg.Collection.DefaultInterval, minCollectionInterval, maxCollectionInterval); err != nil {
		return err
	}
	if cfg.Collection.DefaultFreshnessMultiplier < minFreshnessMultiplier || cfg.Collection.DefaultFreshnessMultiplier > maxFreshnessMultiplier {
		return fmt.Errorf("collection.default_freshness_multiplier must be between %d and %d", minFreshnessMultiplier, maxFreshnessMultiplier)
	}
	if cfg.Auth.RegistrationEnabled && !cfg.Server.TLS.HTTPSEnabled && !isLoopback(cfg.Server.ListenAddress) {
		return errors.New("auth.registration_enabled requires HTTPS or a loopback listen address")
	}
	return validateTLS(&cfg.Server.TLS, opts)
}

func validateCookieName(field, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s is required", field)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			continue
		default:
			return fmt.Errorf("%s must be a cookie token of letters, digits, '_' or '-'", field)
		}
	}
	return nil
}

func durationInRange(name string, d, lower, upper time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("%s must be a positive duration", name)
	}
	if d < lower || d > upper {
		return fmt.Errorf("%s must be between %s and %s", name, lower, upper)
	}
	return nil
}

func isLoopback(address string) bool {
	host := strings.TrimSpace(address)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateTLS(tlsCfg *TLS, opts *Options) error {
	certSet := strings.TrimSpace(tlsCfg.HTTPSCertPath) != ""
	keySet := strings.TrimSpace(tlsCfg.HTTPSKeyPath) != ""
	caSet := strings.TrimSpace(tlsCfg.MTLSCAPath) != ""

	if !tlsCfg.HTTPSEnabled && !tlsCfg.MTLSEnabled && !certSet && !keySet && !caSet {
		return nil
	}
	if !tlsCfg.HTTPSEnabled && (certSet || keySet) {
		return errors.New("server.tls certificate paths are set while HTTPS is disabled")
	}
	if !tlsCfg.MTLSEnabled && caSet {
		return errors.New("server.tls mtls_ca_path is set while mTLS is disabled")
	}
	if tlsCfg.MTLSEnabled && !tlsCfg.HTTPSEnabled {
		return errors.New("server.tls.mtls_enabled requires HTTPS")
	}
	if tlsCfg.HTTPSEnabled && (!certSet || !keySet) {
		return errors.New("server.tls.https_enabled requires https_cert_path and https_key_path")
	}
	if tlsCfg.HTTPSEnabled {
		cert, err := loadCertificate(opts, tlsCfg.HTTPSCertPath, tlsCfg.HTTPSKeyPath)
		if err != nil {
			return err
		}
		tlsCfg.Certificate = &cert
	}
	if tlsCfg.MTLSEnabled {
		if !caSet {
			return errors.New("server.tls.mtls_enabled requires mtls_ca_path")
		}
		pool, err := loadCAPool(opts, tlsCfg.MTLSCAPath)
		if err != nil {
			return err
		}
		tlsCfg.ClientCAs = pool
	}
	return nil
}

func loadCertificate(opts *Options, certPath, keyPath string) (tls.Certificate, error) {
	certPEM, err := opts.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("read TLS certificate: %w", err)
	}
	keyPEM, err := opts.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("read TLS key: %w", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("TLS certificate and key do not match or are malformed: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return tls.Certificate{}, errors.New("TLS certificate is empty")
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse TLS certificate: %w", err)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	if now.Before(parsed.NotBefore) || now.After(parsed.NotAfter) {
		return tls.Certificate{}, errors.New("TLS certificate is expired or not yet valid")
	}
	return cert, nil
}

func loadCAPool(opts *Options, path string) (*x509.CertPool, error) {
	raw, err := opts.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mTLS CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	rest := raw
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse mTLS CA bundle: %w", err)
		}
		pool.AddCert(cert)
		found = true
	}
	if !found {
		return nil, errors.New("mTLS CA bundle contains no certificates")
	}
	return pool, nil
}
