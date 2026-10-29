package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/files"
)

func (s *Server) getFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid folder id.", nil)
		return
	}
	f, crumbs, err := s.files.GetFolder(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": f, "breadcrumb": crumbs})
}

func (s *Server) getFolderChildren(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid folder id.", nil)
		return
	}
	resp, err := s.files.ListChildren(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) postFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	var body struct {
		ParentID uuid.UUID `json:"parent_id"`
		Name     string    `json:"name"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	f, err := s.files.CreateFolder(r.Context(), userID, body.ParentID, body.Name)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) patchFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	var body struct {
		Name     *string    `json:"name"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
	f, err := s.files.PatchFolder(r.Context(), userID, fid, body.Name, body.ParentID)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) deleteFolder(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.DeleteFolder(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) postUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	var body struct {
		FolderID           uuid.UUID  `json:"folder_id"`
		Name               string     `json:"name"`
		ExpectedSizeBytes  *int64     `json:"expected_size_bytes"`
		MimeHint           *string    `json:"mime_hint"`
		MetaCreatedAt      *time.Time `json:"meta_created_at"`
		MetaModifiedAt     *time.Time `json:"meta_modified_at"`
		ConflictPolicy     string     `json:"conflict_policy"`
		FileID             *uuid.UUID `json:"file_id"`
	}
	if decodeJSON(r, &body) != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid request body.", nil)
		return
	}
 sess, err := s.files.InitUpload(r.Context(), userID, files.UploadInitInput{
		FolderID: body.FolderID, Name: body.Name, ExpectedSize: body.ExpectedSizeBytes,
		MimeHint: body.MimeHint, MetaCreatedAt: body.MetaCreatedAt, MetaModifiedAt: body.MetaModifiedAt,
		ConflictPolicy: body.ConflictPolicy, OverwriteFileID: body.FileID,
	})
	if err != nil {
		var cd *files.ConflictDetail
		if errors.As(err, &cd) {
			writeError(w, http.StatusConflict, "CONFLICT", "File already exists.", map[string]any{
				"file_id": cd.Info.FileID, "size_bytes": cd.Info.SizeBytes, "updated_at": cd.Info.UpdatedAt,
			})
			return
		}
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) putUploadContent(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	sid, _ := uuid.Parse(chi.URLParam(r, "id"))
	off := int64(0)
	if h := r.Header.Get("Offset"); h != "" {
		var err error
		off, err = strconv.ParseInt(h, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid Offset header.", nil)
			return
		}
	}
	n, err := s.files.UploadContent(r.Context(), userID, sid, off, r.Body)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"received_bytes": n})
}

func (s *Server) postUploadComplete(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	sid, _ := uuid.Parse(chi.URLParam(r, "id"))
	file, err := s.files.CompleteUpload(r.Context(), userID, sid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) deleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	sid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if err := s.files.AbortUpload(r.Context(), userID, sid); err != nil {
		mapFilesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resolveFileAccess(r *http.Request, fileID uuid.UUID) (uuid.UUID, bool) {
	if mt := r.URL.Query().Get("mt"); mt != "" {
		uid, err := s.tokens.ParseMediaToken(mt, fileID)
		if err != nil {
			return uuid.Nil, false
		}
		return uid, true
	}
	userID, ok, _ := userFromContext(r.Context())
	return userID, ok
}

func (s *Server) getFileContent(w http.ResponseWriter, r *http.Request) {
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid file id.", nil)
		return
	}
	userID, ok := s.resolveFileAccess(r, fid)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	audit := r.URL.Query().Get("mt") == "" && r.Header.Get("Authorization") != ""
	fc, status, err := s.files.ResolveFileContent(r.Context(), userID, fid, r.Header.Get("Range"), audit)
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
	disposition := "attachment"
	if r.URL.Query().Get("mt") != "" && (fc.Mime == "application/pdf" || strings.HasPrefix(fc.Mime, "application/pdf")) {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename*=UTF-8''%s`, disposition, url.PathEscape(fc.Name)))
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fc.Offset, fc.Offset+fc.Length-1, fc.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(fc.Length, 10))
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(fc.Size, 10))
	}
	if err := files.WriteFileContent(w, fc, status, limits); err != nil && !files.IsStreamClientAbort(err) {
		s.log.Error("file stream", "err", err)
	}
}

func (s *Server) postMediaToken(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		return
	}
	fid, _ := uuid.Parse(chi.URLParam(r, "id"))
	if _, err := s.files.GetFile(r.Context(), userID, fid); err != nil {
		mapFilesError(w, err)
		return
	}
	tok, exp, err := s.tokens.IssueMediaToken(userID, fid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "An internal error occurred.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": tok, "expires_at": exp})
}

func mapFilesError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, files.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found.", nil)
	case errors.Is(err, files.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", "Name conflict.", nil)
	case errors.Is(err, files.ErrParentDeleted):
		writeError(w, http.StatusConflict, "PARENT_DELETED", "Parent folder is in trash.", nil)
	case errors.Is(err, files.ErrCycle):
		writeError(w, http.StatusConflict, "CYCLE", "Folder move would create a cycle.", nil)
	case errors.Is(err, files.ErrQuota):
		writeError(w, http.StatusInsufficientStorage, "QUOTA_EXCEEDED", "Storage quota would be exceeded.", nil)
	case errors.Is(err, files.ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", "An internal error occurred.", nil)
	}
}
