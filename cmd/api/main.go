package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	if err := dbpkg.Migrate(db); err != nil {
		log.Fatal(err)
	}

	srv := server.New(cfg, db)
	log.Println("config loaded successfully ✅")
	log.Println("up and running on ", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
