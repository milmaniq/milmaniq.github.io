// Package application contains the use cases of the service. It depends
// on the domain layer and on a set of *ports* (interfaces) that abstract
// the outside world. Concrete implementations of these ports live in
// adapters/out and are wired in main.go (composition root).
package application

import (
	"context"

	"cmd/product-management/domain/product"
)

// ProductWriter is the outbound port used by the write side. Splitting
// writer and reader into two interfaces is the CQRS pattern at the
// port level: it makes it explicit which side of the system uses which
// capability, and lets us back them by different stores later if we
// want to.
type ProductWriter interface {
	Save(ctx context.Context, p product.Product) error
}

// ProductReader is the outbound port used by the read side.
type ProductReader interface {
	List(ctx context.Context) ([]product.Product, error)
}

// IDGenerator abstracts ID creation so the domain stays deterministic
// and easy to test.
type IDGenerator interface {
	NewID() product.ID
}

// ProductView is the data shape returned by the read side. It is a DTO,
// purposely separate from the domain entity so the read API can evolve
// independently of the internal model.
type ProductView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

// viewOf maps a domain entity to the read DTO. Kept here so the domain
// package stays unaware of any transport / serialization concerns.
func viewOf(p product.Product) ProductView {
	return ProductView{
		ID:    string(p.ID()),
		Name:  string(p.Name()),
		Price: int64(p.Price()),
	}
}
