//dispatch layer for the worker system
//redis gives us a jobID -> worker loads the corresponding job -> registry decides which processor should handle it based on job.Type 
//this means worker.go never needs to know about thumbnails/previews or transcoding directly 
//when we add another processing type laeter, seedha add it in registry 
//registry = read heavy. so rw mutex


package worker
import (
	"fmt"
	"sync"
	"github.com/dexisback/YellowBird/internal/domain/job"

)


type Registry struct {
	mu   sync.RWMutex
	processors map[job.JobType]Processor
}

func NewRegistry() *Registry{
	return &Registry{
		processors: make(map[job.JobType]Processor),
	}
}

func(r *Registry) Register(processor Processor){
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processors[processor.Type()] = processor

}

func(r *Registry) Get(jobType job.JobType) (Processor, error){
	r.mu.RLock()  //read lock laga do
	defer r.mu.RUnlock()  //remove when done

	processor, exists := r.processors[jobType]
	if !exists {
		return nil, fmt.Errorf("no processor regsitered for the received job type %q", jobType )

	}
	return processor, nil

}