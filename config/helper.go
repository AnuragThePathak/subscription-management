package config

import (
	"log/slog"
	"os"

	"github.com/anuragthepathak/subscription-management/wrappers"
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