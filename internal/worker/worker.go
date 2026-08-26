// actual worker background worker runtime
// continously blocks on redis for a jobID -> loads that job from the job repository , asks the registry for the processor matching the job type -> executes it

// also owns the lifecycle transitions : queued -> running -> completed/failed
// doesnt know anything about ffmpeg or cloudinary

package worker

import (
	"context"
	"log"
	"time"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/google/uuid"
)

// new, job retry logic
const (
	// maxJobRetries    = 3
	recoveryInterval = (30 * time.Second)
)

type Worker struct {
	queue      *queue.RedisQueue
	jobService job.Service
	registry   *Registry
}

func NewWorker(
	queue *queue.RedisQueue,
	jobService job.Service,
	registry *Registry,
) *Worker {
	return &Worker{
		queue:      queue,
		jobService: jobService,
		registry:   registry,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	log.Println("worker started brrr")
	//first we gotta make sure if the redis consumer group even exists before init:
	if err := w.queue.EnsureGroup(ctx); err != nil {
		return err
	}

	//new: periodically recover messages abandoned by previously crashed workers
	recoveryTicker := time.NewTicker(recoveryInterval)
	defer recoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return ctx.Err()
		case <-recoveryTicker.C:
			if err := w.recoverPending(ctx); err != nil {
				log.Printf("failed to recover pending jobs: %v", err)
			}
			continue
		default:
		}

		messageID, jobID, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("worker shutting down")
				return ctx.Err()
			}
			log.Printf("failed to dequeue job: %v", err)
			continue
		}

		if err := w.processJob(ctx, messageID, jobID); err != nil {
			log.Printf("failed to process job %s: %v", jobID, err)
		}
	}
}

func (w *Worker) processJob(
	ctx context.Context,
	messageID string,
	jobID uuid.UUID,
) error {
	currentJob, err := w.jobService.GetJobEntity(ctx, jobID)
	if err != nil {
		return err
	}

	//note: we dont process the jobs that arent queued
	//this protects against duplicate/stale redis messages:

	if currentJob.Status != job.StatusQueued {
		log.Printf("skipping job %s with status %s", jobID, currentJob.Status)
		return w.queue.Ack(ctx, messageID)
	}

	processor, err := w.registry.Get(currentJob.Type)
	if err != nil {
		_ = w.jobService.FailJob(ctx, jobID, err.Error())
		return w.queue.MoveToDLQ(ctx, messageID, jobID, err.Error(), 0)
	}

	if err := w.jobService.StartJob(ctx, jobID); err != nil {
		return err
	}

	if err := processor.Process(ctx, currentJob); err != nil {
		return w.handleJobFailure(ctx, messageID, jobID, err)
	}

	if err := w.jobService.CompleteJob(ctx, jobID); err != nil {
		return err
	}

	//new: only ack after processing + db completion succeeded
	if err := w.queue.Ack(ctx, messageID); err != nil {
		return err
	}

	return nil
}

// processor failures leave the Redis message pending so the recovery loop
// can reclaim and retry it.
func (w *Worker) handleJobFailure(
	ctx context.Context,
	messageID string,
	jobID uuid.UUID,
	processErr error,
) error {
	log.Printf("job %s failed: %v; leaving message %s pending for retry", jobID, processErr, messageID)
	if err := w.jobService.RetryJob(ctx, jobID); err != nil{   //new : handleJobFailure needs to reset back the DB to queued, and we alr had Retryjob() as a function , so implementing/using it here
		return err
	}
	return nil
}



// recover pending messages abandoned by crashed workers and either retry them
// or dead-letter them once their retry count is exhausted.

func (w *Worker) recoverPending(ctx context.Context) error {
	pending, err := w.queue.Pending(ctx)
	if err != nil {
		return err
	}

	for _, message := range pending {
		if message.Idle < 5*time.Minute {
			continue
		}

		if w.queue.ShouldDeadLetter(message.RetryCount) {
			jobID, err := w.queue.Claim(ctx, message.ID)
			if err != nil {
				log.Printf("failed to claim message %s for DLQ: %v", message.ID, err)
				continue
			}

			errMsg := "maximum retry limits reached"
			if err := w.jobService.FailJob(ctx, jobID, errMsg); err != nil {
				log.Printf("failed to mark job %s as failed: %v", jobID, err)
				continue
			}

			if err := w.queue.MoveToDLQ(ctx, message.ID, jobID, errMsg, message.RetryCount); err != nil {
				log.Printf("failed to move job %s to DLQ: %v", jobID, err)
				continue
			}

			continue
		}

		jobID, err := w.queue.Claim(ctx, message.ID)
		if err != nil {
			log.Printf("failed to reclaim message %s: %v", message.ID, err)
			continue
		}

		log.Printf(
			"retrying job %s (attempt %d)",
			jobID,
			message.RetryCount+1,
		)
		if err := w.jobService.RetryJob(ctx, jobID); err != nil {
    log.Printf(
        "failed to reset job %s for retry: %v",
        jobID,
        err,
    )
    continue
}

		if err := w.processJob(ctx, message.ID, jobID); err != nil {
			log.Printf("retry attempt failed for job %s: %v", jobID, err)
		}
	}

	return nil
}
