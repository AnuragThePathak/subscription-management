package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// User is your database model
type User struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt"`
}

// UserRequest represents the data structure for user registration API requests
type UserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// ToUser converts a signup request to a User model
func (r *UserRequest) ToModel() *User {
	return &User{
		Name:     r.Name,
		Email:    r.Email,
		Password: r.Password, // Will be hashed before storing
	}
}

// UserResponse represents the data structure returned to clients
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToResponse converts a User model to a UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID.Hex(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// UserCollection handles user database operations
type UserCollection struct {
	collection *mongo.Collection
}

// NewUserCollection creates a new instance of UserCollection
func NewUserCollection(db *mongo.Database) *UserCollection {
	// Create a unique index for the email field
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	collection := db.Collection("users")
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Handle error (in production, you might want to log this)
		panic(err)
	}

	return &UserCollection{
		collection: collection,
	}
}

// Create adds a new user to the database from a signup request
func (uc *UserCollection) Create(ctx context.Context, request *UserRequest) (*UserResponse, error) {
	// Convert request to user model
	user := request.ToModel()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
	if err != nil {
		return nil, err
	}
	user.Password = string(hashedPassword)

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Set ID if not provided
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}

	// Insert into database
	_, err = uc.collection.InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}

	// Return user response without sensitive data
	return user.ToResponse(), nil
}

// FindByEmail finds a user by email
func (uc *UserCollection) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := uc.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID finds a user by ID
func (uc *UserCollection) FindByID(ctx context.Context, id bson.ObjectID) (*User, error) {
	var user User
	err := uc.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user's information
func (uc *UserCollection) Update(ctx context.Context, user *User) error {
	// If the password was changed, hash it
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}

	// Update timestamp
	user.UpdatedAt = time.Now()

	// Update the user in the database
	filter := bson.M{"_id": user.ID}
	update := bson.M{"$set": user}
	_, err := uc.collection.UpdateOne(ctx, filter, update)
	return err
}
