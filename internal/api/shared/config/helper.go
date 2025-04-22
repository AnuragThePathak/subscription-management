package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/adapters"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DatabaseConnection establishes a connection to the MongoDB database.
func DatabaseConnection(dbConfig DatabaseConfig) (*adapters.Database, error) {
	dbClientOpts := options.Client().ApplyURI(dbConfig.URL)
	db := adapters.Database{}
	var err error
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		slog.Error("Failed to connect to MongoDB", slog.String("url", dbConfig.URL), slog.String("error", err.Error()))
		return nil, err
	}
	db.DB = db.Client.Database(dbConfig.Name)
	slog.Info("Connected to MongoDB", slog.String("database", dbConfig.Name))
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
	slog.Info("Connected to Redis", slog.String("url", redisConfig.URL), slog.Int("db", redisConfig.DB))
	return &rdb
}

// SetupLogger configures the global logger based on the environment.
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
