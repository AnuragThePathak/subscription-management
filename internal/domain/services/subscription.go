package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/internal/api/shared/apperror"
	"github.com/anuragthepathak/subscription-management/internal/core/logattr"
	"github.com/anuragthepathak/subscription-management/internal/domain/models"
	"github.com/anuragthepathak/subscription-management/internal/domain/repositories"
	"github.com/anuragthepathak/subscription-management/internal/lib"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubscriptionServiceExternal interface {
	CreateSubscription(context.Context, *models.Subscription, string) (*models.Subscription, error)
	GetAllSubscriptions(context.Context) ([]*models.Subscription, error)
	GetSubscriptionByID(context.Context, string, string) (*models.Subscription, error)
	GetSubscriptionsByUserID(context.Context, string, string) ([]*models.Subscription, error)
	DeleteSubscription(context.Context, string, string) error
	CancelSubscription(context.Context, string, string) (*models.Subscription, error)
}

type SubscriptionServiceInternal interface {
	RenewSubscriptionInternal(context.Context, bson.ObjectID) (*models.Subscription, error)
	FetchUpcomingRenewalsInternal(context.Context, []int) ([]*models.Subscription, error)
	FetchSubscriptionByIDInternal(context.Context, bson.ObjectID) (*models.Subscription, error)
	FetchSubscriptionsDueForRenewalInternal(context.Context, time.Time, time.Time) ([]*models.Subscription, error)
	FetchCanceledExpiredSubscriptionsInternal(context.Context) ([]*models.Subscription, error)
	MarkCanceledSubscriptionAsExpiredInternal(context.Context, bson.ObjectID) error
	HasActiveSubscriptionsInternal(context.Context, bson.ObjectID) (bool, error)
}

type SubscriptionService interface {
	SubscriptionServiceExternal
	SubscriptionServiceInternal
}

type SubscriptionMetrics interface {
	IncSubscriptionsCreated(ctx context.Context)
	IncSubscriptionsCanceled(ctx context.Context)
	IncActiveSubscriptions(ctx context.Context)
	DecActiveSubscriptions(ctx context.Context)
}

type subscriptionService struct {
	runTx                  repositories.TxnFn
	subscriptionRepository repositories.SubscriptionRepository
	billRepository         repositories.BillRepository
	metrics                SubscriptionMetrics
}

func NewSubscriptionService(
	txnFn repositories.TxnFn,
	subscriptionRepository repositories.SubscriptionRepository,
	billRepository repositories.BillRepository,
	metrics SubscriptionMetrics,
) SubscriptionService {
	return &subscriptionService{
		txnFn,
		subscriptionRepository,
		billRepository,
		metrics,
	}
}

func (s *subscriptionService) CreateSubscription(ctx context.Context, subscription *models.Subscription, claimedUserID string) (*models.Subscription, error) {
	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}
	subscription.UserID = userID
	subscription.ID = bson.NewObjectID()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	subscription.ValidTill = lib.CalcRenewalDate(today, subscription.Frequency)
	// Create the subscription
	subscription.Status = models.Active
	// Set default values
	if subscription.Currency == "" {
		subscription.Currency = models.USD
	}
	// Continue with validation
	if err = subscription.Validate(); err != nil {
		return nil, err
	}
	subscription.CreatedAt = now
	subscription.UpdatedAt = now

	// Create the bill
	bill := &models.Bill{
		ID:             bson.NewObjectID(),
		Amount:         subscription.Price,
		Currency:       subscription.Currency,
		SubscriptionID: subscription.ID,
		StartDate:      today,
		EndDate:        subscription.ValidTill,
		Status:         models.Paid,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var res *models.Subscription
	err = s.runTx(ctx, func(ctx context.Context) error {
		_, txnErr := s.billRepository.Create(ctx, bill)
		if txnErr != nil {
			return txnErr
		}
		res, txnErr = s.subscriptionRepository.Create(ctx, subscription)
		return txnErr
	})
	if err != nil {
		return nil, err
	}

	s.metrics.IncSubscriptionsCreated(ctx)
	s.metrics.IncActiveSubscriptions(ctx)

	slog.InfoContext(ctx, "Subscription created",
		logattr.SubscriptionID(res.ID.Hex()),
		logattr.SubscriptionName(subscription.Name),
		logattr.ValidTill(subscription.ValidTill),
	)
	return res, nil
}

func (s *subscriptionService) GetAllSubscriptions(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetAll(ctx)
}

func (s *subscriptionService) GetSubscriptionByID(ctx context.Context, id string, claimedUserID string) (*models.Subscription, error) {
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid subscription ID")
	}
	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	// Get the subscription
	subscription, err := s.subscriptionRepository.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if subscription.UserID != userID {
		return nil, apperror.NewForbiddenError("You are not allowed to view this subscription")
	}
	return subscription, nil
}

func (s *subscriptionService) GetSubscriptionsByUserID(ctx context.Context, id string, claimedUserID string) ([]*models.Subscription, error) {
	if claimedUserID != id {
		return nil, apperror.NewForbiddenError("You are not allowed to view this subscription")
	}

	userID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	return s.subscriptionRepository.GetByUserID(ctx, userID)
}

