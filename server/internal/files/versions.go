package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) RestoreVersion(ctx context.Context, userID, fileID, versionID uuid.UUID) (FileDTO, error) {
	var verNum int
	var disk, hash string
	var size int64
	err := s.Pool.QueryRow(ctx, `
		SELECT version_num, disk_path, file_hash, size_bytes FROM file_versions
		WHERE id = $1 AND file_id = $2
	`, versionID, fileID).Scan(&verNum, &disk, &hash, &size)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	ok, err := s.userOwnsFile(ctx, userID, fileID)
	if err != nil || !ok {
		return FileDTO{}, ErrNotFound
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FileDTO{}, err
	}
	defer tx.Rollback(ctx)

	versionMax, err := s.versionMaxBytes(ctx, tx)
	if err != nil {
		return FileDTO{}, err
	}
	var curVersion int
	var curSize int64
	var curDisk string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE((SELECT MAX(version_num) FROM file_versions WHERE file_id = $1),0), size_bytes, disk_path
		FROM files WHERE id = $1 AND user_id = $2 FOR UPDATE
	`, fileID, userID).Scan(&curVersion, &curSize, &curDisk); err != nil {
		return FileDTO{}, ErrNotFound
	}

	now := time.Now().UTC()
	newVer := curVersion + 1
	destAbs, rel, err := s.Layout.VersionFile(userID, fileID, newVer)
	if err != nil {
		return FileDTO{}, err
	}
	srcAbs, err := s.Layout.AbsFromRel(disk)
	if err != nil {
		return FileDTO{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o750); err != nil {
		return FileDTO{}, err
	}
	if err := copyFile(srcAbs, destAbs); err != nil {
		return FileDTO{}, err
	}

	keepHistory := curSize < versionMax && size < versionMax
	if !keepHistory && curVersion > 0 {
		if abs, err := s.Layout.AbsFromRel(curDisk); err == nil {
			_ = os.Remove(abs)
		}
		_, _ = tx.Exec(ctx, `DELETE FROM file_versions WHERE file_id = $1 AND version_num < $2`, fileID, newVer)
	}
	vid, err := uuid.NewV7()
	if err != nil {
		return FileDTO{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO file_versions (id, file_id, version_num, size_bytes, disk_path, file_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, vid, fileID, newVer, size, rel, hash, now)
	if err != nil {
		return FileDTO{}, err
	}
	var out FileDTO
	err = tx.QueryRow(ctx, `
		UPDATE files SET size_bytes = $1, disk_path = $2, file_hash = $3, updated_at = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
			meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
	`, size, rel, hash, now, fileID, userID).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
	)
	if err != nil {
		return FileDTO{}, err
	}
	delta := size - curSize
	if keepHistory {
		delta = size
	}
	_, err = tx.Exec(ctx, `UPDATE users SET storage_used_bytes = storage_used_bytes + $1 WHERE id = $2`, delta, userID)
	if err != nil {
		return FileDTO{}, err
	}
	if err := s.pruneFileVersions(ctx, tx, userID, fileID, MaxFileVersionsKept); err != nil {
		return FileDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return FileDTO{}, err
	}
	var folderID uuid.UUID
	var name, mime string
	var metaCreated, metaModified *time.Time
	_ = s.Pool.QueryRow(ctx, `
		SELECT folder_id, name, mime_type, meta_created_at, meta_modified_at FROM files WHERE id = $1
	`, fileID).Scan(&folderID, &name, &mime, &metaCreated, &metaModified)
	pathSegs, _ := s.folderPathSegments(ctx, userID, folderID)
	_ = writeMetaSidecar(s.Layout, userID, fileID, newVer, name, pathSegs, size, hash, mime, metaCreated, metaModified, now)
	_ = s.Audit.Write(ctx, &userID, "file.version_restored", fmt.Sprintf("restored version %d as v%d", verNum, newVer), nil)
	s.emit(ctx, userID, "file.updated", out)
	return out, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
