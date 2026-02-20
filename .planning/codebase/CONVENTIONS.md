# Coding Conventions

**Analysis Date:** 2026-02-20

## Naming Patterns

**Files:**
- Snake_case for multi-word: `password_recovery.go`, `email_confirm.go`, `phone_confirm.go`
- Single lowercase word for single-concept files: `users.go`, `health.go`, `sender.go`
- Infra packages aliased at import to avoid collision: `jwtinfra`, `s3infra`, `fileapp`

**Functions:**
- Exported: PascalCase — `NewService`, `NewUserRepo`, `Register`, `ClaimsFromContext`
- Unexported: camelCase — `buildUpdateExpr`, `parseCursorPagination`, `realIP`, `generateOTP`
- Constructors: `New{Type}` pattern — `NewService`, `NewUserRepo`, `NewClient`, `NewMailer`

**Variables:**
- camelCase — `dynamoClient`, `sessionSvc`, `refreshDur`, `smsSender`
- Short names acceptable for loop vars and short-lived locals: `u`, `v`, `i`, `k`

**Types:**
- Exported structs: PascalCase — `User`, `Session`, `Service`, `ServiceDeps`, `Config`
- Unexported structs: camelCase — `service`, `mailer`, `sender`, `ipLimiter`
- Interfaces: noun or noun+er — `Service`, `Mailer`, `SMSSender`, `userStore`, `jwtSigner`

**Constants:**
- Exported role constants: PascalCase — `RoleAdmin`, `RoleUser`
- Unexported DynamoDB field names: camelCase prefixed with `field` — `fieldUsername`, `fieldEnable`, `fieldDeletedAt`

## Code Style

**Formatting:**
- Standard `gofmt` (enforced by Go toolchain)
- No additional formatter config detected (no `.editorconfig`, no `golangci-lint` config)

**Linting:**
- No linter config detected (no `.golangci.yml`, no `staticcheck` config)

## Import Organization

**Order (standard Go convention):**
1. Standard library (`context`, `fmt`, `time`, `net/http`, etc.)
2. External dependencies (`github.com/go-chi/chi/v5`, `github.com/aws/...`, etc.)
3. Internal packages (`github.com/go-api-nosql/internal/...`)

**Path Aliases:**
- Use aliases when package name conflicts with another: `jwtinfra "github.com/go-api-nosql/internal/infrastructure/jwt"`, `s3infra "github.com/go-api-nosql/internal/infrastructure/s3"`, `fileapp "github.com/go-api-nosql/internal/application/file"`, `appmiddleware "github.com/go-api-nosql/internal/transport/http/middleware"`

## Error Handling

**Patterns:**
- Domain sentinel errors defined in `internal/domain/errors.go`: `ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrForbidden`, `ErrBadRequest`
- Services wrap with context: `fmt.Errorf("username already taken: %w", domain.ErrConflict)`
- Handlers use `httpError(w, err)` from `internal/transport/http/handler/envelopes.go` which maps via `errors.Is`
- Infrastructure errors (DynamoDB, S3) bubble up and become HTTP 500 — details never exposed to client
- Non-critical failures logged with `slog.Warn` rather than returning an error (e.g., failed OTP record deletion)
- Constructor errors at startup cause `log.Fatal` or graceful fallback with `log.Printf("WARN: ...")`

## Logging

**Framework:** `log/slog` (structured) for request and bootstrap logging; stdlib `log` in `main.go` for startup/shutdown

**Patterns:**
- All HTTP requests: `slog.Info("request", "method", ..., "path", ..., "status", ..., "duration_ms", ..., "remote_addr", ...)` — see `middleware/logging.go`
- Non-fatal infra warnings: `slog.Warn("message", "key", val, "err", err)`
- Bootstrap table creation: `slog.Info("created table", "table", name)`
- Fatal startup errors: `log.Fatalf(...)`
- Do NOT log sensitive data (passwords, tokens, OTPs)

## Comments

**When to Comment:**
- Exported types and functions that need clarification beyond the name
- Non-obvious design decisions or security notes (e.g., X-Forwarded-For spoofing caveat in `ratelimit.go`)
- Package-level doc comments on exported constants blocks

**Style:**
- Single-line `//` comments only (no block `/* */` comments)
- Comment the exported interface, not just the unexported implementation
- Security notes in comments where relevant (see `middleware/ratelimit.go`)

## Function Design

**Size:** Functions stay under 50 lines; long orchestration functions (e.g., `NewRouter`) are acceptable as wiring-only code

**Parameters:**
- Max ~4 direct params; group related deps into `ServiceDeps` struct when more are needed
- Example: `user.ServiceDeps{UserRepo, SessionRepo, DeviceRepo, JWTProvider, RefreshTokenDur}`

**Return Values:**
- Prefer `(value, error)` pairs
- Multiple meaningful values use named position (e.g., `(sess *domain.Session, bearer string, refreshToken string, err error)`)

## Module Design

**Exports:** Export the interface, keep the implementation struct unexported (`service` satisfies `Service`)

**Barrel Files:** Not used — import specific packages directly

**Dependency Injection:** Via `ServiceDeps` structs passed to `NewService`; no global state except the `validate` singleton in `internal/pkg/validate/validate.go`

**Interface Location:** Consumer-defined — each application service defines its own small store/provider interfaces locally (not in the infrastructure package)

---

*Convention analysis: 2026-02-20*
