package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/lib"
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
	GetSubscriptionsDueForRenewal(context.Context, time.Time, time.Time) ([]*models.Subscription, error)
	GetCancelledExpiredSubscriptions(context.Context) ([]*models.Subscription, error)
	Update(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

type subscriptionRepository struct {
	collection *mongo.Collection
}

func NewSubscriptionRepository(ctx context.Context, db *mongo.Database) (SubscriptionRepository, error) {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetSparse(true),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "valid_till", Value: 1},
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
	return lib.FindOne[models.Subscription](ctx, r.collection, filter)
}

func (r *subscriptionRepository) GetAll(ctx context.Context) ([]*models.Subscription, error) {
	return lib.FindMany[models.Subscription](ctx, r.collection, bson.M{})
}

func (r *subscriptionRepository) GetByUserID(ctx context.Context, userID bson.ObjectID) ([]*models.Subscription, error) {
	filter := bson.M{"user_id": userID}
	return lib.FindMany[models.Subscription](ctx, r.collection, filter)
}

func (r *subscriptionRepository) GetActiveSubscriptions(ctx context.Context) ([]*models.Subscription, error) {
	filter := bson.M{
		"status": models.Active,
		"valid_till": bson.M{
			"$gt": time.Now(),
		},
	}
	return lib.FindMany[models.Subscription](ctx, r.collection, filter)
}

func (r *subscriptionRepository) GetSubscriptionsDueForReminder(ctx context.Context, daysBefore []int) ([]*models.Subscription, error) {
	now := time.Now()
	var orConditions []bson.M
	for _, days := range daysBefore {
		targetDay := now.AddDate(0, 0, days)
		startOfTargetDay := time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 0, 0, 0, 0, targetDay.Location())
		endOfTargetDay := startOfTargetDay.Add(24 * time.Hour)

		orConditions = append(orConditions, bson.M{
			"valid_till": bson.M{
				"$gte": startOfTargetDay,
				"$lt":  endOfTargetDay,
			},
		})
	}

	filter := bson.M{
		"status": models.Active,
		"$or":    orConditions,
	}
	return lib.FindMany[models.Subscription](ctx, r.collection, filter)
}

func (r *subscriptionRepository) GetSubscriptionsDueForRenewal(ctx context.Context, startTime, endTime time.Time) ([]*models.Subscription, error) {
	filter := bson.M{
		"status": models.Active,
		"valid_till": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "valid_till", Value: 1}})

	return lib.FindMany[models.Subscription](ctx, r.collection, filter, opts)
}

func (r *subscriptionRepository) GetCancelledExpiredSubscriptions(ctx context.Context) ([]*models.Subscription, error) {
	filter := bson.M{
		"status": models.Cancelled,
		"valid_till": bson.M{
			"$lt": time.Now(),
		},
	}

	return lib.FindMany[models.Subscription](ctx, r.collection, filter)
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
