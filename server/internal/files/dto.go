package files

import (
	"time"

	"github.com/google/uuid"
)

type FolderDTO struct {
	ID        uuid.UUID `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FileDTO struct {
	ID               uuid.UUID  `json:"id"`
	FolderID         uuid.UUID  `json:"folder_id"`
	Name             string     `json:"name"`
	SizeBytes        int64      `json:"size_bytes"`
	MimeType         string     `json:"mime_type"`
	FileHash         string     `json:"file_hash"`
	MetaCreatedAt    *time.Time `json:"meta_created_at"`
	MetaModifiedAt   *time.Time `json:"meta_modified_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ThumbnailStatus      string `json:"thumbnail_status"`
	OfficePreviewStatus  string `json:"office_preview_status"`
	IsFavorite           bool   `json:"is_favorite"`
}

type ChildrenResponse struct {
	Folders []FolderDTO `json:"folders"`
	Files   []FileDTO   `json:"files"`
}

type VersionDTO struct {
	ID         uuid.UUID `json:"id"`
	VersionNum int       `json:"version_num"`
	SizeBytes  int64     `json:"size_bytes"`
	FileHash   string    `json:"file_hash"`
	CreatedAt  time.Time `json:"created_at"`
}

type TrashItem struct {
	Kind      string     `json:"kind"`
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	SizeBytes *int64     `json:"size_bytes,omitempty"`
	DeletedAt time.Time  `json:"deleted_at"`
}

type FavoriteDTO struct {
	ID       uuid.UUID  `json:"id"`
	FileID   *uuid.UUID `json:"file_id,omitempty"`
	FolderID *uuid.UUID `json:"folder_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
