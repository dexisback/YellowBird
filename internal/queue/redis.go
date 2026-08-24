package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const queueKey = "yellowbird:jobs"

type RedisQueue struct {
	client *redis.Client
}

func NewRedisQueue(addr, password string, db int) *RedisQueue {
	return &RedisQueue{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

func (q *RedisQueue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

func (q *RedisQueue) Enqueue(ctx context.Context, jobID uuid.UUID) error {
	return q.client.LPush(ctx, queueKey, jobID.String()).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context) (uuid.UUID, error) {
	res, err := q.client.BRPop(ctx, 0, queueKey).Result()
	if err != nil {
		return uuid.Nil, err
	}
	if len(res) < 2 {
		return uuid.Nil, fmt.Errorf("unexpected redis dequeue response")
	}
	return uuid.Parse(res[1])
}
