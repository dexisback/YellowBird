package storage

import (
	"context"
	"fmt"

	"github.com/dexisback/YellowBird/internal/config"
)

// InitStorage initializes the configured Storage provider (S3 or Cloudinary).
func InitStorage(ctx context.Context, cfg *config.Config) (Storage, error) {
	if cfg.StorageProvider == "s3" {
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("s3 storage selected but S3_BUCKET is empty")
		}
		return NewS3Storage(ctx, S3Config{
			Bucket:          cfg.S3Bucket,
			Region:          cfg.S3Region,
			Endpoint:        cfg.S3Endpoint,
			AccessKeyID:     cfg.AWSAccessKeyID,
			SecretAccessKey: cfg.AWSSecretAccessKey,
		})
	}

	return NewCloudinaryStorage(
		cfg.CLOUDINARY_CLOUD_NAME,
		cfg.CLOUDINARY_API_KEY,
		cfg.CLOUDINARY_API_SECRET,
	)
}
