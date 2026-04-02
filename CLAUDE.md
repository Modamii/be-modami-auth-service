# be-modami-core-service — Claude Instructions

> Stack: Go 1.25 · Gin · MongoDB · Redis · Kafka · Elasticsearch · Keycloak
> Last updated: 2026-04-02

## Project Context

`be-modami-core-service` is the core backend microservice for the Modami fashion marketplace. It manages products, categories, hashtags, blog/community posts, seller profiles, and the home feed. It is one of several services in the Modami v2 platform; authentication is delegated to Keycloak and user identity comes from `auth-service`/`user-service` (PostgreSQL UUID v4 strings).

**Tech stack summary**: Go · Gin · MongoDB (go.mongodb.org/mongo-driver/v2) · Redis · Kafka (franz-go) · Elasticsearch v7 · Keycloak JWKS · swaggo

---

## Architecture

This service follows hexagonal architecture (ports & adapters):

```
cmd/server/          # Entrypoint — wires dependencies, registers routes
config/              # Viper-based config (env vars / config file)
internal/
  domain/            # Pure domain models — no framework imports
  port/              # Interfaces: Repository, Producer, etc.
  dto/               # Request/response structs; ApplyTo(*domain.X) pattern
  service/           # Business logic — depends on port interfaces only
  events/            # Kafka event types and payloads
  adapter/
    handler/         # Gin HTTP handlers
      middleware/    # Auth (Keycloak JWKS), rate limit, logging
    repository/      # MongoDB implementations of port interfaces
    producer/        # Kafka producer implementations
    consumer/        # Kafka consumer implementations (ES sync)
  command/           # Cobra CLI commands (es reindex, migrations)
pkg/
  kafka/             # Kafka client wrapper (franz-go)
  elasticsearch/     # ES client wrapper
  storage/
    database/mongodb/  # MongoDB helpers, pagination, index setup
    redis/             # Redis cache service
  validator/         # Gin request decode + validate helper
```

---

## Key Design Decisions

### ID Types
- **Internal MongoDB IDs**: `bson.ObjectID` (e.g., product.ID, category.ID)
- **External user/seller IDs**: `string` (UUID v4 from auth-service/user-service PostgreSQL)
  - `SellerID string`, `ModeratorID *string`, `BuyerID string`, `ReviewerID string`, etc.
  - Never use `bson.ObjectIDFromHex()` for user/seller IDs

### DTO ApplyTo Pattern
DTOs expose an `ApplyTo(entity *domain.X)` method for scalar field patching. Fields requiring external DB lookup (e.g., `CategoryID` needing a category fetch) are **excluded** from `ApplyTo` and handled explicitly in the service layer after calling it.

### Kafka Producer — nil-interface safety
In `application.go`, always declare the producer as the interface type:
```go
var productProducer port.ProductProducer   // NOT *producer.ProductProducer
if kafkaProducer != nil {
    productProducer = producer.NewProductProducer(kafkaProducer)
}
```
Using the concrete pointer type creates a non-nil interface wrapping a nil pointer.

### Error handling
Use `gitlab.com/lifegoeson-libs/pkg-gokit/apperror` for domain errors. HTTP handlers call `handleError(c, err)` which maps apperror codes to HTTP status codes.

### Logging
Use `logger.FromContext(ctx)` (from `gitlab.com/lifegoeson-libs/pkg-logging/logger`). Never use `fmt.Print*` or `log.*` in production code.

---

## Critical Rules

1. **Never hardcode secrets or credentials** — all config via env vars / Viper.
2. **No `fmt.Print*` or debug statements** committed — use the logger.
3. **User/seller IDs are `string`** (UUID v4) — never `bson.ObjectID`.
4. **Run `make swagger`** after adding or changing handler annotations. Swagger annotations must reference fully-qualified DTO types (e.g., `dto.CreateCategoryRequest`, not bare `CreateCategoryRequest`).
5. **All commits use Conventional Commits format**.
6. **Do not push** — commit locally only unless explicitly asked to push.
7. **Producer interface variables** must be declared as the interface type, not the concrete pointer, to avoid the nil-interface trap.
8. **ApplyTo excludes CategoryID** — always handle category lookup separately in the service after calling `req.ApplyTo(entity)`.

---

## Project Structure (annotated)

