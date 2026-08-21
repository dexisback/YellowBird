package worker

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
)

type TranscodeProcessor struct{}

func NewTranscodeProcessor() *TranscodeProcessor {
	return &TranscodeProcessor{}
}

func (p *TranscodeProcessor) Type() job.JobType {
	return job.TypeTranscode
}

func (p *TranscodeProcessor) Process(ctx context.Context, j *job.Job) error {
	return nil
}
