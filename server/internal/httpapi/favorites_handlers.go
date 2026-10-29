package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) postFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	var body struct {
		FileID   *uuid.UUID `json:"file_id"`
		FolderID *uuid.UUID `json:"folder_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	fav, err := s.files.AddFavorite(r.Context(), userID, body.FileID, body.FolderID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, fav)
}

func (s *Server) deleteFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.RemoveFavorite(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
