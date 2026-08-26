package job

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

func TestValidateCreateJobRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateJobRequest
		wantErr string
	}{
		{
			name:    "transcode without target height",
			req:     CreateJobRequest{Type: TypeTranscode},
			wantErr: "target_height is required for transcode jobs",
		},
		{name: "transcode 360", req: CreateJobRequest{Type: TypeTranscode, TargetHeight: intPtr(360)}},
		{name: "transcode 720", req: CreateJobRequest{Type: TypeTranscode, TargetHeight: intPtr(720)}},
		{name: "transcode 1080", req: CreateJobRequest{Type: TypeTranscode, TargetHeight: intPtr(1080)}},
		{
			name:    "transcode unsupported height",
			req:     CreateJobRequest{Type: TypeTranscode, TargetHeight: intPtr(480)},
			wantErr: "unsupported target height; supported values are 360, 720 and 1080",
		},
		{name: "thumbnail without target height", req: CreateJobRequest{Type: TypeThumbnail}},
		{name: "preview without target height", req: CreateJobRequest{Type: TypePreview}},
		{
			name:    "thumbnail rejects target height",
			req:     CreateJobRequest{Type: TypeThumbnail, TargetHeight: intPtr(720)},
			wantErr: "target_height is only valid for transcode jobs",
		},
		{
			name:    "preview rejects target height",
			req:     CreateJobRequest{Type: TypePreview, TargetHeight: intPtr(720)},
			wantErr: "target_height is only valid for transcode jobs",
		},
		{
			name:    "unsupported job type",
			req:     CreateJobRequest{Type: JobType("audio")},
			wantErr: "unsupported job type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateJobRequest(tt.req)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
