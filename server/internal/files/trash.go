package files

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
)

func (s *Store) ListTrash(ctx context.Context, userID uuid.UUID) ([]TrashItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT 'folder', f.id, f.name, NULL::bigint, f.deleted_at
		FROM folders f
		WHERE f.user_id = $1 AND f.is_deleted = true AND f.parent_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM folders p
		    WHERE p.id = f.parent_id AND p.deleted_batch_id = f.deleted_batch_id AND p.is_deleted = true
		  )
		UNION ALL
		SELECT 'file', fi.id, fi.name, fi.size_bytes, fi.deleted_at
		FROM files fi
		WHERE fi.user_id = $1 AND fi.is_deleted = true
		  AND NOT EXISTS (
		    SELECT 1 FROM folders p
		    WHERE p.id = fi.folder_id AND p.is_deleted = true AND p.deleted_batch_id = fi.deleted_batch_id
		  )
		ORDER BY deleted_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TrashItem
	for rows.Next() {
		var it TrashItem
		if err := rows.Scan(&it.Kind, &it.ID, &it.Name, &it.SizeBytes, &it.DeletedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = []TrashItem{}
	}
	return items, rows.Err()
}

func (s *Store) RestoreFile(ctx context.Context, userID, fileID uuid.UUID, destFolder *uuid.UUID, newName *string, policy string, overwriteID *uuid.UUID) (FileDTO, error) {
	var folderID uuid.UUID
	var name string
	var batch *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT folder_id, name, deleted_batch_id FROM files
		WHERE id = $1 AND user_id = $2 AND is_deleted = true
	`, fileID, userID).Scan(&folderID, &name, &batch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	targetFolder := folderID
	if destFolder != nil {
		targetFolder = *destFolder
	}
	var parentDeleted bool
	err = s.Pool.QueryRow(ctx, `
		SELECT is_deleted FROM folders WHERE id = $1 AND user_id = $2
	`, targetFolder, userID).Scan(&parentDeleted)
	if err != nil {
		return FileDTO{}, ErrNotFound
	}
	if parentDeleted {
		return FileDTO{}, ErrParentDeleted
	}
	if newName != nil {
		name = *newName
	}
	key := names.NameKey(name)
	var conflictID uuid.UUID
	err = s.Pool.QueryRow(ctx, `
		SELECT id FROM files WHERE folder_id = $1 AND name_key = $2 AND is_deleted = false
	`, targetFolder, key).Scan(&conflictID)
	if err == nil && policy != "overwrite" {
		return FileDTO{}, ErrConflict
	}
	if policy == "overwrite" && overwriteID != nil && *overwriteID != conflictID {
		return FileDTO{}, ErrNotFound
	}
	now := time.Now().UTC()
	var out FileDTO
	err = s.Pool.QueryRow(ctx, `
		UPDATE files SET is_deleted = false, deleted_at = NULL, deleted_batch_id = NULL,
			folder_id = $1, name = $2, name_key = $3, updated_at = $4
		WHERE id = $5
		RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
			meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
	`, targetFolder, name, key, now, fileID).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return FileDTO{}, ErrConflict
		}
		return FileDTO{}, err
	}
	s.emit(ctx, userID, "file.created", out)
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return out, nil
}

func (s *Store) HardDeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	var disk string
	var thumb *string
	var size int64
	err := s.Pool.QueryRow(ctx, `
		SELECT disk_path, thumbnail_path, size_bytes FROM files
		WHERE id = $1 AND user_id = $2 AND is_deleted = true
	`, fileID, userID).Scan(&disk, &thumb, &size)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `DELETE FROM file_versions WHERE file_id = $1`, fileID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM files WHERE id = $1`, fileID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET storage_used_bytes = GREATEST(0, storage_used_bytes - $1) WHERE id = $2`, size, userID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = os.RemoveAll(s.Layout.ObjectDir(userID, fileID))
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return nil
}

func (s *Store) HardDeleteFolder(ctx context.Context, userID, folderID uuid.UUID) error {
	var batch *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT deleted_batch_id FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = true
	`, folderID, userID).Scan(&batch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, size_bytes FROM files WHERE user_id = $1 AND deleted_batch_id = $2
	`, userID, batch)
	if err != nil {
		return err
	}
	defer rows.Close()
	type frec struct {
		id   uuid.UUID
		size int64
	}
	var recs []frec
	var total int64
	for rows.Next() {
		var r frec
		if err := rows.Scan(&r.id, &r.size); err != nil {
			return err
		}
		recs = append(recs, r)
		total += r.size
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `DELETE FROM files WHERE user_id = $1 AND deleted_batch_id = $2`, userID, batch)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM folders WHERE user_id = $1 AND deleted_batch_id = $2`, userID, batch)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET storage_used_bytes = GREATEST(0, storage_used_bytes - $1) WHERE id = $2`, total, userID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, r := range recs {
		_ = os.RemoveAll(s.Layout.ObjectDir(userID, r.id))
	}
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return nil
}

func (s *Store) RestoreFolder(ctx context.Context, userID, folderID uuid.UUID, destFolder *uuid.UUID) error {
	var batch *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT deleted_batch_id FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = true
	`, folderID, userID).Scan(&batch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	now := time.Now().UTC()
	_, err = s.Pool.Exec(ctx, `
		UPDATE folders SET is_deleted = false, deleted_at = NULL, deleted_batch_id = NULL, updated_at = $1
		WHERE user_id = $2 AND deleted_batch_id = $3
	`, now, userID, batch)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		UPDATE files SET is_deleted = false, deleted_at = NULL, deleted_batch_id = NULL, updated_at = $1
		WHERE user_id = $2 AND deleted_batch_id = $3
	`, now, userID, batch)
	if err != nil {
		return err
	}
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return nil
}
