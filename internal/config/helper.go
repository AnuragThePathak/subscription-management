package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/anuragthepathak/subscription-management/internal/adapters"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/anuragthepathak/subscription-management/internal/observability"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
	"go.opentelemetry.io/otel"
)

// DatabaseConnection establishes a connection to the MongoDB database.
func DatabaseConnection(dbConfig DatabaseConfig, otelEnabled bool) (*adapters.Database, error) {
	dbClientOpts := options.Client().ApplyURI(
		lib.BuildMongoURI(
			dbConfig.Host,
			dbConfig.Port,
			dbConfig.Username,
			dbConfig.Password,
			dbConfig.Name,
			dbConfig.AuthSource,
		),
	)

	if otelEnabled {
		dbClientOpts.SetMonitor(
			otelmongo.NewMonitor(
				otelmongo.WithMeterProvider(otel.GetMeterProvider()),
			),
		)
	}

	db := adapters.Database{}
	var err error
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		return nil, fmt.Errorf("failed to initialize MongoDB client: %w", err)
	}
	db.DB = db.Client.Database(dbConfig.Name)

	slog.Info("Initialized MongoDB client",
		logattr.Host(dbConfig.Host),
		logattr.Port(dbConfig.Port),
		logattr.Database(dbConfig.Name),
	)
	return &db, nil
}

// RedisConnection establishes a connection to the Redis database.
func RedisConnection(
	redisConfig RedisConfig,
	otelEnabled bool,
) (*adapters.Redis, error) {
	addr := fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port)
	rdb := adapters.Redis{}
	rdb.Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	if otelEnabled {
		if err := redisotel.InstrumentTracing(rdb.Client); err != nil {
			return nil, fmt.Errorf("failed to instrument Redis with tracing: %w", err)
		}
		if err := redisotel.InstrumentMetrics(rdb.Client); err != nil {
			return nil, fmt.Errorf("failed to instrument Redis metrics: %w", err)
		}
	}

	slog.Info("Initialized Redis client",
		logattr.Host(redisConfig.Host),
		logattr.Port(redisConfig.Port),
		logattr.RedisDB(redisConfig.DB),
	)
	return &rdb, nil
}

// SetupLogger configures the global logger based on the environment.
// The handler is wrapped with trace correlation so that any log call
// using slog.InfoContext (or similar) with a traced context automatically
// includes trace_id and span_id fields.
//
// When OTel is enabled, logs are written as JSON to both stderr and
// ./logs/app.log (for Promtail to tail and ship to Loki).
func SetupLogger(env string, otelEnabled bool) error {
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
		}
		logFile, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("failed to open log file named app.log: %w", err)
		}
		writers = append(writers, logFile)

		handler = slog.NewJSONHandler(
			io.MultiWriter(writers...),
			&slog.HandlerOptions{
				Level:     programLevel,
				AddSource: true,
			},
		)
	} else if env == "production" {
		handler = slog.NewJSONHandler(
			os.Stderr,
			&slog.HandlerOptions{
				Level:     programLevel,
				AddSource: true,
			},
		)
	} else {
		handler = slog.NewTextHandler(
			os.Stderr,
			&slog.HandlerOptions{
				Level:     programLevel,
				AddSource: true,
			},
		)
	}

	// Wrap with trace correlation — adds trace_id/span_id when an OTel span is active.
	handler = observability.NewTraceHandler(handler)

	slog.SetDefault(slog.New(handler))
	slog.Info("Logger initialized",
		logattr.Env(env),
		logattr.OtelEnabled(otelEnabled),
	)
	return nil
}

// NewRateLimit creates a rate limiter configuration.
func NewRateLimit(rateConfig RateLimiterConfig) redis_rate.Limit {
	if rateConfig.Burst == 0 {
		rateConfig.Burst = rateConfig.Rate
	}

	return redis_rate.Limit{
		Rate:   rateConfig.Rate,
		Burst:  rateConfig.Burst,
		Period: rateConfig.Period,
	}
}

// QueueRedisConfig returns Redis configuration for the task queue.
func QueueRedisConfig(redisConfig RedisConfig) *asynq.RedisClientOpt {
	return &asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	}
}
