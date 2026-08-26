//go:build e2e

// Package e2e hosts the full end-to-end pipeline test. It exercises the
// complete flow with real FFmpeg:
//
//	multipart upload -> storage -> PostgreSQL -> Redis Stream -> Worker
//	    -> FFmpeg -> renditions -> final Media/Job state
//
// Cloudinary is swapped for an in-process local-disk storage implementation so
// the test is hermetic; everything else (PostgreSQL, Redis, FFmpeg) is real.
package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/dexisback/YellowBird/internal/worker"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localStorage is a storage.Storage backed by a temp directory. It stands in
// for Cloudinary so the E2E test does not depend on external credentials.
type localStorage struct {
	dir string
}

func newLocalStorage(t *testing.T) *localStorage {
	t.Helper()
	return &localStorage{dir: t.TempDir()}
}

func (s *localStorage) Upload(ctx context.Context, input storage.UploadInput) (*storage.UploadResult, error) {
	key := uuid.NewString() + "-" + input.FileName
	f, err := os.Create(filepath.Join(s.dir, key))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := io.Copy(f, input.Reader)
	if err != nil {
		return nil, err
	}
	return &storage.UploadResult{
		StorageKey:       key,
		URL:              "file://" + filepath.Join(s.dir, key),
		OriginalFileName: input.FileName,
		MimeType:         input.MimeType,
		Size:             n,
	}, nil
}

func (s *localStorage) Delete(ctx context.Context, storageKey string) error {
	return os.Remove(filepath.Join(s.dir, storageKey))
}

func (s *localStorage) GetURL(ctx context.Context, storageKey string) (string, error) {
	return "file://" + filepath.Join(s.dir, storageKey), nil
}

func (s *localStorage) Download(ctx context.Context, storageKey, mimeType string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.dir, storageKey))
}

func generateSourceVideo(t *testing.T) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "source.mp4")
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=640x360:rate=10",
		"-pix_fmt", "yuv420p",
		out,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate source video: %v\n%s", err, out)
	}
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	return data
}

func fileHeaderFromBytes(t *testing.T, name, mime string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename="%s"`, name)},
		"Content-Type":        {mime},
	})
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	reader := multipart.NewReader(&buf, w.Boundary())
	form, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

func TestEndToEndVideoPipeline(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := testutil.NewPostgres(t)
	mr := miniredis.RunT(t)
	st := newLocalStorage(t)

	// Repositories / services.
	projectRepo := project.NewRepository(db)
	mediaRepo := media.NewRepository(db)
	jobRepo := job.NewRepository(db)
	renditionRepo := rendition.NewRepository(db)

	renditionService := rendition.NewService(renditionRepo)
	jobService := job.NewService(jobRepo, queue.NewRedisQueue(mr.Addr(), "", 0, "e2e-api"))

	// Registry of real FFmpeg processors.
	registry := worker.NewRegistry()
	registry.Register(worker.NewThumbnailProcessor(mediaRepo, st, renditionService))
	registry.Register(worker.NewPreviewProcessor(mediaRepo, st, renditionService))
	registry.Register(worker.NewTranscodeProcessor(mediaRepo, st, renditionService))

	// The worker uses its own queue connection (its own consumer name).
	workerQueue := queue.NewRedisQueue(mr.Addr(), "", 0, "e2e-worker")
	w := worker.NewWorker(workerQueue, jobService, registry)
	go func() { _ = w.Run(ctx) }()

	// Seed a project + upload a real video.
	ownerID := uuid.New()
	p := &project.Project{OwnerID: ownerID, Name: "e2e project"}
	require.NoError(t, projectRepo.Create(ctx, p))

	source := generateSourceVideo(t)
	mediaService := media.NewService(mediaRepo, projectRepo, st, jobService)
	mediaResp, err := mediaService.CreateMedia(
		ctx, ownerID, p.ID,
		fileHeaderFromBytes(t, "source.mp4", "video/mp4", source),
	)
	require.NoError(t, err)
	assert.Equal(t, media.StatusUploaded, mediaResp.Status)

	// Wait for all five jobs to complete.
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		jobs, err := jobRepo.ListByMedia(ctx, mediaResp.ID)
		require.NoError(t, err)
		if len(jobs) == 5 && allCompleted(jobs) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	jobs, err := jobRepo.ListByMedia(ctx, mediaResp.ID)
	require.NoError(t, err)
	require.Len(t, jobs, 5, "expected thumbnail + preview + 3 transcodes")
	for _, j := range jobs {
		assert.Equal(t, job.StatusCompleted, j.Status, "job %s (%s) should be completed", j.ID, j.Type)
	}

	// All five renditions should exist.
	renditions, err := renditionRepo.ListByMedia(ctx, mediaResp.ID)
	require.NoError(t, err)
	assert.Len(t, renditions, 5)

	types := map[rendition.RenditionType]int{}
	for _, r := range renditions {
		types[r.Type]++
	}
	assert.Equal(t, 1, types[rendition.TypeThumbnail])
	assert.Equal(t, 1, types[rendition.TypePreview])
	assert.Equal(t, 3, types[rendition.TypeTranscode])
}

func allCompleted(jobs []job.Job) bool {
	for _, j := range jobs {
		if j.Status != job.StatusCompleted {
			return false
		}
	}
	return true
}
