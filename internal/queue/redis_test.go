package queue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestQueue(t *testing.T) (*RedisQueue, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewRedisQueue(mr.Addr(), "", 0, "test-consumer"), mr
}

func rawClient(t *testing.T, mr *miniredis.Miniredis) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func pendingCount(t *testing.T, mr *miniredis.Miniredis) int64 {
	t.Helper()
	rc := rawClient(t, mr)
	defer rc.Close()
	p, err := rc.XPending(context.Background(), streamKey, groupName).Result()
	require.NoError(t, err)
	return p.Count
}

func TestShouldDeadLetter(t *testing.T) {
	tests := []struct {
		deliveryCount int64
		want          bool
	}{
		{0, false},
		{1, false},
		{2, false},
		{3, true},
		{10, true},
	}

	for _, tt := range tests {
		q, _ := newTestQueue(t)
		assert.Equal(t, tt.want, q.ShouldDeadLetter(tt.deliveryCount),
			"deliveryCount=%d", tt.deliveryCount)
	}
}

func TestEnsureGroupIdempotent(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))
	require.NoError(t, q.EnsureGroup(ctx), "second call should be a no-op")
}

func TestEnqueueDequeue(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	jobID := uuid.New()
	require.NoError(t, q.Enqueue(ctx, jobID))

	messageID, gotJobID, err := q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, jobID, gotJobID)
	assert.NotEmpty(t, messageID)
}

func TestAckRemovesFromPending(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))
	require.NoError(t, q.Enqueue(ctx, uuid.New()))

	messageID, _, err := q.Dequeue(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pendingCount(t, mr))

	require.NoError(t, q.Ack(ctx, messageID))
	assert.Equal(t, int64(0), pendingCount(t, mr))
}

func TestPendingAndClaim(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	jobID := uuid.New()
	require.NoError(t, q.Enqueue(ctx, jobID))
	messageID, _, err := q.Dequeue(ctx)
	require.NoError(t, err)

	pending, err := q.Pending(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, messageID, pending[0].ID)
	assert.Equal(t, "test-consumer", pending[0].Consumer)

	claimedJobID, err := q.Claim(ctx, messageID)
	require.NoError(t, err)
	assert.Equal(t, jobID, claimedJobID)
}

func TestClaimUnknownMessage(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	_, err := q.Claim(ctx, "9999999999999-0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "was not claimable")
}

func TestMoveToDLQ(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	jobID := uuid.New()
	require.NoError(t, q.Enqueue(ctx, jobID))
	messageID, _, err := q.Dequeue(ctx)
	require.NoError(t, err)

	require.NoError(t, q.MoveToDLQ(ctx, messageID, jobID, "permanent failure", 3))

	rc := rawClient(t, mr)
	defer rc.Close()

	// original message acked
	assert.Equal(t, int64(0), pendingCount(t, mr))

	// DLQ stream has exactly one entry with the expected payload.
	dlqLen, err := rc.XLen(ctx, deadLetterStream).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), dlqLen)

	msgs, err := rc.XRange(ctx, deadLetterStream, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, jobID.String(), msgs[0].Values["job_id"])
	assert.Equal(t, messageID, msgs[0].Values["source_id"])
	assert.Equal(t, "permanent failure", msgs[0].Values["error"])
	assert.Equal(t, "3", msgs[0].Values["retry_count"])
}

func TestDequeueMissingJobID(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	// Manually add a message without a job_id field.
	rc := rawClient(t, mr)
	defer rc.Close()
	_, err := rc.XAdd(ctx, &redis.XAddArgs{Stream: streamKey, Values: map[string]interface{}{"foo": "bar"}}).Result()
	require.NoError(t, err)

	_, _, err = q.Dequeue(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing job_id")
}

func TestDequeueInvalidJobID(t *testing.T) {
	q, mr := newTestQueue(t)
	ctx := context.Background()

	require.NoError(t, q.EnsureGroup(ctx))

	rc := rawClient(t, mr)
	defer rc.Close()
	_, err := rc.XAdd(ctx, &redis.XAddArgs{Stream: streamKey, Values: map[string]interface{}{"job_id": "not-a-uuid"}}).Result()
	require.NoError(t, err)

	_, _, err = q.Dequeue(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid job id")
}
