package files

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type FileCategorySummary struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

var fileCategoryDefs = []struct {
	ID    string
	Label string
	Where string
}{
	{"images", "Images", `f.mime_type ILIKE 'image/%'`},
	{"videos", "Videos", `f.mime_type ILIKE 'video/%'`},
	{"audio", "Audio", `f.mime_type ILIKE 'audio/%'`},
	{"pdfs", "PDF documents", `f.mime_type = 'application/pdf' OR f.name ~* '\.pdf$'`},
	{"archives", "Compressed files", `f.mime_type ILIKE '%zip%' OR f.mime_type ILIKE '%archive%' OR f.name ~* '\.(zip|rar|7z|tar|gz|bz2)$'`},
	{"documents", "Documents", `f.mime_type ILIKE 'text/%' OR f.mime_type ILIKE '%word%' OR f.name ~* '\.(doc|docx|odt|rtf|txt|md)$'`},
	{"spreadsheets", "Spreadsheets", `f.mime_type ILIKE '%spreadsheet%' OR f.mime_type ILIKE '%excel%' OR f.name ~* '\.(xls|xlsx|ods|csv)$'`},
	{"presentations", "Presentations", `f.mime_type ILIKE '%presentation%' OR f.name ~* '\.(ppt|pptx|odp)$'`},
	{"code", "Code & markup", `f.name ~* '\.(js|ts|tsx|jsx|py|go|rs|java|c|cpp|cs|html|css|json|xml|yaml|yml|sql|sh|rb|php|swift|kt)$'`},
}

func categoryWhere(category string) (string, error) {
	for _, c := range fileCategoryDefs {
		if c.ID == category {
			return c.Where, nil
		}
	}
	return "", ErrValidation
}

func (s *Store) FileCategorySummaries(ctx context.Context, userID uuid.UUID) ([]FileCategorySummary, error) {
	out := make([]FileCategorySummary, 0, len(fileCategoryDefs))
	for _, c := range fileCategoryDefs {
		var n int
		q := fmt.Sprintf(`
			SELECT COUNT(*) FROM files f
			WHERE f.user_id = $1 AND f.is_deleted = false AND (%s)
		`, c.Where)
		if err := s.Pool.QueryRow(ctx, q, userID).Scan(&n); err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, FileCategorySummary{ID: c.ID, Label: c.Label, Count: n})
		}
	}
	return out, nil
}

type CategoryFileDTO struct {
	FileDTO
	FolderName string `json:"folder_name"`
}

func (s *Store) ListFilesByCategory(ctx context.Context, userID uuid.UUID, category string, limit int) ([]CategoryFileDTO, error) {
	where, err := categoryWhere(category)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := fmt.Sprintf(`
		SELECT f.id, f.folder_id, f.name, f.size_bytes, f.mime_type, f.file_hash,
		       f.meta_created_at, f.meta_modified_at, f.created_at, f.updated_at, f.thumbnail_status,
		       EXISTS(SELECT 1 FROM favorites fav WHERE fav.user_id = $1 AND fav.file_id = f.id),
		       COALESCE(fold.name, '')
		FROM files f
		LEFT JOIN folders fold ON fold.id = f.folder_id AND fold.user_id = f.user_id
		WHERE f.user_id = $1 AND f.is_deleted = false AND (%s)
		ORDER BY f.updated_at DESC
		LIMIT $2
	`, where)
	rows, err := s.Pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CategoryFileDTO
	for rows.Next() {
		var file CategoryFileDTO
		if err := rows.Scan(
			&file.ID, &file.FolderID, &file.Name, &file.SizeBytes, &file.MimeType, &file.FileHash,
			&file.MetaCreatedAt, &file.MetaModifiedAt, &file.CreatedAt, &file.UpdatedAt, &file.ThumbnailStatus, &file.IsFavorite,
			&file.FolderName,
		); err != nil {
			return nil, err
		}
		list = append(list, file)
	}
	if list == nil {
		list = []CategoryFileDTO{}
	}
	return list, rows.Err()
}
