package httpserver

import (
	"context"
	"errors"

	"github.com/rbee3u/plutus/internal/transport/http/api"
)

// ErrNotImplemented marks Phase 1 endpoints that still need application services.
var ErrNotImplemented = errors.New("not_implemented")

// API implements the generated strict OpenAPI server.
type API struct {
	Ready               ReadyChecker
	RegistrationEnabled bool
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

func (a *API) GetAuthRegistrationStatus(context.Context, api.GetAuthRegistrationStatusRequestObject) (api.GetAuthRegistrationStatusResponseObject, error) {
	return api.GetAuthRegistrationStatus200JSONResponse{Enabled: a.RegistrationEnabled}, nil
}

func (a *API) GetAuthCsrf(context.Context, api.GetAuthCsrfRequestObject) (api.GetAuthCsrfResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PostAuthLogin(context.Context, api.PostAuthLoginRequestObject) (api.PostAuthLoginResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PostAuthLogout(context.Context, api.PostAuthLogoutRequestObject) (api.PostAuthLogoutResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PostAuthLogoutAll(context.Context, api.PostAuthLogoutAllRequestObject) (api.PostAuthLogoutAllResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetAuthMe(context.Context, api.GetAuthMeRequestObject) (api.GetAuthMeResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PutAuthPassword(context.Context, api.PutAuthPasswordRequestObject) (api.PutAuthPasswordResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PostAuthRegister(context.Context, api.PostAuthRegisterRequestObject) (api.PostAuthRegisterResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetAuthSessions(context.Context, api.GetAuthSessionsRequestObject) (api.GetAuthSessionsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) DeleteAuthSession(context.Context, api.DeleteAuthSessionRequestObject) (api.DeleteAuthSessionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListPortfolios(context.Context, api.ListPortfoliosRequestObject) (api.ListPortfoliosResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) CreatePortfolio(context.Context, api.CreatePortfolioRequestObject) (api.CreatePortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) DeletePortfolio(context.Context, api.DeletePortfolioRequestObject) (api.DeletePortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetPortfolio(context.Context, api.GetPortfolioRequestObject) (api.GetPortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PatchPortfolio(context.Context, api.PatchPortfolioRequestObject) (api.PatchPortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ArchivePortfolio(context.Context, api.ArchivePortfolioRequestObject) (api.ArchivePortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) RestorePortfolio(context.Context, api.RestorePortfolioRequestObject) (api.RestorePortfolioResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListConnections(context.Context, api.ListConnectionsRequestObject) (api.ListConnectionsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) CreateConnection(context.Context, api.CreateConnectionRequestObject) (api.CreateConnectionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) DeleteConnection(context.Context, api.DeleteConnectionRequestObject) (api.DeleteConnectionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetConnection(context.Context, api.GetConnectionRequestObject) (api.GetConnectionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PatchConnection(context.Context, api.PatchConnectionRequestObject) (api.PatchConnectionResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListIdentities(context.Context, api.ListIdentitiesRequestObject) (api.ListIdentitiesResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) CreateIdentity(context.Context, api.CreateIdentityRequestObject) (api.CreateIdentityResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetIdentity(context.Context, api.GetIdentityRequestObject) (api.GetIdentityResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) PatchIdentity(context.Context, api.PatchIdentityRequestObject) (api.PatchIdentityResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ArchiveIdentity(context.Context, api.ArchiveIdentityRequestObject) (api.ArchiveIdentityResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) ListJobs(context.Context, api.ListJobsRequestObject) (api.ListJobsResponseObject, error) {
	return nil, ErrNotImplemented
}

func (a *API) GetJob(context.Context, api.GetJobRequestObject) (api.GetJobResponseObject, error) {
	return nil, ErrNotImplemented
}
