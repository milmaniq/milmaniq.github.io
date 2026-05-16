package application

import (
	"context"
	"fmt"
)

// QueryService is the read side of CQRS. It only depends on the
// ProductReader port and never touches the writer - keeping reads and
// writes decoupled even though they happen to share storage today.
type QueryService struct {
	reader ProductReader
}

func NewQueryService(reader ProductReader) *QueryService {
	return &QueryService{reader: reader}
}

// ListProducts returns all known products as DTOs.
func (s *QueryService) ListProducts(ctx context.Context) ([]ProductView, error) {
	products, err := s.reader.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	views := make([]ProductView, 0, len(products))
	for _, p := range products {
		views = append(views, viewOf(p))
	}
	return views, nil
}
