package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/config"
	"github.com/asb131/local-cloud/internal/identity"
	"github.com/asb131/local-cloud/internal/storage"
)

func EnsureAdmin(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, layout *storage.Layout) error {
	var count int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if !cfg.BootstrapConfigured() {
		return fmt.Errorf("no users exist and bootstrap admin env is not configured")
	}

	username := strings.ToLower(cfg.AdminUsername)
	if !identity.ValidUsername(username) {
		return fmt.Errorf("LOCAL_CLOUD_ADMIN_USERNAME: invalid format")
	}

	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}

	userID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	folderID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO users (
			id, username, password_hash, is_admin, storage_quota_bytes,
			created_at, updated_at
		) VALUES ($1, $2, $3, true, $4, $5, $5)
	`, userID, username, hash, cfg.AdminQuotaBytes, now)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO folders (
			id, user_id, parent_id, name, name_key, created_at, updated_at
		) VALUES ($1, $2, NULL, '', '', $3, $3)
	`, folderID, userID, now)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if err := layout.EnsureUserDirs(userID); err != nil {
		return err
	}

	return nil
}

// SyncBootstrapAdminPassword sets the bootstrap admin's password from config on every startup
// so LOCAL_CLOUD_ADMIN_* remains the sign-in for that account (including after credential changes).
func SyncBootstrapAdminPassword(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	if !cfg.BootstrapConfigured() {
		return nil
	}
	username := strings.ToLower(cfg.AdminUsername)
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = NOW()
		WHERE username = $2 AND is_admin = true
	`, hash, username)
	return err
}
