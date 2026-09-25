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
		items = append(items, api.Platform{Id: string(info.ID), Name: info.Name, AdapterAvailable: info.AdapterAvailable, AccountStructures: structures})
	}
	return api.ListPlatforms200JSONResponse{Items: items}, nil
}
