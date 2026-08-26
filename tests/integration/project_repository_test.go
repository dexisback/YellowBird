//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProjectRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := project.NewRepository(db)

	ownerID := uuid.New()
	p := &project.Project{OwnerID: ownerID, Name: "Client Videos", Description: "raw footage"}
	require.NoError(t, repo.Create(ctx, p))
	assert.NotEqual(t, uuid.Nil, p.ID)

	got, err := repo.GetByID(ctx, ownerID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Client Videos", got.Name)

	t.Run("ownership scoping: other owner cannot read", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New(), p.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("update", func(t *testing.T) {
		got.Name = "Renamed"
		require.NoError(t, repo.Update(ctx, ownerID, got))

		updated, err := repo.GetByID(ctx, ownerID, p.ID)
		require.NoError(t, err)
		assert.Equal(t, "Renamed", updated.Name)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, ownerID, p.ID))
		_, err := repo.GetByID(ctx, ownerID, p.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestProjectRepositoryListScopedByOwner(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := project.NewRepository(db)

	ownerA := uuid.New()
	ownerB := uuid.New()

	require.NoError(t, repo.Create(ctx, &project.Project{OwnerID: ownerA, Name: "A1"}))
	require.NoError(t, repo.Create(ctx, &project.Project{OwnerID: ownerA, Name: "A2"}))
	require.NoError(t, repo.Create(ctx, &project.Project{OwnerID: ownerB, Name: "B1"}))

	listA, err := repo.List(ctx, ownerA)
	require.NoError(t, err)
	assert.Len(t, listA, 2)

	listB, err := repo.List(ctx, ownerB)
	require.NoError(t, err)
	assert.Len(t, listB, 1)
	assert.Equal(t, "B1", listB[0].Name)
}
