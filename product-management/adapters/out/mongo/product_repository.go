// Package mongo is the outbound adapter that stores products in MongoDB.
// It implements ports.ProductRepository.
package mongo

import (
	"context"
	"errors"
	"fmt"

	product "cmd/product-management/domain"

	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	databaseName   = "product_management"
	collectionName = "products"
)

type productDocument struct {
	ID    string `bson:"_id"`
	Name  string `bson:"name"`
	Price int64  `bson:"price"`
}

// ProductRepository persists products in MongoDB.
type ProductRepository struct {
	coll *mongodriver.Collection
}

func NewProductRepository(client *mongodriver.Client) *ProductRepository {
	return &ProductRepository{
		coll: client.Database(databaseName).Collection(collectionName),
	}
}

// Save inserts a product document. Returns ErrAlreadyExists when _id is taken.
func (r *ProductRepository) Save(ctx context.Context, p product.Product) error {
	doc := productDocument{
		ID:    string(p.ID()),
		Name:  string(p.Name()),
		Price: int64(p.Price()),
	}

	_, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongodriver.IsDuplicateKeyError(err) {
			return product.ErrAlreadyExists
		}
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

// FindAll returns all products sorted by name.
func (r *ProductRepository) FindAll(ctx context.Context) ([]product.Product, error) {
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := r.coll.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find products: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []productDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}

	out := make([]product.Product, 0, len(docs))
	for _, doc := range docs {
		p, err := product.New(product.ID(doc.ID), product.Name(doc.Name), product.Price(doc.Price))
		if err != nil {
			return nil, fmt.Errorf("rebuild product %q: %w", doc.ID, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// Connect dials MongoDB and verifies the server is reachable.
func Connect(ctx context.Context, uri string) (*mongodriver.Client, error) {
	client, err := mongodriver.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongo: %w", err)
	}
	return client, nil
}

// Disconnect closes the MongoDB client, ignoring "already disconnected" errors.
func Disconnect(ctx context.Context, client *mongodriver.Client) error {
	if client == nil {
		return nil
	}
	err := client.Disconnect(ctx)
	if err != nil && !errors.Is(err, mongodriver.ErrClientDisconnected) {
		return err
	}
	return nil
}
