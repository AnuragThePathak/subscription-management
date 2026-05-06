package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/core/clock"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

type UserServiceExternal interface {
	CreateUser(context.Context, *models.User) (*models.User, error)
	GetAllUsers(context.Context) ([]*models.User, error)
	GetUserByID(context.Context, string, string) (*models.User, error)
	DeleteUser(context.Context, string, string) error
}

type UserServiceInternal interface {
	FetchUserByIDInternal(context.Context, bson.ObjectID) (*models.User, error)
	FetchUserByEmailInternal(context.Context, string) (*models.User, error)
}

type UserService interface {
	UserServiceExternal
	UserServiceInternal
}

type userService struct {
	userRepository              repositories.UserRepository
	subscriptionServiceInternal SubscriptionServiceInternal
	getTime                     clock.NowFn
}

// NewUserService creates a new instance of UserService.
func NewUserService(
	userRepository repositories.UserRepository,
	subscriptionServiceInternal SubscriptionServiceInternal,
	nowFn clock.NowFn,
) UserService {
	return &userService{
		userRepository,
		subscriptionServiceInternal,
		nowFn,
	}
}

// CreateUser creates a new user in the system.
func (us *userService) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	// Check if the user already exists
	existingUser, err := us.userRepository.FindByEmail(ctx, user.Email)
	if existingUser != nil {
		return nil, apperror.NewConflictError("Email already in use").
			WithLogAttributes(logattr.AttemptedID(user.Email))
	}
	if err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			if appErr.Code() != apperror.ErrNotFound {
				return nil, appErr.WithLogAttributes(logattr.AttemptedID(user.Email))
			}
		} else {
			return nil, err
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return nil, apperror.NewInternalError(err).
			WithLogAttributes(logattr.AttemptedID(user.Email))
	}
	user.Password = string(hashedPassword)

	// Set ID
	user.ID = bson.NewObjectID()

	// Set timestamps
	now := us.getTime()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Insert into database
	result, err := us.userRepository.Create(ctx, user)
	if err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			return nil, appErr.WithLogAttributes(logattr.AttemptedID(user.Email))
		} else {
			return nil, err
		}
	}

	slog.InfoContext(ctx, "User created", logattr.UserID(result.ID.Hex()))
	return result, nil
}

func (us *userService) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	return us.userRepository.GetAll(ctx)
}

func (us *userService) GetUserByID(ctx context.Context, id string, claimedUserID string) (*models.User, error) {
	if id != claimedUserID {
		return nil, apperror.NewForbiddenError("You can only view your own profile")
	}
	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	return us.userRepository.FindByID(ctx, userID)
}

func (us *userService) DeleteUser(ctx context.Context, id string, claimedUserID string) error {
	if id != claimedUserID {
		return apperror.NewForbiddenError("You can only delete your own profile")
	}
	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewUnauthorizedError("Invalid user ID")
	}

	// Check if user has any active subscriptions
	hasActive, err := us.subscriptionServiceInternal.HasActiveSubscriptionsInternal(ctx, userID)
	if err != nil {
		return err
	}
	if hasActive {
		return apperror.NewConflictError("User has active subscriptions and cannot be deleted")
	}

	// Delete the user
	if err = us.userRepository.Delete(ctx, userID); err != nil {
		return err
	}

	slog.InfoContext(ctx, "User deleted")
	return nil
}

func (us *userService) FetchUserByIDInternal(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	return us.userRepository.FindByID(ctx, id)
}

func (us *userService) FetchUserByEmailInternal(ctx context.Context, email string) (*models.User, error) {
	return us.userRepository.FindByEmail(ctx, email)
}
