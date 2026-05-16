package controllers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/api/controllers"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/api/shared/endpoint"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	svcmocks "github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Setup Helpers
// ---------------------------------------------------------------------------

// validUser returns a fully hydrated user struct as it would appear in the DB.
// It has a populated ID, timestamps, and a dummy hashed password.
func validUser() *models.User {
	return &models.User{
		ID:        defaultUserID,
		Name:      "Alice",
		Email:     defaultUserEmail,
		Password:  "hashed-password",
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

func validUserResponse() *models.UserResponse {
	return validUser().ToResponse()
}

func setupUserController(t *testing.T) (*mocks.MockUserServiceExternal, http.Handler) {
	t.Helper()

	svc := mocks.NewMockUserServiceExternal(t)
	v := validator.New()
	reqHandler := endpoint.NewRequestHandler(v)
	router := controllers.NewUserController(svc, reqHandler)
	return svc, router
}

// ---------------------------------------------------------------------------
// GET /
// ---------------------------------------------------------------------------

func TestUserController_GetAllUsers(t *testing.T) {
	validUsers := func() []*models.User {
		return []*models.User{
			validUser(),
			validUser(),
		}
	}

	tests := []struct {
		name       string
		setupMocks func(svc *svcmocks.MockUserServiceExternal)
		wantStatus int
		wantUsers  []*models.UserResponse
	}{
		{
			name: "success - calls service and returns 200 OK",
			setupMocks: func(svc *mocks.MockUserServiceExternal) {
				svc.EXPECT().
					GetAllUsers(mock.Anything).
					Return(validUsers(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantUsers:  []*models.UserResponse{validUserResponse(), validUserResponse()},
		},
		{
			name: "Success - empty list and returns 200 OK",
			setupMocks: func(svc *svcmocks.MockUserServiceExternal) {
				svc.EXPECT().GetAllUsers(mock.Anything).Return(nil, nil).Once()
			},
			wantStatus: http.StatusOK,
			wantUsers:  []*models.UserResponse{},
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *svcmocks.MockUserServiceExternal) {
				svc.EXPECT().
					GetAllUsers(mock.Anything).
					Return(nil, apperror.NewDBError(nil)).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, handler := setupUserController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)
			// Assert Wiring
			require.Equal(t, tt.wantStatus, rr.Code)

			// Assert Payload Delivery
			if tt.wantUsers != nil {
				var resp []*models.UserResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.wantUsers, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GET /{id}
// ---------------------------------------------------------------------------

func TestUserController_GetUserByID(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *svcmocks.MockUserServiceExternal)
		wantStatus int
		wantUser   *models.UserResponse
	}{
		{
			name: "success - parses URL param and context, calls service",
			setupMocks: func(svc *svcmocks.MockUserServiceExternal) {
				svc.EXPECT().
					GetUserByID(mock.Anything, defaultUserHex, defaultUserHex).
					Return(validUser(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantUser: validUserResponse(),
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockUserServiceExternal) {
				svc.EXPECT().
					GetUserByID(mock.Anything, defaultUserHex, defaultUserHex).
					Return(nil, errors.New("not found")).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usedID := defaultUserHex
			svc, handler := setupUserController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodGet, "/"+usedID, nil)
			req = injectUserID(req, usedID)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)
			// Assert Wiring
			require.Equal(t, tt.wantStatus, rr.Code)

			// Assert Payload Delivery
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
// DELETE /{id}
// ---------------------------------------------------------------------------

func TestUserController_DeleteUser(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *svcmocks.MockUserServiceExternal)
		wantStatus int
	}{
		{
			name: "success - calls service and returns 204 No Content",
			setupMocks: func(svc *mocks.MockUserServiceExternal) {
				svc.EXPECT().
					DeleteUser(mock.Anything, defaultUserHex, defaultUserHex).
					Return(nil).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockUserServiceExternal) {
				svc.EXPECT().
					DeleteUser(mock.Anything, defaultUserHex, defaultUserHex).
					Return(errors.New("something went wrong")).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usedID := defaultUserHex
			svc, handler := setupUserController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodDelete, "/"+usedID, nil)
			req = injectUserID(req, usedID)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)
			// Assert Wiring
			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantStatus == http.StatusNoContent {
				assert.Empty(t, rr.Body)
			}
		})
	}
}
