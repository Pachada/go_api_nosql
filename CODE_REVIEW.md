# Code Review — go_api_nosql

Full review covering security, Go idioms ([CLAUDE.md](./CLAUDE.md)), bugs, coding practices, and style.

---

## 🔴 Security

### SEC-1 — CRITICAL: Silent auth bypass when JWT keys are missing
**File:** `internal/transport/http/router.go:60-65`

When `JWTProvider` fails to initialize (missing PEM files), `authMw` is silently replaced with a no-op pass-through. Every authenticated route — including admin-only routes — becomes publicly accessible without any token.

```go
// current — dangerous
if deps.JWTProvider != nil {
    authMw = appmiddleware.Auth(deps.JWTProvider)
} else {
    authMw = func(next http.Handler) http.Handler { return next } // ← all routes open
}
```

**Fix:** Fail fast — `log.Fatal` or `panic` if the provider is unavailable, or return HTTP 503/501 instead of skipping auth entirely.

---

### SEC-2 — HIGH: OTP `validate-code` endpoints are not rate-limited
**Files:** `internal/transport/http/router.go:98`, `internal/transport/http/handler/password_recovery.go`, `email_confirm.go`, `phone_confirm.go`

`sensitiveRL` is applied only to the `request` action. The `validate-code` action (where the OTP is checked) has no rate limit. A 6-digit OTP has only 1,000,000 combinations — brute-forceable with no throttling.

The same applies to `/confirm-email/validate-code` and `/confirm-phone/validate-code`.

**Fix:** Apply `sensitiveRL.Limit` (or a stricter limiter) to the `validate-code` action on all three handlers.

---

### SEC-3 — HIGH: OTP/token comparison is not constant-time
**File:** `internal/application/auth/service.go:128, 203, 243`

```go
if v.Code != req.OTP { ... }        // line 128
if v.Code != token { ... }          // line 203
if v.Code != otp { ... }            // line 243
```

Regular string comparison leaks timing information. An attacker who can measure response latency could distinguish a closer guess from a farther one.

**Fix:** Use `subtle.ConstantTimeCompare([]byte(v.Code), []byte(req.OTP)) != 1`.

---

### SEC-4 — MEDIUM: OTP generator has an off-by-one error
**File:** `internal/application/auth/service.go:97`

```go
n, err := rand.Int(rand.Reader, big.NewInt(999999))
```

`rand.Int` returns a value in `[0, max)`, so this generates 0–999,998 — the value 999,999 is never produced. The correct upper bound is `1_000_000`.

**Fix:** `big.NewInt(1_000_000)`

---

### SEC-5 — MEDIUM: No request body size limit on upload endpoints
**File:** `internal/transport/http/handler/files.go:65` (`UploadBase64`)

The handler itself acknowledges the risk in a comment but never enforces it:
```go
// NOTE: base64 decoding materialises the full payload in memory. Callers
// should enforce a maximum payload size (e.g. via http.MaxBytesReader)
```

A malicious client can send an arbitrarily large base64 body, exhausting server memory.

