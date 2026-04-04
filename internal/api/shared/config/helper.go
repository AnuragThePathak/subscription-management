package config

import (
	"log/slog"
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
func DatabaseConnection(dbConfig DatabaseConfig) (*adapters.Database, error) {
	dbClientOpts := options.Client().
		ApplyURI(dbConfig.URL).
		SetMonitor(otelmongo.NewMonitor()) // Inject OpenTelemetry monitor for DB tracing

	db := adapters.Database{}
	var err error
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		slog.Error("Failed to initialize MongoDB client", slog.String("url", dbConfig.URL), slog.String("error", err.Error()))
		return nil, err
	}
	db.DB = db.Client.Database(dbConfig.Name)
	slog.Info("Initialized MongoDB client", slog.String("database", dbConfig.Name))
	return &db, nil
}

// RedisConnection establishes a connection to the Redis database.
func RedisConnection(redisConfig RedisConfig) *adapters.Redis {
	rdb := adapters.Redis{}
	rdb.Client = redis.NewClient(&redis.Options{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})

	// Inject OpenTelemetry tracing hooks into the Redis client
	if err := redisotel.InstrumentTracing(rdb.Client); err != nil {
		slog.Error("Failed to instrument Redis with tracing", slog.Any("error", err))
	}

	slog.Info("Connected to Redis", slog.String("url", redisConfig.URL), slog.Int("db", redisConfig.DB))
	return &rdb
}

// SetupLogger configures the global logger based on the environment.
// The handler is wrapped with trace correlation so that any log call
// using slog.InfoContext (or similar) with a traced context automatically
// includes trace_id and span_id fields.
func SetupLogger(env string) {
	programLevel := new(slog.LevelVar)

	var handler slog.Handler
	if env == "production" {
		programLevel.Set(slog.LevelInfo)
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: programLevel,
		})
	} else {
		programLevel.Set(slog.LevelDebug)
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: programLevel,
		})
	}

	// Wrap with trace correlation — adds trace_id/span_id when an OTel span is active.
	handler = observability.NewTraceHandler(handler)

	slog.SetDefault(slog.New(handler))
	slog.Info("Logger initialized", slog.String("environment", env))
}

// NewRateLimit creates a rate limiter configuration.
func NewRateLimit(rateConfig *RateLimiterConfig) *redis_rate.Limit {
	if rateConfig.Burst == 0 {
		rateConfig.Burst = rateConfig.Rate
	}
	if rateConfig.Period == 0 {
		rateConfig.Period = time.Minute
	}
	slog.Debug("Rate limiter configured", slog.Int("rate", rateConfig.Rate), slog.Int("burst", rateConfig.Burst), slog.Duration("period", rateConfig.Period))

	return &redis_rate.Limit{
		Rate:   rateConfig.Rate,
		Burst:  rateConfig.Burst,
		Period: rateConfig.Period,
	}
}

// QueueRedisConfig returns Redis configuration for the task queue.
func QueueRedisConfig(redisConfig RedisConfig) *asynq.RedisClientOpt {
	slog.Debug("Queue Redis configuration initialized", slog.String("url", redisConfig.URL), slog.Int("db", redisConfig.DB))
	return &asynq.RedisClientOpt{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	}
}
