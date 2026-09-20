package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Options controls how configuration is loaded.
type Options struct {
	Path      string
	LookupEnv func(string) (string, bool)
	ReadFile  func(string) ([]byte, error)
	Now       time.Time
}

type fileConfig struct {
	Server struct {
		ListenAddress string `toml:"listen_address"`
		Port          int    `toml:"port"`
		TLS           struct {
			HTTPSEnabled  bool   `toml:"https_enabled"`
			HTTPSCertPath string `toml:"https_cert_path"`
			HTTPSKeyPath  string `toml:"https_key_path"`
			MTLSEnabled   bool   `toml:"mtls_enabled"`
			MTLSCAPath    string `toml:"mtls_ca_path"`
		} `toml:"tls"`
	} `toml:"server"`
	Auth struct {
		RegistrationEnabled     bool   `toml:"registration_enabled"`
		SessionIdleLifetime     string `toml:"session_idle_lifetime"`
		SessionAbsoluteLifetime string `toml:"session_absolute_lifetime"`
	} `toml:"auth"`
	Jobs struct {
		WorkerPollInterval string `toml:"worker_poll_interval"`
		LeaseDuration      string `toml:"lease_duration"`
		CompletedRetention string `toml:"completed_retention"`
	} `toml:"jobs"`
	Collection struct {
		DefaultInterval            string `toml:"default_interval"`
		DefaultFreshnessMultiplier int    `toml:"default_freshness_multiplier"`
	} `toml:"collection"`
	Database struct {
		DSN        string `toml:"dsn"`
		AllowReset bool   `toml:"allow_reset"`
	} `toml:"database"`
}

// Load reads defaults, an optional TOML file, environment overrides, and secrets.
func Load(opts Options) (Config, error) {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	cfg := defaults()
	if err := applyFile(&cfg, opts); err != nil {
		return Config{}, err
	}
	if err := applyEnv(&cfg, opts.LookupEnv); err != nil {
		return Config{}, err
	}
	if err := applySecrets(&cfg, opts); err != nil {
		return Config{}, err
	}
	if err := Validate(&cfg, opts); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaults() Config {
	return Config{
		Server: Server{
			ListenAddress: defaultListenAddress,
			Port:          defaultPort,
		},
		Auth: Auth{
			RegistrationEnabled:     false,
			SessionIdleLifetime:     7 * 24 * time.Hour,
			SessionAbsoluteLifetime: 30 * 24 * time.Hour,
		},
		Jobs: Jobs{
			WorkerPollInterval: time.Second,
			LeaseDuration:      2 * time.Minute,
			CompletedRetention: 90 * 24 * time.Hour,
		},
		Collection: Collection{
			DefaultInterval:            15 * time.Minute,
			DefaultFreshnessMultiplier: 2,
		},
	}
}

func applyFile(cfg *Config, opts Options) error {
	path := opts.Path
	required := path != ""
	if path == "" {
		path = "plutus.toml"
	}
	raw, err := opts.ReadFile(path)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config file %s: %w", path, err)
	}
	var parsed fileConfig
	if err := toml.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	return overlayFile(cfg, &parsed)
}

func overlayFile(cfg *Config, parsed *fileConfig) error {
	if parsed.Server.ListenAddress != "" {
		cfg.Server.ListenAddress = parsed.Server.ListenAddress
	}
	if parsed.Server.Port != 0 {
		cfg.Server.Port = parsed.Server.Port
	}
	cfg.Server.TLS.HTTPSEnabled = parsed.Server.TLS.HTTPSEnabled
	cfg.Server.TLS.HTTPSCertPath = parsed.Server.TLS.HTTPSCertPath
	cfg.Server.TLS.HTTPSKeyPath = parsed.Server.TLS.HTTPSKeyPath
	cfg.Server.TLS.MTLSEnabled = parsed.Server.TLS.MTLSEnabled
	cfg.Server.TLS.MTLSCAPath = parsed.Server.TLS.MTLSCAPath

	cfg.Auth.RegistrationEnabled = parsed.Auth.RegistrationEnabled
	if err := optionalDuration(&cfg.Auth.SessionIdleLifetime, parsed.Auth.SessionIdleLifetime, "auth.session_idle_lifetime"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Auth.SessionAbsoluteLifetime, parsed.Auth.SessionAbsoluteLifetime, "auth.session_absolute_lifetime"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Jobs.WorkerPollInterval, parsed.Jobs.WorkerPollInterval, "jobs.worker_poll_interval"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Jobs.LeaseDuration, parsed.Jobs.LeaseDuration, "jobs.lease_duration"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Jobs.CompletedRetention, parsed.Jobs.CompletedRetention, "jobs.completed_retention"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Collection.DefaultInterval, parsed.Collection.DefaultInterval, "collection.default_interval"); err != nil {
		return err
	}
	if parsed.Collection.DefaultFreshnessMultiplier != 0 {
		cfg.Collection.DefaultFreshnessMultiplier = parsed.Collection.DefaultFreshnessMultiplier
	}
	if strings.TrimSpace(parsed.Database.DSN) != "" {
		cfg.Database.DSN = strings.TrimSpace(parsed.Database.DSN)
	}
	cfg.Database.AllowReset = parsed.Database.AllowReset
	return nil
}

