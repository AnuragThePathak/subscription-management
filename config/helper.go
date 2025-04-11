package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/anuragthepathak/subscription-management/wrappers"
	"github.com/go-redis/redis_rate/v10"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func DatabaseConnection(dbConfig DatabaseConfig) (*wrappers.Database, error) {
	dbClientOpts := options.Client().ApplyURI(dbConfig.URL)
	db := wrappers.Database{}
	var err error
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		return nil, err
	}
	db.DB = db.Client.Database(dbConfig.Name)
	return &db, nil
}

func RedisConnection(redisConfig RedisConfig) *wrappers.Redis {
	rdb := wrappers.Redis{}
	rdb.Client = redis.NewClient(&redis.Options{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	})
	return &rdb
}

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
}

func NewRateLimit(rateConfig *RateLimiterConfig) *redis_rate.Limit {
	if rateConfig.Burst == 0 {
		rateConfig.Burst = rateConfig.Rate
	}
	if rateConfig.Period == 0 {
		rateConfig.Period = time.Minute
	}
	slog.Debug("Rate limit config", "rate", rateConfig.Rate, "burst", rateConfig.Burst, "period", rateConfig.Period)

	limit := redis_rate.Limit{
		Rate:   rateConfig.Rate,
		Burst:  rateConfig.Burst,
		Period: rateConfig.Period,
	}

	return &limit
}

func QueueRedisConfig(redisConfig RedisConfig) *asynq.RedisClientOpt {
	return &asynq.RedisClientOpt{
		Addr:     redisConfig.URL,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	}
}
