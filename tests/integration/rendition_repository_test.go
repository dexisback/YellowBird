//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRenditionRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := rendition.NewRepository(db)

	mediaID := uuid.New()
	height := 720
	r := &rendition.Rendition{
		MediaID:    mediaID,
		Type:       rendition.TypeTranscode,
		StorageKey: "yellowbird/rendition-720p.mp4",
		URL:        "https://example.com/rendition-720p.mp4",
		MimeType:   "video/mp4",
		Size:       2048,
		Height:     &height,
	}
	require.NoError(t, repo.Create(ctx, r))
	assert.NotEqual(t, uuid.Nil, r.ID)

	got, err := repo.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, rendition.TypeTranscode, got.Type)
	assert.Equal(t, &height, got.Height)

	t.Run("update", func(t *testing.T) {
		got.Size = 4096
		require.NoError(t, repo.Update(ctx, got))

		updated, err := repo.GetByID(ctx, r.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(4096), updated.Size)
	})

	t.Run("get missing returns not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, r.ID))
		_, err := repo.GetByID(ctx, r.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestRenditionRepositoryListByMedia(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := rendition.NewRepository(db)

	mediaID := uuid.New()
	otherMediaID := uuid.New()

	for _, typ := range []rendition.RenditionType{rendition.TypeThumbnail, rendition.TypePreview} {
		rr := &rendition.Rendition{
			MediaID: mediaID, Type: typ,
			StorageKey: uuid.NewString(), URL: "https://example.com/x", MimeType: "image/jpeg", Size: 1,
		}
		require.NoError(t, repo.Create(ctx, rr))
	}
	other := &rendition.Rendition{
		MediaID: otherMediaID, Type: rendition.TypeThumbnail,
		StorageKey: uuid.NewString(), URL: "https://example.com/y", MimeType: "image/jpeg", Size: 1,
	}
	require.NoError(t, repo.Create(ctx, other))

	list, err := repo.ListByMedia(ctx, mediaID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
