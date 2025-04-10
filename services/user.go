package services

import (
	"context"
	"errors"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	// CreateUser creates a new user in the system.
	CreateUser(context.Context, *models.User) (*models.User, error)

	GetAllUsers(context.Context) ([]*models.User, error)

	GetUserByID(context.Context, string) (*models.User, error)

}

type userService struct {
	userRepository repositories.UserRepository
}

// NewUserService creates a new instance of UserService.
func NewUserService(userRepository repositories.UserRepository) UserService {
	return &userService{
		userRepository,
	}
}

// CreateUser creates a new user in the system.
func (us *userService) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	// Check if the user already exists
	existingUser, err := us.userRepository.FindByEmail(ctx, user.Email)
	if existingUser != nil {
		return nil, apperror.NewConflictError("Email already exists")
	}
	if err != nil {
		var appErr apperror.AppError
		if errors.As(err, &appErr) {
			if appErr.Code() != apperror.ErrNotFound {
				return nil, appErr
			}
		}
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return nil, err
	}
	user.Password = string(hashedPassword)
	
	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	
	// Set ID if not provided
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}

	// Insert into database
	return us.userRepository.Create(ctx, user)
}

func (us *userService) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	return us.userRepository.GetAll(ctx)
}

func (us *userService) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid user ID")
	}

	return us.userRepository.FindByID(ctx, userID)
}