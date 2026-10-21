package adapter

// CredentialForm describes existing browser inputs, not server validation or
// credential storage. Exchange-specific semantic validation remains explicit.
type CredentialForm struct {
	Primary    CredentialGroup
	Futures    *CredentialGroup
	Passphrase string
	Help       string
}

// CredentialGroup describes one of the two fixed key-pair inputs.
type CredentialGroup struct {
	KeyLabel    string
	SecretLabel string
	SecretInput string
	Requirement CredentialRequirement
}

// CredentialRequirement supports only the current account selection predicates.
// selected_except intentionally includes selected application-owned accounts;
// form requirements and server validation are separate policies.
type CredentialRequirement struct {
	Mode         string
	AccountTypes []AccountType
}

func primaryCredentialGroup() CredentialGroup {
	return CredentialGroup{KeyLabel: "API key", SecretLabel: "API secret", SecretInput: "password", Requirement: CredentialRequirement{Mode: "always"}}
}

func binanceCredentialForm() CredentialForm {
	primary := primaryCredentialGroup()
	primary.SecretLabel = "API secret or Ed25519 private key"
	primary.SecretInput = "multiline"
	return CredentialForm{Primary: primary, Passphrase: "hidden", Help: "Paste an unencrypted PKCS#8 Ed25519 private key PEM, including its BEGIN and END lines. Plutus detects it automatically; other values use HMAC. The private key is visible while you enter it and is encrypted when stored."}
}

func okxCredentialForm() CredentialForm {
	return CredentialForm{Primary: primaryCredentialGroup(), Passphrase: "required"}
}

func bybitCredentialForm() CredentialForm {
	return CredentialForm{Primary: primaryCredentialGroup(), Passphrase: "optional"}
}

func krakenCredentialForm() CredentialForm {
	return CredentialForm{
		Primary:    CredentialGroup{KeyLabel: "Spot API key", SecretLabel: "Spot API secret", SecretInput: "password", Requirement: CredentialRequirement{Mode: "selected", AccountTypes: []AccountType{"main", "earn"}}},
		Futures:    &CredentialGroup{KeyLabel: "Futures API key", SecretLabel: "Futures API secret", SecretInput: "password", Requirement: CredentialRequirement{Mode: "selected_except", AccountTypes: []AccountType{"earn"}}},
		Passphrase: "hidden",
		Help:       "Main requires both key groups to verify the wallet structure. Earn uses spot credentials; independent derivatives accounts use futures credentials.",
	}
}

func (f CredentialForm) clone() CredentialForm {
	f.Primary.Requirement.AccountTypes = append([]AccountType(nil), f.Primary.Requirement.AccountTypes...)
	if f.Futures != nil {
		group := *f.Futures
		group.Requirement.AccountTypes = append([]AccountType(nil), group.Requirement.AccountTypes...)
		f.Futures = &group
	}
	return f
}
