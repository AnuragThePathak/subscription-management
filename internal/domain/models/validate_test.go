package models_test

import (
	// "testing"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockTime is a stable timestamp used across tests that need deterministic time.
var mockTime = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

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

// mockOneMonthLater is a time one month after mockToday.
var mockOneMonthLater = mockToday.AddDate(0, 1, 0)

// mockYesterday is a time one day before mockToday
var mockYesterday = mockToday.AddDate(0, 0, -1)

// defaultUserID is a stable, deterministic ObjectID used across all tests.
var defaultUserID = bson.NewObjectID()

// defaultSubID is a stable, deterministic ObjectID used across all tests.
var defaultSubID = bson.NewObjectID()

// ---------------------------------------------------------------------------
// Subscription.Validate
// ---------------------------------------------------------------------------

func TestSubscription_Validate(t *testing.T) {
	// validSub returns a minimal Subscription that passes Validate().
	validSub := func() *models.Subscription {
		return &models.Subscription{
			Name:      "Netflix",
			Price:     999,
			Currency:  models.USD,
			Frequency: models.Monthly,
			Category:  models.Entertainment,
			Status:    models.Active,
			ValidTill: mockOneMonthLater,
			UserID:    defaultUserID,
		}
	}

	tests := []struct {
		name        string
		mutate      func(*models.Subscription)
		wantError   bool
		errContains string
	}{
		{
			name: "success - valid subscription",
			mutate: func(_ *models.Subscription) {
				// Leave valid
			},
			wantError: false,
		},
		{
			name: "success - EUR currency accepted",
			mutate: func(s *models.Subscription) {
				s.Currency = models.EUR
			},
			wantError: false,
		},
		{
			name: "success - GBP currency accepted",
			mutate: func(s *models.Subscription) {
				s.Currency = models.GBP
			},
			wantError: false,
		},
		{
			name: "success - yearly frequency accepted",
			mutate: func(s *models.Subscription) {
				s.Frequency = models.Yearly
			},
			wantError: false,
		},
		{
			name: "success - canceled status accepted",
			mutate: func(s *models.Subscription) {
				s.Status = models.Canceled
			},
			wantError: false,
		},
		{
			name: "success - expired status accepted",
			mutate: func(s *models.Subscription) {
				s.Status = models.Expired
			},
			wantError: false,
		},
		{
			name: "error - empty name",
			mutate: func(s *models.Subscription) {
				s.Name = ""
			},
			wantError:   true,
			errContains: "between 2 and 100 characters",
		},
		{
			name: "error - name too short",
			mutate: func(s *models.Subscription) {
				s.Name = "A"
			},
			wantError:   true,
			errContains: "between 2 and 100 characters",
		},
		{
			name: "error - name too long",
			mutate: func(s *models.Subscription) {
				s.Name = strings.Repeat("A", 101)
			},
			wantError:   true,
			errContains: "between 2 and 100 characters",
		},
		{
			name: "error - price is zero",
			mutate: func(s *models.Subscription) {
				s.Price = 0
			},
			wantError:   true,
			errContains: "price must be greater than 0",
		},
		{
			name: "error - price is negative",
			mutate: func(s *models.Subscription) {
				s.Price = -1
			},
			wantError:   true,
			errContains: "price must be greater than 0",
		},
		{
			name: "error - invalid currency",
			mutate: func(s *models.Subscription) {
				s.Currency = "INR"
			},
			wantError:   true,
			errContains: "invalid currency",
		},
		{
			name: "error - invalid frequency",
			mutate: func(s *models.Subscription) {
				s.Frequency = "hourly"
			},
			wantError:   true,
			errContains: "invalid frequency",
		},
		{
			name: "error - invalid category",
			mutate: func(s *models.Subscription) {
				s.Category = "gaming"
			},
			wantError:   true,
			errContains: "invalid category",
		},
		{
			name: "error - invalid status",
			mutate: func(s *models.Subscription) {
				s.Status = "pending"
			},
			wantError:   true,
			errContains: "invalid status",
		},
		{
			name: "error - expiry date is zero",
			mutate: func(s *models.Subscription) {
				s.ValidTill = time.Time{}
			},
			wantError:   true,
			errContains: "expiry date is required",
		},
		{
			name: "error - expiry date in the past",
			mutate: func(s *models.Subscription) {
				s.ValidTill = mockYesterday
			},
			wantError:   true,
			errContains: "expiry date must be in the future",
		},
		{
			// Before(now) is false when ValidTill == now, so exactly now is considered valid.
			name: "success - expiry date equal to now (boundary)",
			mutate: func(s *models.Subscription) {
				s.ValidTill = mockTime
			},
			wantError: false,
		},
		{
			// One nanosecond before now is strictly Before(now), hitting the past-date branch.
			name: "error - expiry date one nanosecond before now (boundary)",
			mutate: func(s *models.Subscription) {
				s.ValidTill = mockTime.Add(-1)
			},
			wantError:   true,
			errContains: "expiry date must be in the future",
		},
		{
			name: "error - missing user ID",
			mutate: func(s *models.Subscription) {
				s.UserID = bson.NilObjectID
			},
			wantError:   true,
			errContains: "user ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSub()
			tt.mutate(s)

			err := s.Validate(mockTime)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, apperror.ErrValidation, appErr.Code())
				} else {
					t.Errorf("error is not of type apperror.AppError")
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// Bill.Validate
// ---------------------------------------------------------------------------

func TestBill_Validate(t *testing.T) {
	// validBill returns a minimal Bill that passes Validate().
	validBill := func() *models.Bill {
		return &models.Bill{
			Amount:         999,
			Currency:       models.USD,
			SubscriptionID: defaultSubID,
			StartDate:      mockToday,
			EndDate:        mockOneMonthLater,
			Status:         models.Paid,
		}
	}

	tests := []struct {
		name        string
		mutate      func(*models.Bill) // The Mutator: Only define what changes!
		wantError   bool
		errContains string
		errCode     int
	}{
		{
			name: "success - valid bill",
			mutate: func(b *models.Bill) {
				// Do nothing, leave it valid
			},
			wantError: false,
		},
		{
			name: "success - EUR currency accepted",
			mutate: func(b *models.Bill) {
				b.Currency = models.EUR
			},
			wantError: false,
		},
		{
			name: "success - GBP currency accepted",
			mutate: func(b *models.Bill) {
				b.Currency = models.GBP
			},
			wantError: false,
		},
		{
			name: "success - refunded status accepted",
			mutate: func(b *models.Bill) {
				b.Status = models.Refunded
			},
			wantError: false,
		},
		{
			name: "error - amount is zero",
			mutate: func(b *models.Bill) {
				b.Amount = 0
			},
			wantError:   true,
			errContains: "amount must be greater than 0",
		},
		{
			name: "error - amount is negative",
			mutate: func(b *models.Bill) {
				b.Amount = -1
			},
			wantError:   true,
			errContains: "amount must be greater than 0",
		},
		{
			name: "error - missing subscription ID",
			mutate: func(b *models.Bill) {
				b.SubscriptionID = bson.NilObjectID
			},
			wantError:   true,
			errContains: "subscription_id is required",
		},
		{
			name: "error - invalid currency",
			mutate: func(b *models.Bill) {
				b.Currency = "INR"
			},
			wantError:   true,
			errContains: "currency must be one of",
		},
		{
			name: "error - start date is zero",
			mutate: func(b *models.Bill) {
				b.StartDate = time.Time{}
			},
			wantError:   true,
			errContains: "start_date is required",
		},
		{
			name: "error - end date is zero",
			mutate: func(b *models.Bill) {
				b.EndDate = time.Time{}
			},
			wantError:   true,
			errContains: "end_date is required",
		},
		{
			name: "error - end date before start date",
			mutate: func(b *models.Bill) {
				b.EndDate = mockYesterday
			},
			wantError:   true,
			errContains: "end_date must be after start_date",
		},
		{
			// Before(StartDate) is false when EndDate == StartDate, so equal dates are valid.
			name: "success - end date equal to start date (boundary)",
			mutate: func(b *models.Bill) {
				b.EndDate = mockToday
			},
			wantError: false,
		},
		{
			// One nanosecond before StartDate is strictly Before(StartDate), triggering the error.
			name: "error - end date one nanosecond before start date (boundary)",
			mutate: func(b *models.Bill) {
				b.EndDate = mockToday.Add(-1)
			},
			wantError:   true,
			errContains: "end_date must be after start_date",
		},
		{
			name: "error - invalid payment status",
			mutate: func(b *models.Bill) {
				b.Status = "pending"
			},
			wantError:   true,
			errContains: "status must be either paid or refunded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := validBill()
			tt.mutate(b)

			err := b.Validate()

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				// Unwrap and Check the Code
				if appErr, ok := errors.AsType[apperror.AppError](err); ok {
					assert.Equal(t, apperror.ErrValidation, appErr.Code())
				} else {
					t.Errorf("error is not of type apperror.AppError")
				}
				return
			}

			require.NoError(t, err)
		})
	}
}
