package admin

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Stats struct {
	UserCount          int64 `json:"user_count"`
	TotalUsedBytes     int64 `json:"total_used_bytes"`
	TotalQuotaBytes    int64 `json:"total_quota_bytes"`
	OpenUploadSessions int64 `json:"open_upload_sessions"`
}

func GetStats(ctx context.Context, pool *pgxpool.Pool) (Stats, error) {
	var s Stats
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.UserCount)
	if err != nil {
		return Stats{}, err
	}
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(storage_used_bytes),0), COALESCE(SUM(storage_quota_bytes),0) FROM users
	`).Scan(&s.TotalUsedBytes, &s.TotalQuotaBytes)
	if err != nil {
		return Stats{}, err
	}
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM upload_sessions WHERE status = 'open'`).Scan(&s.OpenUploadSessions)
	return s, err
}
