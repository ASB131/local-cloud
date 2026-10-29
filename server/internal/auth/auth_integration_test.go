//go:build integration

package auth_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/admin"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/auth"
	"github.com/asb131/local-cloud/internal/bootstrap"
	"github.com/asb131/local-cloud/internal/config"
	"github.com/asb131/local-cloud/internal/db"
	"github.com/asb131/local-cloud/internal/storage"
)

func setupAuthTest(t *testing.T) (*pgxpool.Pool, *auth.Service, *admin.InviteService, uuid.UUID, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	if err := db.RunMigrations(db.MigrateURL(dsn)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `TRUNCATE invite_codes, refresh_tokens, system_logs, users CASCADE`)
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	cfg := config.Config{
		DatabaseURL:     dsn,
		DataDir:         t.TempDir(),
		JWTSecret:       "01234567890123456789012345678901",
		AdminUsername:   "admin",
		AdminPassword:   "integration-test-password",
		AdminQuotaBytes: 107374182400,
	}
	layout, err := storage.NewLayout(cfg.DataDir)
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	if err := layout.EnsureBaseDirs(); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	if err := bootstrap.EnsureAdmin(ctx, pool, cfg, layout); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	var adminID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE username = 'admin'`).Scan(&adminID); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	tokens := auth.NewTokenIssuer(cfg.JWTSecret)
	auditWriter := audit.NewWriter(pool)
	svc := auth.NewService(pool, tokens, auditWriter, auth.NewLoginIPLimiter(), layout)
	inviteSvc := admin.NewInviteService(pool, auditWriter)
	cleanup := func() { pool.Close() }
	return pool, svc, inviteSvc, adminID, cleanup
}

func TestRegisterConsumesInviteOnce(t *testing.T) {
	_, svc, inviteSvc, adminID, cleanup := setupAuthTest(t)
	defer cleanup()
	ctx := context.Background()

	created, err := inviteSvc.Create(ctx, adminID, admin.InviteCreateInput{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Register(ctx, "newuser", "valid-password-12", created.Code, "test", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_, err = svc.Register(ctx, "newuser2", "another-pass-12xx", created.Code, "test", nil)
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}

func TestLoginLockoutAfterFiveFailures(t *testing.T) {
	_, svc, _, _, cleanup := setupAuthTest(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := svc.Login(ctx, "admin", "wrong-password", "test", nil)
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	_, err := svc.Login(ctx, "admin", "wrong-password", "test", nil)
	var lock *auth.LockError
	if !errors.As(err, &lock) {
		t.Fatalf("expected lock, got %v", err)
	}
	if lock.RetryAfterSeconds <= 0 {
		t.Fatalf("retry_after_seconds = %d", lock.RetryAfterSeconds)
	}
}

func TestRefreshReuseRevokesAll(t *testing.T) {
	pool, svc, _, _, cleanup := setupAuthTest(t)
	defer cleanup()
	ctx := context.Background()

	login, err := svc.Login(ctx, "admin", "integration-test-password", "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	firstRefresh := login.RefreshToken
	rotated, err := svc.Refresh(ctx, firstRefresh)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Refresh(ctx, firstRefresh)
	if !errors.Is(err, auth.ErrRefreshReuse) {
		t.Fatalf("expected reuse error, got %v", err)
	}
	var active int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL
	`, login.Profile.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("expected all refresh tokens revoked, active=%d", active)
	}
	_, err = svc.Refresh(ctx, rotated.RefreshToken)
	if !errors.Is(err, auth.ErrInvalidCredentials) && !errors.Is(err, auth.ErrRefreshReuse) {
		t.Fatalf("rotated token should be invalid after reuse purge, got %v", err)
	}
}
