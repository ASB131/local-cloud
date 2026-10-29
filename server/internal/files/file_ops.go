package files

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
)

func (s *Store) GetFile(ctx context.Context, userID, fileID uuid.UUID) (FileDTO, error) {
	var out FileDTO
	err := s.Pool.QueryRow(ctx, `
		SELECT f.id, f.folder_id, f.name, f.size_bytes, f.mime_type, f.file_hash,
		       f.meta_created_at, f.meta_modified_at, f.created_at, f.updated_at, f.thumbnail_status,
		       f.office_preview_status,
		       EXISTS(SELECT 1 FROM favorites fav WHERE fav.user_id = $1 AND fav.file_id = f.id)
		FROM files f WHERE f.id = $2 AND f.user_id = $1 AND f.is_deleted = false
	`, userID, fileID).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
		&out.OfficePreviewStatus, &out.IsFavorite,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	if OfficePreviewEligible(out.Name, out.MimeType, out.SizeBytes) &&
		(out.OfficePreviewStatus == "" || out.OfficePreviewStatus == "none" || out.OfficePreviewStatus == "failed") {
		if err := s.QueueOfficePreviewIfNeeded(ctx, userID, fileID); err == nil {
			out.OfficePreviewStatus = "pending"
		}
	}
	return out, nil
}

func (s *Store) PatchFile(ctx context.Context, userID, fileID uuid.UUID, newName *string, newFolder *uuid.UUID) (FileDTO, error) {
	var folderID uuid.UUID
	var name string
	err := s.Pool.QueryRow(ctx, `
		SELECT folder_id, name FROM files WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&folderID, &name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	if newFolder != nil {
		ok, err := s.userOwnsFolder(ctx, userID, *newFolder)
		if err != nil {
			return FileDTO{}, err
		}
		if !ok {
			return FileDTO{}, ErrNotFound
		}
		folderID = *newFolder
	}
	if newName != nil {
		if err := names.Validate(*newName, false); err != nil {
			return FileDTO{}, ErrValidation
		}
		name = *newName
	}
	key := names.NameKey(name)
	now := time.Now().UTC()
	var out FileDTO
	err = s.Pool.QueryRow(ctx, `
		UPDATE files SET name = $1, name_key = $2, folder_id = $3, updated_at = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
			meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
	`, name, key, folderID, now, fileID, userID).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return FileDTO{}, ErrConflict
		}
		return FileDTO{}, err
	}
	s.emit(ctx, userID, "file.updated", out)
	return out, nil
}

func (s *Store) DeleteFile(ctx context.Context, userID, fileID uuid.UUID) error {
	batch, err := uuid.NewV7()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	tag, err := s.Pool.Exec(ctx, `
		UPDATE files SET is_deleted = true, deleted_at = $1, deleted_batch_id = $2, updated_at = $1
		WHERE id = $3 AND user_id = $4 AND is_deleted = false
	`, now, batch, fileID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.emit(ctx, userID, "file.deleted", map[string]any{"id": fileID})
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return nil
}

func (s *Store) ListVersions(ctx context.Context, userID, fileID uuid.UUID) ([]VersionDTO, error) {
	ok, err := s.userOwnsFile(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, version_num, size_bytes, file_hash, created_at
		FROM file_versions WHERE file_id = $1 ORDER BY version_num DESC
		LIMIT $2
	`, fileID, MaxFileVersionsKept)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VersionDTO
	for rows.Next() {
		var v VersionDTO
		if err := rows.Scan(&v.ID, &v.VersionNum, &v.SizeBytes, &v.FileHash, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
