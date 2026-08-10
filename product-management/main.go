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
	"strings"
	"time"

	httpadapter "cmd/product-management/adapters/in/http"
	"cmd/product-management/adapters/out/inmemory"
	mongoadapter "cmd/product-management/adapters/out/mongo"
	"cmd/product-management/application"
	"cmd/product-management/application/ports"
	product "cmd/product-management/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

const (
	httpAddr            = ":8080"
	shutdownTimeout     = 5 * time.Second
	mongoConnectTimeout = 10 * time.Second
	defaultMongoURI     = "mongodb://localhost:27017"
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
func asIDGenerator() ports.IDGenerator { return uuidIDs{} }

func productRepositoryKind() string {
	switch strings.ToLower(os.Getenv("PRODUCT_REPOSITORY")) {
	case "memory":
		return "memory"
	default:
		return "mongo"
	}
}

func mongoURI() string {
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		return uri
	}
	return defaultMongoURI
}

func newMongoClient(lc fx.Lifecycle, logger zerolog.Logger) (*mongo.Client, error) {
	connectCtx, cancel := context.WithTimeout(context.Background(), mongoConnectTimeout)
	defer cancel()

	client, err := mongoadapter.Connect(connectCtx, mongoURI())
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			if err := mongoadapter.Disconnect(ctx, client); err != nil {
				return err
			}
			logger.Info().Msg("mongo client disconnected")
			return nil
		},
	})

	logger.Info().
		Str("uri", mongoURI()).
		Str("database", "product_management").
		Str("collection", "products").
		Msg("mongo client connected")
	return client, nil
}

func newProductRepository(
	lc fx.Lifecycle,
	logger zerolog.Logger,
) (ports.ProductRepository, error) {
	switch productRepositoryKind() {
	case "memory":
		logger.Info().Msg("using in-memory product repository")
		return inmemory.NewProductRepository(), nil
	default:
		client, err := newMongoClient(lc, logger)
		if err != nil {
			return nil, err
		}
		return mongoadapter.NewProductRepository(client), nil
	}
}

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

			// Outbound adapter: mongo (default) or in-memory via PRODUCT_REPOSITORY=memory.
			newProductRepository,

			// Application services (use cases)
			application.NewProductCommandService,
			application.NewProductQueryService,

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
