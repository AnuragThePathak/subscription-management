package services

import (
	"context"
	"fmt"
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
		slog.WarnContext(ctx, "Login failed: user not found",
			slog.String("email", loginReq.Email),
		)
		return nil, apperror.NewNotFoundError("User not found")
	}

	// Verify password.
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginReq.Password)); err != nil {
		slog.WarnContext(ctx, "Login failed: invalid credentials",
			slog.String("user_id", user.ID.Hex()),
		)
		return nil, apperror.NewUnauthorizedError("Invalid credentials")
	}

	// Generate tokens.
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(err)
	}

	slog.InfoContext(ctx, "Login successful", slog.String("user_id", user.ID.Hex()))
	return tokens, nil
}

// RefreshToken validates a refresh token and issues new tokens.
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error) {
	// Validate the refresh token.
	claims, err := s.jwtService.ValidateToken(refreshToken, models.RefreshToken)
	if err != nil {
		slog.WarnContext(ctx, "Token refresh failed: invalid refresh token")
		return nil, apperror.NewUnauthorizedError("Invalid refresh token")
	}

	// Check if the user still exists.
	userID, err := bson.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID in token")
	}

	user, err := s.userServiceInternal.FetchUserByIDInternal(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Token refresh failed: user no longer exists",
			slog.String("user_id", claims.UserID),
		)
		return nil, apperror.NewUnauthorizedError("User no longer exists")
	}

	// Generate new tokens.
	tokens, err := s.jwtService.GenerateTokens(user.ID.Hex(), user.Email)
	if err != nil {
		return nil, apperror.NewInternalError(fmt.Errorf("failed to generate tokens: %w", err))
	}

	slog.DebugContext(ctx, "Token refreshed", slog.String("user_id", user.ID.Hex()))
	return tokens, nil
}
