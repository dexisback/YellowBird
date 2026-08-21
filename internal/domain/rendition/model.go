package rendition
import (
	"time"
	"github.com/google/uuid"

)


type RenditionType string 


const (
	TypeThumbnail RenditionType = "thumbnail"
	TypePreview RenditionType = "preview"
	TypeTranscode RenditionType = "transcode"
	
)


type Rendition struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	MediaID uuid.UUID `gorm:"type:uuid;not null;index"`
	Type RenditionType `gorm:"type:varchar(50);not null"`
	StorageKey string `gorm:"size:500;not null;unique"`
	URL string `gorm:"type:text;not null"`
	MimeType string `gorm:"size:255;not null"`
	Size int64 `gorm:"not null"`
	Width *int
	Height *int
	DurationSeconds *float64
	CreatedAt time.Time
	UpdatedAt time.Time
} 