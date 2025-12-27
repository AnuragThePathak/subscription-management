# Subscription Management Service

A backend service written in Go for managing recurring subscriptions—handling creation, billing cycles, automated renewals, cancellations, and user notifications.

## What This System Does

This service manages the complete lifecycle of user subscriptions: from initial creation through recurring billing cycles to eventual expiration or cancellation. It supports multiple billing frequencies (daily, weekly, monthly, yearly), handles automatic renewals, processes cancellations with proper validity period handling, and sends email notifications for key lifecycle events.

The system is designed as a learning reference for production-grade backend architecture, emphasizing:

- Clean separation between API layer, domain logic, and infrastructure
- Domain-driven design principles applied pragmatically
- Background processing patterns for time-sensitive operations
- Graceful degradation and proper lifecycle management

---

## Design Philosophy

### Loosely Coupled Monolith

This system is architected as a **loosely coupled monolith**—a single deployable unit with clear internal boundaries. This choice reflects a deliberate tradeoff:

- **Single process simplicity**: One deployment artifact, shared database connections, straightforward debugging
- **Module isolation**: The API server and background scheduler are separate concerns that could be extracted into separate services later if scaling demands it
- **Reduced operational overhead**: No service mesh, no inter-service communication, no distributed tracing complexity

The scheduler and API server share domain logic but have distinct responsibilities. They run as goroutines within the same process but interact only through well-defined service interfaces—making future extraction straightforward.

### Domain-Driven Boundaries

The codebase is organized around **bounded contexts** rather than technical layers:

```
internal/
├── api/           → HTTP transport layer (controllers, middleware, request handling)
├── domain/        → Business logic (models, services, repository interfaces)
├── adapters/      → Infrastructure wiring (database, Redis, server lifecycle)
├── scheduler/     → Background job orchestration (polling, task queue)
├── notifications/ → External integrations (email delivery)
└── lib/           → Shared utilities (time helpers, authentication)
```

**Why this separation matters:**

- `domain/` contains the core business logic, expressed through services that depend only on interfaces. While repository implementations are backed by MongoDB, the business logic itself remains isolated from persistence details and can be tested independently.
- `api/` knows how to handle HTTP but delegates all business decisions to domain services
- `adapters/` handles the messy reality of external systems (connection pooling, graceful shutdown)
- `scheduler/` is treated as a separate subsystem with its own entry points

### Repository Pattern

Domain services depend on **repository interfaces**, not concrete implementations. This:

- Enables testing with in-memory fakes
- Decouples business logic from MongoDB specifics
- Makes database migration feasible without rewriting service logic

