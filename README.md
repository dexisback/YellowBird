# YellowBird

YellowBird is a production-style, asynchronous video and media processing backend written in Go. When a user uploads a video or image, YellowBird durably stores metadata, fans out processing tasks across a distributed queue, and executes background transcoding, preview clipping, and thumbnail extraction via FFmpeg.

The project serves as a real-world systems-engineering playground exploring resilient backend architecture: **at-least-once job delivery**, **Redis Streams consumer groups**, **PostgreSQL transaction concurrency with row locking**, **dead-letter queuing (DLQ)**, **worker crash recovery**, and **graceful shutdown**.

---

## Table of Contents

- [Core Architectural Mental Model](#core-architectural-mental-model)
- [System Architecture](#system-architecture)
- [How It Works: Step-by-Step](#how-it-works-step-by-step)
- [The Processing Pipeline & State Flow](#the-processing-pipeline--state-flow)
  - [Media Lifecycle](#media-lifecycle)
  - [Job Types & Outputs](#job-types--outputs)
- [Job Queue & Reliability Semantics](#job-queue--reliability-semantics)
  - [Redis Streams & Consumer Groups](#redis-streams--consumer-groups)
  - [Failure Handling, Retries & DLQ](#failure-handling-retries--dlq)
  - [Crash Recovery Loop](#crash-recovery-loop)
  - [Graceful Shutdown](#graceful-shutdown)
- [API Reference](#api-reference)
  - [Authentication](#authentication)
  - [Projects](#projects)
  - [Media](#media)
  - [Jobs](#jobs)
  - [Renditions](#renditions)
  - [Health & Ops](#health--ops)
- [Repository Structure](#repository-structure)
- [Package Architecture](#package-architecture)
- [Local Development & Setup](#local-development--setup)
  - [Prerequisites](#prerequisites)
  - [Configuration (.env)](#configuration-env)
  - [Running with Docker Compose](#running-with-docker-compose)
  - [Running Locally (Bare Metal)](#running-locally-bare-metal)
- [Testing Strategy](#testing-strategy)
- [Roadmap](#roadmap)
  - [Currently Implemented](#currently-implemented)
  - [Future Systems-Engineering Experiments](#future-systems-engineering-experiments)
- [Origin & Credits](#origin--credits)

---

## Core Architectural Mental Model

YellowBird separates responsibility across storage, coordination, state, and execution:

| Component | Technology | Core Responsibility |
| :--- | :--- | :--- |
| **API Server** | Go / Gin | Handles client HTTP requests, authentication, validation, durable record creation, and enqueuing jobs. |
| **Durable State** | PostgreSQL 16+ | Single source of truth for users, projects, media metadata, job states, and rendition records. Uses row-level locks (`SELECT FOR UPDATE`) for race-free status synchronizations. |
| **Job Queue** | Redis Streams | Ephemeral work-delivery and coordination substrate. Distributes job IDs to competing consumers via Consumer Groups (`XREADGROUP`). |
| **Worker Nodes** | Go / FFmpeg | Headless, stateless compute consumers. Dequeues jobs, downloads source media, runs FFmpeg transformations, uploads output renditions, and updates durable state. |
| **Object Storage** | Cloudinary (abstracted) | Stores original raw media uploads and processed rendition outputs behind a provider-agnostic `storage.Storage` interface. |

---

## System Architecture

```mermaid
flowchart TD
    Client(["HTTP Client"]) -->|Multipart Upload / REST| API["API Server (cmd/api)"]

    subgraph Storage & State
        DB[("PostgreSQL\n(Durable State / ACID)")]
        Redis[("Redis Streams\n(yellowbird:jobs)")]
        Cloudinary[("Cloudinary\n(Object Storage)")]
    end

    subgraph Worker Subsystem ["Worker Subsystem (cmd/worker)"]
        W1["Worker Process 1\n(Consumer: host-pid)"]
        W2["Worker Process 2\n(Consumer: host-pid)"]
        Registry["Processor Registry"]
        P_Thumb["Thumbnail Processor"]
        P_Prev["Preview Processor"]
        P_Trans["Transcode Processor"]
        FFmpeg[["FFmpeg Engine"]]
    end

    API -->|1. Store Raw Upload| Cloudinary
    API -->|2. Insert Media & Jobs| DB
    API -->|3. XADD Job ID| Redis

    Redis -->|4. XREADGROUP| W1
    Redis -->|4. XREADGROUP| W2

    W1 & W2 -->|5. Dispatch by Job Type| Registry
    Registry --> P_Thumb & P_Prev & P_Trans

    P_Thumb & P_Prev & P_Trans -->|6. Download Source| Cloudinary
    P_Thumb & P_Prev & P_Trans -->|7. Execute Transcode/Extract| FFmpeg
    FFmpeg -->|8. Upload Rendition| Cloudinary
    P_Thumb & P_Prev & P_Trans -->|9. Insert Rendition Record| DB

    W1 & W2 -->|10. Update Job Status & Sync Media| DB
    W1 & W2 -->|11. XACK / Move to DLQ| Redis
```

---

## How It Works: Step-by-Step

1. **Authentication & Project Creation**:
   - A user registers and logs in via `/api/v1/users/login`. The API verifies the bcrypt password hash and issues a signed HMAC-SHA256 (HS256) JWT valid for 24 hours.
   - The user creates a project (`POST /api/v1/projects`) to hold media assets.

2. **Media Upload**:
   - The user sends a `multipart/form-data` request with the media file to `POST /api/v1/media`.
   - The API uploads the raw file to Cloudinary, extracts metadata (size, MIME type), and inserts a `Media` row into PostgreSQL with status `uploaded`.

3. **Job Fan-Out**:
   - In the same request cycle, `media.Service` fans out required background jobs:
     - **Image**: creates a `thumbnail` job and a `preview` job.
     - **Video**: creates a `thumbnail` job, a `preview` job, and three `transcode` jobs targeting resolutions **360p**, **720p**, and **1080p**.
   - Each job is inserted into PostgreSQL with status `queued` and pushed to the Redis Stream `yellowbird:jobs` via `XADD`.

4. **Worker Consumption**:
   - Background workers listening on consumer group `yellowbird-workers` claim jobs via `XREADGROUP`.
   - The worker marks the job as `running` in PostgreSQL and triggers `mediaRepository.SyncStatus()`, transitioning the parent media from `uploaded` $\rightarrow$ `processing`.

5. **FFmpeg Execution & Rendition Upload**:
   - The worker retrieves the processor registered for the job type (`thumbnail`, `preview`, or `transcode`).
   - The processor downloads the source media, streams it to a local temporary file, executes the tailored FFmpeg command, uploads the resulting artifact to Cloudinary, and persists a `Rendition` row in PostgreSQL.

6. **Job Completion & Media Status Sync**:
   - The worker marks the job as `completed` in PostgreSQL and acknowledges the message in Redis via `XACK`.
   - The worker invokes `mediaRepository.SyncStatus()`. Inside a database transaction with a `SELECT FOR UPDATE` row lock on the media row:
     - If **all** sibling jobs for this media are `completed`, media status transitions to `ready`.
     - If any job encountered terminal failure, media status transitions to `failed`.

---

## The Processing Pipeline & State Flow

### Media Lifecycle

```mermaid
stateDiagram-v2
    [*] --> uploaded: API Upload Succeeded
    uploaded --> processing: First Worker Starts Job
    processing --> processing: Sibling Jobs Completing
    processing --> ready: ALL Jobs Completed
    processing --> failed: Terminal Job Failure / DLQ
    ready --> [*]
    failed --> [*]
```

- **`uploaded`**: Media record persisted; child processing jobs created in PostgreSQL and enqueued in Redis.
- **`processing`**: At least one child job has transitioned to `running` or `queued`.
- **`ready`**: All child jobs (`thumbnail`, `preview`, and `transcode` variants) have reached `completed`.
- **`failed`**: A child job suffered terminal failure (e.g. exceeded retry limit, unknown job type, unrecoverable FFmpeg error).

### Job Types & Outputs

| Job Type | Triggered On | Target Parameter | FFmpeg Pipeline Specs | Output Rendition |
| :--- | :--- | :--- | :--- | :--- |
| `thumbnail` | Image / Video | None | `-frames:v 1 -q:v 2` | Single-frame JPEG image |
| `preview` | Image / Video | None | `-t 10 -c:v libx264 -preset fast -crf 28 -c:a aac -movflags +faststart` | 10-second compressed MP4 clip |
| `transcode` | Video Only | `360` | `-vf scale=-2:360 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 360p H.264 / AAC MP4 |
| `transcode` | Video Only | `720` | `-vf scale=-2:720 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 720p H.264 / AAC MP4 |
| `transcode` | Video Only | `1080` | `-vf scale=-2:1080 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart` | 1080p H.264 / AAC MP4 |

---

## Job Queue & Reliability Semantics

### Redis Streams & Consumer Groups

YellowBird uses Redis Streams for distributed, at-least-once job delivery without message loss:

- **Stream Key**: `yellowbird:jobs`
- **Consumer Group**: `yellowbird-workers`
- **Consumer Identity**: `<hostname>-<pid>` (uniquely identifies every running worker process)
- **Dead-Letter Stream**: `yellowbird:jobs:dlq`
- **Claim Timeout (Idle)**: 5 minutes
- **Recovery Interval**: 30 seconds

```
[API] --- XADD ---> [yellowbird:jobs Stream]
                          │
            ┌─────────────┴─────────────┐
            ▼                           ▼
    Worker 1 (host-101)          Worker 2 (host-204)
     XREADGROUP COUNT 1           XREADGROUP COUNT 1
            │                           │
         Pending                     Pending
            │                           │
       (Processing)                (Processing)
            │                           │
          XACK                        XACK
```

### Failure Handling, Retries & DLQ

```mermaid
flowchart TD
    A["Job Dequeued (Delivery 1)"] --> B["Worker Starts Job (DB: running)"]
    B --> C{"FFmpeg / Processor Result"}
    
    C -->|Success| D["DB: completed"]
    D --> E["Sync Media Status (all done? -> ready)"]
    E --> F["XACK Message"]
    
    C -->|Failure| G["DB: reset status to queued"]
    G --> H["Message left Pending in Redis PEL"]
    
    H --> I["Recovery Loop (runs every 30s)"]
    I --> J{"Message Idle > 5m?"}
    J -->|No| H
    J -->|Yes| K{"Delivery Count >= 3?"}
    
    K -->|No (Delivery 2, 3)| L["XCLAIM message by active worker"]
    L --> B
    
    K -->|Yes (Exhausted)| M["DB: fail job ('max retry limit reached')"]
    M --> N["Sync Media Status (media -> failed)"]
    N --> O["XADD to yellowbird:jobs:dlq"]
    O --> P["XACK original message from main stream"]
```

1. **At-Least-Once Delivery**: When a worker dequeues a message, Redis moves it to the **Pending Entries List (PEL)**. It remains there until explicitly acknowledged.
2. **Transient Failures**: If processing fails, `handleJobFailure()` resets the PostgreSQL job status from `running` back to `queued` and leaves the message pending in Redis.
3. **Recovery of Abandoned Messages**: If a worker process crashes mid-job, the message stays idle in the PEL. Every 30 seconds, `Worker.recoverPending()` inspects the PEL for messages idle for $\ge 5\text{ minutes}$ and claims them via `XCLAIM`.
4. **Dead-Letter Queue (DLQ)**: Once a message reaches 3 delivery attempts (`deliveryCount >= 3`), it is quarantined:
   - The job is marked `failed` in PostgreSQL.
   - `mediaRepository.SyncStatus()` transitions the parent media to `failed`.
   - The job is written to `yellowbird:jobs:dlq` with its error context and delivery count.
   - The original message is acknowledged (`XACK`) and removed from `yellowbird:jobs`.

### Graceful Shutdown

Both the API and Worker processes register context cancellation via `signal.NotifyContext` listening for `SIGINT` (Ctrl+C) and `SIGTERM`:

- **API (`cmd/api`)**:
  - Stops accepting incoming HTTP connections.
  - Drains active in-flight requests within a 10-second timeout window.
  - Closes Redis connections and the PostgreSQL connection pool.
- **Worker (`cmd/worker`)**:
  - Halts the recovery ticker and dequeue loop.
  - Finishes active processor execution.
  - Closes Redis and PostgreSQL connections cleanly without abandoning running tasks in invalid states.

---

## API Reference

All responses return standard JSON. Endpoints marked **Protected** require the HTTP header:
`Authorization: Bearer <token>`

### Authentication

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/users/register` | Public | Register a new user account with hashed password. |
| `POST` | `/api/v1/users/login` | Public | Authenticate user and receive a signed JWT token. |
| `GET` | `/api/v1/users` | Public | List all user accounts. |
| `GET` | `/api/v1/users/:id` | Public | Fetch user profile by UUID. |
| `DELETE` | `/api/v1/users/:id` | Public | Remove a user account. |

#### Sample Login Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "7bf3b6e8-2512-4c28-98e3-514782bb0dc1",
    "name": "Alex",
    "email": "alex@example.com",
    "created_at": "2026-09-08T00:00:00Z",
    "updated_at": "2026-09-08T00:00:00Z"
  }
}
```

### Projects

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/projects` | Protected | Create a new project owned by the authenticated user. |
| `GET` | `/api/v1/projects` | Protected | List all projects owned by the caller. |
| `GET` | `/api/v1/projects/:id` | Protected | Get single project details. |
| `PUT` | `/api/v1/projects/:id` | Protected | Update project metadata (e.g. name, description). |
| `DELETE` | `/api/v1/projects/:id` | Protected | Delete project and associated records. |

### Media

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/media` | Protected | Upload media file (`multipart/form-data`), persist record, and trigger job fan-out. |
| `GET` | `/api/v1/media` | Protected | List media assets (filterable by `?project_id=<uuid>`). |
| `GET` | `/api/v1/media/:id` | Protected | Get media status, storage key, and metadata. |
| `PUT` | `/api/v1/media/:id` | Protected | Update media attributes. |
| `DELETE` | `/api/v1/media/:id` | Protected | Delete media record. |

#### Upload Multipart Payload:
- `project_id`: UUID of the parent project (form value).
- `file`: The binary media file (`multipart/form-data`).

### Jobs

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/jobs` | Protected | Manually trigger a processing job. |
| `GET` | `/api/v1/jobs` | Protected | List jobs (filterable by `?media_id=<uuid>`). |
| `GET` | `/api/v1/jobs/:id` | Protected | Get specific job status and error logs. |
| `DELETE` | `/api/v1/jobs/:id` | Protected | Delete a job entry. |

#### Create Job Payload:
```json
{
  "media_id": "7bf3b6e8-2512-4c28-98e3-514782bb0dc1",
  "type": "transcode",
  "target_height": 720
}
```

### Renditions

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/renditions` | Protected | Record a completed rendition metadata entry. |
| `GET` | `/api/v1/renditions` | Protected | List output renditions (filterable by `?media_id=<uuid>`). |
| `GET` | `/api/v1/renditions/:id` | Protected | Get specific rendition details and download URL. |
| `DELETE` | `/api/v1/renditions/:id` | Protected | Delete a rendition entry. |

### Health & Ops

| Method | Path | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Public | Liveness check returning HTTP 200 OK. |

---

## Repository Structure

```
YellowBird/
├── cmd/
│   ├── api/                  # API entrypoint (HTTP server, router wiring, graceful shutdown)
│   └── worker/               # Background worker entrypoint (processors, Redis consumer, recovery)
├── internal/
│   ├── auth/                 # JWT generation/validation (HS256) & bcrypt password hashing
│   ├── config/               # Environment configuration loader (koanf + godotenv)
│   ├── db/                   # PostgreSQL connection pool & schema auto-migration
│   ├── domain/               # Domain modules (clean architecture)
│   │   ├── user/             # User entity, repository, service, handler, routes
│   │   ├── project/          # Project management (owner-scoped)
│   │   ├── media/            # Media uploads, fan-out logic, status synchronization
│   │   ├── job/              # Job state machine, validation, stream enqueue
│   │   └── rendition/        # Output renditions (thumbnail, preview, transcodes)
│   ├── queue/                # Redis Streams client (XADD, XREADGROUP, XCLAIM, XACK, DLQ)
│   ├── storage/              # Provider-agnostic storage interface + Cloudinary implementation
│   ├── worker/               # Worker runtime, processor registry, FFmpeg execution engines
│   ├── server/               # Gin web server, routing, middleware (Recovery, Logging, Auth)
│   ├── mocks/                # Generated/mock implementations for unit testing
│   └── testutil/             # Test containers (isolated throwaway PostgreSQL via Docker)
├── deployments/
│   ├── Dockerfile.api        # Minimal alpine container for API
│   ├── Dockerfile.worker     # Alpine container for worker with FFmpeg installed
│   └── docker-compose.yml    # Multi-container orchestration (Postgres, Redis, API, Worker)
├── tests/
│   ├── integration/          # PostgreSQL + Redis integration test suite (testcontainers)
│   └── e2e/                  # End-to-end pipeline verification with real FFmpeg
├── Makefile                  # Build, test, and vet shortcuts
└── go.mod                    # Go module dependencies (Go 1.26+)
```

---

## Package Architecture

```mermaid
flowchart TD
    subgraph Presentation & Routing
        API_Main["cmd/api/main.go"] --> Server["internal/server"]
        Server --> Middleware["internal/server/middleware\n(Auth, RequestID, Recovery, Logging)"]
        Server --> Handlers["Domain Handlers\n(user, project, media, job, rendition)"]
    end

    subgraph Business Logic ["Domain Layer (internal/domain)"]
        Handlers --> Services["Domain Services\n(user, project, media, job, rendition)"]
        Services --> Repositories["Domain Repositories\n(GORM / SQL)"]
        media_svc["media.Service"] -->|Job Fan-Out| job_svc["job.Service"]
    end

    subgraph Infrastructure Layer
        job_svc --> Queue["internal/queue\n(Redis Streams)"]
        media_svc --> Storage["internal/storage\n(Cloudinary)"]
        Repositories --> DB_Pkg["internal/db\n(PostgreSQL Pool)"]
        Services --> Auth_Pkg["internal/auth\n(JWT & Bcrypt)"]
    end

    subgraph Worker Runtime
        Worker_Main["cmd/worker/main.go"] --> Worker["internal/worker\n(Worker Loop & DLQ)"]
        Worker --> Registry["internal/worker.Registry"]
        Registry --> Processors["Processors\n(Thumbnail, Preview, Transcode)"]
        Processors --> FFmpeg_Exec[["FFmpeg Exec / Local Temp"]]
        Processors --> Storage
        Processors --> Repositories
    end
```

---

## Local Development & Setup

### Prerequisites

- **Go**: 1.26+ installed
- **PostgreSQL**: 16+ (or Docker)
- **Redis**: 7+ (or Docker)
- **FFmpeg**: Installed locally and accessible in `$PATH` (required for running `cmd/worker` locally)
- **Cloudinary Account**: Cloud Name, API Key, API Secret

### Configuration (.env)

Copy `.env.example` to `.env` and supply your credentials:

```bash
cp .env.example .env
```

| Variable | Description | Example |
| :--- | :--- | :--- |
| `PORT` | API HTTP port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://yellowbird:yellowbird@localhost:5432/yellowbird?sslmode=disable` |
| `JWT_SECRET` | Secret key used for signing JWT tokens | `your_secret_key_change_me` |
| `REDIS_ADDR` | Redis address | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password (if any) | `""` |
| `REDIS_DB` | Redis database index | `0` |
| `CLOUDINARY_CLOUD_NAME` | Cloudinary Cloud Name | `your_cloud_name` |
| `CLOUDINARY_API_KEY` | Cloudinary API Key | `your_api_key` |
| `CLOUDINARY_API_SECRET` | Cloudinary API Secret | `your_api_secret` |

### Running with Docker Compose

The easiest way to run the entire system (including Postgres, Redis, API, and Worker with FFmpeg) is Docker Compose:

```bash
docker compose -f deployments/docker-compose.yml up --build
```

To run only the backing data services:

```bash
docker compose -f deployments/docker-compose.yml up -d postgres redis
```

### Running Locally (Bare Metal)

1. Start PostgreSQL and Redis:
   ```bash
   docker compose -f deployments/docker-compose.yml up -d postgres redis
   ```

2. Start the API server:
   ```bash
   go run ./cmd/api
   ```

3. Start the background worker (in a separate terminal):
   ```bash
   go run ./cmd/worker
   ```

---

## Testing Strategy

The repository employs a multi-tiered testing strategy:

| Test Tier | Scope | Requirements | Command |
| :--- | :--- | :--- | :--- |
| **Unit Tests** | Fast, hermetic tests using mocks and in-memory `miniredis` | None | `go test ./...`<br>`make test` |
| **Race Detector** | Verifies zero data races across concurrent queues and workers | None | `go test -race ./internal/...` |
| **Static Analysis** | Go static analysis and compiler vetting | None | `go vet ./...`<br>`make vet` |
| **Integration Tests** | Tests real PostgreSQL transactions, row locking (`SELECT FOR UPDATE`), and Redis streams | Docker (testcontainers) | `go test -tags=integration ./...`<br>`make test-integration` |
| **End-to-End (E2E)** | Full lifecycle from upload $\rightarrow$ stream $\rightarrow$ real FFmpeg execution $\rightarrow$ rendition | Docker + FFmpeg | `go test -tags=e2e ./tests/e2e/...`<br>`make test-e2e` |

To run the complete test suite across all tiers:

```bash
make test-all
```

### Continuous Integration (CI)

GitHub Actions (`.github/workflows/ci.yaml`) automatically validates every push and pull request targeting `main` and `develop`:

1. **Dependency Verification**: `go mod verify`
2. **Formatting Enforcement**: `gofmt -l .` (fails on unformatted files)
3. **Static Analysis**: `go vet ./...`
4. **Hermetic Unit Tests**: `go test ./...`
5. **Race Detection**: `go test -race ./internal/...`
6. **PostgreSQL Integration Tests**: `go test -tags=integration ./tests/integration/...` (via Docker testcontainers)
7. **FFmpeg End-to-End Pipeline**: `go test -tags=e2e ./tests/e2e/...`
8. **Binary Builds**: `go build ./cmd/api` and `go build ./cmd/worker`
9. **Docker Builds**: Validates `deployments/Dockerfile.api` and `deployments/Dockerfile.worker`

To run the primary validation checks locally before pushing:

```bash
gofmt -l .
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./internal/...
go build -o /dev/null ./cmd/api
go build -o /dev/null ./cmd/worker
```


---

## Roadmap

### Currently Implemented

- [x] RESTful API for Users, Projects, Media, Jobs, and Renditions.
- [x] JWT Authentication (HS256) & Bcrypt password security.
- [x] Asynchronous job fan-out (Thumbnail, Preview, 360p/720p/1080p Transcoding).
- [x] Cloudinary storage abstraction with upload/download streaming.
- [x] Redis Streams consumer groups with per-worker consumer identities.
- [x] FFmpeg media processing pipeline for video scaling and image extraction.
- [x] 3-attempt retry semantics and Dead-Letter Queue (`yellowbird:jobs:dlq`).
- [x] 30s background recovery loop claiming abandoned jobs ($> 5\text{ min}$ idle).
- [x] PostgreSQL media status lifecycle (`uploaded` $\rightarrow$ `processing` $\rightarrow$ `ready` / `failed`) with row-level transaction locks.
- [x] Idiomatic graceful shutdown (`signal.NotifyContext`) for API and Worker.
- [x] Multi-tier test suite (Unit, Race, Integration with testcontainers, E2E with real FFmpeg).

### Future Systems-Engineering Experiments

- [ ] **Dynamic Horizontal Worker Autoscaling**: KEDA or custom scaler reacting dynamically to Redis stream pending queue depth.
- [ ] **Job Prioritization & Quality-Aware Encoding**: Priority lanes for thumbnail/preview vs expensive 1080p transcodes.
- [ ] **Adaptive Source-Driven Transcoding**: Inspect source bitrate/resolution using `ffprobe` to skip unnecessary upscaling.
- [ ] **Distributed Tracing & Metrics**: OpenTelemetry spans propagating across HTTP request $\rightarrow$ Redis stream $\rightarrow$ Worker FFmpeg execution.
- [ ] **Multi-Node Consumer Metrics**: Prometheus metrics exporter for worker processing latency, PEL size, and DLQ rates.
- [ ] **ML-Based Frame Selection**: Intelligent scene-detection for keyframe thumbnail selection.

---

## Origin & Credits

*kiiroitori (黄色い鳥)* — named after the legendary Porsche 911 930 RUF CTR "Yellowbird". The RUF CTR is known internally and model-designated by Ruf Automobile as the CTR (Group C Turbo RUF).

Built with engineering passion by **amaanworks** / **dextertwts** / **dexisback**.
