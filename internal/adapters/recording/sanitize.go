package recording

import (
	"strings"

	"github.com/rbee3u/plutus/internal/decimal"
)

func sensitive(key string) bool {
	switch strings.ToLower(key) {
	case "uid", "mainuid", "userid", "accountuid", "memberid", "iban", "iiban", "api_key", "apikey", "secret", "passphrase", "ip", "ips", "iplist", "ip_allowlist", "allowedcidrblock", "allowedcidrblocks", "label", "apikeyname", "api_key_name", "signature", "authorization", "access_token", "refresh_token", "privatekey", "api_secret":
		return true
	}
	return false
}

func sensitiveResponse(path, key string) bool {
	if path == "/ws/v1/balances_snapshot" && key == "account" {
		return true
	}
	if path == "/v5/user/query-api" {
		switch strings.ToLower(key) {
		case "id", "note", "inviterid", "affiliateid", "parentuid", "rsapublickey", "kycregion":
			return true
		}
	}
	return sensitive(key)
}

func visit(value any, fn func(string, any)) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			fn(key, child)
			visit(child, fn)
		}
	case []any:
		for _, child := range v {
			visit(child, fn)
		}
	}
}

func clean(value any, text func(string) string, fields map[string]bool, private func(string) bool, mask func(string, any) any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if private(key) {
				fields[key] = true
				v[key] = mask(key, child)
			} else {
				v[key] = clean(child, text, fields, private, mask)
			}
		}
	case []any:
		for i, child := range v {
			v[i] = clean(child, text, fields, private, mask)
		}
	case string:
		if decimal.Valid(v) {
			return v // Decimal business values must survive even when digits overlap a customer ID.
		}
		out := text(v)
		if out != v {
			fields["credential or customer-reference echo"] = true
		}
		if ipPattern.MatchString(out) {
			fields["IP address"] = true
			out = ipPattern.ReplaceAllString(out, "203.0.113.1")
		}
		return out
	}
	return value
}