> For implementation details, see [ARCHITECTURE.md → Repository Pattern](docs/ARCHITECTURE.md#repository-pattern)

---

## Subscription Lifecycle

A subscription moves through well-defined states with clear transition rules:

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   ┌─────────┐      auto-renew      ┌─────────┐                   │
│   │         │ ◄─────────────────── │         │                   │
│   │ ACTIVE  │                      │ ACTIVE  │ (next period)     │
│   │         │ ────────────────────►│         │                   │
│   └────┬────┘     renewal date     └─────────┘                   │
│        │                                                         │
│        │ user cancels                                            │
│        ▼                                                         │
│   ┌────────────┐     validity ends     ┌─────────┐               │
│   │ CANCELLED  │ ─────────────────────►│ EXPIRED │               │
│   └────────────┘                       └─────────┘               │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**Key behaviors:**

| Action | Behavior |
|--------|----------|
| **Create** | Subscription starts `active`, validity set based on billing frequency |
| **Auto-renew** | Scheduler renews active subscriptions before billing period ends, creates billing record, sends confirmation email |
| **Cancel** | Marks subscription `cancelled` but remains valid until current period ends—no prorated refund mid-cycle |
| **Expire** | Cancelled subscriptions transition to `expired` once validity ends |
| **Delete** | Hard delete is permitted only when billing hasn't started (otherwise, cancel instead) |

**Cancellation nuances:**

- A cancelled subscription with remaining validity continues to work until `ValidTill`
- Refunds are only processed if the current billing period hasn't started yet
- The scheduler automatically marks cancelled subscriptions as expired after their valid period ends

> For the full state machine diagram, see [ARCHITECTURE.md → Domain Model](docs/ARCHITECTURE.md#domain-model)

---

## Background Processing

### Scheduler + Worker Architecture

Time-sensitive operations (renewals, reminders, expirations) are handled by a background subsystem with two components:

```
┌────────────────────┐          ┌────────────────────────┐
│                    │  tasks   │                        │
│    Scheduler       │ ───────► │    Redis Task Queue    │
│   (polls DB on     │          │    (asynq)             │
│    configurable    │          │                        │
│    interval)       │          └───────────┬────────────┘
│                    │                      │
└────────────────────┘                      │ task payloads
                                            ▼
                              ┌──────────────────────────┐
                              │                          │
                              │    Worker Pool           │
                              │   (concurrent handlers)  │
                              │                          │
                              │  • Send reminder emails  │
                              │  • Process renewals      │
                              │  • Mark expirations      │
                              │                          │
                              └──────────────────────────┘
```

**Scheduler responsibilities:**

- Runs on a configurable interval (default: every 12 hours)
- Queries for subscriptions approaching renewal, needing reminders, or requiring expiration
- Enqueues tasks with idempotency keys to prevent duplicate processing

**Worker responsibilities:**

- Consumes tasks from Redis queue
- Handles reminder notifications (configurable days before renewal: e.g., 7, 3, 1)
- Executes automatic renewals (creates billing records, extends validity, sends confirmation)
- Marks cancelled subscriptions as expired when validity ends

**Why this design:**

- Decouples "what needs to be done" (scheduler) from "how to do it" (worker)
- Redis queue provides persistence and retry semantics via [asynq](https://github.com/hibiken/asynq)
- Multiple workers can process concurrently without coordination
- Scheduler and worker failures don't affect API availability

> For task types, deduplication, and retry semantics, see [ARCHITECTURE.md → Scheduler Internals](docs/ARCHITECTURE.md#scheduler-internals)

---

## Project Structure

```
subscription-management/
├── main.go                 # Application entry point, dependency wiring
└── internal/
    ├── adapters/           # Infrastructure adapters and lifecycle
    │   ├── database.go     # MongoDB connection wrapper
    │   ├── redis.go        # Redis client with health checks
    │   ├── server.go       # HTTP server lifecycle
    │   ├── scheduler.go    # Scheduler shutdown interface
    │   └── worker.go       # Worker shutdown interface
    │
    ├── api/                # HTTP transport layer
    │   ├── controllers/    # Route handlers (auth, users, subscriptions)
    │   ├── middlewares/    # Auth, rate limiting
    │   └── shared/         # Cross-cutting API concerns
    │       ├── apperror/   # Typed application errors
    │       ├── config/     # Configuration loading
    │       └── endpoint/   # Request/response helpers
    │
    ├── domain/             # Core business logic
    │   ├── models/         # Domain entities (User, Subscription, Bill)
    │   ├── repositories/   # Data access interfaces + MongoDB implementations
    │   └── services/       # Business operations
    │
    ├── scheduler/          # Background processing
    │   ├── scheduler.go    # Polling loop, task enqueueing
    │   └── worker.go       # Task handlers (reminders, renewals, expirations)
    │
    ├── notifications/      # External integrations
    │   ├── email_sender.go # SMTP email delivery
    │   └── email_template.go # Email templates
    │
    └── lib/                # Shared utilities
        ├── auth.go         # Authentication helpers
        ├── mongo.go        # MongoDB utilities
        └── time.go         # Time calculation helpers
```

---

## Setup

### Prerequisites

- Go 1.24+
- MongoDB
- Redis

### Quick Start

```bash
git clone https://github.com/AnuragThePathak/subscription-management.git
cd subscription-management
go mod download
```

Create `config.yaml` (see [Configuration](#configuration)) and run:

```bash
go run main.go
```

---

## Configuration

The service loads configuration from `config.yaml` or environment variables. Key sections:

| Section | Purpose |
|---------|---------|
| `server` | HTTP port, TLS settings |
| `database` | MongoDB connection URI and database name |
| `jwt` | Token signing secrets and expiration times |
| `rate_limiter` | API rate limiting with Redis backend |
| `scheduler` | Polling interval and reminder schedule |
| `queue_worker` | Worker concurrency |
| `email` | SMTP configuration for notifications |

See [CONFIGURATION.md](docs/CONFIGURATION.md) for detailed options and environment variable mappings.

---

## API Overview

The API follows RESTful conventions with JWT-based authentication.

> For JWT claims structure and token refresh flow, see [ARCHITECTURE.md → Authentication Flow](docs/ARCHITECTURE.md#authentication-flow)

### Authentication

```
POST /api/v1/auth/register    # Create account
POST /api/v1/auth/login       # Get tokens
POST /api/v1/auth/refresh     # Refresh access token
```

### Users (authenticated)

```
GET    /api/v1/users/:id      # Get user
PUT    /api/v1/users/:id      # Update user
DELETE /api/v1/users/:id      # Delete user
```

### Subscriptions (authenticated)

```
GET    /api/v1/subscriptions           # List all subscriptions
POST   /api/v1/subscriptions           # Create subscription
GET    /api/v1/subscriptions/:id       # Get subscription
GET    /api/v1/subscriptions/user/:id  # Get user's subscriptions
PUT    /api/v1/subscriptions/:id/cancel # Cancel subscription
DELETE /api/v1/subscriptions/:id       # Delete subscription
```

---

## Documentation Structure

This project uses multiple documentation files to keep concerns separated:

| File | Purpose |
|------|---------|
| [README.md](README.md) | High-level overview, architecture, quick start |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Technical reference for internals—start with [Quick Reference](docs/ARCHITECTURE.md#quick-reference) and [Design Tradeoffs](docs/ARCHITECTURE.md#design-tradeoffs) |
| [CONFIGURATION.md](docs/CONFIGURATION.md) | Configuration reference, environment variables |
| [CONTRIBUTING.md](docs/CONTRIBUTING.md) | Development setup, code style, PR process |

### ARCHITECTURE.md Quick Links

| Section | What You'll Find |
|---------|------------------|
| [Quick Reference](docs/ARCHITECTURE.md#quick-reference) | Lookup tables for status values, error codes, file locations |
| [Request Flow](docs/ARCHITECTURE.md#request-flow) | How HTTP requests traverse the system |
| [Repository Pattern](docs/ARCHITECTURE.md#repository-pattern) | Interface design and MongoDB implementation |
| [Error Handling](docs/ARCHITECTURE.md#error-handling-strategy) | AppError structure and error propagation |
| [Authentication Flow](docs/ARCHITECTURE.md#authentication-flow) | JWT tokens, middleware, refresh flow |
| [Scheduler Internals](docs/ARCHITECTURE.md#scheduler-internals) | Polling, task types, deduplication, retries |
| [Design Tradeoffs](docs/ARCHITECTURE.md#design-tradeoffs) | Why MongoDB, asynq, chi |
| [Testing Strategy](docs/ARCHITECTURE.md#testing-strategy) | Unit, integration, E2E approaches |

---

## License

MIT — see [LICENSE](LICENSE) for details.
