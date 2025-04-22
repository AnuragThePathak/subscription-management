package models

import (
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PaymentStatus represents bill status.
type PaymentStatus string

const (
	Paid     PaymentStatus = "paid"
	Refunded PaymentStatus = "refunded"
)

// Currency represents valid currency types.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

type Bill struct {
	ID             bson.ObjectID `bson:"_id"`
	Amount         int64         `bson:"amount"`
	Currency       Currency      `bson:"currency"`
	SubscriptionID bson.ObjectID `bson:"subscription_id"`
	StartDate      time.Time     `bson:"start_date"`
	EndDate        time.Time     `bson:"end_date"`
	Status         PaymentStatus `bson:"status"`
	CreatedAt      time.Time     `bson:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at"`
}

// Validate checks if the Bill is valid.
func (b *Bill) Validate() error {
	if b.Amount <= 0 {
		return apperror.NewValidationError("amount must be greater than 0")
	}
	if b.SubscriptionID.IsZero() {
		return apperror.NewValidationError("subscription_id is required")
	}
	if b.Currency != USD && b.Currency != EUR && b.Currency != GBP {
		return apperror.NewValidationError("currency must be one of USD, EUR, GBP")
	}
	if b.StartDate.IsZero() {
		return apperror.NewValidationError("start_date is required")
	}
	if b.EndDate.IsZero() {
		return apperror.NewValidationError("end_date is required")
	}
	if b.EndDate.Before(b.StartDate) {
		return apperror.NewValidationError("end_date must be after start_date")
	}
	if b.Status != Paid && b.Status != Refunded {
		return apperror.NewValidationError("status must be either paid or refunded")
	}
	return nil
}

// BillCreateRequest represents the request to create a new bill.
type BillCreateRequest struct {
	Amount         float64       `json:"amount" validate:"required,gt=0"`
	Currency       Currency      `json:"currency" validate:"required"`
	SubscriptionID bson.ObjectID `json:"subscriptionId" validate:"required"`
}

// BillResponse represents the response for a bill.
type BillResponse struct {
	ID             string        `json:"id"`
	Amount         int64         `json:"amount"`
	Currency       Currency      `json:"currency"`
	StartDate      time.Time     `json:"startDate"` // inclusive
	EndDate        time.Time     `json:"endDate"`   // exclusive
	Status         PaymentStatus `json:"status"`
	SubscriptionID string        `json:"subscriptionId"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

func (b *Bill) ToResponse() BillResponse {
	return BillResponse{
		ID:             b.ID.Hex(),
		Amount:         b.Amount,
		StartDate:      b.StartDate,
		EndDate:        b.EndDate,
		Currency:       b.Currency,
		Status:         b.Status,
		SubscriptionID: b.SubscriptionID.Hex(),
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
	}
}
