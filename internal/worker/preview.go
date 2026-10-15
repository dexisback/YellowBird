package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/storage"
)

type PreviewProcessor struct {
	mediaRepository  media.Repository
	storage          storage.Storage
	renditionService rendition.Service
	cache            *SourceCache
}

func NewPreviewProcessor(
	mediaRepository media.Repository,
	storage storage.Storage,
	renditionService rendition.Service,
	cache ...*SourceCache,
) *PreviewProcessor {
	var c *SourceCache
	if len(cache) > 0 {
		c = cache[0]
	}
	return &PreviewProcessor{
		mediaRepository:  mediaRepository,
		storage:          storage,
		renditionService: renditionService,
		cache:            c,
	}
}

func (p *PreviewProcessor) Type() job.JobType {
	return job.TypePreview
}

func (p *PreviewProcessor) getSourceFile(ctx context.Context, storageKey, mimeType string) (string, func(), error) {
	if p.cache != nil {
		return p.cache.Get(ctx, storageKey, mimeType)
	}

	source, err := p.storage.Download(ctx, storageKey, mimeType)
	if err != nil {
		return "", nil, fmt.Errorf("failed to download source media: %w", err)
	}
	defer source.Close()

	sourceFile, err := os.CreateTemp("", "yellowbird-source-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create source temp file: %w", err)
	}
	sourcePath := sourceFile.Name()

	if _, err := sourceFile.ReadFrom(source); err != nil {
		_ = sourceFile.Close()
		_ = os.Remove(sourcePath)
		return "", nil, fmt.Errorf("failed to write source media: %w", err)
	}

	if err := sourceFile.Close(); err != nil {
		_ = os.Remove(sourcePath)
		return "", nil, fmt.Errorf("failed to close source media: %w", err)
	}

	return sourcePath, func() { _ = os.Remove(sourcePath) }, nil
}

func (p *PreviewProcessor) Process(ctx context.Context, j *job.Job) error {
	mediaFile, err := p.mediaRepository.GetByID(ctx, j.MediaID)
	if err != nil {
		return fmt.Errorf("failed to get media : %w", err)
	}

	sourcePath, release, err := p.getSourceFile(ctx, mediaFile.StorageKey, mediaFile.MimeType)
	if err != nil {
		return err
	}
	defer release()

	previewFile, err := os.CreateTemp("", "yellowbird-preview-*.mp4")
	if err != nil {
		return fmt.Errorf("failed to create preview temp file: %w", err)
	}
	previewPath := previewFile.Name()
	previewFile.Close()

	defer os.Remove(previewPath)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-threads", "0",
		"-i", sourcePath,
		"-t", "10",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "28",
		"-c:a", "aac",
		"-movflags", "+faststart",
		previewPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg generation failed: %w; output: %s", err, string(output))
	}

	generatedFile, err := os.Open(previewPath)
	if err != nil {
		return fmt.Errorf("failed to open generated preview: %w", err)
	}
	defer generatedFile.Close()

	info, err := generatedFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat generated preview: %w", err)
	}

	uploadResult, err := p.storage.Upload(
		ctx,
		storage.UploadInput{
			Reader:   generatedFile,
			FileName: fmt.Sprintf("%s-preview", j.MediaID.String()),
			MimeType: "video/mp4",
			Size:     info.Size(),
		},
	)
	if err != nil {
		_ = p.storage.Delete(ctx, uploadResult.StorageKey)
		return fmt.Errorf("failed to upload preview: %w", err)
	}

	_, err = p.renditionService.CreateRendition(
		ctx, rendition.CreateRendtionRequest{
			MediaID:    j.MediaID,
			Type:       rendition.TypePreview,
			StorageKey: uploadResult.StorageKey,
			URL:        uploadResult.URL,
			MimeType:   uploadResult.MimeType,
			Size:       uploadResult.Size,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create preview rendition: %w", err)
	}

	return nil
}
