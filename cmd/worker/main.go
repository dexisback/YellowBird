package main

import (
	"context"
	"log"

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

	// worker gets its own redis queue connection:
	redisQueue := queue.NewRedisQueue(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

	if err := redisQueue.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}

	// repositories/services needed by the processors 👇🏼
	jobRepository := job.NewRepository(db)
	jobService := job.NewService(jobRepository)
	mediaRepository := media.NewRepository(db)

	renditionRepository := rendition.NewRepository(db)
	renditionService := rendition.NewService(renditionRepository)

	// since cloudinary is our current storage provider:
	cloudinaryStorage, err := storage.NewCloudinaryStorage(cfg.CLOUDINARY_CLOUD_NAME, cfg.CLOUDINARY_API_KEY, cfg.CLOUDINARY_API_SECRET)
	if err != nil {
		log.Fatal(err)
	}

	// now we create the processors ke liye registry:
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

	// now we make the actual background worker:
	backgroundWorker := worker.NewWorker(redisQueue, jobService, registry)
	log.Println("worker booting up rahhhh 🦅🇺🇲🇺🇲🦅🦅🦅🦅...")
	if err := backgroundWorker.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
