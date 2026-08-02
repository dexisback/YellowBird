package project 



import (
	"context"
	"github.com/google/uuid"
)


type Service interface{
	CreateProject(ctx context.Context, ownerID uuid.UUID,  req CreateProjectRequest) (*ProjectResponse, error)  //now a project accepts userId aswell
	GetProject (ctx context.Context, id uuid.UUID) (*ProjectResponse, error)
	ListProjects(ctx context.Context, ownerID uuid.UUID) ([]ProjectResponse, error)
	UpdateProject(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*ProjectResponse, error)
	DeleteProject(ctx context.Context, id uuid.UUID) error 
}


type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{
		repository: repository,
	}
}


//earlier it was this line -> func (s *service) CreateProject(ctx context.Context, req CreateProjectRequest) (*ProjectResponse, error ){

//now it is this line -> CreateProject(ctx context.Context, ownerID uuid.UUID, req CreateProjectRequest) (*ProjectResponse, error)   (so project now accepts an authenticated user)
func (s *service) CreateProject(ctx context.Context, ownerID uuid.UUID, req CreateProjectRequest) (*ProjectResponse, error ){
	project := &Project{
		OwnerID: ownerID, 
		Name:   req.Name , 
		Description: req.Description,
	}


	if err := s.repository.Create(ctx, project); err != nil{
		return  nil, err
	}

	return &ProjectResponse{
		ID: project.ID,
		Name : project.Name,
		Description: project.Description,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}, nil
}


func (s *service) GetProject(ctx context.Context, id uuid.UUID) (*ProjectResponse, error){
	project, err := s.repository.GetByID(ctx, id)
	if err != nil{
		return nil, err
	}
	return &ProjectResponse{
		ID: project.ID,
		Name: project.Name,
		Description: project.Description,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}, nil
}



func (s *service) ListProjects(ctx context.Context, ownerID uuid.UUID) ([]ProjectResponse, error){
	project, err := s.repository.List(ctx, ownerID)
	if err != nil{
		return nil, err
	}

	response := make([]ProjectResponse, 0 , len(project))
	for _, p := range project{
		response = append(response, ProjectResponse{
			ID: p.ID,
			Name: p.Name,
			Description: p.Description,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		})
	}

	return response, nil
}



func (s *service) UpdateProject(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*ProjectResponse,error){
	project , err := s.repository.GetByID(ctx, id)

	if err != nil{
		return nil, err
	}

	if req.Name != "" {
		project.Name = req.Name
	}

	if req.Description != "" {
		project.Description = req.Description
	}

	if err := s.repository.Update(ctx, project); err != nil{
		return nil, err
	}


	return &ProjectResponse{
		ID: project.ID,

			Name: project.Name,
			Description: project.Description,
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		
	}, nil




}



func (s *service) DeleteProject (ctx context.Context, id uuid.UUID) error {
	return s.repository.Delete(ctx, id)
}