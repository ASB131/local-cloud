package files

import (
	"mime"
	"net/http"
	"strings"

	"github.com/asb131/local-cloud/internal/names"
)

func DetectMIME(name string, head []byte) string {
	typ := http.DetectContentType(head)
	if typ != "application/octet-stream" {
		return typ
	}
	ext := names.ExtLower(name)
	switch ext {
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "mov":
		return "video/quicktime"
	case "mkv":
		return "video/x-matroska"
	default:
		if byExt := mime.TypeByExtension("." + ext); byExt != "" {
			return byExt
		}
	}
	return typ
}

func ThumbnailEligible(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/png", "image/gif", "image/webp",
		"video/mp4", "video/webm", "video/quicktime", "video/x-matroska":
		return true
	default:
		return false
	}
}

func ZipStoreMIME(mimeType string) bool {
	m := strings.ToLower(mimeType)
	switch {
	case strings.HasPrefix(m, "image/jpeg"), strings.HasPrefix(m, "image/png"),
		strings.HasPrefix(m, "video/"), strings.Contains(m, "zip"),
		strings.Contains(m, "gzip"), strings.Contains(m, "compressed"):
		return true
	default:
		return false
	}
}
