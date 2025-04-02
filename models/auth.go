package models

type AuthRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

func (r *AuthRequest) ToModel() *User {
	return &User{
		Email:    r.Email,
		Password: r.Password,
	}
}

// AuthResponse represents the data structure returned to clients
type AuthResponse struct {
	Token string `json:"token"`
}