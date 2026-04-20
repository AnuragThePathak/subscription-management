package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Database wraps the MongoDB client and database instance.
type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Shutdown gracefully disconnects the MongoDB client, respecting the provided context.
func (db *Database) Shutdown(ctx context.Context) error {
	slog.Info("Disconnecting MongoDB client")
	if err := db.Client.Disconnect(ctx); err != nil {
		slog.Error("Failed to disconnect MongoDB client", logattr.Error(err))
		return err
	}
	slog.Info("MongoDB client disconnected successfully")
	return nil
}

// Ping checks the connection to the MongoDB server.
func (db *Database) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.Client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	slog.Debug("MongoDB ping successful")
	return nil
}
