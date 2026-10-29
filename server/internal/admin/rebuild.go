package admin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/files"
)

func StartRebuild(ctx context.Context, store *files.Store, auditW *audit.Writer, actor uuid.UUID) (files.RebuildReport, error) {
	_ = auditW.Write(ctx, &actor, "admin.rebuild_started", "rebuild from disk sidecars", nil)
	report, err := store.RebuildFromDisk(ctx)
	if err != nil {
		if err.Error() == "rebuild already running" {
			return files.RebuildReport{}, ConflictError{Message: "Rebuild already running."}
		}
		return report, err
	}
	_ = auditW.Write(ctx, &actor, "admin.rebuild_finished", fmt.Sprintf("repaired=%d inserted=%d missing=%d", report.FilesRepaired, report.FilesInserted, report.FilesMissing), nil)
	return report, nil
}

func LastRebuildReport() files.RebuildReport {
	return files.LastRebuildReport()
}
