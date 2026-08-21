package worker

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
)

type ThumbnailProcessor struct{}

func NewThumbnailProcessor() *ThumbnailProcessor {
	return &ThumbnailProcessor{}
}

func (p *ThumbnailProcessor) Type() job.JobType {
	return job.TypeThumbnail
}

func (p *ThumbnailProcessor) Process(ctx context.Context, j *job.Job) error {
	return nil
}
