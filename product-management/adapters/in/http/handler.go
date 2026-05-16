// Package http is the inbound adapter that exposes the application
// layer over HTTP. It depends on application.{CommandService, QueryService}
// (the driving ports) and never on the domain or any outbound adapter
// directly.
package http

import (
	"net/http"

	"cmd/product-management/application"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Handler bundles the use-case services that the HTTP layer drives.
type Handler struct {
	commands *application.CommandService
	queries  *application.QueryService
	logger   zerolog.Logger
}

func NewHandler(
	commands *application.CommandService,
	queries *application.QueryService,
	logger zerolog.Logger,
) *Handler {
	return &Handler{
		commands: commands,
		queries:  queries,
		logger:   logger,
	}
}

// Routes builds a fresh gin engine and registers the two endpoints.
// gin.Engine implements http.Handler so the composition root can keep
// using the standard *http.Server for graceful shutdown.
func (h *Handler) Routes() http.Handler {
	engine := gin.New()
	engine.Use(gin.Recovery())
	// Distinguish "wrong method" from "unknown URL" (gin returns 404 by default).
	engine.HandleMethodNotAllowed = true

	engine.GET("/products", h.listProducts)
	engine.POST("/products", h.addProduct)

	return engine
}
