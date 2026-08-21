package worker

import (
	"fmt"

	"github.com/dexisback/YellowBird/internal/domain/job"
)

// Registry maps a job.JobType to the Processor responsible for executing it.
type Registry struct {
	processors map[job.JobType]Processor
}

func NewRegistry() *Registry {
	return &Registry{
		processors: make(map[job.JobType]Processor),
	}
}

func (r *Registry) Register(p Processor) {
	r.processors[p.Type()] = p
}

func (r *Registry) Get(t job.JobType) (Processor, error) {
	p, ok := r.processors[t]
	if !ok {
		return nil, fmt.Errorf("no processor registered for job type %q", t)
	}
	return p, nil
}
