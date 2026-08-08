package job


import (
	"time"
	"github.com/google/uuid"
)


type CreateJobRequest struct {
	MediaID  uuid.UUID  `json:"media_id" binding:"required"`
	Type    JobType   `json:"type" binding:"required"`

}


type UpdateJobRequest struct{
	Status   JobStatus `json:"status"`
	Progress int `json:"progress"`
	Error    string  `json:"error"`
}

type JobResponse struct {
	ID    uuid.UUID  `json:"id"`
	MediaID   uuid.UUID  `json:"media_id"`
	Type  JobType    `json:"type"`
	Status   JobStatus `json:"status"`
	Progress int `json:"progress"`
	Error   string   `json:"error,omitempty"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

}


