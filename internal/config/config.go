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
	envDatabaseDSN     = "PLUTUS_DATABASE_DSN"
	envDatabaseDSNFile = "PLUTUS_DATABASE_DSN_FILE"
	envMasterKey       = "PLUTUS_MASTER_KEY"
	envMasterKeyFile   = "PLUTUS_MASTER_KEY_FILE"

	defaultListenAddress = "127.0.0.1"
	defaultPort          = 8080

	minSessionIdle         = time.Minute
	maxSessionIdle         = 90 * 24 * time.Hour
	minSessionAbsolute     = time.Hour
	maxSessionAbsolute     = 365 * 24 * time.Hour
	minWorkerPoll          = 100 * time.Millisecond
	maxWorkerPoll          = time.Minute
	minLease               = 5 * time.Second
	maxLease               = 30 * time.Minute
	minCompletedRetain     = 24 * time.Hour
	maxCompletedRetain     = 365 * 24 * time.Hour
	minCollectionInterval  = time.Minute
	maxCollectionInterval  = 24 * time.Hour
	minFreshnessMultiplier = 1
	maxFreshnessMultiplier = 100

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
	HTTPSEnabled  bool
	HTTPSCertPath string
	HTTPSKeyPath  string
	MTLSEnabled   bool
	MTLSCAPath    string
	Certificate   *tls.Certificate
	ClientCAs     *x509.CertPool
}

// Auth holds registration and session lifetime settings.
type Auth struct {
	RegistrationEnabled     bool
	SessionIdleLifetime     time.Duration
	SessionAbsoluteLifetime time.Duration
}

// Jobs holds durable worker settings.
type Jobs struct {
	WorkerPollInterval time.Duration
	LeaseDuration      time.Duration
	CompletedRetention time.Duration
}

// Collection holds default collection freshness settings.
type Collection struct {
	DefaultInterval            time.Duration
	DefaultFreshnessMultiplier int
}

// Database holds the MySQL DSN. The value must never be logged.
type Database struct {
	DSN string
}

// Crypto holds the optional AES-256 master key. The value must never be logged.
type Crypto struct {
	MasterKey []byte
}

// HasMasterKey reports whether a 32-byte master key is configured.
func (c Crypto) HasMasterKey() bool {
	return len(c.MasterKey) == masterKeySize
}
