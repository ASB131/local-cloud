package jobs

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/files"
	"github.com/asb131/local-cloud/internal/storage"
)

const (
	lockTrash    int64 = 910001
	lockVersions int64 = 910002
	lockLogs     int64 = 910003
	lockUsage    int64 = 910004
	lockBackup   int64 = 910005
)

func purgeTrash(ctx context.Context, pool *pgxpool.Pool, store *files.Store, log *slog.Logger) {
	withAdvisoryLock(ctx, pool, lockTrash, func(c context.Context) {
		n, err := store.PurgeExpiredTrash(c)
		if err != nil {
			log.Error("trash purge", "err", err)
			return
		}
		log.Info("trash purge tick", "purged", n)
	})
}

func purgeVersions(ctx context.Context, pool *pgxpool.Pool, layout *storage.Layout, log *slog.Logger) {
	withAdvisoryLock(ctx, pool, lockVersions, func(c context.Context) {
		var days int
		_ = pool.QueryRow(c, `SELECT version_retention_days FROM settings WHERE id = true`).Scan(&days)
		if days <= 0 {
			days = 7
		}
		cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		rows, err := pool.Query(c, `
			SELECT fv.id, fv.file_id, fv.version_num, fv.disk_path FROM file_versions fv
			WHERE fv.created_at < $1
			  AND fv.version_num < (SELECT MAX(version_num) FROM file_versions WHERE file_id = fv.file_id)
			LIMIT 500
		`, cutoff)
		if err != nil {
			log.Error("version purge query", "err", err)
			return
		}
		defer rows.Close()
		pruned := 0
		for rows.Next() {
			var id, fileID uuid.UUID
			var verNum int
			var rel string
			if err := rows.Scan(&id, &fileID, &verNum, &rel); err != nil {
				continue
			}
			if abs, err := layout.AbsFromRel(rel); err == nil {
				_ = os.Remove(abs)
			}
			_, _ = pool.Exec(c, `DELETE FROM file_versions WHERE id = $1`, id)
			pruned++
		}
		if pruned > 0 {
			log.Info("version purge tick", "pruned", pruned)
		}
	})
}

func purgeLogs(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	withAdvisoryLock(ctx, pool, lockLogs, func(c context.Context) {
		var days int
		_ = pool.QueryRow(c, `SELECT log_retention_days FROM settings WHERE id = true`).Scan(&days)
		if days <= 0 {
			days = 30
		}
		cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		tag, err := pool.Exec(c, `DELETE FROM system_logs WHERE created_at < $1`, cutoff)
		if err != nil {
			log.Error("log purge", "err", err)
			return
		}
		log.Info("log purge tick", "deleted", tag.RowsAffected())
	})
}

func reconcileUsage(ctx context.Context, pool *pgxpool.Pool, store *files.Store, log *slog.Logger) {
	withAdvisoryLock(ctx, pool, lockUsage, func(c context.Context) {
		if err := store.ReconcileUsage(c); err != nil {
			log.Error("usage reconcile", "err", err)
			return
		}
		log.Info("usage reconcile tick")
	})
}

func rotateBackup(ctx context.Context, pool *pgxpool.Pool, layout *storage.Layout, log *slog.Logger) {
	withAdvisoryLock(ctx, pool, lockBackup, func(c context.Context) {
		dir := filepath.Join(layout.Root(), "backups")
		_ = os.MkdirAll(dir, 0o750)
		out := filepath.Join(dir, time.Now().UTC().Format("20060102-150405")+".dump")
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			log.Error("pg_dump backup", "err", "DATABASE_URL not set")
			return
		}
		cmd := exec.CommandContext(c, "pg_dump", "-Fc", "-f", out, dbURL)
		if err := cmd.Run(); err != nil {
			log.Error("pg_dump backup", "err", err)
			return
		}
		entries, _ := os.ReadDir(dir)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		if len(entries) > 7 {
			for i := 0; i < len(entries)-7; i++ {
				_ = os.Remove(filepath.Join(dir, entries[i].Name()))
			}
		}
		log.Info("backup tick", "file", out)
	})
}
