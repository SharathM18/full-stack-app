package product

import (
	"context"
)

type ProductHandler struct {
	ProductService *ProductService
}

func NewProductHandler(productService *ProductService) *ProductHandler {
	return &ProductHandler{
		ProductService: productService,
	}
}

func (h *ProductHandler) CreateProduct(ctx context.Context, userID string, payload *CreateProductPayload) error {
	return nil
}
