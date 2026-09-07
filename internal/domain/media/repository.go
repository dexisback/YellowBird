//supports create/GetByID/ListByProject/Update/Delete

package media

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Create(ctx context.Context, media *Media) error
	GetByID(ctx context.Context, id uuid.UUID) (*Media, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]Media, error)
	Update(ctx context.Context, media *Media) error
	Delete(ctx context.Context, id uuid.UUID) error
	SyncStatus(ctx context.Context, mediaID uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Create(ctx context.Context, media *Media) error {
	return r.db.WithContext(ctx).Create(media).Error
}

func (r *repository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*Media, error) {
	var media Media
	err := r.db.WithContext(ctx).First(&media, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &media, nil
}

func (r *repository) ListByProject(
	ctx context.Context,
	projectID uuid.UUID,
) ([]Media, error) {
	var media []Media
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&media).Error

	if err != nil {
		return nil, err
	}

	if media == nil {
		media = []Media{}
	}

	return media, nil
}

func (r *repository) Update(
	ctx context.Context,
	media *Media,
) error {
	return r.db.WithContext(ctx).Save(media).Error
}

func (r *repository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&Media{}).Error
}

func (r *repository) SyncStatus(ctx context.Context, mediaID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m Media
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", mediaID).Error; err != nil {
			return err
		}

		if m.Status == StatusFailed {
			return nil
		}

		var jobs []job.Job
		if err := tx.Where("media_id = ?", mediaID).Find(&jobs).Error; err != nil {
			return err
		}
		if len(jobs) == 0 {
			return nil
		}

		hasFailed := false
		allCompleted := true
		hasRunningOrQueued := false

		for _, j := range jobs {
			if j.Status == job.StatusFailed {
				hasFailed = true
			}
			if j.Status != job.StatusCompleted {
				allCompleted = false
			}
			if j.Status == job.StatusRunning || j.Status == job.StatusQueued {
				hasRunningOrQueued = true
			}
		}

		var newStatus MediaStatus
		if hasFailed {
			newStatus = StatusFailed
		} else if allCompleted {
			newStatus = StatusReady
		} else if hasRunningOrQueued {
			if m.Status == StatusUploaded || m.Status == StatusPending {
				newStatus = StatusProcessing
			} else {
				return nil
			}
		} else {
			return nil
		}

		if m.Status != newStatus {
			m.Status = newStatus
			return tx.Save(&m).Error
		}
		return nil
	})
}
