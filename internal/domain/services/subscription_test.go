package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	repomocks "github.com/anuragthepathak/subscription-management/internal/domain/repositories/mocks"
	"github.com/anuragthepathak/subscription-management/internal/domain/services"
	svcmocks "github.com/anuragthepathak/subscription-management/internal/domain/services/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockToday represents the start of the day for mockTime.
var mockToday = time.Date(
	mockTime.Year(),
	mockTime.Month(),
	mockTime.Day(),
	0,
	0,
	0,
	0,
	mockTime.Location(),
)

// mockTimeOneMonthLater is a time one month after mockToday.
var mockTimeOneMonthLater = mockToday.AddDate(0, 1, 0)
var mockTimeTwoMonthsLater = mockToday.AddDate(0, 2, 0)

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
		ValidTill: mockTimeOneMonthLater,
		UserID:    defaultUserID,
		CreatedAt: mockTime,
		UpdatedAt: mockTime,
	}
}

// validExpiredSub returns a subscription with models.Expired status.
func validExpiredSub() *models.Subscription {
	sub := validSub()
	sub.Status = models.Expired
	sub.ValidTill = mockToday
	return sub
}

// validCanceledSub returns a subscription with models.Canceled status.
func validCanceledSub() *models.Subscription {
	sub := validSub()
	sub.Status = models.Canceled
	return sub
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

// A bill whose StartDate is BEFORE time.Now() → no refund
func validBill() *models.Bill {
	return &models.Bill{
		ID:             bson.NewObjectID(),
		Amount:         999,
		Currency:       models.USD,
		SubscriptionID: defaultSubID,
		StartDate:      mockToday,
		EndDate:        mockTimeOneMonthLater,
		Status:         models.Paid,
		CreatedAt:      mockTime,
		UpdatedAt:      mockTime,
	}
}

// noopTxnFn is a TxnFn that immediately executes fn without a real DB
// transaction.  This keeps tests fast and dependency-free.
func noopTxnFn(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// newSubService builds a subscriptionService wired with the provided mocks.
func newSubService(
	subRepo *repomocks.MockSubscriptionRepository,
	billRepo *repomocks.MockBillRepository,
	metrics *svcmocks.MockSubscriptionMetrics,
) services.SubscriptionService {
	return services.NewSubscriptionService(
		noopTxnFn,
		subRepo,
		billRepo,
		metrics,
		func() time.Time { return mockTime },
	)
}

// ---------------------------------------------------------------------------
// CreateSubscription
// ---------------------------------------------------------------------------

func Test_subscriptionService_CreateSubscription(t *testing.T) {
	validInput := func() *models.Subscription {
		return &models.Subscription{
			Name:      "Netflix",
			Price:     999,
			Currency:  models.USD,
			Frequency: models.Monthly,
			Category:  models.Entertainment,
		}
	}
	buildMatcher := func(input models.Subscription, userID bson.ObjectID) any {
		return mock.MatchedBy(func(s *models.Subscription) bool {
			isStaticValid := s.Name == input.Name &&
				s.Price == input.Price &&
				s.Currency == input.Currency &&
				s.Frequency == input.Frequency &&
				s.Category == input.Category &&
				s.Status == models.Active &&
				s.ValidTill.Equal(mockTimeOneMonthLater) &&
				s.UserID == userID &&
				s.CreatedAt.Equal(mockTime) &&
				s.UpdatedAt.Equal(mockTime)

			isDynamicValid := s.ID != bson.NilObjectID

			return isStaticValid && isDynamicValid
		})
	}
	buildBillMatcher := func(input models.Subscription) any {
		return mock.MatchedBy(func(b *models.Bill) bool {
			isStaticValid := b.Amount == input.Price &&
				b.Currency == input.Currency &&
				b.StartDate.Equal(mockToday) &&
				b.EndDate.Equal(mockTimeOneMonthLater) &&
				b.Status == models.Paid &&
				b.CreatedAt.Equal(mockTime) &&
				b.UpdatedAt.Equal(mockTime)

			isDynamicValid := b.ID != bson.NilObjectID &&
				b.SubscriptionID != bson.NilObjectID

			return isStaticValid && isDynamicValid
		})
	}

	tests := []struct {
		name          string
		input         *models.Subscription
		claimedUserID string
		parsedUserID  bson.ObjectID
		setupMocks    func(
			subRepo *repomocks.MockSubscriptionRepository,
			billRepo *repomocks.MockBillRepository,
			metrics *svcmocks.MockSubscriptionMetrics,
			input models.Subscription,
			userID bson.ObjectID,
		)
		wantErr      bool
		wantErrCode  apperror.ErrorCode
		assertResult func(
			t *testing.T,
			input models.Subscription,
			got *models.Subscription,
			userID bson.ObjectID,
		)
	}{
		{
			// Happy path - subscription and bill created
			name:          "success - subscription and bill created",
			input:         validInput(),
			claimedUserID: defaultUserHex,
			parsedUserID:  defaultUserID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				metrics *svcmocks.MockSubscriptionMetrics,
				input models.Subscription,
				userID bson.ObjectID,
			) {
				billRepo.EXPECT().
					Create(mock.Anything, buildBillMatcher(input)).
					RunAndReturn(func(_ context.Context, b *models.Bill) (*models.Bill, error) {
						return b, nil
					}).Once()

				subRepo.EXPECT().
					Create(mock.Anything, buildMatcher(input, userID)).
					RunAndReturn(func(_ context.Context, s *models.Subscription) (*models.Subscription, error) {
						return s, nil
					}).Once()

				metrics.EXPECT().IncSubscriptionsCreated(mock.Anything).Once()
			},
			assertResult: func(
				t *testing.T,
				input models.Subscription,
				got *models.Subscription,
				userID bson.ObjectID,
			) {
				t.Helper()
				assert.NotZero(t, got.ID)
				assert.Equal(t, input.Name, got.Name)
				assert.Equal(t, input.Price, got.Price)
				assert.Equal(t, input.Currency, got.Currency)
				assert.Equal(t, input.Frequency, got.Frequency)
				assert.Equal(t, input.Category, got.Category)
				assert.Equal(t, models.Active, got.Status)
				assert.Equal(t, mockTimeOneMonthLater, got.ValidTill)
				assert.Equal(t, userID, got.UserID)
				assert.Equal(t, mockTime, got.CreatedAt)
				assert.Equal(t, mockTime, got.UpdatedAt)
			},
		},
		{
			// claimedUserID is not a valid ObjectID hex.
			name:          "error - invalid claimed user ID",
			input:         validInput(),
			claimedUserID: "not-valid-hex",
			setupMocks: func(
				_ *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				_ models.Subscription,
				_ bson.ObjectID,
			) {
			},
			wantErr:     true,
			wantErrCode: apperror.ErrUnauthorized,
		},
		{
			// Name is too short → Validate() fails.
			name:          "error - subscription validation fails (short name)",
			claimedUserID: defaultUserHex,
			input: func() *models.Subscription {
				s := validInput()
				s.Name = "X" // < 2 chars
				return s
			}(),
			setupMocks: func(
				_ *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				_ models.Subscription,
				_ bson.ObjectID,
			) {
			},
			wantErr:     true,
			wantErrCode: apperror.ErrValidation,
		},
		{
			// Bill repository fails inside the transaction.
			name:          "error - bill repository Create fails",
			input:         validInput(),
			claimedUserID: defaultUserHex,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				metrics *svcmocks.MockSubscriptionMetrics,
				input models.Subscription,
				_ bson.ObjectID,
			) {
				billRepo.EXPECT().
					Create(mock.Anything, buildBillMatcher(input)).
					Return(nil, apperror.NewDBError(errors.New("insert failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// Subscription repository fails after bill is created.
			name:          "error - subscription repository Create fails",
			input:         validInput(),
			claimedUserID: defaultUserHex,
			parsedUserID:  defaultUserID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				metrics *svcmocks.MockSubscriptionMetrics,
				input models.Subscription,
				userID bson.ObjectID,
			) {
				billRepo.EXPECT().
					Create(mock.Anything, buildBillMatcher(input)).
					RunAndReturn(func(_ context.Context, b *models.Bill) (*models.Bill, error) { return b, nil }).Once()

				subRepo.EXPECT().
					Create(mock.Anything, buildMatcher(input, userID)).
					Return(nil, apperror.NewDBError(errors.New("subscription insert failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)

			var inputSnapshot models.Subscription
			if tt.input != nil {
				inputSnapshot = *tt.input
			}
			tt.setupMocks(subRepo, billRepo, metrics, inputSnapshot, tt.parsedUserID)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.CreateSubscription(
				t.Context(), tt.input, tt.claimedUserID,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			if tt.assertResult != nil {
				tt.assertResult(t, inputSnapshot, got, tt.parsedUserID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetAllSubscriptions
// ---------------------------------------------------------------------------

func Test_subscriptionService_GetAllSubscriptions(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(repo *repomocks.MockSubscriptionRepository)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSubs    []*models.Subscription
	}{
		{
			// Success
			name: "success - repository GetAll returns the data",
			setupMocks: func(repo *repomocks.MockSubscriptionRepository) {
				repo.EXPECT().
					GetAll(mock.Anything).
					Return(validSubs(), nil).
					Once()
			},
			wantErr:  false,
			wantSubs: validSubs(),
		},
		// Repo returns a DB error
		{
			name: "error - repository GetAll returns db error",
			setupMocks: func(repo *repomocks.MockSubscriptionRepository) {
				repo.EXPECT().
					GetAll(mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.GetAllSubscriptions(t.Context())

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSubs, got)
		})
	}
}

// ---------------------------------------------------------------------------
// GetSubscriptionByID
// ---------------------------------------------------------------------------

func Test_subscriptionService_GetSubscriptionByID(t *testing.T) {
	tests := []struct {
		name          string
		subID         string
		claimedUserID string
		parsedSubID   bson.ObjectID
		setupMocks    func(
			subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID,
		)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSub     *models.Subscription
	}{
		{
			// Happy Path: The user successfully retrieves their own subscription.
			name:          "success - owner views subscription",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().GetByID(mock.Anything, subID).
					Return(validSub(), nil).Once()
			},
			wantSub: validSub(),
		},
		{
			// subID hex is invalid.
			name:          "error - invalid subscription ID",
			subID:         "bad-hex",
			claimedUserID: defaultUserHex,
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrBadRequest,
		},
		{
			// claimedUserID hex is invalid.
			name:          "error - invalid claimed user ID",
			subID:         defaultSubHex,
			claimedUserID: "bad-hex",
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrUnauthorized,
		},
		{
			// Subscription not found.
			name:          "error - subscription not found",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription belongs to a different user.
			name:          "error - subscription belongs to different user",
			subID:         defaultSubHex,
			claimedUserID: bson.NewObjectID().Hex(), // different user
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().GetByID(mock.Anything, subID).
					Return(validSub(), nil).Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.parsedSubID)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.GetSubscriptionByID(
				t.Context(), tt.subID, tt.claimedUserID,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s", appErr.Code(), tt.wantErrCode)
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
			assert.Equal(t, tt.wantSub, got)
		})
	}
}

// ---------------------------------------------------------------------------
// GetSubscriptionsByUserID
// ---------------------------------------------------------------------------

func Test_subscriptionService_GetSubscriptionsByUserID(t *testing.T) {
	tests := []struct {
		name          string
		id            string
		claimedUserID string
		parsedUserID  bson.ObjectID
		setupMocks    func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID)
		wantErr       bool
		wantErrCode   apperror.ErrorCode
		wantSubs      []*models.Subscription
	}{
		{
			// Happy path: caller owns the resource
			name:          "success - owner views their subscriptions",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedUserID:  defaultUserID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID) {
				subRepo.EXPECT().
					GetByUserID(mock.Anything, userID).
					Return(validSubs(), nil).
					Once()
			},
			wantSubs: validSubs(),
		},
		{
			// id != claimedUserID → forbidden before any repo call
			name:          "error - caller does not own the resource",
			id:            defaultUserHex,
			claimedUserID: bson.NewObjectID().Hex(),
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrForbidden,
		},
		{
			// User id is not a valid hex string
			name:          "error - malformed user id string",
			id:            "bad-hex",
			claimedUserID: "bad-hex",
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrUnauthorized,
		},
		{
			// Repo returns a DB error.
			name:          "error - repository GetByUserID returns db error",
			id:            defaultUserHex,
			claimedUserID: defaultUserHex,
			parsedUserID:  defaultUserID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID) {
				subRepo.EXPECT().
					GetByUserID(mock.Anything, userID).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.parsedUserID)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.GetSubscriptionsByUserID(t.Context(), tt.id, tt.claimedUserID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSubs, got)
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteSubscription
// ---------------------------------------------------------------------------

func Test_subscriptionService_DeleteSubscription(t *testing.T) {
	tests := []struct {
		name          string
		subID         string
		claimedUserID string
		parsedSubID   bson.ObjectID
		setupMocks    func(
			subRepo *repomocks.MockSubscriptionRepository,
			subID bson.ObjectID,
		)
		wantErr     bool
		wantErrCode apperror.ErrorCode
	}{
		{
			// Happy path: expired subscription can be deleted
			name:          "success - expired subscription deleted",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validExpiredSub(), nil).
					Once()

				subRepo.EXPECT().
					Delete(mock.Anything, subID).
					Return(nil).
					Once()
			},
		},
		{
			// subID is invalid
			name:          "error - invalid subscription ID hex",
			subID:         "bad-hex",
			claimedUserID: defaultUserHex,
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrBadRequest,
		},
		{
			// claimedUserID is invalid
			name:          "error - invalid claimed user ID hex",
			subID:         defaultSubHex,
			claimedUserID: "bad-hex",
			setupMocks:    func(_ *repomocks.MockSubscriptionRepository, _ bson.ObjectID) {},
			wantErr:       true,
			wantErrCode:   apperror.ErrUnauthorized,
		},
		{
			// Subscription not found
			name:          "error - subscription not found",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription belongs to a different user.
			name:          "error - forbidden (wrong owner)",
			subID:         defaultSubHex,
			claimedUserID: bson.NewObjectID().Hex(),
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validExpiredSub(), nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrForbidden,
		},
		{
			// Subscription is still active, cannot delete.
			name:          "error - cannot delete non-expired subscription",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// Repository Delete call fails.
			name:          "error - repository Delete fails",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				subID bson.ObjectID,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validExpiredSub(), nil).
					Once()

				subRepo.EXPECT().
					Delete(mock.Anything, subID).
					Return(apperror.NewDBError(errors.New("delete failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.parsedSubID)

			svc := newSubService(subRepo, billRepo, metrics)
			err := svc.DeleteSubscription(t.Context(), tt.subID, tt.claimedUserID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// CancelSubscription
// ---------------------------------------------------------------------------

func Test_subscriptionService_CancelSubscription(t *testing.T) {
	validFutureBill := func() *models.Bill {
		b := validBill()
		b.StartDate = mockTimeOneMonthLater
		b.EndDate = mockTimeTwoMonthsLater
		return b
	}
	buildMatcher := func(updatedSub models.Subscription) any {
		return mock.MatchedBy(func(s *models.Subscription) bool {
			return assert.ObjectsAreEqual(updatedSub, *s)
		})
	}

	tests := []struct {
		name          string
		subID         string
		claimedUserID string
		parsedSubID   bson.ObjectID
		setupMocks    func(
			subRepo *repomocks.MockSubscriptionRepository,
			billRepo *repomocks.MockBillRepository,
			metrics *svcmocks.MockSubscriptionMetrics,
			subID bson.ObjectID,
			updatedSub models.Subscription,
		)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSub     *models.Subscription
	}{
		{
			// Happy path - active subscription canceled (no refund)
			name:          "success - active subscription canceled (no refund)",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				metrics *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				updatedSub models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				subRepo.EXPECT().
					Update(mock.Anything, buildMatcher(updatedSub)).
					RunAndReturn(func(_ context.Context, s *models.Subscription) (*models.Subscription, error) {
						return s, nil
					}).Once()

				metrics.EXPECT().IncSubscriptionsCanceled(mock.Anything).Once()
			},
			wantSub: validCanceledSub(),
		},
		{
			// Happy path - active subscription canceled (with refund)
			name:          "success - active subscription canceled (with refund)",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				metrics *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				updatedSub models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validFutureBill(), nil).
					Once()

				billMatcher := mock.MatchedBy(func(b *models.Bill) bool {
					return b.Status == models.Refunded &&
						b.SubscriptionID == subID &&
						b.StartDate.Equal(mockTimeOneMonthLater) &&
						b.EndDate.Equal(mockTimeTwoMonthsLater) &&
						b.UpdatedAt.Equal(mockTime)
				})
				billRepo.EXPECT().
					Update(mock.Anything, billMatcher).
					RunAndReturn(func(ctx context.Context, b *models.Bill) (*models.Bill, error) {
						return b, nil
					}).Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				subRepo.EXPECT().
					Update(mock.Anything, buildMatcher(updatedSub)).
					RunAndReturn(func(_ context.Context, s *models.Subscription) (*models.Subscription, error) {
						return s, nil
					}).Once()

				metrics.EXPECT().IncSubscriptionsCanceled(mock.Anything).Once()
			},
			wantSub: validCanceledSub(),
		},
		{
			// Invalid subscription ID
			name:          "error - invalid subscription ID hex",
			subID:         "bad-hex",
			claimedUserID: defaultUserHex,
			setupMocks: func(_ *repomocks.MockSubscriptionRepository, _ *repomocks.MockBillRepository, _ *svcmocks.MockSubscriptionMetrics, _ bson.ObjectID, _ models.Subscription) {
			},
			wantErr:     true,
			wantErrCode: apperror.ErrBadRequest,
		},
		{
			// Invalid user ID
			name:          "error - invalid user ID hex",
			subID:         defaultSubHex,
			claimedUserID: "bad-hex",
			setupMocks: func(_ *repomocks.MockSubscriptionRepository, _ *repomocks.MockBillRepository, _ *svcmocks.MockSubscriptionMetrics, _ bson.ObjectID, _ models.Subscription) {
			},
			wantErr:     true,
			wantErrCode: apperror.ErrUnauthorized,
		},
		{
			// Subscription not found
			name:          "error - subscription not found",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription belongs to a different user.
			name:          "error - forbidden (wrong owner)",
			subID:         defaultSubHex,
			claimedUserID: bson.NewObjectID().Hex(),
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrForbidden,
		},
		{
			// Already canceled.
			name:          "error - subscription not active",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validCanceledSub(), nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// GetRecentBill fails.
			name:          "error - bill repository lookup fails",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(nil, apperror.NewDBError(errors.New("lookup failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// Bill refund failed
			name:          "error - bill refund update fails",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validFutureBill(), nil).
					Once()

				billRepo.EXPECT().
					Update(mock.Anything, mock.Anything).
					RunAndReturn(func(ctx context.Context, b *models.Bill) (*models.Bill, error) {
						return nil, apperror.NewDBError(errors.New("connection refused"))
					}).Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// GetRecentBill fails after refund
			name:          "error - get bill after refund fails",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validFutureBill(), nil).
					Once()

				billRepo.EXPECT().
					Update(mock.Anything, mock.Anything).
					RunAndReturn(func(ctx context.Context, b *models.Bill) (*models.Bill, error) {
						return b, nil
					}).Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("no paid bill found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription Update fails.
			name:          "error - subscription Update fails",
			subID:         defaultSubHex,
			claimedUserID: defaultUserHex,
			parsedSubID:   defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				_ *svcmocks.MockSubscriptionMetrics,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				subRepo.EXPECT().
					Update(mock.Anything, mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("update failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			var expectedSub models.Subscription
			if tt.wantSub != nil {
				expectedSub = *tt.wantSub
			}
			tt.setupMocks(subRepo, billRepo, metrics, tt.parsedSubID, expectedSub)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.CancelSubscription(t.Context(), tt.subID, tt.claimedUserID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(),
						tt.wantErrCode,
					)
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
			assert.NotNil(t, got)
			assert.Equal(t, tt.wantSub, got)
		})
	}
}

// ---------------------------------------------------------------------------
// RenewSubscriptionInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_RenewSubscriptionInternal(t *testing.T) {
	// renewedSub is what the subscription looks like after a successful renewal.
	// ValidTill advances by one more month (two months from today).
	renewedSub := func() *models.Subscription {
		s := validSub()
		s.ValidTill = mockTimeTwoMonthsLater
		return s
	}

	buildBillMatcher := func(updatedSub models.Subscription) any {
		return mock.MatchedBy(func(b *models.Bill) bool {
			staticValid := b.Amount == updatedSub.Price &&
				b.Currency == updatedSub.Currency &&
				b.SubscriptionID == updatedSub.ID &&
				b.Status == models.Paid

			dynamicValid := b.ID != bson.NilObjectID &&
				b.StartDate.Equal(mockTimeOneMonthLater) &&
				b.EndDate.Equal(updatedSub.ValidTill) &&
				b.CreatedAt.Equal(mockTime) &&
				b.UpdatedAt.Equal(mockTime)

			return staticValid && dynamicValid
		})
	}

	tests := []struct {
		name       string
		subID      bson.ObjectID
		setupMocks func(
			subRepo *repomocks.MockSubscriptionRepository,
			billRepo *repomocks.MockBillRepository,
			subID bson.ObjectID,
			updatedSub models.Subscription,
		)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSub     *models.Subscription
	}{
		{
			// Happy path: active subscription with a paid bill is renewed.
			name:  "success - active subscription renewed",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				updatedSub models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				billRepo.EXPECT().
					Create(mock.Anything, buildBillMatcher(updatedSub)).
					RunAndReturn(func(_ context.Context, b *models.Bill) (*models.Bill, error) {
						return b, nil
					}).Once()

				subMatcher := mock.MatchedBy(func(s *models.Subscription) bool {
					return assert.ObjectsAreEqual(updatedSub, *s)
				})
				subRepo.EXPECT().
					Update(mock.Anything, subMatcher).
					RunAndReturn(func(_ context.Context, s *models.Subscription) (*models.Subscription, error) {
						return s, nil
					}).Once()
			},
			wantSub: renewedSub(),
		},
		{
			// Subscription not found.
			name:  "error - subscription not found",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription is not active (e.g. already canceled).
			name:  "error - subscription is not active",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				_ *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validCanceledSub(), nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// GetRecentBill fails.
			name:  "error - bill repository lookup fails",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(nil, apperror.NewDBError(errors.New("lookup failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// Subscription has no recent bill at all (first bill).
			name:  "error - no active bill found (nil return)",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(nil, nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			name:  "error - latest bill is not paid",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				unpaidBill := validBill()
				unpaidBill.Status = models.Refunded

				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(unpaidBill, nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// Latest bill is already future-dated → already renewed.
			name:  "error - subscription already renewed",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				futureBill := validBill()
				futureBill.StartDate = mockTimeOneMonthLater
				futureBill.EndDate = mockTimeTwoMonthsLater

				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(futureBill, nil).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// billRepo.Create fails inside the transaction.
			name:  "error - bill repository Create fails",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				billRepo.EXPECT().
					Create(mock.Anything, mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("insert failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
		{
			// subRepo.Update fails inside the transaction.
			name:  "error - subscription Update fails",
			subID: defaultSubID,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				billRepo *repomocks.MockBillRepository,
				subID bson.ObjectID,
				_ models.Subscription,
			) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()

				billRepo.EXPECT().
					GetRecentBill(mock.Anything, subID).
					Return(validBill(), nil).
					Once()

				billRepo.EXPECT().
					Create(mock.Anything, mock.Anything).
					RunAndReturn(func(_ context.Context, b *models.Bill) (*models.Bill, error) {
						return b, nil
					}).Once()

				subRepo.EXPECT().
					Update(mock.Anything, mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("update failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			var expectedSub models.Subscription
			if tt.wantSub != nil {
				expectedSub = *tt.wantSub
			}
			tt.setupMocks(subRepo, billRepo, tt.subID, expectedSub)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.RenewSubscriptionInternal(t.Context(), tt.subID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSub, got)
		})
	}
}

// ---------------------------------------------------------------------------
// FetchUpcomingRenewalsInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_FetchUpcomingRenewalsInternal(t *testing.T) {
	daysAhead := []int{1, 3, 7}

	tests := []struct {
		name        string
		setupMocks  func(subRepo *repomocks.MockSubscriptionRepository)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSubs    []*models.Subscription
	}{
		{
			// Success - repo returns subscriptions due for reminder.
			name: "success - repository returns subscriptions due for reminder",
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository) {
				subRepo.EXPECT().
					GetSubscriptionsDueForReminder(mock.Anything, daysAhead, mockTime).
					Return(validSubs(), nil).
					Once()
			},
			wantSubs: validSubs(),
		},
		{
			// Repo returns a DB error.
			name: "error - repository returns db error",
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository) {
				subRepo.EXPECT().
					GetSubscriptionsDueForReminder(mock.Anything, daysAhead, mockTime).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.FetchUpcomingRenewalsInternal(t.Context(), daysAhead)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSubs, got)
		})
	}
}

// ---------------------------------------------------------------------------
// HasActiveSubscriptionsInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_HasActiveSubscriptionsInternal(t *testing.T) {
	tests := []struct {
		name        string
		userID      bson.ObjectID
		setupMocks  func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID)
		wantActive  bool
		wantErr     bool
		wantErrCode apperror.ErrorCode
	}{
		{
			// Happy Path where user has active subscriptions
			name:   "true - user has subscriptions",
			userID: defaultUserID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID) {
				subRepo.EXPECT().GetByUserID(mock.Anything, userID).
					Return(validSubs(), nil).Once()
			},
			wantActive: true,
		},
		{
			// Happy Path where user has no active subscriptions
			name:   "false - user has no subscriptions",
			userID: defaultUserID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID) {
				subRepo.EXPECT().GetByUserID(mock.Anything, userID).
					Return([]*models.Subscription{}, nil).Once()
			},
			wantActive: false,
		},
		{
			// Repository returns error
			name:   "error - repository returns error",
			userID: defaultUserID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, userID bson.ObjectID) {
				subRepo.EXPECT().GetByUserID(mock.Anything, userID).
					Return(nil, apperror.NewDBError(errors.New("db error"))).Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.userID)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.HasActiveSubscriptionsInternal(t.Context(), tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				assert.False(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantActive, got)
		})
	}
}

// ---------------------------------------------------------------------------
// FetchSubscriptionByIDInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_FetchSubscriptionByIDInternal(t *testing.T) {
	tests := []struct {
		name        string
		subID       bson.ObjectID
		setupMocks  func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSub     *models.Subscription
	}{
		{
			// Success - repository GetByID returns the data.
			name:  "success - repository GetByID returns the data",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil).
					Once()
			},
			wantSub: validSub(),
		},
		{
			// Subscription not found.
			name:  "error - repository GetByID returns not found",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.subID)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.FetchSubscriptionByIDInternal(t.Context(), tt.subID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s", appErr.Code(), tt.wantErrCode)
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
			assert.Equal(t, tt.wantSub, got)
		})
	}
}

// ---------------------------------------------------------------------------
// FetchSubscriptionsDueForRenewalInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_FetchSubscriptionsDueForRenewalInternal(t *testing.T) {
	tests := []struct {
		name       string
		startTime  time.Time
		endTime    time.Time
		setupMocks func(
			subRepo *repomocks.MockSubscriptionRepository,
			startTime, endTime time.Time,
		)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSubs    []*models.Subscription
	}{
		{
			// Success - repo returns subscriptions due for renewal.
			name:      "success - repository returns subscriptions due for renewal",
			startTime: mockToday,
			endTime:   mockTimeOneMonthLater,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				startTime, endTime time.Time,
			) {
				subRepo.EXPECT().
					GetSubscriptionsDueForRenewal(mock.Anything, startTime, endTime).
					Return(validSubs(), nil).
					Once()
			},
			wantSubs: validSubs(),
		},
		{
			// Repo returns a DB error.
			name:      "error - repository returns db error",
			startTime: mockToday,
			endTime:   mockTimeOneMonthLater,
			setupMocks: func(
				subRepo *repomocks.MockSubscriptionRepository,
				startTime, endTime time.Time,
			) {
				subRepo.EXPECT().
					GetSubscriptionsDueForRenewal(mock.Anything, startTime, endTime).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.startTime, tt.endTime)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.FetchSubscriptionsDueForRenewalInternal(t.Context(), tt.startTime, tt.endTime)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSubs, got)
		})
	}
}

// ---------------------------------------------------------------------------
// FetchCanceledExpiredSubscriptionsInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_FetchCanceledExpiredSubscriptionsInternal(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(subRepo *repomocks.MockSubscriptionRepository)
		wantErr     bool
		wantErrCode apperror.ErrorCode
		wantSubs    []*models.Subscription
	}{
		{
			// Success - repo returns canceled/expired subscriptions.
			name: "success - repository returns canceled expired subscriptions",
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository) {
				subRepo.EXPECT().
					GetCanceledExpiredSubscriptions(mock.Anything, mockTime).
					Return(validSubs(), nil).
					Once()
			},
			wantSubs: validSubs(),
		},
		{
			// Repo returns a DB error.
			name: "error - repository returns db error",
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository) {
				subRepo.EXPECT().
					GetCanceledExpiredSubscriptions(mock.Anything, mockTime).
					Return(nil, apperror.NewDBError(errors.New("connection lost"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo)

			svc := newSubService(subRepo, billRepo, metrics)
			got, err := svc.FetchCanceledExpiredSubscriptionsInternal(t.Context())

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
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
			assert.Equal(t, tt.wantSubs, got)
		})
	}
}

// ---------------------------------------------------------------------------
// MarkCanceledSubscriptionAsExpiredInternal
// ---------------------------------------------------------------------------

func Test_subscriptionService_MarkCanceledSubscriptionAsExpiredInternal(t *testing.T) {
	tests := []struct {
		name        string
		subID       bson.ObjectID
		setupMocks  func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID)
		wantErr     bool
		wantErrCode apperror.ErrorCode
	}{
		{
			// Happy path: canceled subscription is marked expired.
			name:  "success - canceled subscription marked as expired",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validCanceledSub(), nil).
					Once()

				matcher := mock.MatchedBy(func(s *models.Subscription) bool {
					changeValid := s.ID == subID &&
						s.Status == models.Expired &&
						s.UpdatedAt.Equal(mockTime)

					staticValid := s.ValidTill.Equal(mockTimeOneMonthLater) &&
						s.UserID == defaultUserID
					return changeValid && staticValid
				})
				subRepo.EXPECT().
					Update(mock.Anything, matcher).
					RunAndReturn(func(_ context.Context, s *models.Subscription) (*models.Subscription, error) {
						return s, nil
					}).Once()
			},
		},
		{
			// Subscription not found.
			name:  "error - subscription not found",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(nil, apperror.NewNotFoundError("not found")).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrNotFound,
		},
		{
			// Subscription is not canceled (e.g. still active).
			name:  "error - subscription is not canceled",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validSub(), nil). // status is Active
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrConflict,
		},
		{
			// Repository Update fails.
			name:  "error - repository Update fails",
			subID: defaultSubID,
			setupMocks: func(subRepo *repomocks.MockSubscriptionRepository, subID bson.ObjectID) {
				subRepo.EXPECT().
					GetByID(mock.Anything, subID).
					Return(validCanceledSub(), nil).
					Once()

				subRepo.EXPECT().
					Update(mock.Anything, mock.Anything).
					Return(nil, apperror.NewDBError(errors.New("update failed"))).
					Once()
			},
			wantErr:     true,
			wantErrCode: apperror.ErrDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subRepo := repomocks.NewMockSubscriptionRepository(t)
			billRepo := repomocks.NewMockBillRepository(t)
			metrics := svcmocks.NewMockSubscriptionMetrics(t)
			tt.setupMocks(subRepo, tt.subID)

			svc := newSubService(subRepo, billRepo, metrics)
			err := svc.MarkCanceledSubscriptionAsExpiredInternal(t.Context(), tt.subID)

			if tt.wantErr {
				assert.Error(t, err)
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, tt.wantErrCode, appErr.Code(),
						"unexpected error code: got %s, want %s",
						appErr.Code(), tt.wantErrCode,
					)
				} else {
					assert.Empty(t, tt.wantErrCode,
						"test case defined a wantErrCode (%s), but received raw error: %v",
						tt.wantErrCode, err,
					)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}
