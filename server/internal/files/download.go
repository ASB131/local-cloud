package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/transfer"
)

type FileContent struct {
	Name     string
	Size     int64
	Mime     string
	Offset   int64
	Length   int64
	FullFile bool
	AbsPath  string
}

func (s *Store) ResolveFileContent(ctx context.Context, userID, fileID uuid.UUID, rangeHeader string, auditFullDownload bool) (FileContent, int, error) {
	var name, disk, mime string
	var size int64
	err := s.Pool.QueryRow(ctx, `
		SELECT name, size_bytes, mime_type, disk_path FROM files
		WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&name, &size, &mime, &disk)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileContent{}, 0, ErrNotFound
		}
		return FileContent{}, 0, err
	}
	abs, err := s.Layout.AbsFromRel(disk)
	if err != nil {
		return FileContent{}, 0, err
	}

	start, end, hasRange, badRange := parseRange(rangeHeader, size)
	if badRange {
		return FileContent{}, http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("bad range")
	}
	if auditFullDownload && !hasRange {
		_ = s.Audit.Write(ctx, &userID, "file.downloaded", fmt.Sprintf("downloaded %d bytes", size), nil)
	}
	if auditFullDownload && hasRange && start == 0 && end == size-1 {
		_ = s.Audit.Write(ctx, &userID, "file.downloaded", fmt.Sprintf("downloaded %d bytes", size), nil)
	}

	fc := FileContent{Name: name, Size: size, Mime: mime, AbsPath: abs}
	if !hasRange {
		fc.FullFile = true
		fc.Offset = 0
		fc.Length = size
		return fc, http.StatusOK, nil
	}
	fc.Offset = start
	fc.Length = end - start + 1
	return fc, http.StatusPartialContent, nil
}

func (s *Store) ResolveVersionContent(ctx context.Context, userID, fileID, versionID uuid.UUID, rangeHeader string) (FileContent, int, error) {
	ok, err := s.userOwnsFile(ctx, userID, fileID)
	if err != nil || !ok {
		return FileContent{}, 0, ErrNotFound
	}
	var name, mime string
	var size int64
	var disk string
	err = s.Pool.QueryRow(ctx, `
		SELECT f.name, fv.size_bytes, f.mime_type, fv.disk_path
		FROM file_versions fv
		JOIN files f ON f.id = fv.file_id AND f.user_id = $1
		WHERE fv.id = $2 AND fv.file_id = $3
	`, userID, versionID, fileID).Scan(&name, &size, &mime, &disk)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileContent{}, 0, ErrNotFound
		}
		return FileContent{}, 0, err
	}
	abs, err := s.Layout.AbsFromRel(disk)
	if err != nil {
		return FileContent{}, 0, err
	}
	start, end, hasRange, badRange := parseRange(rangeHeader, size)
	if badRange {
		return FileContent{}, http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("bad range")
	}
	fc := FileContent{Name: name, Size: size, Mime: mime, AbsPath: abs}
	if !hasRange {
		fc.FullFile = true
		fc.Offset = 0
		fc.Length = size
		return fc, http.StatusOK, nil
	}
	fc.Offset = start
	fc.Length = end - start + 1
	return fc, http.StatusPartialContent, nil
}

func parseRange(h string, size int64) (start, end int64, has bool, bad bool) {
	if h == "" {
	 return 0, 0, false, false
	}
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false, true
	}
	spec := strings.TrimPrefix(h, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false, true
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, false, true
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false, true
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false, true
	}
	if parts[1] == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false, true
		}
	}
	if start >= size {
		return 0, 0, false, true
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true, false
}

func WriteFileContent(w http.ResponseWriter, fc FileContent, status int, limits transfer.SpeedLimits) error {
	f, err := os.Open(fc.AbsPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(fc.Offset, io.SeekStart); err != nil {
		return err
	}
	var reader io.Reader = io.LimitReader(f, fc.Length)
	if kbps := transfer.EffectiveLimitKbps(limits); kbps != nil {
		reader = transfer.ThrottleReader(reader, *kbps)
	}
	w.WriteHeader(status)
	_, err = io.Copy(w, reader)
	return err
}
