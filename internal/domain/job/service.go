package job

import (
	"context"
	"errors"
	"time"

	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/google/uuid"
)

type Service interface {
	CreateJob(ctx context.Context, req CreateJobRequest) (*JobResponse, error)
	GetJob(ctx context.Context, id uuid.UUID) (*JobResponse, error)
	GetJobEntity(ctx context.Context, id uuid.UUID) (*Job, error)
	ListJobsByMedia(ctx context.Context, mediaID uuid.UUID) ([]JobResponse, error)
	StartJob(ctx context.Context, id uuid.UUID) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error
	CompleteJob(ctx context.Context, id uuid.UUID) error
	FailJob(ctx context.Context, id uuid.UUID, errMsg string) error
	DeleteJob(ctx context.Context, id uuid.UUID) error
}

//repository = db access, but you dont want everyone having that accewss. so we just create a repository struct each time

type service struct {
	repository Repository
	queue *queue.RedisQueue   //jobservice now owns the transition. DB job creation -> redis job enqueu, now add this to all functions underneath
}

func NewService(repository Repository, q *queue.RedisQueue) Service {
	return &service{
		repository: repository,
		queue:      q,
	}
}

func (s *service) CreateJob(
	ctx context.Context,
	req CreateJobRequest,
) (*JobResponse, error) {

	
	//refine: validate the job type + target target resolution 
	//before creating/persisting the job 


	if err := validateCreateJobRequest(req); err != nil{
		return nil, err 
	}
	job := &Job{
		MediaID:      req.MediaID,
		Type:         req.Type,
		//new : targetHeight
		TargetHeight: req.TargetHeight,

		Status:   StatusQueued,
		Progress: 0,
	}

	if err := s.repository.Create(ctx, job); err != nil {
		return nil, err
	}

	//enqueue the peristed job for the background worker 
	if err := s.queue.Enqueue(ctx, job.ID); err != nil{
		return nil, err
	}


	return toResponse(job), nil
}

func (s *service) GetJob(
	ctx context.Context,
	id uuid.UUID,
) (*JobResponse, error) {

	job, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResponse(job), nil
}



//worker needs the actual job entity rather than the api facing job repsonse
func (s *service) GetJobEntity(
	ctx context.Context,
	id uuid.UUID,
) (*Job, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *service) ListJobsByMedia(
	ctx context.Context,
	mediaID uuid.UUID,
) ([]JobResponse, error) {

	jobs, err := s.repository.ListByMedia(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	response := make([]JobResponse, 0, len(jobs))

	for _, job := range jobs {
		response = append(response, *toResponse(&job))
	}

	return response, nil
}

func (s *service) StartJob(
	ctx context.Context,
	id uuid.UUID,
) error {

	job, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if job.Status != StatusQueued {
		return errors.New("job is not queued")
	}

	now := time.Now()

	job.Status = StatusRunning
	job.StartedAt = &now

	return s.repository.Update(ctx, job)
}

func (s *service) UpdateProgress(
	ctx context.Context,
	id uuid.UUID,
	progress int,
) error {

	job, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return err
	}

	job.Progress = progress

	return s.repository.Update(ctx, job)
}

func (s *service) CompleteJob(
	ctx context.Context,
	id uuid.UUID,
) error {

	job, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()

	job.Progress = 100
	job.Status = StatusCompleted
	job.CompletedAt = &now

	return s.repository.Update(ctx, job)
}

func (s *service) FailJob(
	ctx context.Context,
	id uuid.UUID,
	errMsg string,
) error {

	job, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return err
	}

	now := time.Now()

	job.Status = StatusFailed
	job.Error = errMsg
	job.CompletedAt = &now

	return s.repository.Update(ctx, job)
}

func (s *service) DeleteJob(
	ctx context.Context,
	id uuid.UUID,
) error {
	return s.repository.Delete(ctx, id)
}

func toResponse(job *Job) *JobResponse {
	return &JobResponse{
		ID:           job.ID,
		MediaID:      job.MediaID,
		Type:         job.Type,
		TargetHeight: job.TargetHeight,
		Status:       job.Status,
		Progress:     job.Progress,
		Error:        job.Error,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
		CreatedAt:    job.CreatedAt,
		UpdatedAt:    job.UpdatedAt,
	}
}

// StartJob() → marks the job as running, sets StartedAt.
// UpdateProgress() → updates the progress percentage.
// CompleteJob() → sets progress to 100, marks completed, sets CompletedAt.
// FailJob() → marks failed, stores an error message, sets CompletedAt.
// DeleteJob() → removes the job record



//new: validation function: 
func validateCreateJobRequest(req CreateJobRequest) error {
	switch req.Type{
	case TypeTranscode:
		if req.TargetHeight == nil{
			return errors.New(
				"target_height is required for transcode jobs", 
			)
		}
		switch *req.TargetHeight{
		case 360, 720, 1080:
			return nil
		default:
			return errors.New("unsupported target height; supported values are 360, 720 and 1080")
		 
			
		}
	case TypeThumbnail, TypePreview:
		if req.TargetHeight != nil{
			return errors.New(
				"target_height is only valid for transcode jobs",
			)
		} 

		return nil
		default:
			return errors.New("unsupported job type")
	}
}