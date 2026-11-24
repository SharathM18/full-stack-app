package auth

import (
	"time"

	"github.com/google/uuid"
)

// UserSessionModel is exact projection of a user session entity
// ofen used for queries output matching
type UserSessionModel struct {
	ID               uuid.UUID  `db:"id"`
	UserID           uuid.UUID  `db:"user_id"`
	RefreshTokenHash string     `db:"refresh_token_hash"`
	UserAgent        *string    `db:"user_agent"`
	IPAddress        *string    `db:"ip_address"`
	ExpiresAt        time.Time  `db:"expires_at"`
	RevokedAt        *time.Time `db:"revoked_at"`
	LastUsedAt       time.Time  `db:"last_used_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// payload to create a user session
type CreateSessionParams struct {
	UserID           uuid.UUID `db:"user_id"`
	RefreshTokenHash string    `db:"refresh_token_hash"`
	UserAgent        *string   `db:"user_agent"`
	IPAddress        *string   `db:"ip_address"`
	ExpiresAt        time.Time `db:"expires_at"`
}
