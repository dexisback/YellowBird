package project

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	CreateProject(ctx context.Context, ownerID uuid.UUID, req CreateProjectRequest) (*ProjectResponse, error)
	GetProject(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (*ProjectResponse, error)
	ListProjects(ctx context.Context, ownerID uuid.UUID) ([]ProjectResponse, error)
	UpdateProject(ctx context.Context, ownerID uuid.UUID, id uuid.UUID, req UpdateProjectRequest) (*ProjectResponse, error)
	DeleteProject(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) error
}

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{
		repository: repository,
	}
}

func (s *service) CreateProject(
	ctx context.Context,
	ownerID uuid.UUID,
	req CreateProjectRequest,
) (*ProjectResponse, error) {
	project := &Project{
		OwnerID:    ownerID,
		Name:       req.Name,
		Description: req.Description,
	}

	if err := s.repository.Create(ctx, project); err != nil {
		return nil, err
	}

	return &ProjectResponse{
		ID:          project.ID,
		OwnerID:     project.OwnerID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *service) GetProject(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID,
) (*ProjectResponse, error) {
	project, err := s.repository.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}

	return &ProjectResponse{
		ID:          project.ID,
		OwnerID:     project.OwnerID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *service) ListProjects(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]ProjectResponse, error) {
	projects, err := s.repository.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	response := make([]ProjectResponse, 0, len(projects))

	for _, project := range projects {
		response = append(response, ProjectResponse{
			ID:          project.ID,
			OwnerID:     project.OwnerID,
			Name:        project.Name,
			Description: project.Description,
			CreatedAt:   project.CreatedAt,
			UpdatedAt:   project.UpdatedAt,
		})
	}

	return response, nil
}

func (s *service) UpdateProject(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID,
	req UpdateProjectRequest,
) (*ProjectResponse, error) {
	project, err := s.repository.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}

	project.Name = req.Name
	project.Description = req.Description

	if err := s.repository.Update(ctx, ownerID, project); err != nil {
		return nil, err
	}

	return &ProjectResponse{
		ID:          project.ID,
		OwnerID:     project.OwnerID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *service) DeleteProject(
	ctx context.Context,
	ownerID uuid.UUID,
	id uuid.UUID,
) error {
	return s.repository.Delete(ctx, ownerID, id)
}