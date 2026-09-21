package config

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func applySecrets(cfg *Config, opts *Options) error {
	dsn, err := exclusiveSecret(opts, envDatabaseDSN, envDatabaseDSNFile, "database DSN")
	if err != nil {
		return err
	}
	if dsn != "" {
		cfg.Database.DSN = dsn
	}

	keyRaw, err := exclusiveSecret(opts, envMasterKey, envMasterKeyFile, "master key")
	if err != nil {
		return err
	}
	if keyRaw == "" {
		return nil
	}
	key, err := decodeMasterKey(keyRaw)
	if err != nil {
		return err
	}
	cfg.Crypto.MasterKey = key
	return nil
}

func exclusiveSecret(opts *Options, envKey, fileEnvKey, label string) (string, error) {
	direct, directOK := opts.LookupEnv(envKey)
	filePath, fileOK := opts.LookupEnv(fileEnvKey)
	if directOK && strings.TrimSpace(direct) != "" && fileOK && strings.TrimSpace(filePath) != "" {
		return "", fmt.Errorf("%s must come from exactly one of %s or %s", label, envKey, fileEnvKey)
	}
	if fileOK {
		path := strings.TrimSpace(filePath)
		if path == "" {
			return "", fmt.Errorf("%s is empty", fileEnvKey)
		}
		raw, err := opts.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", label, err)
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			return "", fmt.Errorf("%s file %s is empty", label, path)
		}
		return value, nil
	}
	if directOK {
		value := strings.TrimSpace(direct)
		if value == "" {
			return "", fmt.Errorf("%s is empty", envKey)
		}
		return value, nil
	}
	return "", nil
}

func decodeMasterKey(raw string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("master key must be base64: %w", err)
		}
	}
	if len(key) != masterKeySize {
		return nil, fmt.Errorf("master key must decode to %d bytes", masterKeySize)
	}
	return key, nil
}
