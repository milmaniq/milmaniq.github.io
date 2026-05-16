// Package memory contains the outbound adapter that stores products in
// process memory. It implements both application.ProductWriter and
// application.ProductReader, so a single instance backs both sides of
// the CQRS split.
//
// Swapping this for a real database (Postgres, SQLite, ...) means
// writing a sibling package that satisfies the same interfaces and
// changing one line in main.go.
package memory

import (
	"context"
	"sort"
	"sync"

	"cmd/product-management/domain/product"
)

// ProductRepository is the in-memory database. The map is the database;
// the mutex keeps it safe under concurrent HTTP requests.
type ProductRepository struct {
	mu    sync.RWMutex
	store map[product.ID]product.Product
}

func NewProductRepository() *ProductRepository {
	return &ProductRepository{
		store: make(map[product.ID]product.Product),
	}
}

// Save persists a product. Returns ErrAlreadyExists if a product with the
// same ID is already stored, mirroring how a real DB would behave with a
// unique key.
func (r *ProductRepository) Save(ctx context.Context, p product.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.store[p.ID()]; exists {
		return product.ErrAlreadyExists
	}
	r.store[p.ID()] = p
	return nil
}

// List returns all products, sorted by name so callers see a stable
// order (Go map iteration is randomized). The returned slice is a copy
// - mutating it cannot corrupt the repository.
func (r *ProductRepository) List(ctx context.Context) ([]product.Product, error) {
	r.mu.RLock()
	out := make([]product.Product, 0, len(r.store))
	for _, p := range r.store {
		out = append(out, p)
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out, nil
}
