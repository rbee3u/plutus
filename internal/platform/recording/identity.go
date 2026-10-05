package recording

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// identityMask preserves equality and inequality between customer references throughout a
// capture. Collapsing all IDs into one placeholder would hide a key ownership or account change.
// Empty nullable references remain empty, so Kraken's iiban fallback is replayed honestly.
func identityMask() func(string, any) any {
	ids := map[string]int{}
	return func(key string, value any) any {
		var text string
		numeric := false
		switch v := value.(type) {
		case string:
			text = v
			if text == "" {
				return ""
			}
		case json.Number:
			text = string(v)
			numeric = true
		case []any:
			return []any{}
		case map[string]any:
			return map[string]any{}
		default:
			return value
		}
		switch strings.ToLower(key) {
		case "iban", "iiban":
			text = strings.ToUpper(strings.Join(strings.Fields(text), ""))
		case "uid", "mainuid", "userid", "accountuid", "memberid", "account":
			text = strings.TrimSpace(text)
		default:
			if numeric {
				return json.Number("123456789")
			}
			return "REDACTED"
		}
		id, ok := ids[text]
		if !ok {
			id = len(ids) + 1
			ids[text] = id
		}
		placeholder := strconv.Itoa(123456789 + id)
		if numeric {
			return json.Number(placeholder)
		}
		if _, err := strconv.ParseUint(text, 10, 64); err == nil {
			return placeholder
		}
		return fmt.Sprintf("REDACTED_ACCOUNT_%d", id)
	}
}
