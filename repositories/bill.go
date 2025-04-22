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

type BillRepository interface {
	Create(context.Context, *models.Bill) (*models.Bill, error)
	GetByID(context.Context, bson.ObjectID) (*models.Bill, error)
	GetRecentBill(context.Context, bson.ObjectID) (*models.Bill, error)
	Update(context.Context, *models.Bill) (*models.Bill, error)
}

type billRepository struct {
	collection *mongo.Collection
}

func NewBillRepository(ctx context.Context, db *mongo.Database) (BillRepository, error) {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "subscription_id", Value: 1},
				{Key: "status", Value: 1},
				{Key: "start_date", Value: -1},
			},
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	collection := db.Collection("bills")
	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %v", err)
	}
	return &billRepository{collection: collection}, nil
}

func (r *billRepository) Create(ctx context.Context, bill *models.Bill) (*models.Bill, error) {
	// Insert the bill into the collection
	_, err := r.collection.InsertOne(ctx, bill)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, apperror.NewConflictError("bill already exists")
		}
		return nil, err
	}

	return bill, nil
}

func (r *billRepository) GetByID(ctx context.Context, id bson.ObjectID) (*models.Bill, error) {
	filter := bson.M{"_id": id}
	return lib.FindOne[models.Bill](ctx, r.collection, filter)
}

func (r *billRepository) GetRecentBill(ctx context.Context, subscriptionID bson.ObjectID) (*models.Bill, error) {
	filter := bson.M{
		"subscription_id": subscriptionID,
		"status":          models.Paid,
	}
	opts := options.FindOne().SetSort(bson.M{"start_date": -1})
	return lib.FindOne[models.Bill](ctx, r.collection, filter, opts)
}

func (r *billRepository) Update(ctx context.Context, bill *models.Bill) (*models.Bill, error) {
	// Update the bill in the collection
	filter := bson.M{"_id": bill.ID}
	update := bson.M{"$set": bill}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apperror.NewDBError(err)
	}
	if res.MatchedCount == 0 {
		return nil, apperror.NewNotFoundError("bill not found")
	}

	return bill, nil
}
