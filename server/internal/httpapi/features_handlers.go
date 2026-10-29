package httpapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) getClientFeatures(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	_ = userID
	writeJSON(w, http.StatusOK, map[string]any{
		"onlyoffice_public_url": s.cfg.OnlyOfficePublicURL,
		"tls_enabled":           s.cfg.TLSCertFile != "",
	})
}

func (s *Server) getOnlyOfficeConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok, _ := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.", nil)
		return
	}
	if s.cfg.OnlyOfficePublicURL == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "OnlyOffice is not configured.", nil)
		return
	}
	fid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION", "Invalid file id.", nil)
		return
	}
	file, err := s.files.GetFile(r.Context(), userID, fid)
	if err != nil {
		mapFilesError(w, err)
		return
	}
	mt, _, err := s.tokens.IssueMediaToken(userID, fid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Could not issue media token.", nil)
		return
	}
	base := s.cfg.InternalBaseURL
	if base == "" {
		base = fmt.Sprintf("http://127.0.0.1%s", s.cfg.HTTPAddr)
	}
	docURL := fmt.Sprintf("%s/api/v1/files/%s/content?mt=%s", base, fid, mt)
	ext := fileExt(file.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"document_server_url": s.cfg.OnlyOfficePublicURL,
		"document": map[string]any{
			"fileType": ext,
			"key":      fmt.Sprintf("%s-%d", fid, file.UpdatedAt.Unix()),
			"title":    file.Name,
			"url":      docURL,
		},
		"document_type": onlyOfficeDocType(ext),
	})
}

func fileExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return ""
}

func onlyOfficeDocType(ext string) string {
	switch ext {
	case "xls", "xlsx", "ods", "csv":
		return "cell"
	case "ppt", "pptx", "odp":
		return "slide"
	default:
		return "word"
	}
}
