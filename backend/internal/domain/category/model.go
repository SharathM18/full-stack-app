package category

import (
	"time"
	"uuid"
)

type CategoryModel struct {
	ID          uuid.UUID `json:"id" db:"id"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	UserID      uuid.UUID `json:"userId" db:"user_id"`
}
