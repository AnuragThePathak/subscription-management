package services

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/core/clock"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/golang-jwt/jwt/v5"
)

//go:generate mockery
// JWTService handles JWT token operations.
type JWTService interface {
	GenerateTokens(userID, email string) (*models.TokenResponse, error)
	ValidateToken(
		tokenString string,
		tokenType models.TokenType,
	) (*models.Claims, error)
}

// JWTConfig holds the JWT token generation and validation settings.
type JWTConfig struct {
	AccessSecret       string `mapstructure:"access_secret"`
	RefreshSecret      string `mapstructure:"refresh_secret"`
	AccessExpiryHours  int    `mapstructure:"access_timeout"`
	RefreshExpiryHours int    `mapstructure:"refresh_timeout"`
	Issuer             string `mapstructure:"issuer"`
}

type jwtService struct {
	config  JWTConfig
	getTime clock.NowFn
}

// NewJWTService creates a new JWT service instance.
func NewJWTService(config JWTConfig, nowFn clock.NowFn) JWTService {
	slog.Info("JWT service created",
		logattr.Issuer(config.Issuer),
		logattr.AccessExpiryHours(config.AccessExpiryHours),
		logattr.RefreshExpiryHours(config.RefreshExpiryHours),
	)

	return &jwtService{
		config:  config,
		getTime: nowFn,
	}
}

// getSecret returns the appropriate secret based on the token type.
func (s *jwtService) getSecret(tokenType models.TokenType) string {
	if tokenType == models.AccessToken {
		return s.config.AccessSecret
	}
	return s.config.RefreshSecret
}

// generateToken creates a new signed JWT token.
func (s *jwtService) generateToken(
	userID,
	email string,
	tokenType models.TokenType,
	expiry time.Time,
) (string, error) {
	now := s.getTime()
	claims := models.Claims{
		UserID: userID,
		Email:  email,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := s.getSecret(tokenType)
	// Sign the token with the secret.
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GenerateTokens creates both access and refresh tokens for a user.
func (s *jwtService) GenerateTokens(userID, email string) (*models.TokenResponse, error) {
	// Generate access token.
	accessExpiry := s.getTime().Add(time.Hour * time.Duration(s.config.AccessExpiryHours))
	accessToken, err := s.generateToken(
		userID,
		email,
		models.AccessToken,
		accessExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token.
	refreshExpiry := s.getTime().Add(time.Hour * time.Duration(s.config.RefreshExpiryHours))
	refreshToken, err := s.generateToken(
		userID,
		email,
		models.RefreshToken,
		refreshExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &models.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry,
	}, nil
}

// ValidateToken validates a token and returns the claims if valid.
func (s *jwtService) ValidateToken(tokenString string, tokenType models.TokenType) (*models.Claims, error) {
	// Choose the appropriate secret based on token type.
	secret := s.getSecret(tokenType)
	// Parse the token.
	token, err := jwt.ParseWithClaims(tokenString, &models.Claims{},
		func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithIssuer(s.config.Issuer),
	)
	if err != nil {
		return nil, err
	}

	// Extract and validate the claims.
	claims, ok := token.Claims.(*models.Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	// Verify token type.
	if claims.Type != tokenType {
		return nil, fmt.Errorf(
			"invalid token type: expected %s, got %s",
			tokenType,
			claims.Type,
		)
	}

	return claims, nil
}
