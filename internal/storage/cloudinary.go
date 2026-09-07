package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
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
	publicID := uuid.NewString()

	result, err := s.client.Upload.Upload(
		ctx,
		input.Reader,
		uploader.UploadParams{
			PublicID: publicID,
			Folder:   "YellowBird",
		})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("cloudinary upload returned nil result")
	}
	if result.Error.Message != "" {
		return nil, fmt.Errorf("cloudinary upload failed: %s", result.Error.Message)
	}
	if result.PublicID == "" {
		return nil, errors.New("cloudinary upload returned empty public ID")
	}

	originalFileName := input.FileName
	mimeType := input.MimeType
	if mimeType == "" || mimeType == "application/octet-stream" {
		if result.ResourceType != "" && result.Format != "" {
			mimeType = fmt.Sprintf("%s/%s", result.ResourceType, result.Format)
		} else if extMime := mime.TypeByExtension(filepath.Ext(originalFileName)); extMime != "" {
			mimeType = extMime
		}
	}

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
	res, err := s.client.Upload.Destroy(
		ctx,
		uploader.DestroyParams{PublicID: storageKey},
	)
	if err != nil {
		return err
	}
	if res != nil && res.Error.Message != "" {
		return fmt.Errorf("cloudinary destroy failed: %s", res.Error.Message)
	}
	return nil
}

func (s *CloudinaryStorage) getURLForMime(storageKey string, mimeType string) (string, error) {
	if strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/") {
		if vid, err := s.client.Video(storageKey); err == nil && vid != nil {
			return vid.String()
		}
	} else if strings.HasPrefix(mimeType, "image/") {
		if img, err := s.client.Image(storageKey); err == nil && img != nil {
			return img.String()
		}
	}

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

	if f, err := s.client.File(storageKey); err == nil && f != nil {
		if u, err := f.String(); err == nil {
			return u, nil
		}
	}

	return "", fmt.Errorf("failed to build cloudinary URL for %s", storageKey)
}

func (s *CloudinaryStorage) GetURL(ctx context.Context, storageKey string) (string, error) {
	return s.getURLForMime(storageKey, "")
}

func (s *CloudinaryStorage) Download(ctx context.Context, storageKey string, mimeType string) (io.ReadCloser, error) {
	url, err := s.getURLForMime(storageKey, mimeType)
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
