// Package config loads and validates process configuration.
package config

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strconv"
	"time"
)

const (
	envDSN           = "PLUTUS_DSN"
	envDSNFile       = "PLUTUS_DSN_FILE"
	envMasterKey     = "PLUTUS_MASTER_KEY"
	envMasterKeyFile = "PLUTUS_MASTER_KEY_FILE"

	defaultListenAddress     = "0.0.0.0"
	defaultPort              = 8080
	defaultConfigFileName    = "config/local.toml"
	defaultEnvFileName       = "config/local.env"
	defaultGotestEnvFileName = "config/gotest.env"
	defaultRecordEnvFileName = "config/record.env"
	defaultSessionCookieName = "plutus_session"
	defaultCSRFCookieName    = "plutus_csrf"

	minSessionIdle     = time.Minute
	maxSessionIdle     = 90 * 24 * time.Hour
	minSessionAbsolute = time.Hour
	maxSessionAbsolute = 365 * 24 * time.Hour
	minCompletedRetain = 24 * time.Hour
	maxCompletedRetain = 365 * 24 * time.Hour
	minValidateTimeout = time.Second
	maxValidateTimeout = 2 * time.Minute

	masterKeySize = 32
)

// Config is the validated runtime configuration.
type Config struct {
	Server     Server
	Auth       Auth
	Jobs       Jobs
	Collection Collection
	Database   Database
	Crypto     Crypto
	Logging    Logging
}

// Logging holds process log output and rotation policies. Size is measured in MiB.
type Logging struct {
	Directory  string
	Level      string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	SQLEnabled bool
}

// Server holds HTTP listen and TLS settings.
type Server struct {
	ListenAddress string
	Port          int
	TLS           TLS
}

// Addr returns host:port.
func (s *Server) Addr() string {
	return net.JoinHostPort(s.ListenAddress, strconv.Itoa(s.Port))
}

// TLS holds validated transport security settings.
type TLS struct {
	CertFile     string
	KeyFile      string
	ClientCAFile string
	Certificate  *tls.Certificate
	ClientCAs    *x509.CertPool
}

// HTTPSEnabled reports whether a server certificate has been loaded.
func (t *TLS) HTTPSEnabled() bool {
	return t.Certificate != nil
}

// MTLSEnabled reports whether client certificate verification has been configured.
func (t *TLS) MTLSEnabled() bool {
	return t.ClientCAs != nil
}

// Auth holds session lifetimes and the shared cookie suffix.
type Auth struct {
	SessionIdleLifetime     time.Duration
	SessionAbsoluteLifetime time.Duration
	CookieSuffix            string
}

// SessionCookieName returns the instance's session cookie name.
func (a *Auth) SessionCookieName() string {
	return a.cookieName(defaultSessionCookieName)
}

// CSRFCookieName returns the instance's CSRF cookie name.
func (a *Auth) CSRFCookieName() string {
	return a.cookieName(defaultCSRFCookieName)
}

func (a *Auth) cookieName(base string) string {
	if a.CookieSuffix == "" {
		return base
	}
	return base + "_" + a.CookieSuffix
}

// Jobs holds the retention policy for finished jobs without reports or points.
type Jobs struct {
	FinishedRetention time.Duration
}

// Collection holds Connection validation settings and the adapter selection.
type Collection struct {
	// ConnectionValidationTimeout bounds one synchronous Connection validation call.
	ConnectionValidationTimeout time.Duration
	// FakeAdapter registers the deterministic offline adapter for every Platform.
	FakeAdapter bool
}

// Database holds the MySQL DSN and development-reset permission. The DSN must never be logged.
type Database struct {
	DSN        string
	AllowReset bool
}

// Crypto holds the optional AES-256 master key. The value must never be logged.
type Crypto struct {
	MasterKey []byte
}

// HasMasterKey reports whether a 32-byte master key is configured.
func (c Crypto) HasMasterKey() bool {
	return len(c.MasterKey) == masterKeySize
}
