// Package product is the domain layer for the Product entity.
//
// It is intentionally free of any infrastructure (no HTTP, no DB, no
// logger). Anything outside this package depends on this package, never
// the reverse. This is the inner-most layer of the hexagon.
package product

import "strings"

// ID is the unique identifier of a Product.
type ID string

// Name is the product display name.
type Name string

// Price is the product price in minor currency units (e.g. cents) so we
// can keep the value as int64 and avoid floating-point issues.
type Price int64

// Product is the domain entity. It only exposes a constructor that
// enforces the invariants - callers never build one with a struct
// literal, which keeps invalid Products from existing in the first place.
type Product struct {
	id    ID
	name  Name
	price Price
}

// New builds a valid Product or returns an error explaining why the input
// is invalid.
func New(id ID, name Name, price Price) (Product, error) {
	if strings.TrimSpace(string(name)) == "" {
		return Product{}, ErrEmptyName
	}
	if price <= 0 {
		return Product{}, ErrInvalidPrice
	}
	return Product{
		id:    id,
		name:  name,
		price: price,
	}, nil
}

func (p Product) ID() ID       { return p.id }
func (p Product) Name() Name   { return p.name }
func (p Product) Price() Price { return p.price }
