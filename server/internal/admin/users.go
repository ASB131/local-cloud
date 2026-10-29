package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/storage"
)

type UserListItem struct {
	ID                uuid.UUID  `json:"id"`
	Username          string     `json:"username"`
	IsAdmin           bool       `json:"is_admin"`
	IsDisabled        bool       `json:"is_disabled"`
	StorageQuotaBytes int64      `json:"storage_quota_bytes"`
	StorageUsedBytes  int64      `json:"storage_used_bytes"`
	SpeedLimitKbps    *int       `json:"speed_limit_kbps"`
	LastLoginAt       *time.Time `json:"last_login_at"`
}

type UserPatch struct {
	StorageQuotaBytes *int64 `json:"storage_quota_bytes"`
	SpeedLimitKbps    *int   `json:"speed_limit_kbps"`
	IsAdmin           *bool  `json:"is_admin"`
	IsDisabled        *bool  `json:"is_disabled"`
}

type Service struct {
	pool   *pgxpool.Pool
	audit  *audit.Writer
	layout *storage.Layout
}

func NewService(pool *pgxpool.Pool, audit *audit.Writer, layout *storage.Layout) *Service {
	return &Service{pool: pool, audit: audit, layout: layout}
}

func (s *Service) ListUsers(ctx context.Context) ([]UserListItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, is_admin, is_disabled, storage_quota_bytes, storage_used_bytes,
		       speed_limit_kbps, last_login_at
		FROM users ORDER BY username ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserListItem
	for rows.Next() {
		var item UserListItem
		var speed *int32
		if err := rows.Scan(
			&item.ID, &item.Username, &item.IsAdmin, &item.IsDisabled,
			&item.StorageQuotaBytes, &item.StorageUsedBytes, &speed, &item.LastLoginAt,
		); err != nil {
			return nil, err
		}
		if speed != nil {
			v := int(*speed)
			item.SpeedLimitKbps = &v
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) PatchUser(ctx context.Context, actorID, targetID uuid.UUID, patch UserPatch) error {
	if patch.StorageQuotaBytes == nil && patch.SpeedLimitKbps == nil && patch.IsAdmin == nil && patch.IsDisabled == nil {
		return errValidation("no fields to update")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var isAdmin bool
	var used int64
	err = tx.QueryRow(ctx, `
		SELECT is_admin, storage_used_bytes FROM users WHERE id = $1 FOR UPDATE
	`, targetID).Scan(&isAdmin, &used)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if patch.IsAdmin != nil && !*patch.IsAdmin && isAdmin {
		if err := ensureNotLastAdmin(ctx, tx, targetID); err != nil {
			return err
		}
	}
	if patch.IsDisabled != nil && *patch.IsDisabled && isAdmin {
		if err := ensureNotLastAdmin(ctx, tx, targetID); err != nil {
			return err
		}
	}
	if patch.StorageQuotaBytes != nil && *patch.StorageQuotaBytes < used {
		return errValidation("quota cannot be below storage already used")
	}

	now := time.Now().UTC()
	if patch.StorageQuotaBytes != nil {
		_, err = tx.Exec(ctx, `UPDATE users SET storage_quota_bytes = $1, updated_at = $2 WHERE id = $3`, *patch.StorageQuotaBytes, now, targetID)
		if err != nil {
			return err
		}
	}
	if patch.SpeedLimitKbps != nil {
		_, err = tx.Exec(ctx, `UPDATE users SET speed_limit_kbps = $1, updated_at = $2 WHERE id = $3`, *patch.SpeedLimitKbps, now, targetID)
		if err != nil {
			return err
		}
	}
	if patch.IsAdmin != nil {
		_, err = tx.Exec(ctx, `UPDATE users SET is_admin = $1, updated_at = $2 WHERE id = $3`, *patch.IsAdmin, now, targetID)
		if err != nil {
			return err
		}
	}
	if patch.IsDisabled != nil {
		_, err = tx.Exec(ctx, `UPDATE users SET is_disabled = $1, updated_at = $2 WHERE id = $3`, *patch.IsDisabled, now, targetID)
		if err != nil {
			return err
		}
		if *patch.IsDisabled {
			_, err = tx.Exec(ctx, `
				UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL
			`, now, targetID)
			if err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.audit.Write(ctx, &actorID, "admin.user_updated", fmt.Sprintf("updated user %s", targetID.String()), nil)
}

func (s *Service) DeleteUser(ctx context.Context, actorID, targetID uuid.UUID) error {
	if actorID == targetID {
		return errValidation("cannot delete your own account")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var isAdmin bool
	err = tx.QueryRow(ctx, `SELECT is_admin FROM users WHERE id = $1 FOR UPDATE`, targetID).Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if isAdmin {
		if err := ensureNotLastAdmin(ctx, tx, targetID); err != nil {
			return err
		}
	}

	var unusedInvites int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM invite_codes WHERE created_by = $1 AND is_used = false
	`, targetID).Scan(&unusedInvites)
	if err != nil {
		return err
	}
	if unusedInvites > 0 {
		return errValidation("revoke unused invites before deleting this user")
	}

	_, err = tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, targetID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errValidation("user cannot be deleted while referenced by invites")
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	userDir := s.layout.UserRoot(targetID)
	_ = os.RemoveAll(userDir)

	return s.audit.Write(ctx, &actorID, "admin.user_deleted", fmt.Sprintf("deleted user %s", targetID.String()), nil)
}

func ensureNotLastAdmin(ctx context.Context, tx pgx.Tx, targetID uuid.UUID) error {
	var adminCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE is_admin = true AND is_disabled = false AND id <> $1
	`, targetID).Scan(&adminCount)
	if err != nil {
		return err
	}
	if adminCount == 0 {
		return errValidation("cannot remove or disable the last admin")
	}
	return nil
}

var ErrNotFound = errors.New("not found")

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func errValidation(msg string) error {
	return ValidationError{Message: msg}
}
