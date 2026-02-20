# Testing Patterns

**Analysis Date:** 2026-02-20 (updated 2026-02-20)

## Test Framework

**Runner:** `go test` (stdlib)

**Assertion Library:** `github.com/stretchr/testify` v1.11.1
- `testify/assert` — non-fatal assertions
- `testify/require` — fatal assertions (stop test on failure)
- `testify/mock` — interface mocking

**Run Commands:**
```bash
go test ./...                          # Run all tests
go test -v ./...                       # Verbose output
go test -cover ./...                   # Show coverage %
go test ./internal/application/user/  # Single package
go test -run TestRegister ./...        # Single test by name pattern
```

**View Coverage:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Test File Organization

**Location:** Co-located with source (`*_test.go` next to the file being tested)

**Naming:** `<source_file>_test.go` — e.g., `service_test.go`, `helpers_test.go`

**Package:** Same package as source (white-box testing) — e.g., `package user`, `package middleware`

## Test Structure

**Suite Organization:** Flat — no test suites; each `Test*` function is standalone

**Patterns:**
- Arrange/Act/Assert within each test function
- `require.NoError` / `require.Error` for preconditions that must hold
- `assert.*` for non-fatal checks
- `mock.AssertExpectations(t)` at end of tests using mocks

## Mocking

**Framework:** `testify/mock`

**Pattern:** Inline mock structs per test file, implementing the private interface defined in the package under test.

```go
type mockUserStore struct{ mock.Mock }

func (m *mockUserStore) Get(ctx context.Context, id string) (*domain.User, error) {
    args := m.Called(ctx, id)
    if u, _ := args.Get(0).(*domain.User); u != nil {
        return u, args.Error(1)
    }
    return nil, args.Error(1)
}
```

**What to Mock:**
- Store interfaces when testing application services (`userStore`, `sessionStore`, etc.)
- `smtp.Mailer` and `sns.SMSSender` for auth service tests
- `jwtSigner` interface for services that sign tokens

**What NOT to Mock:**
- Domain logic (pure functions, no external deps)
- `internal/pkg/` utilities (stateless, tested directly)

## Fixtures and Factories

**Test Data:** Inline within each test; helper functions like `baseReq()` used where a default request struct is reused across multiple tests.

## Coverage

**Requirements:** None enforced (no CI gate yet)

**Current coverage (packages with tests):**
- `internal/infrastructure/dynamo` — `buildUpdateExpr`
- `internal/application/user` — `Register`, `Update`, `Delete`
- `internal/application/auth` — `RequestPasswordRecovery`, `ValidateOTP`, `ChangePassword`
- `internal/transport/http/middleware` — `Auth`, `RequireRole`, `realIP`

## Test Types

**Unit Tests:** ✅ Present
- `internal/infrastructure/dynamo/helpers_test.go` (4 tests)
- `internal/application/user/service_test.go` (10 tests)
- `internal/application/auth/service_test.go` (11 tests)
- `internal/transport/http/middleware/auth_test.go` (4 tests)
- `internal/transport/http/middleware/role_test.go` (4 tests)
- `internal/transport/http/middleware/ratelimit_test.go` (4 tests)

**Integration Tests:** Not present — recommended next step: DynamoDB repo tests against LocalStack (`AWS_ENDPOINT_URL=http://localhost:4566`)

**E2E Tests:** Not used

## Common Patterns

**Error Testing:** Use `errors.Is` to assert sentinel domain errors (e.g., `assert.True(t, errors.Is(err, domain.ErrConflict))`)

**Middleware Testing:** Use `net/http/httptest.NewRequest` + `httptest.NewRecorder`; inject context values directly with `context.WithValue` for claims

**JWT in Tests:** Generate a fresh RSA key pair with `rsa.GenerateKey` and write to `t.TempDir()` — no fixture key files needed

---

*Testing analysis updated: 2026-02-20*
