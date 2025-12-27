# Architecture

This document describes the internal architecture of the Subscription Management Service. It is intended for developers who want to understand how the system works, contribute to the codebase, or learn from its design patterns.

--

## How to Use This Document

This document is a technical reference, not a linear tutorial.

- New readers should start with **Service Architecture Overview** and **Design Tradeoffs**
- Contributors working on specific areas can jump directly to:
  - Repository Pattern
  - Scheduler Internals
  - Authentication Flow
- Most developers do not need to read this end-to-end

---

## Quick Reference

This section provides lookup tables for common values and types. Use this when you need to quickly check valid options.

### Subscription Status Values

| Status | Meaning | Transitions To |
|--------|---------|----------------|
| `active` | Currently valid, will auto-renew | `cancelled` (user action) |
| `cancelled` | Will not renew, but still valid until `ValidTill` | `expired` (automatic) |
| `expired` | No longer valid | (terminal state) |

### Billing Frequencies

| Frequency | ValidTill Extension | Use Case |
|-----------|--------------------|---------|
| `daily` | +1 day | Testing, trials |
| `weekly` | +7 days | Short-term subscriptions |
| `monthly` | +1 month | Standard billing |
| `yearly` | +1 year | Annual plans |

### Subscription Categories

`sports` · `news` · `entertainment` · `lifestyle` · `technology` · `finance` · `politics` · `other`

### Supported Currencies

`USD` · `EUR` · `GBP`

### Error Codes Reference

| Code | HTTP | When to Use |
|------|------|-------------|
| `VALIDATION` | 400 | Invalid input format or values |
| `UNAUTHORIZED` | 401 | Missing or invalid JWT token |
| `FORBIDDEN` | 403 | Valid token but insufficient permissions |
| `NOT_FOUND` | 404 | Resource doesn't exist |
| `CONFLICT` | 409 | Duplicate resource (e.g., email already registered) |
| `UNPROCESSABLE` | 422 | Valid syntax but business rule violation |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL` | 500 | Unexpected server error |
| `DB_ERROR` | 500 | Database operation failed |
| `TIMEOUT` | 504 | Request timeout |

### Key File Locations

| Concern | Location |
|---------|----------|
| Domain models | `internal/domain/models/` |
| Business logic | `internal/domain/services/` |
| Repository interfaces | `internal/domain/repositories/` |
| HTTP handlers | `internal/api/controllers/` |
| Middleware | `internal/api/middlewares/` |
| Error types | `internal/api/shared/apperror/` |
| Configuration | `internal/api/shared/config/` |
| Background tasks | `internal/scheduler/` |
| Email templates | `internal/notifications/` |

---

## Service Architecture Overview

The system is a **loosely coupled monolith** consisting of two main runtime components that share a single process:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              main.go                                        │
│                         (dependency wiring)                                 │
│                                                                             │
│    ┌─────────────────────────────┐    ┌─────────────────────────────┐      │
│    │        API Server           │    │    Background Scheduler     │      │
│    │                             │    │                             │      │
│    │  - HTTP handlers (chi)      │    │  - Polling loop             │      │
│    │  - Authentication           │    │  - Task enqueueing          │      │
│    │  - Rate limiting            │    │                             │      │
│    │  - Request validation       │    │                             │      │
│    └──────────────┬──────────────┘    └──────────────┬──────────────┘      │
│                   │                                  │                      │
│                   ▼                                  ▼                      │
│    ┌─────────────────────────────────────────────────────────────────┐     │
│    │                        Domain Services                          │     │
│    │                                                                  │     │
│    │  - SubscriptionService    - AuthService    - UserService        │     │
│    │  - JWTService             - RateLimiterService                  │     │
│    └──────────────────────────────┬──────────────────────────────────┘     │
│                                   │                                        │
│                                   ▼                                        │
│    ┌─────────────────────────────────────────────────────────────────┐     │
│    │                    Repository Interfaces                        │     │
│    │                                                                  │     │
│    │  UserRepository    SubscriptionRepository    BillRepository     │     │
│    └──────────────────────────────┬──────────────────────────────────┘     │
│                                   │                                        │
│                                   ▼                                        │
│    ┌──────────────────────────────────────────────────────────────────┐    │
│    │                       Infrastructure                             │    │
│    │                                                                   │    │
│    │       MongoDB                Redis               SMTP             │    │
│    └──────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key architectural decisions:**

- Both components run as goroutines in the same process, simplifying deployment
- They share domain services but have separate entry points (HTTP vs polling loop)
- Infrastructure connections (MongoDB, Redis) are established once and shared
- Graceful shutdown coordinates all components via context cancellation

---

## Request Flow

### HTTP Request Path

```
HTTP Request
     │
     ▼
