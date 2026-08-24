package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"

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
		input.Reader,
		uploader.UploadParams{
			PublicID: input.FileName,
			Folder:   "YellowBird",
		})
	if err != nil {
		return nil, err
	}

	originalFileName := input.FileName
	mimeType := input.MimeType

	// Cloudinary result is preferred, but keep caller-provided size as fallback.
	size := int64(result.Bytes)
	if size <= 0 && input.Size > 0 {
		size = input.Size
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

// implemneting GetURL with a mimeType (and later we implement the download() with a mimeType param asw)
func (s *CloudinaryStorage) GetURL(ctx context.Context, storageKey string) (string, error) {
	// Try as an image first, then as a video. The cloudinary builders return
	// an error if they cannot build a URL for the provided public ID.
	if img, err := s.client.Image(storageKey); err == nil && img != nil {
		if u, err := img.String(); err == nil {
			return u, nil
		}
	}

	if vid, err := s.client.Video(storageKey); err == nil && vid != nil {
		if u, err := vid.String(); err == nil {
			return u, nil
		}
	}

	return "", fmt.Errorf("failed to build cloudinary URL for %s", storageKey)
}

func (s *CloudinaryStorage) Download(ctx context.Context, storageKey string, mimeType string) (io.ReadCloser, error) {
	url, err := s.GetURL(ctx, storageKey)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform download request: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, fmt.Errorf("cloudinary download returned status %d", resp.StatusCode)
	}

	return resp.Body, nil

}

//we implement the download() method in this cloudinary provider. cloudinary can generate urls for both images and videos, so we can use the publicID(storageKey)
//to build the appropriate url and stream the asset back to the worker '
//but storage key doesnt alone tell us whether the file is image() or video()
