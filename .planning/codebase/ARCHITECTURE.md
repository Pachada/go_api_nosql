# Architecture

**Analysis Date:** 2026-02-20

## Pattern Overview

**Overall:** Clean Architecture (Hexagonal / Ports & Adapters)

**Key Characteristics:**
- Strict inward dependency rule: `transport` → `application` → `domain` ← `infrastructure`
- Domain layer has zero external dependencies (pure Go structs + sentinel errors)
- Application services define small, consumer-owned interfaces for their dependencies
- Infrastructure adapters satisfy those interfaces; transport layer wires everything together
- Dependency injection via `ServiceDeps` structs (not a DI framework)

## Layers

**Domain (`internal/domain/`):**
- Purpose: Core business entities and error vocabulary
- Location: `internal/domain/`
- Contains: Structs (`User`, `Session`, `Device`, `Notification`, `File`, `Status`, `UserVerification`, `AppVersion`), sentinel errors (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrForbidden`, `ErrBadRequest`), role constants (`RoleAdmin`, `RoleUser`)
- Depends on: nothing (stdlib `time` and `errors` only)
- Used by: application layer, infrastructure layer, transport layer

**Application (`internal/application/`):**
- Purpose: Business logic and orchestration
- Location: `internal/application/{auth,device,file,notification,session,status,user}/service.go`
- Contains: Service interfaces (exported), private implementation structs, private dependency interfaces
- Depends on: `domain` package only; declares its own store/provider interfaces
- Used by: transport layer

**Infrastructure (`internal/infrastructure/`):**
- Purpose: External system adapters (implements application-layer interfaces)
- Location: `internal/infrastructure/{dynamo,jwt,s3,smtp,sns}/`
- Contains: DynamoDB repos, JWT provider, S3 store, SMTP mailer, SNS sender
- Depends on: `domain`, `config`, AWS SDK
- Used by: `cmd/api/main.go` (wiring) and `internal/transport/http/router.go`

**Transport (`internal/transport/http/`):**
- Purpose: HTTP boundary — decode requests, call services, encode responses
- Location: `internal/transport/http/`
- Contains: `router.go` (wiring), `handler/` (one file per resource), `middleware/` (auth, role, rate-limit, logging)
- Depends on: application layer interfaces, infrastructure types for wiring in `router.go`
- Used by: `cmd/api/main.go`

**Shared Packages (`internal/pkg/`):**
- Purpose: Stateless utilities with no domain/infra dependencies
- Location: `internal/pkg/{id,token,validate,device}/`
- Contains: ULID ID generator, refresh token generator, struct validator wrapper, device resolution helper
- Depends on: `domain` (device package only), external libs (`ulid`, `validator`)

**Config (`internal/config/`):**
- Purpose: Environment variable loading into a typed `Config` struct
- Location: `internal/config/config.go`
- Depends on: stdlib only

**Entry Point (`cmd/api/`):**
- Purpose: Wire all layers together and start the HTTP server
- Location: `cmd/api/main.go`
- Responsibilities: load config, bootstrap DynamoDB, init all infra clients, build `transporthttp.Deps`, create router, run server with graceful shutdown

## Data Flow

**Authenticated API Request:**
1. `chi` router dispatches to handler method
2. `appmiddleware.Auth` validates Bearer JWT → injects `*jwtinfra.Claims` into context
3. Handler decodes JSON body, validates with `validate.Struct`
4. Handler calls application service method (via interface)
5. Service executes business logic, calls repo/provider interfaces
6. Infrastructure adapter executes DynamoDB/S3/SMTP/SNS operation
7. Domain entity returned up the call stack
8. Handler maps domain entity → DTO (e.g., `toSafeUser`) → `writeJSON`

**Auth (Login):**
1. POST `/v1/sessions/login` (rate-limited)
2. `sessionH.Login` → `sessionSvc.Login`
3. Service: fetch user by email/username, bcrypt compare, create session record in DynamoDB, sign RS256 JWT
4. Returns `AuthEnvelope{access_token, refresh_token, session, user}`

**State Management:**
- No in-process state beyond the per-IP rate limiter map (`middleware/ratelimit.go`)
- All persistent state in DynamoDB and S3

## Key Abstractions

**Service Interfaces:**
- Purpose: Decouple transport from application implementation
- Examples: `user.Service` (`internal/application/user/service.go`), `auth.Service` (`internal/application/auth/service.go`)
- Pattern: Exported interface defined in same file as implementation; handlers receive the interface type

**Consumer-Defined Store Interfaces:**
- Purpose: Each application service declares only the repo methods it needs
- Examples: `userStore`, `sessionStore`, `deviceStore` in `internal/application/user/service.go`
- Pattern: Unexported interface, defined at top of service file, satisfied by `*dynamo.XxxRepo`

**DTOs (Response Envelopes):**
- Purpose: Control what fields are exposed in API responses
- Examples: `SafeUser`, `PublicUser`, `SafeSession`, `AuthEnvelope`, `CursorUsersEnvelope`
- Location: `internal/transport/http/handler/envelopes.go`
- Pattern: Domain entity → DTO mapping via `toSafeUser()`, `toPublicUser()`, `toSafeSession()` functions

**Domain Sentinel Errors:**
- Purpose: Type-safe error discrimination across layers without leaking infra details
- Location: `internal/domain/errors.go`
- Pattern: Services wrap with `fmt.Errorf("message: %w", domain.ErrXxx)`; `httpError()` in handlers maps to status codes via `errors.Is`

## Entry Points

**HTTP Server:**
- Location: `cmd/api/main.go`
- Triggers: direct execution (`go run ./cmd/api` or `air`)
- Responsibilities: load env, bootstrap DynamoDB tables, init all infrastructure clients, build router, serve on `APP_PORT`, graceful shutdown on SIGINT/SIGTERM

**Router:**
- Location: `internal/transport/http/router.go`
- Triggers: called from `main.go`
- Responsibilities: instantiate all services and handlers, register routes under `/v1`, apply global and per-route middleware

## Error Handling

**Strategy:** Sentinel error wrapping with centralized HTTP mapping

**Patterns:**
- Infrastructure errors bubble up unwrapped (logged as 500 by default)
- Domain errors wrapped: `fmt.Errorf("context: %w", domain.ErrXxx)`
- `httpError(w, err)` in `internal/transport/http/handler/envelopes.go` maps to HTTP status codes via `errors.Is`
- Infrastructure details never exposed in HTTP responses (generic "internal server error" for unknown errors)

## Cross-Cutting Concerns

**Logging:** `log/slog` structured logging — request logger middleware (`middleware/logging.go`) logs method, path, status, duration_ms, remote_addr on every request; `slog.Warn` used for non-fatal infra errors in bootstrap

**Validation:** `go-playground/validator` via `internal/pkg/validate/validate.go` singleton; called in every handler that accepts a request body

**Authentication:** Bearer JWT middleware (`middleware/auth.go`) injects `*jwtinfra.Claims` into context; `ClaimsFromContext` used in handlers; `RequireRole` middleware enforces RBAC for admin routes

---

*Architecture analysis: 2026-02-20*