┌────────────────────┐
│   chi.Router       │
│  - Logger          │
│  - Recoverer       │
│  - Rate Limiter    │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│ Auth Middleware    │  ← Validates JWT, extracts claims
│ (protected routes) │    Stores user ID in context
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   Controller       │  ← Parses request, calls service
│   (handlers)       │    Returns JSON response
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   Domain Service   │  ← Business logic, validation
│                    │    Authorization checks
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│   Repository       │  ← MongoDB operations
│   Implementation   │    Error translation
└────────────────────┘
```

### Error Handling Through Layers

Each layer handles errors differently:

| Layer | Error Handling |
|-------|----------------|
| **Repository** | Translates MongoDB errors to `AppError` types |
| **Service** | Returns `AppError` for business rule violations |
| **Controller** | Converts `AppError` to HTTP response using status code |
| **Middleware** | Catches panics, logs errors, returns structured responses |

---

## Domain Model

### Entity Relationships

```
┌─────────────┐
│    User     │
│             │
│ - ID        │
│ - Name      │
│ - Email     │
│ - Password  │
└──────┬──────┘
       │
       │ 1:N
       ▼
┌─────────────────┐          1:N         ┌────────────┐
│  Subscription   │─────────────────────►│    Bill    │
│                 │                       │            │
│ - ID            │                       │ - ID       │
│ - Name          │                       │ - Amount   │
│ - Price         │                       │ - Currency │
│ - Currency      │                       │ - StartDate│
│ - Frequency     │                       │ - EndDate  │
│ - Category      │                       │ - Status   │
│ - Status        │                       └────────────┘
│ - ValidTill     │
│ - UserID (FK)   │
└─────────────────┘
```

### Subscription State Machine

```
                           ┌──────────────────────────────────────┐
                           │                                      │
                   user    │                                      │ scheduler
                 creates   │              ACTIVE                  │ auto-renews
            ───────────────►             ───────────────────────────►
                           │  (ValidTill in future)               │  (extends ValidTill,
                           │                                      │   creates Bill)
                           └──────────────────┬───────────────────┘
                                              │
                                              │ user cancels
                                              │ (marks cancelled,
                                              │  ValidTill unchanged)
                                              ▼
                           ┌──────────────────────────────────────┐
                           │                                      │
                           │            CANCELLED                 │
                           │                                      │
                           │  (still valid until ValidTill)       │
                           │  (no auto-renewal scheduled)         │
                           │                                      │
                           └──────────────────┬───────────────────┘
                                              │
                                              │ ValidTill passes
                                              │ (scheduler marks expired)
                                              ▼
                           ┌──────────────────────────────────────┐
                           │                                      │
                           │             EXPIRED                  │
                           │                                      │
                           │  (no longer valid)                   │
                           │                                      │
                           └──────────────────────────────────────┘
