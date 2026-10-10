package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rbee3u/plutus/internal/application/auth"
)

func (a *API) setSessionCookies(ctx context.Context, session *auth.Session) {
	c := ginFrom(ctx)
	if c == nil {
		return
	}
	maxAge := int(time.Until(session.AbsoluteExpiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	writeCookie(c, a.SessionCookieName, session.Token, maxAge, true, a.SecureCookies)
	writeCookie(c, a.CSRFCookieName, session.CSRF, maxAge, false, a.SecureCookies)
}

func (a *API) setPreauthCSRFCookie(ctx context.Context, token string) {
	c := ginFrom(ctx)
	if c == nil {
		return
	}
	writeCookie(c, a.CSRFCookieName, token, 3600, false, a.SecureCookies)
}

func (a *API) clearSessionCookies(ctx context.Context) {
	c := ginFrom(ctx)
	if c == nil {
		return
	}
	writeCookie(c, a.SessionCookieName, "", -1, true, a.SecureCookies)
	writeCookie(c, a.CSRFCookieName, "", -1, false, a.SecureCookies)
}

func writeCookie(c *gin.Context, name, value string, maxAge int, httpOnly, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{ //nolint:gosec // CSRF cookies must be readable; session cookies are HttpOnly.
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func requestCookie(ctx context.Context, name string) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	value, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return value
}

func clientIP(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	return c.ClientIP()
}

func userAgent(ctx context.Context) string {
	c := ginFrom(ctx)
	if c == nil {
		return ""
	}
	return c.GetHeader("User-Agent")
}

func retryAfter(ctx context.Context, seconds int) {
	c := ginFrom(ctx)
	if c == nil {
		return
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
}
