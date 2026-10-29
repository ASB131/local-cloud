package files

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/storage"
	"github.com/asb131/local-cloud/internal/transfer"
	"github.com/asb131/local-cloud/internal/ws"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrParentDeleted = errors.New("parent deleted")
var ErrCycle = errors.New("cycle")
var ErrQuota = errors.New("quota exceeded")
var ErrValidation = errors.New("validation")

type Store struct {
	Pool   *pgxpool.Pool
	Layout *storage.Layout
	Audit  *audit.Writer
	Hub    *ws.Hub
	Tokens *auth.TokenIssuer
}

func (s *Store) LoadSpeedLimits(ctx context.Context, userID uuid.UUID) (transfer.SpeedLimits, error) {
	return s.loadSpeedLimits(ctx, userID)
}

func (s *Store) loadSpeedLimits(ctx context.Context, userID uuid.UUID) (transfer.SpeedLimits, error) {
	var userLimit *int32
	var globalLimit *int32
	err := s.Pool.QueryRow(ctx, `SELECT speed_limit_kbps FROM users WHERE id = $1`, userID).Scan(&userLimit)
	if err != nil {
		return transfer.SpeedLimits{}, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT global_speed_limit_kbps FROM settings WHERE id = true`).Scan(&globalLimit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return transfer.SpeedLimits{}, err
	}
	var sl transfer.SpeedLimits
	if userLimit != nil {
		v := int(*userLimit)
		sl.UserKbps = &v
	}
	if globalLimit != nil {
		v := int(*globalLimit)
		sl.GlobalKbps = &v
	}
	return sl, nil
}

func (s *Store) userOwnsFolder(ctx context.Context, userID, folderID uuid.UUID) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false)
	`, folderID, userID).Scan(&ok)
	return ok, err
}

func (s *Store) userOwnsFile(ctx context.Context, userID, fileID uuid.UUID) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM files WHERE id = $1 AND user_id = $2 AND is_deleted = false)
	`, fileID, userID).Scan(&ok)
	return ok, err
}

func (s *Store) emit(ctx context.Context, userID uuid.UUID, eventType string, payload any) {
	if s.Hub == nil {
		return
	}
	s.Hub.Broadcast(userID, map[string]any{"type": eventType, "payload": payload})
}