**Fix:** Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, maxBytes)` before decoding.

---

### SEC-6 — MEDIUM: SNS `SendSMS` ignores the request context
**File:** `internal/infrastructure/sns/sender.go:31`

```go
func (s *sender) SendSMS(to, message string) error {
    _, err := s.client.Publish(context.Background(), ...) // ← ignores ctx
```

The method signature accepts no `ctx` parameter (the `SMSSender` interface only takes `to, message string`). The AWS SDK call uses `context.Background()`, so it cannot be cancelled by the caller or the HTTP request context. This can cause goroutine leaks during graceful shutdown.

**Fix:** Add `ctx context.Context` to the `SMSSender` interface and propagate it to `Publish`.

---

### SEC-7 — MEDIUM: No password strength validation
**File:** `internal/domain/user.go:26`

```go
Password string `json:"password" validate:"required"`
```

`validate:"required"` only rejects empty strings. A single-character password passes validation.

**Fix:** Add a minimum length tag, e.g. `validate:"required,min=8"`.

---

## 🟠 Bugs

### BUG-1: Dead/unreachable code in `RequestPasswordRecovery`
**File:** `internal/application/auth/service.go:88-113`

The `req.PhoneNumber != nil` branch (line 88) always returns an error, making the `smsSender.SendSMS` call on line 113 unreachable. The second `if req.Email != nil` check on line 110 is also always true at that point.

```go
} else if req.PhoneNumber != nil {
    return fmt.Errorf("phone recovery not supported; provide email: %w", domain.ErrBadRequest)
    // ^ always returns, so line 113 is never reached
}
// ...
if req.Email != nil {           // always true here
    return s.mailer.SendEmail(...)
}
return s.smsSender.SendSMS(...) // ← unreachable
```

**Fix:** Remove the final `smsSender.SendSMS` call and the redundant `if req.Email != nil` guard.

---

### BUG-2: `VerificationRepo.Get` does not wrap `domain.ErrNotFound`
**File:** `internal/infrastructure/dynamo/verifications.go:46`

```go
return nil, errors.New("verification not found") // ← bare error
```

Every other repository wraps the sentinel: `fmt.Errorf("... not found: %w", domain.ErrNotFound)`. This inconsistency means `errors.Is(err, domain.ErrNotFound)` returns `false` for verification lookups, causing callers to incorrectly fall into the default 500 branch in `httpError`.

**Fix:** `return nil, fmt.Errorf("verification not found: %w", domain.ErrNotFound)`

---

### BUG-3: `SoftDeleteByUser` silently discards update errors
**File:** `internal/infrastructure/dynamo/sessions.go:73`

```go
_ = r.Update(ctx, sidAttr.Value, map[string]interface{}{"enable": false, "updated_at": now})
```

Errors from individual session updates are thrown away without logging. If an update fails, the session remains active after user deletion — a potential security issue.

**Fix:** Replace `_ =` with `slog.Warn(...)` on error, and consider returning the first error encountered.

---

### BUG-4: `Store.UploadBase64` in `s3/client.go` is dead code
**File:** `internal/infrastructure/s3/client.go:74-81`

`file.Service.UploadBase64` decodes base64 itself and calls `s.s3.Upload` directly. `Store.UploadBase64` is never invoked anywhere in the codebase.

**Fix:** Remove the method to avoid confusion about the intended upload path.

---

## 🟡 Go Patterns (CLAUDE.md violations)

### PAT-1: `NewService` constructors exceed the 4-parameter limit (Rule 2.3)

| Constructor | Parameters |
|---|---|
| `auth.NewService` | 8 |
| `session.NewService` | 5 |
| `user.NewService` | 5 |

**Fix:** Group related dependencies into a named struct, e.g.:
```go
type ServiceDeps struct {
    VerificationRepo *dynamo.VerificationRepo
    UserRepo         *dynamo.UserRepo
    SessionRepo      *dynamo.SessionRepo
    DeviceRepo       *dynamo.DeviceRepo
    Mailer           smtp.Mailer
    SMSSender        sns.SMSSender
    JWTProvider      *jwtinfra.Provider
    RefreshTokenDur  time.Duration
}
func NewService(deps ServiceDeps) Service { ... }
```

---

### PAT-2: Service interfaces are too large (Rule 4.3)

> *"Interfaces with more than three methods are a red flag and should be re-evaluated."*

| Interface | Methods |
|---|---|
| `auth.Service` | 8 |
| `user.Service` | 6 |
| `file.Service` | 5 |
| `session.Service` | 4 |

**Fix:** Consider splitting `auth.Service` into `PasswordRecoveryService`, `EmailConfirmationService`, and `PhoneConfirmationService`.

---

### PAT-3: Services depend on concrete infrastructure types (Rule 4.2)

> *"The function that uses a dependency should define a small interface describing only the behavior it requires."*

All service structs hold `*dynamo.UserRepo`, `*dynamo.SessionRepo`, etc. (concrete pointer types). The application layer is tightly coupled to the infrastructure layer, making testing and swapping implementations difficult.

**Fix:** Define small repository interfaces in the application packages:
```go
// internal/application/session/service.go
type userGetter interface {
    Get(ctx context.Context, userID string) (*domain.User, error)
    GetByUsername(ctx context.Context, username string) (*domain.User, error)
    GetByEmail(ctx context.Context, email string) (*domain.User, error)
}
```

---

### PAT-4: `Deps` struct couples transport directly to infrastructure (Rule 4.2)
**File:** `internal/transport/http/router.go:31-44`

`Deps` imports and holds `*dynamo.*` concrete types. The transport layer should depend on interfaces, not concrete implementations.

---

### PAT-5: `buildUpdateExpr` returns 4 bare values (Rule 2.4)
**File:** `internal/infrastructure/dynamo/helpers.go:26`

> *"If you need to return three or more related values, use a named struct."*

```go
// current
func buildUpdateExpr(...) (expr string, names map[string]string, values map[string]types.AttributeValue, err error)

