package models

import (
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// SubscriptionUpdateRequest represents the data structure for subscription update API requests.
type SubscriptionUpdateRequest struct {
	Name          string    `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Price         float64   `json:"price,omitempty" validate:"omitempty,gt=0"`
	Currency      Currency  `json:"currency,omitempty"`
	Frequency     Frequency `json:"frequency,omitempty"`
	Category      Category  `json:"category,omitempty"`
	PaymentMethod string    `json:"paymentMethod,omitempty"`
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
		PaymentMethod: r.PaymentMethod,
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
