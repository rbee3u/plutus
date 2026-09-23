package httpserver

import (
	"context"
	"errors"

	"github.com/rbee3u/plutus/internal/adapters"
	"github.com/rbee3u/plutus/internal/audit"
	"github.com/rbee3u/plutus/internal/auth"
	"github.com/rbee3u/plutus/internal/capital"
	"github.com/rbee3u/plutus/internal/catalog"
	"github.com/rbee3u/plutus/internal/idempotency"
	"github.com/rbee3u/plutus/internal/identity"
	"github.com/rbee3u/plutus/internal/jobs"
	"github.com/rbee3u/plutus/internal/portfolio"
	"github.com/rbee3u/plutus/internal/transport/http/api"
	"github.com/rbee3u/plutus/internal/valuation"
)

// ErrNotImplemented marks contract operations whose application service is not wired.
var ErrNotImplemented = errors.New("not_implemented")

// API implements the generated strict OpenAPI server.
type API struct {
	Ready             ReadyChecker
	Auth              *auth.Service
	SecureCookies     bool
	SessionCookieName string
	CSRFCookieName    string
	Portfolios        *portfolio.Service
	Identities        *identity.Service
	Adapters          *adapters.Registry
	Catalog           *catalog.Service
	Capital           *capital.Service
	Valuation         *valuation.Service
	Jobs              *jobs.Service
	Audit             *audit.Service
	Idempotency       *idempotency.Store
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