// suggested
type updateExpr struct {
    Expr   string
    Names  map[string]string
    Values map[string]types.AttributeValue
}
func buildUpdateExpr(...) (updateExpr, error)
```

---

### PAT-6: `ValidateOTP` returns 4 bare values (Rule 2.4)
**File:** `internal/application/auth/service.go:116`

```go
ValidateOTP(ctx context.Context, req ValidateOTPRequest) (bearer, refreshToken string, session *domain.Session, err error)
```

**Fix:** Introduce a `ValidateOTPResult` struct (analogous to `LoginResult` already used in `session.Service`).

---

## 🔵 Coding Practices

### PRAC-1: Cursor pagination has no maximum limit cap
**File:** `internal/transport/http/handler/users.go:124-130`

```go
limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
if limit < 1 {
    limit = 50
}
```

There is a minimum of 1 but no maximum. A request with `?limit=1000000` triggers a full DynamoDB table scan.

**Fix:** Add `if limit > 100 { limit = 100 }` (or a configurable constant).

---

### PRAC-2: Partial updates use `map[string]interface{}` (type-unsafe)
**Files:** `user/service.go`, `device/service.go`, `session/service.go`

Map keys are untyped strings — a typo like `"enable"` vs `"enabled"` fails silently at runtime rather than at compile time.

**Fix:** Consider typed update structs or at minimum define key constants.

---

### PRAC-3: `dynamo/bootstrap.go` uses `log.Printf` instead of `slog`
**File:** `internal/infrastructure/dynamo/bootstrap.go`

The rest of the codebase uses structured `slog` logging. Bootstrap is the only file still using the unstructured `log` package.

**Fix:** Replace with `slog.Info(...)` / `slog.Warn(...)`.

---

### PRAC-4: `contentTypeFromName` / `detectContentType` are duplicated
**Files:** `internal/application/file/service.go:172`, `internal/infrastructure/s3/client.go:117`

Identical logic, different function names. The Rule of Three is met (2 occurrences), but since `Store.UploadBase64` is dead code (BUG-4), only one copy is actually used. Removing the dead code resolves the duplication.

---

### PRAC-5: Boolean query params use non-standard casing
**File:** `internal/transport/http/handler/files.go:44-45`

```go
IsPrivate:   r.URL.Query().Get("private") == "True",
IsThumbnail: r.URL.Query().Get("thumbnail") == "True",
```

The HTTP convention for boolean query parameters is lowercase `"true"`. A client sending `?private=true` (the natural choice) silently gets `IsPrivate: false`.

**Fix:** Use `strings.EqualFold(r.URL.Query().Get("private"), "true")` or simply `== "true"`.

---

## ⚪ Style

### STY-1: Middleware uses `http.Error` instead of `writeError`
**Files:** `middleware/auth.go:21,27`, `middleware/role.go:14,21`, `middleware/ratelimit.go:84`

```go
http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
```

`http.Error` sets `Content-Type: text/plain; charset=utf-8`, not `application/json`. All HTTP handlers use `writeError` which correctly sets the JSON content type. This inconsistency means auth/rate-limit errors have a different format than application errors.

**Fix:** Move `writeError`/`writeJSON` to a shared internal package accessible from both middleware and handlers, then use it in middleware.

---

### STY-2: `ClaimsKey` context key is exported
**File:** `internal/transport/http/middleware/auth.go:13`

```go
type contextKey string
const ClaimsKey contextKey = "claims"
```

The idiomatic Go pattern is an **unexported** context key with an **exported** typed accessor. Exporting the raw key allows other packages to extract claims without going through `ClaimsFromContext`, bypassing the type-safe getter.

**Fix:** `const claimsKey contextKey = "claims"` (unexport the constant).

---

### STY-3: `user.errNotImplemented` should map to a domain error
**File:** `internal/application/user/errors.go`

```go
var errNotImplemented = errors.New("not implemented")
```

This bare sentinel is returned to HTTP callers and falls into the default 500 branch in `httpError`. It should either be wrapped with a domain error or map to HTTP 501 Not Implemented.

---

### STY-4: Trailing blank lines
**Files:** `auth/service.go` (line 270), `user/service.go` (line 177), `session/service.go` (line 156)

Minor — extra blank lines at end of file. `gofmt`/`goimports` normally handles this.

---

## Summary

| Category | Count | Critical/High |
|----------|-------|---------------|
| 🔴 Security | 7 | 3 |
| 🟠 Bugs | 4 | 0 |
| 🟡 Go Patterns | 6 | 0 |
| 🔵 Practices | 5 | 0 |
| ⚪ Style | 4 | 0 |
| **Total** | **26** | **3** |

**Top priorities before shipping:**
1. **SEC-1** — Fix silent auth bypass (critical)
2. **SEC-2** — Rate-limit OTP validate-code endpoints (high)
3. **SEC-3** — Constant-time OTP comparison (high)
4. **BUG-2** — Wrap `domain.ErrNotFound` in `VerificationRepo.Get`
5. **BUG-3** — Stop silently discarding errors in `SoftDeleteByUser`
