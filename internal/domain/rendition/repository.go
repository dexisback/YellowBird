package rendition
import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, renditoin *Rendition) error
	GetByID(ctx context.Context, id uuid.UUID) (*Rendition, error)
	ListByMedia(ctx context.Context, mediaID uuid.UUID,) ([]Rendition, error) 
	Update(ctx context.Context, rendition *Rendition) error
	Delete(ctx context.Context, id uuid.UUID) error

}


type repository struct {
	db *gorm.DB
}

func NewRepository()

func (r *repository) Create(ctx context.Context, rendition *Rendition,) error {return r.db.WithContext(ctx).Create(rendition).Error}
func (r *repository) GetByID(ctx context.Context, id uuid.UUID,) (*Rendition, error) {
	var rendition Rendition
	if err := r.db.WithContext(ctx).First(&rendition, "id = ?", id).Error; err != nil {return nil, err} return &rendition, err }

} 