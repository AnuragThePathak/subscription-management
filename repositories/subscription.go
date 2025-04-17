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
	GetActiveSubscriptions(context.Context) ([]*models.Subscription, error)
	GetSubscriptionsDueForReminder(context.Context, []int) ([]*models.Subscription, error)
	Update(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

type subscriptionRepository struct {
	collection *mongo.Collection
}

func NewSubscriptionRepository(ctx context.Context, db *mongo.Database) (SubscriptionRepository, error) {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys: bson.D{
				{Key: "renewalDate", Value: 1},
				{Key: "status", Value: 1},
			},
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := db.Collection("subscriptions")
	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %v", err)
	}

	return &subscriptionRepository{
		collection: collection,
	}, nil
}

func (r *subscriptionRepository) Create(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error) {
	if _, err := r.collection.InsertOne(ctx, subscription); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, apperror.NewConflictError("Subscription already exists")
		}
		return nil, apperror.NewDBError(err)
	}
	return subscription, nil
}

func (r *subscriptionRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	filter := bson.M{"_id": id}
	return r.findSubscription(ctx, filter)
}

func (r *subscriptionRepository) GetAll(ctx context.Context) ([]*models.Subscription, error) {
	return r.findSubscriptions(ctx, bson.M{})
}

func (r *subscriptionRepository) GetByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Subscription, error) {
	filter := bson.M{"userId": userID}
	return r.findSubscriptions(ctx, filter)
}

func (r *subscriptionRepository) GetActiveSubscriptions(ctx context.Context) ([]*models.Subscription, error) {
	filter := bson.M{
		"status": models.Active,
		"renewalDate": bson.M{
			"$gt": time.Now(),
		},
	}
	return r.findSubscriptions(ctx, filter)
}

func (r *subscriptionRepository) GetSubscriptionsDueForReminder(ctx context.Context, daysBefore []int) ([]*models.Subscription, error) {
	now := time.Now()
	var orConditions []bson.M
	for _, days := range daysBefore {
		targetDay := now.AddDate(0, 0, days)
		startOfTargetDay := time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 0, 0, 0, 0, targetDay.Location())
		endOfTargetDay := startOfTargetDay.Add(24 * time.Hour)

		orConditions = append(orConditions, bson.M{
			"renewalDate": bson.M{
				"$gte": startOfTargetDay,
				"$lt":  endOfTargetDay,
			},
		})
	}

	filter := bson.M{
		"status": models.Active,
		"$or":    orConditions,
	}
	return r.findSubscriptions(ctx, filter)
}

func (r *subscriptionRepository) Update(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error) {
	filter := bson.M{"_id": subscription.ID}
	update := bson.M{"$set": subscription}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apperror.NewDBError(err)
	}
	if res.MatchedCount == 0 {
		return nil, apperror.NewNotFoundError("Subscription not found")
	}

	return subscription, nil
}

func (r *subscriptionRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	filter := bson.M{"_id": id}

	res, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return apperror.NewDBError(err)
	}
	if res.DeletedCount == 0 {
		return apperror.NewNotFoundError("Subscription not found")
	}

	return nil
}

func (r *subscriptionRepository) findSubscription(ctx context.Context, filter bson.M) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apperror.NewNotFoundError("Subscription not found")
		}
		return nil, apperror.NewDBError(err)
	}
	return &subscription, nil
}

func (r *subscriptionRepository) findSubscriptions(ctx context.Context, filter bson.M) ([]*models.Subscription, error) {
	var subscriptions []*models.Subscription
	cursor, err := r.collection.Find(ctx, filter)
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
