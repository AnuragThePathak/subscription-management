# Contributing

## Setup

```bash
git clone https://github.com/AnuragThePathak/subscription-management.git
cd subscription-management
go mod download
```

**Requirements:** Go 1.24+, MongoDB, Redis

Create `config.yaml` (see [CONFIGURATION.md](CONFIGURATION.md)) and run:

```bash
go run main.go
```

## Code Style

- Run `gofmt` before committing
- Follow existing patterns in the codebase
- Keep functions focused and small

## Project Layout

| Directory | What goes here |
|-----------|---------------|
| `internal/domain/models/` | Domain entities |
| `internal/domain/services/` | Business logic |
| `internal/domain/repositories/` | Data access interfaces + implementations |
| `internal/api/controllers/` | HTTP handlers |
| `internal/api/middlewares/` | Request middleware |
| `internal/scheduler/` | Background job logic |

## Making Changes

1. Create a feature branch
2. Make your changes
3. Test locally with `go run main.go`
4. Submit a PR with a clear description

## Adding a New Endpoint

1. Add route in the appropriate controller (`internal/api/controllers/`)
2. Add business logic in the service layer (`internal/domain/services/`)
3. Add repository methods if needed (`internal/domain/repositories/`)

## Adding a New Background Task

1. Define task type and payload in `internal/scheduler/scheduler.go`
2. Add handler in `internal/scheduler/worker.go`
3. Register handler in worker's mux

## Common Commands

```bash
go run main.go          # Run the server
```
