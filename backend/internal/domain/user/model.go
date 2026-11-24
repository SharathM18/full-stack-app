package user

import (
	"time"

	"github.com/google/uuid"
)

// UserModel is exact projection of a user entity
type UserModel struct {
	ID              uuid.UUID  `db:"id"`
	Name            *string    `db:"name"`
	Email           *string    `db:"email"`
	PhoneNumber     *string    `db:"phone_number"`
	PasswordHash    *string    `db:"password_hash"`
	GoogleID        *string    `db:"google_id"`
	GitHubID        *string    `db:"github_id"`
	Role            string     `db:"role"`
	IsActive        bool       `db:"is_active"`
	EmailVerifiedAt *time.Time `db:"email_verified_at"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}
