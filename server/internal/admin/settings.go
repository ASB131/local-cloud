package admin

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct {
	GlobalSpeedLimitKbps *int `json:"global_speed_limit_kbps"`
	VersionRetentionDays int  `json:"version_retention_days"`
	VersionMaxBytes      int64 `json:"version_max_bytes"`
}

type SettingsPatch struct {
	GlobalSpeedLimitKbps *int `json:"global_speed_limit_kbps"`
}

func GetSettings(ctx context.Context, pool *pgxpool.Pool) (Settings, error) {
	var s Settings
	var global *int32
	err := pool.QueryRow(ctx, `
		SELECT global_speed_limit_kbps, version_retention_days, version_max_bytes FROM settings WHERE id = true
	`).Scan(&global, &s.VersionRetentionDays, &s.VersionMaxBytes)
	if err != nil {
		return Settings{}, err
	}
	if global != nil {
		v := int(*global)
		s.GlobalSpeedLimitKbps = &v
	}
	return s, nil
}

func PatchSettings(ctx context.Context, pool *pgxpool.Pool, patch SettingsPatch) error {
	if patch.GlobalSpeedLimitKbps == nil {
		return ValidationError{Message: "Nothing to update."}
	}
	var g *int32
	if patch.GlobalSpeedLimitKbps != nil {
		v := int32(*patch.GlobalSpeedLimitKbps)
		g = &v
	}
	_, err := pool.Exec(ctx, `UPDATE settings SET global_speed_limit_kbps = $1 WHERE id = true`, g)
	return err
}
