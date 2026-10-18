package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure S3Storage implements the Storage interface at compile time.
var _ Storage = (*S3Storage)(nil)

func TestNewS3Storage_RequiresBucket(t *testing.T) {
	ctx := context.Background()
	_, err := NewS3Storage(ctx, S3Config{Bucket: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "s3 bucket name is required")
}

func TestNewS3Storage_Success(t *testing.T) {
	ctx := context.Background()
	storage, err := NewS3Storage(ctx, S3Config{
		Bucket:     "yellowbird-test-bucket",
		Region:     "us-east-1",
		PresignTTL: 1 * time.Hour,
	})
	require.NoError(t, err)
	require.NotNil(t, storage)
	assert.Equal(t, "yellowbird-test-bucket", storage.bucket)
	assert.Equal(t, "us-east-1", storage.region)
	assert.Equal(t, 1*time.Hour, storage.presignTTL)
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my video file.mp4", "my-video-file-mp4"},
		{"sample_clip-1080p", "sample_clip-1080p"},
		{"weird@#$&*chars", "weird-----chars"},
		{"", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := sanitizeKey(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