```

**Business invariants:**

1. A subscription cannot be deleted if any billing has occurred (must cancel instead)
2. Cancelled subscriptions remain usable until `ValidTill`
3. Only active subscriptions are auto-renewed
4. Refunds are only possible if the current billing period hasn't started

### Billing Frequency

The `Frequency` type determines how `ValidTill` is calculated on renewal:

| Frequency | Extension |
|-----------|-----------|
| `daily` | +1 day |
| `weekly` | +7 days |
| `monthly` | +1 month |
| `yearly` | +1 year |

---

## Repository Pattern

### Interface Definition

Repositories are defined as interfaces in the `domain/repositories` package:

```go
type SubscriptionRepository interface {
    Create(context.Context, *models.Subscription) (*models.Subscription, error)
    GetByID(context.Context, bson.ObjectID) (*models.Subscription, error)
    GetAll(context.Context) ([]*models.Subscription, error)
    GetByUserID(context.Context, bson.ObjectID) ([]*models.Subscription, error)
    GetActiveSubscriptions(context.Context) ([]*models.Subscription, error)
    GetSubscriptionsDueForReminder(context.Context, []int) ([]*models.Subscription, error)
    GetSubscriptionsDueForRenewal(context.Context, time.Time, time.Time) ([]*models.Subscription, error)
    GetCancelledExpiredSubscriptions(context.Context) ([]*models.Subscription, error)
    Update(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error)
    Delete(ctx context.Context, id bson.ObjectID) error
}
```

**Design rationale:**

- Services depend on interfaces, not implementations
- MongoDB-specific code is isolated in repository implementations
- Enables testing with in-memory fakes
- Database migration is feasible without rewriting service logic

### MongoDB Implementation Details

Each repository:

1. Creates required indexes on initialization
2. Uses the `lib` package helpers for common query patterns
3. Translates MongoDB errors to `AppError` types
4. Uses `context.Context` for timeout and cancellation

**Index strategy:**

```go
// SubscriptionRepository indexes
{Key: "user_id"}                      // Fast lookup by owner
{Key: ["status", "valid_till"]}       // Scheduler queries
```

---

## Error Handling Strategy

### AppError Structure

The `apperror` package defines structured application errors:

```go
type AppError interface {
    error
    Code() ErrorCode      // e.g., "NOT_FOUND", "VALIDATION"
    Message() string      // User-facing message
    Status() int          // HTTP status code
    Unwrap() error        // Original error for debugging
}
```

### Error Codes

| Code | HTTP Status | Usage |
|------|-------------|-------|
| `INTERNAL` | 500 | Unexpected errors |
| `UNAUTHORIZED` | 401 | Missing/invalid token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource doesn't exist |
| `CONFLICT` | 409 | Duplicate resource |
| `VALIDATION` | 400 | Invalid input |
| `UNPROCESSABLE` | 422 | Valid syntax but invalid semantics |
| `RATE_LIMITED` | 429 | Too many requests |
| `DB_ERROR` | 500 | Database failures |
| `TIMEOUT` | 504 | Request timeout |

### Error Flow

```
Repository Error          Service Error            Controller Response
────────────────          ─────────────            ───────────────────
mongo.ErrNoDocuments  →   NotFoundError       →   {"error": "...", "code": "NOT_FOUND"}
DuplicateKeyError     →   ConflictError       →   {"error": "...", "code": "CONFLICT"}
```

---

## Authentication Flow

### Token Types

The system uses two JWT token types:

| Token | Purpose | Expiry | Secret |
|-------|---------|--------|--------|
| **Access** | API authorization | Short (1h default) | `access_secret` |
| **Refresh** | Get new access tokens | Long (7d default) | `refresh_secret` |

### JWT Claims Structure

```go
type Claims struct {
    UserID string    `json:"userId"`
    Email  string    `json:"email"`
    Type   TokenType `json:"type"`    // "access" or "refresh"
    jwt.RegisteredClaims
}
```

### Authentication Middleware

The middleware:

1. Extracts `Bearer` token from `Authorization` header
2. Validates token signature and expiry using `access_secret`
3. Verifies token type is `access`
4. Stores user ID and email in request context
5. Downstream handlers access via `context.Value()`

### Token Refresh Flow

```
Client                                    Server
───────                                   ──────
    │                                         │
    │  POST /auth/refresh                     │
    │  Body: { refreshToken: "..." }          │
    │────────────────────────────────────────►│
    │                                         │
    │                          Validate refresh token
    │                          Generate new access token
    │                          (refresh token unchanged)
    │                                         │
    │◄────────────────────────────────────────│
    │  { accessToken: "...", expiresAt: "..." }
```

---

## Scheduler Internals

### Polling Loop

The scheduler runs a polling loop at configurable intervals (default: 12 hours):

```go
func (s *SubscriptionScheduler) Start(ctx context.Context) error {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    // Run immediately on startup
    s.pollSubscriptions(ctx)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            s.pollSubscriptions(ctx)
        }
    }
}
```

### Task Types

| Task | Trigger | Action |
|------|---------|--------|
| `subscription:reminder` | N days before renewal | Send reminder email |
| `subscription:renewal` | 8 hours before ValidTill | Extend ValidTill, create Bill, send confirmation |
| `subscription:expiration` | ValidTill passed (cancelled) | Mark status as `expired` |

### Task Deduplication

The scheduler uses Redis keys to prevent duplicate task processing:

```go
// Reminder dedup key (expires after 24h)
redisKey := fmt.Sprintf("reminder_sent:%s:%d", subscriptionID, daysBefore)

// Check if already sent
exists, _ := s.redisClient.Exists(ctx, redisKey).Result()
if exists == 0 {
    s.scheduleReminderTask(subscription, daysBefore)
}
```

### Asynq Task Options

Each task is enqueued with retry and timeout semantics:

```go
s.client.Enqueue(
    task,
    asynq.Unique(24*time.Hour),     // Prevent duplicate pending tasks
    asynq.Retention(24*time.Hour),  // Keep completed tasks for debugging
    asynq.Timeout(45*time.Second),  // Handler must complete in time
    asynq.MaxRetry(3),              // Retry on transient failures
)
```

### Worker Processing

The worker registers handlers for each task type:

```go
mux := asynq.NewServeMux()
mux.HandleFunc(ReminderTask, w.handleSubscriptionReminder)
mux.HandleFunc(RenewalTask, w.handleSubscriptionRenewal)
mux.HandleFunc(ExpirationTask, w.handleSubscriptionExpiration)
```

**Renewal handler logic:**

1. Parse task payload (subscription ID, renewal date)
2. Fetch current subscription state
3. Verify still active and not already renewed
4. Calculate new `ValidTill` based on frequency
5. Create billing record
6. Update subscription
7. Send confirmation email

---

## Service Layer Design

### External vs Internal Interfaces

Services expose two interfaces:

```go
// For API controllers (external callers)
type SubscriptionServiceExternal interface {
    CreateSubscription(ctx, *Subscription, claimedUserID) (*Subscription, error)
    GetSubscriptionByID(ctx, id, claimedUserID) (*Subscription, error)
    CancelSubscription(ctx, id, claimedUserID) (*Subscription, error)
    // ... API-facing operations
}

