package application

import (
	"context"
	"fmt"

	"cmd/product-management/application/ports"
	product "cmd/product-management/domain"
)

// ProductQueryService is the read side of CQRS. It only depends on the
// ProductRepository port for reads.
type ProductQueryService struct {
	repo ports.ProductRepository
}

func NewProductQueryService(repo ports.ProductRepository) *ProductQueryService {
	return &ProductQueryService{repo: repo}
}

// ProductView is the data shape returned by the read side. It is a DTO,
// purposely separate from the domain entity so the read API can evolve
// independently of the internal model.
type ProductView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

// ListProducts returns all known products as DTOs.
func (s *ProductQueryService) ListProducts(ctx context.Context) ([]ProductView, error) {
	products, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	views := make([]ProductView, 0, len(products))
	for _, p := range products {
		views = append(views, viewOf(p))
	}
	return views, nil
}

func viewOf(p product.Product) ProductView {
	return ProductView{
		ID:    string(p.ID()),
		Name:  string(p.Name()),
		Price: int64(p.Price()),
	}
}
