package adapters

import (
	"strings"
)

// Credentials is the decrypted read-only API key material passed to an adapter call.
type Credentials struct {
	APIKey     string
	APISecret  string
	Passphrase string
	Futures    *KeyPair
}

// KeyPair is an optional additional API credential group.
type KeyPair struct {
	APIKey    string
	APISecret string
}

// FuturesKey returns the configured futures key, or empty when that group is absent.
func (c Credentials) FuturesKey() string {
	if c.Futures == nil {
		return ""
	}
	return c.Futures.APIKey
}

// FuturesSecret returns the configured futures secret, or empty when that group is absent.
func (c Credentials) FuturesSecret() string {
	if c.Futures == nil {
		return ""
	}
	return c.Futures.APISecret
}

// CheckCredentials enforces complete key pairs and the selected accounts' requirements.
func CheckCredentials(platform Platform, structure AccountStructure, accounts []AccountType, creds Credentials) error {
	spotKey, spotSecret := strings.TrimSpace(creds.APIKey), strings.TrimSpace(creds.APISecret)
	futuresKey, futuresSecret := strings.TrimSpace(creds.FuturesKey()), strings.TrimSpace(creds.FuturesSecret())
	if (spotKey == "") != (spotSecret == "") || (futuresKey == "") != (futuresSecret == "") {
		return NewError(ClassAuthentication, "Supply both the API key and secret for each credential group.")
	}
	if platform != PlatformKraken {
		if spotKey == "" || futuresKey != "" {
			return NewError(ClassAuthentication, "This platform requires one API key and secret.")
		}
		if platform == PlatformBinance {
			if _, _, err := BinancePrivateKey(spotSecret); err != nil {
				return err
			}
		}
		return nil
	}
	if !ValidStructure(platform, structure) {
		return NewError(ClassUnsupported, "The Kraken account structure is not supported.")
	}
	if spotKey == "" && futuresKey == "" {
		return NewError(ClassAuthentication, "Supply at least one Kraken credential group.")
	}
	for _, account := range accounts {
		if (account == "main" || account == "earn") && spotKey == "" {
			return NewError(ClassAuthentication, "Selected main and earn accounts require Kraken spot credentials.")
		}
		if account != "earn" && futuresKey == "" {
			return NewError(ClassAuthentication, "Selected main and derivatives accounts require Kraken futures credentials; main uses them to verify the wallet structure.")
		}
	}
	return nil
}
