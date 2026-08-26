//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMediaRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := media.NewRepository(db)

	projectID := uuid.New()
	m := &media.Media{
		ProjectId:        projectID,
		OriginalFileName: "movie.mp4",
		StorageKey:       "yellowbird/movie.mp4",
		MimeType:         "video/mp4",
		Size:             1024,
		Status:           media.StatusUploaded,
	}
	require.NoError(t, repo.Create(ctx, m))
	assert.NotEqual(t, uuid.Nil, m.ID)

	got, err := repo.GetByID(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, m.ID, got.ID)
	assert.Equal(t, "yellowbird/movie.mp4", got.StorageKey)
	assert.Equal(t, media.StatusUploaded, got.Status)

	t.Run("update", func(t *testing.T) {
		got.Status = media.StatusProcessing
		require.NoError(t, repo.Update(ctx, got))

		updated, err := repo.GetByID(ctx, m.ID)
		require.NoError(t, err)
		assert.Equal(t, media.StatusProcessing, updated.Status)
	})

	t.Run("get missing returns not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, m.ID))
		_, err := repo.GetByID(ctx, m.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestMediaRepositoryListByProject(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := media.NewRepository(db)

	projectID := uuid.New()
	otherProjectID := uuid.New()

	for i := 0; i < 2; i++ {
		m := &media.Media{
			ProjectId: projectID, OriginalFileName: "a.mp4",
			StorageKey: uuid.NewString(), MimeType: "video/mp4", Status: media.StatusUploaded,
		}
		require.NoError(t, repo.Create(ctx, m))
	}
	other := &media.Media{
		ProjectId: otherProjectID, OriginalFileName: "b.mp4",
		StorageKey: uuid.NewString(), MimeType: "video/mp4", Status: media.StatusUploaded,
	}
	require.NoError(t, repo.Create(ctx, other))

	list, err := repo.ListByProject(ctx, projectID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestMediaRepositoryListByProjectEmpty(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := media.NewRepository(db)

	list, err := repo.ListByProject(ctx, uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, list)
	assert.Empty(t, list)
}
