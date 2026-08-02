package project

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (*Project, error)
	List(ctx context.Context, ownerID uuid.UUID) ([]Project, error)
	Update(ctx context.Context, ownerID uuid.UUID, project *Project) error
	Delete(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Create(ctx context.Context, project *Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *repository) GetByID(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID) (*Project, error) {
	var project Project

	err := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&project).Error

	if err != nil {
		return nil, err
	}

	return &project, nil
}

// before auth middleware -> gave every project
// after auth middleware -> gives only the project owned by a particular authenticated user
func (r *repository) List(
	ctx context.Context,
	ownerID uuid.UUID) ([]Project, error) {
	var projects []Project

	err := r.db.WithContext(ctx).Where("owner_id= ?", ownerID).Find(&projects).Error
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (r *repository) Update(
	ctx context.Context,
	ownerID uuid.UUID,
	project *Project) error {
	return r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", project.ID, ownerID).Updates(project).Error
}

func (r *repository) Delete(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&Project{}).Error

}
