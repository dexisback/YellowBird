package job

import (
	"context"
	"errors"
	"time"
	"github.com/google/uuid"
)


type Service interface {
	CreateJob(ctx context.Context, req CreateJobRequest, ) (*JobResponse, error)
	GetJob(ctx context.Context, id uuid.UUID,) (*JobResponse, error)
	ListJobsByMedia(ctx context.Context, mediaID uuid.UUID,) ([]JobResponse , error)
	StartJob(ctx context.Context, id uuid.UUID, ) error 
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int,) error
	CompleteJob (ctx context.Context, id uuid.UUID,) error
	FailJob (ctx context.Context, id uuid.UUID, errMsg string) error
	DeleteJob (ctx context.Context, id uuid.UUID,) error 

}

//repository = db access, but you dont want everyone having that accewss. so we just create a repository struct each time 

type service struct {
	repository Repository 
}



func NewService(repository Repository) Service {
	return &service{
		repository: repository,
	}
}

func (s *service) CreateJob(
	ctx context.Context,
	req CreateJobRequest,
) (*JobResponse, error) {

	job := &Job{
		MediaID:  req.MediaID,
		Type:     req.Type,
		Status:   StatusQueued,
		Progress: 0,
	}

	if err := s.repository.Create(ctx, job); err != nil {
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
		ID:          job.ID,
		MediaID:     job.MediaID,
		Type:        job.Type,
		Status:      job.Status,
		Progress:    job.Progress,
		Error:       job.Error,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}



// StartJob() → marks the job as running, sets StartedAt.
// UpdateProgress() → updates the progress percentage.
// CompleteJob() → sets progress to 100, marks completed, sets CompletedAt.
// FailJob() → marks failed, stores an error message, sets CompletedAt.
// DeleteJob() → removes the job record