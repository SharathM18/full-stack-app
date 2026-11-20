package user

import (
	"time"
	"uuid"
)

type UserModel struct {
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
}
