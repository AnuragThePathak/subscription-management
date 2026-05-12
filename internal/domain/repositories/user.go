package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRepository interface {
	Create(context.Context, *models.User) (*models.User, error)
	FindByEmail(context.Context, string) (*models.User, error)
	FindByID(context.Context, bson.ObjectID) (*models.User, error)
	GetAll(context.Context) ([]*models.User, error)
	Update(ctx context.Context, user *models.User) (*models.User, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(ctx context.Context, db *mongo.Database) (UserRepository, error) {
	// Create a unique index for the email field
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	collection := db.Collection("users")
	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		return nil, fmt.Errorf("failed to create index for email field: %w", err)
	}
	slog.Debug("User repository initialized and index verified")

	return &userRepository{
		collection: collection,
	}, nil
}

// Create adds a new user to the database from a signup request
func (uc *userRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	// Insert into database
	if err := lib.Create(ctx, uc.collection, user); err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok &&
			appErr.Code() == apperror.ErrConflict {
			return nil, apperror.NewConflictError("Email already exists")
		}
		return nil, err
	}

	return user, nil
}

func (uc *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	filter := bson.M{"email": email}
	return lib.FindOne[models.User](ctx, uc.collection, filter)
}

func (uc *userRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	filter := bson.M{"_id": id}
	return lib.FindOne[models.User](ctx, uc.collection, filter)
}

func (uc *userRepository) GetAll(ctx context.Context) ([]*models.User, error) {
	return lib.FindMany[models.User](ctx, uc.collection, bson.M{})
}

func (uc *userRepository) Update(ctx context.Context, user *models.User) (*models.User, error) {
	filter := bson.M{"_id": user.ID}
	if err := lib.Update(ctx, uc.collection, filter, user); err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok &&
			appErr.Code() == apperror.ErrConflict {
			return nil, apperror.NewConflictError("Email already exists")
		}
		return nil, err
	}

	return user, nil
}

func (uc *userRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	return lib.Delete(ctx, uc.collection, bson.M{"_id": id})
}
