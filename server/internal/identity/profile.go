package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserProfile struct {
	ID                       uuid.UUID  `json:"id"`
	Username                 string     `json:"username"`
	IsAdmin                  bool       `json:"is_admin"`
	StorageQuotaBytes        int64      `json:"storage_quota_bytes"`
	StorageUsedBytes         int64      `json:"storage_used_bytes"`
	SpeedLimitKbps           *int       `json:"speed_limit_kbps"`
	EffectiveSpeedLimitKbps  *int       `json:"effective_speed_limit_kbps"`
	RootFolderID             uuid.UUID  `json:"root_folder_id"`
}

func LoadProfile(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) (UserProfile, error) {
	var p UserProfile
	var speedLimit *int32
	err := pool.QueryRow(ctx, `
		SELECT id, username, is_admin, storage_quota_bytes, storage_used_bytes, speed_limit_kbps
		FROM users WHERE id = $1
	`, userID).Scan(&p.ID, &p.Username, &p.IsAdmin, &p.StorageQuotaBytes, &p.StorageUsedBytes, &speedLimit)
	if err != nil {
		return UserProfile{}, err
	}
	if speedLimit != nil {
		v := int(*speedLimit)
		p.SpeedLimitKbps = &v
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM folders WHERE user_id = $1 AND parent_id IS NULL LIMIT 1
	`, userID).Scan(&p.RootFolderID)
	if err != nil {
		return UserProfile{}, err
	}

	var globalLimit *int32
	err = pool.QueryRow(ctx, `SELECT global_speed_limit_kbps FROM settings WHERE id = true`).Scan(&globalLimit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return UserProfile{}, err
	}

	p.EffectiveSpeedLimitKbps = effectiveSpeedLimit(p.SpeedLimitKbps, globalLimit)
	return p, nil
}

func effectiveSpeedLimit(userLimit *int, globalLimit *int32) *int {
	userUnlimited := userLimit != nil && *userLimit == 0
	if userUnlimited {
		return nil
	}
	var globalPos *int
	if globalLimit != nil && *globalLimit > 0 {
		v := int(*globalLimit)
		globalPos = &v
	}
	if userLimit != nil && *userLimit > 0 {
		u := *userLimit
		if globalPos != nil {
			if u < *globalPos {
				return &u
			}
			return globalPos
		}
		return &u
	}
	return globalPos
}
