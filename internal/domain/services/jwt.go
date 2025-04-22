package services

import (
	"fmt"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/golang-jwt/jwt/v5"
)

// JWTService handles JWT token operations.
type JWTService interface {
	GenerateTokens(userID, email string) (*models.TokenResponse, error)
	ValidateToken(tokenString string, tokenType models.TokenType) (*models.Claims, error)
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
	config JWTConfig
}

// NewJWTService creates a new JWT service instance.
func NewJWTService(config JWTConfig) JWTService {
	return &jwtService{
		config: config,
	}
}

// GenerateTokens creates both access and refresh tokens for a user.
func (s *jwtService) GenerateTokens(userID, email string) (*models.TokenResponse, error) {
	// Generate access token.
	accessExpiry := time.Now().Add(time.Hour * time.Duration(s.config.AccessExpiryHours))
	accessToken, err := s.generateToken(userID, email, models.AccessToken, accessExpiry)
	if err != nil {
		return nil, err
	}

	// Generate refresh token.
	refreshExpiry := time.Now().Add(time.Hour * time.Duration(s.config.RefreshExpiryHours))
	refreshToken, err := s.generateToken(userID, email, models.RefreshToken, refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &models.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry,
	}, nil
}

// generateToken creates a new signed JWT token.
func (s *jwtService) generateToken(userID, email string, tokenType models.TokenType, expiry time.Time) (string, error) {
	claims := models.Claims{
		UserID: userID,
		Email:  email,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Choose the appropriate secret based on token type.
	var secret string
	if tokenType == models.AccessToken {
		secret = s.config.AccessSecret
	} else {
		secret = s.config.RefreshSecret
	}

	// Sign the token with the secret.
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a token and returns the claims if valid.
func (s *jwtService) ValidateToken(tokenString string, tokenType models.TokenType) (*models.Claims, error) {
	// Choose the appropriate secret based on token type.
	var secret string
	if tokenType == models.AccessToken {
		secret = s.config.AccessSecret
	} else {
		secret = s.config.RefreshSecret
	}

	// Parse the token.
	token, err := jwt.ParseWithClaims(tokenString, &models.Claims{}, func(token *jwt.Token) (any, error) {
		// Validate the algorithm.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	// Extract and validate the claims.
	if claims, ok := token.Claims.(*models.Claims); ok && token.Valid {
		// Verify token type.
		if claims.Type != tokenType {
			return nil, fmt.Errorf("invalid token type")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
