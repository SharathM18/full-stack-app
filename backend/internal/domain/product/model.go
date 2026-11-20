package product

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ProductModel struct {
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
	Title     string    `json:"title" db:"title"`
	// Use a pointer for description because it is nullable in SQL
	Description   *string `json:"description" db:"description"`
	Price         int     `json:"price" db:"price"`
	StockQuantity int     `json:"stockQuantity" db:"stock_quantity"`
	Size          string  `json:"size" db:"size"`
	Status        *string `json:"status" db:"status"`
	// json.RawMessage safely holds Postgres JSONB data
	Metadata json.RawMessage `json:"metadata" db:"metadata"`
	UserID   uuid.UUID       `json:"userId" db:"user_id"`
}

type ProductCategoryModel struct {
	ProductID  uuid.UUID `json:"productId" db:"product_id"`
	CategoryID uuid.UUID `json:"categoryId" db:"category_id"`
}
