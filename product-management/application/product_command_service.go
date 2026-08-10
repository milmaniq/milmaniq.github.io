package application

import (
	"context"
	"fmt"

	"cmd/product-management/application/ports"
	product "cmd/product-management/domain"
)

// ProductCommandService is the write side of CQRS. It accepts commands,
// runs them through the domain (which enforces invariants) and persists
// the resulting entity through the ProductRepository port.
type ProductCommandService struct {
	repo ports.ProductRepository
	ids  ports.IDGenerator
}

func NewProductCommandService(repo ports.ProductRepository, ids ports.IDGenerator) *ProductCommandService {
	return &ProductCommandService{repo: repo, ids: ids}
}

// AddProductCommand is the input to the AddProduct use case.
type AddProductCommand struct {
	Name  string
	Price int64
}

// AddProductResult is what the use case returns to the caller (an
// inbound adapter, e.g. the HTTP handler).
type AddProductResult struct {
	ID string
}

// AddProduct creates a new product:
//  1. Generate an ID via the abstract IDGenerator port.
//  2. Build the domain entity (validates name + price).
//  3. Persist it through the ProductRepository port.
func (s *ProductCommandService) AddProduct(ctx context.Context, cmd AddProductCommand) (AddProductResult, error) {
	id := s.ids.NewID()

	p, err := product.New(id, product.Name(cmd.Name), product.Price(cmd.Price))
	if err != nil {
		return AddProductResult{}, fmt.Errorf("add product: %w", err)
	}

	if err := s.repo.Save(ctx, p); err != nil {
		return AddProductResult{}, fmt.Errorf("save product: %w", err)
	}
	return AddProductResult{ID: string(id)}, nil
}
