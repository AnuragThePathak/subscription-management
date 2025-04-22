package services

import (
	"context"
	"errors"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

type UserServiceExternal interface {
	CreateUser(context.Context, *models.User) (*models.User, error)
	GetAllUsers(context.Context) ([]*models.User, error)
	GetUserByID(context.Context, string, string) (*models.User, error)
	UpdateUser(context.Context, string, *models.UserUpdateRequest, string) (*models.User, error)
	DeleteUser(context.Context, string, string) error
}

type UserServiceInternal interface {
	FetchUserByIDInternal(context.Context, bson.ObjectID) (*models.User, error)
}

type UserService interface {
	UserServiceExternal
	UserServiceInternal
}

type userService struct {
	userRepository         repositories.UserRepository
	subscriptionRepository repositories.SubscriptionRepository
}

// NewUserService creates a new instance of UserService.
func NewUserService(userRepository repositories.UserRepository, subscriptionRepository repositories.SubscriptionRepository) UserService {
	return &userService{
		userRepository,
		subscriptionRepository,
	}
}

// CreateUser creates a new user in the system.
func (us *userService) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	// Check if the user already exists
	existingUser, err := us.userRepository.FindByEmail(ctx, user.Email)
	if existingUser != nil {
		return nil, apperror.NewConflictError("Email already in use")
	}
	if err != nil {
		var appErr apperror.AppError
		if !errors.As(err, &appErr) || appErr.Code() != apperror.ErrNotFound {
			return nil, err
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return nil, apperror.NewInternalError(err)
	}
	user.Password = string(hashedPassword)

	// Set ID
	user.ID = bson.NewObjectID()

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Insert into database
	return us.userRepository.Create(ctx, user)
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

func (us *userService) UpdateUser(ctx context.Context, id string, updateReq *models.UserUpdateRequest, claimedUserID string) (*models.User, error) {
	if id != claimedUserID {
		return nil, apperror.NewForbiddenError("You can only update your own profile")
	}
	// Convert ID string to ObjectID
	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	// Get the complete user record including password
	existingUser, err := us.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If email is being updated, check if it's available
	if updateReq.Email != "" && updateReq.Email != existingUser.Email {
		emailUser, emailErr := us.userRepository.FindByEmail(ctx, updateReq.Email)
		if emailUser != nil {
			return nil, apperror.NewConflictError("Email already in use")
		}
		if emailErr != nil {
			var appErr apperror.AppError
			if !errors.As(emailErr, &appErr) || appErr.Code() != apperror.ErrNotFound {
				return nil, emailErr
			}
		}
	}

	// Update fields
	if updateReq.Name != "" {
		existingUser.Name = updateReq.Name
	}
	if updateReq.Email != "" {
		existingUser.Email = updateReq.Email
	}

	// Handle password update with verification
	if updateReq.NewPassword != "" {
		// Current password must be provided for password updates
		if updateReq.CurrentPassword == "" {
			return nil, apperror.NewBadRequestError("Current password required to update password")
		}

		// Verify current password
		err = bcrypt.CompareHashAndPassword([]byte(existingUser.Password), []byte(updateReq.CurrentPassword))
		if err != nil {
			return nil, apperror.NewUnauthorizedError("Current password is incorrect")
		}

		// Hash new password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(updateReq.NewPassword), 10)
		if err != nil {
			return nil, apperror.NewInternalError(err)
		}
		existingUser.Password = string(hashedPassword)
	}

	existingUser.UpdatedAt = time.Now()

	// Save the updated user
	return us.userRepository.Update(ctx, existingUser)
}

func (us *userService) DeleteUser(ctx context.Context, id string, claimedUserID string) error {
	if id != claimedUserID {
		return apperror.NewForbiddenError("You can only delete your own profile")
	}
	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewUnauthorizedError("Invalid user ID")
	}

	// Check if user exists before attempting to delete
	_, err = us.userRepository.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Check if user has any subscriptions
	subscriptions, err := us.subscriptionRepository.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if len(subscriptions) > 0 {
		return apperror.NewConflictError("User has active subscriptions and cannot be deleted")
	}

	// Delete the user
	return us.userRepository.Delete(ctx, userID)
}

func (us *userService) FetchUserByIDInternal(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	return us.userRepository.FindByID(ctx, id)
}