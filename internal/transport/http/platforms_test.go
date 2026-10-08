package httpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rbee3u/plutus/internal/platform/adapters"
)

func TestCredentialFormWireRequirements(t *testing.T) {
	for _, platform := range adapters.NewRegistry().Platforms() {
		form := credentialForm(platform.CredentialForm)
		raw, err := json.Marshal(form)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"accountTypes":null`) {
			t.Fatalf("%s: required arrays must not be null: %s", platform.ID, raw)
		}
		if form.Primary.KeyLabel == "" || (form.Futures != nil) != (platform.ID == adapters.PlatformKraken) {
			t.Fatalf("%s: lost credential group: %+v", platform.ID, form)
		}
	}
}
