# YellowBird Backend Architecture & Implementation Guide
> **Comprehensive API Reference, Data Contracts, Processing Pipelines, and Frontend Integration Guide**

---

## Table of Contents

1. [Executive Summary & Architectural Purpose](#1-executive-summary--architectural-purpose)
2. [System Topology & Infrastructure Overview](#2-system-topology--infrastructure-overview)
3. [Networking, Server Configuration & CORS](#3-networking-server-configuration--cors)
4. [Authentication & Authorization Protocol](#4-authentication--authorization-protocol)
5. [Data Models, Enums & Database Contracts](#5-data-models-enums--database-contracts)
6. [Complete REST API Specification](#6-complete-rest-api-specification)
   - [Health & Ops](#61-health--ops)
   - [Users & Authentication](#62-users--authentication)
   - [Projects](#63-projects)
   - [Media Ingestion & Assets](#64-media-ingestion--assets)
   - [Processing Jobs](#65-processing-jobs)
   - [Output Renditions](#66-output-renditions)
7. [The Asynchronous Processing Pipeline](#7-the-asynchronous-processing-pipeline)
   - [Media Fan-Out Pipeline Mechanics](#71-media-fan-out-pipeline-mechanics)
   - [FFmpeg Command Specifications](#72-ffmpeg-command-specifications)
   - [Lifecycle State Machine & Synchronization](#73-lifecycle-state-machine--synchronization)
   - [Worker Failure Recovery & Dead-Letter Queue (DLQ)](#74-worker-failure-recovery--dead-letter-queue-dlq)
8. [Frontend Integration Blueprints (TypeScript & JavaScript)](#8-frontend-integration-blueprints-typescript--javascript)
   - [Complete TypeScript Data Contracts & Interfaces](#81-complete-typescript-data-contracts--interfaces)
   - [API Client Implementation with Interceptors](#82-api-client-implementation-with-interceptors)
   - [Multipart File Upload with Progress Tracking](#83-multipart-file-upload-with-progress-tracking)
   - [Media Polling & Processing State Handler](#84-media-polling--processing-state-handler)
   - [Video Player Quality Switcher & Asset Resolution](#85-video-player-quality-switcher--asset-resolution)
9. [Error Handling & Edge Cases](#9-error-handling--edge-cases)
10. [Local Development & Environment Setup](#10-local-development--environment-setup)

---

## 1. Executive Summary & Architectural Purpose

**YellowBird** is a distributed, asynchronous media ingestion and transcoding backend written in Go. It enables users to organize digital media within scoped projects, upload high-resolution video and image assets, and automatically fan out distributed background processing pipelines across a cluster of headless workers.

### Key Capabilities
- **Durable Media Ingestion**: Direct multipart/form-data streaming to Cloudinary with metadata persistence in PostgreSQL.
- **Automated Asynchronous Fan-Out**: Video uploads immediately trigger parallel FFmpeg jobs for **Thumbnail extraction**, **10-second Preview generation**, and **Multi-Bitrate H.264 Transcoding (360p, 720p, 1080p)**.
- **At-Least-Once Delivery**: Task distribution using Redis Streams (`XADD`, `XREADGROUP`, `XACK`) with consumer groups.
- **Fault-Tolerant Worker Pool**: Crash recovery via Redis Pending Entries List (PEL) reclamation and quarantine in Dead-Letter Queues (DLQ) after 3 delivery attempts.
- **Race-Free Concurrency**: State synchronization using PostgreSQL transaction row-level locking (`SELECT FOR UPDATE`).

---

## 2. System Topology & Infrastructure Overview

YellowBird splits concerns across two standalone Go binaries coordinated via PostgreSQL and Redis Streams:

```mermaid
flowchart TD
    subgraph FrontendApp ["Frontend WebApp (Client)"]
        UI["UI / SPA / Next.js / Vite"]
    end

    subgraph API_Tier ["API Gateway Tier (cmd/api)"]
        Gin["Gin HTTP Engine (:8080)"]
        MW["Middleware (CORS, Auth, RequestID, Recovery, Logging)"]
        Handlers["Domain Handlers (User, Project, Media, Job, Rendition)"]
        Services["Domain Services"]
    end

    subgraph Storage_State ["State & Persistence"]
        DB[("PostgreSQL 16+\n• Users / Projects\n• Media Metadata\n• Job State Machine\n• Renditions Metadata")]
        RedisStreams[("Redis 7+ Streams\n• yellowbird:jobs (Work Queue)\n• yellowbird:jobs:dlq (Dead Letters)")]
        Cloudinary[("Cloudinary Object Storage\n• Source Uploads\n• Output Rendition Artifacts")]
    end

    subgraph Worker_Pool ["Worker Compute Pool (cmd/worker)"]
        W1["Worker Instance 1\n(host-pid)"]
        W2["Worker Instance 2\n(host-pid)"]
        Registry["Processor Registry"]
        P_Thumb["Thumbnail Processor"]
        P_Prev["Preview Processor"]
        P_Trans["Transcode Processor"]
        FFmpegLocal[["FFmpeg Engine / Temp Filesystem"]]
    end

    UI -->|HTTP REST / Multipart| Gin
    Gin --> MW --> Handlers --> Services
    Services -->|Persist / Read State| DB
    Services -->|XADD job_id| RedisStreams
    Services -->|Upload Raw Source| Cloudinary

    RedisStreams -->|XREADGROUP yellowbird-workers| W1 & W2
    W1 & W2 --> Registry
    Registry --> P_Thumb & P_Prev & P_Trans
    P_Thumb & P_Prev & P_Trans -->|Download Raw Source| Cloudinary
    P_Thumb & P_Prev & P_Trans -->|Execute Transcode/Scale| FFmpegLocal
    P_Thumb & P_Prev & P_Trans -->|Upload Artifacts| Cloudinary
    P_Thumb & P_Prev & P_Trans -->|Insert Rendition Record| DB
    W1 & W2 -->|"SyncStatus() / SELECT FOR UPDATE"| DB
    W1 & W2 -->|XACK / MoveToDLQ| RedisStreams
```

### Component Breakdown

| Component | Responsibility | Technology |
| :--- | :--- | :--- |
| **API Server** (`cmd/api`) | Client HTTP interface, JWT authentication, multipart upload handling, durable DB writes, job queuing. | Go 1.26+, Gin, GORM |
| **Worker Subsystem** (`cmd/worker`) | Headless consumer, Redis stream dequeuing, FFmpeg execution, artifact upload, DB state sync. | Go 1.26+, FFmpeg CLI, GORM |
| **Primary Database** | System of record for users, projects, media records, job states, and rendition URLs. | PostgreSQL 16+ |
| **Distributed Queue** | Ephemeral work-delivery substrate distributing job IDs to competing worker consumers. | Redis 7+ Streams |
| **Object Storage** | Durable storage for source uploads and processed web renditions. | Cloudinary API |

---

## 3. Networking, Server Configuration & CORS

### Base URL & Port
- **Default Local Base URL**: `http://localhost:8080`
- **API Endpoint Prefix**: `/api/v1`
- **Production Base URL**: `https://yellowbird.amaanworks.me`

### CORS Policy Configuration
The backend enforces explicit CORS origin validation. The browser frontend **must** run on one of the following allowed origins:

| Origin Allowed | Description |
| :--- | :--- |
| `https://yellowbird.amaanworks.me` | Production Frontend Domain |
| `http://localhost:3000` / `https://localhost:3000` | Local Next.js / React dev server |
| `http://localhost:3001` / `https://localhost:3001` | Alternate dev server port |
| `http://localhost:5173` / `https://localhost:5173` | Local Vite / React / Vue dev server |
| `http://127.0.0.1:3000` / `https://127.0.0.1:3000` | Loopback IP Next.js |
| `http://127.0.0.1:5173` / `https://127.0.0.1:5173` | Loopback IP Vite |
| `http://localhost` / `https://localhost` | Standard local HTTP/HTTPS |
| `http://127.0.0.1` / `https://127.0.0.1` | Standard loopback HTTP/HTTPS |

#### CORS Headers Returned
- `Access-Control-Allow-Origin: <request-origin>`
- `Access-Control-Allow-Credentials: true`
- `Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Origin, Content-Type, Authorization, Accept, X-Requested-With`

> [!NOTE]
> Pre-flight `OPTIONS` requests are automatically intercepted by the CORS middleware and answered with `HTTP 204 No Content`.

### Canonical Request Tracking
The backend assigns a unique UUIDv4 `request_id` to every incoming HTTP request. This ID is logged across the request lifecycle and passed to internal spans.

---

## 4. Authentication & Authorization Protocol

YellowBird uses signed **JSON Web Tokens (JWT)** using the **HMAC-SHA256 (HS256)** algorithm.

### Token Specifications
- **Algorithm**: `HS256`
- **Token Validity**: `24 Hours` from issuance
- **Header Key**: `Authorization`
- **Header Value**: `Bearer <token>`

### JWT Payload Schema
```json
{
  "user_id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
  "exp": 1757577600,
  "iat": 1757491200
}
```

### Security & Scope Model
1. **Public Routes**: Account registration (`POST /api/v1/users/register`), login (`POST /api/v1/users/login`), and public user lookups.
2. **Protected Routes**: All operations on Projects, Media, Jobs, and Renditions require a valid JWT header.
3. **Owner Scoping**: When requesting or modifying projects (`/api/v1/projects`), the backend automatically scopes database queries to the `owner_id` embedded in the token. Users cannot view or edit projects belonging to other accounts.

### Authentication Error Responses
If the header is missing, malformed, or the token is expired/invalid, the API aborts the request with `HTTP 401 Unauthorized`:

```json
// Missing header
{ "error": "missing auth header" }

// Malformed header (not 'Bearer <token>')
{ "error": "invalid auth header" }

// Signature invalid or expired
{ "error": "invalid token" }
```

---

## 5. Data Models, Enums & Database Contracts

### Entity Relationship Diagram

```mermaid
erDiagram
    USERS ||--o{ PROJECTS : owns
    PROJECTS ||--o{ MEDIA : contains
    MEDIA ||--o{ JOBS : triggers
    MEDIA ||--o{ RENDITIONS : produces
    JOBS ||--o| RENDITIONS : generates

    USERS {
        uuid id PK
        string name
        string email UK
        string password_hash
        timestamp created_at
        timestamp updated_at
    }

    PROJECTS {
        uuid id PK
        uuid owner_id FK
        string name
        string description
        timestamp created_at
        timestamp updated_at
    }

    MEDIA {
        uuid id PK
        uuid project_id FK
        string original_file_name
        string storage_key UK
        string mime_type
        int64 size
        string status
        float duration_seconds
        int width
        int height
        timestamp created_at
        timestamp updated_at
    }

    JOBS {
        uuid id PK
        uuid media_id FK
        string type
        int target_height
        string status
        int progress
        string error
        timestamp started_at
        timestamp completed_at
        timestamp created_at
        timestamp updated_at
    }

    RENDITIONS {
        uuid id PK
        uuid media_id FK
        string type
        string storage_key UK
        string url
        string mime_type
        int64 size
        int width
        int height
        float duration_seconds
        timestamp created_at
        timestamp updated_at
    }
```

### Domain Enums

#### `MediaStatus`
Represents the overarching lifecycle of an uploaded media asset:
- `"pending"`: Initial transient state before upload completion.
- `"uploading"`: Stream actively being sent to storage.
- `"uploaded"`: Raw source file safely stored in Cloudinary and DB record committed; child processing jobs enqueued.
- `"processing"`: One or more child processing jobs have transitioned to `running` or `queued`.
- `"ready"`: **All** child jobs (`thumbnail`, `preview`, and all `transcode` variants) have reached `completed`. All renditions are live.
- `"failed"`: One or more child jobs suffered terminal failure.

#### `JobStatus`
Represents the state of an individual worker task:
- `"queued"`: Enqueued in PostgreSQL and Redis Stream; waiting for an available worker.
- `"running"`: A worker has claimed the job and is currently running FFmpeg or uploading the result.
- `"completed"`: FFmpeg processing and rendition upload succeeded.
- `"failed"`: Processing failed or retry limits exceeded.

#### `JobType` & `RenditionType`
- `"thumbnail"`: Single-frame extraction from the video/image (JPEG).
- `"preview"`: 10-second compressed video preview clip (H.264 / AAC MP4).
- `"transcode"`: Full-length video scaled to target resolution (360p, 720p, or 1080p).

---

## 6. Complete REST API Specification

### Route Catalog Summary

| Method | Endpoint | Auth | Request Type | Description |
| :--- | :--- | :--- | :--- | :--- |
| `GET` | `/health` | Public | None | Server liveness check |
| `POST` | `/api/v1/users/register` | Public | JSON | Create a new user account |
| `POST` | `/api/v1/users/login` | Public | JSON | Authenticate & obtain JWT |
| `GET` | `/api/v1/users` | Public | None | List all user accounts |
| `GET` | `/api/v1/users/:id` | Public | None | Get user profile by ID |
| `DELETE`| `/api/v1/users/:id` | Public | None | Delete user account |
| `POST` | `/api/v1/projects` | **Protected** | JSON | Create a new project |
| `GET` | `/api/v1/projects` | **Protected** | None | List caller's projects |
| `GET` | `/api/v1/projects/:id` | **Protected** | None | Get project by ID |
| `PUT` | `/api/v1/projects/:id` | **Protected** | JSON | Update project details |
| `DELETE`| `/api/v1/projects/:id` | **Protected** | None | Delete project |
| `POST` | `/api/v1/media` | **Protected** | `multipart/form-data` | Upload media & trigger pipeline |
| `GET` | `/api/v1/media` | **Protected** | Query `?project_id=` | List media by project |
| `GET` | `/api/v1/media/:id` | **Protected** | None | Get media metadata & status |
| `PUT` | `/api/v1/media/:id` | **Protected** | JSON | Update media status |
| `DELETE`| `/api/v1/media/:id` | **Protected** | None | Delete media record |
| `POST` | `/api/v1/jobs` | **Protected** | JSON | Manually trigger a job |
| `GET` | `/api/v1/jobs` | **Protected** | Query `?media_id=` | List all jobs for media |
| `GET` | `/api/v1/jobs/:id` | **Protected** | None | Get job status & error |
| `DELETE`| `/api/v1/jobs/:id` | **Protected** | None | Delete job record |
| `POST` | `/api/v1/renditions` | **Protected** | JSON | Record a new rendition |
| `GET` | `/api/v1/renditions` | **Protected** | Query `?media_id=` | List renditions for media |
| `GET` | `/api/v1/renditions/:id`| **Protected** | None | Get rendition details |
| `DELETE`| `/api/v1/renditions/:id`| **Protected** | None | Delete rendition record |

---

### 6.1 Health & Ops

#### `GET /health`
Liveness probe endpoint.

- **Auth**: Public
- **Response**: `200 OK`
```json
{
  "status": "ok (also my first gin response)"
}
```

---

### 6.2 Users & Authentication

#### `POST /api/v1/users/register`
Creates a new user account and hashes the password using bcrypt.

- **Auth**: Public
- **Request Body** (`application/json`):
  | Field | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `name` | `string` | `required, min=2, max=100` | Full name of the user |
  | `email` | `string` | `required, email` | Unique email address |
  | `password` | `string` | `required, min=8, max=72` | Raw password string |

```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "password": "SuperSecretPassword123"
}
```

- **Success Response**: `201 Created`
```json
{
  "id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2026-09-10T10:00:00Z",
  "updated_at": "2026-09-10T10:00:00Z"
}
```

- **Error Responses**:
  - `400 Bad Request`: `{"error": "Key: 'RegisterUserRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag"}`
  - `400 Bad Request`: `{"error": "email already exists"}`

---

#### `POST /api/v1/users/login`
Authenticates user credentials and returns a signed JWT token valid for 24 hours.

- **Auth**: Public
- **Request Body** (`application/json`):
  | Field | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `email` | `string` | `required, email` | Account email |
  | `password` | `string` | `required` | Plaintext password |

```json
{
  "email": "jane@example.com",
  "password": "SuperSecretPassword123"
}
```

- **Success Response**: `200 OK`
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiODRjOGE4MTYtMWZhNS00MDc0LTk1ZWMtY2I5YzIwYTEwYzc5IiwiZXhwIjoxNzU3NTc3NjAwLCJpYXQiOjE3NTc0OTEyMDB9.h4x...",
  "user": {
    "id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
    "name": "Jane Doe",
    "email": "jane@example.com",
    "created_at": "2026-09-10T10:00:00Z",
    "updated_at": "2026-09-10T10:00:00Z"
  }
}
```

- **Error Responses**:
  - `400 Bad Request`: Validation failure.
  - `401 Unauthorized`: `{"error": "invalid email or password"}`

---

#### `GET /api/v1/users`
Lists all registered users in the database.

- **Auth**: Public
- **Success Response**: `200 OK`
```json
[
  {
    "id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
    "name": "Jane Doe",
    "email": "jane@example.com",
    "created_at": "2026-09-10T10:00:00Z",
    "updated_at": "2026-09-10T10:00:00Z"
  }
]
```

---

#### `GET /api/v1/users/:id`
Retrieves a single user's public profile by UUID.

- **Auth**: Public
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `200 OK`
```json
{
  "id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "created_at": "2026-09-10T10:00:00Z",
  "updated_at": "2026-09-10T10:00:00Z"
}
```
- **Error Responses**:
  - `400 Bad Request`: `{"error": "invalid user id"}`
  - `404 Not Found`: `{"error": "record not found"}`

---

#### `DELETE /api/v1/users/:id`
Deletes a user account.

- **Auth**: Public
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `204 No Content`
- **Error Responses**:
  - `400 Bad Request`: `{"error": "invalid user id"}`
  - `500 Internal Server Error`: DB deletion failure.

---

### 6.3 Projects

Projects act as parent workspaces holding media files. Every project operation is scoped to the caller's JWT `user_id`.

#### `POST /api/v1/projects`
Creates a new project owned by the authenticated caller.

- **Auth**: Protected (`Authorization: Bearer <token>`)
- **Request Body** (`application/json`):
  | Field | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `name` | `string` | `required, max=255` | Project name |
  | `description` | `string` | Optional | Multi-line text description |

```json
{
  "name": "Marketing Promo Q4",
  "description": "Raw footage and cutdowns for the autumn product launch."
}
```

- **Success Response**: `201 Created`
```json
{
  "id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
  "owner_id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
  "name": "Marketing Promo Q4",
  "description": "Raw footage and cutdowns for the autumn product launch.",
  "created_at": "2026-09-10T10:05:00Z",
  "updated_at": "2026-09-10T10:05:00Z"
}
```

---

#### `GET /api/v1/projects`
Retrieves all projects owned by the currently authenticated user.

- **Auth**: Protected
- **Success Response**: `200 OK`
```json
[
  {
    "id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
    "owner_id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
    "name": "Marketing Promo Q4",
    "description": "Raw footage and cutdowns for the autumn product launch.",
    "created_at": "2026-09-10T10:05:00Z",
    "updated_at": "2026-09-10T10:05:00Z"
  }
]
```

---

#### `GET /api/v1/projects/:id`
Retrieves a single project by UUID. Enforces owner matching.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `200 OK`
```json
{
  "id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
  "owner_id": "84c8a816-1fa5-4074-95ec-cb9c20a10c79",
  "name": "Marketing Promo Q4",
  "description": "Raw footage and cutdowns for the autumn product launch.",
  "created_at": "2026-09-10T10:05:00Z",
  "updated_at": "2026-09-10T10:05:00Z"
}
```
- **Error Responses**:
  - `400 Bad Request`: `{"error": "unable to parse id"}`
  - `404 Not Found`: `{"error": "record not found"}` (Returned if project does not exist OR caller is not the owner)

---

#### `PUT /api/v1/projects/:id`
Updates project name or description.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Request Body** (`application/json`):
```json
{
  "name": "Marketing Promo Q4 - Final Cut",
  "description": "Approved assets only."
}
```
- **Success Response**: `200 OK` (returns updated `ProjectResponse`)

---

#### `DELETE /api/v1/projects/:id`
Deletes a project owned by the user.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `204 No Content`

---

### 6.4 Media Ingestion & Assets

Media endpoints handle file uploads and lifecycle tracking.

#### `POST /api/v1/media`
**The Core Ingestion Endpoint.** Accepts a binary file stream alongside the target `project_id`, uploads the source to Cloudinary, persists a `Media` row (`status: uploaded`), and automatically fans out all processing jobs to Redis.

- **Auth**: Protected (`Authorization: Bearer <token>`)
- **Content-Type**: `multipart/form-data`
- **Form Fields**:
  | Field Name | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `project_id` | `string` (UUID) | Required | UUID of existing project owned by caller |
  | `file` | Binary File | Required | The raw video (`.mp4`, `.mov`, `.mkv`, etc.) or image (`.jpg`, `.png`) |

- **Multipart Request Example (cURL)**:
```bash
curl -X POST http://localhost:8080/api/v1/media \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -F "project_id=e305e717-3bf9-4720-9ba1-19ce9727581a" \
  -F "file=@/path/to/sample-video.mp4"
```

- **Success Response**: `201 Created`
```json
{
  "id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
  "project_id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
  "original_file_name": "sample-video.mp4",
  "storage_key": "YellowBird/0c9a2958-888d-4fcf-8472-3fdbcd603c62",
  "mime_type": "video/mp4",
  "size": 45892301,
  "status": "uploaded",
  "created_at": "2026-09-10T10:10:00Z",
  "updated_at": "2026-09-10T10:10:00Z"
}
```

> [!IMPORTANT]
> Immediately upon successful return of `POST /api/v1/media`:
> - If `mime_type` starts with `video/`: **5 background jobs** are created (`thumbnail`, `preview`, `transcode-360p`, `transcode-720p`, `transcode-1080p`).
> - If `mime_type` is an image: **2 background jobs** are created (`thumbnail`, `preview`).
> - The media status will transition to `processing` as soon as workers claim the tasks, and eventually to `ready` once all jobs finish.

- **Error Responses**:
  - `400 Bad Request`: `{"error": "invalid project id"}` or `{"error": "file is required"}`
  - `500 Internal Server Error`: `{"error": "project not found"}` or upload failure.

---

#### `GET /api/v1/media`
Lists all media assets belonging to a specified project.

- **Auth**: Protected
- **Query Parameter**: `project_id=<uuid>` (Required)
- **Request Example**: `GET /api/v1/media?project_id=e305e717-3bf9-4720-9ba1-19ce9727581a`
- **Success Response**: `200 OK`
```json
[
  {
    "id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "project_id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
    "original_file_name": "sample-video.mp4",
    "storage_key": "YellowBird/0c9a2958-888d-4fcf-8472-3fdbcd603c62",
    "mime_type": "video/mp4",
    "size": 45892301,
    "status": "ready",
    "duration_seconds": 32.5,
    "width": 1920,
    "height": 1080,
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:45Z"
  }
]
```

---

#### `GET /api/v1/media/:id`
Retrieves the real-time status and metadata for a specific media file. **Use this endpoint for status polling.**

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `200 OK`
```json
{
  "id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
  "project_id": "e305e717-3bf9-4720-9ba1-19ce9727581a",
  "original_file_name": "sample-video.mp4",
  "storage_key": "YellowBird/0c9a2958-888d-4fcf-8472-3fdbcd603c62",
  "mime_type": "video/mp4",
  "size": 45892301,
  "status": "processing",
  "created_at": "2026-09-10T10:10:00Z",
  "updated_at": "2026-09-10T10:10:15Z"
}
```

---

#### `PUT /api/v1/media/:id`
Manually updates media attributes (e.g. override status).

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Request Body** (`application/json`):
```json
{
  "status": "ready"
}
```
- **Success Response**: `200 OK`

---

#### `DELETE /api/v1/media/:id`
Deletes a media record by UUID.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `204 No Content`

---

### 6.5 Processing Jobs

Endpoints to inspect worker task progress or trigger custom transformations.

#### `POST /api/v1/jobs`
Manually triggers a processing job for a media record.

- **Auth**: Protected
- **Request Body** (`application/json`):
  | Field | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `media_id` | `string` (UUID) | Required | Parent media ID |
  | `type` | `string` | `required` | `"thumbnail"`, `"preview"`, or `"transcode"` |
  | `target_height` | `integer` | Required if `type == "transcode"` | Resolution height: `360`, `720`, or `1080` |

```json
{
  "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
  "type": "transcode",
  "target_height": 720
}
```

- **Success Response**: `201 Created`
```json
{
  "id": "5f13a078-d421-4d15-8409-f64f434ee494",
  "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
  "type": "transcode",
  "target_height": 720,
  "status": "queued",
  "progress": 0,
  "created_at": "2026-09-10T10:11:00Z",
  "updated_at": "2026-09-10T10:11:00Z"
}
```

- **Validation Rules Enforced**:
  - `type == "transcode"`: `target_height` must be present and one of `[360, 720, 1080]`.
  - `type == "thumbnail"` or `"preview"`: `target_height` must be `null` / omitted.

---

#### `GET /api/v1/jobs`
Lists all background processing jobs associated with a media asset.

- **Auth**: Protected
- **Query Parameter**: `media_id=<uuid>` (Required)
- **Request Example**: `GET /api/v1/jobs?media_id=18c50d4f-9e32-4752-b883-8a3d463d1e66`
- **Success Response**: `200 OK`
```json
[
  {
    "id": "5f13a078-d421-4d15-8409-f64f434ee494",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "thumbnail",
    "target_height": null,
    "status": "completed",
    "progress": 100,
    "started_at": "2026-09-10T10:10:02Z",
    "completed_at": "2026-09-10T10:10:05Z",
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:05Z"
  },
  {
    "id": "6a24b189-e532-4e26-9510-a75a545ff505",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "preview",
    "target_height": null,
    "status": "completed",
    "progress": 100,
    "started_at": "2026-09-10T10:10:05Z",
    "completed_at": "2026-09-10T10:10:12Z",
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:12Z"
  },
  {
    "id": "7b35c290-f643-4f37-0621-b86b656aa616",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "target_height": 360,
    "status": "completed",
    "progress": 100,
    "started_at": "2026-09-10T10:10:12Z",
    "completed_at": "2026-09-10T10:10:20Z",
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:20Z"
  },
  {
    "id": "8c46d301-a754-4a48-1732-c97c767bb727",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "target_height": 720,
    "status": "completed",
    "progress": 100,
    "started_at": "2026-09-10T10:10:20Z",
    "completed_at": "2026-09-10T10:10:32Z",
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:32Z"
  },
  {
    "id": "9d57e412-b865-4b59-2843-da8d878cc838",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "target_height": 1080,
    "status": "completed",
    "progress": 100,
    "started_at": "2026-09-10T10:10:32Z",
    "completed_at": "2026-09-10T10:10:48Z",
    "created_at": "2026-09-10T10:10:00Z",
    "updated_at": "2026-09-10T10:10:48Z"
  }
]
```

---

#### `GET /api/v1/jobs/:id`
Retrieves detailed state, execution timestamps, and error traces for a specific job.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `200 OK`
```json
{
  "id": "9d57e412-b865-4b59-2843-da8d878cc838",
  "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
  "type": "transcode",
  "target_height": 1080,
  "status": "failed",
  "progress": 45,
  "error": "ffmpeg transcode failed: exit status 1. output: Error while opening decoder",
  "started_at": "2026-09-10T10:10:32Z",
  "completed_at": "2026-09-10T10:10:35Z",
  "created_at": "2026-09-10T10:10:00Z",
  "updated_at": "2026-09-10T10:10:35Z"
}
```

---

#### `DELETE /api/v1/jobs/:id`
Deletes a job record.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `204 No Content`

---

### 6.6 Output Renditions

Renditions represent processed web-playable artifacts produced by workers.

#### `GET /api/v1/renditions`
Retrieves all completed output renditions for a media asset. **This is where the frontend fetches streamable URLs and posters.**

- **Auth**: Protected
- **Query Parameter**: `media_id=<uuid>` (Required)
- **Request Example**: `GET /api/v1/renditions?media_id=18c50d4f-9e32-4752-b883-8a3d463d1e66`
- **Success Response**: `200 OK`
```json
[
  {
    "id": "a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "thumbnail",
    "storage_key": "YellowBird/d41d8cd98f00b204e9800998ecf8427e",
    "url": "https://res.cloudinary.com/your-cloud/image/upload/v1234567890/YellowBird/d41d8cd98f00b204e9800998ecf8427e.jpg",
    "mime_type": "image/jpeg",
    "size": 145020,
    "width": null,
    "height": null,
    "duration_seconds": null,
    "created_at": "2026-09-10T10:10:05Z",
    "updated_at": "2026-09-10T10:10:05Z"
  },
  {
    "id": "b2c3d4e5-f6a7-4b6c-9d0e-1f2a3b4c5d6e",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "preview",
    "storage_key": "YellowBird/preview-0c9a2958-888d-4fcf-8472-3fdbcd603c62",
    "url": "https://res.cloudinary.com/your-cloud/video/upload/v1234567890/YellowBird/preview-0c9a2958-888d-4fcf-8472-3fdbcd603c62.mp4",
    "mime_type": "video/mp4",
    "size": 2450800,
    "width": null,
    "height": null,
    "duration_seconds": 10.0,
    "created_at": "2026-09-10T10:10:12Z",
    "updated_at": "2026-09-10T10:10:12Z"
  },
  {
    "id": "c3d4e5f6-a7b8-4c7d-0e1f-2a3b4c5d6e7f",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "storage_key": "YellowBird/transcode-360p-0c9a2958",
    "url": "https://res.cloudinary.com/your-cloud/video/upload/v1234567890/YellowBird/transcode-360p-0c9a2958.mp4",
    "mime_type": "video/mp4",
    "size": 6500400,
    "width": null,
    "height": 360,
    "duration_seconds": 32.5,
    "created_at": "2026-09-10T10:10:20Z",
    "updated_at": "2026-09-10T10:10:20Z"
  },
  {
    "id": "d4e5f6a7-b8c9-4d8e-1f2a-3b4c5d6e7f8a",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "storage_key": "YellowBird/transcode-720p-0c9a2958",
    "url": "https://res.cloudinary.com/your-cloud/video/upload/v1234567890/YellowBird/transcode-720p-0c9a2958.mp4",
    "mime_type": "video/mp4",
    "size": 14200800,
    "width": null,
    "height": 720,
    "duration_seconds": 32.5,
    "created_at": "2026-09-10T10:10:32Z",
    "updated_at": "2026-09-10T10:10:32Z"
  },
  {
    "id": "e5f6a7b8-c9d0-4e9f-2a3b-4c5d6e7f8a9b",
    "media_id": "18c50d4f-9e32-4752-b883-8a3d463d1e66",
    "type": "transcode",
    "storage_key": "YellowBird/transcode-1080p-0c9a2958",
    "url": "https://res.cloudinary.com/your-cloud/video/upload/v1234567890/YellowBird/transcode-1080p-0c9a2958.mp4",
    "mime_type": "video/mp4",
    "size": 28400500,
    "width": null,
    "height": 1080,
    "duration_seconds": 32.5,
    "created_at": "2026-09-10T10:10:48Z",
    "updated_at": "2026-09-10T10:10:48Z"
  }
]
```

---

#### `GET /api/v1/renditions/:id`
Retrieves a specific rendition record by UUID.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `200 OK`

---

#### `POST /api/v1/renditions`
Records a completed rendition metadata entry directly.

- **Auth**: Protected
- **Request Body** (`application/json`):
  | Field | Type | Validation | Description |
  | :--- | :--- | :--- | :--- |
  | `media_id` | `string` (UUID) | Required | Parent media ID |
  | `type` | `string` | Required | `"thumbnail"`, `"preview"`, or `"transcode"` |
  | `storage_key` | `string` | Required | Cloudinary public ID |
  | `url` | `string` | Required | Web-accessible HTTPS URL |
  | `mime_type` | `string` | Required | MIME type string |
  | `size` | `int64` | `gte=0` | Size in bytes |
  | `width` | `integer` | Optional | Video/image width |
  | `height` | `integer` | Optional | Video/image height |
  | `duration_seconds` | `float` | Optional | Video playback duration |

- **Success Response**: `201 Created`

---

#### `DELETE /api/v1/renditions/:id`
Deletes a rendition record.

- **Auth**: Protected
- **Path Parameter**: `:id` (UUID)
- **Success Response**: `204 No Content`

---

## 7. The Asynchronous Processing Pipeline

### 7.1 Media Fan-Out Pipeline Mechanics

When the frontend issues `POST /api/v1/media`, the following operations occur synchronously and asynchronously:

```mermaid
sequenceDiagram
    autonumber
    actor Client as Frontend Client
    participant API as API Server (cmd/api)
    participant S3 as Cloudinary Storage
    participant DB as PostgreSQL
    participant Redis as Redis Stream (yellowbird:jobs)
    participant Worker as Worker Pool (cmd/worker)

    Client->>API: POST /api/v1/media (Multipart: project_id + file)
    API->>DB: Validate project ownership (owner_id == caller)
    API->>S3: Upload raw file stream
    S3-->>API: UploadResult {StorageKey, SecureURL, MimeType, Size}
    API->>DB: INSERT INTO media (status = 'uploaded')
    
    rect rgb(240, 248, 255)
        note right of API: Asynchronous Fan-Out Trigger
        API->>DB: INSERT INTO jobs (thumbnail, queued)
        API->>Redis: XADD yellowbird:jobs (thumbnail_job_id)
        API->>DB: INSERT INTO jobs (preview, queued)
        API->>Redis: XADD yellowbird:jobs (preview_job_id)
        API->>DB: INSERT INTO jobs (transcode 360p/720p/1080p, queued)
        API->>Redis: XADD yellowbird:jobs (transcode_job_ids...)
    end

    API-->>Client: 201 Created (MediaResponse, status = 'uploaded')

    loop Concurrent Worker Execution
        Worker->>Redis: XREADGROUP yellowbird-workers (count=1, block=5s)
        Redis-->>Worker: Message {ID: msg-1, Values: {job_id: uuid}}
        Worker->>DB: UPDATE jobs SET status='running', started_at=now()
        Worker->>DB: SyncStatus(media_id) -> Media status = 'processing'
        Worker->>S3: Download source file stream
        Worker->>Worker: Execute FFmpeg pipeline
        Worker->>S3: Upload generated rendition artifact
        Worker->>DB: INSERT INTO renditions (media_id, type, url, height...)
        Worker->>DB: UPDATE jobs SET status='completed', progress=100
        Worker->>DB: SyncStatus(media_id) -> SELECT FOR UPDATE (check sibling jobs)
        Worker->>Redis: XACK yellowbird:jobs yellowbird-workers msg-1
    end

    Note over DB,Client: When ALL sibling jobs complete -> Media status becomes 'ready'
```

---

### 7.2 FFmpeg Command Specifications

Workers execute localized, hardened FFmpeg child processes configured specifically for high web compatibility:

| Processor | Job Type | Target | FFmpeg Execution Arguments | Output Format |
| :--- | :--- | :--- | :--- | :--- |
| **Thumbnail** | `thumbnail` | Single Frame | `ffmpeg -y -i <source> -frames:v 1 -q:v 2 <output.jpg>` | High-quality JPEG poster |
| **Preview** | `preview` | 10s Clip | `ffmpeg -y -i <source> -t 10 -c:v libx264 -preset fast -crf 28 -c:a aac -movflags +faststart <output.mp4>` | 10-second compressed web MP4 |
| **Transcode 360p** | `transcode` | 360p | `ffmpeg -y -i <source> -vf scale=-2:360 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart <output.mp4>` | 360p H.264 / AAC MP4 |
| **Transcode 720p** | `transcode` | 720p | `ffmpeg -y -i <source> -vf scale=-2:720 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart <output.mp4>` | 720p H.264 / AAC MP4 |
| **Transcode 1080p**| `transcode` | 1080p | `ffmpeg -y -i <source> -vf scale=-2:1080 -c:v libx264 -preset fast -crf 23 -c:a aac -b:a 128k -movflags +faststart <output.mp4>` | 1080p H.264 / AAC MP4 |

> [!NOTE]
> `-movflags +faststart` relocates the MP4 **moov atom** (metadata header) to the beginning of the file. This allows client web browsers to start progressive playback immediately before the entire video finishes downloading.

---

### 7.3 Lifecycle State Machine & Synchronization

Media status transitions are strictly governed by `mediaRepository.SyncStatus()`:

```mermaid
stateDiagram-v2
    [*] --> uploaded: API Upload Complete
    uploaded --> processing: Worker Claims 1st Job (StartJob)
    processing --> processing: Sibling Jobs Progressing
    processing --> ready: ALL Sibling Jobs Completed
    processing --> failed: Any Job DLQ / Terminal Failure
    ready --> [*]
    failed --> [*]
```

#### Concurrency Guarantee via Row-Level Locking
When multiple workers finish sibling jobs concurrently (e.g. thumbnail and 360p transcode completing simultaneously), both invoke `SyncStatus(ctx, mediaID)`.
`SyncStatus` initiates a PostgreSQL database transaction with:
```sql
SELECT * FROM media WHERE id = $1 FOR UPDATE;
```
This acquires an exclusive row lock on the media record. Sibling workers queue up behind this lock, evaluate the current status of all sibling jobs atomically, and safely transition `uploaded` $\rightarrow$ `processing` $\rightarrow$ `ready` without lost updates or race conditions.

---

### 7.4 Worker Failure Recovery & Dead-Letter Queue (DLQ)

YellowBird implements at-least-once processing guarantees:

1. **Pending Entries List (PEL)**: When a worker dequeues a message via `XREADGROUP`, the message enters the consumer group's PEL. It remains pending until the worker invokes `XACK`.
2. **Worker Crash Recovery Loop**: Every 30 seconds, surviving workers run `recoverPending()`. Any job message that has remained in the PEL without an ACK for $> 5\text{ minutes}$ is reclaimed via `XCLAIM`.
3. **Dead-Letter Quarantine**:
   - If a job fails or is retried 3 times (`deliveryCount >= 3`), the worker:
     - Sets the Job status in PostgreSQL to `failed` (`error: "maximum retry limits reached"`).
     - Triggers `SyncStatus(media_id)`, transitioning parent Media status to `failed`.
     - Writes the failure payload into the Redis dead-letter stream `yellowbird:jobs:dlq`.
     - Calls `XACK` on the original stream message to clear the main work queue.

---

## 8. Frontend Integration Blueprints (TypeScript & JavaScript)

### 8.1 Complete TypeScript Data Contracts & Interfaces

Drop these complete definitions directly into your frontend repository (e.g. `src/types/yellowbird.ts`):

```typescript
// ==========================================
// YellowBird Backend Data Contracts
// ==========================================

export type UUID = string;

// --- Enums ---
export type MediaStatus =
  | 'pending'
  | 'uploading'
  | 'uploaded'
  | 'processing'
  | 'ready'
  | 'failed';

export type JobStatus =
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed';

export type JobType =
  | 'thumbnail'
  | 'preview'
  | 'transcode';

export type RenditionType =
  | 'thumbnail'
  | 'preview'
  | 'transcode';

// --- User Domain ---
export interface User {
  id: UUID;
  name: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface RegisterUserRequest {
  name: string;
  email: string;
  password: string;
}

export interface LoginUserRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// --- Project Domain ---
export interface Project {
  id: UUID;
  owner_id: UUID;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface CreateProjectRequest {
  name: string;
  description?: string;
}

export interface UpdateProjectRequest {
  name?: string;
  description?: string;
}

// --- Media Domain ---
export interface Media {
  id: UUID;
  project_id: UUID;
  original_file_name: string;
  storage_key: string;
  mime_type: string;
  size: number;
  status: MediaStatus;
  duration_seconds?: number;
  width?: number;
  height?: number;
  created_at: string;
  updated_at: string;
}

export interface UpdateMediaRequest {
  status: MediaStatus;
}

// --- Job Domain ---
export interface Job {
  id: UUID;
  media_id: UUID;
  type: JobType;
  target_height?: 360 | 720 | 1080 | null;
  status: JobStatus;
  progress: number;
  error?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateJobRequest {
  media_id: UUID;
  type: JobType;
  target_height?: 360 | 720 | 1080;
}

// --- Rendition Domain ---
export interface Rendition {
  id: UUID;
  media_id: UUID;
  type: RenditionType;
  storage_key: string;
  url: string;
  mime_type: string;
  size: number;
  width?: number;
  height?: number;
  duration_seconds?: number;
  created_at: string;
  updated_at: string;
}

// --- Generic API Error Schema ---
export interface ApiError {
  error: string;
}
```

---

### 8.2 API Client Implementation with Interceptors

A production-ready Axios/Fetch client wrapper supporting automatic JWT header injection and standardized error extraction (`src/lib/api.ts`):

```typescript
import axios, { AxiosInstance, AxiosError } from 'axios';
import {
  User,
  LoginResponse,
  LoginUserRequest,
  RegisterUserRequest,
  Project,
  CreateProjectRequest,
  Media,
  Job,
  Rendition,
  ApiError
} from '../types/yellowbird';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1';

class YellowBirdClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request Interceptor: Attach JWT Bearer Token
    this.client.interceptors.request.use((config) => {
      if (typeof window !== 'undefined') {
        const token = localStorage.getItem('yb_auth_token');
        if (token && config.headers) {
          config.headers.Authorization = `Bearer ${token}`;
        }
      }
      return config;
    });

    // Response Interceptor: Format error messages
    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError<ApiError>) => {
        const message = error.response?.data?.error || error.message || 'Unknown network error';
        return Promise.reject(new Error(message));
      }
    );
  }

  // --- Auth ---
  public async register(payload: RegisterUserRequest): Promise<User> {
    const { data } = await this.client.post<User>('/users/register', payload);
    return data;
  }

  public async login(payload: LoginUserRequest): Promise<LoginResponse> {
    const { data } = await this.client.post<LoginResponse>('/users/login', payload);
    if (typeof window !== 'undefined') {
      localStorage.setItem('yb_auth_token', data.token);
    }
    return data;
  }

  public logout(): void {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('yb_auth_token');
    }
  }

  // --- Projects ---
  public async getProjects(): Promise<Project[]> {
    const { data } = await this.client.get<Project[]>('/projects');
    return data;
  }

  public async getProject(id: string): Promise<Project> {
    const { data } = await this.client.get<Project>(`/projects/${id}`);
    return data;
  }

  public async createProject(payload: CreateProjectRequest): Promise<Project> {
    const { data } = await this.client.post<Project>('/projects', payload);
    return data;
  }

  public async deleteProject(id: string): Promise<void> {
    await this.client.delete(`/projects/${id}`);
  }

  // --- Media ---
  public async getMediaByProject(projectId: string): Promise<Media[]> {
    const { data } = await this.client.get<Media[]>('/media', {
      params: { project_id: projectId },
    });
    return data;
  }

  public async getMedia(id: string): Promise<Media> {
    const { data } = await this.client.get<Media>(`/media/${id}`);
    return data;
  }

  public async deleteMedia(id: string): Promise<void> {
    await this.client.delete(`/media/${id}`);
  }

  // --- Jobs ---
  public async getJobsByMedia(mediaId: string): Promise<Job[]> {
    const { data } = await this.client.get<Job[]>('/jobs', {
      params: { media_id: mediaId },
    });
    return data;
  }

  public async getJob(id: string): Promise<Job> {
    const { data } = await this.client.get<Job>(`/jobs/${id}`);
    return data;
  }

  // --- Renditions ---
  public async getRenditionsByMedia(mediaId: string): Promise<Rendition[]> {
    const { data } = await this.client.get<Rendition[]>('/renditions', {
      params: { media_id: mediaId },
    });
    return data;
  }
}

export const api = new YellowBirdClient();
```

---

### 8.3 Multipart File Upload with Progress Tracking

Uploading media requires sending `multipart/form-data` with `project_id` and `file`:

```typescript
import axios from 'axios';
import { Media } from '../types/yellowbird';

/**
 * Uploads a raw media file to YellowBird with granular browser upload progress.
 */
export async function uploadMediaAsset(
  projectId: string,
  file: File,
  onProgress?: (percentCompleted: number) => void
): Promise<Media> {
  const token = localStorage.getItem('yb_auth_token');
  if (!token) {
    throw new Error('User is not authenticated');
  }

  const formData = new FormData();
  formData.append('project_id', projectId);
  formData.append('file', file);

  const response = await axios.post<Media>('http://localhost:8080/api/v1/media', formData, {
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'multipart/form-data',
    },
    onUploadProgress: (progressEvent) => {
      if (progressEvent.total && onProgress) {
        const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
        onProgress(percent);
      }
    },
  });

  return response.data;
}
```

---

### 8.4 Media Polling & Processing State Handler

Because video processing occurs asynchronously in the background, the frontend must monitor media status until completion:

```typescript
import { api } from './api';
import { Media, Rendition, Job } from '../types/yellowbird';

export interface PollOptions {
  intervalMs?: number;
  timeoutMs?: number;
  onJobUpdate?: (jobs: Job[]) => void;
}

/**
 * Polls the media status and its child jobs until 'ready' or 'failed'.
 */
export async function pollMediaUntilReady(
  mediaId: string,
  options: PollOptions = {}
): Promise<{ media: Media; renditions: Rendition[] }> {
  const { intervalMs = 2000, timeoutMs = 180000, onJobUpdate } = options;
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    // 1. Fetch current media metadata
    const media = await api.getMedia(mediaId);

    // 2. Fetch granular child jobs (for progress bars)
    if (onJobUpdate) {
      const jobs = await api.getJobsByMedia(mediaId);
      onJobUpdate(jobs);
    }

    // 3. Evaluate terminal state
    if (media.status === 'ready') {
      const renditions = await api.getRenditionsByMedia(mediaId);
      return { media, renditions };
    }

    if (media.status === 'failed') {
      throw new Error(`Media processing failed for media asset ${mediaId}`);
    }

    // Wait before next poll tick
    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }

  throw new Error(`Polling timed out after ${timeoutMs / 1000}s`);
}
```

---

### 8.5 Video Player Quality Switcher & Asset Resolution

Utility helper to structure fetched renditions into clean player sources:

```typescript
import { Rendition } from '../types/yellowbird';

export interface ResolvedMediaPlayback {
  thumbnailUrl: string | null;
  previewUrl: string | null;
  resolutions: {
    quality: '360p' | '720p' | '1080p';
    height: number;
    url: string;
    mimeType: string;
    sizeBytes: number;
  }[];
}

/**
 * Maps raw backend renditions into a structured video player manifest.
 */
export function resolvePlaybackSources(renditions: Rendition[]): ResolvedMediaPlayback {
  let thumbnailUrl: string | null = null;
  let previewUrl: string | null = null;
  const resolutions: ResolvedMediaPlayback['resolutions'] = [];

  for (const r of renditions) {
    if (r.type === 'thumbnail' && !thumbnailUrl) {
      thumbnailUrl = r.url;
    } else if (r.type === 'preview' && !previewUrl) {
      previewUrl = r.url;
    } else if (r.type === 'transcode' && r.height) {
      resolutions.push({
        quality: `${r.height}p` as '360p' | '720p' | '1080p',
        height: r.height,
        url: r.url,
        mimeType: r.mime_type,
        sizeBytes: r.size,
      });
    }
  }

  // Sort resolutions ascending (360p -> 720p -> 1080p)
  resolutions.sort((a, b) => a.height - b.height);

  return {
    thumbnailUrl,
    previewUrl,
    resolutions,
  };
}
```

#### React Custom Video Player Example
```tsx
import React, { useState } from 'react';
import { ResolvedMediaPlayback } from '../lib/media';

interface VideoPlayerProps {
  playback: ResolvedMediaPlayback;
}

export const VideoPlayer: React.FC<VideoPlayerProps> = ({ playback }) => {
  const [selectedQuality, setSelectedQuality] = useState<number>(
    playback.resolutions[playback.resolutions.length - 1]?.height || 720
  );

  const activeSource = playback.resolutions.find((r) => r.height === selectedQuality)
    || playback.resolutions[0];

  if (!activeSource) {
    return <div className="p-4 text-gray-500">No streamable renditions available.</div>;
  }

  return (
    <div className="relative flex flex-col items-center w-full max-w-4xl bg-black rounded-lg overflow-hidden shadow-2xl">
      <video
        key={activeSource.url}
        controls
        poster={playback.thumbnailUrl || undefined}
        className="w-full h-auto aspect-video"
      >
        <source src={activeSource.url} type={activeSource.mimeType} />
        Your browser does not support the video tag.
      </video>

      {/* Quality Picker Overlay */}
      <div className="flex items-center justify-end w-full p-2 bg-neutral-900 text-white gap-2">
        <span className="text-xs text-neutral-400">Quality:</span>
        <select
          value={selectedQuality}
          onChange={(e) => setSelectedQuality(Number(e.target.value))}
          className="bg-neutral-800 text-xs px-2 py-1 rounded border border-neutral-700 outline-none focus:ring-1 focus:ring-yellow-500"
        >
          {playback.resolutions.map((res) => (
            <option key={res.height} value={res.height}>
              {res.quality} ({Math.round(res.sizeBytes / (1024 * 1024))} MB)
            </option>
          ))}
        </select>
      </div>
    </div>
  );
};
```

---

## 9. Error Handling & Edge Cases

### Standard Error Payload
All non-2xx HTTP responses return a consistent JSON body:
```json
{
  "error": "human-readable error description"
}
```

### Common HTTP Status Codes

| Status Code | Reason | Frontend Handling Strategy |
| :--- | :--- | :--- |
| **`400 Bad Request`** | Missing required form field, invalid UUID parameter, or unsupported transcode resolution. | Display validation message to the user; do not retry without payload modification. |
| **`401 Unauthorized`** | Missing `Authorization` header, expired JWT token, or incorrect credentials. | Clear stored token (`localStorage.removeItem('yb_auth_token')`) and redirect user to `/login`. |
| **`403 Forbidden`** | Request originated from an unlisted CORS domain. | Verify frontend port matches allowed origins (`:3000`, `:5173`). |
| **`404 Not Found`** | Requested entity ID does not exist or caller is not the owner of the parent project. | Display "Asset not found" UI or redirect back to `/projects`. |
| **`500 Internal Error`** | Cloudinary connection timeout, Redis queue failure, or DB error. | Display a toast notification asking the user to retry after a brief delay. |

---

## 10. Local Development & Environment Setup

When testing the frontend against a local instance of the YellowBird backend:

### Environment Variables (`.env`)
```bash
# Server Port
PORT=8080

# PostgreSQL Connection String
DATABASE_URL=postgres://yellowbird:yellowbird@localhost:5432/yellowbird?sslmode=disable

# JWT Secret Key
JWT_SECRET=super_secret_jwt_signing_key_32_bytes_long

# Cloudinary Storage Credentials
CLOUDINARY_CLOUD_NAME=your_cloudinary_cloud_name
CLOUDINARY_API_KEY=your_api_key
CLOUDINARY_API_SECRET=your_api_secret

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=""
REDIS_DB=0
```

### Starting the Backend Services
1. **Start Postgres and Redis via Docker Compose**:
   ```bash
   docker compose -f deployments/docker-compose.yml up -d postgres redis
   ```
2. **Start the API Server**:
   ```bash
   go run ./cmd/api
   ```
3. **Start the Background Worker Pool**:
   ```bash
   go run ./cmd/worker
   ```
4. **Verify Health**:
   ```bash
   curl http://localhost:8080/health
   # Returns: {"status":"ok (also my first gin response)"}
   ```
