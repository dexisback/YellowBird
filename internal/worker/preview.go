package worker

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
)

type PreviewProcessor struct{}

func NewPreviewProcessor() *PreviewProcessor {
	return &PreviewProcessor{}
}

func (p *PreviewProcessor) Type() job.JobType {
	return job.TypePreview
}

func (p *PreviewProcessor) Process(ctx context.Context, j *job.Job) error {
	return nil
}
