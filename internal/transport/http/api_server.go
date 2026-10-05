package httpserver

import (
	"context"
	"errors"

	"github.com/rbee3u/plutus/internal/app"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

// ErrNotImplemented marks contract operations whose application service is not wired.
var ErrNotImplemented = errors.New("not_implemented")

// API implements the generated strict OpenAPI server. It carries the
// composition root's services plus the request-scoped settings the engine
// supplies.
type API struct {
	*app.Services

	Ready             ReadyChecker
	SecureCookies     bool
	SessionCookieName string
	CSRFCookieName    string
}

func (a *API) GetHealthLive(context.Context, api.GetHealthLiveRequestObject) (api.GetHealthLiveResponseObject, error) {
	return api.GetHealthLive200JSONResponse{Status: api.Live}, nil
}

func (a *API) GetHealthReady(ctx context.Context, _ api.GetHealthReadyRequestObject) (api.GetHealthReadyResponseObject, error) {
	if a.Ready == nil {
		return api.GetHealthReady503JSONResponse{Status: api.NotReady, Reason: "readiness_unchecked"}, nil
	}
	if err := a.Ready.Ready(ctx); err != nil {
		return api.GetHealthReady503JSONResponse{Status: api.NotReady, Reason: readyReason(err)}, nil
	}
	return api.GetHealthReady200JSONResponse{Status: api.Ready}, nil
}
