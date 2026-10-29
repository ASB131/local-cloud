package files

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
)

const maxFolderDepth = 40
const maxFoldersPerListing = 5000

func (s *Store) GetFolder(ctx context.Context, userID, folderID uuid.UUID) (FolderDTO, []FolderDTO, error) {
	var f FolderDTO
	var parentID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT id, parent_id, name, created_at, updated_at
		FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, folderID, userID).Scan(&f.ID, &parentID, &f.Name, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FolderDTO{}, nil, ErrNotFound
		}
		return FolderDTO{}, nil, err
	}
	f.ParentID = parentID

	var crumbs []FolderDTO
	cur := folderID
	for {
		var row FolderDTO
		var pid *uuid.UUID
		err := s.Pool.QueryRow(ctx, `
			SELECT id, parent_id, name, created_at, updated_at
			FROM folders WHERE id = $1 AND user_id = $2
		`, cur, userID).Scan(&row.ID, &pid, &row.Name, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			return FolderDTO{}, nil, err
		}
		row.ParentID = pid
		crumbs = append([]FolderDTO{row}, crumbs...)
		if pid == nil {
			break
		}
		cur = *pid
	}
	return f, crumbs, nil
}

func (s *Store) ListChildren(ctx context.Context, userID, folderID uuid.UUID) (ChildrenResponse, error) {
	ok, err := s.userOwnsFolder(ctx, userID, folderID)
	if err != nil {
		return ChildrenResponse{}, err
	}
	if !ok {
		return ChildrenResponse{}, ErrNotFound
	}

	frows, err := s.Pool.Query(ctx, `
		SELECT id, parent_id, name, created_at, updated_at
		FROM folders WHERE user_id = $1 AND parent_id = $2 AND is_deleted = false
		ORDER BY name_key ASC
	`, userID, folderID)
	if err != nil {
		return ChildrenResponse{}, err
	}
	defer frows.Close()
	var folders []FolderDTO
	for frows.Next() {
		var f FolderDTO
		var pid uuid.UUID
		if err := frows.Scan(&f.ID, &pid, &f.Name, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return ChildrenResponse{}, err
		}
		f.ParentID = &pid
		folders = append(folders, f)
		if len(folders) > maxFoldersPerListing {
			return ChildrenResponse{}, fmt.Errorf("too many folders")
		}
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT f.id, f.folder_id, f.name, f.size_bytes, f.mime_type, f.file_hash,
		       f.meta_created_at, f.meta_modified_at, f.created_at, f.updated_at,
		       f.thumbnail_status,
		       EXISTS(SELECT 1 FROM favorites fav WHERE fav.user_id = $1 AND fav.file_id = f.id)
		FROM files f
		WHERE f.user_id = $1 AND f.folder_id = $2 AND f.is_deleted = false
		ORDER BY f.name_key ASC
	`, userID, folderID)
	if err != nil {
		return ChildrenResponse{}, err
	}
	defer rows.Close()
	var files []FileDTO
	for rows.Next() {
		var file FileDTO
		if err := rows.Scan(
			&file.ID, &file.FolderID, &file.Name, &file.SizeBytes, &file.MimeType, &file.FileHash,
			&file.MetaCreatedAt, &file.MetaModifiedAt, &file.CreatedAt, &file.UpdatedAt,
			&file.ThumbnailStatus, &file.IsFavorite,
		); err != nil {
			return ChildrenResponse{}, err
		}
		files = append(files, file)
	}
	if folders == nil {
		folders = []FolderDTO{}
	}
	if files == nil {
		files = []FileDTO{}
	}
	return ChildrenResponse{Folders: folders, Files: files}, nil
}

func (s *Store) CreateFolder(ctx context.Context, userID, parentID uuid.UUID, name string) (FolderDTO, error) {
	if err := names.Validate(name, false); err != nil {
		return FolderDTO{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	ok, err := s.userOwnsFolder(ctx, userID, parentID)
	if err != nil {
		return FolderDTO{}, err
	}
	if !ok {
		return FolderDTO{}, ErrNotFound
	}
	depth, err := s.folderDepth(ctx, userID, parentID)
	if err != nil {
		return FolderDTO{}, err
	}
	if depth >= maxFolderDepth {
		return FolderDTO{}, fmt.Errorf("%w: max folder depth", ErrValidation)
	}

	key := names.NameKey(name)
	id, err := uuid.NewV7()
	if err != nil {
		return FolderDTO{}, err
	}
	now := time.Now().UTC()
	var f FolderDTO
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO folders (id, user_id, parent_id, name, name_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, parent_id, name, created_at, updated_at
	`, id, userID, parentID, name, key, now).Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return FolderDTO{}, ErrConflict
		}
		return FolderDTO{}, err
	}
	s.emit(ctx, userID, "folder.created", f)
	return f, nil
}

