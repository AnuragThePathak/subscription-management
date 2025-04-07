package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/anuragthepathak/subscription-management/wrappers"
	"github.com/go-redis/redis_rate/v10"
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
	limit := redis_rate.Limit{
		Rate:  rateConfig.Rate,
		Burst: rateConfig.Burst,
	}

	switch rateConfig.Unit {
	case "second":
		limit.Period = time.Duration(rateConfig.Duration) * time.Second
	case "minute":
		limit.Period = time.Duration(rateConfig.Duration) * time.Minute
	case "hour":
		limit.Period = time.Duration(rateConfig.Duration) * time.Hour
	case "day":
		limit.Period = time.Duration(rateConfig.Duration) * time.Hour * 24
	case "week":
		limit.Period = time.Duration(rateConfig.Duration) * time.Hour * 24 * 7
	case "month":
		limit.Period = time.Duration(rateConfig.Duration) * time.Hour * 24 * 30
	default:
		limit.Period = time.Duration(rateConfig.Duration) * time.Second
	}

	return &limit
}