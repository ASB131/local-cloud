package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/files"
)

func (s *Server) postTrashRestoreFile(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		DestinationFolderID *uuid.UUID `json:"destination_folder_id"`
		ConflictPolicy      string     `json:"conflict_policy"`
		Name                *string    `json:"name"`
		FileID              *uuid.UUID `json:"file_id"`
	}
	_ = decodeJSON(r, &body)
	file, err := s.files.RestoreFile(r.Context(), userID, fid, body.DestinationFolderID, body.Name, body.ConflictPolicy, body.FileID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) postTrashRestoreFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.RestoreFolder(r.Context(), userID, fid, nil); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteTrashFile(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.HardDeleteFile(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteTrashFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.HardDeleteFolder(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postRestoreVersion(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	vid, _ := uuid.Parse(chi.URLParam(r, "version_id"))
	file, err := s.files.RestoreVersion(r.Context(), userID, fid, vid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) resolveVersionAccess(r *http.Request, fileID, versionID uuid.UUID) (uuid.UUID, bool) {
	if mt := r.URL.Query().Get("mt"); mt != "" {
		uid, err := s.tokens.ParseVersionMediaToken(mt, fileID, versionID)
		if err != nil {
			return uuid.Nil, false
		}
		return uid, true
	}
	userID, ok, _ := userFromContext(r.Context())
	return userID, ok
}

func (s *Server) postVersionMediaToken(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid file id.", nil)
		return
	}
	vid, err := uuid.Parse(chi.URLParam(r, "version_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid version id.", nil)
		return
	}
	if _, err := s.files.GetFile(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	tok, exp, err := s.tokens.IssueVersionMediaToken(userID, fid, vid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "An internal error occurred.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "expires_at": exp})
}

func (s *Server) getVersionContent(w http.ResponseWriter, r *http.Request) {
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid file id.", nil)
		return
	}
	vid, err := uuid.Parse(chi.URLParam(r, "version_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid version id.", nil)
		return
	}
	userID, ok := s.resolveVersionAccess(r, fid, vid)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	fc, status, err := s.files.ResolveVersionContent(r.Context(), userID, fid, vid, r.Header.Get("Range"))
	if err != nil {
		if status == http.StatusRequestedRangeNotSatisfiable {
			writeError(w, http.StatusRequestedRangeNotSatisfiable, "RANGE_NOT_SATISFIABLE", "Invalid range.", nil)
			return
		}
		mapFilesError(w, err)
		return
	}
	limits, _ := s.files.LoadSpeedLimits(r.Context(), userID)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", fc.Mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename*=UTF-8''%s`, url.PathEscape(fc.Name)))
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fc.Offset, fc.Offset+fc.Length-1, fc.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(fc.Length, 10))
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(fc.Size, 10))
	}
	if err := files.WriteFileContent(w, fc, status, limits); err != nil && !files.IsStreamClientAbort(err) {
		s.log.Error("version stream", "err", err)
	}
}

func (s *Server) getFileVersions(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	vers, err := s.files.ListVersions(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": vers})
}
