package services_test

import (
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// jwtCfg is the shared JWT configuration used across all JWT tests.
var jwtCfg = services.JWTConfig{
	AccessSecret:       "test-access-secret",
	RefreshSecret:      "test-refresh-secret",
	AccessExpiryHours:  1,
	RefreshExpiryHours: 24,
	Issuer:             "test-issuer",
}

// newJWTService builds a jwtService with jwtCfg and the provided nowFn so
// individual tests don't need to repeat the wiring.
func newJWTService() services.JWTService {
	return services.NewJWTService(jwtCfg, func() time.Time { return mockTime })
}

// ---------------------------------------------------------------------------
// GenerateTokens
// ---------------------------------------------------------------------------

func Test_jwtService_GenerateTokens(t *testing.T) {
	svc := newJWTService()
	got, err := svc.GenerateTokens(defaultUserHex, defaultUserEmail)

	// Assert the response
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.NotEmpty(t, got.AccessToken)
	assert.NotEmpty(t, got.RefreshToken)
	assert.NotEqual(t, got.AccessToken, got.RefreshToken,
		"access and refresh tokens must be distinct")

	expectedExpiry := mockTime.Add(time.Hour * time.Duration(jwtCfg.AccessExpiryHours))
	assert.Equal(t, expectedExpiry, got.ExpiresAt)

	// Independent Mathematical Verification (The True Unit Test)
	// We parse the generated token using the raw JWT library,
	// NOT the service's ValidateToken method
	parsedToken, err := jwt.Parse(got.AccessToken,
		func(token *jwt.Token) (any, error) {
			// Verify the service used the correct cryptographic signing algorithm
			assert.Equal(t, jwt.SigningMethodHS256, token.Method)

			// Return the hardcoded config secret to prove the token was signed with the right key
			return []byte(jwtCfg.AccessSecret), nil
		},
		// Sync the parser time with our mock time
		jwt.WithTimeFunc(func() time.Time { return mockTime }),
	)
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	// Verify the Claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, defaultUserHex, claims["userId"])
	assert.Equal(t, defaultUserEmail, claims["email"])
	assert.Equal(t, string(models.AccessToken), claims["type"])
	assert.Equal(t, jwtCfg.Issuer, claims["iss"])
}

// ---------------------------------------------------------------------------
// ValidateToken
// ---------------------------------------------------------------------------

func Test_jwtService_ValidateToken(t *testing.T) {
	type tokenParams struct {
		tokenType models.TokenType
		expiry    time.Time
		issuer    string
		secret    string
	}
	buildToken := func(p tokenParams) string {
		if p.tokenType == "" {
			p.tokenType = models.AccessToken
		}

		var expiry time.Time
		var secret string
		if p.tokenType == models.AccessToken {
			expiry = mockTime.Add(time.Hour * time.Duration(jwtCfg.AccessExpiryHours))
			secret = jwtCfg.AccessSecret
		} else {
			expiry = mockTime.Add(time.Hour * time.Duration(jwtCfg.RefreshExpiryHours))
			secret = jwtCfg.RefreshSecret
		}
		if p.expiry.IsZero() {
			p.expiry = expiry
		}
		if p.secret == "" {
			p.secret = secret
		}

		if p.issuer == "" {
			p.issuer = jwtCfg.Issuer
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.Claims{
			UserID: defaultUserHex,
			Email:  defaultUserEmail,
			Type:   p.tokenType,
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        uuid.New().String(),
				ExpiresAt: jwt.NewNumericDate(p.expiry),
				IssuedAt:  jwt.NewNumericDate(mockTime),
				NotBefore: jwt.NewNumericDate(mockTime),
				Issuer:    p.issuer,
			},
		},
		)
		tokenString, _ := token.SignedString([]byte(p.secret))
		return tokenString
	}
	validAccessToken := func() string {
		return buildToken(tokenParams{
			tokenType: models.AccessToken,
		})
	}
	validRefreshToken := func() string {
		return buildToken(tokenParams{
			tokenType: models.RefreshToken,
		})
	}

	tests := []struct {
		name       string
		inputToken string
		tokenType  models.TokenType
		wantErr    bool
	}{
		{
			// Happy path: a valid access token is accepted.
			name:       "success - valid access token",
			inputToken: validAccessToken(),
			tokenType:  models.AccessToken,
		},
		{
			// Happy path: a valid refresh token is accepted.
			name:       "success - valid refresh token",
			inputToken: validRefreshToken(),
			tokenType:  models.RefreshToken,
		},
		{
			// Supplying an access token but asking for refresh → type mismatch.
			name:       "error - wrong token type (access passed as refresh)",
			inputToken: validAccessToken(),
			tokenType:  models.RefreshToken,
			wantErr:    true,
		},
		{
			// Supplying a refresh token but asking for access → wrong secret + type mismatch.
			name:       "error - wrong token type (refresh passed as access)",
			inputToken: validRefreshToken(),
			tokenType:  models.AccessToken,
			wantErr:    true,
		},
		{
			// A completely garbage string.
			name:       "error - malformed token string",
			inputToken: "not.a.jwt",
			tokenType:  models.AccessToken,
			wantErr:    true,
		},
		{
			// Token signed with a different secret than what jwtCfg expects.
			name: "error - token signed with wrong secret",
			inputToken: buildToken(tokenParams{
				secret: "hacked-secret",
			}),
			tokenType: models.AccessToken,
			wantErr:   true,
		},
		{
			// Token was issued in the past and has already expired.
			name: "error - expired token",
			inputToken: buildToken(tokenParams{
				expiry: mockTime.Add(-1 * time.Hour),
			}),
			tokenType: models.AccessToken,
			wantErr:   true,
		},
		{
			// Token issued by a different issuer.
			name: "error - wrong issuer",
			inputToken: buildToken(tokenParams{
				issuer: "invalid-issuer",
			}),
			tokenType: models.AccessToken,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newJWTService()
			got, err := svc.ValidateToken(tt.inputToken, tt.tokenType)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, defaultUserHex, got.UserID)
			assert.Equal(t, defaultUserEmail, got.Email)
			assert.Equal(t, tt.tokenType, got.Type)
			assert.Equal(t, jwtCfg.Issuer, got.Issuer)
		})
	}
}
