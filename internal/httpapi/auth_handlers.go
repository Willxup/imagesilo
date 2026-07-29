package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
)

const (
	sessionCookieName = "imagesilo_session"
	csrfCookieName    = "imagesilo_csrf"
)

type authHandler struct {
	service        *auth.Service
	authenticator  *authenticator
	cookieSecure   bool
	accountLimiter *loginLimiter
	addressLimiter *loginLimiter
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionResponse struct {
	AdminID   string    `json:"adminId"`
	Email     string    `json:"email"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func newAuthHandler(service *auth.Service, authenticator *authenticator, cookieSecure bool) *authHandler {
	return &authHandler{
		service: service, authenticator: authenticator, cookieSecure: cookieSecure,
		accountLimiter: newLoginLimiter(5, 5*time.Minute),
		addressLimiter: newLoginLimiter(20, 5*time.Minute),
	}
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid login request.")
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if request.Email == "" || len(request.Email) > 254 || request.Password == "" || len(request.Password) > 1024 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Email or password has an invalid length.")
		return
	}
	now := time.Now()
	accountKey := "account:" + strings.ToLower(strings.TrimSpace(request.Email))
	addressKey := "address:" + remoteAddress(r)
	if allowed, retry := h.addressLimiter.Allow(addressKey, now); !allowed {
		writeRateLimited(w, r, retry)
		return
	}
	if allowed, retry := h.accountLimiter.Allow(accountKey, now); !allowed {
		writeRateLimited(w, r, retry)
		return
	}
	identity, token, csrfToken, err := h.service.Login(r.Context(), request.Email, request.Password, now)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to complete login.")
		return
	}
	if oldCookie, cookieErr := r.Cookie(sessionCookieName); cookieErr == nil && oldCookie.Value != token {
		if logoutErr := h.service.Logout(r.Context(), oldCookie.Value); logoutErr != nil {
			_ = h.service.Logout(r.Context(), token)
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to rotate administrator session.")
			return
		}
	}
	h.accountLimiter.Reset(accountKey)
	h.addressLimiter.Reset(addressKey)
	h.setSessionCookies(w, token, csrfToken, identity.ExpiresAt)
	writeJSON(w, http.StatusOK, toSessionResponse(identity, csrfToken))
}

func (h *authHandler) current(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authenticator.requireSession(w, r, false)
	if !ok {
		return
	}
	csrfCookie, err := r.Cookie(csrfCookieName)
	if err != nil || h.service.ValidateCSRF(*principal.Session, csrfCookie.Value) != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid_session", "Administrator session is missing its CSRF token.")
		return
	}
	writeJSON(w, http.StatusOK, toSessionResponse(*principal.Session, csrfCookie.Value))
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authenticator.requireSession(w, r, true)
	if !ok {
		return
	}
	if err := h.service.Logout(r.Context(), principal.RawSessionToken); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to complete logout.")
		return
	}
	h.clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.authenticator.requireSession(w, r, true)
	if !ok {
		return
	}
	var request changePasswordRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid password change request.")
		return
	}
	if len(request.CurrentPassword) > 1024 || len(request.NewPassword) > 1024 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Password has an invalid length.")
		return
	}
	err := h.service.ChangePassword(
		r.Context(), *principal.Session, principal.RawSessionToken,
		request.CurrentPassword, request.NewPassword, time.Now(),
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "Current password is incorrect.")
		case errors.Is(err, auth.ErrPasswordTooShort):
			writeError(w, r, http.StatusBadRequest, "password_too_short", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to change password.")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) setSessionCookies(w http.ResponseWriter, token, csrfToken string, expiresAt time.Time) {
	maxAge := int(expiresAt.Sub(time.Now()).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *authHandler) clearSessionCookies(w http.ResponseWriter) {
	for _, cookie := range []http.Cookie{
		{Name: sessionCookieName, HttpOnly: true},
		{Name: csrfCookieName, HttpOnly: false},
	} {
		cookie.Value = ""
		cookie.Path = "/"
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
		cookie.Secure = h.cookieSecure
		cookie.SameSite = http.SameSiteLaxMode
		http.SetCookie(w, &cookie)
	}
}

func toSessionResponse(identity auth.SessionIdentity, csrfToken string) sessionResponse {
	return sessionResponse{AdminID: identity.AdminID, Email: identity.Email, CSRFToken: csrfToken, ExpiresAt: identity.ExpiresAt}
}

func remoteAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeRateLimited(w http.ResponseWriter, r *http.Request, retry time.Duration) {
	seconds := int(retry.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, r, http.StatusTooManyRequests, "login_rate_limited", "Too many login attempts. Try again later.")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain exactly one JSON value")
	}
	return nil
}