func (s *Store) PatchFolder(ctx context.Context, userID, folderID uuid.UUID, newName *string, newParent *uuid.UUID) (FolderDTO, error) {
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
		return FolderDTO{}, fmt.Errorf("%w: root folder cannot be modified", ErrValidation)
	}

	if newParent != nil {
		if *newParent == folderID {
			return FolderDTO{}, ErrCycle
		}
		ok, err := s.userOwnsFolder(ctx, userID, *newParent)
		if err != nil {
			return FolderDTO{}, err
		}
		if !ok {
			return FolderDTO{}, ErrNotFound
		}
		cycle, err := s.wouldCycle(ctx, folderID, *newParent)
		if err != nil {
			return FolderDTO{}, err
		}
		if cycle {
			return FolderDTO{}, ErrCycle
		}
	}
	if newName != nil {
		if err := names.Validate(*newName, false); err != nil {
			return FolderDTO{}, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}

	now := time.Now().UTC()
	if newName != nil {
		name = *newName
	}
	parent := parentID
	if newParent != nil {
		parent = newParent
	}
	key := names.NameKey(name)

	var f FolderDTO
	err = s.Pool.QueryRow(ctx, `
		UPDATE folders SET
			name = $1, name_key = $2, parent_id = $3, updated_at = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, parent_id, name, created_at, updated_at
	`, name, key, parent, now, folderID, userID).Scan(&f.ID, &f.ParentID, &f.Name, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return FolderDTO{}, ErrConflict
		}
		return FolderDTO{}, err
	}
	s.emit(ctx, userID, "folder.updated", f)
	return f, nil
}

func (s *Store) DeleteFolder(ctx context.Context, userID, folderID uuid.UUID) error {
	var parentID *uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT parent_id FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, folderID, userID).Scan(&parentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
		 return ErrNotFound
		}
		return err
	}
	if parentID == nil {
		return fmt.Errorf("%w: root folder cannot be deleted", ErrValidation)
	}
	batch, err := uuid.NewV7()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false
			UNION ALL
			SELECT f.id FROM folders f JOIN subtree s ON f.parent_id = s.id
			WHERE f.user_id = $2 AND f.is_deleted = false
		)
		UPDATE folders SET is_deleted = true, deleted_at = $3, deleted_batch_id = $4, updated_at = $3
		WHERE id IN (SELECT id FROM subtree)
	`, folderID, userID, now, batch)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE files SET is_deleted = true, deleted_at = $1, deleted_batch_id = $2, updated_at = $1
		WHERE user_id = $3 AND folder_id IN (
			WITH RECURSIVE subtree AS (
				SELECT id FROM folders WHERE id = $4
				UNION ALL
				SELECT f.id FROM folders f JOIN subtree s ON f.parent_id = s.id
			)
			SELECT id FROM subtree
		) AND is_deleted = false
	`, now, batch, userID, folderID)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.emit(ctx, userID, "folder.deleted", map[string]any{"id": folderID})
	s.emit(ctx, userID, "trash.changed", map[string]any{})
	return nil
}

func (s *Store) folderDepth(ctx context.Context, userID, folderID uuid.UUID) (int, error) {
	depth := 0
	cur := folderID
	for {
		var parent *uuid.UUID
		err := s.Pool.QueryRow(ctx, `
			SELECT parent_id FROM folders WHERE id = $1 AND user_id = $2
		`, cur, userID).Scan(&parent)
		if err != nil {
			return 0, err
		}
		if parent == nil {
			return depth, nil
		}
		depth++
		cur = *parent
	}
}

func (s *Store) wouldCycle(ctx context.Context, folderID, newParent uuid.UUID) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id FROM folders WHERE id = $1
			UNION ALL
			SELECT f.id, f.parent_id FROM folders f JOIN up ON f.id = up.parent_id
		)
		SELECT EXISTS(SELECT 1 FROM up WHERE id = $2)
	`, newParent, folderID).Scan(&exists)
	return exists, err
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "unique") || contains(err.Error(), "23505"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
