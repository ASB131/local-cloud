package files

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListFavorites(ctx context.Context, userID uuid.UUID) ([]FavoriteDTO, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT fav.id, fav.file_id, fav.folder_id, fav.created_at
		FROM favorites fav
		LEFT JOIN files fi ON fi.id = fav.file_id
		LEFT JOIN folders fo ON fo.id = fav.folder_id
		WHERE fav.user_id = $1
		  AND (fi.id IS NULL OR fi.is_deleted = false)
		  AND (fo.id IS NULL OR fo.is_deleted = false)
		ORDER BY fav.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FavoriteDTO
	for rows.Next() {
		var f FavoriteDTO
		if err := rows.Scan(&f.ID, &f.FileID, &f.FolderID, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if out == nil {
		out = []FavoriteDTO{}
	}
	return out, rows.Err()
}

func (s *Store) AddFavorite(ctx context.Context, userID uuid.UUID, fileID, folderID *uuid.UUID) (FavoriteDTO, error) {
	if (fileID == nil) == (folderID == nil) {
		return FavoriteDTO{}, ErrValidation
	}
	id, err := uuid.NewV7()
	if err != nil {
		return FavoriteDTO{}, err
	}
	var out FavoriteDTO
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO favorites (id, user_id, file_id, folder_id, created_at)
		VALUES ($1,$2,$3,$4,now())
		RETURNING id, file_id, folder_id, created_at
	`, id, userID, fileID, folderID).Scan(&out.ID, &out.FileID, &out.FolderID, &out.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return FavoriteDTO{}, ErrConflict
		}
		return FavoriteDTO{}, err
	}
	s.emit(ctx, userID, "favorite.changed", out)
	return out, nil
}

func (s *Store) RemoveFavorite(ctx context.Context, userID, favoriteID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM favorites WHERE id = $1 AND user_id = $2`, favoriteID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	s.emit(ctx, userID, "favorite.changed", map[string]any{})
	return nil
}

func (s *Store) GetThumbnailPath(ctx context.Context, userID, fileID uuid.UUID) (string, string, error) {
	var status string
	var thumb *string
	err := s.Pool.QueryRow(ctx, `
		SELECT thumbnail_status, thumbnail_path FROM files
		WHERE id = $1 AND user_id = $2 AND is_deleted = false
	`, fileID, userID).Scan(&status, &thumb)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	if status != "ready" || thumb == nil {
		return "", status, ErrNotFound
	}
	return *thumb, status, nil
}
