package worker

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/queue"
)

// Worker is the long-running consumer that pulls jobs off the queue and
// dispatches them to the appropriate Processor via the Registry.
type Worker struct {
	queue    *queue.RedisQueue
	registry *Registry
	jobs     job.Service
}

func New(q *queue.RedisQueue, registry *Registry, jobs job.Service) *Worker {
	return &Worker{
		queue:    q,
		registry: registry,
		jobs:     jobs,
	}
}

// Run blocks until ctx is cancelled, consuming and executing queued jobs.
func (w *Worker) Run(ctx context.Context) error {
	return nil
}
