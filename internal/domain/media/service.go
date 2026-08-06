package media

import (
	"context"
	"errors"

	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
	projectRepository project.Repository
}

func NewService(
	repository Repository,
	projectRepository project.Repository,
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	media := &Media{
		ProjectId: req.ProjectID,
		Status:    StatusPending,
	}

	if err := s.repository.Create(ctx, media); err != nil {
		return nil, err
	}

	return toResponse(media), nil
}

func (s *service) GetMedia(
	ctx context.Context,
	id uuid.UUID,
) (*MediaResponse, error) {
	media, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResponse(media), nil
}

func (s *service) ListMedia(
	ctx context.Context,
	projectID uuid.UUID,
) ([]MediaResponse, error) {
	mediaList, err := s.repository.ListByProject(ctx, projectID)

	if err != nil {
		return nil, err
	}
	response := make([]MediaResponse, 0, len(mediaList))

	for _, media := range mediaList {
		response = append(response, *toResponse(&media))
	}

	return response, nil
}

func (s *service) UpdateMedia(
	ctx context.Context,
	id uuid.UUID,
	req UpdateMediaRequest,
) (*MediaResponse, error) {
	media, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	media.Status = req.Status

	if err := s.repository.Update(ctx, media); err != nil {
		return nil, err
	}

	return toResponse(media), nil
}

func (s *service) DeleteMedia(
	ctx context.Context,
	id uuid.UUID,
) error {
	return s.repository.Delete(ctx, id)
}

func toResponse(media *Media) *MediaResponse {
	return &MediaResponse{
		ID:               media.ID,
		ProjectID:        media.ProjectId,
		OriginalFileName: media.OriginalFileName,
		StorageKey:       media.StorageKey,
		MimeType:         media.MimeType,
		Size:             media.Size,
		Status:           media.Status,
		DurationSeconds:  media.DurationSeconds,
		Width:            media.Width,
		Height:           media.Height,
		CreatedAt:        media.CreatedAt,
		UpdatedAt:        media.UpdatedAt,
	}
}

//one thing you'll notice is that CreateMedia() currently only creates the db record
//
// //we are intentionally not handling the file uploads yet
// //the upload pipeline will be built separately
//