// For scheduler/worker (internal callers)
type SubscriptionServiceInternal interface {
    RenewSubscriptionInternal(ctx, id) (*Subscription, error)
    FetchSubscriptionsDueForRenewalInternal(ctx, start, end) ([]*Subscription, error)
    MarkCancelledSubscriptionAsExpiredInternal(ctx, id) error
    // ... scheduler-facing operations
}

// Combined for dependency injection
type SubscriptionService interface {
    SubscriptionServiceExternal
    SubscriptionServiceInternal
}
```

**Design rationale:**

- Clear separation between what API can do vs what scheduler can do
- `claimedUserID` parameter enforces authorization for external callers
- Internal methods skip authorization checks (trusted caller)
- Single implementation satisfies both interfaces

### Authorization Pattern

External methods include authorization:

```go
func (s *subscriptionService) GetSubscriptionByID(
    ctx context.Context,
    id string,
    claimedUserID string,
) (*models.Subscription, error) {
    subscription, err := s.subscriptionRepository.GetByID(ctx, oid)
    if err != nil {
        return nil, err
    }

    // Authorization check
    if subscription.UserID.Hex() != claimedUserID {
        return nil, apperror.NewForbiddenError("access denied")
    }

    return subscription, nil
}
```

---

## Design Tradeoffs

### Why MongoDB?

**Pros for this use case:**

- Document model fits subscription entities naturally
- Flexible schema for future extensions (custom fields, metadata)
- Good performance for time-range queries with proper indexes
- Simpler operational model for a learning project

**Tradeoffs:**

- No transactional guarantees across collections (renewal + bill creation)
- Denormalization required for some queries

### Why Asynq?

**Chosen over alternatives:**

| Alternative | Why not |
|-------------|---------|
| Database polling | No retry semantics, harder to scale workers |
| RabbitMQ | Operational overhead, overkill for this scale |
| In-process queue | Lost on restart, can't scale workers independently |

**Asynq provides:**

- Redis-backed persistence
- Automatic retries with backoff
- Task deduplication
- Monitoring via asynqmon

### Why chi Router?

**Pros:**

- Lightweight, stdlib-compatible
- Middleware chaining without magic
- URL parameter parsing
- No external dependencies beyond net/http

---

## Testing Strategy

This project is designed with testability in mind, even though a comprehensive test suite is not yet implemented.

- Domain services depend on repository interfaces, allowing business logic to be tested in isolation using mocks or fakes.
- Repository implementations are structured to allow integration testing against a real MongoDB instance.
- HTTP handlers are decoupled from business logic, enabling end-to-end testing using `net/http/httptest`.

This section documents the **intended testing approach** and serves as guidance for future contributors.

### Unit Testing Services

Services should be tested with mocked repositories:

```go
func TestCreateSubscription(t *testing.T) {
    mockRepo := &MockSubscriptionRepository{}
    mockBillRepo := &MockBillRepository{}
    
    service := services.NewSubscriptionService(mockRepo, mockBillRepo)
    
    // Test business logic without hitting database
}
```

### Repository Testing

Repository tests run against a real MongoDB instance:

```go
func TestSubscriptionRepository(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Use test container or local MongoDB
}
```

### End-to-End Testing

For full request flows:

1. Start the server with test configuration
2. Make HTTP requests using `net/http/httptest`
3. Verify database state after operations

---

## Graceful Shutdown

All components participate in graceful shutdown:

```go
apiServer.StartWithGracefulShutdown(
    ctx,
    10*time.Second,    // Shutdown timeout
    database,          // Closeable
    redis,             // Closeable
    schedulerAdapter,  // Closeable
    workerAdapter,     // Closeable
)
```

Shutdown order:

1. Stop accepting new HTTP connections
2. Wait for in-flight requests (up to timeout)
3. Stop scheduler polling loop
4. Wait for worker to finish current tasks
5. Close Redis and MongoDB connections
