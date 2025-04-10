package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"github.com/anuragthepathak/subscription-management/models"
	"github.com/anuragthepathak/subscription-management/repositories"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubscriptionService interface {
	CreateSubscription(ctx context.Context, user *models.Subscription, userID string) (*models.Subscription, error)

	GetAllSubscriptions(ctx context.Context) ([]*models.Subscription, error)

	GetSubscriptionByID(ctx context.Context, subscriptionID string) (*models.Subscription, error)

	GetSubscriptionsByUserID(ctx context.Context, userID string, claimedUserID string) ([]*models.Subscription, error)
}

type subscriptionService struct {

	subscriptionRepository repositories.SubscriptionRepository
}

func NewSubscriptionService(subscriptionRepository repositories.SubscriptionRepository) SubscriptionService {
	return &subscriptionService{
		subscriptionRepository,
	}
}

func (s *subscriptionService) CreateSubscription(ctx context.Context, subscription *models.Subscription, userIDString string) (*models.Subscription, error) {
	slog.Debug("Creating subscription", slog.String("subscription", subscription.Name))
	userID, err := bson.ObjectIDFromHex(userIDString)
	if err != nil {
		return nil, apperror.NewUnauthorizedError("Invalid user ID")
	}
	subscription.UserID = userID

	if subscription.RenewalDate.IsZero() {
		renewalPeriods := map[models.Frequency]int{
			models.Daily:   1,
			models.Weekly:  7,
			models.Monthly: 30,
			models.Yearly:  365,
		}
		
		// Get days to add based on frequency
		daysToAdd := renewalPeriods[subscription.Frequency]
		
		// Set renewal date based on start date and frequency
		subscription.RenewalDate = subscription.StartDate.AddDate(0, 0, daysToAdd)
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

func (s *subscriptionService) GetSubscriptionByID(ctx context.Context, id string) (*models.Subscription, error) {
	subscriptionID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid user ID")
	}
	return s.subscriptionRepository.GetByID(ctx, subscriptionID)
}

func (s *subscriptionService) GetSubscriptionsByUserID(ctx context.Context, userIDString string, claimedUserID string) ([]*models.Subscription, error) {
	if claimedUserID != userIDString {
		return nil, apperror.NewUnauthorizedError("You are not authorized to view this subscription")
	}

	userID, err := bson.ObjectIDFromHex(userIDString)
	if err != nil {
		return nil, apperror.NewBadRequestError("Invalid user ID")
	}

	return s.subscriptionRepository.GetByUserID(ctx, userID)
}