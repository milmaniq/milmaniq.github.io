package product

import "errors"

var (
	ErrEmptyName     = errors.New("product name is empty")
	ErrInvalidPrice  = errors.New("product price must be positive")
	ErrAlreadyExists = errors.New("product already exists")
)
