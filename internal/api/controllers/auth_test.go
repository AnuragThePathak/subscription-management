package controllers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/controllers"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Setup Helpers
// ---------------------------------------------------------------------------

func validTokenResponse() *models.TokenResponse {
	return &models.TokenResponse{
		AccessToken:  "access.token.string",
		RefreshToken: "refresh.token.string",
		ExpiresAt:    mockTime.Add(time.Hour),
	}
}

func setupAuthController(t *testing.T) (*mocks.MockAuthService, *mocks.MockUserServiceExternal, http.Handler) {
	t.Helper()

	authSvc := mocks.NewMockAuthService(t)
	userSvc := mocks.NewMockUserServiceExternal(t)

	v := validator.New()
	reqHandler := endpoint.NewRequestHandler(v)

	router := controllers.NewAuthController(authSvc, userSvc, reqHandler)
	return authSvc, userSvc, router
}

// ---------------------------------------------------------------------------
// POST /register
// ---------------------------------------------------------------------------

func TestAuthController_CreateUser(t *testing.T) {
	validInput := func() *models.UserRequest {
		return &models.UserRequest{
			Name:     "Alice",
			Email:    defaultUserEmail,
			Password: "securepassword123",
		}
	}
	validModelFromInput := func() *models.User {
		return validInput().ToModel()
	}

	tests := []struct {
		name       string
		setupMocks func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal)
		wantStatus int
		wantUser   *models.UserResponse
	}{
		{
			name: "success - parses body, calls user service, returns 201 Created",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				// The Vault Lock: Proves UserRequest mapped to User correctly before hitting the service
				matcher := mock.MatchedBy(func(u *models.User) bool {
					return assert.ObjectsAreEqual(validModelFromInput(), u)
				})

				// Note: Register doesn't use authSvc, only userSvc
				userSvc.EXPECT().
					CreateUser(mock.Anything, matcher).
					Return(validUser(), nil). // Reusing validUser() from user_test.go helpers
					Once()
			},
			wantStatus: http.StatusCreated,
			wantUser:   validUserResponse(),
		},
		{
			name: "error - propagates service error (e.g. email already exists)",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				userSvc.EXPECT().
					CreateUser(mock.Anything, mock.Anything).
					Return(nil, apperror.NewConflictError("email already in use")).
					Once()
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc, userSvc, handler := setupAuthController(t)
			tt.setupMocks(authSvc, userSvc)

			inputBytes, err := json.Marshal(validInput())
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(inputBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantUser != nil {
				var resp *models.UserResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// POST /login
// ---------------------------------------------------------------------------

func TestAuthController_Login(t *testing.T) {
	validInput := func() models.LoginRequest {
		return models.LoginRequest{
			Email:    defaultUserEmail,
			Password: "securepassword123",
		}
	}

	tests := []struct {
		name       string
		setupMocks func(
			authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal,
		)
		wantStatus int
		wantTokens *models.TokenResponse
	}{
		{
			name: "success - parses body, calls auth service, returns 200 OK",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				// We pass the exact dereferenced struct to match the value sent from the controller
				authSvc.EXPECT().
					Login(mock.Anything, validInput()).
					Return(validTokenResponse(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantTokens: validTokenResponse(),
		},
		{
			name: "error - propagates service error",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				authSvc.EXPECT().
					Login(mock.Anything, validInput()).
					Return(nil, apperror.NewUnauthorizedError("unauthorized")).
					Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc, userSvc, handler := setupAuthController(t)
			tt.setupMocks(authSvc, userSvc)

			inputBytes, err := json.Marshal(validInput())
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(inputBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantTokens != nil {
				var resp *models.TokenResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantTokens, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// POST /refresh
// ---------------------------------------------------------------------------

func TestAuthController_RefreshToken(t *testing.T) {
	validInput := func() models.RefreshRequest {
		return models.RefreshRequest{
			RefreshToken: "old.refresh.token",
		}
	}

	tests := []struct {
		name       string
		setupMocks func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal)
		wantStatus int
		wantTokens *models.TokenResponse
	}{
		{
			name: "success - parses body, calls auth service, returns 200 OK",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				authSvc.EXPECT().
					RefreshToken(mock.Anything, validInput().RefreshToken).
					Return(validTokenResponse(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantTokens: validTokenResponse(),
		},
		{
			name: "error - propagates service error for expired refresh token",
			setupMocks: func(authSvc *mocks.MockAuthService, userSvc *mocks.MockUserServiceExternal) {
				authSvc.EXPECT().
					RefreshToken(mock.Anything, validInput().RefreshToken).
					Return(nil, apperror.NewUnauthorizedError("refresh token expired")).
					Once()
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc, userSvc, handler := setupAuthController(t)
			tt.setupMocks(authSvc, userSvc)

			inputBytes, err := json.Marshal(validInput())
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(inputBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantTokens != nil {
				var resp *models.TokenResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantTokens, resp)
			}
		})
	}
}
