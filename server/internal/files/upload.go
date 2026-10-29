package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
	"github.com/asb131/local-cloud/internal/storage"
)

type UploadInitInput struct {
	FolderID          uuid.UUID
	Name              string
	ExpectedSize      *int64
	MimeHint          *string
	MetaCreatedAt     *time.Time
	MetaModifiedAt    *time.Time
	ConflictPolicy    string
	OverwriteFileID   *uuid.UUID
}

type UploadSession struct {
	ID            uuid.UUID `json:"id"`
	ReceivedBytes int64     `json:"received_bytes"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type ConflictInfo struct {
	FileID    uuid.UUID `json:"file_id"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) InitUpload(ctx context.Context, userID uuid.UUID, in UploadInitInput) (UploadSession, error) {
	if err := names.Validate(in.Name, false); err != nil {
		return UploadSession{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	ok, err := s.userOwnsFolder(ctx, userID, in.FolderID)
	if err != nil {
		return UploadSession{}, err
	}
	if !ok {
		return UploadSession{}, ErrNotFound
	}

	key := names.NameKey(in.Name)
	var existingID uuid.UUID
	var existingSize int64
	var existingUpdated time.Time
	err = s.Pool.QueryRow(ctx, `
		SELECT id, size_bytes, updated_at FROM files
		WHERE folder_id = $1 AND name_key = $2 AND is_deleted = false
	`, in.FolderID, key).Scan(&existingID, &existingSize, &existingUpdated)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return UploadSession{}, err
	}
	hasConflict := err == nil
	if hasConflict && in.ConflictPolicy != "overwrite" {
		return UploadSession{}, &ConflictDetail{Info: ConflictInfo{
			FileID: existingID, SizeBytes: existingSize, UpdatedAt: existingUpdated,
		}}
	}
	if in.ConflictPolicy == "overwrite" {
		if in.OverwriteFileID == nil || *in.OverwriteFileID != existingID {
			return UploadSession{}, ErrNotFound
		}
	}

	var replaceSize int64
	if hasConflict && in.ConflictPolicy == "overwrite" {
		replaceSize, _ = existingSize, 0
		_ = replaceSize
		if in.OverwriteFileID != nil {
			err = s.Pool.QueryRow(ctx, `SELECT size_bytes FROM files WHERE id = $1 AND user_id = $2`, *in.OverwriteFileID, userID).Scan(&replaceSize)
			if err != nil {
				return UploadSession{}, ErrNotFound
			}
		}
	}
	if in.ExpectedSize != nil {
		if err := s.checkQuota(ctx, userID, *in.ExpectedSize, replaceSize); err != nil {
			return UploadSession{}, err
		}
	}

	sessionID, err := uuid.NewV7()
	if err != nil {
		return UploadSession{}, err
	}
	abs, rel, err := s.Layout.UploadTempFile(userID, sessionID)
	if err != nil {
		return UploadSession{}, err
	}
	if err := os.MkdirAll(s.Layout.UserTmpDir(userID), 0o750); err != nil {
		return UploadSession{}, err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return UploadSession{}, err
	}
	f.Close()

	expires := time.Now().UTC().Add(24 * time.Hour)
	var targetFileID *uuid.UUID
	if hasConflict && in.ConflictPolicy == "overwrite" {
		targetFileID = in.OverwriteFileID
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO upload_sessions (
			id, user_id, folder_id, file_id, name, expected_size_bytes, temp_path,
			mime_hint, meta_created_at, meta_modified_at, status, expires_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'open',$11,$11)
	`, sessionID, userID, in.FolderID, targetFileID, in.Name, in.ExpectedSize, rel,
		in.MimeHint, in.MetaCreatedAt, in.MetaModifiedAt, expires)
	if err != nil {
		return UploadSession{}, err
	}
	resetUploadHasher(sessionID)
	return UploadSession{ID: sessionID, ReceivedBytes: 0, ExpiresAt: expires}, nil
}

type ConflictDetail struct {
	Info ConflictInfo
}

func (e *ConflictDetail) Error() string { return "conflict" }

func (s *Store) UploadContent(ctx context.Context, userID, sessionID uuid.UUID, offset int64, body io.Reader) (int64, error) {
	var tempPath string
	var received int64
	var status string
	err := s.Pool.QueryRow(ctx, `
		SELECT temp_path, received_bytes, status FROM upload_sessions
		WHERE id = $1 AND user_id = $2 AND status = 'open'
	`, sessionID, userID).Scan(&tempPath, &received, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if offset != received {
		return 0, fmt.Errorf("%w: offset mismatch", ErrValidation)
	}

	abs, err := s.Layout.AbsFromRel(tempPath)
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE, 0o640)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	hasher, ok := getUploadHasher(sessionID)
	if !ok {
		hasher = resetUploadHasher(sessionID)
	}
	written, err := io.Copy(&hashWriter{w: f, h: hasher}, body)
	if err != nil {
		return received, err
	}
	if err := f.Sync(); err != nil {
		return received, err
	}
	newReceived := received + written
	_, err = s.Pool.Exec(ctx, `UPDATE upload_sessions SET received_bytes = $1 WHERE id = $2`, newReceived, sessionID)
	if err != nil {
		return received, err
	}
	return newReceived, nil
}

type hashWriter struct {
	w io.Writer
	h *uploadHasher
}

func (hw *hashWriter) Write(p []byte) (int, error) {
	hw.h.Write(p)
	return hw.w.Write(p)
}

func (s *Store) CompleteUpload(ctx context.Context, userID, sessionID uuid.UUID) (FileDTO, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FileDTO{}, err
	}
	defer tx.Rollback(ctx)

	var folderID uuid.UUID
	var fileID *uuid.UUID
	var name string
	var tempPath string
	var received int64
	var expected *int64
	var metaCreated, metaModified *time.Time
	var mimeHint *string
	err = tx.QueryRow(ctx, `
		SELECT folder_id, file_id, name, temp_path, received_bytes, expected_size_bytes,
		       meta_created_at, meta_modified_at, mime_hint
		FROM upload_sessions WHERE id = $1 AND user_id = $2 AND status = 'open' FOR UPDATE
	`, sessionID, userID).Scan(&folderID, &fileID, &name, &tempPath, &received, &expected,
		&metaCreated, &metaModified, &mimeHint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FileDTO{}, ErrNotFound
		}
		return FileDTO{}, err
	}
	if expected != nil && received != *expected {
		return FileDTO{}, fmt.Errorf("%w: size mismatch", ErrValidation)
	}

	hasher, ok := getUploadHasher(sessionID)
	if !ok {
		return FileDTO{}, fmt.Errorf("%w: missing upload hash state", ErrValidation)
	}
	fileHash := hasher.SumHex()
	dropUploadHasher(sessionID)

	abs, err := s.Layout.AbsFromRel(tempPath)
	if err != nil {
		return FileDTO{}, err
	}
	head := make([]byte, 512)
 hf, _ := os.Open(abs)
 if hf != nil {
	 n, _ := hf.Read(head)
	 head = head[:n]
	 hf.Close()
 }
	mimeType := DetectMIME(name, head)

	var replaceSize int64
	if fileID != nil {
		if err := tx.QueryRow(ctx, `SELECT size_bytes FROM files WHERE id = $1 AND user_id = $2 FOR UPDATE`, *fileID, userID).Scan(&replaceSize); err != nil {
			return FileDTO{}, ErrNotFound
		}
	}
	if err := s.checkQuotaTx(ctx, tx, userID, received, replaceSize); err != nil {
		_ = os.Remove(abs)
		return FileDTO{}, err
	}

	now := time.Now().UTC()
	key := names.NameKey(name)
	versionMax, err := s.versionMaxBytes(ctx, tx)
	if err != nil {
		return FileDTO{}, err
	}

	var out FileDTO
	if fileID == nil {
		newID, err := uuid.NewV7()
		if err != nil {
			return FileDTO{}, err
		}
		ver := 1
		_, rel, err := s.Layout.VersionFile(userID, newID, ver)
		if err != nil {
			return FileDTO{}, err
		}
		if err := os.MkdirAll(s.Layout.ObjectDir(userID, newID), 0o750); err != nil {
			return FileDTO{}, err
		}
	 destAbs, _, _ := s.Layout.VersionFile(userID, newID, ver)
	 if err := os.Rename(abs, destAbs); err != nil {
		 return FileDTO{}, err
	 }
		pathSegs, _ := s.folderPathSegments(ctx, userID, folderID)
		if err := writeMetaSidecar(s.Layout, userID, newID, ver, name, pathSegs, received, fileHash, mimeType, metaCreated, metaModified, now); err != nil {
			return FileDTO{}, err
		}
		vid, _ := uuid.NewV7()
		thumbStatus := "none"
		if ThumbnailEligible(mimeType) {
			thumbStatus = "pending"
		} else {
			thumbStatus = "skipped"
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO files (
				id, user_id, folder_id, name, name_key, size_bytes, mime_type, disk_path, file_hash,
				meta_created_at, meta_modified_at, thumbnail_status, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)
			RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
				meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
		`, newID, userID, folderID, name, key, received, mimeType, rel, fileHash,
			metaCreated, metaModified, thumbStatus, now).Scan(
			&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
			&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
		)
		if err != nil {
			return FileDTO{}, err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO file_versions (id, file_id, version_num, size_bytes, disk_path, file_hash, created_at)
			VALUES ($1,$2,1,$3,$4,$5,$6)
		`, vid, newID, received, rel, fileHash, now)
		if err != nil {
			return FileDTO{}, err
		}
		if thumbStatus == "pending" {
			if err := s.enqueueThumb(ctx, tx, newID); err != nil {
				return FileDTO{}, err
			}
		}
		if OfficePreviewEligible(name, mimeType, received) {
			_, err = tx.Exec(ctx, `UPDATE files SET office_preview_status = 'pending' WHERE id = $1`, newID)
			if err != nil {
				return FileDTO{}, err
			}
			if err := s.enqueueOfficePreview(ctx, tx, newID); err != nil {
				return FileDTO{}, err
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE users SET storage_used_bytes = storage_used_bytes + $1, updated_at = $2 WHERE id = $3
		`, received, now, userID)
		if err != nil {
			return FileDTO{}, err
		}
	} else {
		out, err = s.completeOverwrite(ctx, tx, userID, *fileID, folderID, name, key, abs, received, fileHash, mimeType, metaCreated, metaModified, now, versionMax, replaceSize)
		if err != nil {
			return FileDTO{}, err
		}
	}

	_, err = tx.Exec(ctx, `UPDATE upload_sessions SET status = 'completed' WHERE id = $1`, sessionID)
	if err != nil {
		return FileDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return FileDTO{}, err
	}
	_ = s.Audit.Write(ctx, &userID, "file.uploaded", fmt.Sprintf("uploaded %d bytes into folder %s", received, folderID.String()), nil)
	if fileID == nil {
		s.emit(ctx, userID, "file.created", out)
	}
	return out, nil
}

