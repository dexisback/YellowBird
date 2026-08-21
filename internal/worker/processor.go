package worker

import (
	"context"

	"github.com/dexisback/YellowBird/internal/domain/job"
)

// Processor executes a single background job type. Each concrete processor
// (thumbnail, preview, transcode) implements this and is registered against
// its job.JobType in the Registry.
type Processor interface {
	Type() job.JobType
	Process(ctx context.Context, j *job.Job) error
}
