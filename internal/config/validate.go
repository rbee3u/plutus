package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Validate checks durations, listen settings, and the TLS matrix.
func Validate(cfg *Config, opts *Options) error {
	if err := validateProxy(&cfg.Collection.Proxy, "collection.proxy"); err != nil {
		return err
	}
	if err := validateLogging(&cfg.Logging); err != nil {
		return err
	}
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
	if err := validateCookieSuffix(cfg.Auth.CookieSuffix); err != nil {
		return err
	}
	if err := durationInRange("jobs.finished_retention", cfg.Jobs.FinishedRetention, minCompletedRetain, maxCompletedRetain); err != nil {
		return err
	}
	if err := durationInRange("collection.connection_validation_timeout", cfg.Collection.ConnectionValidationTimeout, minValidateTimeout, maxValidateTimeout); err != nil {
		return err
	}
	return validateTLS(&cfg.Server.TLS, opts)
}

func validateLogging(cfg *Logging) error {
	if strings.TrimSpace(cfg.Directory) == "" {
		return errors.New("logging.directory is required")
	}
	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging.level must be debug, info, warn, or error")
	}
	if cfg.MaxSizeMB <= 0 || cfg.MaxSizeMB > int(^uint(0)>>1)/(1024*1024) {
		return errors.New("logging.max_size_mb must be positive and fit in bytes")
	}
	if cfg.MaxBackups < 0 {
		return errors.New("logging.max_backups must be nonnegative")
	}
	if cfg.MaxAgeDays < 0 || int64(cfg.MaxAgeDays) > int64(^uint64(0)>>1)/int64(24*time.Hour) {
		return errors.New("logging.max_age_days must be nonnegative and fit in a duration")
	}
	return nil
}

func validateCookieSuffix(suffix string) error {
	for _, r := range suffix {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			continue
		default:
			return errors.New("auth.cookie_suffix must contain only letters, digits, '_' or '-'")
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

func validateTLS(tlsCfg *TLS, opts *Options) error {
	certSet := tlsCfg.CertFile != ""
	keySet := tlsCfg.KeyFile != ""
	caSet := tlsCfg.ClientCAFile != ""

	if certSet != keySet {
		return errors.New("server.tls.cert_file and key_file must be configured together")
	}
	if caSet && !certSet {
		return errors.New("server.tls.client_ca_file requires cert_file and key_file")
	}
	if certSet {
		cert, err := loadCertificate(opts, tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return err
		}
		tlsCfg.Certificate = &cert
	}
	if caSet {
		pool, err := loadCAPool(opts, tlsCfg.ClientCAFile)
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
