package wrappers

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Database struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func (db *Database) Shutdown(ctx context.Context) error {
	if err := db.Client.Disconnect(ctx); err != nil {
		return err
	}
	return nil
}
