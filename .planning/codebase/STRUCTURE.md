# Codebase Structure

**Analysis Date:** 2026-02-20

## Directory Layout

```
go_api_nosql/
├── cmd/
│   └── api/
│       └── main.go              # Entry point — wiring + HTTP server
├── internal/
│   ├── config/
│   │   └── config.go            # Typed env-var config
│   ├── domain/                  # Core entities and error types
│   │   ├── user.go
│   │   ├── session.go
│   │   ├── device.go
│   │   ├── file.go
│   │   ├── notification.go
│   │   ├── status.go
│   │   ├── verification.go
│   │   ├── appversion.go
│   │   ├── role.go              # Role constants (RoleAdmin, RoleUser)
│   │   └── errors.go            # Sentinel errors
│   ├── application/             # Business logic (one package per domain concept)
│   │   ├── auth/service.go      # Password recovery, email/phone confirmation
│   │   ├── device/service.go
│   │   ├── file/service.go
│   │   ├── notification/service.go
│   │   ├── session/service.go   # Login, refresh, logout
│   │   ├── status/service.go
│   │   └── user/service.go      # Register, CRUD
│   ├── infrastructure/          # External system adapters
│   │   ├── dynamo/              # DynamoDB repositories
│   │   │   ├── bootstrap.go     # Table + GSI creation on startup
│   │   │   ├── client.go        # DynamoDB client factory
│   │   │   ├── helpers.go       # strKey, compositeKey, buildUpdateExpr
│   │   │   ├── fields.go        # Shared DynamoDB attribute name constants
│   │   │   ├── users.go
│   │   │   ├── sessions.go
│   │   │   ├── devices.go
│   │   │   ├── files.go
│   │   │   ├── notifications.go
│   │   │   ├── statuses.go
│   │   │   ├── verifications.go
│   │   │   └── appversions.go
│   │   ├── jwt/
│   │   │   └── provider.go      # RS256 sign + verify
│   │   ├── s3/
│   │   │   └── client.go        # S3 client + Store (upload/download/presign/delete)
│   │   ├── smtp/
│   │   │   └── mailer.go        # net/smtp email sender
│   │   └── sns/
│   │       └── sender.go        # AWS SNS SMS sender
│   ├── pkg/                     # Reusable, stateless utilities
│   │   ├── id/id.go             # ULID generator
│   │   ├── token/token.go       # Refresh token generator
│   │   ├── validate/validate.go # Validator singleton wrapper
│   │   └── device/device.go     # Device resolve helper
│   └── transport/
│       └── http/
│           ├── router.go        # Route registration + service wiring
│           ├── handler/         # HTTP handlers (one file per resource)
│           │   ├── envelopes.go # DTOs, writeJSON, httpError
│           │   ├── health.go
│           │   ├── users.go
│           │   ├── sessions.go
│           │   ├── devices.go
│           │   ├── files.go
│           │   ├── notifications.go
│           │   ├── statuses.go
│           │   ├── roles.go
│           │   ├── password_recovery.go
│           │   ├── email_confirm.go
│           │   └── phone_confirm.go
│           └── middleware/      # HTTP middleware
│               ├── auth.go      # JWT validation + context injection
│               ├── role.go      # RBAC role enforcement
│               ├── ratelimit.go # Per-IP token-bucket limiter
│               ├── logging.go   # Structured request logger
│               └── response.go  # writeJSONError helper
├── infra/
│   └── localstack/
│       ├── docker-compose.yml   # LocalStack (DynamoDB, S3, SNS) for local dev
│       └── init-aws.sh          # Runs on LocalStack startup: create tables + bucket
├── .air.toml                    # Hot-reload config for development
├── .env.example                 # Environment variable reference with dev defaults
├── go.mod
├── go.sum
└── openapi.yaml                 # OpenAPI spec
```

## Directory Purposes

**`cmd/api/`:**
- Purpose: Application entry point only — no business logic
- Contains: `main.go` — loads config, bootstraps infra, wires deps, starts HTTP server

**`internal/domain/`:**
- Purpose: Pure Go domain model; no imports from other internal packages
- Contains: Entity structs with `json` + `dynamodbav` tags, request/response structs, sentinel errors, role constants
- Key files: `errors.go`, `user.go`, `session.go`

**`internal/application/`:**
- Purpose: One package per domain concept; each contains a `service.go` with an exported interface and private implementation
- Contains: Business logic, input validation, orchestration of repos/providers
- Key files: `user/service.go`, `auth/service.go`, `session/service.go`

**`internal/infrastructure/dynamo/`:**
- Purpose: DynamoDB repository implementations
- Contains: One file per entity (e.g., `users.go`), shared helpers in `helpers.go` and `fields.go`, startup bootstrap in `bootstrap.go`

**`internal/infrastructure/s3/`:**
- Purpose: S3 file storage adapter
- Key files: `client.go` (contains both `NewClient` and `Store` type)

**`internal/pkg/`:**
- Purpose: Shared utilities used by multiple application packages
- Contains: ID generation (`id/`), refresh token generation (`token/`), validation (`validate/`), device resolution (`device/`)

**`internal/transport/http/handler/`:**
- Purpose: HTTP request/response boundary — decode, delegate to service, encode
- Key files: `envelopes.go` (all DTO types + `writeJSON`/`httpError`), one file per resource

**`internal/transport/http/middleware/`:**
- Purpose: Cross-cutting HTTP concerns
- Key files: `auth.go` (JWT check), `role.go` (RBAC), `ratelimit.go` (per-IP), `logging.go`

**`infra/localstack/`:**
- Purpose: Local development infrastructure only
- Generated: No
- Committed: Yes

## Key File Locations

**Entry Points:**
- `cmd/api/main.go`: HTTP server startup and dependency wiring

**Configuration:**
- `internal/config/config.go`: All env vars → typed `Config` struct
- `.env.example`: All variable names with dev defaults

**Core Logic:**
- `internal/transport/http/router.go`: Route definitions + middleware application
- `internal/transport/http/handler/envelopes.go`: Response DTOs and error mapping
- `internal/domain/errors.go`: Sentinel error values
- `internal/infrastructure/dynamo/bootstrap.go`: Table schema definitions

**Testing:**
- No test files present

## Naming Conventions

**Files:**
- Snake_case for multi-word filenames: `password_recovery.go`, `email_confirm.go`
- Single-word preferred for single-concept files: `users.go`, `health.go`

**Directories:**
- Lowercase single words matching the domain concept: `user`, `session`, `dynamo`, `smtp`

## Where to Add New Code

**New domain entity (e.g., `Product`):**
- Domain struct: `internal/domain/product.go`
- DynamoDB repo: `internal/infrastructure/dynamo/products.go`
- Add table to `internal/infrastructure/dynamo/bootstrap.go` and `internal/config/config.go`
- Application service: `internal/application/product/service.go`
- HTTP handler: `internal/transport/http/handler/products.go`
- Register routes: `internal/transport/http/router.go`
- Add repo to `Deps` struct: `internal/transport/http/router.go`

**New utility:**
- If used by ≥2 application packages: `internal/pkg/{name}/{name}.go`
- If used by one package only: keep it in that package's file

**New middleware:**
- `internal/transport/http/middleware/{name}.go`
- Register in `router.go` with `r.Use(...)` or `r.With(...)`

## Special Directories

**`tmp/`:**
- Purpose: Air hot-reload build output (`./tmp/main.exe`)
- Generated: Yes
- Committed: No (gitignored)

**`infra/localstack/`:**
- Purpose: Docker Compose + init script for local AWS emulation
- Generated: No
- Committed: Yes

---

*Structure analysis: 2026-02-20*
