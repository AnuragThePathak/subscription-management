package lib

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func FindOne[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	var res T
	err := collection.FindOne(ctx, filter, opts...).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperror.NewNotFoundError("Document not found")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperror.NewTimeoutError(err)
		}
		return nil, apperror.NewDBError(err)
	}
	return &res, nil
}

func FindMany[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, opts ...options.Lister[options.FindOptions]) ([]*T, error) {
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperror.NewTimeoutError(err)
		}
		return nil, apperror.NewDBError(err)
	}
	defer cursor.Close(ctx)

	var res []*T
	for cursor.Next(ctx) {
		var item T
		if err := cursor.Decode(&item); err != nil {
			return nil, apperror.NewDBError(err)
		}
		res = append(res, &item)
	}

	if err := cursor.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperror.NewTimeoutError(err)
		}
		return nil, apperror.NewDBError(err)
	}
	return res, nil
}

func Count(ctx context.Context, collection *mongo.Collection, filter bson.M, opts ...options.Lister[options.CountOptions]) (int64, error) {
	res, err := collection.CountDocuments(ctx, filter, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return 0, apperror.NewTimeoutError(err)
		}
		return 0, apperror.NewDBError(err)
	}
	return res, nil
}

// BuildMongoURI constructs a connection string dynamically based on the host type.
func BuildMongoURI(host string, port int, username, password, dbName, authSource string) string {
	// Escaping ensures special characters in credentials don't break the URI structure.
	escapedUsername := url.PathEscape(username)
	escapedPassword := url.PathEscape(password)

	// If the host is an Atlas cluster, use the SRV protocol.
	// The port is intentionally omitted because DNS handles it.
	if strings.HasSuffix(host, "mongodb.net") {
		return fmt.Sprintf("mongodb+srv://%s:%s@%s/%s?authSource=%s",
			escapedUsername,
			escapedPassword,
			host,
			dbName,
			authSource,
		)
	}

	// For standard, self-hosted, or Docker databases, use the standard protocol
	// and explicitly include the configured port (defaulting to 27017).
	return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=%s",
		escapedUsername,
		escapedPassword,
		host,
		port,
		dbName,
		authSource,
	)
}
