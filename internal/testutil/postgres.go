//go:build integration || e2e

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/domain/user"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewPostgres spins up a throwaway PostgreSQL container (via testcontainers),
// runs the schema migrations for every domain model, and returns a connected
// *gorm.DB. The container is torn down automatically when the test finishes.
//
// It is only compiled/run under the `integration` build tag (requires Docker).
func NewPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("yellowbird_test"),
		tcpostgres.WithUsername("yellowbird"),
		tcpostgres.WithPassword("yellowbird"),
		testcontainers.WithWaitStrategy(wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to bufild connection string: %v", err)
	}

	var db *gorm.DB
	for attempt := 1; ; attempt++ {
		db, err = gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		if attempt >= 5 {
			t.Fatalf("failed to connect to postgres after %d attempts: %v", attempt, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// NOTE: db.Migrate in the app currently only migrates Project/User/Media.
	// Job and Rendition are included here so repository tests can exercise them.
	if err := db.AutoMigrate(
		&project.Project{},
		&user.User{},
		&media.Media{},
		&job.Job{},
		&rendition.Rendition{},
	); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}
