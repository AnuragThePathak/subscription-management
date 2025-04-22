package models

import (
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
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
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Price     int64         `bson:"price"`
	Currency  Currency      `bson:"currency"`
	Frequency Frequency     `bson:"frequency"`
	Category  Category      `bson:"category"`
	Status    Status        `bson:"status"`
	ValidTill time.Time     `bson:"valid_till"` // Exclusive
	UserID    bson.ObjectID `bson:"user_id"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

// Validate validates the subscription fields.
func (s *Subscription) Validate() error {
	if s.Name == "" || len(s.Name) < 2 || len(s.Name) > 100 {
		return apperror.NewValidationError("name must be between 2 and 100 characters")
	}
	if s.Price <= 0 {
		return apperror.NewValidationError("price must be greater than 0")
	}
	if s.Frequency != Daily && s.Frequency != Weekly && s.Frequency != Monthly && s.Frequency != Yearly {
		return apperror.NewValidationError("invalid frequency")
	}
	if s.Category != Sports && s.Category != News && s.Category != Entertainment &&
		s.Category != Lifestyle && s.Category != Technology && s.Category != Finance &&
		s.Category != Politics && s.Category != Other {
		return apperror.NewValidationError("invalid category")
	}
	if s.Status != Active && s.Status != Cancelled && s.Status != Expired {
		return apperror.NewValidationError("invalid status")
	}
	if s.ValidTill.IsZero() || s.ValidTill.Before(time.Now()) {
		return apperror.NewValidationError("expiry date must be in the future")
	}
	if s.UserID.IsZero() {
		return apperror.NewValidationError("user ID is required")
	}
	return nil
}

// SubscriptionRequest represents the data structure for subscription API requests.
type SubscriptionRequest struct {
	Name      string    `json:"name" validate:"required,min=2,max=100"`
	Price     int64     `json:"price" validate:"required,gt=0"`
	Currency  Currency  `json:"currency"`
	Frequency Frequency `json:"frequency" validate:"required"`
	Category  Category  `json:"category" validate:"required"`
}

// ToSubscription converts a request to a Subscription model.
func (r *SubscriptionRequest) ToModel() *Subscription {
	return &Subscription{
		Name:      r.Name,
		Price:     r.Price,
		Currency:  r.Currency,
		Frequency: r.Frequency,
		Category:  r.Category,
	}
}

// SubscriptionUpdateRequest represents the data structure for subscription update API requests.
type SubscriptionUpdateRequest struct {
	Name          string    `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Price         int64     `json:"price,omitempty" validate:"omitempty,gt=0"`
	Currency      Currency  `json:"currency,omitempty"`
	Frequency     Frequency `json:"frequency,omitempty"`
	Category      Category  `json:"category,omitempty"`
	StartDate     time.Time `json:"startDate,omitzero"`
	RenewalDate   time.Time `json:"renewalDate,omitzero"`
}

// ToModel converts an update request to a Subscription model.
func (r *SubscriptionUpdateRequest) ToModel() *Subscription {
	return &Subscription{
		Name:          r.Name,
		Price:         r.Price,
		Currency:      r.Currency,
		Frequency:     r.Frequency,
		Category:      r.Category,
		ValidTill:     r.RenewalDate,
	}
}

// SubscriptionResponse represents the data structure for subscription API responses.
type SubscriptionResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Price     int64     `json:"price"`
	Currency  string    `json:"currency"`
	Frequency string    `json:"frequency"`
	Category  string    `json:"category"`
	Status    string    `json:"status"`
	ValidTill time.Time `json:"validTill"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToResponse converts a Subscription model to a SubscriptionResponse.
func (s *Subscription) ToResponse() *SubscriptionResponse {
	return &SubscriptionResponse{
		ID:        s.ID.Hex(),
		Name:      s.Name,
		Price:     s.Price,
		Currency:  string(s.Currency),
		Frequency: string(s.Frequency),
		Category:  string(s.Category),
		Status:    string(s.Status),
		ValidTill: s.ValidTill,
		UserID:    s.UserID.Hex(),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
