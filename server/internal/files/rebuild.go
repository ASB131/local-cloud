package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/asb131/local-cloud/internal/names"
)

type RebuildReport struct {
	StartedAt            time.Time `json:"started_at"`
	FinishedAt           time.Time `json:"finished_at"`
	FilesRepaired        int       `json:"files_repaired"`
	FilesInserted        int       `json:"files_inserted"`
	FilesMissing         int       `json:"files_missing"`
	OrphansSkipped       int       `json:"orphans_skipped"`
	Errors               int       `json:"errors"`
	SampleMissingFileIDs []string  `json:"sample_missing_file_ids,omitempty"`
}

var (
	rebuildMu   sync.Mutex
	rebuildBusy bool
	lastReport  RebuildReport
)

func LastRebuildReport() RebuildReport {
	rebuildMu.Lock()
	defer rebuildMu.Unlock()
	return lastReport
}

func (s *Store) RebuildFromDisk(ctx context.Context) (RebuildReport, error) {
	rebuildMu.Lock()
	if rebuildBusy {
		rebuildMu.Unlock()
		return RebuildReport{}, fmt.Errorf("rebuild already running")
	}
	rebuildBusy = true
	rebuildMu.Unlock()
	defer func() {
		rebuildMu.Lock()
		rebuildBusy = false
		rebuildMu.Unlock()
	}()

	report := RebuildReport{StartedAt: time.Now().UTC()}
	usersDir := filepath.Join(s.Layout.Root(), "users")
	userEntries, err := os.ReadDir(usersDir)
	if err != nil {
		report.Errors++
		report.FinishedAt = time.Now().UTC()
		lastReport = report
		return report, err
	}

	for _, ue := range userEntries {
		if !ue.IsDir() {
			continue
		}
		userID, err := uuid.Parse(ue.Name())
		if err != nil {
			report.OrphansSkipped++
			continue
		}
		var exists bool
		if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil || !exists {
			report.OrphansSkipped++
			continue
		}
		objectsDir := filepath.Join(usersDir, ue.Name(), "objects")
		objEntries, _ := os.ReadDir(objectsDir)
		for _, oe := range objEntries {
			if !oe.IsDir() {
				continue
			}
			fileID, err := uuid.Parse(oe.Name())
			if err != nil {
				continue
			}
			if err := s.rebuildObject(ctx, userID, fileID, filepath.Join(objectsDir, oe.Name()), &report); err != nil {
				report.Errors++
			}
		}
	}

	rows, err := s.Pool.Query(ctx, `SELECT id, user_id, disk_path FROM files WHERE is_deleted = false`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var fid, uid uuid.UUID
			var rel string
			if err := rows.Scan(&fid, &uid, &rel); err != nil {
				continue
			}
			abs, err := s.Layout.AbsFromRel(rel)
			if err != nil || fileMissing(abs) {
				report.FilesMissing++
				if len(report.SampleMissingFileIDs) < 100 {
					report.SampleMissingFileIDs = append(report.SampleMissingFileIDs, fid.String())
				}
			}
		}
	}

	if err := s.reconcileAllUsage(ctx); err != nil {
		report.Errors++
	}

	report.FinishedAt = time.Now().UTC()
	rebuildMu.Lock()
	lastReport = report
	rebuildMu.Unlock()
	return report, nil
}

func fileMissing(path string) bool {
	st, err := os.Stat(path)
	return err != nil || st.IsDir()
}

