//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dexisback/YellowBird/internal/domain/user"
	"github.com/dexisback/YellowBird/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewPostgres(t)
	repo := user.NewRepository(db)

	u := &user.User{Name: "Amaan", Email: "amaan@example.com", PasswordHash: "hashed"}
	require.NoError(t, repo.Create(ctx, u))
	assert.NotEqual(t, uuid.Nil, u.ID)

	t.Run("get by id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "amaan@example.com", got.Email)
	})

	t.Run("get by email", func(t *testing.T) {
		got, err := repo.GetByEmail(ctx, "amaan@example.com")
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
	})

	t.Run("get by missing email", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nobody@example.com")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("list", func(t *testing.T) {
		users, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, users, 1)
	})

	t.Run("update", func(t *testing.T) {
		u.Name = "Amaan Updated"
		require.NoError(t, repo.Update(ctx, u))

		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, "Amaan Updated", got.Name)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, u.ID))
		_, err := repo.GetByID(ctx, u.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}
