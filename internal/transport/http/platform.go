package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/platform/adapter"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListPlatforms(ctx context.Context, _ api.ListPlatformsRequestObject) (api.ListPlatformsResponseObject, error) {
	if _, _, err := a.currentSession(ctx); err != nil {
		return nil, err
	}
	infos := a.Adapters.Platforms()
	items := make([]api.Platform, 0, len(infos))
	for i := range infos {
		info := &infos[i]
		structures := make([]api.AccountStructure, 0, len(info.Structures))
		for j := range info.Structures {
			structure := &info.Structures[j]
			types := make([]api.AccountTypeInfo, 0, len(structure.AccountTypes))
			for _, t := range structure.AccountTypes {
				types = append(types, api.AccountTypeInfo{Id: string(t.ID), Name: t.Name})
			}
			modes := make([]api.MarginModeInfo, 0, len(structure.MarginModes))
			for _, m := range structure.MarginModes {
				modes = append(modes, api.MarginModeInfo{Id: string(m.ID), Name: m.Name})
			}
			structures = append(structures, api.AccountStructure{Id: string(structure.ID), Name: structure.Name, AccountTypes: types, MarginModes: modes})
		}
		items = append(items, api.Platform{Id: string(info.ID), Name: info.Name, AdapterAvailable: info.AdapterAvailable, AccountStructures: structures, CredentialForm: credentialForm(info.CredentialForm)})
	}
	return api.ListPlatforms200JSONResponse{Items: items}, nil
}

func credentialForm(form adapter.CredentialForm) api.CredentialForm {
	out := api.CredentialForm{Primary: credentialGroup(form.Primary), Passphrase: api.CredentialFormPassphrase(form.Passphrase), Help: form.Help}
	if form.Futures != nil {
		group := credentialGroup(*form.Futures)
		out.Futures = &group
	}
	return out
}

func credentialGroup(group adapter.CredentialGroup) api.CredentialGroup {
	accounts := make([]string, 0, len(group.Requirement.AccountTypes))
	for _, account := range group.Requirement.AccountTypes {
		accounts = append(accounts, string(account))
	}
	return api.CredentialGroup{KeyLabel: group.KeyLabel, SecretLabel: group.SecretLabel, SecretInput: api.CredentialGroupSecretInput(group.SecretInput), Requirement: api.CredentialRequirement{Mode: api.CredentialRequirementMode(group.Requirement.Mode), AccountTypes: accounts}}
}
