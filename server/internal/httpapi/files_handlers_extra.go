package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/files"
)

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	file, err := s.files.GetFile(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) patchFile(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		Name     *string    `json:"name"`
		FolderID *uuid.UUID `json:"folder_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	file, err := s.files.PatchFile(r.Context(), userID, fid, body.Name, body.FolderID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) postFileCopy(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid file id.", nil)
		return
	}
	var body struct {
		FolderID *uuid.UUID `json:"folder_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	file, err := s.files.CopyFile(r.Context(), userID, fid, body.FolderID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, file)
}

func (s *Server) postFolderCopy(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid folder id.", nil)
		return
	}
	var body struct {
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	folder, err := s.files.CopyFolder(r.Context(), userID, fid, body.ParentID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, folder)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.DeleteFile(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getFileThumbnail(w http.ResponseWriter, r *http.Request) {
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	userID, ok := s.resolveFileAccess(r, fid)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	rel, _, err := s.files.GetThumbnailPath(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	abs, err := s.layout.AbsFromRel(rel)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
		return
	}
	http.ServeFile(w, r, abs)
}

func (s *Server) postArchive(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	var body struct {
		FolderIDs []uuid.UUID `json:"folder_ids"`
		FileIDs   []uuid.UUID `json:"file_ids"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	entries, err := s.files.BuildArchive(r.Context(), userID, body.FolderIDs, body.FileIDs)
	if err != nil {
		if err.Error() == "too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "TOO_LARGE", "Archive too large.", nil)
			return
		}
		mapFilesError(w, err)
		return
	}
	limits, _ := s.files.LoadSpeedLimits(r.Context(), userID)
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(http.StatusOK)
	_ = files.WriteZip(w, entries, limits)
	_ = s.files.Audit.Write(r.Context(), &userID, "file.downloaded", "archive download", nil)
}

func (s *Server) getTrash(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	items, err := s.files.ListTrash(r.Context(), userID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	items, err := s.files.ListFavorites(r.Context(), userID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getFileCategories(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	items, err := s.files.FileCategorySummaries(r.Context(), userID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getFileCategoryFiles(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	category := chi.URLParam(r, "category")
	items, err := s.files.ListFilesByCategory(r.Context(), userID, category, 500)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	hits, err := s.files.Search(r.Context(), userID, q)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": hits})
}
