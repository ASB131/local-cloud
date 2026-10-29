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

func (s *Server) getFileOfficePreview(w http.ResponseWriter, r *http.Request) {
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
	fc, err := s.files.ResolveOfficePreviewContent(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	limits, _ := s.files.LoadSpeedLimits(r.Context(), userID)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", fc.Mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename*=UTF-8''%s`, url.PathEscape(fc.Name)))
	w.Header().Set("Content-Length", strconv.FormatInt(fc.Size, 10))
	if err := files.WriteFileContent(w, fc, http.StatusOK, limits); err != nil && !files.IsStreamClientAbort(err) {
		s.log.Error("office preview stream", "err", err)
	}
}
