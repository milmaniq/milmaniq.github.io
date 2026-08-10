// Package inmemory contains the outbound adapter that stores products in
// process memory. It implements application.ProductRepository.
package inmemory

import (
	"context"
	"sort"
	"sync"

	product "cmd/product-management/domain"
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
func (r *ProductRepository) Save(_ context.Context, p product.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.store[p.ID()]; exists {
		return product.ErrAlreadyExists
	}
	r.store[p.ID()] = p
	return nil
}

// FindAll returns all products, sorted by name so callers see a stable
// order (Go map iteration is randomized). The returned slice is a copy
// - mutating it cannot corrupt the repository.
func (r *ProductRepository) FindAll(_ context.Context) ([]product.Product, error) {
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