func (s *subscriptionService) DeleteSubscription(ctx context.Context, id string, claimedUserID string) error {
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewBadRequestError("Invalid subscription ID")
	}
	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return apperror.NewUnauthorizedError("Invalid user ID")
	}

	subscription, err := s.subscriptionRepository.GetByID(ctx, subscriptionID)
	if err != nil {
		return err
	}

	// Verify ownership
	if subscription.UserID != userID {
		return apperror.NewForbiddenError("You are not allowed to delete this subscription")
	}

	// Check if the subscription is active or still in billing period
	if subscription.Status != models.Expired {
		return apperror.NewConflictError("You can only delete expired subscriptions")
	}

	if err = s.subscriptionRepository.Delete(ctx, subscriptionID); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Subscription deleted",
		logattr.ValidTill(subscription.ValidTill),
	)
	return nil
}

func (s *subscriptionService) CancelSubscription(ctx context.Context, id string, claimedUserID string) (*models.Subscription, error) {
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid subscription ID")
	}

	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	subscription, err := s.subscriptionRepository.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if subscription.UserID != userID {
		return nil, apperror.NewForbiddenError("You are not allowed to cancel this subscription")
	}

	if subscription.Status != models.Active {
		return nil, apperror.NewConflictError("Only active subscriptions can be canceled")
	}

	latestBill, err := s.billRepository.GetRecentBill(ctx, subscription.ID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Update the subscription status
	subscription.Status = models.Canceled
	subscription.UpdatedAt = now

	var res *models.Subscription
	err = s.runTx(ctx, func(ctx context.Context) error {
		if latestBill.StartDate.After(now) && latestBill.Status == models.Paid {
			// Refund the bill
			latestBill.Status = models.Refunded
			latestBill.UpdatedAt = now

			_, txnErr := s.billRepository.Update(ctx, latestBill)
			if txnErr != nil {
				return txnErr
			}

			// Update the subscription validity
			activeBill, txnErr := s.billRepository.GetRecentBill(ctx, subscription.ID)
			if txnErr != nil {
				return txnErr
			}
			if activeBill != nil && activeBill.Status == models.Paid {
				subscription.ValidTill = activeBill.EndDate
			}
		}

		var txnErr error
		res, txnErr = s.subscriptionRepository.Update(ctx, subscription)
		return txnErr
	})
	if err != nil {
		return nil, err
	}

	s.metrics.IncSubscriptionsCanceled(ctx)
	s.metrics.DecActiveSubscriptions(ctx)

	slog.InfoContext(ctx, "Subscription canceled",
		logattr.ValidTill(res.ValidTill),
	)
	return res, nil
}

func (s *subscriptionService) RenewSubscriptionInternal(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	subscription, err := s.subscriptionRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if subscription.Status != models.Active {
		return nil, apperror.NewConflictError("Only active subscriptions can be renewed")
	}

	// Get the latest bill
	latestBill, err := s.billRepository.GetRecentBill(ctx, subscription.ID)
	if err != nil {
		return nil, err
	}
	if latestBill == nil {
		return nil, apperror.NewNotFoundError("No active bill found for this subscription")
	}
	if latestBill.Status != models.Paid {
		return nil, apperror.NewConflictError("Only paid subscriptions can be renewed")
	}

	// Check if the subscription is already renewed
	now := time.Now()
	if latestBill.StartDate.After(now) {
		return nil, apperror.NewConflictError("Subscription is already renewed")
	}

	// Create a new bill
	newStartDate := latestBill.EndDate
	newValidity := lib.CalcRenewalDate(newStartDate, subscription.Frequency)
	subscription.ValidTill = newValidity
	subscription.UpdatedAt = now

	bill := &models.Bill{
		ID:             bson.NewObjectID(),
		Amount:         subscription.Price,
		Currency:       subscription.Currency,
		SubscriptionID: subscription.ID,
		StartDate:      newStartDate,
		EndDate:        newValidity,
		Status:         models.Paid,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var res *models.Subscription
	err = s.runTx(ctx, func(ctx context.Context) error {
		_, txnErr := s.billRepository.Create(ctx, bill)
		if txnErr != nil {
			return txnErr
		}
		// Update the subscription
		res, txnErr = s.subscriptionRepository.Update(ctx, subscription)
		return txnErr
	})
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "Subscription renewed",
		logattr.ValidTill(res.ValidTill),
	)
	return res, nil
}

func (s *subscriptionService) FetchUpcomingRenewalsInternal(ctx context.Context, daysAhead []int) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetSubscriptionsDueForReminder(ctx, daysAhead)
}

func (s *subscriptionService) HasActiveSubscriptionsInternal(ctx context.Context, userID bson.ObjectID) (bool, error) {
	subscriptions, err := s.subscriptionRepository.GetByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(subscriptions) > 0, nil
}

func (s *subscriptionService) FetchSubscriptionByIDInternal(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	// Get the subscription
	return s.subscriptionRepository.GetByID(ctx, id)
}

func (s *subscriptionService) FetchSubscriptionsDueForRenewalInternal(ctx context.Context, startTime, endTime time.Time) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetSubscriptionsDueForRenewal(ctx, startTime, endTime)
}

func (s *subscriptionService) FetchCanceledExpiredSubscriptionsInternal(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetCanceledExpiredSubscriptions(ctx)
}

func (s *subscriptionService) MarkCanceledSubscriptionAsExpiredInternal(ctx context.Context, id bson.ObjectID) error {
	subscription, err := s.subscriptionRepository.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if subscription.Status != models.Canceled {
		return apperror.NewConflictError("Only canceled subscriptions can be marked as expired")
	}
	subscription.Status = models.Expired
	subscription.UpdatedAt = time.Now()
	_, err = s.subscriptionRepository.Update(ctx, subscription)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "Canceled subscription marked as expired",
		logattr.ValidTill(subscription.ValidTill),
	)
	return nil
}
