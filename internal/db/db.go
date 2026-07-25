package db

import (
	"fmt"
	"github.com/dexisback/YellowBird/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)


func Connect(cfg *config.Config) (*gorm.DB, error){
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})

	if err != nil{
		return  nil, fct.Errorf("failed to connect to db &w",err)
	}

	//else:
	return db, nil 
}



func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&project.Project{},
	)
}
