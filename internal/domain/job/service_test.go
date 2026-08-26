package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/mocks"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

func newTestService(t *testing.T, repo job.Repository) (job.Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	q := queue.NewRedisQueue(mr.Addr(), "", 0, "test-consumer")
	return job.NewService(repo, q), mr
}

func redisLen(t *testing.T, mr *miniredis.Miniredis, stream string) int64 {
	t.Helper()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	n, err := rc.XLen(context.Background(), stream).Result()
	require.NoError(t, err)
	return n
}

func TestCreateJob(t *testing.T) {
	t.Run("rejects invalid request without persisting", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		svc, _ := newTestService(t, repo)

		resp, err := svc.CreateJob(context.Background(), job.CreateJobRequest{
			MediaID: uuid.New(),
			Type:    job.TypeTranscode, // missing TargetHeight
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	})

	t.Run("persists job and enqueues it", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		var created *job.Job
		repo.On("Create", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { created = args.Get(1).(*job.Job) }).
			Return(nil)

		svc, mr := newTestService(t, repo)

		mediaID := uuid.New()
		req := job.CreateJobRequest{
			MediaID:      mediaID,
			Type:         job.TypeTranscode,
			TargetHeight: intPtr(720),
		}

		resp, err := svc.CreateJob(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, mediaID, created.MediaID)
		assert.Equal(t, job.TypeTranscode, created.Type)
		assert.Equal(t, job.StatusQueued, created.Status)
		assert.Equal(t, 0, created.Progress)
		assert.Equal(t, intPtr(720), created.TargetHeight)

		assert.Equal(t, created.ID, resp.ID)
		assert.Equal(t, int64(1), redisLen(t, mr, "yellowbird:jobs"))
	})

	t.Run("repository failure propagates", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		repo.On("Create", mock.Anything, mock.Anything).Return(assert.AnError)

		svc, _ := newTestService(t, repo)

		_, err := svc.CreateJob(context.Background(), job.CreateJobRequest{
			MediaID: uuid.New(),
			Type:    job.TypeThumbnail,
		})

		require.Error(t, err)
	})
}

func TestStartJob(t *testing.T) {
	t.Run("queued -> running", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		id := uuid.New()
		stored := &job.Job{ID: id, Status: job.StatusQueued}
		repo.On("GetByID", mock.Anything, id).Return(stored, nil)
		repo.On("Update", mock.Anything, mock.Anything).Return(nil)

		svc, _ := newTestService(t, repo)

		require.NoError(t, svc.StartJob(context.Background(), id))
		assert.Equal(t, job.StatusRunning, stored.Status)
		assert.NotNil(t, stored.StartedAt)
	})

	t.Run("rejects non-queued job", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		id := uuid.New()
		repo.On("GetByID", mock.Anything, id).Return(&job.Job{ID: id, Status: job.StatusRunning}, nil)

		svc, _ := newTestService(t, repo)

		err := svc.StartJob(context.Background(), id)
		require.Error(t, err)
		assert.EqualError(t, err, "job is not queued")
		repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})

	t.Run("not found", func(t *testing.T) {
		repo := new(mocks.MockJobRepository)
		id := uuid.New()
		repo.On("GetByID", mock.Anything, id).Return(nil, assert.AnError)

		svc, _ := newTestService(t, repo)

		require.Error(t, svc.StartJob(context.Background(), id))
	})
}

func TestUpdateProgress(t *testing.T) {
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	stored := &job.Job{ID: id, Status: job.StatusRunning, Progress: 10}
	repo.On("GetByID", mock.Anything, id).Return(stored, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc, _ := newTestService(t, repo)

	require.NoError(t, svc.UpdateProgress(context.Background(), id, 55))
	assert.Equal(t, 55, stored.Progress)
}

func TestCompleteJob(t *testing.T) {
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	stored := &job.Job{ID: id, Status: job.StatusRunning, Progress: 80}
	repo.On("GetByID", mock.Anything, id).Return(stored, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc, _ := newTestService(t, repo)

	require.NoError(t, svc.CompleteJob(context.Background(), id))
	assert.Equal(t, job.StatusCompleted, stored.Status)
	assert.Equal(t, 100, stored.Progress)
	assert.NotNil(t, stored.CompletedAt)
}

func TestFailJob(t *testing.T) {
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	stored := &job.Job{ID: id, Status: job.StatusRunning}
	repo.On("GetByID", mock.Anything, id).Return(stored, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc, _ := newTestService(t, repo)

	require.NoError(t, svc.FailJob(context.Background(), id, "boom"))
	assert.Equal(t, job.StatusFailed, stored.Status)
	assert.Equal(t, "boom", stored.Error)
	assert.NotNil(t, stored.CompletedAt)
}

func TestRetryJob(t *testing.T) {
	now := time.Now()
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	stored := &job.Job{
		ID:          id,
		Status:      job.StatusFailed,
		Error:       "previous failure",
		Progress:    42,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	repo.On("GetByID", mock.Anything, id).Return(stored, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc, _ := newTestService(t, repo)

	require.NoError(t, svc.RetryJob(context.Background(), id))
	assert.Equal(t, job.StatusQueued, stored.Status)
	assert.Equal(t, "", stored.Error)
	assert.Equal(t, 0, stored.Progress)
	assert.Nil(t, stored.StartedAt)
	assert.Nil(t, stored.CompletedAt)
}

func TestGetJobEntity(t *testing.T) {
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	entity := &job.Job{ID: id, Status: job.StatusQueued}
	repo.On("GetByID", mock.Anything, id).Return(entity, nil)

	svc, _ := newTestService(t, repo)

	got, err := svc.GetJobEntity(context.Background(), id)
	require.NoError(t, err)
	assert.Same(t, entity, got)
}

func TestDeleteJob(t *testing.T) {
	repo := new(mocks.MockJobRepository)
	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	svc, _ := newTestService(t, repo)

	require.NoError(t, svc.DeleteJob(context.Background(), id))
	repo.AssertCalled(t, "Delete", mock.Anything, id)
}
