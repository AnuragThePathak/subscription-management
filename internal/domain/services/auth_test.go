package services_test

import (
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	svcmocks "github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validTokenResp() *models.TokenResponse {
	return &models.TokenResponse{
		AccessToken:  "access.token.string",
		RefreshToken: "refresh.token.string",
		ExpiresAt:    mockTime.Add(time.Hour),
	}
}

// newAuthService is a convenience constructor that wires up an authService
// with the provided mocks so individual tests don't need to repeat the wiring.
func newAuthService(
	userSvc *svcmocks.MockUserServiceInternal,
	jwtSvc *svcmocks.MockJWTService,
) services.AuthService {
	return services.NewAuthService(userSvc, jwtSvc)
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func Test_authService_Login(t *testing.T) {
	plainPassword := "correctpassword"
	hashBytes, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.MinCost)
	hashedPassword := string(hashBytes)

	validInput := func() models.LoginRequest {
		return models.LoginRequest{
			Email:    defaultUserEmail,
			Password: plainPassword,
		}
	}
	validUser := func() *models.User {
		return &models.User{
			ID:       defaultUserID,
			Email:    defaultUserEmail,
			Password: hashedPassword,
		}
	}

	tests := []struct {
		name       string
		input      models.LoginRequest
		setupMocks func(
			userSvc *svcmocks.MockUserServiceInternal,
			jwtSvc *svcmocks.MockJWTService,
			input models.LoginRequest,
		)
		wantErr         bool
		wantErrCode     apperror.ErrorCode
		wantEnrichedErr bool
		wantResp        *models.TokenResponse
	}{
		{
			// Happy path: credentials match and tokens are issued.
			name:     "success - valid credentials",
			input:    validInput(),
			wantResp: validTokenResp(),
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				input models.LoginRequest,
			) {
				userSvc.EXPECT().
					FetchUserByEmailInternal(mock.Anything, input.Email).
					Return(validUser(), nil).
					Once()

				jwtSvc.EXPECT().
					GenerateTokens(defaultUserHex, input.Email).
					Return(validTokenResp(), nil).
					Once()
			},
		},
		{
			// User not found in the repository.
			name:  "error - user not found",
			input: validInput(),
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				input models.LoginRequest,
			) {
				userSvc.EXPECT().
					FetchUserByEmailInternal(mock.Anything, input.Email).
					Return(nil, apperror.NewNotFoundError("user not found")).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrNotFound,
			wantEnrichedErr: true,
		},
		{
			// User service returns a raw (non-AppError) error.
			name:  "error - user service returns unexpected error",
			input: validInput(),
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				input models.LoginRequest,
			) {
				userSvc.EXPECT().
					FetchUserByEmailInternal(mock.Anything, input.Email).
					Return(nil, errors.New("db unreachable")).
					Once()
			},
			wantErr: true,
		},
		{
			// Correct email, wrong password → unauthorized.
			name: "error - wrong password",
			input: func() models.LoginRequest {
				req := validInput()
				req.Password = "wrongpassword"
				return req
			}(),
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				input models.LoginRequest,
			) {
				userSvc.EXPECT().
					FetchUserByEmailInternal(mock.Anything, input.Email).
					Return(validUser(), nil).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrUnauthorized,
			wantEnrichedErr: true,
		},
		{
			// Password matches but JWT signing fails.
			name:  "error - token generation fails",
			input: validInput(),
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				input models.LoginRequest,
			) {
				userSvc.EXPECT().
					FetchUserByEmailInternal(mock.Anything, input.Email).
					Return(validUser(), nil).
					Once()

				jwtSvc.EXPECT().
					GenerateTokens(defaultUserHex, input.Email).
					Return(nil, errors.New("signing failed")).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrInternal,
			wantEnrichedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := svcmocks.NewMockUserServiceInternal(t)
			jwtSvc := svcmocks.NewMockJWTService(t)
			tt.setupMocks(userSvc, jwtSvc, tt.input)

			svc := newAuthService(userSvc, jwtSvc)
			got, err := svc.Login(t.Context(), tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
					if tt.wantEnrichedErr {
						assert.NotEmpty(t, appErr.LogAttributes(),
							"expected error to be enriched with log attributes",
						)
					}
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResp, got)
		})
	}
}

