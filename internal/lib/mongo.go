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

// BuildMongoURI constructs a connection string dynamically based on the host type.
func BuildMongoURI(host string, port int, username, password, dbName, authSource string) string {
	// Construct the base URL using Go's native struct
	u := &url.URL{
		Scheme:   "mongodb",
		User:     url.UserPassword(username, password), // This safely triggers the hidden password escaper!
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     "/" + dbName,
		RawQuery: "authSource=" + authSource,
	}

	// Handle the Atlas SRV protocol vs Standard protocol
	if strings.HasSuffix(host, "mongodb.net") {
		u.Scheme = "mongodb+srv"
		u.Host = host // SRV drops the port
	}

	// Let Go compile the final, perfectly escaped string
	return u.String()
}

func Create(
	ctx context.Context,
	collection *mongo.Collection,
	model any,
	opts ...options.Lister[options.InsertOneOptions],
) error {
	_, err := collection.InsertOne(ctx, model, opts...)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperror.NewConflictError("document already exists")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return apperror.NewTimeoutError(err)
		}
		return apperror.NewDBError(err)
	}
	return nil
}

func FindOne[T any](
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
	opts ...options.Lister[options.FindOneOptions],
) (*T, error) {
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

func FindMany[T any](
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*T, error) {
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

func Count(
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
	opts ...options.Lister[options.CountOptions],
) (int64, error) {
	res, err := collection.CountDocuments(ctx, filter, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return 0, apperror.NewTimeoutError(err)
		}
		return 0, apperror.NewDBError(err)
	}
	return res, nil
}

func Update(
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
	model any,
	opts ...options.Lister[options.ReplaceOptions],
) error {
	res, err := collection.ReplaceOne(ctx, filter, model, opts...)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperror.NewConflictError("document conflict")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return apperror.NewTimeoutError(err)
		}
		return apperror.NewDBError(err)
	}
	if res.MatchedCount == 0 {
		return apperror.NewNotFoundError("Document not found")
	}
	return nil
}

func Delete(
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
	opts ...options.Lister[options.DeleteOneOptions],
) error {
	res, err := collection.DeleteOne(ctx, filter, opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return apperror.NewTimeoutError(err)
		}
		return apperror.NewDBError(err)
	}
	if res.DeletedCount == 0 {
		return apperror.NewNotFoundError("Document not found")
	}
	return nil
}
