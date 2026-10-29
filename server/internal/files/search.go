package files

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type SearchHit struct {
	Kind string `json:"kind"`
	Item any    `json:"item"`
}

type SearchFileDTO struct {
	FileDTO
	FolderName string `json:"folder_name"`
}

func escapeILIKE(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return q
}

func nameTokens(q string) []string {
	parts := strings.Fields(strings.TrimSpace(q))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildNamePredicates(alias string, tokens []string, startArg int) (string, []any) {
	if len(tokens) == 0 {
		return "TRUE", nil
	}
	args := make([]any, 0, len(tokens))
	clauses := make([]string, 0, len(tokens))
	for i, t := range tokens {
		args = append(args, "%"+escapeILIKE(t)+"%")
		clauses = append(clauses, fmt.Sprintf("%s.name ILIKE $%d ESCAPE '\\'", alias, startArg+i))
	}
	return strings.Join(clauses, " AND "), args
}

func (s *Store) Search(ctx context.Context, userID uuid.UUID, q string) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, ErrValidation
	}
	tokens := nameTokens(q)
	if len(tokens) == 0 {
		return nil, ErrValidation
	}

	folderPred, folderArgs := buildNamePredicates("folders", tokens, 2)
	filePred, filePredArgs := buildNamePredicates("f", tokens, 2)

	var hits []SearchHit

	fQuery := fmt.Sprintf(`
		SELECT id, parent_id, name, created_at, updated_at FROM folders
		WHERE user_id = $1 AND is_deleted = false AND parent_id IS NOT NULL AND %s
		ORDER BY name
		LIMIT 50
	`, folderPred)
	fArgs := append([]any{userID}, folderArgs...)
	frows, err := s.Pool.Query(ctx, fQuery, fArgs...)
	if err != nil {
		return nil, err
	}
	for frows.Next() {
		var f FolderDTO
		var pid uuid.UUID
		if err := frows.Scan(&f.ID, &pid, &f.Name, &f.CreatedAt, &f.UpdatedAt); err != nil {
			frows.Close()
			return nil, err
		}
		f.ParentID = &pid
		hits = append(hits, SearchHit{Kind: "folder", Item: f})
	}
	frows.Close()

	fileQuery := fmt.Sprintf(`
		SELECT f.id, f.folder_id, f.name, f.size_bytes, f.mime_type, f.file_hash,
		       f.meta_created_at, f.meta_modified_at, f.created_at, f.updated_at, f.thumbnail_status,
		       EXISTS(SELECT 1 FROM favorites fav WHERE fav.user_id = $1 AND fav.file_id = f.id),
		       COALESCE(fold.name, '')
		FROM files f
		LEFT JOIN folders fold ON fold.id = f.folder_id AND fold.user_id = f.user_id
		WHERE f.user_id = $1 AND f.is_deleted = false AND %s
		ORDER BY f.name
		LIMIT 100
	`, filePred)
	fileQueryArgs := append([]any{userID}, filePredArgs...)
	rows, err := s.Pool.Query(ctx, fileQuery, fileQueryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var file SearchFileDTO
		if err := rows.Scan(
			&file.ID, &file.FolderID, &file.Name, &file.SizeBytes, &file.MimeType, &file.FileHash,
			&file.MetaCreatedAt, &file.MetaModifiedAt, &file.CreatedAt, &file.UpdatedAt, &file.ThumbnailStatus, &file.IsFavorite,
			&file.FolderName,
		); err != nil {
			return nil, err
		}
		hits = append(hits, SearchHit{Kind: "file", Item: file})
	}
	if hits == nil {
		hits = []SearchHit{}
	}
	return hits, rows.Err()
}
