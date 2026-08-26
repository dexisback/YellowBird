# YellowBird

Video upload and processing platform. Upload a video, and YellowBird generates
thumbnails, previews, and multiple transcode renditions in the background.

The project is structured as two independent Go processes that share PostgreSQL
and Redis:

- **API** (`cmd/api`) — HTTP server for auth, projects, media, jobs, and renditions.
- **Worker** (`cmd/worker`) — background consumer that pulls jobs off Redis Streams
  and runs FFmpeg to produce renditions.

---

## Table of contents

- [Purpose](#purpose)
- [Concepts explored](#concepts-explored)
- [What it does](#what-it-does)
- [Architecture](#architecture)
- [The processing pipeline](#the-processing-pipeline)
- [Retries and dead-lettering](#retries-and-dead-lettering)
- [Repository layout](#repository-layout)
- [Tech stack](#tech-stack)
- [Getting started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration)
  - [Run locally](#run-locally)
  - [Run with Docker](#run-with-docker)
- [API reference](#api-reference)
- [Testing](#testing)
- [Makefile reference](#makefile-reference)
- [Docker reference](#docker-reference)
- [Future scope](#future-scope)
- [Known limitations](#known-limitations)

---

## Purpose

YellowBird is not meant to be a production product. It is a playground for
building — and understanding — a production-style media backend. A user uploads
a media file; the system stores the original, creates processing jobs, and
distributes them to background workers that run FFmpeg-based transcoding,
thumbnail generation, preview generation, and (eventually) other media
transformations.

The point is the machinery underneath the upload API. Each feature is a vehicle
for learning real backend and systems-engineering concepts, implemented in the
open rather than hidden behind a managed service:

- **Redis Streams** and consumer groups for the job queue.
- **Reliable job processing** — at-least-once delivery, acknowledgement, and
  pending-entry inspection.
- **Retries and dead-letter queues** — reclaiming abandoned work and quarantining
  permanently failing jobs.
- **Concurrent workers** — multiple consumers competing over a single stream.
- **Distributed processing** — API and worker as separate processes that
  coordinate only through PostgreSQL and Redis.
- **Storage abstractions** — a provider-agnostic `Storage` interface with a
  Cloudinary implementation, so other backends can be swapped in.
- **Fault recovery** — worker crashes, message reclamation, and job state repair.
- **Multi-node execution** — the foundation for scaling workers horizontally.

The MVP is a concrete, working slice of that; it is deliberately not the end
state.

## What it does

1. A client registers and logs in, then creates a **project**.
2. The client uploads a media file (image or video) to the project. The file is
   stored in **Cloudinary** and its metadata is written to **PostgreSQL**.
3. Uploading media fans out into **processing jobs**:
   - every upload → a `thumbnail` job and a `preview` job
   - video uploads additionally → three `transcode` jobs (360p, 720p, 1080p)
4. Each job is pushed to a **Redis Stream**.
5. The **worker** consumes the stream, downloads the source from Cloudinary,
   runs **FFmpeg**, uploads the resulting **rendition** back to Cloudinary, and
   records the rendition plus the final job state in PostgreSQL.

The API and worker communicate only through the shared database and the Redis
stream — there is no direct RPC between them.

## Architecture

```
                    ┌──────────────────────────────────────────────┐
                    │                   API (Gin)                 │
                    │  auth · projects · media · jobs · renditions│
                    └──────────────┬───────────────┬──────────────┘
                                   │               │
                          upload   │               │  job fan-out
                                   ▼               ▼
                            ┌───────────┐   ┌─────────────┐   ┌────────────┐
                            │ Cloudinary│   │  PostgreSQL │   │ Redis      │
                            │ (storage) │   │ (metadata,  │   │ Streams    │
                            │           │   │  state)     │   │ (queue)    │
                            └─────▲─────┘   └──────▲──────┘   └─────┬──────┘
                                  │                │                │
                        download/ │ upload         │ read/write     │ consume
                                  │                │                │
                            ┌─────┴────────────────┴────────────────┴──────┐
                            │                 Worker                       │
                            │  consumer group → registry → processors      │
                            │  thumbnail · preview · transcode (FFmpeg)    │
                            └──────────────────────────────────────────────┘
```

### Processes

| Process   | Entrypoint          | Role                                                                               |
|-----------|---------------------|------------------------------------------------------------------------------------|
| API       | `cmd/api/main.go`   | Serves HTTP, owns a Redis client with consumer name `api` (used only to enqueue).  |
| Worker    | `cmd/worker/main.go`| Owns its own Redis client with consumer name `<hostname>-<pid>`. Runs processors.  |

Each process builds its own `*queue.RedisQueue`. The API's queue is created once
in `server.New` and injected into the job service; the worker's is created in
`cmd/worker/main.go`. They are deliberately separate so each process has its own
client connection and consumer identity.

### Dependency flow (API)

```
server.New(cfg, db)
 └─ queue.RedisQueue("api") ──► s.redis
      └─ registerRoutes()
           job.NewService(jobRepo, s.redis)        ──► jobService
           media.NewService(mediaRepo, projectRepo, cloudinaryStorage, jobService)
```

`job.Service` is the only domain that holds the Redis queue (it enqueues job IDs).
`media.Service` depends on `job.Service` for job creation and does not touch Redis
directly. `project`, `user`, `rendition`, `auth`, and `storage` have no Redis
dependency.

## The processing pipeline

```
multipart upload
  → API: media handler
  → storage.Upload (Cloudinary)
  → media repository (PostgreSQL row, status "uploaded")
  → fan-out: job.Service.CreateJob for thumbnail + preview (+ 3 transcodes for video)
       → job repository (PostgreSQL row, status "queued")
       → queue.Enqueue (Redis XADD to "yellowbird:jobs")
  → worker: XREADGROUP
  → registry.Get(job.Type) → processor
  → storage.Download (source) → FFmpeg → storage.Upload (rendition)
  → rendition repository (PostgreSQL row)
  → job.Service.CompleteJob → queue.Ack
```

### Job types

| Type        | Input                | Output                            |
|-------------|----------------------|-----------------------------------|
| `thumbnail` | any media            | single-frame JPEG                 |
| `preview`   | any media            | 10-second H.264/AAC MP4           |
| `transcode` | video (360/720/1080) | height-scaled H.264/AAC MP4       |

## Retries and dead-lettering

The queue uses Redis Streams consumer groups:

1. `XREADGROUP` delivers a job to a consumer; until `XACK`, it is **pending**.
2. If processing succeeds, the worker calls `Ack`.
3. If processing fails, the message stays pending. A recovery loop (every 30s)
   inspects pending entries, `XCLAIM`s any that have been idle longer than 5
   minutes, and retries them.
4. After `maxRetries` (3) deliveries, the message is moved to the dead-letter
   stream (`yellowbird:jobs:dlq`) and the job is marked `failed`.

| Key                    | Value                          |
|------------------------|--------------------------------|
| Stream                 | `yellowbird:jobs`              |
| Consumer group         | `yellowbird-workers`           |
| Dead-letter stream     | `yellowbird:jobs:dlq`          |
| Max retries            | 3                              |
| Pending timeout        | 5 minutes                      |
| Recovery interval      | 30 seconds                     |

## Repository layout

```
cmd/
  api/          API entrypoint
  worker/       worker entrypoint
internal/
  auth/         JWT generation/validation, password, RBAC stubs
  config/       env-based config loading (koanf + godotenv)
  db/           Postgres connection + migrations
  domain/
    user/       registration, login, user CRUD
    project/    projects (owner-scoped)
    media/      media upload + job fan-out
    job/        jobs + state machine + validation
    rendition/  processed outputs (thumbnail/preview/transcode)
  queue/        Redis Streams client (consumer groups, retries, DLQ)
  storage/      storage provider contract + Cloudinary implementation
  worker/       worker runtime, processor registry, FFmpeg processors
  server/       Gin engine, routing, middleware
  mocks/        testify mocks for domain interfaces (tests)
  testutil/     test helpers (testcontainers Postgres, etc.)
migrations/     SQL migrations (reserved)
deployments/    Dockerfiles + docker-compose
tests/
  integration/  integration tests (Postgres via testcontainers)
  e2e/          end-to-end pipeline test (real FFmpeg)
scripts/
  chaoscow/     chaos/ops scripts
```

Domains follow a consistent layering: `model.go` (GORM entities), `dto.go`
(request/response contracts), `repository.go` (DB access behind an interface),
`service.go` (business logic), `handler.go` (HTTP), `routes.go` (route wiring).

## Tech stack

- **Language**: Go 1.26
- **HTTP**: Gin
- **Database**: PostgreSQL (GORM)
- **Queue**: Redis Streams (go-redis)
- **Storage**: Cloudinary
- **Video**: FFmpeg
- **Auth**: JWT (HS256, 24h expiry)
- **Config**: koanf + godotenv
- **Testing**: stdlib `testing`, testify, miniredis, testcontainers-go

## Getting started

### Prerequisites

- Go 1.26+
- PostgreSQL 16+ (or a Neon connection string)
- Redis 7+
- FFmpeg (worker only)
- A Cloudinary account (cloud name, API key, API secret)
- Docker + Docker Compose (optional, for containerized runs and integration tests)

### Configuration

Configuration is read from environment variables. Copy the example and fill it in:

```sh
cp .env.example .env
```

| Variable                | Purpose                                   |
|-------------------------|-------------------------------------------|
| `PORT`                  | HTTP port for the API                     |
| `DATABASE_URL`          | PostgreSQL connection string              |
| `JWT_SECRET`            | secret used to sign JWTs                  |
| `REDIS_ADDR`            | Redis address (`host:port`)               |
| `REDIS_PASSWORD`        | Redis password (empty for local)          |
| `REDIS_DB`              | Redis database index                      |
| `CLOUDINARY_CLOUD_NAME` | Cloudinary cloud name                     |
| `CLOUDINARY_API_KEY`    | Cloudinary API key                        |
| `CLOUDINARY_API_SECRET` | Cloudinary API secret                     |

### Run locally

Start the backing services (or point `DATABASE_URL`/`REDIS_ADDR` at existing ones):

```sh
docker compose -f deployments/docker-compose.yml up -d postgres redis
```

Then run each process:

```sh
go run ./cmd/api
go run ./cmd/worker
```

The API migrates the schema on startup; the worker assumes the schema already
exists, so start the API first.

### Run with Docker

```sh
docker compose -f deployments/docker-compose.yml up --build
```

This builds and runs `postgres`, `redis`, `api`, and `worker`. The compose file
interpolates `JWT_SECRET` and `CLOUDINARY_*` from the environment (or a `.env`
file), so export them first:

```sh
export JWT_SECRET=...
export CLOUDINARY_CLOUD_NAME=...
export CLOUDINARY_API_KEY=...
export CLOUDINARY_API_SECRET=...
```

## API reference

All responses are JSON. Protected endpoints require
`Authorization: Bearer <token>`.

### Health

| Method | Path       | Auth | Description        |
|--------|------------|------|--------------------|
| GET    | `/health`  | No   | Liveness check     |

### Users

| Method | Path                       | Auth | Description                              |
|--------|----------------------------|------|------------------------------------------|
| POST   | `/api/v1/users/register`   | No   | Register a user                          |
| POST   | `/api/v1/users/login`      | No   | Authenticate a user                      |
| GET    | `/api/v1/users`            | No   | List users                               |
| GET    | `/api/v1/users/:id`        | No   | Get a user                               |
| DELETE | `/api/v1/users/:id`        | No   | Delete a user                            |

> User endpoints are currently not behind the auth middleware.

### Projects

| Method | Path                       | Auth | Description                    |
|--------|----------------------------|------|--------------------------------|
| POST   | `/api/v1/projects`         | Yes  | Create a project               |
| GET    | `/api/v1/projects`         | Yes  | List the caller's projects     |
| GET    | `/api/v1/projects/:id`     | Yes  | Get a project                  |
| PUT    | `/api/v1/projects/:id`     | Yes  | Update a project               |
| DELETE | `/api/v1/projects/:id`     | Yes  | Delete a project               |

### Media

| Method | Path                    | Auth | Description                                          |
|--------|-------------------------|------|------------------------------------------------------|
| POST   | `/api/v1/media`         | Yes  | Upload media (`multipart/form-data`)                 |
| GET    | `/api/v1/media`         | Yes  | List media (`?project_id=`)                          |
| GET    | `/api/v1/media/:id`     | Yes  | Get media                                            |
| PUT    | `/api/v1/media/:id`     | Yes  | Update media status                                  |
| DELETE | `/api/v1/media/:id`     | Yes  | Delete media                                         |

Upload request (`Content-Type: multipart/form-data`):

- `project_id` — form field (UUID)
- `file` — the media file

### Jobs

| Method | Path                    | Auth | Description                            |
|--------|-------------------------|------|----------------------------------------|
| POST   | `/api/v1/jobs`          | Yes  | Create a job                           |
| GET    | `/api/v1/jobs`          | Yes  | List jobs (`?media_id=`)               |
| GET    | `/api/v1/jobs/:id`      | Yes  | Get a job                              |
| DELETE | `/api/v1/jobs/:id`      | Yes  | Delete a job                           |

Create-job body:

```json
{
  "media_id": "uuid",
  "type": "thumbnail | preview | transcode",
  "target_height": 720
}
```

`target_height` is required for `transcode` (360, 720, or 1080) and must be
omitted for `thumbnail`/`preview`.

### Renditions

| Method | Path                        | Auth | Description                              |
|--------|-----------------------------|------|------------------------------------------|
| POST   | `/api/v1/renditions`        | Yes  | Create a rendition                       |
| GET    | `/api/v1/renditions`        | Yes  | List renditions (`?media_id=`)           |
| GET    | `/api/v1/renditions/:id`    | Yes  | Get a rendition                          |
| DELETE | `/api/v1/renditions/:id`    | Yes  | Delete a rendition                       |

## Testing

Tests are split into three tiers:

| Tier        | Command                              | Requires                        |
|-------------|--------------------------------------|---------------------------------|
| Unit        | `go test ./...`                      | nothing (hermetic)              |
| Integration | `go test -tags integration ./...`    | Docker (testcontainers Postgres)|
| E2E         | `go test -tags e2e ./tests/e2e/...`  | Docker + FFmpeg                 |

Unit tests use miniredis (in-process Redis) and testify mocks; integration tests
spin up a throwaway PostgreSQL container; the E2E test runs the full pipeline
with a real FFmpeg against a local-disk storage double.

## Makefile reference

| Command               | Action                                            |
|-----------------------|---------------------------------------------------|
| `make test`           | Run unit tests (`go test ./...`)                  |
| `make test-unit`      | Run unit tests                                    |
| `make test-integration`| Run integration tests (`-tags integration`)      |
| `make test-e2e`       | Run end-to-end tests (`-tags e2e`)                |
| `make test-all`       | Run unit + integration + e2e                      |
| `make vet`            | Run `go vet ./...`                                |
| `make tidy`           | Run `go mod tidy`                                 |

## Docker reference

| Command                                                                 | Action                                    |
|-------------------------------------------------------------------------|-------------------------------------------|
| `docker compose -f deployments/docker-compose.yml up --build`           | Build and run the full stack              |
| `docker compose -f deployments/docker-compose.yml up -d postgres redis` | Start only the backing services           |
| `docker compose -f deployments/docker-compose.yml down`                 | Stop the stack                            |
| `docker build -f deployments/Dockerfile.api -t yellowbird-api .`        | Build the API image alone                 |
| `docker build -f deployments/Dockerfile.worker -t yellowbird-worker .`  | Build the worker image alone              |

The worker image includes FFmpeg; the API image does not.

## Future scope

The roadmap is intentionally larger than the MVP. The project is meant to keep
raising the difficulty of the problems it solves rather than treating the MVP as
final.

**Workers and scaling**

- Distributed, horizontally scalable workers.
- Worker autoscaling based on queue depth and load.
- Smarter scheduling and job prioritization.
- Multi-node execution with per-node consumer identity and metrics.

**Processing quality**

- Per-title, quality-aware encoding.
- Intelligent bitrate and codec selection.
- Video-quality optimisation.
- Adaptive processing driven by source characteristics (resolution, codec,
  complexity).

**Media intelligence**

- ML-based thumbnail and frame selection.
- Geolocation and context-aware media processing.

**Operability**

- Stronger observability and tracing (OpenTelemetry, structured logging,
  per-job trace IDs).
- Chaos and failure testing (fault injection, partition and crash drills).
- More sophisticated media-processing pipelines (audio extraction, packaging,
  dynamic packaging, multi-track).

## Known limitations

- Login currently returns the user object; JWT issuance (`LoginResponse` with a
  token) is defined but not yet wired into the handler.
- User management endpoints (`/users`) are not protected by the auth middleware.
- Media status is set to `uploaded` on upload; there is no listener yet that
  flips it to `ready` when all renditions complete.
- `migrations/` and the YAML configs under `configs/` are reserved/stubs.

---

## CI

GitHub Actions (`.github/workflows/ci.yaml`) runs on push/PR to `main` and
`develop`: formatting check (`gofmt`), `go vet`, unit tests, integration tests,
E2E tests, and Docker image builds for both the API and the worker.

---

<img width="1784" height="1004" alt="Yellowbird" src="https://github.com/user-attachments/assets/3da40bd3-bca3-4b66-9bdc-d94d29264698" />

*kiiroitori (黄色い鳥)* — named after the Porsche 911 930 RUF CTR "Yellowbird".
The RUF CTR is known internally and model-designated by Ruf Automobile as the
CTR, which stands for Group C Turbo RUF. One of my favourite pieces of art. 
Built with ❤️ by amaanworks/dextertwts/dexisback
