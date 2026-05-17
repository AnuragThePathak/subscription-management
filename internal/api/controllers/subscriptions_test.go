package controllers_test

import (
	"bytes"
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
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---------------------------------------------------------------------------
// Setup Helpers
// ---------------------------------------------------------------------------

// defaultSubID is a stable, deterministic ObjectID used across all tests.
var defaultSubID = bson.NewObjectID()
var defaultSubHex = defaultSubID.Hex()

// validSub returns a minimal Subscription that passes Validate().
func validSub() *models.Subscription {
	return &models.Subscription{
		ID:        defaultSubID,
		Name:      "Netflix",
		Price:     999,
		Currency:  models.USD,
		Frequency: models.Monthly,
		Category:  models.Entertainment,
		Status:    models.Active,
		ValidTill: mockTime,
		UserID:    defaultUserID,
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

func validSubResponse() *models.SubscriptionResponse {
	return validSub().ToResponse()
}

var sub2ID = bson.NewObjectID()

// validSubs returns a slice of two distinct subscriptions.
func validSubs() []*models.Subscription {
	sub1 := validSub()
	sub2 := validSub()
	sub2.ID = sub2ID
	sub2.Name = "Spotify"
	return []*models.Subscription{sub1, sub2}
}

func validSubsResponse() []*models.SubscriptionResponse {
	res, _ := endpoint.ToResponseSlice(validSubs(), nil)
	return res
}

func setupSubscriptionController(t *testing.T) (*mocks.MockSubscriptionServiceExternal, http.Handler) {
	t.Helper()

	svc := mocks.NewMockSubscriptionServiceExternal(t)
	v := validator.New()
	reqHandler := endpoint.NewRequestHandler(v)
	router := controllers.NewSubscriptionController(svc, reqHandler)
	return svc, router
}

// ---------------------------------------------------------------------------
// POST /
// ---------------------------------------------------------------------------

func TestSubscriptionController_CreateSubscription(t *testing.T) {
	validInput := func() *models.SubscriptionRequest {
		return &models.SubscriptionRequest{
			Name:      "Netflix",
			Price:     999,
			Frequency: models.Monthly,
			Category:  models.Entertainment,
		}
	}
	validModelFromInput := func() *models.Subscription {
		return validInput().ToModel()
	}

	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
		wantSub    *models.SubscriptionResponse
	}{
		{
			name: "success - parses body and context, calls service, returns 201 Created",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				matcher := mock.MatchedBy(func(s *models.Subscription) bool {
					return assert.ObjectsAreEqual(validModelFromInput(), s)
				})

				svc.EXPECT().
					CreateSubscription(mock.Anything, matcher, defaultUserHex).
					Return(validSub(), nil).
					Once()
			},
			wantStatus: http.StatusCreated,
			wantSub:    validSubResponse(),
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					CreateSubscription(mock.Anything, mock.Anything, defaultUserHex).
					Return(nil, apperror.NewInternalError(errors.New("db down"))).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			inputBytes, err := json.Marshal(validInput())
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(inputBytes))
			req.Header.Set("Content-Type", "application/json")
			req = injectUserID(req, defaultUserHex)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantSub != nil {
				var resp *models.SubscriptionResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantSub, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GET /
// ---------------------------------------------------------------------------

func TestSubscriptionController_GetAllSubscriptions(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
		wantSubs   []*models.SubscriptionResponse
	}{
		{
			name: "success - calls service and returns 200 OK",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetAllSubscriptions(mock.Anything).
					Return(validSubs(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSubs:   validSubsResponse(),
		},
		{
			name: "Success - empty list and returns 200 OK",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetAllSubscriptions(mock.Anything).
					Return(nil, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSubs:   []*models.SubscriptionResponse{},
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().GetAllSubscriptions(mock.Anything).Return(nil, errors.New("db error")).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantSubs != nil {
				var resp []*models.SubscriptionResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.wantSubs, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GET /user/{id}
// ---------------------------------------------------------------------------

func TestSubscriptionController_GetSubscriptionsByUserID(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
		wantSubs   []*models.SubscriptionResponse
	}{
		{
			name: "success - parses URL param and context, calls service",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetSubscriptionsByUserID(mock.Anything, defaultUserHex, defaultUserHex).
					Return(validSubs(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSubs:   validSubsResponse(),
		},
		{
			name: "Success - empty list and returns 200 OK",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetSubscriptionsByUserID(mock.Anything, defaultUserHex, defaultUserHex).
					Return(nil, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSubs:   []*models.SubscriptionResponse{},
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().GetSubscriptionsByUserID(mock.Anything, defaultUserHex, defaultUserHex).Return(nil, errors.New("db error")).Once()
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := defaultUserHex
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodGet, "/user/"+userID, nil)
			req = injectUserID(req, userID)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantSubs != nil {
				var resp []*models.SubscriptionResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.wantSubs, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GET /{subscriptionID}
// ---------------------------------------------------------------------------

func TestSubscriptionController_GetSubscriptionByID(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
		wantSub    *models.SubscriptionResponse
	}{
		{
			name: "success - extracts ID via middleware, context via auth, calls service",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetSubscriptionByID(mock.Anything, defaultSubHex, defaultUserHex).
					Return(validSub(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSub:    validSubResponse(),
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					GetSubscriptionByID(mock.Anything, defaultSubHex, defaultUserHex).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subID := defaultSubHex
			userID := defaultUserHex
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodGet, "/"+subID, nil)
			req = injectUserID(req, userID)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantSub != nil {
				var resp *models.SubscriptionResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantSub, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PUT /{subscriptionID}/cancel
// ---------------------------------------------------------------------------

func TestSubscriptionController_CancelSubscription(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
		wantSub    *models.SubscriptionResponse
	}{
		{
			name: "success - extracts ID via middleware, context via auth, calls service",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					CancelSubscription(mock.Anything, defaultSubHex, defaultUserHex).
					Return(validSub(), nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantSub:    validSubResponse(),
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					CancelSubscription(mock.Anything, defaultSubHex, defaultUserHex).
					Return(nil, apperror.NewConflictError("already canceled")).
					Once()
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subID := defaultSubHex
			userID := defaultUserHex
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodPut, "/"+subID+"/cancel", nil)
			req = injectUserID(req, userID)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantSub != nil {
				var resp *models.SubscriptionResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, tt.wantSub, resp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DELETE /{subscriptionID}
// ---------------------------------------------------------------------------

func TestSubscriptionController_DeleteSubscription(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(svc *mocks.MockSubscriptionServiceExternal)
		wantStatus int
	}{
		{
			name: "success - calls service and returns 204 No Content",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					DeleteSubscription(mock.Anything, defaultSubHex, defaultUserHex).
					Return(nil).
					Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "error - propagates service error",
			setupMocks: func(svc *mocks.MockSubscriptionServiceExternal) {
				svc.EXPECT().
					DeleteSubscription(mock.Anything, defaultSubHex, defaultUserHex).
					Return(apperror.NewConflictError("not expired")).Once()
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, handler := setupSubscriptionController(t)
			tt.setupMocks(svc)

			req := httptest.NewRequest(http.MethodDelete, "/"+defaultSubHex, nil)
			req = injectUserID(req, defaultUserHex)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			require.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantStatus == http.StatusNoContent {
				assert.Empty(t, rr.Body)
			}
		})
	}
}
