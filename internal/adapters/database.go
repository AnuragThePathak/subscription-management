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
	slog.Info("Disconnecting MongoDB client", slog.String("component", "database"))
	if err := db.Client.Disconnect(ctx); err != nil {
		slog.Error("Failed to disconnect MongoDB client", slog.String("component", "database"), slog.Any("error", err))
		return err
	}
	slog.Info("MongoDB client disconnected successfully", slog.String("component", "database"))
	return nil
}
