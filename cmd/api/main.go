//builds go run ./cmd/api

package main

import (
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

	server := server.New(cfg, db)
	server.Run()

	//else:
	log.Println("config loaded succesfullyuh ✅")
	log.Println("up and runnin, on ", cfg.Port)
}
