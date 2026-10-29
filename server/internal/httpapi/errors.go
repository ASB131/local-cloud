package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/asb131/local-cloud/internal/admin"
	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/identity"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Error: apiError{Code: code, Message: message, Details: details},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mapServiceError(w http.ResponseWriter, err error) {
	var lock *auth.LockError
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password.", nil)
	case errors.Is(err, auth.ErrAccountDisabled):
		writeError(w, http.StatusForbidden, "ACCOUNT_DISABLED", "This account has been disabled.", nil)
	case errors.Is(err, auth.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many login attempts from this address. Try again later.", nil)
	case errors.Is(err, auth.ErrRefreshReuse):
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Session invalidated. Sign in again.", nil)
	case errors.As(err, &lock):
		writeError(w, http.StatusLocked, "LOCKED", "Account temporarily locked.", map[string]any{
			"retry_after_seconds": lock.RetryAfterSeconds,
		})
	default:
		var val identity.ValidationError
		if errors.As(err, &val) {
			writeError(w, http.StatusBadRequest, "VALIDATION", val.Message, nil)
			return
		}
		var adminVal admin.ValidationError
		if errors.As(err, &adminVal) {
			writeError(w, http.StatusBadRequest, "VALIDATION", adminVal.Message, nil)
			return
		}
		if errors.Is(err, admin.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
			return
		}
		var conflict admin.ConflictError
		if errors.As(err, &conflict) {
			writeError(w, http.StatusConflict, "CONFLICT", conflict.Message, nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL", "An internal error occurred.", nil)
	}
}
