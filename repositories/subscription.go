package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SubscriptionRepository interface {
	Create(context.Context, *models.Subscription) (*models.Subscription, error)

	GetByID(context.Context, bson.ObjectID) (*models.Subscription, error)

	GetAll(context.Context) ([]*models.Subscription, error)

	GetByUserID(context.Context, bson.ObjectID) ([]*models.Subscription, error)
}


type subscriptionRepository struct {
	collection *mongo.Collection
}

func NewSubscriptionRepository(db *mongo.Database) (SubscriptionRepository, error) {
	// Create index on user field for faster lookups
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetSparse(true),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.Collection("users")
	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		return nil, fmt.Errorf("failed to create index for user field: %v", err)
	}

	return &subscriptionRepository{
		collection: db.Collection("subscriptions"),
	}, nil
}

func (r *subscriptionRepository) Create(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error) {
	if _, err := r.collection.InsertOne(ctx, subscription); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, apperror.NewConflictError("Failed to generate a unique subscription ID")
		}
		return nil, apperror.NewDBError(err)
	}

	return subscription, nil
}

func (r *subscriptionRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apperror.NewNotFoundError("Subscription not found")
		}
		return nil, apperror.NewDBError(err)
	}

	return &subscription, nil
}

func (r *subscriptionRepository) GetAll(ctx context.Context) ([]*models.Subscription, error) {
	var subscriptions []*models.Subscription
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, apperror.NewDBError(err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var subscription models.Subscription
		if err := cursor.Decode(&subscription); err != nil {
			return nil, apperror.NewDBError(err)
		}
		subscriptions = append(subscriptions, &subscription)
	}

	if err := cursor.Err(); err != nil {
		return nil, apperror.NewDBError(err)
	}

	return subscriptions, nil
}

func (r *subscriptionRepository) GetByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Subscription, error) {
	var subscriptions []*models.Subscription
	cursor, err := r.collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, apperror.NewDBError(err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var subscription models.Subscription
		if err := cursor.Decode(&subscription); err != nil {
			return nil, apperror.NewDBError(err)
		}
		subscriptions = append(subscriptions, &subscription)
	}

	if err := cursor.Err(); err != nil {
		return nil, apperror.NewDBError(err)
	}

	return subscriptions, nil
}