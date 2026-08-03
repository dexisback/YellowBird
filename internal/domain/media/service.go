package media

import (
	"context"
	"errors"

	"github.com/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/google/uuid"
)

type Service interface {
	CreateMedia(
		ctx context.Context,
		ownerID uuid.UUID,
		req CreateMediaRequest,

	) (*MediaResponse, error)

	GetMedia(
		ctx context.Context,
		id uuid.UUID,
	) (*MediaResponse, error)

	ListMedia(
		ctx context.Context,
		projectID uuid.UUID,

	) ([]MediaResponse, error)

	UpdateMedia(
		ctx context.Context,
		id uuid.UUID,
		req UpdateMediaRequest,
	) (*MediaResponse, error)

	DeleteMedia(
		ctx context.Context,
		id uuid.UUID,
	) error
}

type service struct {
	repository        Repository
	projectRepository project.NewRepository
}

func NewService(
	repository Repository,
	projectRepository project.NewRepository,
) Service {
	return &service{
		repository:        repository,
		projectRepository: projectRepository,
	}
}

func (s *service) CreateMedia(
	ctx context.Context,
	ownerID uuid.UUID,
	req CreateMediaRequest,
) (*MediaResponse, error) {
	//verify the project exists and belongs to the authenticated user

	_, err := s.projectRepository.GetByID(
		ctx,
		ownerID,
		req.ProjectID,
	)

	if err != nil {
		return nil, errors.New("project not found")
	}

}
