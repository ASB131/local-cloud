//go:build integration

package db_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"log/slog"

	"github.com/asb131/local-cloud/internal/bootstrap"
	"github.com/asb131/local-cloud/internal/config"
	"github.com/asb131/local-cloud/internal/db"
	"github.com/asb131/local-cloud/internal/httpapi"
	"github.com/asb131/local-cloud/internal/storage"
)

func TestMigrationsBootstrapAndReady(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	if err := db.RunMigrations(db.MigrateURL(dsn)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `TRUNCATE invite_codes, refresh_tokens, system_logs, users CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	cfg := config.Config{
		DatabaseURL:     dsn,
		DataDir:         t.TempDir(),
		JWTSecret:       "01234567890123456789012345678901",
		AdminUsername:   "bootstrapdb",
		AdminPassword:   "integration-test-password",
		AdminQuotaBytes: 107374182400,
		LogLevel:        "error",
		HTTPAddr:        ":8090",
	}

	layout, err := storage.NewLayout(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.EnsureBaseDirs(); err != nil {
		t.Fatal(err)
	}

	if err := bootstrap.EnsureAdmin(ctx, pool, cfg, layout); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var roots int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM folders WHERE parent_id IS NULL`).Scan(&roots); err != nil {
		t.Fatal(err)
	}
	if roots != 1 {
		t.Fatalf("root folders = %d", roots)
	}

	srv := httpapi.New(slog.Default(), cfg, pool, layout)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzFailsWithoutDatabase(t *testing.T) {
	cfg := config.Config{
		DatabaseURL: "postgres://invalid:invalid@127.0.0.1:1/none?sslmode=disable",
		DataDir:     t.TempDir(),
		JWTSecret:   "01234567890123456789012345678901",
		HTTPAddr:    ":8090",
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err == nil {
		pool.Close()
		t.Skip("unexpectedly connected to invalid database")
	}

	layout, err := storage.NewLayout(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv := httpapi.New(slog.Default(), cfg, nil, layout)
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}
