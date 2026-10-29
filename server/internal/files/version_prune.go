package files

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const MaxFileVersionsKept = 10

func (s *Store) pruneFileVersions(ctx context.Context, tx pgx.Tx, userID, fileID uuid.UUID, keep int) error {
	if keep <= 0 {
		keep = MaxFileVersionsKept
	}
	var currentDisk string
	if err := tx.QueryRow(ctx, `SELECT disk_path FROM files WHERE id = $1 AND user_id = $2`, fileID, userID).Scan(&currentDisk); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		SELECT id, disk_path, size_bytes FROM file_versions
		WHERE file_id = $1
		ORDER BY version_num DESC
		OFFSET $2
	`, fileID, keep)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var rel string
		var size int64
		if err := rows.Scan(&id, &rel, &size); err != nil {
			return err
		}
		if rel == currentDisk {
			_, _ = tx.Exec(ctx, `DELETE FROM file_versions WHERE id = $1`, id)
			continue
		}
		if abs, err := s.Layout.AbsFromRel(rel); err == nil {
			_ = os.Remove(abs)
		}
		_, _ = tx.Exec(ctx, `DELETE FROM file_versions WHERE id = $1`, id)
		if size > 0 {
			_, _ = tx.Exec(ctx, `
				UPDATE users SET storage_used_bytes = GREATEST(0, storage_used_bytes - $1), updated_at = NOW()
				WHERE id = $2
			`, size, userID)
		}
	}
	return rows.Err()
}
