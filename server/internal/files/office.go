package files

import (
	"context"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const MaxOfficePreviewBytes = 100 * 1024 * 1024

func OfficePreviewEligible(name, mimeType string, sizeBytes int64) bool {
	if sizeBytes <= 0 || sizeBytes > MaxOfficePreviewBytes {
		return false
	}
	ext := strings.ToLower(name)
	if i := strings.LastIndex(ext, "."); i >= 0 {
		ext = ext[i+1:]
	} else {
		ext = ""
	}
	switch ext {
	case "doc", "docx", "odt", "rtf", "xls", "xlsx", "ods", "ppt", "pptx", "odp":
		return true
	}
	m := strings.ToLower(mimeType)
	return strings.Contains(m, "word") ||
		strings.Contains(m, "excel") ||
		strings.Contains(m, "spreadsheet") ||
		strings.Contains(m, "powerpoint") ||
		strings.Contains(m, "presentation") ||
		m == "application/rtf" ||
		m == "application/vnd.oasis.opendocument.text" ||
		m == "application/vnd.oasis.opendocument.spreadsheet" ||
		m == "application/vnd.oasis.opendocument.presentation"
}

func (s *Store) QueueOfficePreviewIfNeeded(ctx context.Context, userID, fileID uuid.UUID) error {
	var name, mime, status string
	var size int64
	err := s.Pool.QueryRow(ctx, `
		SELECT name, size_bytes, mime_type, office_preview_status
		FROM files WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&name, &size, &mime, &status)
	if err != nil {
		return err
	}
	if !OfficePreviewEligible(name, mime, size) {
		return nil
	}
	if status == "pending" || status == "ready" {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE files SET office_preview_status = 'pending', updated_at = now() WHERE id = $1`, fileID); err != nil {
		return err
	}
	if err := s.enqueueOfficePreview(ctx, tx, fileID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) enqueueOfficePreview(ctx context.Context, tx pgx.Tx, fileID uuid.UUID) error {
	jobID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO office_preview_jobs (id, file_id, status, available_at, created_at, updated_at)
		VALUES ($1, $2, 'pending', now(), now(), now())
	`, jobID, fileID)
	return err
}

func (s *Store) ResolveOfficePreviewContent(ctx context.Context, userID, fileID uuid.UUID) (FileContent, error) {
	var name, disk, status string
	var size int64
	err := s.Pool.QueryRow(ctx, `
		SELECT name, size_bytes, office_preview_path, office_preview_status
		FROM files WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&name, &size, &disk, &status)
	if err != nil {
		return FileContent{}, ErrNotFound
	}
	if status != "ready" || disk == "" {
		return FileContent{}, ErrNotFound
	}
	abs, err := s.Layout.AbsFromRel(disk)
	if err != nil {
		return FileContent{}, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return FileContent{}, err
	}
	pdfName := name
	if dot := strings.LastIndex(pdfName, "."); dot > 0 {
		pdfName = pdfName[:dot] + ".pdf"
	} else {
		pdfName = pdfName + ".pdf"
	}
	pdfSize := fi.Size()
	_ = size
	return FileContent{
		Name:     pdfName,
		Size:     pdfSize,
		Mime:     "application/pdf",
		Offset:   0,
		Length:   pdfSize,
		FullFile: true,
		AbsPath:  abs,
	}, nil
}
