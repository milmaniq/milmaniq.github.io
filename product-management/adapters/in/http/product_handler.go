package http

import (
	"errors"
	"net/http"

	"cmd/product-management/application"
	product "cmd/product-management/domain"

	"github.com/gin-gonic/gin"
)

// listProducts handles GET /products by calling the read side of CQRS.
func (h *Handler) listProducts(c *gin.Context) {
	views, err := h.queries.ListProducts(c.Request.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("list products failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list products"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"products": views})
}

type addProductRequest struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

// addProduct handles POST /products. It maps the JSON body to an
// AddProductCommand and dispatches it through the write side of CQRS.
func (h *Handler) addProduct(c *gin.Context) {
	var req addProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	res, err := h.commands.AddProduct(c.Request.Context(), application.AddProductCommand{
		Name:  req.Name,
		Price: req.Price,
	})
	if err != nil {
		// Map known domain errors to 400, everything else to 500.
		switch {
		case errors.Is(err, product.ErrEmptyName),
			errors.Is(err, product.ErrInvalidPrice),
			errors.Is(err, product.ErrAlreadyExists):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			h.logger.Error().Err(err).Msg("add product failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add product"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": res.ID})
}
