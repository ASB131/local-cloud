package files

import (
	"context"

	"github.com/google/uuid"
)

func (s *Store) folderPathSegments(ctx context.Context, userID, folderID uuid.UUID) ([]string, error) {
	_, crumbs, err := s.GetFolder(ctx, userID, folderID)
	if err != nil {
		return nil, err
	}
	var path []string
	for _, c := range crumbs {
		if c.ParentID != nil && c.Name != "" {
			path = append(path, c.Name)
		}
	}
	return path, nil
}
