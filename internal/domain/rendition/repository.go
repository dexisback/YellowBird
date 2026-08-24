package rendition

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, rendition *Rendition) error
	GetByID(ctx context.Context, id uuid.UUID) (*Rendition, error)
	ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]Rendition, error)
	Update(ctx context.Context, rendition *Rendition) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, rendition *Rendition) error {
	return r.db.WithContext(ctx).Create(rendition).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*Rendition, error) {
	var rendition Rendition
	err := r.db.WithContext(ctx).First(&rendition, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rendition, nil
}

func (r *repository) ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]Rendition, error) {
	var renditions []Rendition
	err := r.db.WithContext(ctx).Where("media_id = ?", mediaID).Find(&renditions).Error
	if err != nil {
		return nil, err
	}
	if renditions == nil {
		renditions = []Rendition{}
	}
	return renditions, nil
}

func (r *repository) Update(ctx context.Context, rendition *Rendition) error {
	return r.db.WithContext(ctx).Save(rendition).Error
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&Rendition{}).Error
}
