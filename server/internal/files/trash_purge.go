package files

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PurgeExpiredTrash permanently deletes trashed items older than settings.trash_retention_days.
func (s *Store) PurgeExpiredTrash(ctx context.Context) (int, error) {
	var days int
	if err := s.Pool.QueryRow(ctx, `SELECT trash_retention_days FROM settings WHERE id = true`).Scan(&days); err != nil || days <= 0 {
		days = 30
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	rows, err := s.Pool.Query(ctx, `
		SELECT 'folder', f.id, f.user_id
		FROM folders f
		WHERE f.is_deleted = true AND f.deleted_at < $1 AND f.parent_id IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM folders p
		    WHERE p.id = f.parent_id AND p.deleted_batch_id = f.deleted_batch_id AND p.is_deleted = true
		  )
		UNION ALL
		SELECT 'file', fi.id, fi.user_id
		FROM files fi
		WHERE fi.is_deleted = true AND fi.deleted_at < $1
		  AND NOT EXISTS (
		    SELECT 1 FROM folders p
		    WHERE p.id = fi.folder_id AND p.is_deleted = true AND p.deleted_batch_id = fi.deleted_batch_id
		  )
	`, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type item struct {
		kind   string
		id     uuid.UUID
		userID uuid.UUID
	}
	var batch []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.kind, &it.id, &it.userID); err != nil {
			return 0, err
		}
		batch = append(batch, it)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	purged := 0
	for _, it := range batch {
		var err error
		if it.kind == "folder" {
			err = s.HardDeleteFolder(ctx, it.userID, it.id)
		} else {
			err = s.HardDeleteFile(ctx, it.userID, it.id)
		}
		if err == nil {
			purged++
		}
	}
	if purged > 0 {
		_ = s.Audit.Write(ctx, nil, "trash.purged", "purged expired trash", nil)
	}
	return purged, nil
}
