package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func withAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, key int64, fn func(context.Context)) {
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&ok); err != nil || !ok {
		return
	}
	defer func() { _, _ = pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, key) }()
	fn(ctx)
}
