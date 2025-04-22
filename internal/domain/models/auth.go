package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType represents the type of JWT token.
type TokenType string

const (
	AccessToken  TokenType = "access"  // Used for API authorization.
	RefreshToken TokenType = "refresh" // Used to obtain new access tokens.
)

// Claims represents the JWT claims structure.
type Claims struct {
	UserID string    `json:"userId"`
	Email  string    `json:"email"`
	Type   TokenType `json:"type"`
	jwt.RegisteredClaims
}

// TokenResponse is returned after successful authentication.
type TokenResponse struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// LoginRequest represents user login credentials.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
