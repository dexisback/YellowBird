package db

import (
	"fmt"

	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/user"

	"github.com/dexisback/YellowBird/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to db &w", err)
	}

	//else:
	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&project.Project{},
		&user.User{},
		&media.Media{},
	)
}
