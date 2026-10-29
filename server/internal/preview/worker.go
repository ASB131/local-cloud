package preview

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/storage"
	"github.com/asb131/local-cloud/internal/ws"
)

type Worker struct {
	Pool   *pgxpool.Pool
	Layout *storage.Layout
	Hub    *ws.Hub
	Log    *slog.Logger
}

func (w *Worker) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if !w.processOne(ctx) {
			time.Sleep(time.Second)
		}
	}
}

func (w *Worker) processOne(ctx context.Context) bool {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)

	var jobID, fileID uuid.UUID
	var attempts int
	err = tx.QueryRow(ctx, `
		SELECT id, file_id, attempts FROM thumbnail_jobs
		WHERE status = 'pending' AND available_at <= now()
		ORDER BY available_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED
	`).Scan(&jobID, &fileID, &attempts)
	if err != nil {
		return false
	}
	_, _ = tx.Exec(ctx, `UPDATE thumbnail_jobs SET status = 'running', updated_at = now() WHERE id = $1`, jobID)

	var userID uuid.UUID
	var disk, mime string
	err = tx.QueryRow(ctx, `SELECT user_id, disk_path, mime_type FROM files WHERE id = $1`, fileID).Scan(&userID, &disk, &mime)
	if err != nil {
		_, _ = tx.Exec(ctx, `UPDATE thumbnail_jobs SET status = 'failed', updated_at = now() WHERE id = $1`, jobID)
		_ = tx.Commit(ctx)
		return true
	}
	_ = tx.Commit(ctx)

	srcAbs, _ := w.Layout.AbsFromRel(disk)
	thumbAbs, thumbRel, _ := w.Layout.ThumbFile(userID, fileID)
	_ = os.MkdirAll(filepath.Dir(thumbAbs), 0o750)

	var cmd *exec.Cmd
	if len(mime) >= 6 && mime[:6] == "video/" {
		cmd = exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "1", "-i", srcAbs, "-frames:v", "1", "-vf", "scale=480:-1", thumbAbs)
	} else {
		cmd = exec.CommandContext(ctx, "ffmpeg", "-y", "-i", srcAbs, "-vf", "scale=480:-1", thumbAbs)
	}
	if err := cmd.Run(); err != nil {
		w.failJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}

	_, err = w.Pool.Exec(ctx, `
		UPDATE files SET thumbnail_status = 'ready', thumbnail_path = $1, updated_at = now() WHERE id = $2
	`, thumbRel, fileID)
	if err != nil {
		return true
	}
	_, _ = w.Pool.Exec(ctx, `UPDATE thumbnail_jobs SET status = 'done', updated_at = now() WHERE id = $1`, jobID)
	if w.Hub != nil {
		w.Hub.Broadcast(userID, map[string]any{
			"type":    "file.thumbnail_ready",
			"payload": map[string]any{"id": fileID, "thumbnail_status": "ready"},
		})
	}
	return true
}

func (w *Worker) failJob(ctx context.Context, jobID, fileID uuid.UUID, attempts int, msg string) {
	attempts++
	delays := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}
	status := "pending"
	available := time.Now().UTC().Add(delays[0])
	if attempts >= 3 {
		status = "failed"
		_, _ = w.Pool.Exec(ctx, `UPDATE files SET thumbnail_status = 'failed', updated_at = now() WHERE id = $1`, fileID)
	} else if attempts-1 < len(delays) {
		available = time.Now().UTC().Add(delays[attempts-1])
	}
	_, _ = w.Pool.Exec(ctx, `
		UPDATE thumbnail_jobs SET status = $1, attempts = $2, available_at = $3, last_error = $4, updated_at = now()
		WHERE id = $5
	`, status, attempts, available, msg, jobID)
}
