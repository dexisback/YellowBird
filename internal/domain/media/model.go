package media

import (
	"time"

	"github.com/google/uuid"
)

type MediaStatus string

// define all statuses , state machine
const (
	StatusPending    MediaStatus = "pending"
	StatusUploading  MediaStatus = "uploading"
	StatusUploaded   MediaStatus = "uploaded"
	StatusProcessing MediaStatus = "processing"
	StatusReady      MediaStatus = "ready"
	StatusFailed     MediaStatus = "failed"
)

type Media struct {
	ID               uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectId        uuid.UUID   `gorm:"type:uuid;not null;index"`
	OriginalFileName string      `gorm:"size:255;not null"`
	StorageKey       string      `gorm:"size:500;not null; unique"`
	MimeType         string      `gorm:"size:255;not null"`
	Status           MediaStatus `gorm:"size:255;not null"`
	DurationSeconds  *float64
	Width            *int
	Height           *int

	CreatedAt time.Time
	UpdatedAt time.Time
}

//originalFileName  -> movie.mp4
// //storage Key (S3 key for that media/cloudinary public key if we using cloudinary, etc )
// //projectID -> links the media file to a project
// //Status -> upload lifecycle (pending->uploaded->processing->ready/failed)
// //width, height, durationSeconds are pointers because they arent known immediately after uploade; fmmpeg fills them in later
