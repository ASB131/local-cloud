package files

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
)

func copyDisplayName(name string, attempt int) string {
	var suffix string
	switch {
	case attempt == 0:
		suffix = " - Copy"
	default:
		suffix = fmt.Sprintf(" - Copy (%d)", attempt+1)
	}
	base, ext := splitNameExt(name)
	return base + suffix + ext
}

func splitNameExt(name string) (base, ext string) {
	i := strings.LastIndex(name, ".")
	if i <= 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

func (s *Store) uniqueFileNameInFolder(ctx context.Context, folderID uuid.UUID, desired string) (string, error) {
	if ok, err := s.fileNameTaken(ctx, folderID, desired); err != nil {
		return "", err
	} else if !ok {
		return desired, nil
	}
	for attempt := 0; attempt < 50; attempt++ {
		name := copyDisplayName(desired, attempt)
		ok, err := s.fileNameTaken(ctx, folderID, name)
		if err != nil {
			return "", err
		}
		if !ok {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w: could not allocate copy name", ErrValidation)
}

func (s *Store) uniqueFolderNameUnderParent(ctx context.Context, parentID uuid.UUID, desired string) (string, error) {
	if ok, err := s.folderNameTaken(ctx, parentID, desired); err != nil {
		return "", err
	} else if !ok {
		return desired, nil
	}
	for attempt := 0; attempt < 50; attempt++ {
		name := copyDisplayName(desired, attempt)
		ok, err := s.folderNameTaken(ctx, parentID, name)
		if err != nil {
			return "", err
		}
		if !ok {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w: could not allocate copy name", ErrValidation)
}

func (s *Store) fileNameTaken(ctx context.Context, folderID uuid.UUID, name string) (bool, error) {
	key := names.NameKey(name)
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM files WHERE folder_id = $1 AND name_key = $2 AND is_deleted = false
		)
	`, folderID, key).Scan(&exists)
	return exists, err
}

func (s *Store) folderNameTaken(ctx context.Context, parentID uuid.UUID, name string) (bool, error) {
	key := names.NameKey(name)
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM folders WHERE parent_id = $1 AND name_key = $2 AND is_deleted = false
		)
	`, parentID, key).Scan(&exists)
	return exists, err
}

func (s *Store) CopyFile(ctx context.Context, userID, fileID uuid.UUID, destFolderID *uuid.UUID) (FileDTO, error) {
	var srcFolder uuid.UUID
	var name, diskPath, hash, mime string
	var size int64
	var metaCreated, metaModified *time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT folder_id, name, size_bytes, mime_type, disk_path, file_hash, meta_created_at, meta_modified_at
		FROM files WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&srcFolder, &name, &size, &mime, &diskPath, &hash, &metaCreated, &metaModified)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	destFolder := srcFolder
	if destFolderID != nil {
		destFolder = *destFolderID
	}
	ok, err := s.userOwnsFolder(ctx, userID, destFolder)
	if err != nil {
		return FileDTO{}, err
	}
	if !ok {
		return FileDTO{}, ErrNotFound
	}
	copyName, err := s.uniqueFileNameInFolder(ctx, destFolder, name)
	if err != nil {
		return FileDTO{}, err
	}
	if err := s.checkQuota(ctx, userID, size, 0); err != nil {
		return FileDTO{}, err
	}

	srcAbs, err := s.Layout.AbsFromRel(diskPath)
	if err != nil {
		return FileDTO{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FileDTO{}, err
	}
	defer tx.Rollback(ctx)

	newID, err := uuid.NewV7()
	if err != nil {
		return FileDTO{}, err
	}
	ver := 1
	if err := os.MkdirAll(s.Layout.ObjectDir(userID, newID), 0o750); err != nil {
		return FileDTO{}, err
	}
	destAbs, rel, err := s.Layout.VersionFile(userID, newID, ver)
	if err != nil {
		return FileDTO{}, err
	}
	if err := copyFile(srcAbs, destAbs); err != nil {
		return FileDTO{}, err
	}

	now := time.Now().UTC()
	key := names.NameKey(copyName)
	thumbStatus := "skipped"
	if ThumbnailEligible(mime) {
		thumbStatus = "pending"
	}
	var out FileDTO
	err = tx.QueryRow(ctx, `
		INSERT INTO files (
			id, user_id, folder_id, name, name_key, size_bytes, mime_type, disk_path, file_hash,
			meta_created_at, meta_modified_at, thumbnail_status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)
		RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
			meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
	`, newID, userID, destFolder, copyName, key, size, mime, rel, hash,
		metaCreated, metaModified, thumbStatus, now).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
	)
	if err != nil {
		return FileDTO{}, err
	}
	vid, _ := uuid.NewV7()
	_, err = tx.Exec(ctx, `
		INSERT INTO file_versions (id, file_id, version_num, size_bytes, disk_path, file_hash, created_at)
		VALUES ($1,$2,1,$3,$4,$5,$6)
	`, vid, newID, size, rel, hash, now)
	if err != nil {
		return FileDTO{}, err
	}
	if thumbStatus == "pending" {
		if err := s.enqueueThumb(ctx, tx, newID); err != nil {
			return FileDTO{}, err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE users SET storage_used_bytes = storage_used_bytes + $1, updated_at = $2 WHERE id = $3
	`, size, now, userID)
	if err != nil {
		return FileDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return FileDTO{}, err
	}

	pathSegs, _ := s.folderPathSegments(ctx, userID, destFolder)
	_ = writeMetaSidecar(s.Layout, userID, newID, ver, copyName, pathSegs, size, hash, mime, metaCreated, metaModified, now)
	_ = s.Audit.Write(ctx, &userID, "file.copied", fmt.Sprintf("copied file to folder %s", destFolder.String()), nil)
	s.emit(ctx, userID, "file.created", out)
	return out, nil
}

func (s *Store) CopyFolder(ctx context.Context, userID, folderID uuid.UUID, destParentID *uuid.UUID) (FolderDTO, error) {
	var parentID *uuid.UUID
	var name string
	err := s.Pool.QueryRow(ctx, `
		SELECT parent_id, name FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, folderID, userID).Scan(&parentID, &name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FolderDTO{}, ErrNotFound
		}
		return FolderDTO{}, err
	}
	if parentID == nil {
		return FolderDTO{}, fmt.Errorf("%w: cannot copy root folder", ErrValidation)
	}

	destParent := *parentID
	if destParentID != nil {
		destParent = *destParentID
	}
	if folderID == destParent {
		return FolderDTO{}, fmt.Errorf("%w: cannot copy folder into itself", ErrValidation)
	}
	ok, err := s.userOwnsFolder(ctx, userID, destParent)
	if err != nil {
		return FolderDTO{}, err
	}
	if !ok {
		return FolderDTO{}, ErrNotFound
	}
	cycle, err := s.wouldCycle(ctx, folderID, destParent)
	if err != nil {
		return FolderDTO{}, err
	}
	if cycle {
		return FolderDTO{}, ErrCycle
	}

	copyName, err := s.uniqueFolderNameUnderParent(ctx, destParent, name)
	if err != nil {
		return FolderDTO{}, err
	}
	newRoot, err := s.CreateFolder(ctx, userID, destParent, copyName)
	if err != nil {
		return FolderDTO{}, err
	}
	if err := s.copyFolderContents(ctx, userID, folderID, newRoot.ID); err != nil {
		return FolderDTO{}, err
	}
	return newRoot, nil
}

func (s *Store) copyFolderContents(ctx context.Context, userID, srcFolderID, destFolderID uuid.UUID) error {
	children, err := s.ListChildren(ctx, userID, srcFolderID)
	if err != nil {
		return err
	}
	for _, f := range children.Files {
		dest := destFolderID
		if _, err := s.CopyFile(ctx, userID, f.ID, &dest); err != nil {
			return err
		}
	}
	for _, sub := range children.Folders {
		subCopy, err := s.CopyFolder(ctx, userID, sub.ID, &destFolderID)
		if err != nil {
			return err
		}
		_ = subCopy
	}
	return nil
}
