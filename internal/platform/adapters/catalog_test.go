package adapters

import (
	"reflect"
	"testing"
)

func TestCatalogResultsCannotMutateDefinitions(t *testing.T) {
	registry := NewRegistry()
	before := registry.Platforms()
	changed := registry.Platforms()
	changed[0].Name = "changed"
	changed[0].Structures[0].AccountTypes[0].Name = "changed"
	changed[1].Structures[0].MarginModes[0].Name = "changed"
	changed[3].CredentialForm.Primary.Requirement.AccountTypes[0] = "changed"
	changed[3].CredentialForm.Futures.Requirement.AccountTypes[0] = "changed"
	changed[3].CredentialForm.Futures.KeyLabel = "changed"
	if !reflect.DeepEqual(before, registry.Platforms()) {
		t.Fatal("returned catalog shares mutable definitions")
	}
}

func TestCatalogCredentialForms(t *testing.T) {
	seen := map[Platform]bool{}
	structures := map[AccountStructure]bool{}
	for _, platform := range NewRegistry().Platforms() {
		if seen[platform.ID] {
			t.Fatalf("duplicate Platform %s", platform.ID)
		}
		seen[platform.ID] = true
		for _, structure := range platform.Structures {
			if structures[structure.ID] || !ValidStructure(platform.ID, structure.ID) {
				t.Fatalf("invalid Structure %s", structure.ID)
			}
			structures[structure.ID] = true
			if ValidAccountType(structure.ID, TypeManual) {
				t.Fatal("Manual entered the exchange catalog")
			}
		}
		form := platform.CredentialForm
		if form.Primary.KeyLabel == "" || form.Primary.SecretLabel == "" {
			t.Fatalf("missing labels: %s", platform.ID)
		}
		switch platform.ID {
		case PlatformBinance:
			if form.Primary.SecretInput != "multiline" || form.Passphrase != "hidden" || form.Help == "" {
				t.Fatalf("Binance form: %+v", form)
			}
		case PlatformOKX:
			if form.Passphrase != "required" {
				t.Fatalf("OKX form: %+v", form)
			}
		case PlatformBybit:
			if form.Passphrase != "optional" {
				t.Fatalf("Bybit form: %+v", form)
			}
		case PlatformKraken:
			if form.Futures == nil || form.Primary.Requirement.Mode != "selected" || !reflect.DeepEqual(form.Primary.Requirement.AccountTypes, []AccountType{"main", "earn"}) || form.Futures.Requirement.Mode != "selected_except" || !reflect.DeepEqual(form.Futures.Requirement.AccountTypes, []AccountType{"earn"}) {
				t.Fatalf("Kraken form: %+v", form)
			}
		}
	}
}
