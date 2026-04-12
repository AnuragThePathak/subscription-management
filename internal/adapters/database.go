package adapters

import (
	"context"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Database wraps the MongoDB client and database instance.
type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Shutdown gracefully disconnects the MongoDB client, respecting the provided context.
func (db *Database) Shutdown(ctx context.Context) error {
	slog.InfoContext(ctx, "Disconnecting MongoDB client")
	if err := db.Client.Disconnect(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to disconnect MongoDB client", slog.Any("error", err))
		return err
	}
	slog.InfoContext(ctx, "MongoDB client disconnected successfully")
	return nil
}

// Ping checks the connection to the MongoDB server.
func (db *Database) Ping(ctx context.Context) error {
	if err := db.Client.Ping(ctx, nil); err != nil {
		slog.ErrorContext(ctx, "MongoDB ping failed", slog.Any("error", err))
		return err
	}
	slog.DebugContext(ctx, "MongoDB ping successful")
	return nil
}
