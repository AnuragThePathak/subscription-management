package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

// AuthService provides authentication operations.
type AuthService interface {
	Login(ctx context.Context, loginReq models.LoginRequest) (*models.TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error)
}

type authService struct {
	userServiceInternal UserServiceInternal
	jwtService          JWTService
}

// NewAuthService creates a new instance of AuthService.
func NewAuthService(userServiceInternal UserServiceInternal, jwtService JWTService) AuthService {
	return &authService{
		userServiceInternal: userServiceInternal,
		jwtService:          jwtService,
	}
}

// Login authenticates a user and returns JWT tokens.
func (s *authService) Login(ctx context.Context, loginReq models.LoginRequest) (*models.TokenResponse, error) {
	// Find the user by email.
	user, err := s.userServiceInternal.FetchUserByEmailInternal(ctx, loginReq.Email)
	if err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			return nil, appErr.WithMetadata(apperror.KeyAttemptedID, loginReq.Email)
		} else {
			return nil, err
		}
	}

	// Verify password.
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password)); err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid credentials").
			WithMetadata(apperror.KeyAttemptedID, loginReq.Email)
	}

	// Generate tokens.
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(err).
			WithMetadata(apperror.KeyUserID, user.ID.Hex())
	}

	slog.InfoContext(ctx, "Login successful", slog.String("user_id", user.ID.Hex()))
	return tokens, nil
}

// RefreshToken validates a refresh token and issues new tokens.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error) {
	// Validate the refresh token.
	claims, err := s.jwtService.ValidateToken(refreshToken, models.RefreshToken)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid refresh token")
	}

	// Check if the user still exists.
	userID, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID in token")
	}

	user, err := s.userServiceInternal.FetchUserByIDInternal(ctx, userID)
	if err != nil {
		if appErr, ok := errors.AsType[apperror.AppError](err); ok {
			return nil, appErr.WithMetadata(apperror.KeyUserID, claims.UserID)
		} else {
			return nil, err
		}
	}

	// Generate new tokens.
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(err).
			WithMetadata(apperror.KeyUserID, user.ID.Hex())
	}

	slog.DebugContext(ctx, "Token refreshed", slog.String("user_id", user.ID.Hex()))
	return tokens, nil
}
