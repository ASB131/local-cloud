package auth

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/asb131/local-cloud/internal/audit"
	"github.com/asb131/local-cloud/internal/identity"
	"github.com/asb131/local-cloud/internal/storage"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account locked")
	ErrRateLimited        = errors.New("rate limited")
	ErrRefreshReuse       = errors.New("refresh reuse")
)

type LockError struct {
	RetryAfterSeconds int
}

func (e *LockError) Error() string {
	return "account locked"
}

type Service struct {
	pool    *pgxpool.Pool
	tokens  *TokenIssuer
	audit   *audit.Writer
	limiter *LoginIPLimiter
	layout  *storage.Layout
}

func NewService(pool *pgxpool.Pool, tokens *TokenIssuer, audit *audit.Writer, limiter *LoginIPLimiter, layout *storage.Layout) *Service {
	return &Service{
		pool:    pool,
		tokens:  tokens,
		audit:   audit,
		limiter: limiter,
		layout:  layout,
	}
}

type AuthResult struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	Profile         identity.UserProfile
}

type loginUserRow struct {
	id           uuid.UUID
	passwordHash string
	isAdmin      bool
	isDisabled   bool
	attempts     int
	lockedUntil  *time.Time
}

func (s *Service) Login(ctx context.Context, username, password, deviceLabel string, ip net.IP) (AuthResult, error) {
	if !s.limiter.Allow(ipString(ip)) {
		return AuthResult{}, ErrRateLimited
	}

	normalized := identity.NormalizeUsername(username)
	row, err := s.loadUserByUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = s.audit.Write(ctx, nil, "auth.login.failure", "login failed", ip)
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	now := time.Now().UTC()
	if row.lockedUntil != nil && row.lockedUntil.After(now) {
		sec := int(row.lockedUntil.Sub(now).Seconds())
		if sec < 1 {
			sec = 1
		}
		return AuthResult{}, &LockError{RetryAfterSeconds: sec}
	}

	ok, err := VerifyPassword(password, row.passwordHash)
	if err != nil || !ok {
		if err := s.recordFailedLogin(ctx, row.id, row.attempts+1, ip); err != nil {
			return AuthResult{}, err
		}
		return AuthResult{}, ErrInvalidCredentials
	}

	if row.isDisabled {
		return AuthResult{}, ErrAccountDisabled
	}

	if err := s.clearLoginFailures(ctx, row.id, now); err != nil {
		return AuthResult{}, err
	}

	result, err := s.issueSession(ctx, row.id, normalized, row.isAdmin, deviceLabel)
	if err != nil {
		return AuthResult{}, err
	}
	uid := row.id
	_ = s.audit.Write(ctx, &uid, "auth.login.success", "login succeeded", ip)
	return result, nil
}

func (s *Service) Register(ctx context.Context, username, password, invitePlain, deviceLabel string, ip net.IP) (AuthResult, error) {
	if !s.limiter.Allow(ipString(ip)) {
		return AuthResult{}, ErrRateLimited
	}
	normalized := identity.NormalizeUsername(username)
	if !identity.ValidUsername(normalized) {
		return AuthResult{}, identity.ErrValidation("username format invalid")
	}
	if err := identity.ValidatePassword(password); err != nil {
		return AuthResult{}, identity.ErrValidation(err.Error())
	}

	hash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	codeHash := HashInviteCode(invitePlain)
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AuthResult{}, err
	}
	defer tx.Rollback(ctx)

	var inviteID uuid.UUID
	var defaultQuota *int64
	var isUsed bool
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, is_used, expires_at, default_quota_bytes
		FROM invite_codes
		WHERE code_hash = $1
		FOR UPDATE
	`, codeHash).Scan(&inviteID, &isUsed, &expiresAt, &defaultQuota)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	if isUsed || !expiresAt.After(now) {
		return AuthResult{}, ErrInvalidCredentials
	}

	var quota int64
	if defaultQuota != nil {
		quota = *defaultQuota
	} else {
		if err := tx.QueryRow(ctx, `SELECT default_user_quota_bytes FROM settings WHERE id = true`).Scan(&quota); err != nil {
			return AuthResult{}, err
		}
	}

	userID, err := uuid.NewV7()
	if err != nil {
		return AuthResult{}, err
	}
	folderID, err := uuid.NewV7()
	if err != nil {
		return AuthResult{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO users (
			id, username, password_hash, is_admin, storage_quota_bytes, created_at, updated_at
		) VALUES ($1, $2, $3, false, $4, $5, $5)
	`, userID, normalized, hash, quota, now)
	if err != nil {
		return AuthResult{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO folders (id, user_id, parent_id, name, name_key, created_at, updated_at)
		VALUES ($1, $2, NULL, '', '', $3, $3)
	`, folderID, userID, now)
	if err != nil {
		return AuthResult{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE invite_codes
		SET is_used = true, used_by = $1, used_at = $2
		WHERE id = $3
	`, userID, now, inviteID)
	if err != nil {
		return AuthResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, err
	}

	if err := s.layout.EnsureUserDirs(userID); err != nil {
		return AuthResult{}, err
	}

	result, err := s.issueSession(ctx, userID, normalized, false, deviceLabel)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.audit.Write(ctx, &userID, "auth.register", "registration succeeded", ip)
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, refreshPlain string) (AuthResult, error) {
	hash := HashRefreshToken(refreshPlain)
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AuthResult{}, err
	}
	defer tx.Rollback(ctx)

	var tokenID uuid.UUID
	var userID uuid.UUID
	var revokedAt *time.Time
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, revoked_at, expires_at
		FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE
	`, hash).Scan(&tokenID, &userID, &revokedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}

	if revokedAt != nil {
		_, _ = tx.Exec(ctx, `
			UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL
		`, now, userID)
		if err := tx.Commit(ctx); err != nil {
			return AuthResult{}, err
		}
		return AuthResult{}, ErrRefreshReuse
	}
	if !expiresAt.After(now) {
		return AuthResult{}, ErrInvalidCredentials
	}

	var username string
	var isAdmin bool
	var isDisabled bool
	err = tx.QueryRow(ctx, `
		SELECT username, is_admin, is_disabled FROM users WHERE id = $1
	`, userID).Scan(&username, &isAdmin, &isDisabled)
	if err != nil {
		return AuthResult{}, err
	}
	if isDisabled {
		return AuthResult{}, ErrAccountDisabled
	}

	_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`, now, tokenID)
	if err != nil {
		return AuthResult{}, err
	}

	newPlain, newHash, err := NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}
	newExp := RefreshExpiry()
	newID, err := uuid.NewV7()
	if err != nil {
		return AuthResult{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, newID, userID, newHash, newExp, now)
	if err != nil {
		return AuthResult{}, err
	}

	access, accessExp, err := s.tokens.IssueAccess(userID, username, isAdmin)
	if err != nil {
		return AuthResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, err
	}

	profile, err := identity.LoadProfile(ctx, s.pool, userID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		AccessToken:     access,
		AccessExpiresAt: accessExp,
		RefreshToken:    newPlain,
		Profile:         profile,
	}, nil
}

