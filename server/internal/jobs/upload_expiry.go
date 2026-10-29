package jobs

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/storage"
)

func RunUploadExpiry(ctx context.Context, pool *pgxpool.Pool, layout *storage.Layout, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expireUploads(ctx, pool, layout)
		}
	}
}

func expireUploads(ctx context.Context, pool *pgxpool.Pool, layout *storage.Layout) {
	rows, err := pool.Query(ctx, `
		SELECT id, temp_path FROM upload_sessions
		WHERE status = 'open' AND expires_at < now()
	`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var temp string
		if err := rows.Scan(&id, &temp); err != nil {
			continue
		}
		if abs, err := layout.AbsFromRel(temp); err == nil {
			_ = os.Remove(abs)
		}
		_, _ = pool.Exec(ctx, `UPDATE upload_sessions SET status = 'expired' WHERE id = $1`, id)
	}
}
