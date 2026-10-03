package config

import (
	"bytes"
	"errors"
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

	getwd func() (string, error)

	// walkEnvFile, when EnvFile is empty, is the relative path walked from cwd to the module root.
	// Empty means config/local.env. Lookup and LookupRecording select their own files.
	walkEnvFile string

	// EnvFile, when set, is the only env file loaded. The nearest walked env file is skipped.
	EnvFile string
}

type fileConfig struct {
	Logging struct {
		Directory  *string `toml:"directory"`
		Level      *string `toml:"level"`
		MaxSizeMB  *int    `toml:"max_size_mb"`
		MaxBackups *int    `toml:"max_backups"`
		MaxAgeDays *int    `toml:"max_age_days"`
		Compress   *bool   `toml:"compress"`
		SQLEnabled *bool   `toml:"sql_enabled"`
	} `toml:"logging"`
	Server struct {
		ListenAddress *string `toml:"listen_address"`
		Port          *int    `toml:"port"`
		TLS           struct {
			CertFile     string `toml:"cert_file"`
			KeyFile      string `toml:"key_file"`
			ClientCAFile string `toml:"client_ca_file"`
		} `toml:"tls"`
	} `toml:"server"`
	Auth struct {
		SessionIdleLifetime     *string `toml:"session_idle_lifetime"`
		SessionAbsoluteLifetime *string `toml:"session_absolute_lifetime"`
		CookieSuffix            string  `toml:"cookie_suffix"`
	} `toml:"auth"`
	Jobs struct {
		FinishedRetention *string `toml:"finished_retention"`
	} `toml:"jobs"`
	Collection struct {
		ConnectionValidationTimeout *string `toml:"connection_validation_timeout"`
		FakeAdapter                 bool    `toml:"fake_adapter"`
		Proxy                       struct {
			Mode *string `toml:"mode"`
			URL  *string `toml:"url"`
		} `toml:"proxy"`
	} `toml:"collection"`
	Database struct {
		AllowReset bool `toml:"allow_reset"`
	} `toml:"database"`
}

// Load reads defaults, an env file, an optional TOML file, environment overrides, and secrets.
// When Options.EnvFile is set, only that file is read. Otherwise an optional `config/local.env`
// in the working directory or Go module root is used. Existing process environment values win
// over the env file. A missing default env file is ignored. The nearest `config/local.env` wins
// when walking. `go test` reads `PLUTUS_GOTEST_*` from `config/gotest.env` through Lookup instead.
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
	if err := applyDotEnv(&opts); err != nil {
		return Config{}, err
	}

	cfg := defaults()
	if err := applyFile(&cfg, &opts); err != nil {
		return Config{}, err
	}
	if err := applyEnv(&cfg, opts.LookupEnv); err != nil {
		return Config{}, err
	}
	if err := applySecrets(&cfg, &opts); err != nil {
		return Config{}, err
	}
	if err := Validate(&cfg, &opts); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaults() Config {
	return Config{
		Logging: Logging{Directory: "./logs", Level: "info", MaxSizeMB: 100, MaxBackups: 5, Compress: true, SQLEnabled: true},
		Server: Server{
			ListenAddress: defaultListenAddress,
			Port:          defaultPort,
		},
		Auth: Auth{
			SessionIdleLifetime:     7 * 24 * time.Hour,
			SessionAbsoluteLifetime: 30 * 24 * time.Hour,
		},
		Jobs: Jobs{
			FinishedRetention: 90 * 24 * time.Hour,
		},
		Collection: Collection{
			ConnectionValidationTimeout: 15 * time.Second,
			Proxy:                       Proxy{Mode: "environment"},
		},
	}
}

