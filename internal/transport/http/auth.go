package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/rbee3u/plutus/internal/service/auth"
	"github.com/rbee3u/plutus/internal/transport/http/api"
)

func (a *API) current(ctx context.Context) (auth.User, auth.Session, error) {
	if a.Auth == nil {
		return auth.User{}, auth.Session{}, auth.ErrUnauthenticated
	}
	return a.Auth.Authenticate(ctx, requestCookie(ctx, a.SessionCookieName))
}

func (a *API) currentSession(ctx context.Context) (auth.User, auth.Session, error) {
	user, session, err := a.current(ctx)
	if errors.Is(err, auth.ErrUnauthenticated) || errors.Is(err, auth.ErrUserDisabled) {
		return auth.User{}, auth.Session{}, errUnauthenticated
	}
	return user, session, err
}

func (a *API) requireCSRF(ctx context.Context, header *string, session *auth.Session) error {
	token := stringValue(header)
	if session != nil {
		if !auth.CSRFValid(token, session.CSRF) {
			return auth.ErrCSRF
		}
		return nil
	}
	if !auth.CSRFValid(token, requestCookie(ctx, a.CSRFCookieName)) {
		return auth.ErrCSRF
	}
	return nil
}

func (a *API) GetAuthCsrf(ctx context.Context, _ api.GetAuthCsrfRequestObject) (api.GetAuthCsrfResponseObject, error) {
	_, session, err := a.currentSession(ctx)
	if err == nil {
		c := ginFrom(ctx)
		if c != nil {
			writeCookie(c, a.CSRFCookieName, session.CSRF, 3600, false, a.SecureCookies)
		}
		return api.GetAuthCsrf200JSONResponse{Token: session.CSRF}, nil
	}
	if !errors.Is(err, errUnauthenticated) {
		return nil, err
	}
	token, err := auth.NewPreauthCSRF()
	if err != nil {
		return nil, err
	}
	a.setPreauthCSRFCookie(ctx, token)
	return api.GetAuthCsrf200JSONResponse{Token: token}, nil
}

func (a *API) PostAuthLogin(ctx context.Context, request api.PostAuthLoginRequestObject) (api.PostAuthLoginResponseObject, error) {
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, nil); err != nil {
		return nil, err
	}
	if a.Auth == nil || request.Body == nil {
		return api.PostAuthLogin401ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusUnauthorized, "unauthenticated", "Unauthenticated", "Authentication is unavailable.")), nil
	}
	user, session, err := a.Auth.Login(ctx, request.Body.Username, request.Body.Password, clientIP(ctx), userAgent(ctx))
	if err != nil {
		if resp := a.loginError(ctx, err); resp != nil {
			return resp, nil
		}
		return nil, err
	}
	a.setSessionCookies(ctx, &session)
	return api.PostAuthLogin200JSONResponse{User: toAPIUser(user)}, nil
}

func (a *API) loginError(ctx context.Context, err error) api.PostAuthLoginResponseObject {
	switch {
	case errors.Is(err, auth.ErrRateLimited):
		retryAfter(ctx, 60)
		return api.PostAuthLogin429ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusTooManyRequests, "rate_limited", "Too many requests", "Login is temporarily rate limited."))
	case errors.Is(err, auth.ErrPasswordPolicy):
		return api.PostAuthLogin400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrUserDisabled), errors.Is(err, auth.ErrUnauthenticated):
		return api.PostAuthLogin401ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials", "The username or password is incorrect."))
	default:
		return nil
	}
}

func (a *API) PostAuthLogout(ctx context.Context, request api.PostAuthLogoutRequestObject) (api.PostAuthLogoutResponseObject, error) {
	_, session, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if err := a.Auth.Logout(ctx, session.ID); err != nil {
		return nil, err
	}
	a.clearSessionCookies(ctx)
	return api.PostAuthLogout204Response{}, nil
}

func (a *API) PostAuthLogoutAll(ctx context.Context, request api.PostAuthLogoutAllRequestObject) (api.PostAuthLogoutAllResponseObject, error) {
	user, session, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if err := a.Auth.LogoutAll(ctx, user.ID); err != nil {
		return nil, err
	}
	a.clearSessionCookies(ctx)
	return api.PostAuthLogoutAll204Response{}, nil
}

func (a *API) GetAuthMe(ctx context.Context, _ api.GetAuthMeRequestObject) (api.GetAuthMeResponseObject, error) {
	user, _, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := a.Auth.CountSessions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return api.GetAuthMe200JSONResponse{User: toAPIUser(user), SessionCount: sessions}, nil
}

func (a *API) PutAuthPassword(ctx context.Context, request api.PutAuthPasswordRequestObject) (api.PutAuthPasswordResponseObject, error) {
	user, session, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if request.Body == nil {
		return api.PutAuthPassword400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", "A request body is required."))}, nil
	}
	if err := a.Auth.ChangePassword(ctx, user.ID, request.Body.CurrentPassword, request.Body.NewPassword); err != nil {
		if errors.Is(err, auth.ErrPasswordPolicy) || errors.Is(err, auth.ErrSamePassword) {
			return api.PutAuthPassword400ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: api.ProblemApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusBadRequest, "validation_failed", "Request validation failed", err.Error()))}, nil
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return api.PutAuthPassword422ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusUnprocessableEntity, "invalid_current_password", "Current password is incorrect", "The current password is incorrect.")), nil
		}
		return nil, err
	}
	a.clearSessionCookies(ctx)
	return api.PutAuthPassword204Response{}, nil
}

func (a *API) GetAuthSessions(ctx context.Context, _ api.GetAuthSessionsRequestObject) (api.GetAuthSessionsResponseObject, error) {
	user, session, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	items, err := a.Auth.ListSessions(ctx, user.ID, session.ID)
	if err != nil {
		return nil, err
	}
	out := make([]api.Session, 0, len(items))
	for i := range items {
		out = append(out, toAPISession(&items[i]))
	}
	return api.GetAuthSessions200JSONResponse{Items: out}, nil
}

func (a *API) DeleteAuthSession(ctx context.Context, request api.DeleteAuthSessionRequestObject) (api.DeleteAuthSessionResponseObject, error) {
	user, session, err := a.currentSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.requireCSRF(ctx, request.Params.XCSRFToken, &session); err != nil {
		return nil, err
	}
	if err := a.Auth.RevokeOwnedSession(ctx, user.ID, request.SessionId); err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) {
			return api.DeleteAuthSession404ApplicationProblemPlusJSONResponse(a.problem(ctx, http.StatusNotFound, "not_found", "Not found", "The session was not found.")), nil
		}
		return nil, err
	}
	if request.SessionId == session.ID {
		a.clearSessionCookies(ctx)
	}
	return api.DeleteAuthSession204Response{}, nil
}

func toAPIUser(user auth.User) api.User {
	return api.User{
		Id:       user.ID,
		Username: user.Username,
		Status:   api.UserStatus(user.Status),
	}
}

func toAPISession(session *auth.Session) api.Session {
	return api.Session{
		AbsoluteExpiresAt: session.AbsoluteExpiresAt,
		UserAgent:         session.UserAgent,
		CreatedAt:         session.CreatedAt,
		Current:           session.Current,
		Id:                session.ID,
		IdleExpiresAt:     session.IdleExpiresAt,
		LastSeenAt:        session.LastSeenAt,
	}
}
