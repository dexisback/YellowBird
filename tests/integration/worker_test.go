//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/dexisback/YellowBird/internal/worker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerRunEndToEnd drives the real worker loop against a real PostgreSQL
// job repository and a miniredis-backed Redis Stream: create a job, let the
// worker dequeue + process + complete it, then verify both the DB state and
// that the message was acknowledged.
func TestWorkerRunEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := testutil.NewPostgres(t)
	mr := miniredis.RunT(t)

	jobRepo := job.NewRepository(db)
	redisQueue := queue.NewRedisQueue(mr.Addr(), "", 0, "integration-worker")
	jobService := job.NewService(jobRepo, redisQueue)

	registry := worker.NewRegistry()
	registry.Register(&stubProcessor{typ: job.TypeThumbnail})

	// Create a job: this persists it and enqueues it into the Redis stream.
	mediaID := uuid.New()
	resp, err := jobService.CreateJob(ctx, job.CreateJobRequest{
		MediaID: mediaID,
		Type:    job.TypeThumbnail,
	})
	require.NoError(t, err)

	w := worker.NewWorker(redisQueue, jobService, registry)
	go func() { _ = w.Run(ctx) }()

	// Poll until the worker marks the job completed.
	deadline := time.Now().Add(15 * time.Second)
	var status job.JobStatus
	for time.Now().Before(deadline) {
		entity, err := jobService.GetJobEntity(ctx, resp.ID)
		require.NoError(t, err)
		status = entity.Status
		if status == job.StatusCompleted {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.Equal(t, job.StatusCompleted, status, "worker should complete the job")

	// The message must have been acked from the stream.
	assert.Equal(t, int64(0), pendingCount(t, mr))
}
