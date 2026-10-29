package admin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/auth"
)

type InviteCreateInput struct {
	ExpiresAt         *time.Time
	DefaultQuotaBytes *int64
}

type InviteCreateResult struct {
	ID                uuid.UUID  `json:"id"`
	Code              string     `json:"code"`
	ExpiresAt         time.Time  `json:"expires_at"`
	DefaultQuotaBytes *int64     `json:"default_quota_bytes,omitempty"`
}

type InviteListItem struct {
	ID                uuid.UUID  `json:"id"`
	CodeHint          string     `json:"code_hint"`
	ExpiresAt         time.Time  `json:"expires_at"`
	IsUsed            bool       `json:"is_used"`
	UsedByUsername    *string    `json:"used_by_username"`
	DefaultQuotaBytes *int64     `json:"default_quota_bytes,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type InviteService struct {
	pool  *pgxpool.Pool
	audit *audit.Writer
}

func NewInviteService(pool *pgxpool.Pool, audit *audit.Writer) *InviteService {
	return &InviteService{pool: pool, audit: audit}
}

func (s *InviteService) Create(ctx context.Context, creatorID uuid.UUID, in InviteCreateInput) (InviteCreateResult, error) {
	code, hash, hint, err := auth.GenerateInviteCode()
	if err != nil {
		return InviteCreateResult{}, err
	}
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	if in.ExpiresAt != nil {
		expires = in.ExpiresAt.UTC()
	}
	id, err := uuid.NewV7()
	if err != nil {
		return InviteCreateResult{}, err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO invite_codes (
			id, code_hash, code_hint, created_by, expires_at, default_quota_bytes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, hash, hint, creatorID, expires, in.DefaultQuotaBytes, now)
	if err != nil {
		return InviteCreateResult{}, err
	}
	_ = s.audit.Write(ctx, &creatorID, "invite.created", "invite created", nil)
	return InviteCreateResult{
		ID:                id,
		Code:              code,
		ExpiresAt:         expires,
		DefaultQuotaBytes: in.DefaultQuotaBytes,
	}, nil
}

func (s *InviteService) List(ctx context.Context) ([]InviteListItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ic.id, ic.code_hint, ic.expires_at, ic.is_used, u.username, ic.default_quota_bytes, ic.created_at
		FROM invite_codes ic
		LEFT JOIN users u ON u.id = ic.used_by
		ORDER BY ic.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InviteListItem
	for rows.Next() {
		var item InviteListItem
		var usedBy *string
		if err := rows.Scan(
			&item.ID, &item.CodeHint, &item.ExpiresAt, &item.IsUsed, &usedBy,
			&item.DefaultQuotaBytes, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.UsedByUsername = usedBy
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *InviteService) Revoke(ctx context.Context, actorID, inviteID uuid.UUID) error {
	var isUsed bool
	err := s.pool.QueryRow(ctx, `SELECT is_used FROM invite_codes WHERE id = $1`, inviteID).Scan(&isUsed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if isUsed {
		return ConflictError{Message: "invite already used"}
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM invite_codes WHERE id = $1 AND is_used = false`, inviteID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = s.audit.Write(ctx, &actorID, "invite.revoked", "invite revoked", nil)
	return nil
}

type ConflictError struct {
	Message string
}

func (e ConflictError) Error() string {
	return e.Message
}
