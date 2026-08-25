package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpadapter "cmd/product-management/adapters/in/http"
	mongoadapter "cmd/product-management/adapters/out/mongo"
	"cmd/product-management/application"
	"cmd/product-management/application/ports"
	product "cmd/product-management/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

type uuidIDs struct{}

func (uuidIDs) NewID() product.ID { return product.ID(uuid.NewString()) }

type testEnv struct {
	srv *httptest.Server
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)

	ctx := context.Background()

	mongoC, err := mongodb.Run(ctx, "mongo:7")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mongoC.Terminate(ctx))
	})

	uri, err := mongoC.ConnectionString(ctx)
	require.NoError(t, err)

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongoadapter.Connect(connectCtx, uri)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mongoadapter.Disconnect(ctx, client))
	})

	repo := mongoadapter.NewProductRepository(client)
	var idGen ports.IDGenerator = uuidIDs{}
	commands := application.NewProductCommandService(repo, idGen)
	queries := application.NewProductQueryService(repo)
	handler := httpadapter.NewHandler(commands, queries, zerolog.Nop())

	srv := httptest.NewServer(handler.Routes())
	t.Cleanup(srv.Close)

	return &testEnv{srv: srv}
}

func (e *testEnv) addProduct(t *testing.T, name string, price int64) string {
	t.Helper()

	resp, err := http.Post(
		e.srv.URL+"/products",
		"application/json",
		bytes.NewBufferString(fmt.Sprintf(`{"name":%q,"price":%d}`, name, price)),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body["id"])
	return body["id"]
}

func (e *testEnv) listProducts(t *testing.T) []application.ProductView {
	t.Helper()

	resp, err := http.Get(e.srv.URL + "/products")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Products []application.ProductView `json:"products"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body.Products
}

func TestProductAPI(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("add product", func(t *testing.T) {
		id := env.addProduct(t, "Widget", 1299)
		require.NotEmpty(t, id)
	})

	t.Run("list products", func(t *testing.T) {
		id := env.addProduct(t, "Gadget", 999)

		products := env.listProducts(t)

		var got *application.ProductView
		for i := range products {
			if products[i].ID == id {
				got = &products[i]
				break
			}
		}
		require.NotNil(t, got, "added product not found in list response")
		require.Equal(t, "Gadget", got.Name)
		require.Equal(t, int64(999), got.Price)
	})
}
