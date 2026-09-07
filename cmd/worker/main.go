package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dexisback/YellowBird/internal/config"
	dbpkg "github.com/dexisback/YellowBird/internal/db"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/media"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisback/YellowBird/internal/storage"
	"github.com/dexisback/YellowBird/internal/worker"
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
	defer func() {
		if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "worker"
	}
	consumer := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	redisQueue := queue.NewRedisQueue(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, consumer)
	defer redisQueue.Close()

	if err := redisQueue.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}

	jobRepository := job.NewRepository(db)
	jobService := job.NewService(jobRepository, redisQueue)
	mediaRepository := media.NewRepository(db)

	renditionRepository := rendition.NewRepository(db)
	renditionService := rendition.NewService(renditionRepository)

	cloudinaryStorage, err := storage.NewCloudinaryStorage(cfg.CLOUDINARY_CLOUD_NAME, cfg.CLOUDINARY_API_KEY, cfg.CLOUDINARY_API_SECRET)
	if err != nil {
		log.Fatal(err)
	}

	registry := worker.NewRegistry()

	registry.Register(
		worker.NewThumbnailProcessor(mediaRepository, cloudinaryStorage, renditionService),
	)

	registry.Register(
		worker.NewPreviewProcessor(mediaRepository, cloudinaryStorage, renditionService),
	)

	registry.Register(
		worker.NewTranscodeProcessor(mediaRepository, cloudinaryStorage, renditionService),
	)

	backgroundWorker := worker.NewWorker(redisQueue, jobService, registry, mediaRepository)
	log.Println("worker booting up rahhhh 🦅🇺🇲🇺🇲🦅🦅🦅🦅...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := backgroundWorker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
