# Configuration

All configuration is loaded from `config.yaml` in the project root (see [`example.yaml`](../example.yaml) for the full reference). Environment variables with the `APP_` prefix can override any config value.

---

## Quick Start

```bash
cp example.yaml config.yaml
# Edit config.yaml with your actual values
```

The service will fail fast on startup if required fields are missing — see [Validation Rules](#validation-rules) below.

---

## Environment Variable Overrides

[Viper](https://github.com/spf13/viper) loads config in this priority order (highest wins):

1. Environment variables (prefixed with `APP_`)
2. `config.yaml` file
3. Built-in defaults

Environment variable naming follows Viper's convention — uppercase, underscored, with `APP_` prefix:

```bash
# Override database host
export APP_DATABASE_HOST=my-mongo-host

# Override server port
export APP_SERVER_PORT=9090

# Enable OpenTelemetry
export APP_OTEL_ENABLED=true
```

---

## Configuration Sections

### Server

| Key | Default | Description |
|---|---|---|
| `server.port` | `8080` | HTTP listen port |
| `server.request_timeout` | `10s` | Per-request context deadline |
| `server.tls.enabled` | `false` | Enable HTTPS |
| `server.tls.cert_path` | — | Path to TLS certificate (required if TLS enabled) |
| `server.tls.key_path` | — | Path to TLS private key (required if TLS enabled) |

### Database (MongoDB)

| Key | Default | Description |
|---|---|---|
| `database.host` | **required** | MongoDB hostname |
| `database.port` | `27017` | MongoDB port |
| `database.username` | **required** | MongoDB username |
| `database.password` | **required** | MongoDB password |
| `database.name` | **required** | Database name |
| `database.auth_source` | `admin` | Authentication database |

> MongoDB must run as a **replica set** for transaction support.

### Redis

| Key | Default | Description |
|---|---|---|
| `redis.host` | **required** | Redis hostname |
| `redis.port` | `6379` | Redis port |
| `redis.password` | — | Redis password (empty for no auth) |
| `redis.db` | `0` | Redis database number |

### JWT

| Key | Default | Description |
|---|---|---|
| `jwt.access_secret` | **required** | Secret for signing access tokens |
| `jwt.refresh_secret` | **required** | Secret for signing refresh tokens |
| `jwt.access_timeout` | `1` | Access token expiry in hours |
| `jwt.refresh_timeout` | `72` | Refresh token expiry in hours |
| `jwt.issuer` | **required** | Token issuer claim |

### Rate Limiter

| Key | Default | Description |
|---|---|---|
| `rate_limiter.app.rate` | **required** | Maximum requests per period |
| `rate_limiter.app.burst` | — | Maximum burst capacity |
| `rate_limiter.app.period` | `1m` | Time window for rate limiting |

### Scheduler

| Key | Default | Description |
|---|---|---|
| `scheduler.name` | **required** | Scheduler instance name (used in logs) |
| `scheduler.interval` | `12h` | Polling interval for subscription checks |
| `scheduler.reminder_days` | `[1, 3, 7]` | Days before renewal to send reminders |
| `scheduler.startup_delay` | `15m` | Delay before the first poll on startup |
| `scheduler.enabled_for_env` | `[production, staging]` | Environments where the scheduler runs |

### Queue Worker

| Key | Default | Description |
|---|---|---|
| `queue_worker.name` | — | Worker instance name (used in logs) |
| `queue_worker.concurrency` | `2` | Number of concurrent task processors |
| `queue_worker.enabled_for_env` | `[production, staging]` | Environments where the worker runs |

> To enable the scheduler or worker in development, add `"development"` to the `enabled_for_env` list.

### Asynq

| Key | Default | Description |
|---|---|---|
| `asynq.queue_name` | `subscription` | Name of the Redis-backed task queue |

### Email (SMTP)

| Key | Default | Description |
|---|---|---|
| `email.smtp_host` | **required** | SMTP server hostname |
| `email.smtp_port` | `587` | SMTP server port |
| `email.from_email` | **required** | Sender email address |
| `email.from_name` | `Subscription Management` | Display name in From header |
| `email.smtp_username` | **required** | SMTP authentication username |
| `email.smtp_password` | **required** | SMTP authentication password |
| `email.account_url` | — | Account management URL (used in email templates) |
| `email.support_url` | — | Support URL (used in email templates) |

### OpenTelemetry

| Key | Default | Description |
|---|---|---|
| `otel.enabled` | `false` | Enable/disable tracing and metrics |
| `otel.service_name` | `subscription-management` | Service name in traces and metrics |
| `otel.jaeger_endpoint` | `localhost:4317` | OTLP gRPC endpoint for Jaeger |

When `otel.enabled` is `false`, no traces are exported and a no-op metrics adapter is used. The `/metrics` Prometheus endpoint remains available regardless.

Metric names are also configurable:

```yaml
otel:
  metrics:
    subscriptions_created_count:
      name: "subscriptions_created_total"
      description: "Total number of successfully created subscriptions"
    subscriptions_canceled_count:
      name: "subscriptions_canceled_total"
      description: "Total number of canceled subscriptions"
    active_subscriptions_count:
      name: "active_subscriptions_total"
      description: "Current number of active subscriptions"
```

### Environment

| Key | Default | Description |
|---|---|---|
| `env` | — | Environment identifier (`development`, `staging`, `production`) |

This value controls which optional components are started (scheduler, worker) via the `enabled_for_env` lists.

---

## Validation Rules

The service validates all config on startup and exits with a descriptive error listing all missing fields. The full validation logic is in [`config.Validate()`](../internal/config/configure.go).

**Required fields** (service will not start without these):
- Database: `host`, `username`, `password`, `name`, `auth_source`
- Redis: `host`
- JWT: `access_secret`, `refresh_secret`, `issuer`
- Rate limiter: `app.rate`
- Scheduler: `name`, `interval`, `reminder_days`, `startup_delay`
- Queue worker: `concurrency`
- Email: `smtp_host`, `from_email`, `smtp_username`, `smtp_password`
- OTel: `service_name`, `jaeger_endpoint`

**Port validation**: `database.port` and `redis.port` must be between 1 and 65535.
