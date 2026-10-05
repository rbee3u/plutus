package httpserver

import (
	"github.com/rbee3u/plutus/internal/domain/identity"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func toAPIIdentity(item *identity.Identity) api.Identity {
	accounts := make([]api.Account, 0, len(item.Accounts))
	for i := range item.Accounts {
		accounts = append(accounts, toAPIAccount(&item.Accounts[i]))
	}
	return api.Identity{
		Id:               item.ID,
		PortfolioId:      item.PortfolioID,
		Platform:         string(item.Platform),
		DisplayName:      item.DisplayName,
		AccountStructure: string(item.AccountStructure),
		MarginMode:       strPtr(string(item.MarginMode)),
		ExternalRef:      item.ExternalRef,
		Status:           api.IdentityStatus(item.Status),
		ArchivedAt:       item.ArchivedAt,
		Connection: api.Connection{
			Status:                api.ConnectionStatus(item.Connection.Status),
			CredentialsConfigured: item.Connection.CredentialsConfigured,
			CredentialsUpdatedAt:  item.Connection.CredentialsUpdatedAt,
			LastValidatedAt:       item.Connection.LastValidatedAt,
			LastCollectedAt:       item.Connection.LastCollectedAt,
			LastErrorClass:        item.Connection.LastErrorClass,
			LastErrorDetail:       item.Connection.LastErrorDetail,
		},
		Accounts:  accounts,
		Version:   int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func toAPIAccount(item *identity.Account) api.Account {
	return api.Account{
		Id:          item.ID,
		IdentityId:  item.IdentityID,
		AccountType: string(item.AccountType),
		DisplayName: item.DisplayName,
		Selected:    item.Selected,
		Version:     int(item.Version), // #nosec G115 -- Versions increment from 1; overflowing int requires billions of edits even on 32-bit hosts.
	}
}

func toAPIOutcome(item *identity.Identity, outcome *identity.ValidationOutcome) api.ValidationOutcome {
	return api.ValidationOutcome{
		Ok:          outcome.OK,
		ExternalRef: strPtr(outcome.ExternalRef),
		ErrorClass:  strPtr(outcome.ErrorClass),
		ErrorDetail: strPtr(outcome.ErrorDetail),
		ValidatedAt: outcome.ValidatedAt,
		Identity:    toAPIIdentity(item),
	}
}
