package services

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubscriptionService interface {
	CreateSubscription(context.Context, *models.Subscription, string) (*models.Subscription, error)
	GetAllSubscriptions(context.Context) ([]*models.Subscription, error)
	GetSubscriptionByID(context.Context, string, string) (*models.Subscription, error)
	GetSubscriptionsByUserID(context.Context, string, string) ([]*models.Subscription, error)
	UpdateSubscription(context.Context, string, *models.Subscription, string) (*models.Subscription, error)
	DeleteSubscription(context.Context, string, string) error
	CancelSubscription(context.Context, string, string) (*models.Subscription, error)
	RenewSubscription(context.Context, string, string) (*models.Subscription, error)
	GetUpcomingRenewals(context.Context, string) ([]*models.Subscription, error)
}

type subscriptionService struct {
	subscriptionRepository repositories.SubscriptionRepository
}

func NewSubscriptionService(subscriptionRepository repositories.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{
		subscriptionRepository,
	}
}

func (s *subscriptionService) CreateSubscription(ctx context.Context, subscription *models.Subscription, claimedUserID string) (*models.Subscription, error) {
	slog.Debug("Creating subscription", slog.String("subscription", subscription.Name))
	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}
	subscription.UserID = userID

	if subscription.RenewalDate.IsZero() {
		// Set renewal date based on start date and frequency
		subscription.RenewalDate = calcRenewalDate(subscription.StartDate, subscription.Frequency)
	}

	// Check if subscription is already expired
	if subscription.RenewalDate.Before(time.Now()) {
		subscription.Status = models.Expired
	}

	// Continue with validation
	if err := subscription.Validate(); err != nil {
		return nil, err
	}

	// Set default values
	if subscription.Currency == "" {
		subscription.Currency = models.USD
	}
	if subscription.Status == "" {
		subscription.Status = models.Active
	}

	// Set timestamps
	now := time.Now()
	subscription.CreatedAt = now
	subscription.UpdatedAt = now

	// Set ID if not provided
	if subscription.ID.IsZero() {
		subscription.ID = bson.NewObjectID()
	}

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

func (s *subscriptionService) UpdateSubscription(ctx context.Context, id string, subscription *models.Subscription, claimedUserID string) (*models.Subscription, error) {
	slog.Debug("Updating subscription", slog.String("subscriptionID", id))
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid subscription ID")
	}

	userID, err := bson.ObjectIDFromHex(claimedUserID)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}

	// Get the existing subscription
	existingSubscription, err := s.subscriptionRepository.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if existingSubscription.UserID != userID {
		return nil, apperror.NewForbiddenError("You are not allowed to update this subscription")
	}

	// Update only the fields that are provided in the update request
	if subscription.Name != "" {
		existingSubscription.Name = subscription.Name
	}

	if subscription.Price > 0 {
		existingSubscription.Price = subscription.Price
	}

	if subscription.Currency != "" {
		existingSubscription.Currency = subscription.Currency
	}

	if subscription.Frequency != "" {
		existingSubscription.Frequency = subscription.Frequency
	}

	if subscription.Category != "" {
		existingSubscription.Category = subscription.Category
	}

	if subscription.PaymentMethod != "" {
		existingSubscription.PaymentMethod = subscription.PaymentMethod
	}

	if subscription.Status != "" {
		existingSubscription.Status = subscription.Status
	}

	if !subscription.StartDate.IsZero() {
		existingSubscription.StartDate = subscription.StartDate
	}

	if !subscription.RenewalDate.IsZero() {
		existingSubscription.RenewalDate = subscription.RenewalDate
	} else if subscription.Frequency != "" || !subscription.StartDate.IsZero() {
		// Recalculate renewal date if either frequency or start date has changed
		existingSubscription.RenewalDate = calcRenewalDate(existingSubscription.StartDate, existingSubscription.Frequency)
	}

	// Always update the UpdatedAt field
	existingSubscription.UpdatedAt = time.Now()

	// Check if subscription is already expired
	if existingSubscription.RenewalDate.Before(time.Now()) {
		existingSubscription.Status = models.Expired
	}

	// Validate the updated subscription
	if err := existingSubscription.Validate(); err != nil {
		return nil, err
	}

	// Update in repository using the existing repository method
	return s.subscriptionRepository.Update(ctx, existingSubscription)
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
	if subscription.Status == models.Active {
		return apperror.NewConflictError("You need to cancel the subscription first")
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

	subscription.Status = models.Cancelled
	subscription.UpdatedAt = time.Now()

	return s.subscriptionRepository.Update(ctx, subscription)
}