func (s *Store) completeOverwrite(ctx context.Context, tx pgx.Tx, userID, fileID, folderID uuid.UUID, name, key, absPath string, size int64, hash, mime string, metaCreated, metaModified *time.Time, now time.Time, versionMax int64, oldSize int64) (FileDTO, error) {
	var curVersion int
	var oldDisk string
	var oldHash string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE((SELECT MAX(version_num) FROM file_versions WHERE file_id = $1),0), disk_path, file_hash
		FROM files WHERE id = $1 AND user_id = $2 FOR UPDATE
	`, fileID, userID).Scan(&curVersion, &oldDisk, &oldHash)
	if err != nil {
		return FileDTO{}, err
	}
	newVer := curVersion + 1
	destAbs, rel, err := s.Layout.VersionFile(userID, fileID, newVer)
	if err != nil {
		return FileDTO{}, err
	}
	if err := os.Rename(absPath, destAbs); err != nil {
		return FileDTO{}, err
	}
	keepHistory := oldSize < versionMax && size < versionMax
	if !keepHistory && curVersion > 0 {
		if oldAbs, err := s.Layout.AbsFromRel(oldDisk); err == nil {
			_ = os.Remove(oldAbs)
		}
		_, _ = tx.Exec(ctx, `DELETE FROM file_versions WHERE file_id = $1 AND version_num < $2`, fileID, newVer)
	}
	vid, _ := uuid.NewV7()
	_, err = tx.Exec(ctx, `
		INSERT INTO file_versions (id, file_id, version_num, size_bytes, disk_path, file_hash, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, vid, fileID, newVer, size, rel, hash, now)
	if err != nil {
		return FileDTO{}, err
	}
	thumbStatus := "none"
	if ThumbnailEligible(mime) {
		thumbStatus = "pending"
	} else {
		thumbStatus = "skipped"
	}
	var out FileDTO
	err = tx.QueryRow(ctx, `
		UPDATE files SET name = $1, name_key = $2, size_bytes = $3, mime_type = $4, disk_path = $5, file_hash = $6,
			meta_created_at = $7, meta_modified_at = $8, thumbnail_status = $9, updated_at = $10
		WHERE id = $11
		RETURNING id, folder_id, name, size_bytes, mime_type, file_hash,
			meta_created_at, meta_modified_at, created_at, updated_at, thumbnail_status
	`, name, key, size, mime, rel, hash, metaCreated, metaModified, thumbStatus, now, fileID).Scan(
		&out.ID, &out.FolderID, &out.Name, &out.SizeBytes, &out.MimeType, &out.FileHash,
		&out.MetaCreatedAt, &out.MetaModifiedAt, &out.CreatedAt, &out.UpdatedAt, &out.ThumbnailStatus,
	)
	if err != nil {
		return FileDTO{}, err
	}
	delta := size - oldSize
	if keepHistory {
		delta = size
	}
	_, err = tx.Exec(ctx, `UPDATE users SET storage_used_bytes = storage_used_bytes + $1, updated_at = $2 WHERE id = $3`, delta, now, userID)
	if err != nil {
		return FileDTO{}, err
	}
	if thumbStatus == "pending" {
		_ = s.enqueueThumb(ctx, tx, fileID)
	}
	if OfficePreviewEligible(name, mime, size) {
		_, _ = tx.Exec(ctx, `UPDATE files SET office_preview_status = 'pending', office_preview_path = NULL WHERE id = $1`, fileID)
		_ = s.enqueueOfficePreview(ctx, tx, fileID)
	}
	pathSegs, _ := s.folderPathSegments(ctx, userID, folderID)
	_ = writeMetaSidecar(s.Layout, userID, fileID, newVer, name, pathSegs, size, hash, mime, metaCreated, metaModified, now)
	if err := s.pruneFileVersions(ctx, tx, userID, fileID, MaxFileVersionsKept); err != nil {
		return FileDTO{}, err
	}
	s.emit(ctx, userID, "file.updated", out)
	return out, nil
}

