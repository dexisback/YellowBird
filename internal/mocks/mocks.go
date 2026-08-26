// Package mocks provides testify-based mock implementations of the domain
// interfaces. Tests across packages (job, media, worker, ...) depend on these
// so they can exercise service/worker logic without a real database, storage
// provider, or message broker.
package mocks

import (
	"context"
	"io"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// job.Repository
// ---------------------------------------------------------------------------

type MockJobRepository struct {
	mock.Mock
}

func (m *MockJobRepository) Create(ctx context.Context, j *job.Job) error {
	return m.Called(ctx, j).Error(0)
}

func (m *MockJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*job.Job), args.Error(1)
}

func (m *MockJobRepository) ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]job.Job, error) {
	args := m.Called(ctx, mediaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]job.Job), args.Error(1)
}

func (m *MockJobRepository) ListQueued(ctx context.Context) ([]job.Job, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]job.Job), args.Error(1)
}

func (m *MockJobRepository) Update(ctx context.Context, j *job.Job) error {
	return m.Called(ctx, j).Error(0)
}

func (m *MockJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ---------------------------------------------------------------------------
// job.Service
// ---------------------------------------------------------------------------

type MockJobService struct {
	mock.Mock
}

func (m *MockJobService) CreateJob(ctx context.Context, req job.CreateJobRequest) (*job.JobResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*job.JobResponse), args.Error(1)
}

func (m *MockJobService) GetJob(ctx context.Context, id uuid.UUID) (*job.JobResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*job.JobResponse), args.Error(1)
}

func (m *MockJobService) GetJobEntity(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*job.Job), args.Error(1)
}

func (m *MockJobService) ListJobsByMedia(ctx context.Context, mediaID uuid.UUID) ([]job.JobResponse, error) {
	args := m.Called(ctx, mediaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]job.JobResponse), args.Error(1)
}

func (m *MockJobService) StartJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockJobService) UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error {
	return m.Called(ctx, id, progress).Error(0)
}

func (m *MockJobService) CompleteJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockJobService) FailJob(ctx context.Context, id uuid.UUID, errMsg string) error {
	return m.Called(ctx, id, errMsg).Error(0)
}

func (m *MockJobService) DeleteJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockJobService) RetryJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ---------------------------------------------------------------------------
// media.Repository
// ---------------------------------------------------------------------------

type MockMediaRepository struct {
	mock.Mock
}

func (m *MockMediaRepository) Create(ctx context.Context, media *media.Media) error {
	return m.Called(ctx, media).Error(0)
}

func (m *MockMediaRepository) GetByID(ctx context.Context, id uuid.UUID) (*media.Media, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*media.Media), args.Error(1)
}

func (m *MockMediaRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]media.Media, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]media.Media), args.Error(1)
}

func (m *MockMediaRepository) Update(ctx context.Context, media *media.Media) error {
	return m.Called(ctx, media).Error(0)
}

func (m *MockMediaRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ---------------------------------------------------------------------------
// project.Repository
// ---------------------------------------------------------------------------

type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Create(ctx context.Context, p *project.Project) error {
	return m.Called(ctx, p).Error(0)
}

func (m *MockProjectRepository) GetByID(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) (*project.Project, error) {
	args := m.Called(ctx, ownerID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*project.Project), args.Error(1)
}

func (m *MockProjectRepository) List(ctx context.Context, ownerID uuid.UUID) ([]project.Project, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]project.Project), args.Error(1)
}

func (m *MockProjectRepository) Update(ctx context.Context, ownerID uuid.UUID, p *project.Project) error {
	return m.Called(ctx, ownerID, p).Error(0)
}

func (m *MockProjectRepository) Delete(ctx context.Context, ownerID uuid.UUID, id uuid.UUID) error {
	return m.Called(ctx, ownerID, id).Error(0)
}

// ---------------------------------------------------------------------------
// rendition.Repository
// ---------------------------------------------------------------------------

type MockRenditionRepository struct {
	mock.Mock
}

func (m *MockRenditionRepository) Create(ctx context.Context, r *rendition.Rendition) error {
	return m.Called(ctx, r).Error(0)
}

func (m *MockRenditionRepository) GetByID(ctx context.Context, id uuid.UUID) (*rendition.Rendition, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rendition.Rendition), args.Error(1)
}

func (m *MockRenditionRepository) ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]rendition.Rendition, error) {
	args := m.Called(ctx, mediaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]rendition.Rendition), args.Error(1)
}

func (m *MockRenditionRepository) Update(ctx context.Context, r *rendition.Rendition) error {
	return m.Called(ctx, r).Error(0)
}

func (m *MockRenditionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ---------------------------------------------------------------------------
// rendition.Service
// ---------------------------------------------------------------------------

type MockRenditionService struct {
	mock.Mock
}

func (m *MockRenditionService) CreateRendition(ctx context.Context, req rendition.CreateRendtionRequest) (*rendition.RenditionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rendition.RenditionResponse), args.Error(1)
}

func (m *MockRenditionService) GetRendition(ctx context.Context, id uuid.UUID) (*rendition.RenditionResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rendition.RenditionResponse), args.Error(1)
}

func (m *MockRenditionService) ListRenditionsByMedia(ctx context.Context, mediaID uuid.UUID) ([]rendition.RenditionResponse, error) {
	args := m.Called(ctx, mediaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]rendition.RenditionResponse), args.Error(1)
}

func (m *MockRenditionService) UpdateRendition(ctx context.Context, id uuid.UUID, req rendition.CreateRendtionRequest) (*rendition.RenditionResponse, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rendition.RenditionResponse), args.Error(1)
}

func (m *MockRenditionService) DeleteRendition(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ---------------------------------------------------------------------------
// storage.Storage
// ---------------------------------------------------------------------------

type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Upload(ctx context.Context, input storage.UploadInput) (*storage.UploadResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.UploadResult), args.Error(1)
}

func (m *MockStorage) Delete(ctx context.Context, storageKey string) error {
	return m.Called(ctx, storageKey).Error(0)
}

func (m *MockStorage) GetURL(ctx context.Context, storageKey string) (string, error) {
	args := m.Called(ctx, storageKey)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) Download(ctx context.Context, storageKey string, mimeType string) (io.ReadCloser, error) {
	args := m.Called(ctx, storageKey, mimeType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
