# [Service Name] — Claude Instructions

> Stack: Go [x.x] · [Gin / Chi / Echo] · [MongoDB / PostgreSQL] · [Redis] · [Kafka] · [Elasticsearch]
> Last updated: [YYYY-MM-DD]

## Project Context

[2–3 sentences: what this service does, what domain it owns, and which other services it interacts with.]

**Tech stack summary**: Go · [HTTP framework] · [Primary DB] · [Cache] · [Message broker] · [Auth mechanism]

---

## Architecture

This service follows hexagonal architecture (ports & adapters):

```
cmd/server/          # Entrypoint — wires dependencies, registers routes
config/              # Viper-based config (env vars / config file)
internal/
  domain/            # Pure domain models — no framework imports
  port/              # Interfaces: Repository, Producer, Consumer, etc.
  dto/               # Request/response structs; ApplyTo(*domain.X) pattern
  service/           # Business logic — depends on port interfaces only
  events/            # Event types and payloads (Kafka / domain events)
  adapter/
    handler/         # HTTP handlers
      middleware/    # Auth, rate limit, logging
    repository/      # DB implementations of port interfaces
    producer/        # Message producer implementations
    consumer/        # Message consumer implementations
  command/           # Cobra CLI commands
pkg/
  [shared packages]
```

---

## Key Design Decisions

### ID Types
- **Internal DB IDs**: [bson.ObjectID / uuid.UUID / int64] — describe when each is used
- **External user/service IDs**: [string UUID v4 / int64] — describe source service and why

### DTO ApplyTo Pattern
DTOs expose `ApplyTo(entity *domain.X)` for scalar field patching. Fields requiring external DB lookup are **excluded** from `ApplyTo` and handled in the service layer after calling it.

### Nil-interface safety (Kafka / optional dependencies)
Declare optional dependencies as the interface type, not the concrete pointer:
```go
var myProducer port.MyProducer   // NOT *producer.MyProducer
if dep != nil {
    myProducer = producer.NewMyProducer(dep)
}
```

### Error handling
[Describe the error library and pattern — e.g., apperror codes → HTTP status mapping]

### Logging
[Describe the logger — e.g., `logger.FromContext(ctx)`. Never use `fmt.Print*` or `log.*`.]

---

## Critical Rules

1. **Never hardcode secrets or credentials** — all config via env vars / Viper.
2. **No `fmt.Print*` or debug statements** committed — use the project logger.
3. **[ID type rule]** — e.g., user/seller IDs are `string` UUID v4, never `bson.ObjectID`.
4. **Run `make swagger`** after changing handler annotations. Reference DTO types with full package path (e.g., `dto.CreateXRequest`).
5. **All commits use Conventional Commits format**.
6. **Do not push** — commit locally only unless explicitly asked.
7. **ApplyTo excludes fields requiring DB lookups** — handle them separately in the service.

---

## Project Structure (annotated)

```
cmd/
  server/
    main.go              # Entry point
    application.go       # DI wiring + route registration
    connections.go       # DB / cache / broker connection setup
config/
  config.go              # Config struct
internal/
  domain/                # [List key domain files and what they contain]
  port/                  # [List interface files]
  dto/                   # [List DTO files and key ApplyTo methods]
  service/               # [List service files]
  events/                # [List event type files]
  adapter/
    handler/             # [List handlers]
    repository/          # [List repo implementations]
    producer/            # [List producers]
    consumer/            # [List consumers]
  command/               # [List CLI commands]
pkg/                     # [List shared packages]
docs/                    # Swagger generated files
```

---

## Common Commands

```bash
make run          # go run ./cmd/server
make build        # go build -o bin/server ./cmd/server
make test         # go test ./...
make swagger      # regenerate swagger docs
make lint         # golangci-lint run
make tidy         # go mod tidy
make docker-up    # docker-compose up -d
make docker-down  # docker-compose down
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

**Scopes**: [list domain-relevant scopes, e.g., `product` · `auth` · `kafka` · `es` · `config`]

### Branch Naming
```
feature/<scope>-short-description
fix/<scope>-short-description
chore/<description>
refactor/<description>
```

---

## API Overview

Base path: `/[version]/[service-prefix]`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | — | Health check |
| GET | `/swagger/*any` | — | Swagger UI |
| [add endpoints] | | | |

Swagger UI: `http://localhost:[PORT]/swagger/index.html`

---

## Environment Variables

```
APP_HOST / APP_PORT         # Server listen address
[DB]_URI / [DB]_DSN         # Database connection
[CACHE]_ADDR                # Cache address
[BROKER]_BROKERS            # Message broker addresses
[AUTH]_JWKS_URL             # Auth/JWT validation endpoint
OBSERVABILITY_SERVICE_NAME  # Service name for logs/traces
```

---

## Key Documentation

@docs/architecture-design.md
@INTERGRATE.md
