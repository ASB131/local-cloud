package files

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/names"
	"github.com/asb131/local-cloud/internal/transfer"
)

const maxArchiveEntries = 100000

type archiveEntry struct {
	Path string
	Abs  string
	Mime string
}

func (s *Store) BuildArchive(ctx context.Context, userID uuid.UUID, folderIDs, fileIDs []uuid.UUID) ([]archiveEntry, error) {
	var entries []archiveEntry
	addFile := func(prefix string, id uuid.UUID) error {
		var name, disk, mime string
		err := s.Pool.QueryRow(ctx, `
			SELECT name, disk_path, mime_type FROM files
			WHERE id = $1 AND user_id = $2 AND is_deleted = false
		`, id, userID).Scan(&name, &disk, &mime)
		if err != nil {
			return ErrNotFound
		}
		abs, err := s.Layout.AbsFromRel(disk)
		if err != nil {
			return err
		}
		p := names.SanitizeZipSegment(name)
		if prefix != "" {
			p = prefix + "/" + p
		}
		entries = append(entries, archiveEntry{Path: p, Abs: abs, Mime: mime})
		if len(entries) > maxArchiveEntries {
			return fmt.Errorf("too large")
		}
		return nil
	}
	var walkFolder func(folderID uuid.UUID, prefix string) error
	walkFolder = func(folderID uuid.UUID, prefix string) error {
		var fname string
		err := s.Pool.QueryRow(ctx, `SELECT name FROM folders WHERE id = $1 AND user_id = $2 AND is_deleted = false`, folderID, userID).Scan(&fname)
		if err != nil {
			return ErrNotFound
		}
		seg := names.SanitizeZipSegment(fname)
		if prefix != "" {
			seg = prefix + "/" + seg
		}
		rows, err := s.Pool.Query(ctx, `SELECT id, name FROM files WHERE folder_id = $1 AND user_id = $2 AND is_deleted = false`, folderID, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uuid.UUID
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return err
			}
			if err := addFile(seg+"/", id); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
		frows, err := s.Pool.Query(ctx, `SELECT id, name FROM folders WHERE parent_id = $1 AND user_id = $2 AND is_deleted = false`, folderID, userID)
		if err != nil {
			return err
		}
		for frows.Next() {
			var cid uuid.UUID
			var cn string
			if err := frows.Scan(&cid, &cn); err != nil {
				frows.Close()
				return err
			}
			if err := walkFolder(cid, seg); err != nil {
				frows.Close()
				return err
			}
		}
		frows.Close()
		return nil
	}
	for _, fid := range fileIDs {
		if err := addFile("", fid); err != nil {
			return nil, err
		}
	}
	for _, fid := range folderIDs {
		if err := walkFolder(fid, ""); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func WriteZip(w io.Writer, entries []archiveEntry, limits transfer.SpeedLimits) error {
	zw := zip.NewWriter(transfer.ThrottleWriter(w, derefKbps(limits)))
	defer zw.Close()
	for _, e := range entries {
		method := zip.Deflate
		if ZipStoreMIME(e.Mime) {
			method = zip.Store
		}
		h := &zip.FileHeader{Name: e.Path, Method: method}
		wr, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		f, err := os.Open(e.Abs)
		if err != nil {
			return err
		}
		_, err = io.Copy(wr, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func derefKbps(l transfer.SpeedLimits) int {
	if k := transfer.EffectiveLimitKbps(l); k != nil {
		return *k
	}
	return 0
}