func (s *Store) AbortUpload(ctx context.Context, userID, sessionID uuid.UUID) error {
	var tempPath string
	err := s.Pool.QueryRow(ctx, `
		SELECT temp_path FROM upload_sessions WHERE id = $1 AND user_id = $2 AND status = 'open'
	`, sessionID, userID).Scan(&tempPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	abs, _ := s.Layout.AbsFromRel(tempPath)
	_ = os.Remove(abs)
	dropUploadHasher(sessionID)
	_, err = s.Pool.Exec(ctx, `UPDATE upload_sessions SET status = 'aborted' WHERE id = $1`, sessionID)
	return err
}

func (s *Store) checkQuota(ctx context.Context, userID uuid.UUID, incoming, replace int64) error {
	var used, quota int64
	err := s.Pool.QueryRow(ctx, `SELECT storage_used_bytes, storage_quota_bytes FROM users WHERE id = $1`, userID).Scan(&used, &quota)
	if err != nil {
		return err
	}
	if quota == 0 {
		return ErrQuota
	}
	if used-replace+incoming > quota {
		return ErrQuota
	}
	return nil
}

func (s *Store) checkQuotaTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, incoming, replace int64) error {
	var used, quota int64
	err := tx.QueryRow(ctx, `SELECT storage_used_bytes, storage_quota_bytes FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&used, &quota)
	if err != nil {
		return err
	}
	if quota == 0 {
		return ErrQuota
	}
	if used-replace+incoming > quota {
		return ErrQuota
	}
	return nil
}

func (s *Store) versionMaxBytes(ctx context.Context, q pgxQueryable) (int64, error) {
	var v int64
	err := q.QueryRow(ctx, `SELECT version_max_bytes FROM settings WHERE id = true`).Scan(&v)
	return v, err
}

type pgxQueryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Store) enqueueThumb(ctx context.Context, tx pgx.Tx, fileID uuid.UUID) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO thumbnail_jobs (id, file_id, status, available_at, created_at, updated_at)
		VALUES ($1,$2,'pending',$3,$3,$3)
	`, id, fileID, now)
	return err
}

