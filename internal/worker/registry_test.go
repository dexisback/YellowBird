package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubProcessor struct {
	typ   job.JobType
	err   error
	calls int
}

func (p *stubProcessor) Type() job.JobType { return p.typ }

func (p *stubProcessor) Process(ctx context.Context, j *job.Job) error {
	p.calls++
	return p.err
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	proc := &stubProcessor{typ: job.TypeThumbnail}

	r.Register(proc)

	got, err := r.Get(job.TypeThumbnail)
	require.NoError(t, err)
	assert.Same(t, proc, got)
}

func TestRegistryGetMissing(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get(job.TypePreview)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no processor regsitered")
}

func TestRegistryRegisterOverwrites(t *testing.T) {
	r := NewRegistry()
	first := &stubProcessor{typ: job.TypeTranscode}
	second := &stubProcessor{typ: job.TypeTranscode}

	r.Register(first)
	r.Register(second)

	got, err := r.Get(job.TypeTranscode)
	require.NoError(t, err)
	assert.Same(t, second, got)
}

func TestProcessorErrorPropagates(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubProcessor{typ: job.TypeThumbnail, err: errors.New("boom")})

	proc, err := r.Get(job.TypeThumbnail)
	require.NoError(t, err)

	err = proc.Process(context.Background(), &job.Job{})
	require.Error(t, err)
	assert.EqualError(t, err, "boom")
}
