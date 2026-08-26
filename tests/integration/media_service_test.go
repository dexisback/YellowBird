//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/mocks"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateMediaServiceIntegration(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	mr := miniredis.RunT(t)

	projectRepo := project.NewRepository(db)
	mediaRepo := media.NewRepository(db)
	jobRepo := job.NewRepository(db)
	jobService := job.NewService(jobRepo, queue.NewRedisQueue(mr.Addr(), "", 0, "api-test"))

	st := new(mocks.MockStorage)
	st.On("Upload", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{
			StorageKey:       "yellowbird/integration-movie",
			URL:              "https://example.com/integration-movie",
			OriginalFileName: "movie.mp4",
			MimeType:         "video/mp4",
			Size:             10,
		}, nil)

	ownerID := uuid.New()
	p := &project.Project{OwnerID: ownerID, Name: "integration project"}
	require.NoError(t, projectRepo.Create(ctx, p))

	svc := media.NewService(mediaRepo, projectRepo, st, jobService)

	resp, err := svc.CreateMedia(
		ctx, ownerID, p.ID,
		newFileHeader(t, "movie.mp4", "video/mp4", "fake video bytes"),
	)
	require.NoError(t, err)
	assert.Equal(t, media.StatusUploaded, resp.Status)

	t.Run("media persisted to DB", func(t *testing.T) {
		got, err := mediaRepo.GetByID(ctx, resp.ID)
		require.NoError(t, err)
		assert.Equal(t, "video/mp4", got.MimeType)
		assert.Equal(t, "yellowbird/integration-movie", got.StorageKey)
	})

	t.Run("five processing jobs persisted", func(t *testing.T) {
		jobs, err := jobRepo.ListByMedia(ctx, resp.ID)
		require.NoError(t, err)
		require.Len(t, jobs, 5)

		var heights []int
		for _, j := range jobs {
			if j.Type == job.TypeTranscode {
				require.NotNil(t, j.TargetHeight)
				heights = append(heights, *j.TargetHeight)
			}
		}
		assert.ElementsMatch(t, []int{360, 720, 1080}, heights)
	})

	t.Run("five messages enqueued to Redis", func(t *testing.T) {
		rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rc.Close()
		n, err := rc.XLen(ctx, "yellowbird:jobs").Result()
		require.NoError(t, err)
		assert.Equal(t, int64(5), n)
	})
}

func TestCreateMediaServiceIntegrationJobFailureMarksMediaFailed(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	mr := miniredis.RunT(t)

	projectRepo := project.NewRepository(db)
	mediaRepo := media.NewRepository(db)
	jobRepo := job.NewRepository(db)
	jobService := job.NewService(jobRepo, queue.NewRedisQueue(mr.Addr(), "", 0, "api-test"))

	st := new(mocks.MockStorage)
	st.On("Upload", mock.Anything, mock.Anything).
		Return(&storage.UploadResult{StorageKey: "yellowbird/x", MimeType: "image/png"}, nil)

	ownerID := uuid.New()
	p := &project.Project{OwnerID: ownerID, Name: "integration project"}
	require.NoError(t, projectRepo.Create(ctx, p))

	svc := media.NewService(mediaRepo, projectRepo, st, jobService)

	// Kill Redis so job enqueue fails after the media row is created.
	mr.Close()

	_, err := svc.CreateMedia(
		ctx, ownerID, p.ID,
		newFileHeader(t, "movie.png", "image/png", "fake image bytes"),
	)
	require.Error(t, err)

	// The media record must have been marked failed.
	list, err := mediaRepo.ListByProject(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, media.StatusFailed, list[0].Status)
}
