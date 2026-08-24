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

type ThumbnailProcessor struct {
	mediaRepository    media.Repository
	storage           storage.Storage
	renditionService    rendition.Service
}

func NewThumbnailProcessor(mediaRepository media.Repository, storage storage.Storage, renditionService rendition.Service) *ThumbnailProcessor {
	return &ThumbnailProcessor{
		mediaRepository: mediaRepository,
		storage: storage, 
		renditionService: renditionService,
	}


}


func (p *ThumbnailProcessor) Type() job.JobType {
	return job.TypeThumbnail
}

func (p *ThumbnailProcessor) Process(ctx context.Context, j *job.Job) error {
	mediaFile, err := p.mediaRepository.GetByID(ctx, j.MediaID)
	if err != nil {
		return fmt.Errorf("failed to get media: %w", err)
	}

	source, err := p.storage.Download(ctx, mediaFile.StorageKey, mediaFile.MimeType)
	if err != nil{
		return fmt.Errorf("failed to download media: %w", err)

	}
	defer source.Close()

	sourceFile, err := os.CreateTemp("", "yellowbird-source-*")
	if err != nil{
		return fmt.Errorf("failed to create source temp file: %w", err)

	}
	defer func() {
		sourceFile.Close()
		os.Remove(sourceFile.Name())
	}()

	if _, err := sourceFile.ReadFrom(source); err != nil {
		return fmt.Errorf("failed to write source media: %w", err)
	}
	sourcePath := sourceFile.Name()
	outputFile, err := os.CreateTemp("", "yellowbird-*.jpg")
	if err != nil{
		return fmt.Errorf("failed to create thumbnail temp file: %w", err)

	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", sourcePath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,)   //thumbnnail generator command
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail generation failed: %w; output: %s", err, string(output))
	}
	thumbnailFile, err := os.Open(outputPath)
	if err != nil{
		return fmt.Errorf("failed to open generated thumbnail : %w", err)

	}
	defer thumbnailFile.Close()
	// We don't have a multipart.FileHeader for the generated file; pass nil.
	uploadResult, err := p.storage.Upload(ctx, storage.UploadInput{
		File:     thumbnailFile,
		Header:   nil,
		FileName: fmt.Sprintf("%s-thumbnail", j.ID.String()),
	})
	if err != nil {
		return fmt.Errorf("failed to upload thumbnail: %w", err)
	}
	_, err = p.renditionService.CreateRendition(
		ctx, rendition.CreateRendtionRequest{
			MediaID:    j.MediaID,
			Type:       rendition.TypeThumbnail,
			StorageKey: uploadResult.StorageKey,
			URL:        uploadResult.URL,
			MimeType:   uploadResult.MimeType,
			Size:       uploadResult.Size,
		},
	)
	if err != nil{
		return fmt.Errorf("failed to create thumbnail rendition %w", err)
	}
	return nil 
}