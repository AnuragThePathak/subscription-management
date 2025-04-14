package models

import (
	"context"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Currency represents valid currency types.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

// Frequency represents subscription billing frequency.
type Frequency string

const (
	Daily   Frequency = "daily"
	Weekly  Frequency = "weekly"
	Monthly Frequency = "monthly"
	Yearly  Frequency = "yearly"
)

// Category represents subscription categories.
type Category string

const (
	Sports        Category = "sports"
	News          Category = "news"
	Entertainment Category = "entertainment"
	Lifestyle     Category = "lifestyle"
	Technology    Category = "technology"
	Finance       Category = "finance"
	Politics      Category = "politics"
	Other         Category = "other"
)

// Status represents subscription status.
type Status string

const (
	Active    Status = "active"
	Cancelled Status = "cancelled"
	Expired   Status = "expired"
)

// Subscription represents a subscription in the database.
type Subscription struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	Name          string        `bson:"name"`
	Price         float64       `bson:"price"`
	Currency      Currency      `bson:"currency"`
	Frequency     Frequency     `bson:"frequency"`
	Category      Category      `bson:"category"`
	PaymentMethod string        `bson:"paymentMethod"`
	Status        Status        `bson:"status"`
	StartDate     time.Time     `bson:"startDate"`
	RenewalDate   time.Time     `bson:"renewalDate"`
	UserID        bson.ObjectID `bson:"userId"`
	CreatedAt     time.Time     `bson:"createdAt"`
	UpdatedAt     time.Time     `bson:"updatedAt"`
}

// Validate validates the subscription fields.
func (s *Subscription) Validate() error {
	if s.Name == "" || len(s.Name) < 2 || len(s.Name) > 100 {
		return apperror.NewValidationError("name must be between 2 and 100 characters")
	}
	if s.Price <= 0 {
		return apperror.NewValidationError("price must be greater than 0")
	}
	if s.Currency != USD && s.Currency != EUR && s.Currency != GBP {
		return apperror.NewValidationError("invalid currency")
	}
	if s.Frequency != Daily && s.Frequency != Weekly && s.Frequency != Monthly && s.Frequency != Yearly {
		return apperror.NewValidationError("invalid frequency")
	}
	if s.Category != Sports && s.Category != News && s.Category != Entertainment &&
		s.Category != Lifestyle && s.Category != Technology && s.Category != Finance &&
		s.Category != Politics && s.Category != Other {
		return apperror.NewValidationError("invalid category")
	}
	if s.PaymentMethod == "" {
		return apperror.NewValidationError("payment method is required")
	}
	if s.Status != Active && s.Status != Cancelled && s.Status != Expired {
		return apperror.NewValidationError("invalid status")
	}
	if s.StartDate.IsZero() || s.StartDate.After(time.Now()) {
		return apperror.NewValidationError("start date must be in the past")
	}
	if s.RenewalDate.IsZero() || (s.Status != Expired && !s.RenewalDate.After(s.StartDate)) {
		return apperror.NewValidationError("renewal date must be after the start date")
	}
	if s.UserID.IsZero() {
		return apperror.NewValidationError("user ID is required")
	}
	return nil
}

// SubscriptionCollection handles database operations for subscriptions.
type SubscriptionCollection struct {
	collection *mongo.Collection
}

// Update updates an existing subscription.
func (sc *SubscriptionCollection) Update(ctx context.Context, subscription *Subscription) error {
	// Pre-save logic to set renewal date if not provided
	if subscription.RenewalDate.IsZero() {
		renewalPeriods := map[Frequency]int{
			Daily:   1,
			Weekly:  7,
			Monthly: 30,
			Yearly:  365,
		}

		// Get days to add based on frequency
		daysToAdd := renewalPeriods[subscription.Frequency]

		// Set renewal date based on start date and frequency
		subscription.RenewalDate = subscription.StartDate.AddDate(0, 0, daysToAdd)
	}

	// Check if subscription is already expired
	if subscription.RenewalDate.Before(time.Now()) {
		subscription.Status = Expired
	}

	// Validate subscription
	if err := subscription.Validate(); err != nil {
		return err
	}

	// Update timestamp
	subscription.UpdatedAt = time.Now()

	// Update in database
	filter := bson.M{"_id": subscription.ID}
	update := bson.M{"$set": subscription}
	_, err := sc.collection.UpdateOne(ctx, filter, update)
	return err
}

// SubscriptionRequest represents the data structure for subscription API requests.
type SubscriptionRequest struct {
	Name          string    `json:"name" validate:"required,min=2,max=100"`
	Price         float64   `json:"price" validate:"required,gt=0"`
	Currency      Currency  `json:"currency"`
	Frequency     Frequency `json:"frequency" validate:"required"`
	Category      Category  `json:"category" validate:"required"`
	PaymentMethod string    `json:"paymentMethod" validate:"required"`
	StartDate     time.Time `json:"startDate" validate:"required"`
	RenewalDate   time.Time `json:"renewalDate"`
}

// ToSubscription converts a request to a Subscription model.
func (r *SubscriptionRequest) ToModel() *Subscription {
	return &Subscription{
		Name:          r.Name,
		Price:         r.Price,
		Currency:      r.Currency,
		Frequency:     r.Frequency,
		Category:      r.Category,
		PaymentMethod: r.PaymentMethod,
		Status:        Active,
		StartDate:     r.StartDate,
		RenewalDate:   r.RenewalDate,
	}
}

// SubscriptionResponse represents the data structure for subscription API responses.
type SubscriptionResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Price         float64   `json:"price"`
	Currency      string    `json:"currency"`
	Frequency     string    `json:"frequency"`
	Category      string    `json:"category"`
	PaymentMethod string    `json:"paymentMethod"`
	Status        string    `json:"status"`
	StartDate     time.Time `json:"startDate"`
	RenewalDate   time.Time `json:"renewalDate"`
	UserID        string    `json:"userId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ToResponse converts a Subscription model to a SubscriptionResponse.
func (s *Subscription) ToResponse() *SubscriptionResponse {
	return &SubscriptionResponse{
		ID:            s.ID.Hex(),
		Name:          s.Name,
		Price:         s.Price,
		Currency:      string(s.Currency),
		Frequency:     string(s.Frequency),
		Category:      string(s.Category),
		PaymentMethod: s.PaymentMethod,
		Status:        string(s.Status),
		StartDate:     s.StartDate,
		RenewalDate:   s.RenewalDate,
		UserID:        s.UserID.Hex(),
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}
