package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User represents the database model for a user.
type User struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Password  string        `bson:"password"`
	CreatedAt time.Time     `bson:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt"`
}

// UserRequest represents the data structure for user registration API requests.
type UserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// ToModel converts a UserRequest to a User model.
func (r *UserRequest) ToModel() *User {
	return &User{
		Name:     r.Name,
		Email:    r.Email,
		Password: r.Password, // Will be hashed before storing.
	}
}

// UserResponse represents the data structure returned to clients.
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToResponse converts a User model to a UserResponse.
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID.Hex(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// // Update updates a user's information
// func (uc *UserCollection) Update(ctx context.Context, user *User) error {
// 	// If the password was changed, hash it
// 	if user.Password != "" {
// 		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), 10)
// 		if err != nil {
// 			return err
// 		}
// 		user.Password = string(hashedPassword)
// 	}

// 	// Update timestamp
// 	user.UpdatedAt = time.Now()

// 	// Update the user in the database
// 	filter := bson.M{"_id": user.ID}
// 	update := bson.M{"$set": user}
// 	_, err := uc.collection.UpdateOne(ctx, filter, update)
// 	return err
// }
