package httpserver

import (
	"context"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) ListPlatforms(ctx context.Context, _ api.ListPlatformsRequestObject) (api.ListPlatformsResponseObject, error) {
	if _, _, ok := a.currentSession(ctx); !ok {
		return nil, errUnauthenticated
	}
	infos := a.Adapters.Platforms()
	items := make([]api.Platform, 0, len(infos))
	for i := range infos {
		info := &infos[i]
		modes := make([]api.AccountMode, 0, len(info.Modes))
		for j := range info.Modes {
			mode := &info.Modes[j]
			types := make([]api.AccountTypeInfo, 0, len(mode.AccountTypes))
			for k := range mode.AccountTypes {
				types = append(types, api.AccountTypeInfo{Id: string(mode.AccountTypes[k].ID), Name: mode.AccountTypes[k].Name})
			}
			modes = append(modes, api.AccountMode{Id: string(mode.ID), Name: mode.Name, AccountTypes: types})
		}
		items = append(items, api.Platform{Id: string(info.ID), Name: info.Name, AdapterAvailable: info.AdapterAvailable, AccountModes: modes})
	}
	return api.ListPlatforms200JSONResponse{Items: items}, nil
}
