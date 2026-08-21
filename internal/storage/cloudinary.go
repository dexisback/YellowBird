package storage

import (
	"context"
	"fmt"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type CloudinaryStorage struct {
	client *cloudinary.Cloudinary
}

func NewCloudinaryStorage(cloudName, apiKey, apiSecret string) (*CloudinaryStorage, error) {
	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise cloudinary: %w", err)
	}
	return &CloudinaryStorage{
		client: cld,
	}, nil
}

func (s *CloudinaryStorage) Upload(
	ctx context.Context,
	input UploadInput,
) (*UploadResult, error) {
	result, err := s.client.Upload.Upload(
		ctx,
		input.File,
		uploader.UploadParams{
			PublicID: input.FileName,
			Folder:   "YellowBird",
		})
	if err != nil {
		return nil, err
	}

	originalFileName := input.FileName
	if input.Header != nil && input.Header.Filename != "" {
		originalFileName = input.Header.Filename
	}

	mimeType := ""
	if input.Header != nil {
		mimeType = input.Header.Header.Get("Content-Type")
	}

	var size int64
	if input.Header != nil {
		size = input.Header.Size
	} else {
		size = int64(result.Bytes)
	}

	return &UploadResult{
		StorageKey:       result.PublicID,
		URL:              result.SecureURL,
		OriginalFileName: originalFileName,
		MimeType:         mimeType,
		Size:             size,
	}, nil
}

func (s *CloudinaryStorage) Delete(
	ctx context.Context,
	storageKey string,
) error {
	_, err := s.client.Upload.Destroy(
		ctx,
		uploader.DestroyParams{PublicID: storageKey},
	)
	return err
}