func (s *subscriptionService) RenewSubscription(ctx context.Context, id string, claimedUserID string) (*models.Subscription, error) {
	slog.Debug("Renewing subscription", slog.String("subscriptionID", id))
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
		return nil, apperror.NewForbiddenError("You are not allowed to renew this subscription")
	}

	if subscription.Status != models.Active && subscription.Status != models.Expired {
		return nil, apperror.NewConflictError("Only active subscriptions can be renewed")
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.UTC().Location())
	if subscription.RenewalDate.Before(today) {
		subscription.StartDate = today
	} else {
		subscription.StartDate = subscription.RenewalDate
	}
	subscription.RenewalDate = calcRenewalDate(subscription.StartDate, subscription.Frequency)
	subscription.Status = models.Active

	subscription.ID = bson.NewObjectID()
	subscription.CreatedAt = now
	subscription.UpdatedAt = now

	return s.subscriptionRepository.Create(ctx, subscription)
}

func (s *subscriptionService) GetUpcomingRenewals(ctx context.Context, daysParam string) ([]*models.Subscription, error) {
	slog.Debug("Fetching subscriptions with upcoming renewals")
	// Default to 7 days if not specified
	days := []int{7}

	// Parse days parameter if provided
	if daysParam != "" {
		rawDays := strings.Split(daysParam, ",")
		days = make([]int, 0, len(rawDays))
		for _, raw := range rawDays {
			day, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				return nil, apperror.NewBadRequestError("Invalid days parameter")
			}
			if day < 0 {
				return nil, apperror.NewBadRequestError("Days parameter must be positive")
			}
			days = append(days, day)
		}
	}

	return s.subscriptionRepository.GetSubscriptionsDueForReminder(ctx, days)
}

func calcRenewalDate(start time.Time, frequency models.Frequency) time.Time {
    switch frequency {
    case models.Daily:
        return start.AddDate(0, 0, 1)
    case models.Weekly:
        return start.AddDate(0, 0, 7)
    case models.Monthly:
        // Get original day to preserve
        originalDay := start.Day()
        
        // Get next month date
        nextMonth := time.Date(
            start.Year(),
            start.Month() + 1,
            1, // temporarily use 1st of month
            start.Hour(),
            start.Minute(),
            start.Second(),
            start.Nanosecond(),
            start.Location(),
        )
        
        // Handle December → January transition
        if start.Month() == time.December {
            nextMonth = time.Date(
                start.Year() + 1,
                time.January,
                1,
                start.Hour(),
                start.Minute(),
                start.Second(),
                start.Nanosecond(),
                start.Location(),
            )
        }
        
        // Find out how many days are in the next month
        lastDayOfNextMonth := time.Date(
            nextMonth.Year(),
            nextMonth.Month() + 1,
            0, // This gives the last day of nextMonth
            0, 0, 0, 0,
            nextMonth.Location(),
        ).Day()
        
        // Use either the original day or the last day of the month, whichever is smaller
        renewalDay := originalDay
        if renewalDay > lastDayOfNextMonth {
            renewalDay = lastDayOfNextMonth
        }
        
        return time.Date(
            nextMonth.Year(),
            nextMonth.Month(),
            renewalDay,
            start.Hour(),
            start.Minute(),
            start.Second(),
            start.Nanosecond(),
            start.Location(),
        )
    case models.Yearly:
        return start.AddDate(1, 0, 0)
    default:
        return start // fallback, no change
    }
}
