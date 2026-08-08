package job

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)
//repository interface tells and says what can be done
//the struct says "what data i have and how do i do it"
type Repository interface {
	Create(ctx context.Context, job *Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*Job, error)
	ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]Job, error)
	ListQueued(ctx context.Context) ([]Job, error)
	Update(ctx context.Context, job *Job) error
	Delete(ctx context.Context, id uuid.UUID) error
}
type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(
	ctx context.Context, job *Job,
) error {
	return r.db.WithContext(ctx).Create(job).Error

}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Job, error) {
	var job Job
	if err := r.db.WithContext(ctx).First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *repository) ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]Job, error) {
	var jobs []Job

	err := r.db.WithContext(ctx).Where("media_id = ?", mediaID).
		Order("created_at DESC").
		Find(&jobs).Error

	return jobs, err
}

//this fetches all jobs from the db whose status is `QUEUED`, ordered from oldest to newest

func (r *repository) ListQueued(ctx context.Context) ([]Job, error) {
	var jobs []Job
	err := r.db.WithContext(ctx).Where("status = ?", StatusQueued).
		Order("created_at ASC").
		Find(&jobs).Error

	return jobs, err
}

func (r *repository) Update(
	ctx context.Context, job *Job,
) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *repository) Delete(
	ctx context.Context, id uuid.UUID,
) error {

	return r.db.WithContext(ctx).
		Delete(&Job{}, "id = ?", id).Error
}

//eventually the worker will do something like : jobs, err:= jobRepository.ListQueued(ctx)
// for _, job := range jobs {//provess with ffmpeg}.
