package lib

import (
	"context"
	"errors"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func FindOne[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, opts ...options.Lister[options.FindOneOptions]) (*T, error) {
	var result T
	err := collection.FindOne(ctx, filter, opts...).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperror.NewNotFoundError("Document not found")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperror.NewTimeoutError(err)
		}
		return nil, apperror.NewDBError(err)
	}
	return &result, nil
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

	var results []*T
	for cursor.Next(ctx) {
		var item T
		if err := cursor.Decode(&item); err != nil {
			return nil, apperror.NewDBError(err)
		}
		results = append(results, &item)
	}

	if err := cursor.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperror.NewTimeoutError(err)
		}
		return nil, apperror.NewDBError(err)
	}
	return results, nil
}