func applyFile(cfg *Config, opts *Options) error {
	path := opts.Path
	required := path != ""
	if path == "" {
		path = defaultConfigFileName
	}
	raw, err := opts.ReadFile(path)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config file %s: %w", path, err)
	}
	var parsed fileConfig
	decoder := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		var unknown *toml.StrictMissingError
		if errors.As(err, &unknown) {
			keys := make([]string, len(unknown.Errors))
			for i := range unknown.Errors {
				keys[i] = strings.Join(unknown.Errors[i].Key(), ".")
			}
			return fmt.Errorf("config file %s contains unsupported fields: %s; see docs/development.md#configuration-migration", path, strings.Join(keys, ", "))
		}
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	return overlayFile(cfg, &parsed)
}

func overlayFile(cfg *Config, parsed *fileConfig) error {
	if parsed.Server.ListenAddress != nil {
		cfg.Server.ListenAddress = strings.TrimSpace(*parsed.Server.ListenAddress)
	}
	if parsed.Server.Port != nil {
		cfg.Server.Port = *parsed.Server.Port
	}
	cfg.Server.TLS.CertFile = strings.TrimSpace(parsed.Server.TLS.CertFile)
	cfg.Server.TLS.KeyFile = strings.TrimSpace(parsed.Server.TLS.KeyFile)
	cfg.Server.TLS.ClientCAFile = strings.TrimSpace(parsed.Server.TLS.ClientCAFile)
	cfg.Auth.CookieSuffix = strings.TrimSpace(parsed.Auth.CookieSuffix)

	if err := optionalDuration(&cfg.Auth.SessionIdleLifetime, parsed.Auth.SessionIdleLifetime, "auth.session_idle_lifetime"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Auth.SessionAbsoluteLifetime, parsed.Auth.SessionAbsoluteLifetime, "auth.session_absolute_lifetime"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Jobs.FinishedRetention, parsed.Jobs.FinishedRetention, "jobs.finished_retention"); err != nil {
		return err
	}
	if err := optionalDuration(&cfg.Collection.ConnectionValidationTimeout, parsed.Collection.ConnectionValidationTimeout, "collection.connection_validation_timeout"); err != nil {
		return err
	}
	cfg.Collection.FakeAdapter = parsed.Collection.FakeAdapter
	if parsed.Collection.Proxy.Mode != nil {
		cfg.Collection.Proxy.Mode = strings.TrimSpace(*parsed.Collection.Proxy.Mode)
	}
	if parsed.Collection.Proxy.URL != nil {
		cfg.Collection.Proxy.URL = strings.TrimSpace(*parsed.Collection.Proxy.URL)
	}
	cfg.Database.AllowReset = parsed.Database.AllowReset
	if parsed.Logging.Directory != nil {
		cfg.Logging.Directory = strings.TrimSpace(*parsed.Logging.Directory)
	}
	if parsed.Logging.Level != nil {
		cfg.Logging.Level = strings.TrimSpace(*parsed.Logging.Level)
	}
	if parsed.Logging.MaxSizeMB != nil {
		cfg.Logging.MaxSizeMB = *parsed.Logging.MaxSizeMB
	}
	if parsed.Logging.MaxBackups != nil {
		cfg.Logging.MaxBackups = *parsed.Logging.MaxBackups
	}
	if parsed.Logging.MaxAgeDays != nil {
		cfg.Logging.MaxAgeDays = *parsed.Logging.MaxAgeDays
	}
	if parsed.Logging.Compress != nil {
		cfg.Logging.Compress = *parsed.Logging.Compress
	}
	if parsed.Logging.SQLEnabled != nil {
		cfg.Logging.SQLEnabled = *parsed.Logging.SQLEnabled
	}
	return nil
}

