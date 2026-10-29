package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/admin"
)

func (s *Server) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	st, err := admin.GetSettings(r.Context(), s.pool)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) patchAdminSettings(w http.ResponseWriter, r *http.Request) {
	var patch admin.SettingsPatch
	if decodeJSON(r, &patch) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	if err := admin.PatchSettings(r.Context(), s.pool, patch); err != nil {
		mapServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getAdminStats(w http.ResponseWriter, r *http.Request) {
	st, err := admin.GetStats(r.Context(), s.pool)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) getAdminLogs(w http.ResponseWriter, r *http.Request) {
	q := admin.LogQuery{EventType: r.URL.Query().Get("event_type"), Limit: 50}
	if c := r.URL.Query().Get("cursor"); c != "" {
		if t, err := time.Parse(time.RFC3339, c); err == nil {
			q.Cursor = &t
		}
	}
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		if id, err := uuid.Parse(uid); err == nil {
			q.UserID = &id
		}
	}
	items, err := admin.ListLogs(r.Context(), s.pool, q)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) postAdminRebuild(w http.ResponseWriter, r *http.Request) {
	actorID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	report, err := admin.StartRebuild(r.Context(), s.files, s.audit, actorID)
	if err != nil {
		var c admin.ConflictError
		if errors.As(err, &c) {
			writeError(w, http.StatusConflict, "CONFLICT", c.Message, nil)
			return
		}
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, report)
}

func (s *Server) getAdminRebuild(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, admin.LastRebuildReport())
}
