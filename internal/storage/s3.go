package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const (
	defaultS3Region   = "us-east-1"
	defaultPresignTTL = 24 * time.Hour
)

// S3Config holds the connection and authentication parameters for Amazon S3.
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string        // Optional: for MinIO, LocalStack, or custom S3-compatible endpoints
	AccessKeyID     string        // Optional: if omitted, AWS SDK falls back to IAM task role / EC2 instance profile / env vars
	SecretAccessKey string        // Optional
	PresignTTL      time.Duration // Duration for generated viewing URLs (default 24h)
}

// S3Storage implements the Storage interface backed by Amazon Simple Storage Service (S3).
type S3Storage struct {
	client        *s3.Client
	uploader      *manager.Uploader
	presignClient *s3.PresignClient
	bucket        string
	region        string
	presignTTL    time.Duration
}

// NewS3Storage creates a new S3-backed Storage instance using AWS SDK v2.
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3 bucket name is required")
	}

	region := cfg.Region
	if region == "" {
		region = defaultS3Region
	}

	var optFns []func(*awsconfig.LoadOptions) error
	optFns = append(optFns, awsconfig.WithRegion(region))

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	var s3OptFns []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3OptFns...)
	uploader := manager.NewUploader(client)
	presignClient := s3.NewPresignClient(client)

	ttl := cfg.PresignTTL
	if ttl <= 0 {
		ttl = defaultPresignTTL
	}

	return &S3Storage{
		client:        client,
		uploader:      uploader,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		region:        region,
		presignTTL:    ttl,
	}, nil
}

// Upload uploads an input stream to Amazon S3 using multipart streaming.
func (s *S3Storage) Upload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	if input.Reader == nil {
		return nil, errors.New("s3 upload input reader is nil")
	}

	ext := filepath.Ext(input.FileName)
	baseName := strings.TrimSuffix(filepath.Base(input.FileName), ext)
	if ext == "" && input.MimeType != "" {
		if exts, err := mime.ExtensionsByType(input.MimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}

	// Canonical S3 key pattern: yellowbird/<uuid>-<clean-name><ext>
	key := fmt.Sprintf("yellowbird/%s-%s%s", uuid.NewString(), sanitizeKey(baseName), ext)

	mimeType := input.MimeType
	if mimeType == "" || mimeType == "application/octet-stream" {
		if detected := mime.TypeByExtension(ext); detected != "" {
			mimeType = detected
		} else {
			mimeType = "application/octet-stream"
		}
	}

	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        input.Reader,
		ContentType: aws.String(mimeType),
	}

	result, err := s.uploader.Upload(ctx, putInput)
	if err != nil {
		return nil, fmt.Errorf("s3 upload failed for key %s: %w", key, err)
	}

	// Fallback to presigned URL if uploader did not return an explicit location URL
	url := result.Location
	if url == "" {
		url, _ = s.GetURL(ctx, key)
	}

	return &UploadResult{
		StorageKey:       key,
		URL:              url,
		OriginalFileName: input.FileName,
		MimeType:         mimeType,
		Size:             input.Size,
	}, nil
}

// Delete removes an object from the S3 bucket.
func (s *S3Storage) Delete(ctx context.Context, storageKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return fmt.Errorf("s3 delete failed for key %s: %w", storageKey, err)
	}
	return nil
}

// GetURL generates a secure, time-limited presigned GET URL for the object.
func (s *S3Storage) GetURL(ctx context.Context, storageKey string) (string, error) {
	presignedReq, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	}, s3.WithPresignExpires(s.presignTTL))
	if err != nil {
		return "", fmt.Errorf("failed to generate s3 presigned URL for %s: %w", storageKey, err)
	}
	return presignedReq.URL, nil
}

// Download returns a readable stream of the object bytes directly from S3.
func (s *S3Storage) Download(ctx context.Context, storageKey string, _ string) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storageKey),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 download failed for key %s: %w", storageKey, err)
	}
	return resp.Body, nil
}

func sanitizeKey(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	if cleaned == "" {
		return "file"
	}
	return cleaned
}
