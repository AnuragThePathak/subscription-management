# Configuration

Configuration is validated on startup and the service fails fast on missing required values or invalid values.

Configuration is loaded from `config.yaml` in the project root. Environment variables with `APP_` prefix override file settings.

## Example Configuration

```yaml
server:
  port: 8080
tls:
  enabled: false
  cert_path: ""
  key_path: ""

database:
  url: "mongodb://localhost:27017"
  name: "subscription_management"

jwt:
  access_secret: "your-access-secret-key-here"
  refresh_secret: "your-refresh-secret-key-here"
  access_timeout: 1      # hours
  refresh_timeout: 168   # hours (7 days)
  issuer: "subscription-management"

redis:
  url: "localhost:6379"
  password: ""
  db: 0

rate_limiter:
  app:
    rate: 1
    burst: 5
    period: "2s"

scheduler:
  interval: "12h"
  reminder_days: [1, 3, 7]

queue_worker:
  concurrency: 2

email:
  smtp_host: "smtp.gmail.com"
  smtp_port: 587
  from_email: "no-reply@example.com"
  from_name: "Subscription Management"
  smtp_username: "your-email@gmail.com"
  smtp_password: "your-app-password"
  account_url: "https://example.com/account"
  support_url: "https://example.com/support"

env: "development"
```

## Environment Variables

Override any setting with `APP_` prefix:

```bash
APP_DATABASE_URL="mongodb://..."
APP_JWT_ACCESS_SECRET="..."
APP_REDIS_URL="redis:6379"
```

## Required Fields

The service will not start without these:

- `database.url`, `database.name`
- `jwt.access_secret`, `jwt.refresh_secret`, `jwt.issuer`
- `redis.url`
- `rate_limiter.app.rate`
- `email.smtp_host`, `from_email`, `smtp_username`, `smtp_password`

## Notes

- **JWT secrets**: Use different values for access and refresh tokens
- **Gmail SMTP**: Requires an App Password, not your regular password
- **Rate limiter**: `rate: 1, burst: 5, period: "2s"` = 1 req/2s average, bursts up to 5
- **Scheduler interval**: How often to check for renewals/reminders (Go duration format: `"12h"`, `"30m"`)
