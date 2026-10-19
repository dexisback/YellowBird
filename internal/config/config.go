//using koanf

// both executables need ts -- cmd/api and cmd/worker
package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	JWT_SECRET            string
	CLOUDINARY_CLOUD_NAME string
	CLOUDINARY_API_KEY    string
	CLOUDINARY_API_SECRET string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	WorkerConcurrency     int

	// Storage configuration
	StorageProvider    string // "s3" or "cloudinary"
	S3Bucket           string
	S3Region           string
	S3Endpoint         string // Optional: for LocalStack / MinIO
	AWSAccessKeyID     string // Optional: falls back to IAM task role in ECS
	AWSSecretAccessKey string // Optional
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	k := koanf.New(".")

	if err := k.Load(env.Provider("", ".", nil), nil); err != nil {
		return nil, fmt.Errorf("failed to load env variables %w", err)
	}

	storageProvider := k.String("STORAGE_PROVIDER")
	s3Bucket := k.String("S3_BUCKET")
	if storageProvider == "" {
		if s3Bucket != "" {
			storageProvider = "s3"
		} else {
			storageProvider = "cloudinary"
		}
	}

	s3Region := k.String("S3_REGION")
	if s3Region == "" {
		s3Region = "us-east-1"
	}

	databaseURL := k.String("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = k.String("NEON_DB")
	}

	cfg := &Config{
		Port:        k.String("PORT"),
		DatabaseURL: databaseURL,
		JWT_SECRET:  k.String("JWT_SECRET"),

		CLOUDINARY_CLOUD_NAME: k.String("CLOUDINARY_CLOUD_NAME"),
		CLOUDINARY_API_KEY:    k.String("CLOUDINARY_API_KEY"),
		CLOUDINARY_API_SECRET: k.String("CLOUDINARY_API_SECRET"),

		StorageProvider:    storageProvider,
		S3Bucket:           s3Bucket,
		S3Region:           s3Region,
		S3Endpoint:         k.String("S3_ENDPOINT"),
		AWSAccessKeyID:     k.String("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey: k.String("AWS_SECRET_ACCESS_KEY"),

		RedisAddr:     k.String("REDIS_ADDR"),
		RedisPassword: k.String("REDIS_PASSWORD"),
		RedisDB:       k.Int("REDIS_DB"),

		WorkerConcurrency: k.Int("WORKER_CONCURRENCY"),
	}

	if cfg.WorkerConcurrency <= 0 {
		cfg.WorkerConcurrency = 4
	}

	return cfg, nil
}
