# YellowBird

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-336791?style=flat&logo=postgresql)](https://www.postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-7+-DC382D?style=flat&logo=redis)](https://redis.io)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-6+-007808?style=flat&logo=ffmpeg)](https://ffmpeg.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

YellowBird is a high-throughput, fault-tolerant media processing pipeline written in Go. It decouples media ingestion from compute-intensive video transformations using Redis Streams consumer groups, transactional state coordination in PostgreSQL, and a distributed FFmpeg worker pool.

The system is designed for production reliability: **at-least-once job delivery**, **worker crash recovery via PEL claims**, **dead-letter queuing (DLQ)**, **concurrency-safe status aggregation with row-level locks**, and **deduplicated source media caching**.

---

## Architecture

The following diagram illustrates the end-to-end data flow from client ingestion to background rendition processing:

```mermaid
flowchart TD
    Client(["HTTP Client"]) -->|"1. Upload Media / REST"| API["API Server (cmd/api)"]

    subgraph Storage ["Storage & Persistence"]
        DB[("PostgreSQL<br/>(State & Row Locks)")]
        ObjStore[("Object Storage<br/>(Cloudinary / S3)")]
        Redis[("Redis Streams<br/>(Job Queue & DLQ)")]
    end

    subgraph Workers ["Worker Subsystem (cmd/worker)"]
        Pool["Worker Pool (Goroutines)"]
        FFmpeg[["FFmpeg Processing Engine<br/>(Thumbnails · Previews · Transcoding)"]]
        Pool --> FFmpeg
    end

    API -->|"2. Store Raw File"| ObjStore
    API -->|"3. Insert Media & Jobs"| DB
    API -->|"4. Fan-Out Tasks (XADD)"| Redis

    Redis -->|"5. Consume Jobs (XREADGROUP)"| Pool
    Pool -->|"6. Fetch Source Media"| ObjStore
    FFmpeg -->|"7. Upload Renditions"| ObjStore
    Pool -->|"8. Sync Status & Acknowledge (XACK)"| DB
```

### End-to-End Processing Flow

1. **Ingestion**: The client authenticates via JWT and uploads an image or video file to `POST /api/v1/media`.
2. **Storage & Recording**: The API streams the raw file to Object Storage, persists the `media` record in PostgreSQL (`status: uploaded`), and determines required child jobs.
3. **Job Fan-Out**: The API inserts corresponding child `jobs` (`queued`) into PostgreSQL and publishes task IDs to the Redis Stream `yellowbird:jobs` via `XADD`.
4. **Queue Consumption**: Worker processes listening on consumer group `yellowbird-workers` claim tasks via `XREADGROUP` using unique consumer IDs (`<hostname>-<pid>`).
5. **Execution**: The worker checks its thread-safe local cache (`SourceCache`), streams the source file if needed, and executes the designated FFmpeg pipeline (thumbnail extraction, preview clip, or resolution transcoding).
6. **Rendition Storage**: Transcoded artifacts are uploaded to Object Storage, and metadata records are saved to the `renditions` table.
7. **Transactional Synchronization**: Upon completing a job, the worker executes a PostgreSQL transaction with a row-level lock (`SELECT FOR UPDATE`) on the parent `media` record. If all sibling jobs are complete, the media transitions to `ready`; if a job exhausts retries, it transitions to `failed`.
8. **Acknowledgment**: The worker acknowledges the message in Redis via `XACK`, removing it from the Pending Entries List (PEL).

---

## System Design & Reliability Guarantees

### Distributed Queue & Consumer Groups
- **Stream Substrate**: Uses Redis Streams (`yellowbird:jobs`) backed by consumer groups (`yellowbird-workers`).
- **At-Least-Once Delivery**: Claimed messages remain in Redis's Pending Entries List (PEL) until explicitly acknowledged with `XACK`.
- **Bounded Concurrency**: Workers process tasks using an internal goroutine pool regulated by a semaphore channel (`WORKER_CONCURRENCY`).

### Fault Tolerance & Dead-Letter Queue (DLQ)
- **Retry Budget**: If processing fails due to transient faults (e.g. network blips during upload), the job status is reset to `queued` in PostgreSQL and left pending in Redis.
- **Dead-Letter Escalation**: If a job fails across 3 delivery attempts (`deliveryCount >= 3`), it is quarantined to `yellowbird:jobs:dlq`, marked as `failed` in PostgreSQL, and acknowledged from the primary stream.
- **Crash Recovery Loop**: An autonomous background routine runs every 30 seconds (`Worker.recoverPending`). It inspects the PEL for messages idle for over 5 minutes and re-claims them via `XCLAIM` to handle node crashes.

### Concurrency-Safe State Synchronization
- **Race-Free Fan-Out**: A single video upload fans out into up to 5 concurrent jobs (thumbnail, preview, 360p, 720p, 1080p).
- **Row-Level Locking**: Workers update sibling status through `mediaRepository.SyncStatus()`, utilizing `SELECT ... FOR UPDATE` inside a database transaction to eliminate race conditions when transitioning parent media from `processing` to `ready`.

### Performance Optimizations
- **Local Source Caching**: Workers maintain an in-memory `SourceCache` backed by `golang.org/x/sync/singleflight`. Parallel jobs targeting the same parent video share a single source download, eliminating redundant network I/O.
- **Fast Encoding Flags**: FFmpeg invocations leverage `-preset veryfast`, `-movflags +faststart`, and multi-threaded CPU encoding for low latency.

---

## Media Processing Profiles

| Job Type | Target Asset | Resolution / Profile | FFmpeg Pipeline Specs | Output Rendition |
| :--- | :--- | :--- | :--- | :--- |
| `thumbnail` | Image / Video | Frame 1 | `-frames:v 1 -q:v 2` | JPEG image |
| `preview` | Image / Video | 10s Clip | `-t 10 -c:v libx264 -preset fast -crf 28 -c:a aac -movflags +faststart` | 10s MP4 preview |
| `transcode` | Video Only | 360p | `-vf scale=-2:360 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 360p H.264 MP4 |
| `transcode` | Video Only | 720p | `-vf scale=-2:720 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 720p H.264 MP4 |
| `transcode` | Video Only | 1080p | `-vf scale=-2:1080 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 1080p H.264 MP4 |

---

## Quickstart

### Prerequisites
- [Go 1.26+](https://golang.org/dl/)
- [Docker & Docker Compose](https://www.docker.com/)
- [FFmpeg](https://ffmpeg.org/download.html) (if running worker outside Docker)
- A [Cloudinary](https://cloudinary.com/) account (or configured object storage provider)

### 1. Environment Configuration

Copy the example environment file and set your credentials:

```bash
cp .env.example .env
```

| Variable | Description | Default / Example |
| :--- | :--- | :--- |
| `PORT` | API server listening port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@localhost:5432/yellowbird?sslmode=disable` |
| `JWT_SECRET` | Secret key for signing JWT tokens | `your_jwt_secret_key` |
| `REDIS_ADDR` | Redis host and port | `localhost:6379` |
| `REDIS_PASSWORD` | Redis authentication password | `""` |
| `REDIS_DB` | Redis database index | `0` |
| `CLOUDINARY_CLOUD_NAME`| Cloudinary cloud identifier | `your_cloud_name` |
| `CLOUDINARY_API_KEY` | Cloudinary API key | `your_api_key` |
| `CLOUDINARY_API_SECRET`| Cloudinary API secret | `your_api_secret` |
| `WORKER_CONCURRENCY` | Maximum concurrent FFmpeg worker goroutines | `4` |

### 2. Run with Docker Compose

To start the complete environment (API server, worker pool, and Redis):

```bash
docker compose -f deployments/docker-compose.yml up --build
```

To run only the backing datastores:

```bash
docker compose -f deployments/docker-compose.yml up -d redis
```

### 3. Run Locally (Bare Metal)

Ensure PostgreSQL and Redis are running, then start each service:

```bash
# Terminal 1: Start the API server
go run ./cmd/api

# Terminal 2: Start the background worker
go run ./cmd/worker
```

---

## API Reference

All protected endpoints require an `Authorization: Bearer <token>` header obtained from `/api/v1/users/login`.

### Endpoints Overview

| Scope | Method | Endpoint | Access | Description |
| :--- | :--- | :--- | :--- | :--- |
| **Auth** | `POST` | `/api/v1/users/register` | Public | Register a new user |
| | `POST` | `/api/v1/users/login` | Public | Authenticate user and receive JWT |
| | `GET` | `/api/v1/users/:id` | Public | Retrieve user profile |
| **Projects** | `POST` | `/api/v1/projects` | Protected | Create an asset project |
| | `GET` | `/api/v1/projects` | Protected | List caller's projects |
| | `GET` | `/api/v1/projects/:id` | Protected | Get project details |
| **Media** | `POST` | `/api/v1/media` | Protected | Upload media file (`multipart/form-data`) |
| | `GET` | `/api/v1/media` | Protected | List media (`?project_id=<uuid>`) |
| | `GET` | `/api/v1/media/:id` | Protected | Get media status & metadata |
| **Jobs** | `GET` | `/api/v1/jobs` | Protected | List processing jobs (`?media_id=<uuid>`) |
| | `GET` | `/api/v1/jobs/:id` | Protected | Get job status & error logs |
| **Renditions**| `GET` | `/api/v1/renditions` | Protected | List generated outputs (`?media_id=<uuid>`) |
| | `GET` | `/api/v1/renditions/:id`| Protected | Get rendition details & download URL |
| **Ops** | `GET` | `/health` | Public | Healthcheck endpoint |

### Example Workflow

#### 1. Upload Media
```bash
curl -X POST http://localhost:8080/api/v1/media \
  -H "Authorization: Bearer $TOKEN" \
  -F "project_id=7bf3b6e8-2512-4c28-98e3-514782bb0dc1" \
  -F "file=@sample.mp4"
```

Response (`HTTP 201 Created`):
```json
{
  "id": "c4b9d031-64d8-4f51-b0e2-e1d5e396e95c",
  "project_id": "7bf3b6e8-2512-4c28-98e3-514782bb0dc1",
  "filename": "sample.mp4",
  "media_type": "video",
  "status": "uploaded",
  "created_at": "2026-10-15T22:00:00Z"
}
```

#### 2. Query Media Status & Renditions
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/media/c4b9d031-64d8-4f51-b0e2-e1d5e396e95c
```

Once all sibling jobs complete, status transitions from `processing` to `ready`:
```json
{
  "id": "c4b9d031-64d8-4f51-b0e2-e1d5e396e95c",
  "status": "ready",
  "renditions": [
    { "type": "thumbnail", "resolution": null, "url": "https://res.cloudinary.com/.../thumb.jpg" },
    { "type": "preview", "resolution": null, "url": "https://res.cloudinary.com/.../preview.mp4" },
    { "type": "transcode", "resolution": "720p", "url": "https://res.cloudinary.com/.../720p.mp4" },
    { "type": "transcode", "resolution": "1080p", "url": "https://res.cloudinary.com/.../1080p.mp4" }
  ]
}
```

---

## Testing & Quality Assurance

The codebase maintains a comprehensive testing pyramid including hermetic unit tests, concurrent race detection, isolated database integration suites, and end-to-end transcoding runs:

```bash
# Run unit tests
make test-unit

# Run Go static analysis
make vet

# Run tests with race condition detector
go test -race -count=1 ./internal/...

# Run PostgreSQL and Redis integration tests (requires Docker)
make test-integration

# Run end-to-end pipeline verification with real FFmpeg
make test-e2e

# Run all test suites
make test-all
```

---

## Repository Structure

```text
YellowBird/
├── cmd/
│   ├── api/                  # API server entrypoint & router initialization
│   └── worker/               # Background worker entrypoint, Redis consumer & recovery
├── internal/
│   ├── auth/                 # JWT authentication & bcrypt hashing
│   ├── config/               # Environment configuration loader (koanf + godotenv)
│   ├── db/                   # PostgreSQL connection pool & schema migrations
│   ├── domain/               # Domain-driven architectural packages
│   │   ├── user/             # User accounts & credentials
│   │   ├── project/          # Project scoping & ownership
│   │   ├── media/            # Ingestion, job fan-out, and status sync
│   │   ├── job/              # Job state machine & queue dispatch
│   │   └── rendition/        # Processed media rendition entities
│   ├── queue/                # Redis Streams client (XADD, XREADGROUP, XCLAIM, XACK, DLQ)
│   ├── storage/              # Provider-agnostic storage interface & Cloudinary provider
│   ├── worker/               # Worker runtime, processor registry & FFmpeg engines
│   └── server/               # Gin web server, routing, and HTTP middleware
├── deployments/
│   ├── Dockerfile.api        # Minimal container for the API server
│   ├── Dockerfile.worker     # Alpine container with FFmpeg for background workers
│   └── docker-compose.yml    # Local multi-service orchestration
├── tests/
│   ├── integration/          # Testcontainers-based PostgreSQL & Redis test suite
│   └── e2e/                  # End-to-end pipeline validation with real FFmpeg
├── Makefile                  # Build, test, and lint commands
└── go.mod                    # Go module dependencies
```

---

## Origin & Credits

*kiiroitori (黄色い鳥)* — named after the legendary Porsche 911 930 RUF CTR "Yellowbird", renowned for raw speed, engineering discipline, and focused mechanical execution.

Crafted by **[dexisback](https://github.com/dexisback)**.
