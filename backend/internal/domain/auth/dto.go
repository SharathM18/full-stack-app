package auth

import "github.com/google/uuid"

// request payload
type SignupRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// response payload
type UserDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        *string   `json:"name"`
	Email       *string   `json:"email,omitempty"`
	PhoneNumber *string   `json:"phoneNumber,omitempty"`
	Role        string    `json:"role"`
}

type AuthResponse struct {
	AccessToken string  `json:"accessToken"`
	TokenType   string  `json:"tokenType"`
	ExpiresIn   int64   `json:"expiresIn"`
	User        UserDTO `json:"user"`
}
