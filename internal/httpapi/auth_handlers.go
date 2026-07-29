package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Willxup/imagesilo/internal/auth"
)

const sessionCookieName = "imagesilo_session"

type authHandler struct {
	service      *auth.Service
	cookieSecure bool
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionResponse struct {
	AdminID   string    `json:"adminId"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func newAuthHandler(service *auth.Service, cookieSecure bool) *authHandler {
	return &authHandler{service: service, cookieSecure: cookieSecure}
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Invalid login request.")
		return
	}
	identity, token, err := h.service.Login(r.Context(), request.Email, request.Password, time.Now())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect.")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to complete login.")
		return
	}
	h.setSessionCookie(w, token, identity.ExpiresAt)
	writeJSON(w, http.StatusOK, toSessionResponse(identity))
}

func (h *authHandler) current(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.authenticateRequest(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator session is required.")
		return
	}
	writeJSON(w, http.StatusOK, toSessionResponse(identity))
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Unable to complete logout.")
			return
		}
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) authenticateRequest(r *http.Request) (auth.SessionIdentity, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return auth.SessionIdentity{}, false
	}
	identity, err := h.service.Authenticate(cookie.Value, time.Now())
	return identity, err == nil
}

func (h *authHandler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *authHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func toSessionResponse(identity auth.SessionIdentity) sessionResponse {
	return sessionResponse{AdminID: identity.AdminID, Email: identity.Email, ExpiresAt: identity.ExpiresAt}
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