func (s *Service) Logout(ctx context.Context, userID uuid.UUID, refreshPlain string, ip net.IP) error {
	hash := HashRefreshToken(refreshPlain)
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $1
		WHERE user_id = $2 AND token_hash = $3 AND revoked_at IS NULL
	`, now, userID, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidCredentials
	}
	_ = s.audit.Write(ctx, &userID, "auth.logout", "logout succeeded", ip)
	return nil
}

func (s *Service) issueSession(ctx context.Context, userID uuid.UUID, username string, isAdmin bool, deviceLabel string) (AuthResult, error) {
	access, accessExp, err := s.tokens.IssueAccess(userID, username, isAdmin)
	if err != nil {
		return AuthResult{}, err
	}
	refreshPlain, refreshHash, err := NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}
	refreshExp := RefreshExpiry()
	tokenID, err := uuid.NewV7()
	if err != nil {
		return AuthResult{}, err
	}
	label := truncateDeviceLabel(deviceLabel)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, device_label, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, tokenID, userID, refreshHash, nullString(label), refreshExp, time.Now().UTC())
	if err != nil {
		return AuthResult{}, err
	}
	profile, err := identity.LoadProfile(ctx, s.pool, userID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		AccessToken:     access,
		AccessExpiresAt: accessExp,
		RefreshToken:    refreshPlain,
		Profile:         profile,
	}, nil
}

func (s *Service) loadUserByUsername(ctx context.Context, username string) (loginUserRow, error) {
	var row loginUserRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, password_hash, is_admin, is_disabled, failed_login_attempts, locked_until
		FROM users WHERE username = $1
	`, username).Scan(&row.id, &row.passwordHash, &row.isAdmin, &row.isDisabled, &row.attempts, &row.lockedUntil)
	return row, err
}

func (s *Service) recordFailedLogin(ctx context.Context, userID uuid.UUID, attempts int, ip net.IP) error {
	now := time.Now().UTC()
	var lockedUntil *time.Time
	if attempts >= 5 {
		t := now.Add(15 * time.Minute)
		lockedUntil = &t
		uid := userID
		_ = s.audit.Write(ctx, &uid, "auth.lockout", "account locked for 15 minutes", ip)
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = $1, locked_until = $2, updated_at = $3
		WHERE id = $4
	`, attempts, lockedUntil, now, userID)
	if err != nil {
		return err
	}
	uid := userID
	return s.audit.Write(ctx, &uid, "auth.login.failure", "login failed", ip)
}

func (s *Service) clearLoginFailures(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL, last_login_at = $1, updated_at = $1
		WHERE id = $2
	`, now, userID)
	return err
}

func truncateDeviceLabel(label string) string {
	if len(label) <= 64 {
		return label
	}
	return label[:64]
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ipString(ip net.IP) string {
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}
