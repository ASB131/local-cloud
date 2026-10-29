//go:build integration

package files

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/asb131/local-cloud/internal/db"
	"github.com/asb131/local-cloud/internal/names"
)

func TestListFilesByCategoryWithFolderJoin(t *testing.T) {
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
	defer pool.Close()

	userID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	rootID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	subID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	fileID, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()

	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, username, password_hash, storage_quota_bytes, storage_used_bytes, is_admin, created_at, updated_at)
		VALUES ($1, $2, 'x', 1073741824, 0, false, $3, $3)
	`, userID, "cat-int-"+userID.String()[:8], now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO folders (id, user_id, parent_id, name, name_key, created_at, updated_at)
		VALUES ($1, $2, NULL, 'Root', 'root', $3, $3)
	`, rootID, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO folders (id, user_id, parent_id, name, name_key, created_at, updated_at)
		VALUES ($1, $2, $3, 'src', 'src', $4, $4)
	`, subID, userID, rootID, now)
	if err != nil {
		t.Fatal(err)
	}
	key := names.NameKey("main.go")
	_, err = pool.Exec(ctx, `
		INSERT INTO files (
			id, user_id, folder_id, name, name_key, size_bytes, mime_type, disk_path, file_hash,
			thumbnail_status, office_preview_status, created_at, updated_at
		) VALUES ($1,$2,$3,'main.go',$4,12,'text/plain','users/x/objects/x/v1','abc','skipped','skipped',$5,$5)
	`, fileID, userID, subID, key, now)
	if err != nil {
		t.Fatal(err)
	}

	store := &Store{Pool: pool}
	list, err := store.ListFilesByCategory(ctx, userID, "code", 10)
	if err != nil {
		t.Fatalf("ListFilesByCategory: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 file, got %d", len(list))
	}
	if list[0].FolderName != "src" {
		t.Fatalf("folder_name: got %q", list[0].FolderName)
	}

	_, _ = pool.Exec(ctx, `DELETE FROM files WHERE user_id = $1`, userID)
	_, _ = pool.Exec(ctx, `DELETE FROM folders WHERE user_id = $1`, userID)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}
