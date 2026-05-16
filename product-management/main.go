// product-management is a small example service that demonstrates the
// same architectural patterns as the company's user-management service -
// hex / ports & adapters and a CQRS-style command/query split, wired
// together with uber/fx for dependency injection.
//
// main.go is the composition root: the only place that knows about
// both the abstractions (interfaces in the application package) and
// the concrete adapters that implement them. Everything is registered
// via fx.Provide; side-effects (starting the HTTP server) are wired
// via fx.Invoke + fx.Lifecycle.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	httpadapter "cmd/product-management/adapters/in/http"
	"cmd/product-management/adapters/out/memory"
	"cmd/product-management/application"
	"cmd/product-management/domain/product"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

const (
	httpAddr        = ":8080"
	shutdownTimeout = 5 * time.Second
)

// uuidIDs is the production implementation of application.IDGenerator,
// backed by github.com/google/uuid.
type uuidIDs struct{}

func (uuidIDs) NewID() product.ID { return product.ID(uuid.NewString()) }

// newLogger constructs the application's structured logger.
// Provided to fx so any component can take a zerolog.Logger parameter
// and have it injected.
func newLogger() zerolog.Logger {
	return zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Logger()
}

// asIDGenerator exposes the concrete uuidIDs as the application.IDGenerator
// port. Doing it here means the application package never imports uuid.
func asIDGenerator() application.IDGenerator { return uuidIDs{} }

// asWriter and asReader expose the same in-memory repository instance
// under the two CQRS port interfaces. fx caches values by type, so
// memory.NewProductRepository runs exactly once and both wrappers see
// the same *memory.ProductRepository.
func asWriter(r *memory.ProductRepository) application.ProductWriter { return r }
func asReader(r *memory.ProductRepository) application.ProductReader { return r }

// newHTTPHandler builds the gin engine and exposes it as http.Handler so
// the composition root can wrap it in *http.Server for graceful shutdown.
func newHTTPHandler(h *httpadapter.Handler) http.Handler { return h.Routes() }

// newHTTPServer builds the standard *http.Server. ListenAndServe is NOT
// called here - that's the lifecycle hook's job (see registerHTTPServer).
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// registerHTTPServer wires the *http.Server into fx's lifecycle.
//
// fx supplies the Lifecycle and Shutdowner parameters automatically -
// they're built-ins. Everything else (logger, server) comes from the
// providers above.
func registerHTTPServer(
	lc fx.Lifecycle,
	logger zerolog.Logger,
	server *http.Server,
	shutdowner fx.Shutdowner,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// ListenAndServe blocks until shutdown; run it in a goroutine.
			// Any non-graceful error triggers fx's shutdown sequence.
			go func() {
				logger.Info().Str("addr", server.Addr).Msg("http server starting")
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error().Err(err).Msg("http server error")
					_ = shutdowner.Shutdown(fx.ExitCode(1))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				return err
			}
			logger.Info().Msg("http server stopped")
			return nil
		},
	})
}

// appOptions returns the fx options describing the application graph.
// Splitting it out from newApp lets tests inject overrides via
// fx.Replace / fx.Decorate without redefining the whole graph.
func appOptions(overrides ...fx.Option) []fx.Option {
	// Quiet gin's startup banner; our own logs go through zerolog.
	gin.SetMode(gin.ReleaseMode)

	opts := []fx.Option{
		// fx itself emits a stream of lifecycle events (Provided, Invoked,
		// Started, ...). NopLogger silences them; in production a service
		// would bridge them into its main logger - that's exactly what
		// user-management's internallogger.NewFxLogger does.
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),

		fx.Provide(
			// Infrastructure
			newLogger,
			asIDGenerator,

			// Outbound adapter: in-memory repository, exposed under both
			// CQRS port interfaces.
			memory.NewProductRepository,
			asWriter,
			asReader,

			// Application services (use cases)
			application.NewCommandService,
			application.NewQueryService,

			// Inbound HTTP adapter
			httpadapter.NewHandler,
			newHTTPHandler,
			newHTTPServer,
		),

		// Side effects: start the HTTP server.
		fx.Invoke(registerHTTPServer),
	}
	return append(opts, overrides...)
}

// newApp builds the configured fx.App, optionally with test overrides.
func newApp(overrides ...fx.Option) *fx.App {
	return fx.New(appOptions(overrides...)...)
}

func main() {
	// fx.App.Run() starts the app, blocks until SIGINT / SIGTERM (or until
	// fx.Shutdowner is triggered), then runs OnStop hooks in reverse order.
	newApp().Run()
}
