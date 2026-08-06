//supports create/GetByID/ListByProject/Update/Delete

package media

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, media *Media) error
	GetByID(ctx context.Context, id uuid.UUID) (*Media, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]Media, error)
	Update(ctx context.Context, media *Media) error
	Delete(ctx context.Context, id uuid.UUID) error
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
