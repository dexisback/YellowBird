//builds go run ./cmd/api

package main

import (
	"context"
	"log"

	"github.com/dexisback/YellowBird/internal/config"
	dbpkg "github.com/dexisback/YellowBird/internal/db"
	"github.com/dexisback/YellowBird/internal/server"
)

func main() {
	cfg, err := config.Load()

	if err != nil {
		log.Fatal(err)
	}

	db, err := dbpkg.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// if err := db.AutoMigrate(db) ; err != nil{
	// 	log.Fatal(err)
	// }

	if err := dbpkg.Migrate(db); err != nil {
		log.Fatal(err)
	}

	srv := server.New(cfg, db)
	log.Println("config loaded successfully ✅")
	log.Println("up and running on ", cfg.Port)

	log.Fatal(srv.Run(context.Background()))
}
