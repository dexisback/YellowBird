//later we will have jobs like transcode_1080p, transcode_720p, extract_audio, generate_preview

package job

import (
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

type JobType string

const (
	TypeThumbnail JobType = "thumbnail"
	TypeTranscode JobType = "transcode"
	TypePreview   JobType = "preview"
)

type Job struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	MediaID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Type         JobType   `gorm:"type:varchar(50);not null"`
	TargetHeight *int      `gorm:"type:int"`

	Status      JobStatus `gorm:"type:varchar(50);not null; default;'queued'"`
	Progress    int       `gorm:"default:0"`
	Error       string    `gorm:"type:text"`
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
