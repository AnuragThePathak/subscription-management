package main

import (
	"log/slog"
	"os"

	"github.com/AnuragThePathak/my-go-packages/env"
	"github.com/AnuragThePathak/my-go-packages/srv"
	"github.com/anuragthepathak/subscription-management/wrappers"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func databaseConnection() (*wrappers.Database, error) {
	dbUrl, err := env.GetEnv("DB_URL")
	if err != nil {
		return nil, err
	}

	dbName, err := env.GetEnv("DB_NAME")
	if err != nil {
		return nil, err
	}

	dbClientOpts := options.Client().ApplyURI(dbUrl)

	db := wrappers.Database{}
	
	if db.Client, err = mongo.Connect(dbClientOpts); err != nil {
		return nil, err
	}
	
	db.DB = db.Client.Database(dbName)

	return &db, nil
}

func serverConfig() (*srv.ServerConfig, error) {
	var err error
	config := srv.ServerConfig{}

	// Load port
	if config.Port, err = env.GetEnvAsInt("PORT", 8080); err != nil {
		return nil, err
	}
	slog.Debug("Loaded server port", slog.Int("port", config.Port))

	// Load TLS enabled flag
	if config.TLSEnabled, err = env.GetEnvAsBool("TLS_ENABLED", false); err != nil {
		return nil, err
	}
	slog.Debug("TLS enabled flag", slog.Bool("enabled", config.TLSEnabled))

	if config.TLSEnabled {
		// Load TLS certificate path
		if config.TLSCertPath, err = env.GetEnv("TLS_CERT_PATH"); err != nil {
			return nil, err
		}

		// Load TLS key path
		if config.TLSKeyPath, err = env.GetEnv("TLS_KEY_PATH"); err != nil {
			return nil, err
		}
		slog.Debug("Loaded TLS configuration",
			slog.String("certPath", config.TLSCertPath),
			slog.String("keyPath", config.TLSKeyPath),
		)
	}

	return &config, nil
}

func setupLogger() {
	env, _ := env.GetEnv("ENV")
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