func applyEnv(cfg *Config, lookup func(string) (string, bool)) error {
	if err := rejectRemovedEnv(lookup); err != nil {
		return err
	}
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
	if err := optionalString(&cfg.Server.TLS.CertFile, lookup, "PLUTUS_SERVER_TLS_CERT_FILE"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Server.TLS.KeyFile, lookup, "PLUTUS_SERVER_TLS_KEY_FILE"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Server.TLS.ClientCAFile, lookup, "PLUTUS_SERVER_TLS_CLIENT_CA_FILE"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Auth.CookieSuffix, lookup, "PLUTUS_AUTH_COOKIE_SUFFIX"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Auth.SessionIdleLifetime, lookup, "PLUTUS_AUTH_SESSION_IDLE_LIFETIME"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Auth.SessionAbsoluteLifetime, lookup, "PLUTUS_AUTH_SESSION_ABSOLUTE_LIFETIME"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Jobs.FinishedRetention, lookup, "PLUTUS_JOBS_FINISHED_RETENTION"); err != nil {
		return err
	}
	if err := optionalDurationEnv(&cfg.Collection.ConnectionValidationTimeout, lookup, "PLUTUS_COLLECTION_CONNECTION_VALIDATION_TIMEOUT"); err != nil {
		return err
	}
	if err := optionalBool(&cfg.Collection.FakeAdapter, lookup, "PLUTUS_COLLECTION_FAKE_ADAPTER"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Collection.Proxy.Mode, lookup, "PLUTUS_COLLECTION_PROXY_MODE"); err != nil {
		return err
	}
	if err := optionalString(&cfg.Collection.Proxy.URL, lookup, "PLUTUS_COLLECTION_PROXY_URL"); err != nil {
		return err
	}
	if err := optionalBool(&cfg.Database.AllowReset, lookup, "PLUTUS_DATABASE_ALLOW_RESET"); err != nil {
		return err
	}
	return nil
}

// Removed environment names must fail instead of silently selecting defaults.
func rejectRemovedEnv(lookup func(string) (string, bool)) error {
	removed := []struct {
		key         string
		instruction string
	}{
		{"PLUTUS_SERVER_TLS_HTTPS_ENABLED", "HTTPS is derived from cert_file and key_file"},
		{"PLUTUS_SERVER_TLS_HTTPS_CERT_PATH", "use PLUTUS_SERVER_TLS_CERT_FILE"},
		{"PLUTUS_SERVER_TLS_HTTPS_KEY_PATH", "use PLUTUS_SERVER_TLS_KEY_FILE"},
		{"PLUTUS_SERVER_TLS_MTLS_ENABLED", "mTLS is derived from client_ca_file"},
		{"PLUTUS_SERVER_TLS_MTLS_CA_PATH", "use PLUTUS_SERVER_TLS_CLIENT_CA_FILE"},
		{"PLUTUS_AUTH_SESSION_COOKIE_NAME", "use PLUTUS_AUTH_COOKIE_SUFFIX"},
		{"PLUTUS_AUTH_CSRF_COOKIE_NAME", "use PLUTUS_AUTH_COOKIE_SUFFIX"},
		{"PLUTUS_JOBS_WORKER_POLL_INTERVAL", "worker polling is fixed at 1s"},
		{"PLUTUS_JOBS_LEASE_DURATION", "the worker lease is fixed at 2m"},
		{"PLUTUS_JOBS_COMPLETED_RETENTION", "use PLUTUS_JOBS_FINISHED_RETENTION"},
		{"PLUTUS_COLLECTION_DEFAULT_INTERVAL", "new Portfolios default to 15m; edit each Portfolio's interval"},
		{"PLUTUS_COLLECTION_STAGGER", "the delay between Identities is fixed at 3s"},
		{"PLUTUS_COLLECTION_VALIDATE_TIMEOUT", "use PLUTUS_COLLECTION_CONNECTION_VALIDATION_TIMEOUT"},
	}
	for _, field := range removed {
		if _, ok := lookup(field.key); ok {
			return fmt.Errorf("%s is no longer supported: %s", field.key, field.instruction)
		}
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

func optionalDuration(dst *time.Duration, raw *string, name string) error {
	if raw == nil {
		return nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(*raw))
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
	return optionalDuration(dst, &v, key)
}
