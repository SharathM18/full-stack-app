package product

import (
	"context"

	"github.com/sharathm18/myapp/internal/server"
)

type ProductService struct {
	AppServer         *server.APP
	ProductRepository *ProductRepository
}

func NewProductService(appSrv *server.APP, productRepo *ProductRepository) *ProductService {
	return &ProductService{
		AppServer:         appSrv,
		ProductRepository: productRepo,
	}
}

func (s *ProductService) CreateProduct(ctx context.Context, userID string, payload *CreateProductPayload) (*ProductModel, error) {
	// Call the repository to create the product
	product, err := s.ProductRepository.CreateProduct(ctx, userID, payload)
	if err != nil {
		return nil, err
	}

	return product, nil
}
