package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/lib"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
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
	GetUpcomingRenewalsInternal(context.Context, []int) ([]*models.Subscription, error)
	FetchSubscriptionByIDInternal(context.Context, bson.ObjectID) (*models.Subscription, error)
	FetchSubscriptionsDueForRenewalInternal(context.Context, time.Time, time.Time) ([]*models.Subscription, error)
	FetchCancelledExpiredSubscriptionsInternal(context.Context) ([]*models.Subscription, error)
	MarkCancelledSubscriptionAsExpiredInternal(context.Context, bson.ObjectID) error
}

type SubscriptionService interface {
	SubscriptionServiceExternal
	SubscriptionServiceInternal
}

type subscriptionService struct {
	subscriptionRepository repositories.SubscriptionRepository
	billRepository         repositories.BillRepository
}

func NewSubscriptionService(
	subscriptionRepository repositories.SubscriptionRepository,
	billRepository repositories.BillRepository,
) SubscriptionService {
	return &subscriptionService{
		subscriptionRepository,
		billRepository,
	}
}

func (s *subscriptionService) CreateSubscription(ctx context.Context, subscription *models.Subscription, claimedUserID string) (*models.Subscription, error) {
	slog.Debug("Creating subscription", slog.String("subscription", subscription.Name))
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
	_, err = s.billRepository.Create(ctx, bill)
	if err != nil {
		return nil, err
	}

	subscription.CreatedAt = now
	subscription.UpdatedAt = now

	return s.subscriptionRepository.Create(ctx, subscription)
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
	slog.Debug("Deleting subscription", slog.String("subscriptionID", id))
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

	// Check if the subscription is active
	if subscription.Status != models.Cancelled {
		return apperror.NewConflictError("You can only delete expired subscriptions")
	}

	return s.subscriptionRepository.Delete(ctx, subscriptionID)
}

func (s *subscriptionService) CancelSubscription(ctx context.Context, id string, calimedUserID string) (*models.Subscription, error) {
	slog.Debug("Canceling subscription", slog.String("subscriptionID", id))
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid subscription ID")
	}

	userID, err := bson.ObjectIDFromHex(calimedUserID)
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
	if latestBill.StartDate.After(now) && latestBill.Status == models.Paid {
		// Refund the bill
		latestBill.Status = models.Refunded
		latestBill.UpdatedAt = now

		_, err = s.billRepository.Update(ctx, latestBill)
		if err != nil {
			return nil, err
		}

		// Update the subscription validity
		activeBill, err := s.billRepository.GetRecentBill(ctx, subscription.ID)
		if err != nil {
			return nil, err
		}
		if activeBill != nil && activeBill.Status == models.Paid {
			subscription.ValidTill = activeBill.EndDate
		}
	}

	// Update the subscription status
	subscription.Status = models.Cancelled
	subscription.UpdatedAt = now

	return s.subscriptionRepository.Update(ctx, subscription)
}

func (s *subscriptionService) RenewSubscriptionInternal(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	slog.Debug("Renewing subscription", slog.String("subscriptionID", id.Hex()))

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
	_, err = s.billRepository.Create(ctx, bill)
	if err != nil {
		return nil, err
	}

	// Update the subscription
	subscription.ValidTill = newValidity
	subscription.UpdatedAt = now

	return s.subscriptionRepository.Update(ctx, subscription)
}

func (s *subscriptionService) GetUpcomingRenewalsInternal(ctx context.Context, days []int) ([]*models.Subscription, error) {
	slog.Debug("Fetching subscriptions with upcoming renewals")
	return s.subscriptionRepository.GetSubscriptionsDueForReminder(ctx, days)
}

func (s *subscriptionService) FetchSubscriptionByIDInternal(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	// Get the subscription
	return s.subscriptionRepository.GetByID(ctx, id)
}

func (s *subscriptionService) FetchSubscriptionsDueForRenewalInternal(ctx context.Context, startTime, endTime time.Time) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetSubscriptionsDueForRenewal(ctx, startTime, endTime)
}

func (s *subscriptionService) FetchCancelledExpiredSubscriptionsInternal(ctx context.Context) ([]*models.Subscription, error) {
	return s.subscriptionRepository.GetCancelledExpiredSubscriptions(ctx)
}

func (s *subscriptionService) MarkCancelledSubscriptionAsExpiredInternal(ctx context.Context, id bson.ObjectID) error {
	slog.Debug("Marking cancelled subscriptions as expired")
	subscription, err := s.subscriptionRepository.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if subscription.Status != models.Cancelled {
		return apperror.NewConflictError("Only cancelled subscriptions can be marked as expired")
	}
	subscription.Status = models.Expired
	subscription.UpdatedAt = time.Now()
	_, err = s.subscriptionRepository.Update(ctx, subscription)
	if err != nil {
		return err
	}
	return nil
}
