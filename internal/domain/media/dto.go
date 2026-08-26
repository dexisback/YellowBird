package media

import (
	"time"

	"github.com/google/uuid"
)

type CreateMediaRequest struct {
	ProjectID uuid.UUID `form:"project_id" binding:"required"`
}

type UpdateMediaRequest struct {
	Status MediaStatus `json:"status"`
}

type MediaResponse struct {
	ID               uuid.UUID `json:"id"`
	ProjectID        uuid.UUID `json:"project_id"`
	OriginalFileName string    `json:"original_file_name"`
	StorageKey       string    `json:"storage_key"`

	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`

	Status          MediaStatus `json:"status"`
	DurationSeconds *float64    `json:"duration_seconds,omitempty"`

	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

//why doesnt CreateMediaRequest have things like OriginalFileName/StorageKey/MimeType/Size? because we're going to implement upload like:
// //client -> multipart/form-data -> gin -> read uploaded file -> now here backend extracts filename, mime type, size, storage key -> and then we reach service
// //so the client NEVER SENDS THOSE VALUES in JSON , they come from the uploaded file itself
