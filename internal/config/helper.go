package config

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/adapters"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
)

// DatabaseConnection establishes a connection to the MongoDB database.
func DatabaseConnection(ctx context.Context, dbConfig DatabaseConfig, otelEnabled bool) (*adapters.Database, error) {
	dbClientOpts := options.Client().ApplyURI(dbConfig.URL)
	if otelEnabled {
		dbClientOpts.SetMonitor(otelmongo.NewMonitor())
	}

	db := adapters.Database{}
	var err error
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		slog.ErrorContext(ctx, "Failed to initialize MongoDB client",
			slog.String("host", redactURL(dbConfig.URL)),
			slog.Any("error", err),
		)
		return nil, err
	}
	db.DB = db.Client.Database(dbConfig.Name)
	slog.InfoContext(ctx, "Initialized MongoDB client",
		slog.String("host", redactURL(dbConfig.URL)),
		slog.String("database", dbConfig.Name),
	)
	return &db, nil
}

// RedisConnection establishes a connection to the Redis database.
func RedisConnection(ctx context.Context, redisConfig RedisConfig, otelEnabled bool) *adapters.Redis {
	rdb := adapters.Redis{}
	rdb.Client = redis.NewClient(&redis.Options{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	if otelEnabled {
		if err := redisotel.InstrumentTracing(rdb.Client); err != nil {
			slog.ErrorContext(ctx, "Failed to instrument Redis with tracing", slog.Any("error", err))
		}
	}

	slog.Info("Connected to Redis", slog.String("url", redisConfig.URL), slog.Int("db", redisConfig.DB))
	return &rdb
}

// SetupLogger configures the global logger based on the environment.
// The handler is wrapped with trace correlation so that any log call
// using slog.InfoContext (or similar) with a traced context automatically
// includes trace_id and span_id fields.
//
// When OTel is enabled, logs are written as JSON to both stderr and
// ./logs/app.log (for Promtail to tail and ship to Loki).
// ./logs/app.log (for Promtail to tail and ship to Loki).
func SetupLogger(ctx context.Context, env string, otelEnabled bool) error {
	programLevel := new(slog.LevelVar)
	if env == "production" {
		programLevel.Set(slog.LevelInfo)
	} else {
		programLevel.Set(slog.LevelDebug)
	}

	var handler slog.Handler
	if otelEnabled {
		// When OTel is enabled, always use JSON for structured log ingestion.
		// Promtail tails ./logs/app.log and requires JSON for trace_id extraction.
		writers := []io.Writer{os.Stderr}

		if err := os.MkdirAll("logs", 0o755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		} else if logFile, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		} else {
			writers = append(writers, logFile)
		}

		handler = slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{
			Level:     programLevel,
			AddSource: true,
		})
	} else if env == "production" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level:     programLevel,
			AddSource: true,
		})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     programLevel,
			AddSource: true,
		})
	}

	// Wrap with trace correlation — adds trace_id/span_id when an OTel span is active.
	handler = observability.NewTraceHandler(handler)

	slog.SetDefault(slog.New(handler))
	slog.InfoContext(ctx, "Logger initialized", slog.String("environment", env))
	return nil
}

// NewRateLimit creates a rate limiter configuration.
func NewRateLimit(ctx context.Context, rateConfig *RateLimiterConfig) *redis_rate.Limit {
	if rateConfig.Burst == 0 {
		rateConfig.Burst = rateConfig.Rate
	}
	if rateConfig.Period == 0 {
		rateConfig.Period = time.Minute
	}
	slog.InfoContext(ctx, "Rate limiter configured", slog.Int("rate", rateConfig.Rate), slog.Int("burst", rateConfig.Burst), slog.Duration("period", rateConfig.Period))

	return &redis_rate.Limit{
		Rate:   rateConfig.Rate,
		Burst:  rateConfig.Burst,
		Period: rateConfig.Period,
	}
}

// QueueRedisConfig returns Redis configuration for the task queue.
func QueueRedisConfig(ctx context.Context, redisConfig RedisConfig) *asynq.RedisClientOpt {
	slog.DebugContext(ctx, "Queue Redis configuration initialized",
		slog.String("addr", redactURL(redisConfig.URL)),
		slog.Int("db", redisConfig.DB),
	)
	return &asynq.RedisClientOpt{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	}
}

// redactURL strips credentials from a connection string, returning only the host.
// Falls back to the raw value if parsing fails.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "<unparseable>"
	}
	return u.Host
}
