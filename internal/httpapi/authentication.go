package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/Willxup/imagesilo/internal/apitoken"
	"github.com/Willxup/imagesilo/internal/auth"
)

type principal struct {
	Session         *auth.SessionIdentity
	APIToken        *apitoken.Identity
	RawSessionToken string
}

type authenticator struct {
	sessions *auth.Service
	tokens   *apitoken.Service
}

func newAuthenticator(sessions *auth.Service, tokens *apitoken.Service) *authenticator {
	return &authenticator{sessions: sessions, tokens: tokens}
}

func (a *authenticator) requireSession(w http.ResponseWriter, r *http.Request, requireCSRF bool) (principal, bool) {
	if a.sessions == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session is required.")
		return principal{}, false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session is required.")
		return principal{}, false
	}
	identity, err := a.sessions.Authenticate(cookie.Value, time.Now())
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid_session", "Administrator session is invalid or expired.")
		return principal{}, false
	}
	if requireCSRF && !a.validCSRF(r, identity) {
		writeError(w, r, http.StatusForbidden, "invalid_csrf", "CSRF validation failed.")
		return principal{}, false
	}
	return principal{Session: &identity, RawSessionToken: cookie.Value}, true
}

func (a *authenticator) requireScope(w http.ResponseWriter, r *http.Request, scope apitoken.Scope) (principal, bool) {
	return a.requireScopes(w, r, scope)
}

func (a *authenticator) requireScopes(w http.ResponseWriter, r *http.Request, scopes ...apitoken.Scope) (principal, bool) {
	if a.sessions != nil {
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil {
			if identity, err := a.sessions.Authenticate(cookie.Value, time.Now()); err == nil {
				if !isSafeMethod(r.Method) && !a.validCSRF(r, identity) {
					writeError(w, r, http.StatusForbidden, "invalid_csrf", "CSRF validation failed.")
					return principal{}, false
				}
				return principal{Session: &identity, RawSessionToken: cookie.Value}, true
			}
		}
	}

	plaintext, present := bearerToken(r)
	if !present || a.tokens == nil {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session or Bearer token is required.")
		return principal{}, false
	}
	identity, err := a.tokens.Authenticate(plaintext, time.Now())
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid_token", "Bearer token is invalid or expired.")
		return principal{}, false
	}
	for _, scope := range scopes {
		if !identity.HasScope(scope) {
			writeError(w, r, http.StatusForbidden, "insufficient_scope", "Bearer token does not have the required scope.")
			return principal{}, false
		}
	}
	return principal{APIToken: &identity}, true
}

func (a *authenticator) privateImageAccess(r *http.Request) (bool, bool) {
	if a.sessions != nil {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if _, err := a.sessions.Authenticate(cookie.Value, time.Now()); err == nil {
				return true, false
			}
		}
	}
	plaintext, present := bearerToken(r)
	if !present || a.tokens == nil {
		return false, false
	}
	identity, err := a.tokens.Authenticate(plaintext, time.Now())
	if err != nil {
		return false, false
	}
	if !identity.HasScope(apitoken.ScopeImagesReadPrivate) {
		return false, true
	}
	return true, false
}

func (a *authenticator) validCSRF(r *http.Request, identity auth.SessionIdentity) bool {
	if a.sessions == nil {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) != 1 {
		return false
	}
	return a.sessions.ValidateCSRF(identity, header) == nil
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}
	return value, true
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
