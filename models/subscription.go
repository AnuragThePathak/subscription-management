package models

import (
	"context"
	"time"

	"github.com/anuragthepathak/subscription-management/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Currency represents valid currency types
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

// Frequency represents subscription billing frequency
type Frequency string

const (
	Daily   Frequency = "daily"
	Weekly  Frequency = "weekly"
	Monthly Frequency = "monthly"
	Yearly  Frequency = "yearly"
)

// Category represents subscription categories
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

// Status represents subscription status
type Status string

const (
	Active    Status = "active"
	Cancelled Status = "cancelled"
	Expired   Status = "expired"
)

// Subscription represents a subscription in the database
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

// Modified Validate to handle the automatically set renewal date
func (s *Subscription) Validate() error {
	// Name validation
	if s.Name == "" {
		return apperror.NewValidationError("subscription name is required")
	}
	nameLength := len(s.Name)
	if nameLength < 2 || nameLength > 100 {
		return apperror.NewValidationError("name must be between 2 and 100 characters")
	}

	// Price validation
	if s.Price <= 0 {
		return apperror.NewValidationError("price must be greater than 0")
	}

	// Currency validation
	switch s.Currency {
	case USD, EUR, GBP:
		// Valid
	default:
		return apperror.NewValidationError("invalid currency")
	}

	// Frequency validation
	switch s.Frequency {
	case Daily, Weekly, Monthly, Yearly:
		// Valid
	default:
		return apperror.NewValidationError("invalid frequency")
	}

	// Category validation
	switch s.Category {
	case Sports, News, Entertainment, Lifestyle, Technology, Finance, Politics, Other:
		// Valid
	default:
		return apperror.NewValidationError("invalid category")
	}

	// PaymentMethod validation
	if s.PaymentMethod == "" {
		return apperror.NewValidationError("payment method is required")
	}

	// Status validation
	switch s.Status {
	case Active, Cancelled, Expired:
		// Valid
	default:
		return apperror.NewValidationError("invalid status")
	}

	// StartDate validation
	if s.StartDate.IsZero() {
		return apperror.NewValidationError("start date is required")
	}
	if s.StartDate.After(time.Now()) {
		return apperror.NewValidationError("start date must be in the past")
	}

	// RenewalDate validation
	if s.RenewalDate.IsZero() {
		return apperror.NewValidationError("renewal date is required")
	}
	
	// Note: We don't check if renewal date is after start date if it's already expired,
	// since we may have automatically set status to expired
	if s.Status != Expired && !s.RenewalDate.After(s.StartDate) {
		return apperror.NewValidationError("renewal date must be after the start date")
	}

	// UserID validation
	if s.UserID.IsZero() {
		return apperror.NewValidationError("user ID is required")
	}

	return nil
}

// SubscriptionCollection handles database operations for subscriptions
type SubscriptionCollection struct {
	collection *mongo.Collection
}

// NewSubscriptionCollection creates a new subscription collection handler
func NewSubscriptionCollection(db *mongo.Database) *SubscriptionCollection {
	// Create collection
	collection := db.Collection("subscriptions")

	// Create index on user field for faster lookups
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "user", Value: 1}},
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		panic(err)
	}

	return &SubscriptionCollection{
		collection: collection,
	}
}

// Create adds a new subscription to the database
func (sc *SubscriptionCollection) Create(ctx context.Context, subscription *Subscription) error {
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

	// Continue with validation
	if err := subscription.Validate(); err != nil {
		return err
	}

	// Set default values
	if subscription.Currency == "" {
		subscription.Currency = USD
	}
	if subscription.Status == "" {
		subscription.Status = Active
	}

	// Set timestamps
	now := time.Now()
	subscription.CreatedAt = now
	subscription.UpdatedAt = now

	// Set ID if not provided
	if subscription.ID.IsZero() {
		subscription.ID = bson.NewObjectID()
	}

	// Insert into database
	_, err := sc.collection.InsertOne(ctx, subscription)
	return err
}

// GetByID retrieves a subscription by its ID
func (sc *SubscriptionCollection) GetByID(ctx context.Context, id bson.ObjectID) (*Subscription, error) {
	var subscription Subscription
	err := sc.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

// GetByUser retrieves all subscriptions for a specific user
func (sc *SubscriptionCollection) GetByUser(ctx context.Context, userID bson.ObjectID) ([]*Subscription, error) {
	cursor, err := sc.collection.Find(ctx, bson.M{"user": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subscriptions []*Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// Update updates an existing subscription
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

// SubscriptionRequest represents the data structure for subscription API requests
type SubscriptionRequest struct {
	Name          string    `json:"name" validate:"required,min=2,max=100"`
	Price         float64   `json:"price" validate:"required,gt=0"`
	Currency      Currency  `json:"currency"`
	Frequency     Frequency `json:"frequency" validate:"required"`
	Category      Category  `json:"category" validate:"required"`
	PaymentMethod string    `json:"paymentMethod" validate:"required"`
	StartDate     time.Time `json:"startDate" validate:"required"`
	RenewalDate   time.Time `json:"renewalDate" validate:"required,gtfield=StartDate"`
}

// ToSubscription converts a request to a Subscription model
func (r *SubscriptionRequest) ToSubscription(userID bson.ObjectID) *Subscription {
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
		UserID:        userID,
	}
}

// SubscriptionResponse represents the data structure for subscription API responses
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
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ToResponse converts a Subscription model to a SubscriptionResponse
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
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}
