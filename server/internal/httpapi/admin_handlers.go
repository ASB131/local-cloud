package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/admin"
)

func (s *Server) getAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.adminUsers.ListUsers(r.Context())
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (s *Server) patchAdminUser(w http.ResponseWriter, r *http.Request) {
	actorID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid user id.", nil)
		return
	}
	var patch admin.UserPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	if err := s.adminUsers.PatchUser(r.Context(), actorID, targetID, patch); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteAdminUser(w http.ResponseWriter, r *http.Request) {
	actorID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid user id.", nil)
		return
	}
	if err := s.adminUsers.DeleteUser(r.Context(), actorID, targetID); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type inviteCreateRequest struct {
	ExpiresAt         *time.Time `json:"expires_at"`
	DefaultQuotaBytes *int64     `json:"default_quota_bytes"`
}

func (s *Server) postAdminInvite(w http.ResponseWriter, r *http.Request) {
	actorID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	var body inviteCreateRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	result, err := s.adminInvites.Create(r.Context(), actorID, admin.InviteCreateInput{
		ExpiresAt:         body.ExpiresAt,
		DefaultQuotaBytes: body.DefaultQuotaBytes,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) getAdminInvites(w http.ResponseWriter, r *http.Request) {
	items, err := s.adminInvites.List(r.Context())
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) deleteAdminInvite(w http.ResponseWriter, r *http.Request) {
	actorID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	inviteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid invite id.", nil)
		return
	}
	if err := s.adminInvites.Revoke(r.Context(), actorID, inviteID); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
