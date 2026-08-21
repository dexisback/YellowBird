//actual worker background worker runtime
//continously blocks on redis for a jobID -> loads that job from the job repository , asks the registry for the processor matching the job type -> executes it

//also owns the lifecycle transitions : queued -> running -> completed/failed
//doesnt know anything about ffmpeg or cloudinary

package worker

import (
	"context"
	"log"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisbakc/YellowBird/internal/queue"
)


type Worker struct {
	queue    *queue.RedisQueue
	jobService    job.Service
	registry    *Registry

}


func NewWorker(
	queue *queue.RedisQueue,
	jobService job.Service, 
	regsitry *Registry, 

) *Worker {
	return &Worker{
		queue: queue, 
		jobService: jobService,
		registry: regsitry,
	}
}




func (w *Worker) Run(ctx context.Context) error {
	log.Println("worker started")
	for {
		jobId, err := w.queue.Dequeue(ctx)
		if err != nil{
			if ctx.Err() != nil{
				log.Println("worker shutting down")
				return ctx.Err()
			}

			log.Printf("worker shutting down")
			return ctx.Err()
		}
		log.Printf("failed to dequeue job : %v", err)
		continue
	}
	if err := w.processJob(ctx, jobId); err != nil{
		log.Printf("failed to process job %s %v",jobId, err);

	}
}




func (w *Worker) Run(ctx context.Context) error{
	log.Println("worker started")
	for{
		jobId, err := w.queue.Dequeue(ctx)
		if err != nil{
			if ctx.Err() != nil{
				log.Println("worker shutting down")
				return ctx.Err()
			}
			log.Printf("failed to dequeu job: %v", err)
			continue
		}

		if err := w.processJOb(ctx, jobId); err != nil{
			log.Printf("failed to process job %s: %v", jobId, err)
		}
	}
}



func (w *Worker) processJob(
	ctx context.Context, 
	jobId uuid.UUID,
) error {
	currentJob, err := w.jobService.GetJob(ctx, jobId)
	if err != nil{
		return err
	}

	processor, err := w.registry.Get(currentJob.Type)
	if err != nil{
		_ = w.jobService.FailJob(ctx, jobId, err.Error())
		return err 
	}

	if err := w.jobService.StartJob(ctx, jobId) ; err != nil{
		return err
	}
	if err := processor.Process(ctx, currentJob); err != nil{
		if updateErr := w.jobService.FailJob(ctx, jobId, err.Error()); updateErr != nil{
			log.Printf("failed to mark job %s as failed : %v", jobId, updateErr)

		}
		return err
	}

	if err := w.jobService.CompleteJob(ctx, jobId); err != nil{
		return err
	}
	return nil
}