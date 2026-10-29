package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/identity"
)

type loginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceLabel string `json:"device_label"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type registerRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	InviteCode  string `json:"invite_code"`
	DeviceLabel string `json:"device_label"`
}

type authResponse struct {
	AccessToken     string                `json:"access_token"`
	AccessExpiresAt time.Time             `json:"access_expires_at"`
	RefreshToken    string                `json:"refresh_token"`
	User            identity.UserProfile  `json:"user"`
}

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	result, err := s.auth.Login(r.Context(), body.Username, body.Password, body.DeviceLabel, clientIP(r, s.trustedProxies))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:     result.AccessToken,
		AccessExpiresAt: result.AccessExpiresAt,
		RefreshToken:    result.RefreshToken,
		User:            result.Profile,
	})
}

func (s *Server) postRegister(w http.ResponseWriter, r *http.Request) {
	var body registerRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	result, err := s.auth.Register(r.Context(), body.Username, body.Password, body.InviteCode, body.DeviceLabel, clientIP(r, s.trustedProxies))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid invite or credentials.", nil)
			return
		}
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{
		AccessToken:     result.AccessToken,
		AccessExpiresAt: result.AccessExpiresAt,
		RefreshToken:    result.RefreshToken,
		User:            result.Profile,
	})
}

func (s *Server) postRefresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if err := decodeJSON(r, &body); err != nil || body.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	result, err := s.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:     result.AccessToken,
		AccessExpiresAt: result.AccessExpiresAt,
		RefreshToken:    result.RefreshToken,
		User:            result.Profile,
	})
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	var body logoutRequest
	if err := decodeJSON(r, &body); err != nil || body.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	if err := s.auth.Logout(r.Context(), userID, body.RefreshToken, clientIP(r, s.trustedProxies)); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	profile, err := identity.LoadProfile(r.Context(), s.pool, userID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}
