// Package application contains the use cases of the service. It depends
// on the domain layer and on a set of *ports* (interfaces) that abstract
// the outside world. Concrete implementations of these ports live in
// adapters/out and are wired in main.go (composition root).
package ports

import (
	"context"

	product "cmd/product-management/domain"
)

// ProductRepository is the outbound port for persisting and loading products.
type ProductRepository interface {
	Save(ctx context.Context, p product.Product) error
	FindAll(ctx context.Context) ([]product.Product, error)
}

// IDGenerator abstracts ID creation so the domain stays deterministic
// and easy to test.
type IDGenerator interface {
	NewID() product.ID
}