```
cmd/
  server/
    main.go              # cobra root + serve command
    application.go       # DI wiring + route registration
    connections.go       # MongoDB, Redis, ES, Kafka connection setup
config/
  config.go              # Config struct (Viper)
  config.yaml            # Default config (override with env vars)
internal/
  domain/
    product.go           # Product, ProductImage, SelectProduct, etc.
    masterdata.go        # Category, Hashtag
    engagement.go        # Favorite, SavedProduct, Follow, Review
    blog.go              # BlogPost, BlogAuthor
    report.go            # Report
    errors.go            # Domain error sentinels
  port/
    product.go           # ProductRepository, ProductProducer interfaces
    masterdata.go        # CategoryRepository, HashtagRepository
    engagement.go        # FavoriteRepository, FollowRepository, ReviewRepository
    blog.go              # BlogRepository
  dto/
    product_dto.go       # CreateProductRequest, UpdateProductRequest.ApplyTo, ResubmitRequest.ApplyTo
    blog_dto.go          # CreateBlogPostRequest, UpdateBlogPostRequest.ApplyTo
    masterdata_dto.go    # CreateCategoryRequest, UpdateCategoryRequest.ApplyTo
  service/
    product_service.go
    masterdata_service.go
    seller_service.go
    blog_service.go
    home_feed_service.go  # Concurrent 4-section home feed
  events/
    product_events.go    # ProductCreatedPayload, ProductUpdatedPayload, ProductDeletedPayload
  adapter/
    handler/
      product_handler.go
      masterdata_handler.go
      seller_handler.go
      blog_handler.go
      home_feed_handler.go
      search_handler.go
      http_helpers.go      # ok(), created(), handleError(), okWithCursor(), etc.
      swagger_types.go     # StandardSuccessEnvelope, StandardErrorEnvelope for swaggo
      middleware/
        auth.go            # Keycloak JWKS JWT validation
        ratelimit.go
        logging.go
    repository/
      product_repo.go
      masterdata_repo.go
      engagement_repo.go
      blog_repo.go
    producer/
      product_producer.go  # ProductCreatedWithData, ProductUpdatedWithData, ProductDeleted
    consumer/
      sync_product_consumer.go  # Kafka → ES sync (created/updated/deleted)
  command/
    es_command.go          # CLI: es init-index / delete-index / reindex / health
    migrate_command.go
    context.go             # CommandContext (shares connections with CLI)
pkg/
  kafka/
    kafka.go               # KafkaService wrapping franz-go
    topics.go              # Topic name constants
  elasticsearch/
    client.go              # ES wrapper (BulkIndexProducts, DeleteProduct, etc.)
  storage/
    database/mongodb/
      mongodb.go           # Connection + EnsureIndexes
      pagination/          # Cursor-based pagination helpers
    redis/
      redis.go             # RedisCacheService
  validator/
    validator.go           # DecodeAndValidateGin
docs/                      # swaggo-generated swagger files
```

---

## Common Commands

```bash
make run          # go run ./cmd/server
make build        # go build -o bin/server ./cmd/server
make test         # go test (excludes pkg/kafka)
make swagger      # regenerate swagger docs
make lint         # golangci-lint run
make tidy         # go mod tidy
make docker-up    # docker-compose up -d
make docker-down  # docker-compose down

# CLI commands (after build)
./bin/server es init-index
./bin/server es reindex --batch-size 100 --clean
./bin/server es health
```

---

## Git Conventions

### Commit Format
```
<type>(<scope>): <short description>

[optional body]
[optional footer: Closes #issue]
```

**Types**: `feat` · `fix` · `docs` · `style` · `refactor` · `test` · `chore` · `perf` · `ci`

**Scopes**: `product` · `category` · `blog` · `seller` · `feed` · `search` · `auth` · `kafka` · `es` · `config` · `ci`

Examples:
```
feat(feed): add /home-feeds endpoint with 4 concurrent sections
fix(product): remove nil-interface trap in kafka producer wiring
refactor(dto): add ApplyTo pattern to UpdateProductRequest
```

### Branch Naming
```
feature/<scope>-short-description
fix/<scope>-short-description
chore/<description>
refactor/<description>
```

---

## API Overview

Base path: `/v1/core-services`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/home-feeds` | — | 4-section home feed |
| GET | `/products/feed` | — | Cursor-paginated product feed |
| GET | `/products/featured` | — | Featured products |
| GET | `/products/select` | — | Select/curated products |
| GET | `/products/search` | — | Product search |
| GET | `/products/me` | Required | Authenticated seller's products |
| GET | `/products/:id` | — | Product by ID |
| POST | `/products` | Required | Create product |
| PUT | `/products/:id` | Required | Update product |
| DELETE | `/products/:id` | Required | Delete product |
| POST | `/products/:id/submit` | Required | Submit for moderation |
| GET | `/categories` | — | List categories |
| POST | `/categories` | Required + `category.create` | Create category |
| PUT | `/categories/:id` | Required + `category.update` | Update category |
| GET | `/sellers/:id` | — | Seller profile |
| GET | `/community` | — | Community feed |
| GET | `/blog/posts` | — | Blog post list |
| POST | `/blog/posts` | Required + `blog.create` | Create blog post |
| GET | `/health` | — | Health check |
| GET | `/swagger/*any` | — | Swagger UI |

Swagger UI: `http://localhost:8080/swagger/index.html`

---

## Environment Variables (key ones)

```
APP_HOST / APP_PORT              # Server listen address (default :8080)
MONGODB_URI                      # MongoDB connection string
REDIS_ADDR                       # Redis address
ELASTICSEARCH_URL                # ES endpoint
KEYCLOAK_JWKS_URL               # Keycloak JWKS endpoint for JWT validation
KAFKA_BROKERS                    # Comma-separated broker list
KAFKA_ENABLE                     # true/false
OBSERVABILITY_SERVICE_NAME       # Service name for logs/traces
```
