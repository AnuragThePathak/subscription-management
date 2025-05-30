# Subscription Management Service

A robust Go-based subscription management system for handling user subscriptions, billing, and authentication.

## Overview

This project provides a complete backend service for managing user subscriptions, with features for authentication, subscription lifecycle management, billing, and notifications.

## Features

- User authentication and authorization
- Subscription creation, management, and tracking
- Automated billing
- Email notifications for subscription events
- Rate limiting for API protection
- Background job processing with scheduler and worker components

## Technologies

- Go (Golang)
- MongoDB for persistent storage
- Redis for caching and rate limiting
- JWT for authentication
- RESTful API design
  
## Internal Working

- Two main components api server and scheduler in a loosely coupled monolith architecture.
- Configuration management using either yaml or environment variables.
- Users can create and cancel subscriptions.
- Active subscriptions are automatically renewed just before the billing period ends.
- Cancelled subscriptions are refunded only if the billing period has yet not started.
- Cancelled subscriptions remain active until the current billing ends and becomes expired automatically after that.
- Reminder emails are sent on multiple days before renewal for active subscriptions and also immediately it is renewed.

## Project Structure

```
📦 
├─ .gitignore
├─ go.mod
├─ go.sum
├─ internal
│  ├─ adapters
│  │  ├─ database.go
│  │  ├─ redis.go
│  │  ├─ scheduler.go
│  │  ├─ server.go
│  │  └─ worker.go
│  ├─ api
│  │  ├─ controllers
│  │  │  ├─ auth.go
│  │  │  ├─ subscriptions.go
│  │  │  └─ user.go
│  │  ├─ middlewares
│  │  │  ├─ auth.go
│  │  │  └─ rate_limiter.go
│  │  └─ shared
│  │     ├─ apperror
│  │     │  ├─ error.go
│  │     │  └─ factory.go
│  │     ├─ config
│  │     │  ├─ config.go
│  │     │  ├─ configure.go
│  │     │  └─ helper.go
│  │     └─ endpoint
│  │        ├─ endpoint.go
│  │        ├─ internal_request.go
│  │        └─ response.go
│  ├─ domain
│  │  ├─ models
│  │  │  ├─ auth.go
│  │  │  ├─ bill.go
│  │  │  ├─ subscription.go
│  │  │  └─ user.go
│  │  ├─ repositories
│  │  │  ├─ bill.go
│  │  │  ├─ subscription.go
│  │  │  └─ user.go
│  │  └─ services
│  │     ├─ auth.go
│  │     ├─ jwt.go
│  │     ├─ rate_limiter.go
│  │     ├─ subscription.go
│  │     └─ user.go
│  ├─ lib
│  │  ├─ auth.go
│  │  ├─ mongo.go
│  │  └─ time.go
│  ├─ notifications
│  │  ├─ email_sender.go
│  │  └─ email_template.go
│  └─ scheduler
│     ├─ scheduler.go
│     └─ worker.go
└─ main.go
```

## Getting Started

### Prerequisites

- Go 1.24 or later
- MongoDB
- Redis

### Installation

1. Clone the repository
   ```bash
   git clone https://github.com/AnuragThePathak/subscription-management.git
   cd subscription-management
   ```

2. Install dependencies
   ```bash
   go mod download
   ```

3. Configure the application
   Create a `config.yaml` file based on the example configuration provided in the repository.

4. Run the application
   ```bash
   go run main.go
   ```

## API Documentation

The API supports the following endpoints:

- **Authentication**
  - `POST /api/v1/auth/register` - Register a new user
  - `POST /api/v1/auth/login` - Log in an existing user
  - `POST /api/v1/auth/refresh` - Refresh authentication token
    
- **Users**
  - `GET /api/v1/users` - List users
  - `GET /api/v1/users/:id` - Get user information
  - `PUT /api/v1/users/:id` - Update user information
  - `DELETE /api/v1/users/:id` - Delete user
    
- **Subscriptions**
  - `GET /api/v1/subscriptions` - List subscriptions
  - `POST /api/v1/subscriptions` - Create a new subscription
  - `GET /api/v1/subscriptions/:id` - Get subscription details
  - `GET /api/v1/subscriptions/user/:id` - List subscriptions for a user
  - `PUT /api/v1/subscriptions/:id/cancel` - Cancel a subscription
  - `DELETE /api/v1/subscriptions/:id` - Delete a subscription

## Development

### Building

Build the application with:

```bash
go build -o subscription-service
```

### Example configuration

config.yaml

```yaml
server:
  port: 8080  # Port your server will run on
tls:
  enabled: false  # Set to true to enable TLS
  cert_path: ""  # Path to TLS certificate (required if TLS is enabled)
  key_path: ""  # Path to TLS private key (required if TLS is enabled)
database:
  url: "mongodb://localhost:27017"  # MongoDB connection URI
  name: "main"  # MongoDB database name
jwt:
  access_secret: "your-access-secret-key-here"  # Secret used to sign access tokens
  refresh_secret: "your-refresh-secret-key-here"  # Secret used to sign refresh tokens
  access_timeout: 1  # Expiry in hours for access tokens
  refresh_timeout: 168  # Expiry in hours for refresh tokens (e.g., 7 days)
  issuer: "subscription-management"  # Issuer claim for tokens
rate_limiter:
  app:
    rate: 1
    burst: 5
    period: "2s"
  redis:
    url: "localhost:6379"
    password: ""
    db: 0
scheduler:
  interval: "12h"
  reminder_days: [1, 3, 7]  # Days before expiration to send reminders
queue_worker:
  concurrency: 2  # Number of concurrent workers for processing tasks
email:
  smtp_host: "smtp.gmail.com"  # SMTP server host
  smtp_port: 587  # SMTP server port
  from_email: "no-reply@subscription.com"
  from_name: "Subscription Management"  # Name to display in the "From" field
  smtp_username: "your-email@gmail.com"
  smtp_password: "your-app-password"  # SMTP server password
  account_url: "https://example.com/account"  # URL for account management
  support_url: "https://example.com/support"  # URL for support
env: "development"  # Environment (development, production, etc.)
```


## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
