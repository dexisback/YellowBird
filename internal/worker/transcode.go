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

type TranscodeProcessor struct {
	mediaRepository  media.Repository
	storage          storage.Storage
	renditionService rendition.Service
}

func NewTranscodeProcessor(mediaRepository media.Repository, storage storage.Storage, renditionService rendition.Service) *TranscodeProcessor {
	return &TranscodeProcessor{
		mediaRepository:  mediaRepository,
		storage:          storage,
		renditionService: renditionService,
	}
}

func (p *TranscodeProcessor) Type() job.JobType {
	return job.TypeTranscode
}

func (p *TranscodeProcessor) Process(
	ctx context.Context, j *job.Job,
) error {
	mediaFile, err := p.mediaRepository.GetByID(ctx, j.MediaID)
	if err != nil {
		return fmt.Errorf("failed to get media: %w", err)
	}

	source, err := p.storage.Download(
		ctx, mediaFile.StorageKey, mediaFile.MimeType,
	)
	if err != nil {
		return fmt.Errorf("failed to download source media: %w", err)
	}
	defer source.Close()

	sourceFile, err := os.CreateTemp("", "yellowbird-source-*")
	if err != nil {
		return fmt.Errorf(
			"failed to create source temp file: %w", err,
		)
	}

	sourcePath := sourceFile.Name()

	defer func() {
		sourceFile.Close()
		os.Remove(sourcePath)
	}()

	if _, err := sourceFile.ReadFrom(source); err != nil {
		return fmt.Errorf("failed to write source media: %w", err)
	}

	if err := sourceFile.Close(); err != nil {
		return fmt.Errorf("failed to close source media: %w", err)
	}

	outputFile, err := os.CreateTemp("", "yellowbird-transcode-*.mp4")
	if err != nil {
		return fmt.Errorf("failed to create transcode temp file: %w", err)
	}

	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		os.Remove(outputPath)

		return fmt.Errorf("failed to close transcode file: %w", err)
	}
	defer os.Remove(outputPath)

	targetHeight := 720
	if j.TargetHeight != nil {
		targetHeight = *j.TargetHeight
	}

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-i", sourcePath,

		// Target height while preserving aspect ratio.
		"-vf", fmt.Sprintf("scale=-2:%d", targetHeight),

		// H.264 video.
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "23",

		// AAC audio.
		"-c:a", "aac",
		"-b:a", "128k",

		// Better playback for web delivery.
		"-movflags", "+faststart",

		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg transcode failed: %w. output: %s", err, string(output))
	}

	generatedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("failed to open transcoded file: %w", err)
	}
	defer generatedFile.Close()

	info, err := generatedFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to load stats of transcoded file: %w", err)
	}

	uploadResult, err := p.storage.Upload(ctx, storage.UploadInput{
		Reader:   generatedFile,
		FileName: fmt.Sprintf("%s-%dp", j.MediaID.String(), targetHeight),
		MimeType: "video/mp4",
		Size:     info.Size(),
	})
	if err != nil {
		return fmt.Errorf(
			"failed to upload transcoded file: %w", err,
		)
	}

	_, err = p.renditionService.CreateRendition(
		ctx, rendition.CreateRendtionRequest{
			MediaID:    j.MediaID,
			Type:       rendition.TypeTranscode,
			StorageKey: uploadResult.StorageKey,
			URL:        uploadResult.URL,
			MimeType:   uploadResult.MimeType,
			Size:       uploadResult.Size,
			Height:     &targetHeight,
		},
	)
	if err != nil {
		_ = p.storage.Delete(
			ctx, uploadResult.StorageKey,
		)
		return fmt.Errorf("failed to create transcode rendition: %w", err)
	}

	return nil
}
