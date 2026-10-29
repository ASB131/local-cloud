package db

import (
	"strings"
)

// MigrateURL converts a standard postgres DSN to pgx5 scheme for golang-migrate.
func MigrateURL(databaseURL string) string {
	if strings.HasPrefix(databaseURL, "pgx5://") {
		return databaseURL
	}
	if strings.HasPrefix(databaseURL, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")
	}
	if strings.HasPrefix(databaseURL, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgresql://")
	}
	return databaseURL
}
