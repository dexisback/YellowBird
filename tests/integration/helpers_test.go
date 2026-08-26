//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

func newFileHeader(t *testing.T, name, mime, content string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, name)},
	}
	if mime != "" {
		header.Set("Content-Type", mime)
	}
	part, err := w.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	reader := multipart.NewReader(&buf, w.Boundary())
	form, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

func pendingCount(t *testing.T, mr *miniredis.Miniredis) int64 {
	t.Helper()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	p, err := rc.XPending(context.Background(), "yellowbird:jobs", "yellowbird-workers").Result()
	require.NoError(t, err)
	return p.Count
}

// stubProcessor is a no-op worker.Processor used by the worker integration test.
type stubProcessor struct {
	typ job.JobType
}

func (p *stubProcessor) Type() job.JobType { return p.typ }

func (p *stubProcessor) Process(ctx context.Context, j *job.Job) error { return nil }