type sidecarMeta struct {
	FileID         uuid.UUID   `json:"file_id"`
	UserID         uuid.UUID   `json:"user_id"`
	VersionNum     int         `json:"version_num"`
	ByteSize       int64       `json:"byte_size"`
	SHA256         string      `json:"sha256"`
	MimeType       string      `json:"mime_type"`
	Name           string      `json:"name"`
	FolderPath     []string    `json:"folder_path"`
	MetaCreatedAt  *time.Time  `json:"meta_created_at"`
	MetaModifiedAt *time.Time  `json:"meta_modified_at"`
	UploadedAt     time.Time   `json:"uploaded_at"`
}

func writeMetaSidecar(layout *storage.Layout, userID, fileID uuid.UUID, ver int, name string, folderPath []string, size int64, hash, mime string, metaCreated, metaModified *time.Time, uploaded time.Time) error {
	path := layout.ObjectDir(userID, fileID)
	if err := os.MkdirAll(path, 0o750); err != nil {
		return err
	}
	if folderPath == nil {
		folderPath = []string{}
	}
	meta := sidecarMeta{
		FileID: fileID, UserID: userID, VersionNum: ver, ByteSize: size, SHA256: hash,
		MimeType: mime, Name: name, FolderPath: folderPath, MetaCreatedAt: metaCreated,
		MetaModifiedAt: metaModified, UploadedAt: uploaded,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path+"/meta.json", data, 0o640)
}
