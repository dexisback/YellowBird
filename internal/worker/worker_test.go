package worker

import (
	"context"
	"errors"
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

const (
	testStream  = "yellowbird:jobs"
	testGroup   = "yellowbird-workers"
	testDLQ     = "yellowbird:jobs:dlq"
)

func newTestWorker(t *testing.T, mr *miniredis.Miniredis) (*Worker, *mocks.MockJobService) {
	t.Helper()
	q := queue.NewRedisQueue(mr.Addr(), "", 0, "test-consumer")
	svc := new(mocks.MockJobService)
	return NewWorker(q, svc, NewRegistry()), svc
}

func redisClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func pendingCount(t *testing.T, mr *miniredis.Miniredis) int64 {
	t.Helper()
	rc := redisClient(t, mr)
	defer rc.Close()
	p, err := rc.XPending(context.Background(), testStream, testGroup).Result()
	require.NoError(t, err)
	return p.Count
}

func streamLen(t *testing.T, mr *miniredis.Miniredis, stream string) int64 {
	t.Helper()
	rc := redisClient(t, mr)
	defer rc.Close()
	n, err := rc.XLen(context.Background(), stream).Result()
	require.NoError(t, err)
	return n
}

// seedPending enqueues a job and dequeues it once, leaving it in the pending
// entries list of the consumer group (as if a worker had claimed it).
func seedPending(t *testing.T, mr *miniredis.Miniredis, w *Worker, jobID uuid.UUID) string {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, w.queue.EnsureGroup(ctx))
	require.NoError(t, w.queue.Enqueue(ctx, jobID))
	messageID, gotJobID, err := w.queue.Dequeue(ctx)
	require.NoError(t, err)
	require.Equal(t, jobID, gotJobID)
	return messageID
}

func TestWorkerProcessJobSuccess(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)
	w.registry.Register(&stubProcessor{typ: job.TypeThumbnail})

	jobID := uuid.New()
	svc.On("GetJobEntity", mock.Anything, jobID).
		Return(&job.Job{ID: jobID, Type: job.TypeThumbnail, Status: job.StatusQueued}, nil)
	svc.On("StartJob", mock.Anything, jobID).Return(nil)
	svc.On("CompleteJob", mock.Anything, jobID).Return(nil)

	messageID := seedPending(t, mr, w, jobID)

	require.NoError(t, w.processJob(context.Background(), messageID, jobID))

	svc.AssertCalled(t, "StartJob", mock.Anything, jobID)
	svc.AssertCalled(t, "CompleteJob", mock.Anything, jobID)
	svc.AssertNotCalled(t, "RetryJob", mock.Anything, mock.Anything)
	assert.Equal(t, int64(0), pendingCount(t, mr), "message should be acked on success")
}

func TestWorkerProcessJobSkipsNonQueued(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)

	jobID := uuid.New()
	svc.On("GetJobEntity", mock.Anything, jobID).
		Return(&job.Job{ID: jobID, Type: job.TypeThumbnail, Status: job.StatusCompleted}, nil)

	messageID := seedPending(t, mr, w, jobID)

	require.NoError(t, w.processJob(context.Background(), messageID, jobID))

	svc.AssertNotCalled(t, "StartJob", mock.Anything, mock.Anything)
	svc.AssertNotCalled(t, "CompleteJob", mock.Anything, mock.Anything)
	assert.Equal(t, int64(0), pendingCount(t, mr), "stale message should be acked")
}

func TestWorkerProcessJobUnknownProcessor(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)
	// deliberately leave the registry empty

	jobID := uuid.New()
	svc.On("GetJobEntity", mock.Anything, jobID).
		Return(&job.Job{ID: jobID, Type: job.TypeTranscode, Status: job.StatusQueued}, nil)
	svc.On("FailJob", mock.Anything, jobID, mock.Anything).Return(nil)

	messageID := seedPending(t, mr, w, jobID)

	require.NoError(t, w.processJob(context.Background(), messageID, jobID))

	svc.AssertCalled(t, "FailJob", mock.Anything, jobID, mock.Anything)
	assert.Equal(t, int64(1), streamLen(t, mr, testDLQ), "job should be moved to DLQ")
	assert.Equal(t, int64(0), pendingCount(t, mr), "original message should be acked")
}

func TestWorkerProcessJobProcessorFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)
	w.registry.Register(&stubProcessor{typ: job.TypeThumbnail, err: errors.New("ffmpeg exploded")})

	jobID := uuid.New()
	svc.On("GetJobEntity", mock.Anything, jobID).
		Return(&job.Job{ID: jobID, Type: job.TypeThumbnail, Status: job.StatusQueued}, nil)
	svc.On("StartJob", mock.Anything, jobID).Return(nil)
	svc.On("RetryJob", mock.Anything, jobID).Return(nil)

	messageID := seedPending(t, mr, w, jobID)

	require.NoError(t, w.processJob(context.Background(), messageID, jobID))

	svc.AssertCalled(t, "RetryJob", mock.Anything, jobID)
	svc.AssertNotCalled(t, "CompleteJob", mock.Anything, jobID)
	assert.Equal(t, int64(1), pendingCount(t, mr), "failed job should remain pending for retry")
}

func TestWorkerRecoverPendingRetries(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)
	w.registry.Register(&stubProcessor{typ: job.TypeThumbnail})

	jobID := uuid.New()
	svc.On("GetJobEntity", mock.Anything, jobID).
		Return(&job.Job{ID: jobID, Type: job.TypeThumbnail, Status: job.StatusQueued}, nil)
	svc.On("StartJob", mock.Anything, jobID).Return(nil)
	svc.On("CompleteJob", mock.Anything, jobID).Return(nil)
	svc.On("RetryJob", mock.Anything, jobID).Return(nil)

	base := time.Now().UTC()
	mr.SetTime(base)
	seedPending(t, mr, w, jobID)
	mr.SetTime(base.Add(6 * time.Minute)) // make it look abandoned

	require.NoError(t, w.recoverPending(context.Background()))

	svc.AssertCalled(t, "RetryJob", mock.Anything, jobID)
	svc.AssertCalled(t, "CompleteJob", mock.Anything, jobID)
	assert.Equal(t, int64(0), pendingCount(t, mr), "reprocessed message should be acked")
}

func TestWorkerRecoverPendingDeadLetters(t *testing.T) {
	mr := miniredis.RunT(t)
	w, svc := newTestWorker(t, mr)

	jobID := uuid.New()
	svc.On("FailJob", mock.Anything, jobID, mock.Anything).Return(nil)

	base := time.Now().UTC()
	mr.SetTime(base)
	messageID := seedPending(t, mr, w, jobID)

	// Bump the delivery count to maxRetries (3) by reclaiming the message twice
	// (first delivery = 1, each XCLAIM = +1).
	for i := 1; i <= 2; i++ {
		mr.SetTime(base.Add(6 * time.Minute * time.Duration(i)))
		_, err := w.queue.Claim(context.Background(), messageID)
		require.NoError(t, err)
	}
	mr.SetTime(base.Add(18 * time.Minute))

	require.NoError(t, w.recoverPending(context.Background()))

	svc.AssertCalled(t, "FailJob", mock.Anything, jobID, mock.Anything)
	assert.Equal(t, int64(1), streamLen(t, mr, testDLQ), "exhausted job should be dead-lettered")
	assert.Equal(t, int64(0), pendingCount(t, mr))
}