func (s *Store) rebuildObject(ctx context.Context, userID, fileID uuid.UUID, objDir string, report *RebuildReport) error {
	metaPath := filepath.Join(objDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil
	}
	var meta sidecarMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	if meta.UserID != userID || meta.FileID != fileID {
		report.OrphansSkipped++
		return nil
	}

	versions, err := scanVersionBlobs(objDir)
	if err != nil || len(versions) == 0 {
		return err
	}
	root := s.Layout.Root()
	for i := range versions {
		rel, err := filepath.Rel(root, versions[i].abs)
		if err != nil {
			return err
		}
		versions[i].rel = filepath.ToSlash(rel)
	}
	cur := versions[len(versions)-1]

	rootID, err := s.userRootFolder(ctx, userID)
	if err != nil {
		return err
	}
	folderID, err := s.ensureFolderPath(ctx, userID, rootID, meta.FolderPath)
	if err != nil {
		return err
	}

	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE id = $1 AND user_id = $2)`, fileID, userID).Scan(&exists); err != nil {
		return err
	}

	hash, size, err := hashFile(cur.abs)
	if err != nil {
		return err
	}
	if size == 0 && meta.ByteSize > 0 {
		size = meta.ByteSize
	}
	if hash == "" && meta.SHA256 != "" {
		hash = meta.SHA256
	}

	now := time.Now().UTC()
	if !exists {
		key := names.NameKey(meta.Name)
		_, err = s.Pool.Exec(ctx, `
			INSERT INTO files (
				id, user_id, folder_id, name, name_key, size_bytes, mime_type, disk_path, file_hash,
				meta_created_at, meta_modified_at, thumbnail_status, created_at, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'none',$12,$12)
		`, fileID, userID, folderID, meta.Name, key, size, meta.MimeType, cur.rel, hash,
			meta.MetaCreatedAt, meta.MetaModifiedAt, now)
		if err != nil {
			return err
		}
		report.FilesInserted++
	} else {
		var curHash string
		var curSize int64
		_ = s.Pool.QueryRow(ctx, `SELECT file_hash, size_bytes FROM files WHERE id = $1`, fileID).Scan(&curHash, &curSize)
		if curHash != hash || curSize != size {
			_, err = s.Pool.Exec(ctx, `
				UPDATE files SET size_bytes = $1, file_hash = $2, disk_path = $3, updated_at = $4
				WHERE id = $5 AND user_id = $6
			`, size, hash, cur.rel, now, fileID, userID)
			if err != nil {
				return err
			}
			report.FilesRepaired++
		}
	}

	for _, v := range versions {
		h, sz, err := hashFile(v.abs)
		if err != nil {
			continue
		}
		vid, _ := uuid.NewV7()
		_, _ = s.Pool.Exec(ctx, `
			INSERT INTO file_versions (id, file_id, version_num, size_bytes, disk_path, file_hash, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (file_id, version_num) DO UPDATE SET
				size_bytes = EXCLUDED.size_bytes,
				disk_path = EXCLUDED.disk_path,
				file_hash = EXCLUDED.file_hash
		`, vid, fileID, v.num, sz, v.rel, h, now)
	}
	return nil
}

type versionBlob struct {
	num int
	abs string
	rel string
}

func scanVersionBlobs(objDir string) ([]versionBlob, error) {
	entries, err := os.ReadDir(objDir)
	if err != nil {
		return nil, err
	}
	var out []versionBlob
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "v") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), "v"))
		if err != nil {
			continue
		}
		out = append(out, versionBlob{num: n, abs: filepath.Join(objDir, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].num < out[j].num })
	return out, nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func (s *Store) userRootFolder(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT id FROM folders WHERE user_id = $1 AND parent_id IS NULL LIMIT 1`, userID).Scan(&id)
	return id, err
}

func (s *Store) ensureFolderPath(ctx context.Context, userID, rootID uuid.UUID, path []string) (uuid.UUID, error) {
	cur := rootID
	for _, seg := range path {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if err := names.Validate(seg, false); err != nil {
			return uuid.Nil, err
		}
		key := names.NameKey(seg)
		var next uuid.UUID
		err := s.Pool.QueryRow(ctx, `
			SELECT id FROM folders
			WHERE user_id = $1 AND parent_id = $2 AND name_key = $3 AND is_deleted = false
		`, userID, cur, key).Scan(&next)
		if err == nil {
			cur = next
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, err
		}
		next, err = uuid.NewV7()
		if err != nil {
			return uuid.Nil, err
		}
		now := time.Now().UTC()
		_, err = s.Pool.Exec(ctx, `
			INSERT INTO folders (id, user_id, parent_id, name, name_key, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$6)
		`, next, userID, cur, seg, key, now)
		if err != nil {
			return uuid.Nil, err
		}
		cur = next
	}
	return cur, nil
}

func (s *Store) ReconcileUsage(ctx context.Context) error {
	return s.reconcileAllUsage(ctx)
}

func (s *Store) reconcileAllUsage(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE users u SET storage_used_bytes = COALESCE((
			SELECT SUM(fv.size_bytes) FROM file_versions fv
			JOIN files f ON f.id = fv.file_id WHERE f.user_id = u.id AND f.is_deleted = false
		), 0)
	`)
	return err
}
