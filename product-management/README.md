# product-management

A small Go service that demonstrates the same architectural patterns
used in the company's `user-management` service - **hexagonal
architecture (ports & adapters)** with a **CQRS-style command/query
split** - kept simple, with an in-memory database.

Libraries used (all from go.dev, no company-internal deps):

- [`uber/fx`](https://github.com/uber-go/fx) for dependency injection.
- [`gin`](https://github.com/gin-gonic/gin) for the HTTP router.
- [`zerolog`](https://github.com/rs/zerolog) for structured JSON logging.
- [`google/uuid`](https://github.com/google/uuid) for ID generation.

It exposes exactly two endpoints:

| Method | Path        | Description        |
| ------ | ----------- | ------------------ |
| GET    | `/products` | List all products. |
| POST   | `/products` | Add a new product. |

`price` is in minor currency units (e.g. cents) so it stays an `int64`.
Storage is a `map` behind a `sync.RWMutex`, hidden behind interfaces -
swapping it for a real DB is a single new package and one line in
`main.go`.

## Run it

```bash
go run .
```

```bash
# add a product
curl -X POST -H 'Content-Type: application/json' \
     -d '{"name":"Car Wipers","price":1999}' \
     http://localhost:8080/products
# => 201 {"id":"<hex>"}

# list products
curl http://localhost:8080/products
# => 200 {"products":[{"id":"...","name":"Car Wipers","price":1999}]}
```

## Project layout

```
product-management/
├── main.go                          ← composition root: wires interfaces to adapters
├── go.mod                           ← gin, zerolog, google/uuid (+ transitive)
│
├── domain/product/                  ← pure business rules, no I/O
│   ├── product.go                   ←   Product entity + value types
│   └── errors.go
│
├── application/                     ← use cases (drives the domain)
│   ├── ports.go                     ←   ProductWriter, ProductReader, IDGenerator
│   ├── command_service.go           ←   write side of CQRS (AddProduct)
│   └── query_service.go             ←   read side of CQRS (ListProducts)
│
└── adapters/                        ← all I/O lives here
    ├── in/http/                     ← inbound: HTTP endpoints
    └── out/memory/                  ← outbound: in-memory map (the "database")
```

## How the patterns map onto the code

### Hexagonal architecture (ports & adapters)

- **Domain** (`domain/product`) is the core. It depends on nothing.
- **Application** (`application`) depends only on the domain and on a
  set of *ports* (interfaces in `ports.go`): `ProductWriter`,
  `ProductReader`, `IDGenerator`.
- **Adapters** (`adapters/in`, `adapters/out`) are the concrete
  implementations that plug into the ports. The HTTP handler is a
  *driving* (inbound) adapter; the in-memory repository is a *driven*
  (outbound) adapter.
- **`main.go`** is the **composition root** - the only file that knows
  about both the abstractions and the concrete adapters and wires them
  together. It uses `uber/fx` to express the wiring declaratively (see
  "Dependency injection with fx" below).

The dependency direction is **adapters → application → domain**. Swap
the in-memory repo for SQLite/Postgres by writing a new package that
satisfies `ProductWriter` + `ProductReader`; nothing in the application
or domain layer changes.

### Dependency injection with fx

`main.go` is split into three things, mirroring the company's
`user-management` service:

```go
func appOptions(overrides ...fx.Option) []fx.Option { ... }
func newApp(overrides ...fx.Option) *fx.App         { return fx.New(appOptions(overrides...)...) }
func main()                                         { newApp().Run() }
```

The bag of options is built from two pieces:

- **`fx.Provide(...)`** lists every constructor in the graph
  (`memory.NewProductRepository`, `application.NewCommandService`,
  `httpadapter.NewHandler`, `newLogger`, ...). fx looks at each
  constructor's parameter types and return types and works out the
  order to call them in. Each value is built **at most once** and
  reused everywhere it's needed - so the same `*memory.ProductRepository`
  instance backs both `ProductWriter` and `ProductReader`.
- **`fx.Invoke(registerHTTPServer)`** triggers the side-effects.
  `registerHTTPServer` takes an `fx.Lifecycle` and uses
  `lc.Append(fx.Hook{OnStart, OnStop})` to start the HTTP server in a
  goroutine on app start and call `server.Shutdown(ctx)` on app stop.

`fx.App.Run()` ties it together: it builds the graph, runs all
`OnStart` hooks in dependency order, blocks until SIGINT/SIGTERM (or
until something calls `fx.Shutdowner.Shutdown(...)`), then runs
`OnStop` hooks in reverse order.

The `overrides ...fx.Option` parameter is the testing seam: a test
launches the same graph but passes `fx.Replace(...)` or
`fx.Decorate(...)` to swap in a fake clock, an in-process HTTP
listener, etc., without touching production code.

### CQRS at the port level

Reads and writes go through *separate* interfaces:

- `application.ProductWriter.Save` - used by `CommandService.AddProduct`.
- `application.ProductReader.List` - used by `QueryService.ListProducts`.

Today both interfaces happen to be implemented by the same
`memory.ProductRepository`. Tomorrow we can split them: keep an
in-memory write-through cache for reads, point writes at Postgres,
without touching any code in `application/` or `domain/`. That is what
the CQRS split buys you.

The HTTP handler likewise has a clean separation:
`POST /products → CommandService`, `GET /products → QueryService`.

### End-to-end flow

`POST /products`:

1. `addProduct` HTTP handler decodes JSON into `application.AddProductCommand`.
2. `CommandService.AddProduct` generates an ID via the `IDGenerator` port.
3. `product.New(...)` builds the domain entity (validates name + price).
4. `ProductWriter.Save` persists it - the in-memory adapter just stores
   it in a map under a mutex.
5. The handler returns `201 Created` with the new ID.

`GET /products`:

1. `listProducts` HTTP handler calls `QueryService.ListProducts`.
2. `QueryService` calls `ProductReader.List`, then maps each domain
   entity to a `ProductView` DTO (so the API can evolve independently
   of the internal model).
3. The handler renders `{"products": [...]}` as JSON.

### Where the company project's complexity went

The company `user-management` service uses uber/fx for DI, MongoDB for
storage, gRPC + SCIM for transport, OpenTelemetry for observability,
event sourcing on top of all of that, etc. Those are all *adapters*
and *infrastructure choices*; the **architectural shape (domain →
application ports → adapters) is identical to this project**. To level
up this demo you only swap adapters:

| Concept        | Here                                | Production-grade option                       |
| -------------- | ----------------------------------- | --------------------------------------------- |
| Repository     | `adapters/out/memory/products.go`   | Postgres, SQLite, MongoDB                     |
| HTTP router    | `gin`                               | gRPC + gRPC-gateway, chi, etc.                |
| Logger         | `zerolog`                           | zerolog (already production-grade)            |
| ID generation  | `google/uuid`                       | uuid (already production-grade)               |
| DI / lifecycle | `uber/fx`                           | uber/fx (already production-grade)            |
| Persistence    | In-memory map (lost on restart)     | Durable DB + migrations                       |

The application and domain layers wouldn't change at all.