// ---------------------------------------------------------------------------
// RefreshToken
// ---------------------------------------------------------------------------

func Test_authService_RefreshToken(t *testing.T) {
	refreshToken := "some.refresh.jwt"

	validClaims := func() *models.Claims {
		return &models.Claims{
			UserID: defaultUserHex,
			Email:  defaultUserEmail,
			Type:   models.RefreshToken,
		}
	}
	validUser := func() *models.User {
		return &models.User{
			ID:    defaultUserID,
			Email: defaultUserEmail,
		}
	}

	tests := []struct {
		name         string
		refreshToken string
		setupMocks   func(
			userSvc *svcmocks.MockUserServiceInternal,
			jwtSvc *svcmocks.MockJWTService,
			refreshToken string,
		)
		wantErr         bool
		wantErrCode     apperror.ErrorCode
		wantEnrichedErr bool
		wantResp        *models.TokenResponse
	}{
		{
			// Happy path: valid token, user still exists, new tokens issued.
			name:         "success - valid refresh token",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(validClaims(), nil).
					Once()

				userSvc.EXPECT().
					FetchUserByIDInternal(mock.Anything, defaultUserID).
					Return(validUser(), nil).
					Once()

				jwtSvc.EXPECT().
					GenerateTokens(defaultUserHex, defaultUserEmail).
					Return(validTokenResp(), nil).
					Once()
			},
			wantResp: validTokenResp(),
		},
		{
			// The refresh token itself is invalid (expired, bad signature, etc.).
			name:         "error - invalid refresh token",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(nil, errors.New("token expired")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrUnauthorized,
		},
		{
			// Token validates but the embedded UserID is not a valid ObjectID hex.
			name:         "error - invalid user ID in token claims",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				claims := validClaims()
				claims.UserID = "not-a-valid-hex"
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(claims, nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrUnauthorized,
		},
		{
			// Token is valid, but the user no longer exists.
			name:         "error - user not found after token validation",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(validClaims(), nil).
					Once()
				userSvc.EXPECT().
					FetchUserByIDInternal(mock.Anything, defaultUserID).
					Return(nil, apperror.NewNotFoundError("user not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
			wantEnrichedErr: true,
		},
		{
			// User lookup returns a raw (non-AppError) error.
			name:         "error - user service returns unexpected error",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(validClaims(), nil).
					Once()

				userSvc.EXPECT().
					FetchUserByIDInternal(mock.Anything, defaultUserID).
					Return(nil, errors.New("connection reset")).
					Once()
			},
			wantErr: true,
		},
		{
			// User still exists, but new token generation fails.
			name:         "error - token generation fails",
			refreshToken: refreshToken,
			setupMocks: func(
				userSvc *svcmocks.MockUserServiceInternal,
				jwtSvc *svcmocks.MockJWTService,
				refreshToken string,
			) {
				jwtSvc.EXPECT().
					ValidateToken(refreshToken, models.RefreshToken).
					Return(validClaims(), nil).
					Once()

				userSvc.EXPECT().
					FetchUserByIDInternal(mock.Anything, defaultUserID).
					Return(validUser(), nil).
					Once()

				jwtSvc.EXPECT().
					GenerateTokens(defaultUserHex, defaultUserEmail).
					Return(nil, errors.New("signing error")).
					Once()
			},
			wantErr:         true,
			wantErrCode:     apperror.ErrInternal,
			wantEnrichedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userSvc := svcmocks.NewMockUserServiceInternal(t)
			jwtSvc := svcmocks.NewMockJWTService(t)
			tt.setupMocks(userSvc, jwtSvc, tt.refreshToken)

			svc := newAuthService(userSvc, jwtSvc)
			got, err := svc.RefreshToken(t.Context(), tt.refreshToken)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
					if tt.wantEnrichedErr {
						assert.NotEmpty(t, appErr.LogAttributes(),
							"expected error to be enriched with log attributes",
						)
					}
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResp, got)
		})
	}
}
