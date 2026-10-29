package preview

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/storage"
	"github.com/asb131/local-cloud/internal/ws"
)

type OfficeWorker struct {
	Pool   *pgxpool.Pool
	Layout *storage.Layout
	Hub    *ws.Hub
	Log    *slog.Logger
}

func (w *OfficeWorker) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if !w.processOne(ctx) {
			time.Sleep(time.Second)
		}
	}
}

func (w *OfficeWorker) processOne(ctx context.Context) bool {
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return false
	}
	defer tx.Rollback(ctx)

	var jobID, fileID uuid.UUID
	var attempts int
	err = tx.QueryRow(ctx, `
		SELECT id, file_id, attempts FROM office_preview_jobs
		WHERE status = 'pending' AND available_at <= now()
		ORDER BY available_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED
	`).Scan(&jobID, &fileID, &attempts)
	if err != nil {
		return false
	}
	_, _ = tx.Exec(ctx, `UPDATE office_preview_jobs SET status = 'running', updated_at = now() WHERE id = $1`, jobID)

	var userID uuid.UUID
	var disk, name string
	err = tx.QueryRow(ctx, `SELECT user_id, disk_path, name FROM files WHERE id = $1`, fileID).Scan(&userID, &disk, &name)
	if err != nil {
		_, _ = tx.Exec(ctx, `UPDATE office_preview_jobs SET status = 'failed', updated_at = now() WHERE id = $1`, jobID)
		_ = tx.Commit(ctx)
		return true
	}
	_ = tx.Commit(ctx)

	srcAbs, err := w.Layout.AbsFromRel(disk)
	if err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}

	tmpDir, err := os.MkdirTemp(w.Layout.Root(), "office-convert-*")
	if err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "soffice",
		"--headless", "--nologo", "--nofirststartwizard",
		"--convert-to", "pdf",
		"--outdir", tmpDir,
		srcAbs,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, strings.TrimSpace(string(out)))
		return true
	}

	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	converted := filepath.Join(tmpDir, base+".pdf")
	if _, err := os.Stat(converted); err != nil {
		entries, _ := os.ReadDir(tmpDir)
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
				converted = filepath.Join(tmpDir, e.Name())
				break
			}
		}
	}
	if _, err := os.Stat(converted); err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, "libreoffice produced no pdf")
		return true
	}

	destAbs, destRel, err := w.Layout.OfficePreviewPDF(userID, fileID)
	if err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o750); err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	in, err := os.Open(converted)
	if err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	defer in.Close()
	out, err := os.OpenFile(destAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		w.failOfficeJob(ctx, jobID, fileID, attempts, err.Error())
		return true
	}
	out.Close()

	_, err = w.Pool.Exec(ctx, `
		UPDATE files SET office_preview_status = 'ready', office_preview_path = $1, updated_at = now() WHERE id = $2
	`, destRel, fileID)
	if err != nil {
		return true
	}
	_, _ = w.Pool.Exec(ctx, `UPDATE office_preview_jobs SET status = 'done', updated_at = now() WHERE id = $1`, jobID)
	if w.Hub != nil {
		w.Hub.Broadcast(userID, map[string]any{
			"type":    "file.office_preview_ready",
			"payload": map[string]any{"id": fileID, "office_preview_status": "ready"},
		})
	}
	return true
}

func (w *OfficeWorker) failOfficeJob(ctx context.Context, jobID, fileID uuid.UUID, attempts int, msg string) {
	attempts++
	delays := []time.Duration{2 * time.Minute, 10 * time.Minute, 30 * time.Minute}
	status := "pending"
	available := time.Now().UTC().Add(delays[0])
	if attempts >= 3 {
		status = "failed"
		_, _ = w.Pool.Exec(ctx, `UPDATE files SET office_preview_status = 'failed', updated_at = now() WHERE id = $1`, fileID)
	} else if attempts-1 < len(delays) {
		available = time.Now().UTC().Add(delays[attempts-1])
	}
	_, _ = w.Pool.Exec(ctx, `
		UPDATE office_preview_jobs SET status = $1, attempts = $2, available_at = $3, last_error = $4, updated_at = now()
		WHERE id = $5
	`, status, attempts, available, msg, jobID)
	if w.Log != nil && status == "failed" {
		w.Log.Warn("office preview failed", "file_id", fileID, "err", msg)
	}
}
