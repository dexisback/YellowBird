// rewriting queue/redis to implement redis streams now

package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"


	// "github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	streamKey = "yellowbird:jobs"
	groupName = "yellowbird-workers"

	deadLetterStream = "yellowbird:jobs:dlq" // ded☠️

	maxRetries = 3 // todo: make this a knob later on

	pendingTimeout = 5 * time.Minute // a message that has been pending longer than this is abandoned and can be picked up by another free worker then
)

type RedisQueue struct {
	client   *redis.Client
	consumer string
}

func NewRedisQueue(addr string, password string, db int, consumer string) *RedisQueue {
	return &RedisQueue{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		consumer: consumer,
	}
}

func (q *RedisQueue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// new : creates the consumer group if it doesnt alr exists
// every worker process belongs to the same consumer group
// while each worker gets its own consumer name:

func (q *RedisQueue) EnsureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()

	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create redis consumer group: %w", err)
	}
	return nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, jobID uuid.UUID) error {
	_, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"job_id": jobID.String(),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// dequeue claims a new message for this consumer
// the message remains in the stream until Ack() is called.
func (q *RedisQueue) Dequeue(ctx context.Context) (string, uuid.UUID, error) {
	result, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: q.consumer,
		Streams:  []string{streamKey, ">"},
		Count:    1,
		Block:    5 * time.Second,
	}).Result()

	if err != nil {
		return "", uuid.Nil, err
	}

	if len(result) == 0 || len(result[0].Messages) == 0 {
		return "", uuid.Nil, fmt.Errorf("no message returned from redis stream")
	}

	message := result[0].Messages[0]
	jobIDValue, ok := message.Values["job_id"]
	if !ok {
		return message.ID, uuid.Nil, fmt.Errorf("redis message %s missing job_id", message.ID)
	}

	jobIDString, ok := jobIDValue.(string)
	if !ok {
		return message.ID, uuid.Nil, fmt.Errorf(
			"invalid job_id in redis message %s",
			message.ID,
		)
	}

	jobID, err := uuid.Parse(jobIDString)
	if err != nil {
		return message.ID, uuid.Nil, fmt.Errorf(
			"invalid job id %q: %w",
			jobIDString,
			err,
		)
	}

	return message.ID, jobID, nil
}

// NEW: acknowledges successful processing.
//
// Until this happens, Redis considers the message pending.
func (q *RedisQueue) Ack(
	ctx context.Context,
	messageID string,
) error {
	return q.client.XAck(
		ctx,
		streamKey,
		groupName,
		messageID,
	).Err()
}






// NEW: inspect pending messages.
//
// Used to find jobs whose workers crashed before ACKing them.
func (q *RedisQueue) Pending(
	ctx context.Context,
) ([]redis.XPendingExt, error) {
	return q.client.XPendingExt(
		ctx,
		&redis.XPendingExtArgs{
			Stream: streamKey,
			Group:  groupName,
			Start:  "-",
			End:    "+",
			Count:  100,
		},
	).Result()
}

// NEW: reclaim an abandoned message.
//
// Another worker can take ownership of a message that has been
// pending longer than pendingTimeout.
func (q *RedisQueue) Claim(
	ctx context.Context,
	messageID string,
) (uuid.UUID, error) {
	messages, err := q.client.XClaim(
		ctx,
		&redis.XClaimArgs{
			Stream:   streamKey,
			Group:    groupName,
			Consumer: q.consumer,
			MinIdle:  pendingTimeout,
			Messages: []string{messageID},
		},
	).Result()

	if err != nil {
		return uuid.Nil, err
	}

	if len(messages) == 0 {
		return uuid.Nil, fmt.Errorf(
			"message %s was not claimable",
			messageID,
		)
	}

	jobIDValue, ok := messages[0].Values["job_id"]
	if !ok {
		return uuid.Nil, fmt.Errorf(
			"claimed message %s missing job_id",
			messageID,
		)
	}

	jobIDString, ok := jobIDValue.(string)
	if !ok {
		return uuid.Nil, fmt.Errorf(
			"invalid job_id in message %s",
			messageID,
		)
	}

	return uuid.Parse(jobIDString)
}



//decider: whether one should move to dlq or not:
func (q *RedisQueue) ShouldDeadLetter(deliveryCount int64,)bool {
	return deliveryCount >= maxRetries
}




// NEW: move a permanently failing job to the dead-letter stream.
// The original message is acknowledged only after the DLQ entry
// has been successfully written.
func (q *RedisQueue) MoveToDLQ(
	ctx context.Context,
	messageID string,
	jobID uuid.UUID,
	errMsg string,
	deliveryCount int64,  //new addition
) error {
	_, err := q.client.XAdd(
		ctx,
		&redis.XAddArgs{
			Stream: deadLetterStream,
			Values: map[string]interface{}{
				"job_id":      jobID.String(),
				"source_id":   messageID,
				"error":       errMsg,
				"failed_at":   time.Now().UTC().Format(time.RFC3339),
				"retry_count": strconv.FormatInt(deliveryCount, 10),
			},
		},
	).Result()

	if err != nil {
		return fmt.Errorf(
			"failed to move job %s to dead letter queue: %w",
			jobID,
			err,
		)
	}

	return q.Ack(ctx, messageID)
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}





