package rendition

import (
	"time"

	"github.com/google/uuid"
)

type CreateRendtionRequest struct {
	MediaID         uuid.UUID     `json:"media_id" binding:"required"`
	Type            RenditionType `json:"type" binding:"required"`
	StorageKey      string        `json:"storage_key" binding:"required"`
	URL             string        `json:"url" binding:"required"`
	MimeType        string        `json:"mime_type" binding:"required"`
	Size            int64         `json:"size" binding:"gte=0"`
	Width           *int          `json:"width"`
	Height          *int          `json:"height"`
	DurationSeconds *float64      `json:"duration_seconds"`
}

type RenditionResponse struct {
	ID uuid.UUID `json:"id"`
	MediaID uuid.UUID `json:"media_id"`
	Type RenditionType `json:"type"`
	StorageKey string `json:"storage_key"`
	URL string `json:"url"`
	MimeType string `json:"mime_type"`
	Size int64 `json:"size"`
	Width *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}



