package media

import (
	"context"
	"strings"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/storage" //the media service now needs the service provider (phase 1)
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {

	//Yes. CreateMediaRequest needs to change, because we're
	//no longer creating media from a JSON body containing only project_id; the file is coming through multipart/form-data.
	CreateMedia(
		ctx context.Context,
		ownerID uuid.UUID,
		// req CreateMediaRequest,
		projectID uuid.UUID,
		fileHeader *multipart.FileHeader,

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
	storage           storage.Storage //needa update
	jobService        job.Service
}

func NewService(
	repository Repository,
	projectRepository project.Repository,
	storage storage.Storage,
	jobService job.Service,
) Service {
	return &service{
		repository:        repository,
		projectRepository: projectRepository,
		storage:           storage,
		jobService:        jobService,
	}
}

func (s *service) CreateMedia(
	ctx context.Context,
	ownerID uuid.UUID,
	// req CreateMediaRequest,
	projectID uuid.UUID,
	fileHeader *multipart.FileHeader,
) (*MediaResponse, error) {
	//verify the project exists and belongs to the authenticated user

	_, err := s.projectRepository.GetByID(
		ctx,
		ownerID,
		projectID,
	)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	//open the uploaded multipart file:
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	uploadResult, err := s.storage.Upload(
		ctx,
		storage.UploadInput{
			Reader:   file,
			FileName: fileHeader.Filename,
			MimeType: fileHeader.Header.Get("Content-Type"),
			Size:     fileHeader.Size,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload media: %w", err)
	}

	//persist the uploaded media metadata
	media := &Media{
		ProjectId: projectID,
		// Status:    StatusPending,
		OriginalFileName: uploadResult.OriginalFileName,
		StorageKey:       uploadResult.StorageKey,
		MimeType:         uploadResult.MimeType,
		Size:             uploadResult.Size,
		Status:           StatusUploaded,
	}

	if err := s.repository.Create(ctx, media); err != nil {
		// Database failed after storage succeeded.
		// Clean up the orphaned storage object.
		_ = s.storage.Delete(ctx, uploadResult.StorageKey)
		return nil, fmt.Errorf("failed to create media record: %w", err)
	}

	//after creating the media row, we create the job:
	//create the processingJ*bs , each CreateJ*b() call persists the job and enqueues it into the redis stream
   // ---------------------------------------------------------
    // PROCESSING JOB FAN-OUT
    //
    // Each CreateJob():
    //   1. creates a Job row in PostgreSQL
    //   2. XADDs the job ID into the Redis Stream
    //   3. a worker eventually consumes it
    // ---------------------------------------------------------
	processingJobs := []job.CreateJobRequest{
		{
			MediaID: media.ID,
			Type: job.TypeThumbnail,
		},
		{
			MediaID: media.ID,
			Type: job.TypePreview,
		},

	}

	//videos additionally get 360/720/1080 transcoding jobs
	if strings.HasPrefix(media.MimeType, "video/"){
		for _, height := range []int{360,720,1080}{
			targetHeight := height
			processingJobs = append(
				processingJobs, 
				job.CreateJobRequest{
					MediaID: media.ID,
					Type: job.TypeTranscode,
					TargetHeight: &targetHeight,
				},
			)
		}
		
	}
	//persists + enqueue every processing job:
	for _, jobRequest := range processingJobs{
		if _, err := s.jobService.CreateJob(ctx, jobRequest); err != nil{
			media.Status = StatusFailed
			_ = s.repository.Update(ctx, media)
			return nil , fmt.Errorf("failed to create processing job: %w", err)

		}
	}



	// for _, req := range jobs{
	// 	if _, err := s.jobService.CreateJob(ctx, req); err != nil{
	// 		//the media alr exists, so mark failed if job creation/enqueing fails
	// 		media.Status = StatusFailed
	// 		_ = s.repository.Update(ctx, media)

	// 		return nil, fmt.Errorf("failed to create processing jobs :%w", err,)
	// 	}
	// }

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
