// actual worker background worker runtime
// continously blocks on redis for a jobID -> loads that job from the job repository , asks the registry for the processor matching the job type -> executes it

// also owns the lifecycle transitions : queued -> running -> completed/failed
// doesnt know anything about ffmpeg or cloudinary

package worker

import (
	"context"
	"log"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/google/uuid"
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
	log.Println("worker started")
	for {
		jobID, err := w.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("worker shutting down")
				return ctx.Err()
			}
			log.Printf("failed to dequeue job: %v", err)
			continue
		}

		if err := w.processJob(ctx, jobID); err != nil {
			log.Printf("failed to process job %s: %v", jobID, err)
		}
	}
}

func (w *Worker) processJob(
	ctx context.Context,
	jobID uuid.UUID,
) error {
	currentJob, err := w.jobService.GetJobEntity(ctx, jobID)
	if err != nil {
		return err
	}

	//note: we dont process the jobs that arent queued
	//this protects against duplicate/stale redis messages:

	if currentJob.Status != job.StatusQueued {
		log.Printf("skipping job %s with status %s", jobID, currentJob.Status,)
		return nil
	}


	processor, err := w.registry.Get(currentJob.Type)
	if err != nil {
		_ = w.jobService.FailJob(ctx, jobID, err.Error())
		return err
	}

	if err := w.jobService.StartJob(ctx, jobID); err != nil {
		return err
	}

	if err := processor.Process(ctx, currentJob); err != nil {
		if updateErr := w.jobService.FailJob(ctx, jobID, err.Error()); updateErr != nil {
			log.Printf("failed to mark job %s as failed: %v", jobID, updateErr)
		}
		return err
	}

	if err := w.jobService.CompleteJob(ctx, jobID); err != nil {
		return err
	}

	return nil
}
