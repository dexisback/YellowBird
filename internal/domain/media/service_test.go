package media_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/mocks"
	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newFileHeader(t *testing.T, name, mime, content string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, name)},
	}
	if mime != "" {
		header.Set("Content-Type", mime)
	}
	part, err := w.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	reader := multipart.NewReader(&buf, w.Boundary())
	form, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

type mediaServiceFixture struct {
	projectRepo *mocks.MockProjectRepository
	mediaRepo   *mocks.MockMediaRepository
	storage     *mocks.MockStorage
	jobService  *mocks.MockJobService
	service     media.Service
}

func newFixture(t *testing.T) *mediaServiceFixture {
	t.Helper()
	f := &mediaServiceFixture{
		projectRepo: new(mocks.MockProjectRepository),
		mediaRepo:   new(mocks.MockMediaRepository),
		storage:     new(mocks.MockStorage),
		jobService:  new(mocks.MockJobService),
	}
	f.service = media.NewService(f.mediaRepo, f.projectRepo, f.storage, f.jobService)
	return f
}

func createJobRequests(svc *mocks.MockJobService) []job.CreateJobRequest {
	var reqs []job.CreateJobRequest
	for _, c := range svc.Calls {
		if c.Method == "CreateJob" {
			reqs = append(reqs, c.Arguments.Get(1).(job.CreateJobRequest))
		}
	}
	return reqs
}

func (f *mediaServiceFixture) setupHappyPath(t *testing.T, ownerID, projectID uuid.UUID, mime string) {
	t.Helper()
	f.projectRepo.On("GetByID", mock.Anything, ownerID, projectID).
		Return(&project.Project{ID: projectID, OwnerID: ownerID}, nil)
	f.storage.On("Upload", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{
			StorageKey:       "yellowbird/storage-key",
			URL:              "https://example.com/key",
			OriginalFileName: "movie.mp4",
			MimeType:         mime,
			Size:             1234,
		}, nil)
	mediaID := uuid.New()
	f.mediaRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { args.Get(1).(*media.Media).ID = mediaID }).
		Return(nil)
	f.jobService.On("CreateJob", mock.Anything, mock.Anything).
		Return(&job.JobResponse{ID: uuid.New()}, nil)
}

func TestCreateMediaImageFanOut(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.setupHappyPath(t, ownerID, projectID, "image/png")

	resp, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.png", "image/png", "fake image"),
	)

	require.NoError(t, err)
	assert.Equal(t, media.StatusUploaded, resp.Status)
	assert.Equal(t, "yellowbird/storage-key", resp.StorageKey)

	reqs := createJobRequests(f.jobService)
	require.Len(t, reqs, 2)
	assert.Equal(t, job.TypeThumbnail, reqs[0].Type)
	assert.Equal(t, job.TypePreview, reqs[1].Type)
	for _, r := range reqs {
		assert.Equal(t, resp.ID, r.MediaID)
		assert.Nil(t, r.TargetHeight)
	}
}

func TestCreateMediaVideoFanOut(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.setupHappyPath(t, ownerID, projectID, "video/mp4")

	_, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake video"),
	)

	require.NoError(t, err)

	reqs := createJobRequests(f.jobService)
	require.Len(t, reqs, 5)

	assert.Equal(t, job.TypeThumbnail, reqs[0].Type)
	assert.Equal(t, job.TypePreview, reqs[1].Type)

	var heights []int
	for _, r := range reqs[2:] {
		assert.Equal(t, job.TypeTranscode, r.Type)
		require.NotNil(t, r.TargetHeight)
		heights = append(heights, *r.TargetHeight)
	}
	assert.Equal(t, []int{360, 720, 1080}, heights)
}

func TestCreateMediaProjectNotFound(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.projectRepo.On("GetByID", mock.Anything, ownerID, projectID).
		Return(nil, gorm.ErrRecordNotFound)

	_, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake"),
	)

	require.Error(t, err)
	assert.EqualError(t, err, "project not found")
	f.storage.AssertNotCalled(t, "Upload", mock.Anything, mock.Anything)
	f.mediaRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateMediaUploadFailure(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.projectRepo.On("GetByID", mock.Anything, ownerID, projectID).
		Return(&project.Project{ID: projectID}, nil)
	f.storage.On("Upload", mock.Anything, mock.Anything).
		Return(nil, errors.New("cloudinary down"))

	_, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake"),
	)

	require.Error(t, err)
	f.mediaRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateMediaRepoFailureCleansUpStorage(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.projectRepo.On("GetByID", mock.Anything, ownerID, projectID).
		Return(&project.Project{ID: projectID}, nil)
	f.storage.On("Upload", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{StorageKey: "yellowbird/orphan"}, nil)
	f.mediaRepo.On("Create", mock.Anything, mock.Anything).
		Return(errors.New("db down"))
	f.storage.On("Delete", mock.Anything, "yellowbird/orphan").Return(nil)

	_, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake"),
	)

	require.Error(t, err)
	f.storage.AssertCalled(t, "Delete", mock.Anything, "yellowbird/orphan")
	f.jobService.AssertNotCalled(t, "CreateJob", mock.Anything, mock.Anything)
}

func TestCreateMediaJobCreationFailureMarksMediaFailed(t *testing.T) {
	f := newFixture(t)
	ownerID, projectID := uuid.New(), uuid.New()
	f.projectRepo.On("GetByID", mock.Anything, ownerID, projectID).
		Return(&project.Project{ID: projectID}, nil)
	f.storage.On("Upload", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{StorageKey: "yellowbird/key", MimeType: "video/mp4"}, nil)
	mediaID := uuid.New()
	f.mediaRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { args.Get(1).(*media.Media).ID = mediaID }).
		Return(nil)
	f.mediaRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	f.jobService.On("CreateJob", mock.Anything, mock.Anything).
		Return(nil, errors.New("redis down"))

	_, err := f.service.CreateMedia(
		context.Background(),
		ownerID,
		projectID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake"),
	)

	require.Error(t, err)
	f.mediaRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
	// Verify the media was marked failed.
	for _, c := range f.mediaRepo.Calls {
		if c.Method == "Update" {
			m := c.Arguments.Get(1).(*media.Media)
			assert.Equal(t, media.StatusFailed, m.Status)
		}
	}
}
