package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/files"
	"github.com/asb131/local-cloud/internal/storage"
)

// RunScheduler starts periodic maintenance jobs (single-process deployment).
func RunScheduler(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool, layout *storage.Layout, store *files.Store) {
	go RunUploadExpiry(ctx, pool, layout, 15*time.Minute)
	go runDailyAt(ctx, log, 2, 15, func(c context.Context) { purgeTrash(c, pool, store, log) })
	go runDailyAt(ctx, log, 2, 30, func(c context.Context) { purgeVersions(c, pool, layout, log) })
	go runDailyAt(ctx, log, 2, 45, func(c context.Context) { purgeLogs(c, pool, log) })
	go runDailyAt(ctx, log, 3, 0, func(c context.Context) { reconcileUsage(c, pool, store, log) })
	go runDailyAt(ctx, log, 3, 15, func(c context.Context) { rotateBackup(c, pool, layout, log) })
}

func runDailyAt(ctx context.Context, log *slog.Logger, hour, min int, fn func(context.Context)) {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		wait := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			wait.Stop()
			return
		case <-wait.C:
			fn(ctx)
		}
	}
}
