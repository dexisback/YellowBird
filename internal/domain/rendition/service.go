package rendition


import(
	"context"
	"github.com/google/uuid"
)

type Service interface {
	CreateRendition(ctx context.Context, req CreateRendtionRequest) (*RenditionResponse, error)
	GetRendition(ctx context.Context, id uuid.UUID) (*RenditionResponse, error)
	ListRenditionsByMedia(ctx context.Context, mediaID uuid.UUID) ([]RenditionResponse, error)
	UpdateRendition(ctx context.Context, id uuid.UUID, req CreateRendtionRequest, ) (*RenditionResponse, error)
	DeleteRendition(ctx context.Context, id uuid.UUID) error

}


type service struct {
	repository Repository

}


func NewService(repository Repository) Service {
	return &service{
		repository: repository,
	}
}


func (s *service) CreateRendition(ctx context.Context, req CreateRendtionRequest) (*RenditionResponse, error) {
	rendition := &Rendition{
			MediaID:         req.MediaID,
		Type:            req.Type,
		StorageKey:      req.StorageKey,
		URL:             req.URL,
		MimeType:        req.MimeType,
		Size:            req.Size,
		Width:           req.Width,
		Height:          req.Height,
		DurationSeconds: req.DurationSeconds,
	}


	if err := s.repository.Create(ctx, rendition) ; err != nil{
		return nil, err
	}
	return toResponse(rendition), nil
}


func (s *service) GetRendition (ctx context.Context, id uuid.UUID,) (*RenditionResponse, error) {
	rendition, err := s.repository.GetByID(ctx, id)
	if err != nil{
		return nil, err
	}
	return toResponse(rendition), nil
} 


func (s *service) ListRenditionsByMedia(ctx context.Context, mediaID uuid.UUID,) ([]RenditionResponse, error) {
	renditions, err := s.repository.ListByMedia(ctx, mediaID)
	if err != nil{
		return nil, err
	}

	response := make([]RenditionResponse, 0, len(renditions))
	for _, rendition := range renditions{
		response = append(response, *toResponse(&rendition))
	}

	return response, nil
}


func (s *service) UpdateRendition(ctx context.Context, id uuid.UUID, req CreateRendtionRequest,)(*RenditionResponse, error){
	rendition, err := s.repository.GetByID(ctx, id)
	if err != nil{
		return nil, err 
	}
	rendition.MediaID = req.MediaID
	rendition.Type = req.Type
	rendition.StorageKey = req.URL
	rendition.MimeType = req.MimeType
	rendition.Size = req.Size
	rendition.Width = req.Width 
	rendition.Height = req.Height
	rendition.DurationSeconds = req.DurationSeconds


	if err := s.repository.Update(ctx, rendition); err != nil{
		return nil, err 
	}
	return toResponse(rendition) , nil
}



func (s *service) DeleteRendition(ctx context.Context, id uuid.UUID) error{
	return s.repository.Delete(ctx, id)
}


func toResponse(rendition *Rendition) *RenditionResponse{
	return &RenditionResponse{
			ID:              rendition.ID,
		MediaID:         rendition.MediaID,
		Type:            rendition.Type,
		StorageKey:      rendition.StorageKey,
		URL:              rendition.URL,
		MimeType:        rendition.MimeType,
		Size:            rendition.Size,
		Width:           rendition.Width,
		Height:          rendition.Height,
		DurationSeconds: rendition.DurationSeconds,
		CreatedAt:       rendition.CreatedAt,
		UpdatedAt:       rendition.UpdatedAt,
	}
}