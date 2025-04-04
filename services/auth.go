package services

import (
	"context"
	"fmt"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

// AuthService provides authentication operations
type AuthService interface {
	Login(ctx context.Context, loginReq models.LoginRequest) (*models.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error)
}

type authService struct {
	userRepository repositories.UserRepository
	jwtService     JWTService
}

// NewAuthService creates a new instance of AuthService
func NewAuthService(userRepository repositories.UserRepository, jwtService JWTService) AuthService {
	return &authService{
		userRepository: userRepository,
		jwtService:     jwtService,
	}
}

// Login authenticates a user and returns JWT tokens
func (s *authService) Login(ctx context.Context, loginReq models.LoginRequest) (*models.TokenResponse, error) {
	// Find the user by email
	user, err := s.userRepository.FindByEmail(ctx, loginReq.Email)
	if err != nil {
		return nil, apperror.NewNotFoundError("User not found")
	}

	// Verify password
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password)); err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid credentials")
	}

	// Generate tokens
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(err)
	}

	return tokens, nil
}

// RefreshToken validates a refresh token and issues new tokens
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error) {
	// First, validate the refresh token
	claims, err := s.jwtService.ValidateToken(refreshToken, models.RefreshToken)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid refresh token")
	}

	// Check if the user still exists
	userID, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID in token")
	}

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("User no longer exists")
	}

	// Generate new tokens
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(fmt.Errorf("failed to generate tokens: %w", err))
	}

	return tokens, nil
}