func applyEnv(cfg *Config, lookup func(string) (string, bool)) error {
	if err := optionalString(&cfg.Server.ListenAddress, lookup, "PLUTUS_SERVER_LISTEN_ADDRESS"); err != nil {
		return err
	}
	if v, ok := lookup("PLUTUS_SERVER_PORT"); ok {
		port, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("PLUTUS_SERVER_PORT: %w", err)
		}
		cfg.Server.Port = port
	}
	if err := optionalBool(&cfg.Server.TLS.HTTPSEnabled, lookup, "PLUTUS_SERVER_TLS_HTTPS_ENABLED"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Server.TLS.HTTPSCertPath, lookup, "PLUTUS_SERVER_TLS_HTTPS_CERT_PATH"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Server.TLS.HTTPSKeyPath, lookup, "PLUTUS_SERVER_TLS_HTTPS_KEY_PATH"); err != nil {
		return err
	}
	if err := optionalBool(&cfg.Server.TLS.MTLSEnabled, lookup, "PLUTUS_SERVER_TLS_MTLS_ENABLED"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Server.TLS.MTLSCAPath, lookup, "PLUTUS_SERVER_TLS_MTLS_CA_PATH"); err != nil {
		return err
	}
	if err := optionalBool(&cfg.Auth.RegistrationEnabled, lookup, "PLUTUS_AUTH_REGISTRATION_ENABLED"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Auth.SessionIdleLifetime, lookup, "PLUTUS_AUTH_SESSION_IDLE_LIFETIME"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Auth.SessionAbsoluteLifetime, lookup, "PLUTUS_AUTH_SESSION_ABSOLUTE_LIFETIME"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Jobs.WorkerPollInterval, lookup, "PLUTUS_JOBS_WORKER_POLL_INTERVAL"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Jobs.LeaseDuration, lookup, "PLUTUS_JOBS_LEASE_DURATION"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Jobs.CompletedRetention, lookup, "PLUTUS_JOBS_COMPLETED_RETENTION"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Collection.DefaultInterval, lookup, "PLUTUS_COLLECTION_DEFAULT_INTERVAL"); err != nil {
		return err
	}
	if v, ok := lookup("PLUTUS_COLLECTION_DEFAULT_FRESHNESS_MULTIPLIER"); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("PLUTUS_COLLECTION_DEFAULT_FRESHNESS_MULTIPLIER: %w", err)
		}
		cfg.Collection.DefaultFreshnessMultiplier = n
	}
	if err := optionalBool(&cfg.Database.AllowReset, lookup, "PLUTUS_DATABASE_ALLOW_RESET"); err != nil {
		return err
	}
	return nil
}

func optionalString(dst *string, lookup func(string) (string, bool), key string) error {
	v, ok := lookup(key)
	if !ok {
		return nil
	}
	*dst = strings.TrimSpace(v)
	return nil
}

func optionalBool(dst *bool, lookup func(string) (string, bool), key string) error {
	v, ok := lookup(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*dst = parsed
	return nil
}

func optionalDuration(dst *time.Duration, raw, name string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	*dst = d
	return nil
}

func optionalDurationEnv(dst *time.Duration, lookup func(string) (string, bool), key string) error {
	v, ok := lookup(key)
	if !ok {
		return nil
	}
	return optionalDuration(dst, v, key)
}
